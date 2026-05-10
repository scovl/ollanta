package auth

import (
	"testing"
	"time"
)

func TestGenerateAndParseAccessToken(t *testing.T) {
	secret := []byte("test-secret-key-for-testing")
	token, err := GenerateAccessToken(secret, 42, "testuser", time.Hour)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := ParseAccessToken(secret, token)
	if err != nil {
		t.Fatalf("ParseAccessToken() error = %v", err)
	}
	if claims.Login != "testuser" {
		t.Errorf("Login = %q, want testuser", claims.Login)
	}
	if claims.Subject != "42" {
		t.Errorf("Subject = %q, want 42", claims.Subject)
	}
}

func TestParseAccessTokenRejectsWrongSecret(t *testing.T) {
	secret := []byte("correct-secret")
	token, err := GenerateAccessToken(secret, 1, "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseAccessToken([]byte("wrong-secret"), token)
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestParseAccessTokenRejectsInvalidToken(t *testing.T) {
	_, err := ParseAccessToken([]byte("secret"), "invalid-token-string")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestParseAccessTokenRejectsExpiredToken(t *testing.T) {
	secret := []byte("test-secret")
	token, err := GenerateAccessToken(secret, 1, "user", -time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseAccessToken(secret, token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}
