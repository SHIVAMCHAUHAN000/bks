package config

import (
	"os"
	"testing"
)

func TestEnvOr(t *testing.T) {
	key := "TEST_ENV_OR_VAR"
	os.Unsetenv(key)

	if got := envOr(key, "default_val"); got != "default_val" {
		t.Errorf("expected default_val, got %s", got)
	}

	os.Setenv(key, "custom_val")
	defer os.Unsetenv(key)

	if got := envOr(key, "default_val"); got != "custom_val" {
		t.Errorf("expected custom_val, got %s", got)
	}
}

func TestPositiveIntEnv(t *testing.T) {
	key := "TEST_POS_INT_VAR"
	os.Unsetenv(key)

	if got := positiveIntEnv(key, 5); got != 5 {
		t.Errorf("expected 5 for unset env, got %d", got)
	}

	os.Setenv(key, "10")
	if got := positiveIntEnv(key, 5); got != 10 {
		t.Errorf("expected 10, got %d", got)
	}

	os.Setenv(key, "0")
	if got := positiveIntEnv(key, 5); got != 5 {
		t.Errorf("expected fallback 5 for 0, got %d", got)
	}

	os.Setenv(key, "-3")
	if got := positiveIntEnv(key, 5); got != 5 {
		t.Errorf("expected fallback 5 for negative int, got %d", got)
	}

	os.Setenv(key, "invalid_number")
	if got := positiveIntEnv(key, 5); got != 5 {
		t.Errorf("expected fallback 5 for invalid number, got %d", got)
	}
	os.Unsetenv(key)
}

func TestLoad(t *testing.T) {
	os.Setenv("PORT", "9999")
	os.Setenv("DATABASE_PATH", "custom_path.db")
	os.Setenv("RESEND_API_KEY", "re_test_123")
	os.Setenv("RESEND_WEBHOOK_SECRET", "whsec_test_secret")
	os.Setenv("MAIL_FROM", "test@example.com")
	os.Setenv("MAIL_REPLY_TO", "replies@example.com")
	os.Setenv("PUBLIC_API_URL", "https://api.example.com")
	os.Setenv("CORS_ALLOWED_ORIGIN", "https://app.example.com")
	os.Setenv("DEFAULT_FOLLOWUP_DELAY_DAYS", "7")
	os.Setenv("DEFAULT_FOLLOWUP_LIMIT", "5")

	cfg := Load()

	if cfg.Port != "9999" {
		t.Errorf("expected port 9999, got %s", cfg.Port)
	}
	if cfg.DatabasePath != "custom_path.db" {
		t.Errorf("expected database path custom_path.db, got %s", cfg.DatabasePath)
	}
	if cfg.ResendAPIKey != "re_test_123" {
		t.Errorf("expected resend api key re_test_123, got %s", cfg.ResendAPIKey)
	}
	if cfg.ResendWebhookSecret != "whsec_test_secret" {
		t.Errorf("expected resend webhook secret whsec_test_secret, got %s", cfg.ResendWebhookSecret)
	}
	if cfg.MailFrom != "test@example.com" {
		t.Errorf("expected mail from test@example.com, got %s", cfg.MailFrom)
	}
	if cfg.MailReplyTo != "replies@example.com" {
		t.Errorf("expected mail reply to replies@example.com, got %s", cfg.MailReplyTo)
	}
	if cfg.PublicAPIURL != "https://api.example.com" {
		t.Errorf("expected public api url https://api.example.com, got %s", cfg.PublicAPIURL)
	}
	if cfg.CORSOrigin != "https://app.example.com" {
		t.Errorf("expected cors origin https://app.example.com, got %s", cfg.CORSOrigin)
	}
	if cfg.DefaultFollowupDelayDays != 7 {
		t.Errorf("expected default followup delay days 7, got %d", cfg.DefaultFollowupDelayDays)
	}
	if cfg.DefaultFollowupLimit != 5 {
		t.Errorf("expected default followup limit 5, got %d", cfg.DefaultFollowupLimit)
	}
}
