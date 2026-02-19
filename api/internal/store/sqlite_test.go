package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/seantiz/vulcan/internal/model"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func makeTestWorkload() *model.Workload {
	cpus := 1
	mem := 128
	timeout := 30
	return &model.Workload{
		ID:        model.NewID(),
		Status:    model.StatusPending,
		Isolation: model.IsolationIsolate,
		Runtime:   model.RuntimeNode,
		NodeID:    "test-node",
		CPULimit:  &cpus,
		MemLimit:  &mem,
		TimeoutS:  &timeout,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func TestCreateAndGetWorkload(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()

	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	got, err := s.GetWorkload(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetWorkload: %v", err)
	}

	if got.ID != w.ID {
		t.Errorf("ID = %q, want %q", got.ID, w.ID)
	}
	if got.Status != w.Status {
		t.Errorf("Status = %q, want %q", got.Status, w.Status)
	}
	if got.Isolation != w.Isolation {
		t.Errorf("Isolation = %q, want %q", got.Isolation, w.Isolation)
	}
	if got.Runtime != w.Runtime {
		t.Errorf("Runtime = %q, want %q", got.Runtime, w.Runtime)
	}
	if got.NodeID != w.NodeID {
		t.Errorf("NodeID = %q, want %q", got.NodeID, w.NodeID)
	}
	if *got.CPULimit != *w.CPULimit {
		t.Errorf("CPULimit = %d, want %d", *got.CPULimit, *w.CPULimit)
	}
}

func TestGetWorkloadNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.GetWorkload(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("GetWorkload error = %v, want ErrNotFound", err)
	}
}

func TestListWorkloadsPagination(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert 5 workloads with staggered creation times.
	for i := 0; i < 5; i++ {
		w := makeTestWorkload()
		w.CreatedAt = time.Now().UTC().Add(time.Duration(i) * time.Second).Truncate(time.Second)
		if err := s.CreateWorkload(ctx, w); err != nil {
			t.Fatalf("CreateWorkload[%d]: %v", i, err)
		}
	}

	// Get first page of 2.
	workloads, total, err := s.ListWorkloads(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(workloads) != 2 {
		t.Errorf("len(workloads) = %d, want 2", len(workloads))
	}

	// Get second page of 2.
	workloads2, total2, err := s.ListWorkloads(ctx, 2, 2)
	if err != nil {
		t.Fatalf("ListWorkloads page 2: %v", err)
	}
	if total2 != 5 {
		t.Errorf("total page 2 = %d, want 5", total2)
	}
	if len(workloads2) != 2 {
		t.Errorf("len(workloads) page 2 = %d, want 2", len(workloads2))
	}
}

func TestListWorkloadsOrdering(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert workloads with ascending created_at.
	for i := 0; i < 3; i++ {
		w := makeTestWorkload()
		w.CreatedAt = time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC)
		if err := s.CreateWorkload(ctx, w); err != nil {
			t.Fatalf("CreateWorkload[%d]: %v", i, err)
		}
	}

	workloads, _, err := s.ListWorkloads(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}

	// Should be ordered DESC — newest first.
	for i := 1; i < len(workloads); i++ {
		if workloads[i].CreatedAt.After(workloads[i-1].CreatedAt) {
			t.Errorf("workloads not in DESC order: [%d].CreatedAt=%v > [%d].CreatedAt=%v",
				i, workloads[i].CreatedAt, i-1, workloads[i-1].CreatedAt)
		}
	}
}

func TestListWorkloadsEmpty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	workloads, total, err := s.ListWorkloads(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if workloads != nil {
		t.Errorf("workloads = %v, want nil", workloads)
	}
}

func TestUpdateWorkloadStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()

	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	if err := s.UpdateWorkloadStatus(ctx, w.ID, model.StatusRunning); err != nil {
		t.Fatalf("UpdateWorkloadStatus: %v", err)
	}

	got, _ := s.GetWorkload(ctx, w.ID)
	if got.Status != model.StatusRunning {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusRunning)
	}
}

func TestUpdateWorkloadStatusKilledSetsFinishedAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()

	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	if err := s.UpdateWorkloadStatus(ctx, w.ID, model.StatusKilled); err != nil {
		t.Fatalf("UpdateWorkloadStatus: %v", err)
	}

	got, _ := s.GetWorkload(ctx, w.ID)
	if got.Status != model.StatusKilled {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusKilled)
	}
	if got.FinishedAt == nil {
		t.Error("FinishedAt is nil, expected it to be set for killed status")
	}
}

func TestUpdateWorkloadStatusNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.UpdateWorkloadStatus(ctx, "nonexistent", model.StatusRunning)
	if err != ErrNotFound {
		t.Errorf("UpdateWorkloadStatus error = %v, want ErrNotFound", err)
	}
}

func TestUpdateWorkloadStatusValidLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()

	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	// pending → running
	if err := s.UpdateWorkloadStatus(ctx, w.ID, model.StatusRunning); err != nil {
		t.Fatalf("pending→running: %v", err)
	}
	got, _ := s.GetWorkload(ctx, w.ID)
	if got.Status != model.StatusRunning {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusRunning)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt is nil, expected it to be set for running status")
	}

	// running → completed
	if err := s.UpdateWorkloadStatus(ctx, w.ID, model.StatusCompleted); err != nil {
		t.Fatalf("running→completed: %v", err)
	}
	got, _ = s.GetWorkload(ctx, w.ID)
	if got.Status != model.StatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusCompleted)
	}
	if got.FinishedAt == nil {
		t.Error("FinishedAt is nil, expected it to be set for completed status")
	}
}

func TestUpdateWorkloadStatusInvalidTransition(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		from, to string
	}{
		{"pending→completed", model.StatusPending, model.StatusCompleted},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := makeTestWorkload()
			w.Status = tc.from
			if err := s.CreateWorkload(ctx, w); err != nil {
				t.Fatalf("CreateWorkload: %v", err)
			}

			err := s.UpdateWorkloadStatus(ctx, w.ID, tc.to)
			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("got error %v, want ErrInvalidTransition", err)
			}
		})
	}
}

func TestUpdateWorkloadStatusTerminalCannotTransition(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()

	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	// Move to running, then completed (terminal).
	if err := s.UpdateWorkloadStatus(ctx, w.ID, model.StatusRunning); err != nil {
		t.Fatalf("pending→running: %v", err)
	}
	if err := s.UpdateWorkloadStatus(ctx, w.ID, model.StatusCompleted); err != nil {
		t.Fatalf("running→completed: %v", err)
	}

	// completed → killed should fail
	err := s.UpdateWorkloadStatus(ctx, w.ID, model.StatusKilled)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("completed→killed: got error %v, want ErrInvalidTransition", err)
	}
}

func TestUpdateWorkload(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()

	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	// Transition to running, then update all mutable fields.
	now := time.Now().UTC()
	exitCode := 0
	durationMS := 150
	w.Status = model.StatusRunning
	w.StartedAt = &now
	if err := s.UpdateWorkload(ctx, w); err != nil {
		t.Fatalf("UpdateWorkload (running): %v", err)
	}

	w.Status = model.StatusCompleted
	w.Output = []byte("hello world")
	w.ExitCode = &exitCode
	w.Error = ""
	w.DurationMS = &durationMS
	finishedAt := now.Add(time.Duration(durationMS) * time.Millisecond)
	w.FinishedAt = &finishedAt

	if err := s.UpdateWorkload(ctx, w); err != nil {
		t.Fatalf("UpdateWorkload (completed): %v", err)
	}

	got, err := s.GetWorkload(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetWorkload: %v", err)
	}
	if got.Status != model.StatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, model.StatusCompleted)
	}
	if string(got.Output) != "hello world" {
		t.Errorf("Output = %q, want %q", string(got.Output), "hello world")
	}
	if *got.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", *got.ExitCode)
	}
	if *got.DurationMS != 150 {
		t.Errorf("DurationMS = %d, want 150", *got.DurationMS)
	}
	if got.FinishedAt == nil {
		t.Error("FinishedAt is nil")
	}
}

func TestUpdateWorkloadNotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	w := makeTestWorkload()
	w.ID = "nonexistent"
	err := s.UpdateWorkload(ctx, w)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestUpdateWorkloadInvalidTransition(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()

	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	// pending → completed is invalid
	w.Status = model.StatusCompleted
	err := s.UpdateWorkload(ctx, w)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("got error %v, want ErrInvalidTransition", err)
	}
}

func TestGetWorkloadStats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create workloads in various states.
	for i := 0; i < 3; i++ {
		w := makeTestWorkload()
		w.Isolation = model.IsolationIsolate
		if err := s.CreateWorkload(ctx, w); err != nil {
			t.Fatalf("CreateWorkload: %v", err)
		}
		// Move first two to completed with a duration.
		if i < 2 {
			if err := s.UpdateWorkloadStatus(ctx, w.ID, model.StatusRunning); err != nil {
				t.Fatalf("UpdateWorkloadStatus running: %v", err)
			}
			if err := s.UpdateWorkloadStatus(ctx, w.ID, model.StatusCompleted); err != nil {
				t.Fatalf("UpdateWorkloadStatus completed: %v", err)
			}
			dur := 100 + i*100 // 100, 200
			now := time.Now().UTC()
			w.Status = model.StatusCompleted
			w.DurationMS = &dur
			w.FinishedAt = &now
			w.StartedAt = &now
			if _, err := s.db.ExecContext(ctx,
				"UPDATE workloads SET duration_ms = ? WHERE id = ?", dur, w.ID); err != nil {
				t.Fatalf("set duration: %v", err)
			}
		}
	}

	// Add a microvm workload.
	w := makeTestWorkload()
	w.Isolation = model.IsolationMicroVM
	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload (microvm): %v", err)
	}

	stats, err := s.GetWorkloadStats(ctx)
	if err != nil {
		t.Fatalf("GetWorkloadStats: %v", err)
	}

	if stats.Total != 4 {
		t.Errorf("Total = %d, want 4", stats.Total)
	}
	if stats.CountByStatus[model.StatusCompleted] != 2 {
		t.Errorf("completed count = %d, want 2", stats.CountByStatus[model.StatusCompleted])
	}
	if stats.CountByStatus[model.StatusPending] != 2 {
		t.Errorf("pending count = %d, want 2", stats.CountByStatus[model.StatusPending])
	}
	if stats.CountByIsolation[model.IsolationIsolate] != 3 {
		t.Errorf("isolate count = %d, want 3", stats.CountByIsolation[model.IsolationIsolate])
	}
	if stats.CountByIsolation[model.IsolationMicroVM] != 1 {
		t.Errorf("microvm count = %d, want 1", stats.CountByIsolation[model.IsolationMicroVM])
	}
	if stats.AvgDurationMS != 150 {
		t.Errorf("AvgDurationMS = %f, want 150", stats.AvgDurationMS)
	}
}

func TestGetWorkloadStatsEmpty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	stats, err := s.GetWorkloadStats(ctx)
	if err != nil {
		t.Fatalf("GetWorkloadStats: %v", err)
	}
	if stats.Total != 0 {
		t.Errorf("Total = %d, want 0", stats.Total)
	}
	if stats.AvgDurationMS != 0 {
		t.Errorf("AvgDurationMS = %f, want 0", stats.AvgDurationMS)
	}
}

func TestInsertAndGetLogLines(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()
	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	// Insert three log lines.
	for i := 0; i < 3; i++ {
		if err := s.InsertLogLine(ctx, w.ID, i, fmt.Sprintf("line %d", i)); err != nil {
			t.Fatalf("InsertLogLine[%d]: %v", i, err)
		}
	}

	lines, err := s.GetLogLines(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetLogLines: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("len(lines) = %d, want 3", len(lines))
	}

	for i, l := range lines {
		if l.Seq != i {
			t.Errorf("lines[%d].Seq = %d, want %d", i, l.Seq, i)
		}
		want := fmt.Sprintf("line %d", i)
		if l.Line != want {
			t.Errorf("lines[%d].Line = %q, want %q", i, l.Line, want)
		}
		if l.WorkloadID != w.ID {
			t.Errorf("lines[%d].WorkloadID = %q, want %q", i, l.WorkloadID, w.ID)
		}
		if l.ID == 0 {
			t.Errorf("lines[%d].ID = 0, expected non-zero auto-increment ID", i)
		}
	}
}

func TestGetLogLinesOrdering(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()
	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	// Insert lines out of order.
	for _, seq := range []int{2, 0, 1} {
		if err := s.InsertLogLine(ctx, w.ID, seq, fmt.Sprintf("line %d", seq)); err != nil {
			t.Fatalf("InsertLogLine[%d]: %v", seq, err)
		}
	}

	lines, err := s.GetLogLines(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetLogLines: %v", err)
	}

	// Should be ordered by seq ASC regardless of insertion order.
	for i := 0; i < len(lines)-1; i++ {
		if lines[i].Seq >= lines[i+1].Seq {
			t.Errorf("lines not ordered by seq: lines[%d].Seq=%d >= lines[%d].Seq=%d",
				i, lines[i].Seq, i+1, lines[i+1].Seq)
		}
	}
}

func TestGetLogLinesEmpty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	w := makeTestWorkload()
	if err := s.CreateWorkload(ctx, w); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}

	lines, err := s.GetLogLines(ctx, w.ID)
	if err != nil {
		t.Fatalf("GetLogLines: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("len(lines) = %d, want 0", len(lines))
	}
	if lines == nil {
		t.Error("lines is nil, expected empty slice")
	}
}

func TestGetLogLinesIsolation(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	w1 := makeTestWorkload()
	w2 := makeTestWorkload()
	if err := s.CreateWorkload(ctx, w1); err != nil {
		t.Fatalf("CreateWorkload w1: %v", err)
	}
	if err := s.CreateWorkload(ctx, w2); err != nil {
		t.Fatalf("CreateWorkload w2: %v", err)
	}

	// Insert lines for both workloads.
	if err := s.InsertLogLine(ctx, w1.ID, 0, "w1 line"); err != nil {
		t.Fatalf("InsertLogLine w1: %v", err)
	}
	if err := s.InsertLogLine(ctx, w2.ID, 0, "w2 line"); err != nil {
		t.Fatalf("InsertLogLine w2: %v", err)
	}

	lines1, err := s.GetLogLines(ctx, w1.ID)
	if err != nil {
		t.Fatalf("GetLogLines w1: %v", err)
	}
	if len(lines1) != 1 {
		t.Fatalf("w1 len(lines) = %d, want 1", len(lines1))
	}
	if lines1[0].Line != "w1 line" {
		t.Errorf("w1 line = %q, want %q", lines1[0].Line, "w1 line")
	}

	lines2, err := s.GetLogLines(ctx, w2.ID)
	if err != nil {
		t.Fatalf("GetLogLines w2: %v", err)
	}
	if len(lines2) != 1 {
		t.Fatalf("w2 len(lines) = %d, want 1", len(lines2))
	}
	if lines2[0].Line != "w2 line" {
		t.Errorf("w2 line = %q, want %q", lines2[0].Line, "w2 line")
	}
}

func TestMigrationIdempotency(t *testing.T) {
	// Opening the store twice on the same DB shouldn't error.
	s1, err := NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("First open: %v", err)
	}

	// The in-memory DB won't persist between opens, but we can verify
	// the CREATE TABLE IF NOT EXISTS works by calling it on the same connection.
	if _, err := s1.db.Exec(createWorkloadsTable); err != nil {
		t.Fatalf("Second migration: %v", err)
	}
	s1.Close()
}
