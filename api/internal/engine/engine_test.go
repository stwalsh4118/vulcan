package engine_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/engine"
	"github.com/seantiz/vulcan/internal/model"
	"github.com/seantiz/vulcan/internal/store"
)

// delayBackend is a configurable mock backend for engine tests.
type delayBackend struct {
	delay  time.Duration
	output []byte
	err    error
}

func (d *delayBackend) Execute(ctx context.Context, _ backend.WorkloadSpec) (backend.WorkloadResult, error) {
	select {
	case <-time.After(d.delay):
	case <-ctx.Done():
		return backend.WorkloadResult{}, ctx.Err()
	}
	if d.err != nil {
		return backend.WorkloadResult{}, d.err
	}
	return backend.WorkloadResult{
		ExitCode: 0,
		Output:   d.output,
	}, nil
}

func (d *delayBackend) Capabilities() backend.BackendCapabilities {
	return backend.BackendCapabilities{
		Name:                "delay",
		SupportedRuntimes:   []string{model.RuntimeNode},
		SupportedIsolations: []string{model.IsolationIsolate},
		MaxConcurrency:      10,
	}
}

func (d *delayBackend) Cleanup(_ context.Context, _ string) error { return nil }

func newTestEngine(t *testing.T, b backend.Backend) (*engine.Engine, store.Store) {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	reg := backend.NewRegistry()
	reg.Register(model.IsolationIsolate, b)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	eng := engine.NewEngine(s, reg, logger)
	return eng, s
}

func makeAsyncWorkload() *model.Workload {
	timeout := 10
	return &model.Workload{
		ID:        model.NewID(),
		Status:    model.StatusPending,
		Isolation: model.IsolationIsolate,
		Runtime:   model.RuntimeNode,
		TimeoutS:  &timeout,
		CreatedAt: time.Now().UTC(),
	}
}

// waitForStatus polls the store until the workload reaches the expected status.
func waitForStatus(t *testing.T, s store.Store, id, expected string, timeout time.Duration) *model.Workload {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		w, err := s.GetWorkload(context.Background(), id)
		if err != nil {
			t.Fatalf("GetWorkload: %v", err)
		}
		if w.Status == expected {
			return w
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("workload %s did not reach status %q within %v", id, expected, timeout)
	return nil
}

func TestSubmitHappyPath(t *testing.T) {
	b := &delayBackend{delay: 10 * time.Millisecond, output: []byte("hello")}
	eng, s := newTestEngine(t, b)

	w := makeAsyncWorkload()
	if err := eng.Submit(context.Background(), w); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Should be pending immediately.
	got, _ := s.GetWorkload(context.Background(), w.ID)
	if got.Status != model.StatusPending {
		t.Errorf("initial status = %q, want pending", got.Status)
	}

	// Wait for completion.
	completed := waitForStatus(t, s, w.ID, model.StatusCompleted, 5*time.Second)
	if string(completed.Output) != "hello" {
		t.Errorf("output = %q, want %q", string(completed.Output), "hello")
	}
	if completed.ExitCode == nil || *completed.ExitCode != 0 {
		t.Errorf("exit code = %v, want 0", completed.ExitCode)
	}
	if completed.DurationMS == nil || *completed.DurationMS <= 0 {
		t.Errorf("duration_ms = %v, want > 0", completed.DurationMS)
	}
	if completed.StartedAt == nil {
		t.Error("started_at is nil")
	}
	if completed.FinishedAt == nil {
		t.Error("finished_at is nil")
	}
}

func TestSubmitBackendError(t *testing.T) {
	b := &delayBackend{err: errors.New("backend crash")}
	eng, s := newTestEngine(t, b)

	w := makeAsyncWorkload()
	if err := eng.Submit(context.Background(), w); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	failed := waitForStatus(t, s, w.ID, model.StatusFailed, 5*time.Second)
	if failed.Error == "" {
		t.Error("expected error message, got empty")
	}
}

func TestSubmitTimeout(t *testing.T) {
	b := &delayBackend{delay: 5 * time.Second} // will exceed timeout
	eng, s := newTestEngine(t, b)

	w := makeAsyncWorkload()
	timeout := 1
	w.TimeoutS = &timeout
	if err := eng.Submit(context.Background(), w); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	failed := waitForStatus(t, s, w.ID, model.StatusFailed, 5*time.Second)
	if failed.Error == "" {
		t.Error("expected timeout error message")
	}
}

func TestSubmitDefaultTimeout(t *testing.T) {
	b := &delayBackend{delay: 10 * time.Millisecond, output: []byte("ok")}
	eng, s := newTestEngine(t, b)

	w := makeAsyncWorkload()
	w.TimeoutS = nil // should use default 30s

	if err := eng.Submit(context.Background(), w); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	completed := waitForStatus(t, s, w.ID, model.StatusCompleted, 5*time.Second)
	if string(completed.Output) != "ok" {
		t.Errorf("output = %q, want %q", string(completed.Output), "ok")
	}
}

func TestSubmitUnresolvableBackend(t *testing.T) {
	b := &delayBackend{delay: 10 * time.Millisecond, output: []byte("ok")}
	eng, s := newTestEngine(t, b)

	w := makeAsyncWorkload()
	w.Isolation = "nonexistent" // no backend registered for this isolation
	if err := eng.Submit(context.Background(), w); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	failed := waitForStatus(t, s, w.ID, model.StatusFailed, 5*time.Second)
	if failed.Error == "" {
		t.Error("expected resolve backend error message, got empty")
	}
	if failed.StartedAt == nil {
		t.Error("started_at should be set even when backend resolution fails after running transition")
	}
}

// loggingBackend calls LogWriter with configured lines during execution.
type loggingBackend struct {
	lines []string
}

func (lb *loggingBackend) Execute(_ context.Context, spec backend.WorkloadSpec) (backend.WorkloadResult, error) {
	for _, line := range lb.lines {
		if spec.LogWriter != nil {
			spec.LogWriter(line)
		}
	}
	return backend.WorkloadResult{ExitCode: 0, Output: []byte("done"), LogLines: lb.lines}, nil
}

func (lb *loggingBackend) Capabilities() backend.BackendCapabilities {
	return backend.BackendCapabilities{
		Name:                "logging",
		SupportedRuntimes:   []string{model.RuntimeNode},
		SupportedIsolations: []string{model.IsolationIsolate},
		MaxConcurrency:      10,
	}
}

func (lb *loggingBackend) Cleanup(_ context.Context, _ string) error { return nil }

func TestDualWriteLogPersistence(t *testing.T) {
	expectedLines := []string{"hello", "world", "done"}
	b := &loggingBackend{lines: expectedLines}
	eng, s := newTestEngine(t, b)

	w := makeAsyncWorkload()
	if err := eng.Submit(context.Background(), w); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	waitForStatus(t, s, w.ID, model.StatusCompleted, 5*time.Second)

	// Verify log lines were persisted to the store.
	lines, err := s.GetLogLines(context.Background(), w.ID)
	if err != nil {
		t.Fatalf("GetLogLines: %v", err)
	}
	if len(lines) != len(expectedLines) {
		t.Fatalf("len(lines) = %d, want %d", len(lines), len(expectedLines))
	}
	for i, l := range lines {
		if l.Seq != i {
			t.Errorf("lines[%d].Seq = %d, want %d", i, l.Seq, i)
		}
		if l.Line != expectedLines[i] {
			t.Errorf("lines[%d].Line = %q, want %q", i, l.Line, expectedLines[i])
		}
	}
}

func TestDualWriteSSEUnaffected(t *testing.T) {
	expectedLines := []string{"log line 1", "log line 2"}
	b := &loggingBackend{lines: expectedLines}
	eng, s := newTestEngine(t, b)

	w := makeAsyncWorkload()

	// Subscribe to log broker before submission.
	ch, unsub := eng.Broker().Subscribe(w.ID)
	defer unsub()

	if err := eng.Submit(context.Background(), w); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Collect SSE lines from broker.
	var received []string
	for line := range ch {
		received = append(received, line)
	}

	if len(received) != len(expectedLines) {
		t.Fatalf("SSE lines = %d, want %d", len(received), len(expectedLines))
	}
	for i, line := range received {
		if line != expectedLines[i] {
			t.Errorf("SSE line[%d] = %q, want %q", i, line, expectedLines[i])
		}
	}

	// Also verify persistence worked.
	waitForStatus(t, s, w.ID, model.StatusCompleted, 5*time.Second)
	lines, _ := s.GetLogLines(context.Background(), w.ID)
	if len(lines) != len(expectedLines) {
		t.Errorf("persisted lines = %d, want %d", len(lines), len(expectedLines))
	}
}

func TestSubmitConcurrent(t *testing.T) {
	b := &delayBackend{delay: 50 * time.Millisecond, output: []byte("done")}
	eng, s := newTestEngine(t, b)

	ids := make([]string, 5)
	for i := range ids {
		w := makeAsyncWorkload()
		ids[i] = w.ID
		if err := eng.Submit(context.Background(), w); err != nil {
			t.Fatalf("Submit[%d]: %v", i, err)
		}
	}

	// Wait for all to complete.
	for _, id := range ids {
		waitForStatus(t, s, id, model.StatusCompleted, 5*time.Second)
	}
}
