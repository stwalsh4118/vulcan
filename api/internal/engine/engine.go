package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/model"
	"github.com/seantiz/vulcan/internal/store"
)

// DefaultTimeoutS is the default timeout in seconds when none is specified.
const DefaultTimeoutS = 30

// Engine orchestrates asynchronous workload execution.
type Engine struct {
	store    store.Store
	registry *backend.Registry
	logger   *slog.Logger
	wg       sync.WaitGroup
	broker   *LogBroker
}

// NewEngine creates a new execution engine.
func NewEngine(s store.Store, reg *backend.Registry, logger *slog.Logger) *Engine {
	return &Engine{
		store:    s,
		registry: reg,
		logger:   logger,
		broker:   NewLogBroker(),
	}
}

// Broker returns the engine's log broker for SSE subscription.
func (e *Engine) Broker() *LogBroker {
	return e.broker
}

// Submit creates a workload record and launches asynchronous execution in a
// goroutine. The workload is stored with status "pending" before returning.
// The goroutine operates on a copy of the workload to avoid data races with
// the caller.
func (e *Engine) Submit(ctx context.Context, w *model.Workload) error {
	if err := e.store.CreateWorkload(ctx, w); err != nil {
		return fmt.Errorf("create workload: %w", err)
	}

	wCopy := *w
	e.wg.Go(func() {
		e.execute(&wCopy)
	})

	return nil
}

// Wait blocks until all in-flight workload goroutines complete.
func (e *Engine) Wait() {
	e.wg.Wait()
}

// execute runs the workload lifecycle in a goroutine: pending→running→completed/failed.
func (e *Engine) execute(w *model.Workload) {
	// Close the log stream when execution finishes, regardless of outcome.
	defer e.broker.Close(w.ID)

	// Transition to running.
	if err := e.store.UpdateWorkloadStatus(context.Background(), w.ID, model.StatusRunning); err != nil {
		e.logger.Error("failed to transition to running", "workload_id", w.ID, "error", err)
		e.finishFailed(w.ID, nil, fmt.Sprintf("failed to start: %v", err))
		return
	}

	// Capture start time immediately after the running transition so that
	// started_at stays consistent across success, failure, and resolve-error paths.
	start := time.Now()

	// Determine timeout.
	timeoutS := DefaultTimeoutS
	if w.TimeoutS != nil && *w.TimeoutS > 0 {
		timeoutS = *w.TimeoutS
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutS)*time.Second)
	defer cancel()

	// Build the workload spec. The LogWriter dual-writes: persist to SQLite
	// for historical viewing, then publish to LogBroker for real-time SSE.
	var seq atomic.Int32
	spec := backend.WorkloadSpec{
		ID:          w.ID,
		Runtime:     w.Runtime,
		Isolation:   w.Isolation,
		Code:        w.Code,
		CodeArchive: w.CodeArchive,
		TimeoutS:    timeoutS,
		LogWriter: func(line string) {
			currentSeq := int(seq.Add(1) - 1)
			if err := e.store.InsertLogLine(ctx, w.ID, currentSeq, line); err != nil {
				e.logger.Error("failed to persist log line", "workload_id", w.ID, "seq", currentSeq, "error", err)
			}
			e.broker.Publish(w.ID, line)
		},
	}
	if w.CPULimit != nil {
		spec.CPULimit = *w.CPULimit
	}
	if w.MemLimit != nil {
		spec.MemLimitMB = *w.MemLimit
	}

	// Resolve backend.
	b, err := e.registry.Resolve(w.Isolation, w.Runtime)
	if err != nil {
		e.finishFailed(w.ID, &start, fmt.Sprintf("resolve backend: %v", err))
		return
	}

	result, err := b.Execute(ctx, spec)
	durationMS := int(time.Since(start).Milliseconds())

	if err != nil {
		errMsg := err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			errMsg = fmt.Sprintf("workload timed out after %ds", timeoutS)
		}
		e.finishFailed(w.ID, &start, errMsg)
		return
	}

	// Success — update to completed. Use backend-reported duration if available,
	// otherwise use wall-clock measurement.
	now := time.Now().UTC()
	dur := durationMS
	if result.DurationMS > 0 {
		dur = result.DurationMS
	}

	completed := &model.Workload{
		ID:         w.ID,
		Status:     model.StatusCompleted,
		Output:     result.Output,
		ExitCode:   &result.ExitCode,
		Error:      result.Error,
		DurationMS: &dur,
		StartedAt:  &start,
		FinishedAt: &now,
	}

	if err := e.store.UpdateWorkload(context.Background(), completed); err != nil {
		e.logger.Error("failed to update completed workload", "workload_id", w.ID, "error", err)
	}
}

// finishFailed marks a workload as failed with the given error message.
// startedAt may be nil if execution never started.
func (e *Engine) finishFailed(id string, startedAt *time.Time, errMsg string) {
	now := time.Now().UTC()
	var durationMS int
	if startedAt != nil {
		durationMS = int(time.Since(*startedAt).Milliseconds())
	}

	w := &model.Workload{
		ID:         id,
		Status:     model.StatusFailed,
		Error:      errMsg,
		DurationMS: &durationMS,
		StartedAt:  startedAt,
		FinishedAt: &now,
	}

	if err := e.store.UpdateWorkload(context.Background(), w); err != nil {
		e.logger.Error("failed to update failed workload", "workload_id", id, "error", err)
	}
}
