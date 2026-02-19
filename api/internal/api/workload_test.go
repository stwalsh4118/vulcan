package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"context"

	"github.com/seantiz/vulcan/internal/backend"
	"github.com/seantiz/vulcan/internal/model"
)

func TestCreateWorkloadValid(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	body := `{"runtime":"node","code":"console.log('hello')","resources":{"cpus":1,"mem_mb":128,"timeout_s":30}}`
	resp, err := http.Post(ts.URL+"/v1/workloads", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /v1/workloads: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}

	var wl model.Workload
	if err := json.NewDecoder(resp.Body).Decode(&wl); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(wl.ID) != 26 {
		t.Errorf("ID length = %d, want 26", len(wl.ID))
	}
	if wl.Status != model.StatusPending {
		t.Errorf("Status = %q, want %q", wl.Status, model.StatusPending)
	}
	if wl.Runtime != "node" {
		t.Errorf("Runtime = %q, want %q", wl.Runtime, "node")
	}
	if wl.CPULimit == nil || *wl.CPULimit != 1 {
		t.Errorf("CPULimit = %v, want 1", wl.CPULimit)
	}
}

func TestCreateWorkloadMissingRuntime(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	body := `{"code":"console.log('hello')"}`
	resp, err := http.Post(ts.URL+"/v1/workloads", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST /v1/workloads: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}

	var errResp map[string]string
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp["error"] == "" {
		t.Error("expected error message in response")
	}
}

func TestCreateWorkloadInvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/v1/workloads", "application/json", bytes.NewBufferString("not json"))
	if err != nil {
		t.Fatalf("POST /v1/workloads: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestGetWorkloadExisting(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Create a workload first.
	body := `{"runtime":"go"}`
	createResp, _ := http.Post(ts.URL+"/v1/workloads", "application/json", bytes.NewBufferString(body))
	var created model.Workload
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	// Get by ID.
	resp, err := http.Get(ts.URL + "/v1/workloads/" + created.ID)
	if err != nil {
		t.Fatalf("GET /v1/workloads/%s: %v", created.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var got model.Workload
	json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
}

func TestGetWorkloadNotFound(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/workloads/nonexistent")
	if err != nil {
		t.Fatalf("GET /v1/workloads/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestListWorkloadsEmpty(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/workloads")
	if err != nil {
		t.Fatalf("GET /v1/workloads: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var listResp listWorkloadsResponse
	json.NewDecoder(resp.Body).Decode(&listResp)

	if listResp.Total != 0 {
		t.Errorf("total = %d, want 0", listResp.Total)
	}
	if len(listResp.Workloads) != 0 {
		t.Errorf("workloads count = %d, want 0", len(listResp.Workloads))
	}
}

func TestListWorkloadsPagination(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Create 5 workloads.
	for i := 0; i < 5; i++ {
		body := fmt.Sprintf(`{"runtime":"node","code":"test%d"}`, i)
		resp, _ := http.Post(ts.URL+"/v1/workloads", "application/json", bytes.NewBufferString(body))
		resp.Body.Close()
	}

	// List with limit=2, offset=0.
	resp, err := http.Get(ts.URL + "/v1/workloads?limit=2&offset=0")
	if err != nil {
		t.Fatalf("GET /v1/workloads: %v", err)
	}
	defer resp.Body.Close()

	var listResp listWorkloadsResponse
	json.NewDecoder(resp.Body).Decode(&listResp)

	if listResp.Total != 5 {
		t.Errorf("total = %d, want 5", listResp.Total)
	}
	if len(listResp.Workloads) != 2 {
		t.Errorf("workloads count = %d, want 2", len(listResp.Workloads))
	}
	if listResp.Limit != 2 {
		t.Errorf("limit = %d, want 2", listResp.Limit)
	}
	if listResp.Offset != 0 {
		t.Errorf("offset = %d, want 0", listResp.Offset)
	}
}

func TestListWorkloadsDefaultLimit(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/workloads")
	if err != nil {
		t.Fatalf("GET /v1/workloads: %v", err)
	}
	defer resp.Body.Close()

	var listResp listWorkloadsResponse
	json.NewDecoder(resp.Body).Decode(&listResp)

	if listResp.Limit != defaultListLimit {
		t.Errorf("limit = %d, want %d", listResp.Limit, defaultListLimit)
	}
}

func TestDeleteWorkloadExisting(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Create a workload.
	body := `{"runtime":"python"}`
	createResp, _ := http.Post(ts.URL+"/v1/workloads", "application/json", bytes.NewBufferString(body))
	var created model.Workload
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	// Delete it.
	req, _ := http.NewRequest("DELETE", ts.URL+"/v1/workloads/"+created.ID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /v1/workloads/%s: %v", created.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var deleted model.Workload
	json.NewDecoder(resp.Body).Decode(&deleted)

	if deleted.Status != model.StatusKilled {
		t.Errorf("Status = %q, want %q", deleted.Status, model.StatusKilled)
	}
	if deleted.FinishedAt == nil {
		t.Error("FinishedAt is nil, expected it to be set")
	}
}

func TestDeleteWorkloadNotFound(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	req, _ := http.NewRequest("DELETE", ts.URL+"/v1/workloads/nonexistent", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /v1/workloads/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// stubBackend is a minimal Backend for endpoint tests.
type stubBackend struct{}

func (s *stubBackend) Execute(_ context.Context, _ backend.WorkloadSpec) (backend.WorkloadResult, error) {
	return backend.WorkloadResult{}, nil
}
func (s *stubBackend) Capabilities() backend.BackendCapabilities {
	return backend.BackendCapabilities{
		Name:                "test-isolate",
		SupportedRuntimes:   []string{model.RuntimeNode},
		SupportedIsolations: []string{model.IsolationIsolate},
		MaxConcurrency:      4,
	}
}
func (s *stubBackend) Cleanup(_ context.Context, _ string) error { return nil }

func TestListBackendsEndpoint(t *testing.T) {
	srv := newTestServer(t)
	srv.registry.Register(model.IsolationIsolate, &stubBackend{})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/backends")
	if err != nil {
		t.Fatalf("GET /v1/backends: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var backends []backend.BackendInfo
	if err := json.NewDecoder(resp.Body).Decode(&backends); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(backends) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(backends))
	}
	if backends[0].Name != model.IsolationIsolate {
		t.Errorf("backend name = %q, want %q", backends[0].Name, model.IsolationIsolate)
	}
}

func TestListBackendsEmpty(t *testing.T) {
	srv := newTestServer(t)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/backends")
	if err != nil {
		t.Fatalf("GET /v1/backends: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var backends []backend.BackendInfo
	if err := json.NewDecoder(resp.Body).Decode(&backends); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(backends) != 0 {
		t.Errorf("expected 0 backends, got %d", len(backends))
	}
}
