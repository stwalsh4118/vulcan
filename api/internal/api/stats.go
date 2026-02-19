package api

import (
	"net/http"
)

// statsResponse is the JSON response for GET /v1/stats.
type statsResponse struct {
	Total         int            `json:"total"`
	ByStatus      map[string]int `json:"by_status"`
	ByIsolation   map[string]int `json:"by_isolation"`
	AvgDurationMS float64        `json:"avg_duration_ms"`
}

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetWorkloadStats(r.Context())
	if err != nil {
		s.logger.Error("get workload stats", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	s.writeJSON(w, http.StatusOK, statsResponse{
		Total:         stats.Total,
		ByStatus:      stats.CountByStatus,
		ByIsolation:   stats.CountByIsolation,
		AvgDurationMS: stats.AvgDurationMS,
	})
}
