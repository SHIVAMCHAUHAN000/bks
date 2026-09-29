package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"automationtool/api/internal/lead"
)

func (s *Server) leadsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createLead(w, r)
	case http.MethodGet:
		leads, err := s.leads.List(lead.ListFilter{Query: r.URL.Query().Get("q"), Segment: r.URL.Query().Get("segment")})
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, leads, http.StatusOK)
	default:
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) createLead(w http.ResponseWriter, r *http.Request) {
	var input lead.Input
	if decodeJSON(r, &input) != nil {
		writeError(w, "invalid JSON", 400)
		return
	}
	id, err := s.leads.Create(input)
	switch {
	case errors.Is(err, lead.ErrMissingEmail):
		writeError(w, "Enter an email or a phone number", 400)
		return
	case errors.Is(err, lead.ErrInvalidEmail):
		writeError(w, "That email address is not valid", 400)
		return
	case errors.Is(err, lead.ErrDuplicate):
		writeError(w, "A lead with this email or phone number already exists", 409)
		return
	case err != nil:
		serverError(w, err)
		return
	}
	created, err := s.leads.Get(id)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, created, 201)
}

func (s *Server) bulkDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		IDs []int64 `json:"ids"`
	}
	if decodeJSON(r, &input) != nil || len(input.IDs) == 0 {
		writeError(w, "provide the ids of the leads to delete", 400)
		return
	}
	deleted, err := s.leads.Delete(input.IDs...)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, map[string]int{"deleted": deleted}, 200)
}

func (s *Server) leadDetailHandler(w http.ResponseWriter, r *http.Request) {
	id, action, err := leadID(r.URL.Path)
	if err != nil {
		writeError(w, "bad lead id", 400)
		return
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		found, err := s.leads.Get(id)
		if errors.Is(err, lead.ErrNotFound) {
			writeError(w, "lead not found", 404)
			return
		}
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, found, 200)
	case action == "" && r.Method == http.MethodDelete:
		deleted, err := s.leads.Delete(id)
		if err != nil {
			serverError(w, err)
			return
		}
		if deleted == 0 {
			writeError(w, "lead not found", 404)
			return
		}
		writeJSON(w, map[string]int{"deleted": deleted}, 200)
	case action == "events" && r.Method == http.MethodGet:
		s.events(w, id)
	case action == "reply" && r.Method == http.MethodPost:
		s.recordReply(w, r, id)
	case action == "invalid" && r.Method == http.MethodPatch:
		s.markInvalid(w, id)
	default:
		writeError(w, "not found", 404)
	}
}
func (s *Server) events(w http.ResponseWriter, id int64) {
	events, err := s.leads.Events(id)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, events, 200)
}
func (s *Server) recordReply(w http.ResponseWriter, r *http.Request, id int64) {
	var input struct {
		Content string `json:"content"`
	}
	if decodeJSON(r, &input) != nil || strings.TrimSpace(input.Content) == "" {
		writeError(w, "reply text is required", 400)
		return
	}
	if err := s.leads.RecordReply(id, input.Content); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true}, 200)
}
func (s *Server) markInvalid(w http.ResponseWriter, id int64) {
	if err := s.leads.MarkInvalid(id); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true}, 200)
}
func leadID(path string) (int64, string, error) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(path, "/api/leads/"), "/"), "/")
	id, err := strconv.ParseInt(parts[0], 10, 64)
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	return id, action, err
}
func serverError(w http.ResponseWriter, err error) {
	writeError(w, err.Error(), http.StatusInternalServerError)
}

func (s *Server) conversationsHandler(w http.ResponseWriter, r *http.Request) {
	conversations, err := s.leads.Conversations(r.URL.Query().Get("filter") == "replied", r.URL.Query().Get("q"))
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, conversations, 200)
}
