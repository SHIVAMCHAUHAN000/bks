package main

import (
	"automationtool/api/internal/config"
	"automationtool/api/internal/database"
	"automationtool/api/internal/httpapi"
	"automationtool/api/internal/lead"
	"automationtool/api/internal/mailer"
	"log"
	"net/http"
	"time"
)

func main() {
	cfg := config.Load()
	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	resend := mailer.NewResend(cfg.ResendAPIKey, cfg.MailFrom, cfg.MailReplyTo, cfg.PublicAPIURL)
	server := httpapi.New(lead.NewRepository(db), resend, resend, cfg.ResendWebhookSecret, cfg.CORSOrigin)
	server.Configure(httpapi.SendSettings{
		Interval:          time.Second / time.Duration(cfg.SendRatePerSecond),
		DailyLimit:        cfg.DailySendLimit,
		PublicURL:         cfg.PublicAPIURL,
		From:              cfg.MailFrom,
		ReplyTo:           cfg.MailReplyTo,
		Simulated:         resend.Simulated(),
		WebhookConfigured: cfg.ResendWebhookSecret != "",
	})
	if resend.Simulated() {
		log.Printf("RESEND_API_KEY is empty: campaigns are simulated and no real email is sent")
	}
	log.Printf("Automation Tool API listening at :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, server.Routes()))
}
