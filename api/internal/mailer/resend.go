package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Message is one fully personalised email.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
	Headers map[string]string
}

type Mailer interface {
	// Send delivers a message and returns the provider's email id.
	Send(ctx context.Context, message Message) (string, error)
}

type ReceivedEmailReader interface {
	ReceivedText(ctx context.Context, emailID string) (string, error)
}

// RateLimitError means the provider asked us to slow down.
type RateLimitError struct{ RetryAfter time.Duration }

func (e RateLimitError) Error() string { return "resend rate limit reached" }

type Resend struct {
	apiKey  string
	from    string
	replyTo string
	baseURL string
	client  *http.Client
}

func NewResend(apiKey, from, replyTo, publicAPIURL string) *Resend {
	return &Resend{apiKey: apiKey, from: from, replyTo: replyTo, baseURL: "https://api.resend.com", client: &http.Client{Timeout: 30 * time.Second}}
}

// Simulated reports whether sends are only recorded locally.
func (m *Resend) Simulated() bool { return m.apiKey == "" }
func (m *Resend) From() string    { return m.from }
func (m *Resend) ReplyTo() string { return m.replyTo }

func (m *Resend) Send(ctx context.Context, message Message) (string, error) {
	// Local development intentionally records a simulated send instead of sending email.
	if m.apiKey == "" {
		return "", nil
	}
	payload := map[string]any{"from": m.from, "to": []string{message.To}, "subject": message.Subject, "html": message.HTML, "text": message.Text}
	if m.replyTo != "" {
		payload["reply_to"] = []string{m.replyTo}
	}
	if len(message.Headers) > 0 {
		payload["headers"] = message.Headers
	}
	payloadBytes, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/emails", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+m.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := m.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(response.Body, 1<<16))
	if response.StatusCode == http.StatusTooManyRequests {
		wait, _ := strconv.Atoi(response.Header.Get("Retry-After"))
		if wait < 1 {
			wait = 1
		}
		return "", RateLimitError{RetryAfter: time.Duration(wait) * time.Second}
	}
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("resend returned %s: %s", response.Status, strings.TrimSpace(string(respBody)))
	}
	var sent struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &sent)
	return sent.ID, nil
}

// IsRateLimit reports whether err is a provider rate limit.
func IsRateLimit(err error) (time.Duration, bool) {
	var limit RateLimitError
	if errors.As(err, &limit) {
		return limit.RetryAfter, true
	}
	return 0, false
}

// ReceivedText fetches the body from Resend because email.received webhooks contain metadata only.
func (m *Resend) ReceivedText(ctx context.Context, emailID string) (string, error) {
	if m.apiKey == "" {
		return "", fmt.Errorf("RESEND_API_KEY is required to retrieve inbound email content")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+"/emails/receiving/"+emailID, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+m.apiKey)
	response, err := m.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("resend returned %s while retrieving received email", response.Status)
	}
	var received struct {
		Text string `json:"text"`
		HTML string `json:"html"`
	}
	if err := json.NewDecoder(response.Body).Decode(&received); err != nil {
		return "", err
	}
	if received.Text != "" {
		return StripQuotedReply(received.Text), nil
	}
	return received.HTML, nil
}

// StripQuotedReply removes the quoted original message that mail clients append to replies.
func StripQuotedReply(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if (strings.HasPrefix(trimmed, "On ") && strings.HasSuffix(trimmed, "wrote:")) || strings.HasPrefix(trimmed, "-----Original Message-----") || (strings.HasPrefix(trimmed, "From:") && i > 0) {
			if kept := strings.TrimSpace(strings.Join(lines[:i], "\n")); kept != "" {
				return kept
			}
			break
		}
	}
	return strings.TrimSpace(text)
}
