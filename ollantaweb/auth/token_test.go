package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIToken(t *testing.T) {
	plain, hash, err := GenerateAPIToken()
	if err != nil {
		t.Fatalf("GenerateAPIToken() error = %v", err)
	}
	if !strings.HasPrefix(plain, apiTokenPrefix) {
		t.Errorf("plain = %q, want %q prefix", plain, apiTokenPrefix)
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if HashToken(plain) != hash {
		t.Error("HashToken(plain) should match the returned hash")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	plain, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	if !strings.HasPrefix(plain, refreshTokenPrefix) {
		t.Errorf("plain = %q, want %q prefix", plain, refreshTokenPrefix)
	}
	if strings.HasPrefix(plain, apiTokenPrefix) {
		t.Error("refresh token should not have api token prefix")
	}
	if HashToken(plain) != hash {
		t.Error("HashToken(plain) should match the returned hash")
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	h1 := HashToken("test-token-value")
	h2 := HashToken("test-token-value")
	if h1 != h2 {
		t.Error("HashToken should be deterministic")
	}
}

func TestHashTokenDifferent(t *testing.T) {
	h1 := HashToken("token-a")
	h2 := HashToken("token-b")
	if h1 == h2 {
		t.Error("different tokens should produce different hashes")
	}
}

func TestIsAPIToken(t *testing.T) {
	if !IsAPIToken(apiTokenPrefix + "abc123") {
		t.Error("should detect api token")
	}
	if IsAPIToken("not-a-token") {
		t.Error("should not detect non-token")
	}
	if IsAPIToken(apiTokenPrefix) {
		t.Error("prefix alone should not be a token")
	}
}

func TestTokenUniqueness(t *testing.T) {
	p1, _, _ := GenerateAPIToken()
	p2, _, _ := GenerateAPIToken()
	if p1 == p2 {
		t.Error("tokens should be unique")
	}
}
