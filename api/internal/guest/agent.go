// Package guest implements the Firecracker microVM guest agent.
// It handles workload execution inside a microVM: receiving code over vsock,
// extracting it, running it with the appropriate runtime, and streaming results back.
package guest

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	fc "github.com/seantiz/vulcan/internal/backend/firecracker"
)

// defaultEntrypoints maps each runtime to its default entrypoint filename.
var defaultEntrypoints = map[string]string{
	"go":     "main.go",
	"node":   "index.js",
	"python": "main.py",
}

// runtimeCommands maps each runtime to the command used to execute code.
var runtimeCommands = map[string]struct {
	bin  string
	args func(entrypoint string) []string
}{
	"go":     {bin: "go", args: func(ep string) []string { return []string{"run", ep} }},
	"node":   {bin: "node", args: func(ep string) []string { return []string{ep} }},
	"python": {bin: "python3", args: func(ep string) []string { return []string{ep} }},
}

// Agent handles vsock connections and executes workloads.
type Agent struct {
	listener net.Listener
	workDir  string
}

// New creates a new guest agent with the given listener and work directory.
func New(listener net.Listener, workDir string) *Agent {
	return &Agent{
		listener: listener,
		workDir:  workDir,
	}
}

// Serve accepts connections and handles workloads. It blocks until the listener
// is closed or an unrecoverable error occurs.
func (a *Agent) Serve() error {
	for {
		conn, err := a.listener.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		go a.handleConnection(conn)
	}
}

// handleConnection processes a single workload request on conn.
func (a *Agent) handleConnection(conn net.Conn) {
	defer conn.Close()

	var req fc.GuestRequest
	if err := fc.ReadMessage(conn, &req); err != nil {
		log.Printf("read request: %v", err)
		sendResult(conn, fc.GuestResponse{
			ExitCode: 1,
			Error:    fmt.Sprintf("read request: %v", err),
		})
		return
	}

	resp := a.executeWorkload(conn, &req)
	sendResult(conn, resp)
}

// executeWorkload runs the workload described by req, streaming log lines to conn.
func (a *Agent) executeWorkload(conn net.Conn, req *fc.GuestRequest) fc.GuestResponse {
	// Validate runtime.
	rtCmd, ok := runtimeCommands[req.Runtime]
	if !ok {
		return fc.GuestResponse{
			ExitCode: 1,
			Error:    fmt.Sprintf("unsupported runtime: %q", req.Runtime),
		}
	}

	// Determine entrypoint.
	entrypoint := req.Entrypoint
	if entrypoint == "" {
		entrypoint = defaultEntrypoints[req.Runtime]
	}

	// Validate entrypoint stays within work directory (path traversal guard).
	if err := validatePath(a.workDir, entrypoint); err != nil {
		return fc.GuestResponse{
			ExitCode: 1,
			Error:    fmt.Sprintf("invalid entrypoint: %v", err),
		}
	}

	// Extract code to work directory (cleans and recreates it).
	if err := a.extractCode(req.Code, entrypoint); err != nil {
		return fc.GuestResponse{
			ExitCode: 1,
			Error:    fmt.Sprintf("extract code: %v", err),
		}
	}

	// Build command with timeout.
	timeout := time.Duration(req.TimeoutS) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	entrypointPath := filepath.Join(a.workDir, entrypoint)
	cmd := exec.CommandContext(ctx, rtCmd.bin, rtCmd.args(entrypointPath)...)
	cmd.Dir = a.workDir

	// Set environment.
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Pipe input if provided.
	if len(req.Input) > 0 {
		cmd.Stdin = bytes.NewReader(req.Input)
	}

	// Capture stdout and stderr, streaming log lines.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fc.GuestResponse{ExitCode: 1, Error: fmt.Sprintf("stdout pipe: %v", err)}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fc.GuestResponse{ExitCode: 1, Error: fmt.Sprintf("stderr pipe: %v", err)}
	}

	if err := cmd.Start(); err != nil {
		return fc.GuestResponse{ExitCode: 1, Error: fmt.Sprintf("start command: %v", err)}
	}

	// Mutex protects concurrent writes to conn from stdout/stderr goroutines.
	var writeMu sync.Mutex

	// Stream stdout and stderr lines as log messages.
	var output strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamLines(conn, &writeMu, stdoutPipe, &output)
	}()

	var stderrBuf strings.Builder
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		streamLines(conn, &writeMu, stderrPipe, &stderrBuf)
	}()

	<-done
	<-stderrDone

	waitErr := cmd.Wait()

	exitCode := 0
	errMsg := ""
	if waitErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			errMsg = fmt.Sprintf("timeout after %s", timeout)
		} else {
			errMsg = waitErr.Error()
		}
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	combinedOutput := output.String()
	if stderrBuf.Len() > 0 {
		combinedOutput += stderrBuf.String()
	}

	return fc.GuestResponse{
		ExitCode: exitCode,
		Output:   combinedOutput,
		Error:    errMsg,
	}
}

// streamLines reads lines from r, sends each as a log message over conn
// (protected by mu), and appends to output.
func streamLines(conn net.Conn, mu *sync.Mutex, r io.Reader, output *strings.Builder) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		output.WriteString(line + "\n")

		msg := fc.GuestMessage{
			Type: fc.MsgTypeLog,
			Line: line,
		}
		mu.Lock()
		err := fc.WriteMessage(conn, &msg)
		mu.Unlock()
		if err != nil {
			log.Printf("write log line: %v", err)
			return
		}
	}
}

// sendResult sends the final GuestResponse wrapped in a GuestMessage.
func sendResult(conn net.Conn, resp fc.GuestResponse) {
	msg := fc.GuestMessage{
		Type:     fc.MsgTypeResult,
		Response: &resp,
	}
	if err := fc.WriteMessage(conn, &msg); err != nil {
		log.Printf("write result: %v", err)
	}
}

// validatePath checks that joining baseDir with relPath stays within baseDir.
func validatePath(baseDir, relPath string) error {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("resolve base dir: %w", err)
	}
	full := filepath.Join(absBase, relPath)
	cleaned := filepath.Clean(full)
	if !strings.HasPrefix(cleaned, absBase+string(filepath.Separator)) && cleaned != absBase {
		return fmt.Errorf("path %q escapes work directory", relPath)
	}
	return nil
}

// extractCode writes code to the work directory. If the code appears to be
// a base64-encoded tar.gz archive, it is decoded and extracted. Otherwise,
// the code is written as a single file.
func (a *Agent) extractCode(code, entrypoint string) error {
	// Clean and recreate the work directory.
	if err := os.RemoveAll(a.workDir); err != nil {
		return fmt.Errorf("clean work dir: %w", err)
	}
	if err := os.MkdirAll(a.workDir, 0o755); err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}

	// Check if code is a base64-encoded archive.
	if isBase64Archive(code) {
		return extractArchive(a.workDir, code)
	}

	// Write as a single file.
	path := filepath.Join(a.workDir, entrypoint)
	return os.WriteFile(path, []byte(code), 0o644)
}

// isBase64Archive checks if the string looks like a base64-encoded tar.gz.
// tar.gz files start with the gzip magic bytes (1f 8b), so after base64
// decoding the first few bytes should match.
func isBase64Archive(s string) bool {
	if len(s) < 4 {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(s[:4])
	if err != nil {
		return false
	}
	return len(decoded) >= 2 && decoded[0] == 0x1f && decoded[1] == 0x8b
}

// extractArchive decodes a base64-encoded tar.gz and safely extracts it to dir.
// Each entry is validated to prevent path traversal (zip-slip).
func extractArchive(dir, encoded string) error {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("decode base64: %w", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve dir: %w", err)
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		// Validate path stays within extraction directory.
		target := filepath.Join(absDir, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(target, absDir+string(filepath.Separator)) && target != absDir {
			return fmt.Errorf("archive entry %q escapes extraction directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create parent dir: %w", err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755)
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}
			if _, err := io.Copy(f, io.LimitReader(tr, fc.MaxMessageSize)); err != nil {
				f.Close()
				return fmt.Errorf("write file %s: %w", target, err)
			}
			f.Close()
		}
	}

	return nil
}
