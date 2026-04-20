// Package config loads ollantaweb server configuration from environment variables.
// All fields have sensible defaults; only OLLANTA_DATABASE_URL is required.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"
)

// Config holds all runtime configuration for the ollantaweb server.
type Config struct {
	// Addr is the TCP address the HTTP server listens on (e.g. ":8080").
	Addr string

	// DatabaseURL is the PostgreSQL connection string.
	// Required. Format: postgres://user:pass@host:5432/dbname?sslmode=disable
	DatabaseURL string

	// ZincSearchURL is the base URL of the ZincSearch instance.
	ZincSearchURL string

	// ZincSearchUser is the ZincSearch admin username.
	ZincSearchUser string

	// ZincSearchPassword is the ZincSearch admin password.
	ZincSearchPassword string

	// SearchBackend selects the search engine: "zincsearch" (default), "postgres".
	SearchBackend string

	// IndexCoordinator selects the index job coordinator: "memory" (default) or "pgnotify".
	// "pgnotify" uses Postgres LISTEN/NOTIFY for multi-replica coordination.
	IndexCoordinator string

	// LogLevel controls log verbosity ("debug", "info", "warn", "error").
	LogLevel string

	// JWTSecret is the HMAC-SHA256 signing key for access tokens.
	// If not set, a random secret is generated at startup (tokens won't survive restarts).
	JWTSecret string

	// JWTExpiry is the lifetime of access tokens.
	JWTExpiry time.Duration

	// RefreshExpiry is the lifetime of refresh tokens.
	RefreshExpiry time.Duration

	// OAuthRedirectBase is the external base URL used for OAuth callback URLs.
	OAuthRedirectBase string

	// GitHub OAuth credentials. If empty, GitHub login is disabled.
	GitHubClientID     string
	GitHubClientSecret string

	// GitLab OAuth credentials. If empty, GitLab login is disabled.
	GitLabClientID     string
	GitLabClientSecret string

	// GoogleOAuth credentials. If empty, Google login is disabled.
	GoogleClientID     string
	GoogleClientSecret string

	// ScannerToken is a pre-shared key accepted for POST /api/v1/scans.
	// If empty, scanner push requires a regular JWT or API token.
	ScannerToken string
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	jwtSecret := os.Getenv("OLLANTA_JWT_SECRET")
	if jwtSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, errors.New("could not generate JWT secret")
		}
		jwtSecret = hex.EncodeToString(b)
	}

	jwtExpiry, err := parseDuration(os.Getenv("OLLANTA_JWT_EXPIRY"), 15*time.Minute)
	if err != nil {
		return nil, errors.New("invalid OLLANTA_JWT_EXPIRY")
	}
	refreshExpiry, err := parseDuration(os.Getenv("OLLANTA_REFRESH_EXPIRY"), 30*24*time.Hour)
	if err != nil {
		return nil, errors.New("invalid OLLANTA_REFRESH_EXPIRY")
	}

	cfg := &Config{
		Addr:              envOr("OLLANTA_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("OLLANTA_DATABASE_URL"),
		ZincSearchURL:      envOr("OLLANTA_ZINCSEARCH_URL", "http://localhost:4080"),
		ZincSearchUser:     envOr("OLLANTA_ZINCSEARCH_USER", "admin"),
		ZincSearchPassword: envOr("OLLANTA_ZINCSEARCH_PASSWORD", "admin"),
		SearchBackend:      envOr("OLLANTA_SEARCH_BACKEND", "zincsearch"),
		IndexCoordinator:  envOr("OLLANTA_INDEX_COORDINATOR", "memory"),
		LogLevel:          envOr("OLLANTA_LOG_LEVEL", "info"),
		JWTSecret:         jwtSecret,
		JWTExpiry:         jwtExpiry,
		RefreshExpiry:     refreshExpiry,
		OAuthRedirectBase: os.Getenv("OLLANTA_OAUTH_REDIRECT_BASE"),
		GitHubClientID:     os.Getenv("OLLANTA_GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("OLLANTA_GITHUB_CLIENT_SECRET"),
		GitLabClientID:     os.Getenv("OLLANTA_GITLAB_CLIENT_ID"),
		GitLabClientSecret: os.Getenv("OLLANTA_GITLAB_CLIENT_SECRET"),
		GoogleClientID:     os.Getenv("OLLANTA_GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("OLLANTA_GOOGLE_CLIENT_SECRET"),
		ScannerToken:       os.Getenv("OLLANTA_SCANNER_TOKEN"),
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("OLLANTA_DATABASE_URL is required")
	}
	return cfg, nil
}

// MustLoad calls Load and panics on error. For use in main().
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic("ollantaweb config: " + err.Error())
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(s string, fallback time.Duration) (time.Duration, error) {
	if s == "" {
		return fallback, nil
	}
	return time.ParseDuration(s)
}
