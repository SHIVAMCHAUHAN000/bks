package httpapi

import (
	"fmt"
	"automationtool/api/internal/lead"
	"log"
	"net/http"
)

func (s *Server) automationHandler(w http.ResponseWriter, r *http.Request) {
	var campaign lead.Campaign
	if decodeJSON(r, &campaign) != nil {
		writeJSON(w, map[string]string{"error": "invalid JSON"}, 400)
		return
	}
	if campaign.Subject == "" || campaign.Body == "" {
		writeJSON(w, map[string]string{"error": "subject and body are required"}, 400)
		return
	}
	targets, err := s.leads.CampaignTargets(campaign)
	if err != nil {
		serverError(w, err)
		return
	}
	if len(targets) == 0 {
		writeJSON(w, map[string]any{"sent": 0, "error": "No eligible leads found matching this mode and filter."}, 400)
		return
	}
	sent := 0
	var lastErr error
	for _, target := range targets {
		if err := s.mailer.Send(target, campaign.Subject, campaign.Body); err == nil {
			if err = s.leads.RecordSend(target.ID, campaign); err == nil {
				sent++
			}
		} else {
			log.Printf("Failed to send email to %s: %v", target.Email, err)
			lastErr = err
		}
	}
	if sent == 0 && lastErr != nil {
		writeJSON(w, map[string]string{"error": fmt.Sprintf("Failed to send: %v", lastErr)}, 400)
		return
	}
	writeJSON(w, map[string]int{"sent": sent}, 200)
}
