package httpapi

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"automationtool/api/internal/lead"
	"automationtool/api/internal/mailer"
)

// buildMessage personalises the campaign for one lead.
func (s *Server) buildMessage(c lead.Campaign, l lead.Lead, unsubToken string) (mailer.Message, string, string) {
	subject := strings.TrimSpace(strings.ReplaceAll(lead.Render(c.Subject, l), "\n", " "))
	body := strings.TrimSpace(lead.Render(c.Body, l))
	text, html := body, lead.PlainToHTML(body)
	headers := map[string]string{}
	if c.IncludeUnsubscribe && unsubToken != "" {
		link := s.send.PublicURL + "/unsubscribe/" + unsubToken
		text += "\n\n--\nIf you would prefer not to hear from us, reply \"unsubscribe\" or use this link: " + link
		html += `<p style="margin:24px 0 0;font-family:Arial,Helvetica,sans-serif;font-size:11px;color:#888">If you would prefer not to hear from us, <a href="` + link + `" style="color:#888">unsubscribe here</a>.</p>`
		headers["List-Unsubscribe"] = "<" + link + ">"
		headers["List-Unsubscribe-Post"] = "List-Unsubscribe=One-Click"
	}
	if c.TrackOpens {
		html += fmt.Sprintf(`<img src="%s/track/open/%d" width="1" height="1" alt="" style="display:none"/>`, s.send.PublicURL, l.ID)
	}
	return mailer.Message{To: l.Email, Subject: subject, HTML: html, Text: text, Headers: headers}, subject, body
}

func (s *Server) previewHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		lead.Campaign
		LeadID int64 `json:"leadId"`
	}
	if decodeJSON(r, &input) != nil {
		writeError(w, "invalid JSON", 400)
		return
	}
	targets, err := s.leads.CampaignTargets(input.Campaign)
	if err != nil {
		serverError(w, err)
		return
	}
	var sample lead.Lead
	if input.LeadID > 0 {
		if sample, err = s.leads.Get(input.LeadID); err != nil {
			writeError(w, "lead not found", 404)
			return
		}
	} else if len(targets) > 0 {
		sample = targets[0]
	} else {
		sample = lead.Lead{Name: "Priya Sharma", Organization: "Example Pvt Ltd", Designation: "Director", Address: "Mumbai", Email: "priya@example.com"}
	}
	message, subject, _ := s.buildMessage(input.Campaign, sample, "preview-token")
	sentToday, _ := s.leads.SentToday()
	writeJSON(w, map[string]any{
		"eligible": len(targets), "lead": sample, "subject": subject, "text": message.Text, "html": message.HTML,
		"unknown": lead.UnknownPlaceholders(input.Subject, input.Body), "fields": lead.TemplateFields,
		"dailyLimit": s.send.DailyLimit, "sentToday": sentToday,
	}, 200)
}

func (s *Server) automationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var campaign lead.Campaign
	if decodeJSON(r, &campaign) != nil {
		writeError(w, "invalid JSON", 400)
		return
	}
	if strings.TrimSpace(campaign.Subject) == "" || strings.TrimSpace(campaign.Body) == "" {
		writeError(w, "subject and body are required", 400)
		return
	}
	if unknown := lead.UnknownPlaceholders(campaign.Subject, campaign.Body); len(unknown) > 0 {
		writeError(w, "Unknown placeholder(s): {{"+strings.Join(unknown, "}}, {{")+"}}", 400)
		return
	}
	if campaign.Mode != "followup" {
		campaign.Mode = "initial"
	}
	s.campaignMu.Lock()
	defer s.campaignMu.Unlock()
	if s.activeRun != 0 {
		writeError(w, "A campaign is already sending. Wait for it to finish.", http.StatusConflict)
		return
	}
	targets, err := s.leads.CampaignTargets(campaign)
	if err != nil {
		serverError(w, err)
		return
	}
	if len(targets) == 0 {
		writeError(w, "No eligible leads found matching this mode and filter.", 400)
		return
	}
	runID, err := s.leads.CreateCampaignRun(campaign, len(targets))
	if err != nil {
		serverError(w, err)
		return
	}
	s.activeRun = runID
	s.runWG.Add(1)
	go s.runCampaign(runID, campaign, targets)
	run, _ := s.leads.CampaignRun(runID)
	writeJSON(w, run, http.StatusAccepted)
}

// runCampaign sends one email at a time, respecting the rate and daily limits.
func (s *Server) runCampaign(runID int64, campaign lead.Campaign, targets []lead.Lead) {
	defer s.runWG.Done()
	defer func() {
		s.campaignMu.Lock()
		s.activeRun = 0
		s.campaignMu.Unlock()
	}()
	sent, failed, lastError := 0, 0, ""
	status := "completed"
	for i, target := range targets {
		if s.send.DailyLimit > 0 {
			if today, err := s.leads.SentToday(); err == nil && today >= s.send.DailyLimit {
				status, lastError = "daily_limit", fmt.Sprintf("Stopped after reaching the daily limit of %d emails. Run the campaign again tomorrow to continue.", s.send.DailyLimit)
				break
			}
		}
		if i > 0 && s.send.Interval > 0 {
			time.Sleep(s.send.Interval)
		}
		token := ""
		if campaign.IncludeUnsubscribe {
			token, _ = s.leads.EnsureUnsubToken(target.ID)
		}
		message, subject, body := s.buildMessage(campaign, target, token)
		resendID, err := s.sendWithRetry(message)
		if err == nil {
			err = s.leads.RecordSend(lead.SentMessage{LeadID: target.ID, CampaignID: runID, Mode: campaign.Mode, Subject: subject, Body: body, From: s.send.From, To: target.Email, ResendID: resendID})
		}
		if err != nil {
			failed++
			lastError = err.Error()
			log.Printf("campaign=%d send failed lead=%d: %v", runID, target.ID, err)
			_ = s.leads.RecordFailure(target.ID, runID, subject, err.Error())
		} else {
			sent++
		}
		_ = s.leads.UpdateCampaignRun(runID, sent, failed, lastError)
	}
	if status == "completed" && sent == 0 && failed > 0 {
		status = "failed"
	}
	_ = s.leads.UpdateCampaignRun(runID, sent, failed, lastError)
	_ = s.leads.FinishCampaignRun(runID, status)
	log.Printf("campaign=%d finished status=%s sent=%d failed=%d", runID, status, sent, failed)
}

func (s *Server) sendWithRetry(message mailer.Message) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		id, err := s.mailer.Send(ctx, message)
		cancel()
		if err == nil {
			return id, nil
		}
		lastErr = err
		wait, limited := mailer.IsRateLimit(err)
		if !limited {
			return "", err
		}
		time.Sleep(wait)
	}
	return "", lastErr
}

func (s *Server) campaignsHandler(w http.ResponseWriter, r *http.Request) {
	runs, err := s.leads.CampaignRuns(20)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, runs, 200)
}

func (s *Server) campaignHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/campaigns/"), "/"), 10, 64)
	if err != nil {
		writeError(w, "bad campaign id", 400)
		return
	}
	run, err := s.leads.CampaignRun(id)
	if err != nil {
		writeError(w, "campaign not found", 404)
		return
	}
	writeJSON(w, run, 200)
}

func (s *Server) settingsHandler(w http.ResponseWriter, r *http.Request) {
	sentToday, _ := s.leads.SentToday()
	s.campaignMu.Lock()
	active := s.activeRun
	s.campaignMu.Unlock()
	warnings := []string{}
	if s.send.Simulated {
		warnings = append(warnings, "RESEND_API_KEY is not set, so campaigns are only simulated.")
	} else if strings.Contains(s.send.From, "resend.dev") {
		warnings = append(warnings, "MAIL_FROM uses resend.dev, which can only send to your own address. Use a verified domain.")
	}
	if s.send.ReplyTo == "" {
		warnings = append(warnings, "MAIL_REPLY_TO is not set, so replies will not reach the Inbox automatically.")
	}
	if !s.send.WebhookConfigured {
		warnings = append(warnings, "RESEND_WEBHOOK_SECRET is not set, so delivery, bounce and reply events are not recorded.")
	}
	if strings.Contains(s.send.PublicURL, "localhost") {
		warnings = append(warnings, "PUBLIC_API_URL is localhost, so open tracking and unsubscribe links only work on this computer.")
	}
	writeJSON(w, map[string]any{
		"simulated": s.send.Simulated, "from": s.send.From, "replyTo": s.send.ReplyTo, "dailyLimit": s.send.DailyLimit,
		"sentToday": sentToday, "perMinute": int(time.Minute / max(s.send.Interval, time.Millisecond)), "activeCampaign": active, "warnings": warnings,
	}, 200)
}

func (s *Server) unsubscribeHandler(w http.ResponseWriter, r *http.Request) {
	token := strings.Trim(strings.TrimPrefix(r.URL.Path, "/unsubscribe/"), "/")
	ok, err := s.leads.Unsubscribe(token)
	if r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	message := "You have been unsubscribed and will not receive further emails from us."
	if err != nil || !ok {
		message = "This unsubscribe link is not valid or has expired. Reply to our email with \"unsubscribe\" and we will remove you."
	}
	fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Unsubscribe</title></head><body style="font-family:Arial,sans-serif;max-width:520px;margin:80px auto;padding:0 20px;color:#222"><h2>Unsubscribe</h2><p>%s</p></body></html>`, message)
}
