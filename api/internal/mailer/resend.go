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
