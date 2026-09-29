package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestVerifyResend(t *testing.T) {
	rawKey := []byte("01234567890123456789012345678901") // 32 bytes
	b64Key := base64.StdEncoding.EncodeToString(rawKey)
	secret := "whsec_" + b64Key

	body := []byte(`{"type":"email.delivered"}`)
	msgID := "msg_test_123"

	// 1. Missing secret
	_, err := VerifyResend("", http.Header{}, body)
	if err == nil || err.Error() != "RESEND_WEBHOOK_SECRET is not configured" {
		t.Errorf("expected missing secret error, got %v", err)
	}

	// 2. Missing headers
	_, err = VerifyResend(secret, http.Header{}, body)
	if err == nil || err.Error() != "missing Svix signature headers" {
		t.Errorf("expected missing headers error, got %v", err)
	}

	// 3. Expired timestamp (> 5 minutes)
	oldTimestamp := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	hdr := http.Header{}
	hdr.Set("svix-id", msgID)
	hdr.Set("svix-timestamp", oldTimestamp)
	hdr.Set("svix-signature", "v1,dGVzdA==")
	_, err = VerifyResend(secret, hdr, body)
	if err == nil || err.Error() != "expired or invalid Svix timestamp" {
		t.Errorf("expected expired timestamp error, got %v", err)
	}

	// 4. Invalid timestamp format
	hdr.Set("svix-timestamp", "not_a_number")
	_, err = VerifyResend(secret, hdr, body)
	if err == nil || err.Error() != "expired or invalid Svix timestamp" {
		t.Errorf("expected invalid timestamp error, got %v", err)
	}

	// 5. Valid signature
	nowTimestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, rawKey)
	mac.Write([]byte(msgID + "." + nowTimestamp + "."))
	mac.Write(body)
	validSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	hdr.Set("svix-timestamp", nowTimestamp)
	hdr.Set("svix-signature", fmt.Sprintf("v1,%s", validSig))

	verifiedID, err := VerifyResend(secret, hdr, body)
	if err != nil {
		t.Fatalf("expected valid signature to pass, got: %v", err)
	}
	if verifiedID != msgID {
		t.Errorf("expected verified ID %s, got %s", msgID, verifiedID)
	}

	// 6. Multiple signatures in svix-signature header
	hdr.Set("svix-signature", fmt.Sprintf("v1,invalidsig v2,something v1,%s", validSig))
	verifiedID, err = VerifyResend(secret, hdr, body)
	if err != nil {
		t.Fatalf("expected multi-signature with valid signature to pass, got: %v", err)
	}
	if verifiedID != msgID {
		t.Errorf("expected verified ID %s, got %s", msgID, verifiedID)
	}

	// 7. Invalid signature
	hdr.Set("svix-signature", "v1,d3Jvbmc=")
	_, err = VerifyResend(secret, hdr, body)
	if err == nil || err.Error() != "invalid Svix signature" {
		t.Errorf("expected invalid signature error, got %v", err)
	}

	// 8. Invalid secret base64
	_, err = VerifyResend("whsec_!!!invalidbase64!!!", hdr, body)
	if err == nil {
		t.Errorf("expected invalid base64 secret error, got nil")
	}
}
