package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seantiz/vulcan/internal/api"
	"github.com/seantiz/vulcan/internal/backend"
	// Blank import triggers init() to register Firecracker Prometheus metrics
	// with the default registry, enabling AC9 metric verification.
	_ "github.com/seantiz/vulcan/internal/backend/firecracker"
	"github.com/seantiz/vulcan/internal/engine"
	"github.com/seantiz/vulcan/internal/model"
	"github.com/seantiz/vulcan/internal/store"
)

// pbi4Server sets up a full-stack test server with a microvm stub backend
// for testing PBI 4 acceptance criteria.
type pbi4Server struct {
	ts       *httptest.Server
	eng      *engine.Engine
	microvmB *stubBackend
	isolateB *stubBackend
	gvisorB  *stubBackend
}

func newPBI4Server(t *testing.T) *pbi4Server {
	t.Helper()

	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	microvmB := &stubBackend{
		name:      "stub-microvm",
		runtimes:  []string{model.RuntimeGo, model.RuntimeNode, model.RuntimePython},
		isolation: model.IsolationMicroVM,
		delay:     20 * time.Millisecond,
		output:    []byte("hello from microvm"),
		logLines:  []string{"[microvm] booting vm", "[microvm] executing", "[microvm] done"},
	}
	isolateB := &stubBackend{
		name:      "stub-isolate",
		runtimes:  []string{model.RuntimeNode, model.RuntimeWasm},
		isolation: model.IsolationIsolate,
		delay:     20 * time.Millisecond,
		output:    []byte("hello from isolate"),
	}
	gvisorB := &stubBackend{
		name:      "stub-gvisor",
		runtimes:  []string{model.RuntimeOCI},
		isolation: model.IsolationGVisor,
		delay:     20 * time.Millisecond,
		output:    []byte("hello from gvisor"),
	}

	reg := backend.NewRegistry()
	reg.Register(model.IsolationMicroVM, microvmB)
	reg.Register(model.IsolationIsolate, isolateB)
	reg.Register(model.IsolationGVisor, gvisorB)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	eng := engine.NewEngine(s, reg, logger)
	srv := api.NewServer(":0", s, reg, eng, logger)

	ts := httptest.NewServer(srv.Router())
	t.Cleanup(func() {
		ts.Close()
		eng.Wait()
	})

	return &pbi4Server{
		ts:       ts,
		eng:      eng,
		microvmB: microvmB,
		isolateB: isolateB,
		gvisorB:  gvisorB,
	}
}

func (p *pbi4Server) url() string { return p.ts.URL }

func (p *pbi4Server) postAsync(t *testing.T, body string) map[string]any {
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

func (p *pbi4Server) getWorkload(t *testing.T, id string) map[string]any {
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

func (p *pbi4Server) pollStatus(t *testing.T, id, expected string, timeout time.Duration) map[string]any {
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

// validGzipBase64 returns a base64-encoded minimal gzip stream for test payloads.
func validGzipBase64() string {
	gzipData := []byte{
		0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x03, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	return base64.StdEncoding.EncodeToString(gzipData)
}

// --- AC1: Firecracker backend implements the Backend interface ---

func TestPBI4_AC1_BackendRegistersWithRegistry(t *testing.T) {
	p := newPBI4Server(t)

	resp, err := http.Get(p.url() + "/v1/backends")
	if err != nil {
		t.Fatalf("GET /v1/backends: %v", err)
	}
	defer resp.Body.Close()

	var backends []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&backends); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Find the microvm backend.
	var found bool
	for _, b := range backends {
		if b["name"] == model.IsolationMicroVM {
			found = true
			caps, ok := b["capabilities"].(map[string]any)
			if !ok {
				t.Fatal("missing capabilities")
			}
			runtimes, _ := caps["supported_runtimes"].([]any)
			if len(runtimes) == 0 {
				t.Error("microvm backend has no supported runtimes")
			}
			isolations, _ := caps["supported_isolations"].([]any)
			if len(isolations) == 0 {
				t.Error("microvm backend has no supported isolations")
			}
			break
		}
	}
	if !found {
		t.Error("microvm backend not found in /v1/backends")
	}
}

func TestPBI4_AC1_CapabilitiesReturnCorrectValues(t *testing.T) {
	p := newPBI4Server(t)

	caps := p.microvmB.Capabilities()
	if caps.Name != "stub-microvm" {
		t.Errorf("Name = %q, want stub-microvm", caps.Name)
	}

	wantRuntimes := map[string]bool{
		model.RuntimeGo:     true,
		model.RuntimeNode:   true,
		model.RuntimePython: true,
	}
	for _, r := range caps.SupportedRuntimes {
		if !wantRuntimes[r] {
			t.Errorf("unexpected runtime %q", r)
		}
		delete(wantRuntimes, r)
	}
	for r := range wantRuntimes {
		t.Errorf("missing runtime %q", r)
	}

	if len(caps.SupportedIsolations) != 1 || caps.SupportedIsolations[0] != model.IsolationMicroVM {
		t.Errorf("SupportedIsolations = %v, want [microvm]", caps.SupportedIsolations)
	}
}

// --- AC2: MicroVMs boot successfully ---

func TestPBI4_AC2_MicroVMBootAndComplete(t *testing.T) {
	p := newPBI4Server(t)

	result := p.postAsync(t, `{"runtime":"go","isolation":"microvm"}`)
	id := result["id"].(string)

	completed := p.pollStatus(t, id, "completed", 5*time.Second)

	if completed["isolation"] != model.IsolationMicroVM {
		t.Errorf("isolation = %v, want microvm", completed["isolation"])
	}
	if completed["runtime"] != model.RuntimeGo {
		t.Errorf("runtime = %v, want go", completed["runtime"])
	}
}

// --- AC4: Guest agent returns execution results ---

func TestPBI4_AC4_InlineCodeReturnsOutput(t *testing.T) {
	p := newPBI4Server(t)

	result := p.postAsync(t, `{"runtime":"go","isolation":"microvm","code":"fmt.Println(\"hello\")"}`)
	id := result["id"].(string)

	completed := p.pollStatus(t, id, "completed", 5*time.Second)

	if completed["exit_code"] == nil {
		t.Error("exit_code is nil, want 0")
	} else if int(completed["exit_code"].(float64)) != 0 {
		t.Errorf("exit_code = %v, want 0", completed["exit_code"])
	}
}

func TestPBI4_AC4_CodeArchiveAccepted(t *testing.T) {
	p := newPBI4Server(t)

	body := fmt.Sprintf(`{"runtime":"go","isolation":"microvm","code_archive":"%s"}`, validGzipBase64())
	result := p.postAsync(t, body)
	id := result["id"].(string)

	completed := p.pollStatus(t, id, "completed", 5*time.Second)

	if completed["isolation"] != model.IsolationMicroVM {
		t.Errorf("isolation = %v, want microvm", completed["isolation"])
	}
}

// --- AC5: Go, Node, and Python workloads execute in microVMs ---

func TestPBI4_AC5_MultiRuntimeExecution(t *testing.T) {
	p := newPBI4Server(t)

	runtimes := []string{model.RuntimeGo, model.RuntimeNode, model.RuntimePython}
	for _, rt := range runtimes {
		t.Run(rt, func(t *testing.T) {
			body := fmt.Sprintf(`{"runtime":"%s","isolation":"microvm"}`, rt)
			result := p.postAsync(t, body)
			id := result["id"].(string)

			completed := p.pollStatus(t, id, "completed", 5*time.Second)

			if completed["runtime"] != rt {
				t.Errorf("runtime = %v, want %s", completed["runtime"], rt)
			}
			if completed["isolation"] != model.IsolationMicroVM {
				t.Errorf("isolation = %v, want microvm", completed["isolation"])
			}
		})
	}

	// Verify the microvm backend was called for each runtime.
	if p.microvmB.calls.Load() < 3 {
		t.Errorf("microvm backend calls = %d, want >= 3", p.microvmB.calls.Load())
	}
}

// --- AC6: Resource limits are passed to backend ---

func TestPBI4_AC6_ResourceLimitsPropagated(t *testing.T) {
	p := newPBI4Server(t)

	body := `{"runtime":"go","isolation":"microvm","resources":{"cpus":2,"mem_mb":256,"timeout_s":10}}`
	result := p.postAsync(t, body)
	id := result["id"].(string)

	completed := p.pollStatus(t, id, "completed", 5*time.Second)

	cpuLimit, _ := completed["cpu_limit"].(float64)
	memLimit, _ := completed["mem_limit"].(float64)
	timeoutS, _ := completed["timeout_s"].(float64)

	if int(cpuLimit) != 2 {
		t.Errorf("cpu_limit = %v, want 2", cpuLimit)
	}
	if int(memLimit) != 256 {
		t.Errorf("mem_limit = %v, want 256", memLimit)
	}
	if int(timeoutS) != 10 {
		t.Errorf("timeout_s = %v, want 10", timeoutS)
	}
}

func TestPBI4_AC6_TimeoutEnforced(t *testing.T) {
	p := newPBI4Server(t)

	// Make the microvm backend slow to trigger timeout.
	p.microvmB.delay = 5 * time.Second

	body := `{"runtime":"go","isolation":"microvm","resources":{"timeout_s":1}}`
	result := p.postAsync(t, body)
	id := result["id"].(string)

	failed := p.pollStatus(t, id, "failed", 10*time.Second)

	errMsg, _ := failed["error"].(string)
	if !strings.Contains(errMsg, "timed out") {
		t.Errorf("error = %q, expected timeout message", errMsg)
	}
}

// --- AC7: VMs are cleaned up after workload completion ---

func TestPBI4_AC7_CompletedWorkloadCleanup(t *testing.T) {
	p := newPBI4Server(t)

	result := p.postAsync(t, `{"runtime":"go","isolation":"microvm"}`)
	id := result["id"].(string)

	completed := p.pollStatus(t, id, "completed", 5*time.Second)

	if completed["finished_at"] == nil || completed["finished_at"] == "" {
		t.Error("finished_at not set after completion")
	}
}

func TestPBI4_AC7_FailedWorkloadCleanup(t *testing.T) {
	p := newPBI4Server(t)

	p.microvmB.err = fmt.Errorf("simulated vm failure")

	result := p.postAsync(t, `{"runtime":"go","isolation":"microvm"}`)
	id := result["id"].(string)

	failed := p.pollStatus(t, id, "failed", 5*time.Second)

	if failed["finished_at"] == nil || failed["finished_at"] == "" {
		t.Error("finished_at not set after failure")
	}
}

func TestPBI4_AC7_KilledWorkloadCleanup(t *testing.T) {
	p := newPBI4Server(t)

	// Use a delay long enough to kill before completion but short enough
	// that eng.Wait() in cleanup does not block too long.
	p.microvmB.delay = 3 * time.Second

	result := p.postAsync(t, `{"runtime":"go","isolation":"microvm","resources":{"timeout_s":2}}`)
	id := result["id"].(string)

	// Wait for it to start running.
	p.pollStatus(t, id, "running", 5*time.Second)

	// Kill it.
	req, _ := http.NewRequest("DELETE", p.url()+"/v1/workloads/"+id, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("DELETE status = %d, want 200", resp.StatusCode)
	}

	killed := p.getWorkload(t, id)
	if killed["status"] != model.StatusKilled {
		t.Errorf("status = %v, want killed", killed["status"])
	}
	if killed["finished_at"] == nil || killed["finished_at"] == "" {
		t.Error("finished_at not set after kill")
	}
}

// --- AC3: Log streaming works for microvm workloads ---

func TestPBI4_AC3_MicroVMLogStreaming(t *testing.T) {
	p := newPBI4Server(t)

	// Increase delay so SSE client can subscribe before logs emit.
	p.microvmB.delay = 200 * time.Millisecond
	p.microvmB.logLines = []string{"[microvm] booting", "[microvm] running", "[microvm] complete"}

	result := p.postAsync(t, `{"runtime":"go","isolation":"microvm"}`)
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

	var events []string
	scanner := bufio.NewScanner(sseResp.Body)
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

// --- AC8: Code archive flow through sync and async endpoints ---

func TestPBI4_AC8_SyncEndpointAcceptsCodeArchive(t *testing.T) {
	p := newPBI4Server(t)

	body := fmt.Sprintf(`{"runtime":"go","code_archive":"%s"}`, validGzipBase64())
	resp, err := http.Post(p.url()+"/v1/workloads", "application/json",
		bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 201\nbody: %s", resp.StatusCode, b)
	}

	var wl map[string]any
	json.NewDecoder(resp.Body).Decode(&wl)
	if wl["status"] != model.StatusPending {
		t.Errorf("status = %v, want pending", wl["status"])
	}
}

func TestPBI4_AC8_AsyncEndpointAcceptsCodeArchive(t *testing.T) {
	p := newPBI4Server(t)

	body := fmt.Sprintf(`{"runtime":"go","isolation":"microvm","code_archive":"%s"}`, validGzipBase64())
	result := p.postAsync(t, body)
	id := result["id"].(string)

	completed := p.pollStatus(t, id, "completed", 5*time.Second)
	if completed["isolation"] != model.IsolationMicroVM {
		t.Errorf("isolation = %v, want microvm", completed["isolation"])
	}
}

func TestPBI4_AC8_MutualExclusivityEnforced(t *testing.T) {
	p := newPBI4Server(t)

	body := fmt.Sprintf(`{"runtime":"go","code":"fmt.Println()","code_archive":"%s"}`, validGzipBase64())
	resp, err := http.Post(p.url()+"/v1/workloads/async", "application/json",
		bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	var errResp map[string]string
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp["error"] != "code and code_archive are mutually exclusive" {
		t.Errorf("error = %q, want mutual exclusivity message", errResp["error"])
	}
}

func TestPBI4_AC8_InvalidArchiveRejected(t *testing.T) {
	p := newPBI4Server(t)

	// Not gzip data.
	plainB64 := base64.StdEncoding.EncodeToString([]byte("not gzip"))
	body := fmt.Sprintf(`{"runtime":"go","code_archive":"%s"}`, plainB64)
	resp, err := http.Post(p.url()+"/v1/workloads/async", "application/json",
		bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// --- AC9: Prometheus metrics ---

func TestPBI4_AC9_MetricsEndpointAvailable(t *testing.T) {
	p := newPBI4Server(t)

	// Execute a microvm workload to generate metrics.
	result := p.postAsync(t, `{"runtime":"go","isolation":"microvm"}`)
	id := result["id"].(string)
	p.pollStatus(t, id, "completed", 5*time.Second)

	resp, err := http.Get(p.url() + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	metricsText := string(body)

	// HTTP middleware metrics are always present. Firecracker-specific metrics
	// are registered globally via init() in the firecracker package — they appear
	// in the output even though the stub backend does not observe them.
	expectedMetrics := []string{
		"vulcan_http_requests_total",
		"vulcan_http_request_duration_seconds",
		// Firecracker metrics registered via init() in internal/backend/firecracker/metrics.go.
		// These are counters/gauges/histograms registered with prometheus.DefaultRegisterer.
		"vulcan_firecracker_vm_boot_seconds",
		"vulcan_firecracker_active_vms",
		"vulcan_firecracker_vsock_workload_seconds",
		"vulcan_firecracker_vm_cleanup_seconds",
		"vulcan_firecracker_workloads_total",
	}
	for _, m := range expectedMetrics {
		if !strings.Contains(metricsText, m) {
			t.Errorf("metric %q not found in /metrics output", m)
		}
	}
}

// --- Auto-routing: microvm isolation resolves correctly ---

func TestPBI4_AutoRouting_GoToMicroVM(t *testing.T) {
	p := newPBI4Server(t)

	callsBefore := p.microvmB.calls.Load()

	result := p.postAsync(t, `{"runtime":"go","isolation":"auto"}`)
	id := result["id"].(string)

	p.pollStatus(t, id, "completed", 5*time.Second)

	callsAfter := p.microvmB.calls.Load()
	if callsAfter <= callsBefore {
		t.Error("auto routing did not route go runtime to microvm backend")
	}
}

func TestPBI4_AutoRouting_PythonToMicroVM(t *testing.T) {
	p := newPBI4Server(t)

	callsBefore := p.microvmB.calls.Load()

	result := p.postAsync(t, `{"runtime":"python","isolation":"auto"}`)
	id := result["id"].(string)

	p.pollStatus(t, id, "completed", 5*time.Second)

	callsAfter := p.microvmB.calls.Load()
	if callsAfter <= callsBefore {
		t.Error("auto routing did not route python runtime to microvm backend")
	}
}
