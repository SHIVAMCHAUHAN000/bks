package httpapi

import (
	"leaddesk/api/internal/lead"
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
	sent := 0
	for _, target := range targets {
		if err := s.mailer.Send(target, campaign.Subject, campaign.Body); err == nil {
			if err = s.leads.RecordSend(target.ID, campaign); err == nil {
				sent++
			}
		}
	}
	writeJSON(w, map[string]int{"sent": sent}, 200)
}
