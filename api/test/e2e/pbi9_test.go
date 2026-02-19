package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seantiz/vulcan/internal/api"
	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/engine"
	"github.com/seantiz/vulcan/internal/model"
	"github.com/seantiz/vulcan/internal/store"
)

// pbi9Server sets up a full-stack test server with a logging stub backend.
type pbi9Server struct {
	ts    *httptest.Server
	eng   *engine.Engine
	store *store.SQLiteStore
	stub  *stubBackend
}

func newPBI9Server(t *testing.T, logLines []string) *pbi9Server {
	t.Helper()

	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	stub := &stubBackend{
		name:      "stub-isolate",
		runtimes:  []string{model.RuntimeNode},
		isolation: model.IsolationIsolate,
		delay:     50 * time.Millisecond,
		output:    []byte("done"),
		logLines:  logLines,
	}

	reg := backend.NewRegistry()
	reg.Register(model.IsolationIsolate, stub)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	eng := engine.NewEngine(s, reg, logger)
	srv := api.NewServer(":0", s, reg, eng, logger)

	ts := httptest.NewServer(srv.Router())
	t.Cleanup(func() {
		ts.Close()
		eng.Wait()
	})

	return &pbi9Server{ts: ts, eng: eng, store: s, stub: stub}
}

func (p *pbi9Server) url() string { return p.ts.URL }

func (p *pbi9Server) postAsync(t *testing.T, body string) map[string]any {
	t.Helper()
	resp, err := http.Post(p.url()+"/v1/workloads/async", "application/json",
		strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
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

func (p *pbi9Server) pollStatus(t *testing.T, id, expected string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(p.url() + "/v1/workloads/" + id)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		var wl map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&wl); err != nil {
			resp.Body.Close()
			t.Fatalf("decode: %v", err)
		}
		resp.Body.Close()
		if wl["status"] == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("workload %s did not reach %q within %v", id, expected, timeout)
}

type logHistoryResp struct {
	WorkloadID string `json:"workload_id"`
	Lines      []struct {
		Seq       int    `json:"seq"`
		Line      string `json:"line"`
		CreatedAt string `json:"created_at"`
	} `json:"lines"`
}

func (p *pbi9Server) getHistory(t *testing.T, id string) logHistoryResp {
	t.Helper()
	resp, err := http.Get(p.url() + "/v1/workloads/" + id + "/logs/history")
	if err != nil {
		t.Fatalf("GET history: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("history status = %d, want 200\nbody: %s", resp.StatusCode, b)
	}
	var result logHistoryResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	return result
}

// --- AC1 + AC6: Log lines persisted during execution with correct ordering ---

func TestPBI9_AC1_AC6_LogPersistenceDuringExecution(t *testing.T) {
	expectedLines := []string{"starting", "processing", "complete"}
	p := newPBI9Server(t, expectedLines)

	result := p.postAsync(t, `{"runtime":"node"}`)
	id := result["id"].(string)

	p.pollStatus(t, id, "completed", 5*time.Second)

	history := p.getHistory(t, id)
	if history.WorkloadID != id {
		t.Errorf("workload_id = %q, want %q", history.WorkloadID, id)
	}
	if len(history.Lines) != len(expectedLines) {
		t.Fatalf("len(lines) = %d, want %d", len(history.Lines), len(expectedLines))
	}
	for i, l := range history.Lines {
		if l.Seq != i {
			t.Errorf("lines[%d].seq = %d, want %d", i, l.Seq, i)
		}
		if l.Line != expectedLines[i] {
			t.Errorf("lines[%d].line = %q, want %q", i, l.Line, expectedLines[i])
		}
		if l.CreatedAt == "" {
			t.Errorf("lines[%d].created_at is empty", i)
		}
	}
}

// --- AC2: History endpoint returns correct JSON ---

func TestPBI9_AC2_HistoryEndpointReturnsJSON(t *testing.T) {
	p := newPBI9Server(t, []string{"hello", "world"})

	result := p.postAsync(t, `{"runtime":"node"}`)
	id := result["id"].(string)
	p.pollStatus(t, id, "completed", 5*time.Second)

	resp, err := http.Get(p.url() + "/v1/workloads/" + id + "/logs/history")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var body logHistoryResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Lines) != 2 {
		t.Errorf("len(lines) = %d, want 2", len(body.Lines))
	}
}

// --- AC2: History endpoint returns 404 for non-existent workload ---

func TestPBI9_AC2_HistoryNotFound(t *testing.T) {
	p := newPBI9Server(t, nil)

	resp, err := http.Get(p.url() + "/v1/workloads/nonexistent/logs/history")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// --- AC4: SSE streaming still works for active workloads ---

func TestPBI9_AC4_SSEStreamingUnchanged(t *testing.T) {
	expectedLines := []string{"sse line 1", "sse line 2", "sse line 3"}
	p := newPBI9Server(t, expectedLines)
	p.stub.delay = 200 * time.Millisecond

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

	scanner := bufio.NewScanner(sseResp.Body)
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			events = append(events, data)
		}
	}

	if len(events) != len(expectedLines) {
		t.Fatalf("got %d SSE events, want %d: %v", len(events), len(expectedLines), events)
	}
	for i, e := range events {
		if e != expectedLines[i] {
			t.Errorf("event[%d] = %q, want %q", i, e, expectedLines[i])
		}
	}
}

// --- AC1 + AC4: SSE lines match persisted lines ---

func TestPBI9_SSELinesMatchPersistedLines(t *testing.T) {
	expectedLines := []string{"alpha", "beta", "gamma"}
	p := newPBI9Server(t, expectedLines)
	p.stub.delay = 200 * time.Millisecond

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

	scanner := bufio.NewScanner(sseResp.Body)
	var sseLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			sseLines = append(sseLines, data)
		}
	}

	// After SSE stream ends, fetch the persisted history.
	p.pollStatus(t, id, "completed", 5*time.Second)
	history := p.getHistory(t, id)

	if len(sseLines) != len(history.Lines) {
		t.Fatalf("SSE lines = %d, persisted = %d, want equal", len(sseLines), len(history.Lines))
	}
	for i, sl := range sseLines {
		if sl != history.Lines[i].Line {
			t.Errorf("mismatch at [%d]: SSE=%q, persisted=%q", i, sl, history.Lines[i].Line)
		}
	}
}

// --- AC1: Cross-workload isolation ---

func TestPBI9_CrossWorkloadIsolation(t *testing.T) {
	p := newPBI9Server(t, []string{"shared-line-1", "shared-line-2"})

	// Submit two workloads concurrently.
	r1 := p.postAsync(t, `{"runtime":"node"}`)
	id1 := r1["id"].(string)
	r2 := p.postAsync(t, `{"runtime":"node"}`)
	id2 := r2["id"].(string)

	p.pollStatus(t, id1, "completed", 5*time.Second)
	p.pollStatus(t, id2, "completed", 5*time.Second)

	h1 := p.getHistory(t, id1)
	h2 := p.getHistory(t, id2)

	// Each workload should have its own log lines.
	if len(h1.Lines) != 2 {
		t.Fatalf("w1 lines = %d, want 2", len(h1.Lines))
	}
	if len(h2.Lines) != 2 {
		t.Fatalf("w2 lines = %d, want 2", len(h2.Lines))
	}

	// Workload IDs in response should match.
	if h1.WorkloadID != id1 {
		t.Errorf("h1.workload_id = %q, want %q", h1.WorkloadID, id1)
	}
	if h2.WorkloadID != id2 {
		t.Errorf("h2.workload_id = %q, want %q", h2.WorkloadID, id2)
	}
}
