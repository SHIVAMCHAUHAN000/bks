package httpapi

import (
	"encoding/json"
	"net/http"

	"leaddesk/api/internal/lead"
	"leaddesk/api/internal/mailer"
)

type Server struct {
	leads         *lead.Repository
	mailer        mailer.Mailer
	inbound       mailer.ReceivedEmailReader
	corsOrigin    string
	webhookSecret string
}

func New(leads *lead.Repository, mailer mailer.Mailer, inbound mailer.ReceivedEmailReader, webhookSecret, corsOrigin string) *Server {
	return &Server{leads: leads, mailer: mailer, inbound: inbound, webhookSecret: webhookSecret, corsOrigin: corsOrigin}
}
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/api/leads", s.leadsHandler)
	mux.HandleFunc("/api/leads/", s.leadDetailHandler)
	mux.HandleFunc("/api/import", s.importHandler)
	mux.HandleFunc("/api/automation/send", s.automationHandler)
	mux.HandleFunc("/api/dashboard", s.dashboardHandler)
	mux.HandleFunc("/webhooks/resend", s.resendWebhookHandler)
	mux.HandleFunc("/track/open/", s.trackingHandler)
	return cors(mux, s.corsOrigin)
}
func cors(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, value any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func decodeJSON(r *http.Request, value any) error { return json.NewDecoder(r.Body).Decode(value) }
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}
