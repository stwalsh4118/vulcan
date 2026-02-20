package guest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	fc "github.com/seantiz/vulcan/internal/backend/firecracker"
)

// executeOverPipe sends a GuestRequest via a pipe and reads back GuestMessages.
func executeOverPipe(t *testing.T, workDir string, req fc.GuestRequest) ([]fc.GuestMessage, fc.GuestResponse) {
	t.Helper()
	server, client := net.Pipe()

	agent := New(nil, filepath.Join(workDir, "work"))

	// Send request and read responses concurrently.
	go func() {
		if err := fc.WriteMessage(client, &req); err != nil {
			t.Errorf("write request: %v", err)
		}
	}()

	// Handle the connection on the server side.
	done := make(chan struct{})
	go func() {
		defer close(done)
		agent.handleConnection(server)
	}()

	// Read all messages from the client side.
	var logs []fc.GuestMessage
	var finalResp fc.GuestResponse
	for {
		var msg fc.GuestMessage
		if err := fc.ReadMessage(client, &msg); err != nil {
			break
		}
		if msg.Type == fc.MsgTypeLog {
			logs = append(logs, msg)
		} else if msg.Type == fc.MsgTypeResult {
			if msg.Response != nil {
				finalResp = *msg.Response
			}
			break
		}
	}

	<-done
	client.Close()
	return logs, finalResp
}

func TestExecuteGoWorkload(t *testing.T) {
	if _, err := findExecutable("go"); err != nil {
		t.Skip("go not available")
	}

	workDir := t.TempDir()
	req := fc.GuestRequest{
		Runtime:  "go",
		Code:     "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello from go\")\n}\n",
		TimeoutS: 30,
	}

	_, resp := executeOverPipe(t, workDir, req)

	if resp.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0; error: %s", resp.ExitCode, resp.Error)
	}
	if !strings.Contains(resp.Output, "hello from go") {
		t.Errorf("Output = %q, want to contain 'hello from go'", resp.Output)
	}
}

func TestExecuteNodeWorkload(t *testing.T) {
	if _, err := findExecutable("node"); err != nil {
		t.Skip("node not available")
	}

	workDir := t.TempDir()
	req := fc.GuestRequest{
		Runtime:  "node",
		Code:     `console.log("hello from node");`,
		TimeoutS: 10,
	}

	logs, resp := executeOverPipe(t, workDir, req)

	if resp.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0; error: %s", resp.ExitCode, resp.Error)
	}
	if !strings.Contains(resp.Output, "hello from node") {
		t.Errorf("Output = %q, want to contain 'hello from node'", resp.Output)
	}
	if len(logs) == 0 {
		t.Error("expected at least one streamed log line")
	}
}

func TestExecutePythonWorkload(t *testing.T) {
	if _, err := findExecutable("python3"); err != nil {
		t.Skip("python3 not available")
	}

	workDir := t.TempDir()
	req := fc.GuestRequest{
		Runtime:  "python",
		Code:     `print("hello from python")`,
		TimeoutS: 10,
	}

	_, resp := executeOverPipe(t, workDir, req)

	if resp.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0; error: %s", resp.ExitCode, resp.Error)
	}
	if !strings.Contains(resp.Output, "hello from python") {
		t.Errorf("Output = %q, want to contain 'hello from python'", resp.Output)
	}
}

func TestExecuteUnsupportedRuntime(t *testing.T) {
	workDir := t.TempDir()
	req := fc.GuestRequest{
		Runtime:  "ruby",
		Code:     `puts "hello"`,
		TimeoutS: 10,
	}

	_, resp := executeOverPipe(t, workDir, req)

	if resp.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", resp.ExitCode)
	}
	if !strings.Contains(resp.Error, "unsupported runtime") {
		t.Errorf("Error = %q, want to contain 'unsupported runtime'", resp.Error)
	}
}

func TestExecuteTimeout(t *testing.T) {
	if _, err := findExecutable("node"); err != nil {
		t.Skip("node not available")
	}

	workDir := t.TempDir()
	req := fc.GuestRequest{
		Runtime:  "node",
		Code:     `setTimeout(() => {}, 60000); // sleep for 60s`,
		TimeoutS: 1,
	}

	_, resp := executeOverPipe(t, workDir, req)

	if resp.ExitCode == 0 {
		t.Error("expected non-zero exit code for timeout")
	}
	if !strings.Contains(resp.Error, "timeout") && !strings.Contains(resp.Error, "signal") {
		t.Errorf("Error = %q, want to indicate timeout or signal", resp.Error)
	}
}

func TestExecuteNonZeroExit(t *testing.T) {
	if _, err := findExecutable("node"); err != nil {
		t.Skip("node not available")
	}

	workDir := t.TempDir()
	req := fc.GuestRequest{
		Runtime:  "node",
		Code:     `process.exit(42);`,
		TimeoutS: 10,
	}

	_, resp := executeOverPipe(t, workDir, req)

	if resp.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", resp.ExitCode)
	}
}

func TestExecuteMultiLineOutput(t *testing.T) {
	if _, err := findExecutable("node"); err != nil {
		t.Skip("node not available")
	}

	workDir := t.TempDir()
	req := fc.GuestRequest{
		Runtime:  "node",
		Code:     "console.log('line1');\nconsole.log('line2');\nconsole.log('line3');",
		TimeoutS: 10,
	}

	logs, resp := executeOverPipe(t, workDir, req)

	if resp.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0; error: %s", resp.ExitCode, resp.Error)
	}
	if len(logs) < 3 {
		t.Errorf("expected at least 3 log lines, got %d", len(logs))
	}
}

func TestExtractCodeInline(t *testing.T) {
	workDir := t.TempDir()
	agent := &Agent{workDir: filepath.Join(workDir, "work")}

	err := agent.extractCode("console.log('test');", "index.js")
	if err != nil {
		t.Fatalf("extractCode: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(agent.workDir, "index.js"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "console.log('test');" {
		t.Errorf("content = %q, want \"console.log('test');\"", string(content))
	}
}

func TestExtractArchiveEndToEnd(t *testing.T) {
	// Create a tar.gz archive in memory with a single file.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	content := []byte(`console.log("from archive");`)
	hdr := &tar.Header{
		Name: "index.js",
		Mode: 0o644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gz.Close()

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	workDir := t.TempDir()
	agent := &Agent{workDir: filepath.Join(workDir, "work")}

	err := agent.extractCode(encoded, "index.js")
	if err != nil {
		t.Fatalf("extractCode: %v", err)
	}

	extracted, err := os.ReadFile(filepath.Join(agent.workDir, "index.js"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(extracted) != `console.log("from archive");` {
		t.Errorf("extracted content = %q, want 'console.log(\"from archive\");'", string(extracted))
	}
}

func TestExtractArchivePathTraversal(t *testing.T) {
	// Create an archive with a path traversal entry.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	hdr := &tar.Header{
		Name: "../../../etc/evil",
		Mode: 0o644,
		Size: 5,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte("pwned"))
	tw.Close()
	gz.Close()

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	workDir := t.TempDir()
	agent := &Agent{workDir: filepath.Join(workDir, "work")}

	err := agent.extractCode(encoded, "index.js")
	if err == nil {
		t.Fatal("expected error for path traversal archive entry")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Errorf("error = %q, want to contain 'escapes'", err.Error())
	}
}

func TestIsBase64Archive(t *testing.T) {
	// Valid gzip magic: 1f 8b
	gzipData := []byte{0x1f, 0x8b, 0x08, 0x00}
	encoded := base64.StdEncoding.EncodeToString(gzipData)
	if !isBase64Archive(encoded) {
		t.Error("expected true for gzip-encoded data")
	}

	if isBase64Archive("console.log('hello');") {
		t.Error("expected false for plain code")
	}

	if isBase64Archive("ab") {
		t.Error("expected false for short string")
	}
}

func TestCustomEntrypoint(t *testing.T) {
	if _, err := findExecutable("node"); err != nil {
		t.Skip("node not available")
	}

	workDir := t.TempDir()
	req := fc.GuestRequest{
		Runtime:    "node",
		Code:       `console.log("custom entrypoint");`,
		Entrypoint: "app.js",
		TimeoutS:   10,
	}

	_, resp := executeOverPipe(t, workDir, req)

	if resp.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0; error: %s", resp.ExitCode, resp.Error)
	}
	if !strings.Contains(resp.Output, "custom entrypoint") {
		t.Errorf("Output = %q, want to contain 'custom entrypoint'", resp.Output)
	}
}

func TestEntrypointPathTraversal(t *testing.T) {
	workDir := t.TempDir()
	req := fc.GuestRequest{
		Runtime:    "node",
		Code:       `console.log("evil");`,
		Entrypoint: "../../etc/evil.js",
		TimeoutS:   10,
	}

	_, resp := executeOverPipe(t, workDir, req)

	if resp.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", resp.ExitCode)
	}
	if !strings.Contains(resp.Error, "invalid entrypoint") {
		t.Errorf("Error = %q, want to contain 'invalid entrypoint'", resp.Error)
	}
}

func TestAgentSurvivesErrors(t *testing.T) {
	// Verify agent handles a second connection after an error on the first.
	workDir := t.TempDir()

	// First request: unsupported runtime (will error).
	_, resp1 := executeOverPipe(t, workDir, fc.GuestRequest{
		Runtime:  "ruby",
		Code:     "puts 'hello'",
		TimeoutS: 10,
	})
	if resp1.ExitCode != 1 {
		t.Errorf("first request: ExitCode = %d, want 1", resp1.ExitCode)
	}

	// Skip second request if node is not available (need a valid runtime to prove survival).
	if _, err := findExecutable("node"); err != nil {
		t.Skip("node not available for second request")
	}

	// Second request: valid workload (should succeed, proving agent didn't crash).
	_, resp2 := executeOverPipe(t, workDir, fc.GuestRequest{
		Runtime:  "node",
		Code:     `console.log("still alive");`,
		TimeoutS: 10,
	})
	if resp2.ExitCode != 0 {
		t.Errorf("second request: ExitCode = %d, want 0; error: %s", resp2.ExitCode, resp2.Error)
	}
	if !strings.Contains(resp2.Output, "still alive") {
		t.Errorf("second request: Output = %q, want to contain 'still alive'", resp2.Output)
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		relPath string
		wantErr bool
	}{
		{"simple filename", "main.go", false},
		{"subdirectory", "src/main.go", false},
		{"parent escape", "../main.go", true},
		{"deep escape", "../../etc/passwd", true},
		{"hidden escape", "foo/../../bar", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath("/work", tt.relPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath(%q) error = %v, wantErr %v", tt.relPath, err, tt.wantErr)
			}
		})
	}
}

// findExecutable checks if a command is available on PATH.
func findExecutable(name string) (string, error) {
	return exec.LookPath(name)
}
