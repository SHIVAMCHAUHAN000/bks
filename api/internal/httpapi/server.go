package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"automationtool/api/internal/lead"
	"automationtool/api/internal/mailer"
)

// SendSettings controls how campaigns are delivered.
type SendSettings struct {
	// Interval between two emails. Resend's default limit is 2 requests per second.
	Interval time.Duration
	// DailyLimit stops a campaign once this many emails were sent today (0 = no limit).
	DailyLimit int
	// PublicURL is the public API address used for open tracking and unsubscribe links.
	PublicURL string
	From      string
	ReplyTo   string
	Simulated bool
	// WebhookConfigured tells the UI whether replies will arrive automatically.
	WebhookConfigured bool
}

type Server struct {
	leads         *lead.Repository
	mailer        mailer.Mailer
	inbound       mailer.ReceivedEmailReader
	corsOrigin    string
	webhookSecret string
	send          SendSettings

	campaignMu sync.Mutex
	activeRun  int64
	runWG      sync.WaitGroup
}

func New(leads *lead.Repository, mailer mailer.Mailer, inbound mailer.ReceivedEmailReader, webhookSecret, corsOrigin string) *Server {
	return &Server{leads: leads, mailer: mailer, inbound: inbound, webhookSecret: webhookSecret, corsOrigin: corsOrigin,
		send: SendSettings{Interval: 600 * time.Millisecond, PublicURL: "http://localhost:8080", WebhookConfigured: webhookSecret != ""}}
}

// Configure replaces the delivery settings.
func (s *Server) Configure(settings SendSettings) {
	settings.PublicURL = strings.TrimSuffix(settings.PublicURL, "/")
	s.send = settings
}

// Wait blocks until background campaigns have finished (used by tests and shutdown).
func (s *Server) Wait() { s.runWG.Wait() }

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.rootHandler)
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/api/leads", s.leadsHandler)
	mux.HandleFunc("/api/leads/delete", s.bulkDeleteHandler)
	mux.HandleFunc("/api/leads/", s.leadDetailHandler)
	mux.HandleFunc("/api/import/preview", s.importPreviewHandler)
	mux.HandleFunc("/api/import", s.importHandler)
	mux.HandleFunc("/api/automation/preview", s.previewHandler)
	mux.HandleFunc("/api/automation/send", s.automationHandler)
	mux.HandleFunc("/api/campaigns", s.campaignsHandler)
	mux.HandleFunc("/api/campaigns/", s.campaignHandler)
	mux.HandleFunc("/api/conversations", s.conversationsHandler)
	mux.HandleFunc("/api/dashboard", s.dashboardHandler)
	mux.HandleFunc("/api/settings", s.settingsHandler)
	mux.HandleFunc("/webhooks/resend", s.resendWebhookHandler)
	mux.HandleFunc("/track/open/", s.trackingHandler)
	mux.HandleFunc("/unsubscribe/", s.unsubscribeHandler)
	return logging(cors(mux, s.corsOrigin))
}
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)
		log.Printf("http method=%s path=%s status=%d duration=%s", r.Method, r.URL.Path, writer.status, time.Since(started).Round(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func cors(next http.Handler, origin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowOrigin := origin
		reqOrigin := r.Header.Get("Origin")
		if reqOrigin != "" && (reqOrigin == origin || reqOrigin == "http://localhost:3000" || reqOrigin == "http://localhost:3001") {
			allowOrigin = reqOrigin
		}
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
func writeError(w http.ResponseWriter, message string, status int) {
	writeJSON(w, map[string]string{"error": message}, status)
}
func decodeJSON(r *http.Request, value any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 20<<20)).Decode(value)
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}
func (s *Server) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]any{
		"name":      "Automation Tool API",
		"status":    "running",
		"health":    "/health",
		"dashboard": "/api/dashboard",
		"leads":     "/api/leads",
		"frontend":  s.corsOrigin,
	}, http.StatusOK)
}
