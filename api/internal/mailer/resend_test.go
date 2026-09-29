package mailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewResend(t *testing.T) {
	r := NewResend("key_123", "from@example.com", "reply@example.com", "https://api.example.com/")
	if r.apiKey != "key_123" || r.from != "from@example.com" || r.replyTo != "reply@example.com" {
		t.Errorf("unexpected config: %+v", r)
	}
	if r.Simulated() {
		t.Error("expected real mode with an API key")
	}
}

func TestSendSimulated(t *testing.T) {
	r := NewResend("", "from@example.com", "", "http://localhost:8080")
	if _, err := r.Send(context.Background(), Message{To: "test@example.com", Subject: "Hello", Text: "World"}); err != nil {
		t.Errorf("expected nil error in simulation mode, got %v", err)
	}
}

func TestSendRealPayloadAndRateLimit(t *testing.T) {
	var got map[string]any
	limited := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limited {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"id":"email_abc"}`))
	}))
	defer server.Close()
	r := NewResend("key", "Team <team@example.com>", "reply@example.com", "")
	r.baseURL = server.URL
	id, err := r.Send(context.Background(), Message{To: "a@example.com", Subject: "Hi", HTML: "<p>Hi</p>", Text: "Hi", Headers: map[string]string{"List-Unsubscribe": "<https://x>"}})
	if err != nil || id != "email_abc" {
		t.Fatalf("send: id=%q err=%v", id, err)
	}
	if got["text"] != "Hi" || got["html"] != "<p>Hi</p>" || got["headers"] == nil || got["reply_to"] == nil {
		t.Errorf("unexpected payload: %v", got)
	}
	limited = true
	_, err = r.Send(context.Background(), Message{To: "a@example.com"})
	if wait, ok := IsRateLimit(err); !ok || wait.Seconds() != 2 {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestReceivedTextMissingKey(t *testing.T) {
	r := NewResend("", "from@example.com", "", "http://localhost:8080")
	if _, err := r.ReceivedText(context.Background(), "email_123"); err == nil {
		t.Errorf("expected error when apiKey is missing, got nil")
	}
}

func TestStripQuotedReply(t *testing.T) {
	text := "Yes, let's talk tomorrow.\n\nOn Mon, 1 Sep 2026 at 10:00, Team <team@example.com> wrote:\n> Hello"
	if got := StripQuotedReply(text); got != "Yes, let's talk tomorrow." {
		t.Errorf("got %q", got)
	}
	if got := StripQuotedReply("Only a reply"); got != "Only a reply" {
		t.Errorf("got %q", got)
	}
}
