package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"leaddesk/api/internal/lead"
)

type Mailer interface {
	Send(lead lead.Lead, subject, body string) error
}

type ReceivedEmailReader interface {
	ReceivedText(ctx context.Context, emailID string) (string, error)
}
type Resend struct {
	apiKey       string
	from         string
	publicAPIURL string
}

func NewResend(apiKey, from, publicAPIURL string) *Resend {
	return &Resend{apiKey: apiKey, from: from, publicAPIURL: strings.TrimSuffix(publicAPIURL, "/")}
}

func (m *Resend) Send(lead lead.Lead, subject, body string) error {
	// Local development intentionally records a simulated send instead of sending email.
	if m.apiKey == "" {
		return nil
	}
	html := strings.ReplaceAll(body, "\n", "<br>") + fmt.Sprintf(`<img src="%s/track/open/%d" width="1" height="1" alt=""/>`, m.publicAPIURL, lead.ID)
	payload, _ := json.Marshal(map[string]any{"from": m.from, "to": []string{lead.Email}, "subject": subject, "html": html})
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+m.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("resend returned %s", response.Status)
	}
	return nil
}

// ReceivedText fetches the body from Resend because email.received webhooks contain metadata only.
func (m *Resend) ReceivedText(ctx context.Context, emailID string) (string, error) {
	if m.apiKey == "" {
		return "", fmt.Errorf("RESEND_API_KEY is required to retrieve inbound email content")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.resend.com/emails/receiving/"+emailID, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+m.apiKey)
	response, err := http.DefaultClient.Do(request)
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
		return received.Text, nil
	}
	return received.HTML, nil
}
