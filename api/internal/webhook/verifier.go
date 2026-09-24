// Package webhook verifies signed Resend/Svix webhook deliveries.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxAge = 5 * time.Minute

func VerifyResend(secret string, header http.Header, body []byte) (string, error) {
	if secret == "" {
		return "", errors.New("RESEND_WEBHOOK_SECRET is not configured")
	}
	id, timestamp, signature := header.Get("svix-id"), header.Get("svix-timestamp"), header.Get("svix-signature")
	if id == "" || timestamp == "" || signature == "" {
		return "", errors.New("missing Svix signature headers")
	}

	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(unix, 0)).Abs() > maxAge {
		return "", errors.New("expired or invalid Svix timestamp")
	}
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + timestamp + "."))
	mac.Write(body)
	expected := mac.Sum(nil)
	for _, candidate := range strings.Fields(signature) {
		version, encoded, found := strings.Cut(candidate, ",")
		if !found || version != "v1" {
			continue
		}
		signatureBytes, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil && subtle.ConstantTimeCompare(expected, signatureBytes) == 1 {
			return id, nil
		}
	}
	return "", errors.New("invalid Svix signature")
}

func decodeSecret(secret string) ([]byte, error) {
	encoded := strings.TrimPrefix(secret, "whsec_")
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook secret: %w", err)
	}
	return key, nil
}
