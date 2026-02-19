package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/seantiz/vulcan/internal/model"
)

func TestGetStatsEmpty(t *testing.T) {
	srv := newTestServer(t)

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/stats")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var stats statsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Total != 0 {
		t.Errorf("total = %d, want 0", stats.Total)
	}
	if stats.AvgDurationMS != 0 {
		t.Errorf("avg_duration_ms = %f, want 0", stats.AvgDurationMS)
	}
}

func TestGetStatsPopulated(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Create workloads in different states.
	for range 3 {
		w := &model.Workload{
			ID: model.NewID(), Status: model.StatusPending,
			Isolation: model.IsolationIsolate, Runtime: model.RuntimeNode,
			CreatedAt: time.Now().UTC(),
		}
		if err := srv.store.CreateWorkload(ctx, w); err != nil {
			t.Fatalf("CreateWorkload: %v", err)
		}
		// Move to running then completed.
		if err := srv.store.UpdateWorkloadStatus(ctx, w.ID, model.StatusRunning); err != nil {
			t.Fatalf("pending→running: %v", err)
		}
		dur := 100
		completed := &model.Workload{
			ID: w.ID, Status: model.StatusCompleted,
			DurationMS: &dur, StartedAt: ptrTime(time.Now()), FinishedAt: ptrTime(time.Now()),
		}
		if err := srv.store.UpdateWorkload(ctx, completed); err != nil {
			t.Fatalf("UpdateWorkload: %v", err)
		}
	}

	// One failed workload.
	fw := &model.Workload{
		ID: model.NewID(), Status: model.StatusPending,
		Isolation: model.IsolationMicroVM, Runtime: model.RuntimePython,
		CreatedAt: time.Now().UTC(),
	}
	if err := srv.store.CreateWorkload(ctx, fw); err != nil {
		t.Fatalf("CreateWorkload: %v", err)
	}
	if err := srv.store.UpdateWorkloadStatus(ctx, fw.ID, model.StatusFailed); err != nil {
		t.Fatalf("pending→failed: %v", err)
	}

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/stats")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var stats statsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Total != 4 {
		t.Errorf("total = %d, want 4", stats.Total)
	}
	if stats.ByStatus["completed"] != 3 {
		t.Errorf("by_status[completed] = %d, want 3", stats.ByStatus["completed"])
	}
	if stats.ByStatus["failed"] != 1 {
		t.Errorf("by_status[failed] = %d, want 1", stats.ByStatus["failed"])
	}
	if stats.ByIsolation[model.IsolationIsolate] != 3 {
		t.Errorf("by_isolation[isolate] = %d, want 3", stats.ByIsolation[model.IsolationIsolate])
	}
	if stats.ByIsolation[model.IsolationMicroVM] != 1 {
		t.Errorf("by_isolation[microvm] = %d, want 1", stats.ByIsolation[model.IsolationMicroVM])
	}
	if stats.AvgDurationMS != 100 {
		t.Errorf("avg_duration_ms = %f, want 100", stats.AvgDurationMS)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
