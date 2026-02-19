package api

import "net/http"

func (s *Server) handleListBackends(w http.ResponseWriter, _ *http.Request) {
	backends := s.registry.List()
	s.writeJSON(w, http.StatusOK, backends)
}
