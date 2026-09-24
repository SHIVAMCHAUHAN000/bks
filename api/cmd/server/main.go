package main

import (
	"automationtool/api/internal/config"
	"automationtool/api/internal/database"
	"automationtool/api/internal/httpapi"
	"automationtool/api/internal/lead"
	"automationtool/api/internal/mailer"
	"log"
	"net/http"
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
	log.Printf("Automation Tool API listening at :%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, server.Routes()))
}
