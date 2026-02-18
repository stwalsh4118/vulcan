package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	startupTimeout = 10 * time.Second
	pollInterval   = 100 * time.Millisecond
)

// lockedBuffer is a thread-safe wrapper around bytes.Buffer.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lb *lockedBuffer) Write(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(p)
}

func (lb *lockedBuffer) String() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.String()
}

// serverProc holds the running server subprocess and its output.
type serverProc struct {
	cmd    *exec.Cmd
	stdout *lockedBuffer
	url    string
}

var (
	builtBinary string
	buildOnce   sync.Once
	buildErr    error
)

func getBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "vulcan-e2e-*")
		if err != nil {
			buildErr = err
			return
		}
		binary := filepath.Join(dir, "vulcan")
		cmd := exec.Command("go", "build", "-o", binary, "./cmd/vulcan")
		cmd.Dir = filepath.Join(findRepoRoot(t), "api")
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("go build failed: %w\n%s", err, out)
			return
		}
		builtBinary = binary
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return builtBinary
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "api", "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

func startServer(t *testing.T, binary string) *serverProc {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	dbPath := filepath.Join(t.TempDir(), "test.db")

	stdout := &lockedBuffer{}
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		"VULCAN_LISTEN_ADDR="+addr,
		"VULCAN_DB_PATH="+dbPath,
		"VULCAN_LOG_LEVEL=info",
	)
	cmd.Stdout = stdout
	cmd.Stderr = stdout

	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}

	sp := &serverProc{
		cmd:    cmd,
		stdout: stdout,
		url:    "http://" + addr,
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	deadline := time.Now().Add(startupTimeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(sp.url + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return sp
			}
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("server did not become ready within %v\nstdout:\n%s", startupTimeout, stdout.String())
	return nil
}

// AC1: go build produces a binary that starts an HTTP server.
func TestAC1_BinaryBuildsAndStarts(t *testing.T) {
	binary := getBinary(t)
	if _, err := os.Stat(binary); os.IsNotExist(err) {
		t.Fatal("binary does not exist after build")
	}

	sp := startServer(t, binary)
	if sp == nil {
		t.Fatal("server did not start")
	}
}

// AC2: GET /healthz returns 200 with server status.
func TestAC2_Healthz(t *testing.T) {
	binary := getBinary(t)
	sp := startServer(t, binary)

	resp, err := http.Get(sp.url + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
}

// AC3: GET /metrics returns Prometheus-formatted metrics.
func TestAC3_Metrics(t *testing.T) {
	binary := getBinary(t)
	sp := startServer(t, binary)

	resp, err := http.Get(sp.url + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	if !strings.Contains(body, "vulcan_http_requests_total") {
		t.Error("metrics output missing vulcan_http_requests_total")
	}
	if !strings.Contains(body, "vulcan_http_request_duration_seconds") {
		t.Error("metrics output missing vulcan_http_request_duration_seconds")
	}
}

// AC4: POST /v1/workloads accepts a request body and stores it.
func TestAC4_CreateWorkload(t *testing.T) {
	binary := getBinary(t)
	sp := startServer(t, binary)

	payload := `{"runtime":"node","code":"console.log('hello')","resources":{"cpus":1,"mem_mb":128,"timeout_s":30}}`
	resp, err := http.Post(sp.url+"/v1/workloads", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("POST /v1/workloads: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 201\nbody: %s", resp.StatusCode, body)
	}

	var wl map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&wl); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if wl["status"] != "pending" {
		t.Errorf("status = %v, want pending", wl["status"])
	}
	if id, ok := wl["id"].(string); !ok || len(id) != 26 {
		t.Errorf("id = %v, expected 26-char ULID", wl["id"])
	}
}

// AC5: GET /v1/workloads/:id retrieves a stored workload by ID.
func TestAC5_GetWorkloadByID(t *testing.T) {
	binary := getBinary(t)
	sp := startServer(t, binary)

	createResp, err := http.Post(sp.url+"/v1/workloads", "application/json",
		bytes.NewBufferString(`{"runtime":"python"}`))
	if err != nil {
		t.Fatalf("POST /v1/workloads: %v", err)
	}
	var created map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	createResp.Body.Close()
	id, ok := created["id"].(string)
	if !ok {
		t.Fatal("created workload missing id field")
	}

	resp, err := http.Get(sp.url + "/v1/workloads/" + id)
	if err != nil {
		t.Fatalf("GET /v1/workloads/%s: %v", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["id"] != id {
		t.Errorf("id = %v, want %v", got["id"], id)
	}
}

// AC6: GET /v1/workloads lists workloads with pagination.
func TestAC6_ListWorkloadsPagination(t *testing.T) {
	binary := getBinary(t)
	sp := startServer(t, binary)

	for i := 0; i < 3; i++ {
		body := fmt.Sprintf(`{"runtime":"node","code":"test%d"}`, i)
		resp, err := http.Post(sp.url+"/v1/workloads", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("POST /v1/workloads[%d]: %v", i, err)
		}
		resp.Body.Close()
	}

	resp, err := http.Get(sp.url + "/v1/workloads?limit=2&offset=0")
	if err != nil {
		t.Fatalf("GET /v1/workloads: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var listResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	totalRaw, ok := listResp["total"].(float64)
	if !ok {
		t.Fatal("total field missing or not a number")
	}
	if int(totalRaw) != 3 {
		t.Errorf("total = %d, want 3", int(totalRaw))
	}

	workloads, ok := listResp["workloads"].([]any)
	if !ok {
		t.Fatal("workloads field missing or not an array")
	}
	if len(workloads) != 2 {
		t.Errorf("workloads count = %d, want 2", len(workloads))
	}
}

// AC7: DELETE /v1/workloads/:id marks a workload as killed.
func TestAC7_DeleteWorkload(t *testing.T) {
	binary := getBinary(t)
	sp := startServer(t, binary)

	createResp, err := http.Post(sp.url+"/v1/workloads", "application/json",
		bytes.NewBufferString(`{"runtime":"go"}`))
	if err != nil {
		t.Fatalf("POST /v1/workloads: %v", err)
	}
	var created map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	createResp.Body.Close()
	id, ok := created["id"].(string)
	if !ok {
		t.Fatal("created workload missing id field")
	}

	req, _ := http.NewRequest("DELETE", sp.url+"/v1/workloads/"+id, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /v1/workloads/%s: %v", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var deleted map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&deleted); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if deleted["status"] != "killed" {
		t.Errorf("status = %v, want killed", deleted["status"])
	}
}

// AC8: Structured JSON logs are written to stdout on every request.
func TestAC8_StructuredJSONLogs(t *testing.T) {
	binary := getBinary(t)
	sp := startServer(t, binary)

	resp, err := http.Get(sp.url + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	resp.Body.Close()

	// Poll for log output with a deadline.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		output := sp.stdout.String()
		if strings.Contains(output, `"msg":"request"`) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	scanner := bufio.NewScanner(strings.NewReader(sp.stdout.String()))
	foundRequestLog := false
	for scanner.Scan() {
		line := scanner.Text()
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if msg, ok := entry["msg"].(string); ok && msg == "request" {
			foundRequestLog = true
			for _, key := range []string{"method", "path", "status", "duration_ms"} {
				if _, ok := entry[key]; !ok {
					t.Errorf("request log missing field %q", key)
				}
			}
		}
	}
	if !foundRequestLog {
		t.Errorf("no structured request log found in stdout\noutput:\n%s", sp.stdout.String())
	}
}

// AC9: Server is configurable via environment variables.
func TestAC9_EnvVarConfiguration(t *testing.T) {
	binary := getBinary(t)
	sp := startServer(t, binary)

	resp, err := http.Get(sp.url + "/healthz")
	if err != nil {
		t.Fatalf("server not reachable at custom address: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}
