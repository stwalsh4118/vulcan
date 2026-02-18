package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/seantiz/vulcan/internal/model"
	"github.com/seantiz/vulcan/internal/store"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
	maxBodySize      = 1 << 20 // 1 MB
)

// createWorkloadRequest is the JSON body for POST /v1/workloads.
type createWorkloadRequest struct {
	Runtime   string          `json:"runtime"`
	Isolation string          `json:"isolation"`
	Code      string          `json:"code"`
	Input     json.RawMessage `json:"input"`
	Resources *resourcesReq   `json:"resources"`
}

type resourcesReq struct {
	CPUs     *int `json:"cpus"`
	MemMB    *int `json:"mem_mb"`
	TimeoutS *int `json:"timeout_s"`
}

// listWorkloadsResponse wraps the paginated list response.
type listWorkloadsResponse struct {
	Workloads []*model.Workload `json:"workloads"`
	Total     int               `json:"total"`
	Limit     int               `json:"limit"`
	Offset    int               `json:"offset"`
}

func (s *Server) handleCreateWorkload(w http.ResponseWriter, r *http.Request) {
	var req createWorkloadRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Runtime == "" {
		s.writeError(w, http.StatusBadRequest, "runtime is required")
		return
	}

	now := time.Now().UTC()
	wl := &model.Workload{
		ID:        model.NewID(),
		Status:    model.StatusPending,
		Isolation: req.Isolation,
		Runtime:   req.Runtime,
		NodeID:    "",
		CreatedAt: now,
	}

	if req.Resources != nil {
		wl.CPULimit = req.Resources.CPUs
		wl.MemLimit = req.Resources.MemMB
		wl.TimeoutS = req.Resources.TimeoutS
	}

	if err := s.store.CreateWorkload(r.Context(), wl); err != nil {
		s.logger.Error("create workload", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create workload")
		return
	}

	s.writeJSON(w, http.StatusCreated, wl)
}

func (s *Server) handleGetWorkload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	wl, err := s.store.GetWorkload(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, "workload not found")
		return
	}
	if err != nil {
		s.logger.Error("get workload", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to get workload")
		return
	}

	s.writeJSON(w, http.StatusOK, wl)
}

func (s *Server) handleListWorkloads(w http.ResponseWriter, r *http.Request) {
	limit := parseIntQuery(r, "limit", defaultListLimit)
	offset := parseIntQuery(r, "offset", 0)

	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}
	if offset < 0 {
		offset = 0
	}

	workloads, total, err := s.store.ListWorkloads(r.Context(), limit, offset)
	if err != nil {
		s.logger.Error("list workloads", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to list workloads")
		return
	}

	if workloads == nil {
		workloads = []*model.Workload{}
	}

	s.writeJSON(w, http.StatusOK, listWorkloadsResponse{
		Workloads: workloads,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
	})
}

func (s *Server) handleDeleteWorkload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.store.UpdateWorkloadStatus(r.Context(), id, model.StatusKilled); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.writeError(w, http.StatusNotFound, "workload not found")
			return
		}
		s.logger.Error("delete workload", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to kill workload")
		return
	}

	wl, err := s.store.GetWorkload(r.Context(), id)
	if err != nil {
		s.logger.Error("get killed workload", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to retrieve workload")
		return
	}

	s.writeJSON(w, http.StatusOK, wl)
}

// writeJSON writes a JSON response with the given status code.
func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.logger.Error("encode response", "error", err)
	}
}

// writeError writes a JSON error response.
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

// parseIntQuery parses an integer query parameter with a default value.
func parseIntQuery(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
