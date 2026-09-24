package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/mail"
	"strings"

	"automationtool/api/internal/lead"
	"automationtool/api/internal/webhook"
)

const maxWebhookBody = 1 << 20

type resendPayload struct {
	Type string `json:"type"`
	Data struct {
		EmailID   string   `json:"email_id"`
		From      string   `json:"from"`
		To        []string `json:"to"`
		Subject   string   `json:"subject"`
		MessageID string   `json:"message_id"`
		Bounce    struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"bounce"`
	} `json:"data"`
}

func (s *Server) resendWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	receiptID, err := webhook.VerifyResend(s.webhookSecret, r.Header, body)
	if err != nil {
		http.Error(w, "invalid webhook", http.StatusUnauthorized)
		return
	}

	var payload resendPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if !handledWebhookType(payload.Type) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	event := lead.WebhookEvent{Type: payload.Type, Sender: emailAddress(payload.Data.From), Subject: payload.Data.Subject, MessageID: payload.Data.MessageID, BounceType: payload.Data.Bounce.Type, BounceMessage: payload.Data.Bounce.Message}
	if len(payload.Data.To) > 0 {
		event.Recipient = emailAddress(payload.Data.To[0])
	}
	if payload.Type == "email.received" {
		content, err := s.inbound.ReceivedText(context.Background(), payload.Data.EmailID)
		if err != nil {
			serverError(w, err)
			return
		}
		event.Content = content
	}
	processed, err := s.leads.ProcessWebhook(receiptID, event)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"processed": processed}, http.StatusOK)
}

func handledWebhookType(value string) bool {
	switch value {
	case "email.delivered", "email.bounced", "email.opened", "email.received":
		return true
	}
	return false
}
func emailAddress(value string) string {
	parsed, err := mail.ParseAddress(value)
	if err == nil {
		return strings.ToLower(parsed.Address)
	}
	return strings.ToLower(strings.TrimSpace(value))
}
