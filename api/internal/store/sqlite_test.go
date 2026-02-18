package store

import (
	"context"
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

	err := s.UpdateWorkloadStatus(ctx, "nonexistent", model.StatusKilled)
	if err != ErrNotFound {
		t.Errorf("UpdateWorkloadStatus error = %v, want ErrNotFound", err)
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
