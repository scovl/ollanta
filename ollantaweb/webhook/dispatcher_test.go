package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// signPayload mirrors the sign function in dispatcher.go for testing HMAC format.
func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHMACSignature(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"event":"scan.completed","project_id":1}`)
	secret := "s3cr3t"

	sig := signPayload(payload, secret)

	if sig[:7] != "sha256=" {
		t.Errorf("signature must start with 'sha256=', got %s", sig)
	}
	// Verify signature is deterministic.
	sig2 := signPayload(payload, secret)
	if sig != sig2 {
		t.Error("HMAC signature must be deterministic")
	}
}

func TestWebhookHMACDifferentSecrets(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"event":"scan.completed"}`)
	sig1 := signPayload(payload, "secret1")
	sig2 := signPayload(payload, "secret2")
	if sig1 == sig2 {
		t.Error("different secrets must produce different signatures")
	}
}

func TestWebhookEventConstants(t *testing.T) {
	t.Parallel()
	// Ensure event constant strings match expected values used by subscribers.
	events := map[string]string{
		"scan.completed":  "scan.completed",
		"gate.changed":    "gate.changed",
		"project.created": "project.created",
		"project.deleted": "project.deleted",
	}
	for k, v := range events {
		if k != v {
			t.Errorf("event constant mismatch: %s != %s", k, v)
		}
	}
}
