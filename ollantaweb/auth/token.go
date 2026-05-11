package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const (
	apiTokenPrefix     = "olt_"
	refreshTokenPrefix = "ort_"
	tokenBytes         = 32
)

// GenerateAPIToken returns a new random API token (plain text with olt_ prefix)
// and its SHA-256 hex digest for storage.
func GenerateAPIToken() (plain, hash string, err error) {
	b := make([]byte, tokenBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate api token: %w", err)
	}
	plain = apiTokenPrefix + hex.EncodeToString(b)
	hash = HashToken(plain)
	return plain, hash, nil
}

// GenerateRefreshToken returns a new random refresh token (plain text with ort_ prefix)
// and its SHA-256 hex digest for storage.
func GenerateRefreshToken() (plain, hash string, err error) {
	b := make([]byte, tokenBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	plain = refreshTokenPrefix + hex.EncodeToString(b)
	hash = HashToken(plain)
	return plain, hash, nil
}

// HashToken returns the SHA-256 hex digest of a token string.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// IsAPIToken reports whether the string looks like an Ollanta API token.
func IsAPIToken(s string) bool {
	return len(s) > len(apiTokenPrefix) && s[:len(apiTokenPrefix)] == apiTokenPrefix
}
