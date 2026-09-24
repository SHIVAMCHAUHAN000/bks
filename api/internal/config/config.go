package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	DatabasePath string
	ResendAPIKey string
	MailFrom     string
	PublicAPIURL string
}

func Load() Config {
	// A missing .env is fine in deployments, where the OS injects environment variables.
	_ = godotenv.Load()

	return Config{
		Port:         envOr("PORT", "8080"),
		DatabasePath: envOr("DATABASE_PATH", "data/leaddesk.db"),
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		MailFrom:     os.Getenv("MAIL_FROM"),
		PublicAPIURL: envOr("PUBLIC_API_URL", "http://localhost:8080"),
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
