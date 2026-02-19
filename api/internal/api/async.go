package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/seantiz/vulcan/internal/model"
)

func (s *Server) handleAsyncWorkload(w http.ResponseWriter, r *http.Request) {
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
		CreatedAt: now,
	}
	if wl.Isolation == "" {
		wl.Isolation = model.IsolationAuto
	}

	if req.Resources != nil {
		wl.CPULimit = req.Resources.CPUs
		wl.MemLimit = req.Resources.MemMB
		wl.TimeoutS = req.Resources.TimeoutS
	}

	if err := s.engine.Submit(r.Context(), wl); err != nil {
		s.logger.Error("submit async workload", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to submit workload")
		return
	}

	s.writeJSON(w, http.StatusAccepted, wl)
}
