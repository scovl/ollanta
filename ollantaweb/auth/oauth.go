package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// OAuthUser holds normalized user data returned by any OAuth provider.
type OAuthUser struct {
	ProviderID string
	Login      string
	Email      string
	Name       string
	AvatarURL  string
}

// OAuthProvider is implemented by each supported OAuth provider.
type OAuthProvider interface {
	// AuthURL returns the authorization URL the browser should be redirected to.
	AuthURL(state string) string
	// Exchange takes the callback code and returns the authenticated user's profile.
	Exchange(ctx context.Context, code string) (*OAuthUser, error)
}

// GenerateOAuthState returns a random hex string suitable for use as CSRF state.
func GenerateOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
