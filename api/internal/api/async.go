package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/seantiz/vulcan/internal/model"
)

// gzipMagic is the two-byte magic number at the start of gzip data.
var gzipMagic = []byte{0x1f, 0x8b}

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

	// Validate and pass code through to the execution engine (transient, not persisted).
	if err := s.parseCodeFields(&req, wl, w); err != nil {
		return // error already written
	}

	if err := s.engine.Submit(r.Context(), wl); err != nil {
		s.logger.Error("submit async workload", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to submit workload")
		return
	}

	s.writeJSON(w, http.StatusAccepted, wl)
}

// parseCodeFields validates and extracts code/code_archive from the request
// into the workload model. Returns an error if validation fails (error already
// written to w). Returns nil on success.
func (s *Server) parseCodeFields(req *createWorkloadRequest, wl *model.Workload, w http.ResponseWriter) error {
	if req.Code != "" && req.CodeArchive != "" {
		s.writeError(w, http.StatusBadRequest, "code and code_archive are mutually exclusive")
		return errValidation
	}

	wl.Code = req.Code

	if req.CodeArchive != "" {
		archive, err := base64.StdEncoding.DecodeString(req.CodeArchive)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "code_archive must be valid base64")
			return errValidation
		}
		if len(archive) < 2 || archive[0] != gzipMagic[0] || archive[1] != gzipMagic[1] {
			s.writeError(w, http.StatusBadRequest, "code_archive must be a gzip-compressed archive")
			return errValidation
		}
		wl.CodeArchive = archive
	}

	return nil
}
