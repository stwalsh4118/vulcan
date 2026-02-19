package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/seantiz/vulcan/internal/model"
)

func TestStreamLogsNotFound(t *testing.T) {
	srv := newTestServer(t)

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/workloads/nonexistent/logs")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestStreamLogsCompletedWorkload(t *testing.T) {
	srv := newTestServer(t)

	// Create a workload and move it to completed.
	wl := &model.Workload{
		ID:        model.NewID(),
		Status:    model.StatusPending,
		Isolation: "isolate",
		Runtime:   "node",
		CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.CreateWorkload(context.Background(), wl); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}
	if err := srv.store.UpdateWorkloadStatus(context.Background(), wl.ID, model.StatusRunning); err != nil {
		t.Fatalf("pending→running: %v", err)
	}
	if err := srv.store.UpdateWorkloadStatus(context.Background(), wl.ID, model.StatusCompleted); err != nil {
		t.Fatalf("running→completed: %v", err)
	}

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/workloads/" + wl.ID + "/logs")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

func TestStreamLogsReceivesEvents(t *testing.T) {
	srv := newTestServer(t)

	// Create a pending workload.
	wl := &model.Workload{
		ID:        model.NewID(),
		Status:    model.StatusPending,
		Isolation: "isolate",
		Runtime:   "node",
		CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.CreateWorkload(context.Background(), wl); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Start streaming in a goroutine.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", ts.URL+"/v1/workloads/"+wl.ID+"/logs", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Publish some log lines and close the stream.
	broker := srv.engine.Broker()
	broker.Publish(wl.ID, "hello world")
	broker.Publish(wl.ID, "goodbye")
	broker.Close(wl.ID)

	// Read SSE events from the response body.
	scanner := bufio.NewScanner(resp.Body)
	var events []string
	for scanner.Scan() {
		line := scanner.Text()
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			events = append(events, data)
		}
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2: %v", len(events), events)
	}
	if events[0] != "hello world" {
		t.Errorf("event[0] = %q, want %q", events[0], "hello world")
	}
	if events[1] != "goodbye" {
		t.Errorf("event[1] = %q, want %q", events[1], "goodbye")
	}
}

func TestGetLogHistoryHappyPath(t *testing.T) {
	srv := newTestServer(t)

	wl := &model.Workload{
		ID:        model.NewID(),
		Status:    model.StatusPending,
		Isolation: "isolate",
		Runtime:   "node",
		CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.CreateWorkload(context.Background(), wl); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	// Insert some log lines.
	for i := 0; i < 3; i++ {
		if err := srv.store.InsertLogLine(context.Background(), wl.ID, i, fmt.Sprintf("line %d", i)); err != nil {
			t.Fatalf("InsertLogLine[%d]: %v", i, err)
		}
	}

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/workloads/" + wl.ID + "/logs/history")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body logHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.WorkloadID != wl.ID {
		t.Errorf("workload_id = %q, want %q", body.WorkloadID, wl.ID)
	}
	if len(body.Lines) != 3 {
		t.Fatalf("len(lines) = %d, want 3", len(body.Lines))
	}
	for i, l := range body.Lines {
		if l.Seq != i {
			t.Errorf("lines[%d].seq = %d, want %d", i, l.Seq, i)
		}
		want := fmt.Sprintf("line %d", i)
		if l.Line != want {
			t.Errorf("lines[%d].line = %q, want %q", i, l.Line, want)
		}
	}
}

func TestGetLogHistoryEmpty(t *testing.T) {
	srv := newTestServer(t)

	wl := &model.Workload{
		ID:        model.NewID(),
		Status:    model.StatusPending,
		Isolation: "isolate",
		Runtime:   "node",
		CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.CreateWorkload(context.Background(), wl); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/workloads/" + wl.ID + "/logs/history")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var body logHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(body.Lines) != 0 {
		t.Errorf("len(lines) = %d, want 0", len(body.Lines))
	}
	if body.Lines == nil {
		t.Error("lines is nil, expected empty array")
	}
}

func TestGetLogHistoryNotFound(t *testing.T) {
	srv := newTestServer(t)

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/workloads/nonexistent/logs/history")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestStreamLogsMultiLineData(t *testing.T) {
	srv := newTestServer(t)

	wl := &model.Workload{
		ID:        model.NewID(),
		Status:    model.StatusPending,
		Isolation: "isolate",
		Runtime:   "node",
		CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.CreateWorkload(context.Background(), wl); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", ts.URL+"/v1/workloads/"+wl.ID+"/logs", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	// Publish a multi-line log entry (e.g. a stack trace).
	broker := srv.engine.Broker()
	broker.Publish(wl.ID, "error: something failed\n  at main.go:42\n  at handler.go:10")
	broker.Close(wl.ID)

	// Parse SSE events: consecutive "data:" lines form one event, separated by blank lines.
	scanner := bufio.NewScanner(resp.Body)
	var events []string
	var current []string
	for scanner.Scan() {
		line := scanner.Text()
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			current = append(current, data)
		} else if line == "" && len(current) > 0 {
			events = append(events, strings.Join(current, "\n"))
			current = nil
		}
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1: %v", len(events), events)
	}

	want := "error: something failed\n  at main.go:42\n  at handler.go:10"
	if events[0] != want {
		t.Errorf("event = %q, want %q", events[0], want)
	}
}
