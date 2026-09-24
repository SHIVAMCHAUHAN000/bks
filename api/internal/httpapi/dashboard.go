package httpapi

import "net/http"

func (s *Server) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := s.leads.Stats()
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, stats, 200)
}
