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

// pbi10Server sets up a full-stack test server for PBI-10 SSE tests.
type pbi10Server struct {
	ts    *httptest.Server
	eng   *engine.Engine
	store *store.SQLiteStore
	stub  *stubBackend
}

func newPBI10Server(t *testing.T, logLines []string, delay time.Duration) *pbi10Server {
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
		delay:     delay,
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

	return &pbi10Server{ts: ts, eng: eng, store: s, stub: stub}
}

func (p *pbi10Server) url() string { return p.ts.URL }

func (p *pbi10Server) postAsync(t *testing.T, body string) map[string]any {
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

func (p *pbi10Server) pollStatus(t *testing.T, id, expected string, timeout time.Duration) {
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

// sseEvent represents a parsed SSE event with optional named type.
type sseEvent struct {
	Type string
	Data string
}

// readSSEEvents reads all SSE events from the response body, properly parsing
// named events (event: <type>) and data lines.
func readSSEEvents(t *testing.T, resp *http.Response) []sseEvent {
	t.Helper()
	scanner := bufio.NewScanner(resp.Body)
	var events []sseEvent
	var currentType string
	var currentData []string
	for scanner.Scan() {
		line := scanner.Text()
		if et, ok := strings.CutPrefix(line, "event: "); ok {
			currentType = et
		} else if data, ok := strings.CutPrefix(line, "data: "); ok {
			currentData = append(currentData, data)
		} else if line == "" && len(currentData) > 0 {
			events = append(events, sseEvent{Type: currentType, Data: strings.Join(currentData, "\n")})
			currentType = ""
			currentData = nil
		}
	}
	if len(currentData) > 0 {
		events = append(events, sseEvent{Type: currentType, Data: strings.Join(currentData, "\n")})
	}
	return events
}

// --- AC2: Done event sent on workload completion ---

func TestPBI10_AC2_DoneEventSentOnCompletion(t *testing.T) {
	logLines := []string{"line 1", "line 2", "line 3"}
	p := newPBI10Server(t, logLines, 200*time.Millisecond)

	result := p.postAsync(t, `{"runtime":"node"}`)
	id := result["id"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", p.url()+"/v1/workloads/"+id+"/logs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET logs: %v", err)
	}
	defer resp.Body.Close()

	events := readSSEEvents(t, resp)

	// Should have 3 log events + 1 done event.
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4: %v", len(events), events)
	}

	// Verify log lines are unnamed events.
	for i, logLine := range logLines {
		if events[i].Type != "" {
			t.Errorf("event[%d].Type = %q, want unnamed", i, events[i].Type)
		}
		if events[i].Data != logLine {
			t.Errorf("event[%d].Data = %q, want %q", i, events[i].Data, logLine)
		}
	}

	// Verify the final event is the done event.
	last := events[len(events)-1]
	if last.Type != "done" {
		t.Errorf("last event type = %q, want %q", last.Type, "done")
	}
	if last.Data != "stream complete" {
		t.Errorf("last event data = %q, want %q", last.Data, "stream complete")
	}
}

// --- AC2: Done event format matches SSE spec ---

func TestPBI10_AC2_DoneEventFormat(t *testing.T) {
	p := newPBI10Server(t, []string{"hello"}, 200*time.Millisecond)

	result := p.postAsync(t, `{"runtime":"node"}`)
	id := result["id"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", p.url()+"/v1/workloads/"+id+"/logs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET logs: %v", err)
	}
	defer resp.Body.Close()

	// Read raw bytes to verify exact format.
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(raw)

	// The done event must be present in the raw SSE format.
	expected := "event: done\ndata: stream complete\n\n"
	if !strings.Contains(body, expected) {
		t.Errorf("response body does not contain done event in expected format\ngot:\n%s", body)
	}
}

// --- Terminal workload: no done event ---

func TestPBI10_TerminalWorkloadNoDoneEvent(t *testing.T) {
	p := newPBI10Server(t, []string{"line"}, 50*time.Millisecond)

	result := p.postAsync(t, `{"runtime":"node"}`)
	id := result["id"].(string)

	// Wait for workload to complete first.
	p.pollStatus(t, id, "completed", 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", p.url()+"/v1/workloads/"+id+"/logs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// For terminal workloads, stream should be empty (no events, no done event).
	events := readSSEEvents(t, resp)
	if len(events) != 0 {
		t.Errorf("got %d events for terminal workload, want 0: %v", len(events), events)
	}
}

// --- Incremental delivery: lines arrive before workload completes ---

func TestPBI10_IncrementalDelivery(t *testing.T) {
	logLines := []string{"first", "second", "third"}
	p := newPBI10Server(t, logLines, 200*time.Millisecond)
	// Add per-line delay so lines are emitted over time, not all at once.
	p.stub.logLineDelay = 200 * time.Millisecond

	result := p.postAsync(t, `{"runtime":"node"}`)
	id := result["id"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", p.url()+"/v1/workloads/"+id+"/logs", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET logs: %v", err)
	}
	defer resp.Body.Close()

	// Read events incrementally and verify the first log line arrives while
	// the workload is still running (i.e., before completion).
	scanner := bufio.NewScanner(resp.Body)
	firstLogReceived := false
	var currentType string
	var currentData []string

	for scanner.Scan() {
		line := scanner.Text()
		if et, ok := strings.CutPrefix(line, "event: "); ok {
			currentType = et
		} else if data, ok := strings.CutPrefix(line, "data: "); ok {
			currentData = append(currentData, data)
		} else if line == "" && len(currentData) > 0 {
			if currentType == "" && !firstLogReceived {
				// First unnamed event = first log line. Check workload status.
				firstLogReceived = true
				statusResp, err := http.Get(p.url() + "/v1/workloads/" + id)
				if err != nil {
					t.Fatalf("GET status: %v", err)
				}
				var wl map[string]any
				if err := json.NewDecoder(statusResp.Body).Decode(&wl); err != nil {
					statusResp.Body.Close()
					t.Fatalf("decode: %v", err)
				}
				statusResp.Body.Close()
				if wl["status"] != "running" {
					t.Errorf("workload status when first log received = %q, want %q", wl["status"], "running")
				}
			}
			currentType = ""
			currentData = nil
		}
	}

	if !firstLogReceived {
		t.Fatal("no log lines received from SSE stream")
	}
}

// --- Historical log fallback still works ---

func TestPBI10_HistoricalLogsFallback(t *testing.T) {
	logLines := []string{"alpha", "beta", "gamma"}
	p := newPBI10Server(t, logLines, 50*time.Millisecond)

	result := p.postAsync(t, `{"runtime":"node"}`)
	id := result["id"].(string)

	p.pollStatus(t, id, "completed", 5*time.Second)

	// Fetch historical logs via the history endpoint.
	resp, err := http.Get(p.url() + "/v1/workloads/" + id + "/logs/history")
	if err != nil {
		t.Fatalf("GET history: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("history status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		WorkloadID string `json:"workload_id"`
		Lines      []struct {
			Seq  int    `json:"seq"`
			Line string `json:"line"`
		} `json:"lines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(body.Lines) != len(logLines) {
		t.Fatalf("history has %d lines, want %d", len(body.Lines), len(logLines))
	}
	for i, l := range body.Lines {
		if l.Line != logLines[i] {
			t.Errorf("history[%d] = %q, want %q", i, l.Line, logLines[i])
		}
	}
}
