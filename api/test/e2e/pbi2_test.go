package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/seantiz/vulcan/internal/api"
	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/engine"
	"github.com/seantiz/vulcan/internal/model"
	"github.com/seantiz/vulcan/internal/store"
)

// stubBackend is a configurable mock backend for E2E tests.
type stubBackend struct {
	name         string
	runtimes     []string
	isolation    string
	delay        time.Duration
	logLineDelay time.Duration // delay between each log line emission
	output       []byte
	logLines     []string
	err          error
	calls        atomic.Int64
}

func (s *stubBackend) Execute(ctx context.Context, spec backend.WorkloadSpec) (backend.WorkloadResult, error) {
	s.calls.Add(1)

	// Delay first to give SSE subscribers time to connect.
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return backend.WorkloadResult{}, ctx.Err()
	}

	// Emit log lines after delay so subscribers can receive them.
	if spec.LogWriter != nil {
		for _, line := range s.logLines {
			spec.LogWriter(line)
			if s.logLineDelay > 0 {
				select {
				case <-time.After(s.logLineDelay):
				case <-ctx.Done():
					return backend.WorkloadResult{}, ctx.Err()
				}
			}
		}
	}

	if s.err != nil {
		return backend.WorkloadResult{}, s.err
	}

	return backend.WorkloadResult{
		ExitCode: 0,
		Output:   s.output,
	}, nil
}

func (s *stubBackend) Capabilities() backend.BackendCapabilities {
	return backend.BackendCapabilities{
		Name:                s.name,
		SupportedRuntimes:   s.runtimes,
		SupportedIsolations: []string{s.isolation},
		MaxConcurrency:      10,
	}
}

func (s *stubBackend) Cleanup(_ context.Context, _ string) error { return nil }

// pbi2Server sets up a full-stack test server with stub backends registered.
type pbi2Server struct {
	ts       *httptest.Server
	eng      *engine.Engine
	isolateB *stubBackend
	microvmB *stubBackend
	gvisorB  *stubBackend
}

func newPBI2Server(t *testing.T) *pbi2Server {
	t.Helper()

	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	isolateB := &stubBackend{
		name:      "stub-isolate",
		runtimes:  []string{model.RuntimeNode, model.RuntimeWasm},
		isolation: model.IsolationIsolate,
		delay:     20 * time.Millisecond,
		output:    []byte("isolate-output"),
		logLines:  []string{"log line 1", "log line 2"},
	}
	microvmB := &stubBackend{
		name:      "stub-microvm",
		runtimes:  []string{model.RuntimeGo, model.RuntimePython},
		isolation: model.IsolationMicroVM,
		delay:     20 * time.Millisecond,
		output:    []byte("microvm-output"),
	}
	gvisorB := &stubBackend{
		name:      "stub-gvisor",
		runtimes:  []string{model.RuntimeOCI},
		isolation: model.IsolationGVisor,
		delay:     20 * time.Millisecond,
		output:    []byte("gvisor-output"),
	}

	reg := backend.NewRegistry()
	reg.Register(model.IsolationIsolate, isolateB)
	reg.Register(model.IsolationMicroVM, microvmB)
	reg.Register(model.IsolationGVisor, gvisorB)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	eng := engine.NewEngine(s, reg, logger)
	srv := api.NewServer(":0", s, reg, eng, logger)

	ts := httptest.NewServer(srv.Router())
	t.Cleanup(func() {
		ts.Close()
		eng.Wait()
	})

	return &pbi2Server{
		ts:       ts,
		eng:      eng,
		isolateB: isolateB,
		microvmB: microvmB,
		gvisorB:  gvisorB,
	}
}

func (p *pbi2Server) url() string { return p.ts.URL }

// postAsync submits an async workload and returns the response body.
func (p *pbi2Server) postAsync(t *testing.T, body string) map[string]any {
	t.Helper()
	resp, err := http.Post(p.url()+"/v1/workloads/async", "application/json",
		bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /v1/workloads/async: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 202\nbody: %s", resp.StatusCode, b)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return result
}

// getWorkload retrieves a workload by ID.
func (p *pbi2Server) getWorkload(t *testing.T, id string) map[string]any {
	t.Helper()
	resp, err := http.Get(p.url() + "/v1/workloads/" + id)
	if err != nil {
		t.Fatalf("GET workload: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return result
}

// pollStatus polls until the workload reaches the expected status.
func (p *pbi2Server) pollStatus(t *testing.T, id, expected string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		wl := p.getWorkload(t, id)
		if wl["status"] == expected {
			return wl
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("workload %s did not reach status %q within %v", id, expected, timeout)
	return nil
}

// --- AC1: Backend interface is defined and documented ---

func TestPBI2_AC1_BackendInterfaceCompiles(t *testing.T) {
	// If this test compiles, the Backend interface exists and is implementable.
	var _ backend.Backend = (*stubBackend)(nil)
}

// --- AC2: Workload status transitions are enforced ---

func TestPBI2_AC2_InvalidTransitionRejected(t *testing.T) {
	p := newPBI2Server(t)

	// Create a workload (sync) — stays in pending state.
	resp, err := http.Post(p.url()+"/v1/workloads", "application/json",
		bytes.NewBufferString(`{"runtime":"node"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	id := created["id"].(string)

	// Kill the workload (pending→killed is valid).
	req, _ := http.NewRequest("DELETE", p.url()+"/v1/workloads/"+id, nil)
	delResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE status = %d, want 200", delResp.StatusCode)
	}

	// Attempt to kill again (killed→killed is invalid).
	req2, _ := http.NewRequest("DELETE", p.url()+"/v1/workloads/"+id, nil)
	delResp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	delResp2.Body.Close()

	if delResp2.StatusCode != http.StatusConflict {
		t.Errorf("second DELETE status = %d, want 409", delResp2.StatusCode)
	}
}

// --- AC3: POST /v1/workloads/async returns 202 ---

func TestPBI2_AC3_AsyncReturns202(t *testing.T) {
	p := newPBI2Server(t)

	result := p.postAsync(t, `{"runtime":"node"}`)

	if result["status"] != "pending" {
		t.Errorf("status = %v, want pending", result["status"])
	}
	if id, ok := result["id"].(string); !ok || len(id) != 26 {
		t.Errorf("id = %v, expected 26-char ULID", result["id"])
	}
}

// --- AC4: GET /v1/workloads/:id reflects real-time status ---

func TestPBI2_AC4_RealTimeStatusUpdates(t *testing.T) {
	p := newPBI2Server(t)

	result := p.postAsync(t, `{"runtime":"node"}`)
	id := result["id"].(string)

	// Poll until completed.
	completed := p.pollStatus(t, id, "completed", 5*time.Second)

	if string(completed["output"].(string)) == "" {
		t.Error("expected output, got empty")
	}
}

// --- AC5: GET /v1/workloads/:id/logs streams log lines via SSE ---

func TestPBI2_AC5_SSELogStreaming(t *testing.T) {
	p := newPBI2Server(t)

	// Use a longer delay so the SSE client can subscribe before logs are emitted.
	// Logs are emitted after the delay in the stub backend.
	p.isolateB.delay = 200 * time.Millisecond
	p.isolateB.logLines = []string{"building", "running", "done"}

	// Submit async workload. The engine goroutine starts and delays 200ms
	// before emitting logs, giving the SSE client time to subscribe.
	result := p.postAsync(t, `{"runtime":"node"}`)
	id := result["id"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", p.url()+"/v1/workloads/"+id+"/logs", nil)
	sseResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET logs: %v", err)
	}
	defer sseResp.Body.Close()

	if ct := sseResp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	// Read SSE events. The scanner blocks until the stream closes (workload finishes).
	scanner := bufio.NewScanner(sseResp.Body)
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			events = append(events, data)
		}
	}

	if len(events) < 3 {
		t.Errorf("got %d log events, want >= 3: %v", len(events), events)
	}
}

// --- AC6: auto isolation mode resolves to correct backend ---

func TestPBI2_AC6_AutoRouting(t *testing.T) {
	p := newPBI2Server(t)

	// Submit with isolation=auto, runtime=node → should route to isolate backend.
	callsBefore := p.isolateB.calls.Load()

	result := p.postAsync(t, `{"runtime":"node","isolation":"auto"}`)
	id := result["id"].(string)

	p.pollStatus(t, id, "completed", 5*time.Second)

	callsAfter := p.isolateB.calls.Load()
	if callsAfter <= callsBefore {
		t.Error("auto routing did not route node runtime to isolate backend")
	}
}

// --- AC7: Timed-out workloads are killed and marked failed ---

func TestPBI2_AC7_TimeoutEnforcement(t *testing.T) {
	p := newPBI2Server(t)

	// Override the isolate backend to be slow.
	p.isolateB.delay = 5 * time.Second

	result := p.postAsync(t, `{"runtime":"node","resources":{"timeout_s":1}}`)
	id := result["id"].(string)

	failed := p.pollStatus(t, id, "failed", 10*time.Second)

	errMsg, _ := failed["error"].(string)
	if !strings.Contains(errMsg, "timed out") {
		t.Errorf("error = %q, expected timeout message", errMsg)
	}
}

// --- AC8: GET /v1/backends returns registered backends ---

func TestPBI2_AC8_ListBackends(t *testing.T) {
	p := newPBI2Server(t)

	resp, err := http.Get(p.url() + "/v1/backends")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var backends []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&backends); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(backends) != 3 {
		t.Fatalf("got %d backends, want 3", len(backends))
	}

	// Verify each has name and capabilities with supported_runtimes.
	for _, b := range backends {
		if b["name"] == nil || b["name"] == "" {
			t.Errorf("backend missing name: %v", b)
		}
		caps, ok := b["capabilities"].(map[string]any)
		if !ok {
			t.Errorf("backend missing capabilities: %v", b)
			continue
		}
		if caps["supported_runtimes"] == nil {
			t.Errorf("backend capabilities missing supported_runtimes: %v", caps)
		}
	}
}

// --- AC9: GET /v1/stats returns aggregate statistics ---

func TestPBI2_AC9_StatsEndpoint(t *testing.T) {
	p := newPBI2Server(t)

	// Submit and wait for a few workloads to complete.
	for _, rt := range []string{"node", "python"} {
		result := p.postAsync(t, `{"runtime":"`+rt+`"}`)
		id := result["id"].(string)
		p.pollStatus(t, id, "completed", 5*time.Second)
	}

	resp, err := http.Get(p.url() + "/v1/stats")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var stats map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	total, _ := stats["total"].(float64)
	if int(total) < 2 {
		t.Errorf("total = %d, want >= 2", int(total))
	}

	byStatus, ok := stats["by_status"].(map[string]any)
	if !ok {
		t.Fatal("by_status missing or wrong type")
	}
	completed, _ := byStatus["completed"].(float64)
	if int(completed) < 2 {
		t.Errorf("by_status.completed = %d, want >= 2", int(completed))
	}

	if stats["avg_duration_ms"] == nil {
		t.Error("avg_duration_ms is missing")
	}
}
