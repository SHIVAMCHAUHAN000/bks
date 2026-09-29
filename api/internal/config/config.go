package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                     string
	DatabasePath             string
	ResendAPIKey             string
	ResendWebhookSecret      string
	MailFrom                 string
	MailReplyTo              string
	PublicAPIURL             string
	CORSOrigin               string
	DefaultFollowupDelayDays int
	DefaultFollowupLimit     int
	// SendRatePerSecond keeps campaigns under Resend's API rate limit (default 2/s).
	SendRatePerSecond int
	// DailySendLimit caps emails per day to protect a new domain's reputation (0 = no cap).
	DailySendLimit int
}

func Load() Config {
	// A missing .env is fine in deployments, where the OS injects environment variables.
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()
	_ = godotenv.Load("../.env.local")
	_ = godotenv.Load("../.env")

	return Config{
		Port:                     envOr("PORT", "8080"),
		DatabasePath:             envOr("DATABASE_PATH", "data/automation_tool.db"),
		ResendAPIKey:             os.Getenv("RESEND_API_KEY"),
		ResendWebhookSecret:      os.Getenv("RESEND_WEBHOOK_SECRET"),
		MailFrom:                 envOr("MAIL_FROM", "onboarding@resend.dev"),
		MailReplyTo:              os.Getenv("MAIL_REPLY_TO"),
		PublicAPIURL:             envOr("PUBLIC_API_URL", "http://localhost:8080"),
		CORSOrigin:               envOr("CORS_ALLOWED_ORIGIN", "http://localhost:3000"),
		DefaultFollowupDelayDays: positiveIntEnv("DEFAULT_FOLLOWUP_DELAY_DAYS", 3),
		DefaultFollowupLimit:     positiveIntEnv("DEFAULT_FOLLOWUP_LIMIT", 3),
		SendRatePerSecond:        positiveIntEnv("SEND_RATE_PER_SECOND", 2),
		DailySendLimit:           nonNegativeIntEnv("DAILY_SEND_LIMIT", 0),
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func positiveIntEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func nonNegativeIntEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}
