// Package config loads ollantaweb server configuration from environment variables.
// All fields have sensible defaults; only OLLANTA_DATABASE_URL is required.
package config

import (
	"errors"
	"os"
)

// Config holds all runtime configuration for the ollantaweb server.
type Config struct {
	// Addr is the TCP address the HTTP server listens on (e.g. ":8080").
	Addr string

	// DatabaseURL is the PostgreSQL connection string.
	// Required. Format: postgres://user:pass@host:5432/dbname?sslmode=disable
	DatabaseURL string

	// MeilisearchURL is the base URL of the Meilisearch instance.
	MeilisearchURL string

	// MeilisearchAPIKey is the Meilisearch master key (empty for local dev).
	MeilisearchAPIKey string

	// LogLevel controls log verbosity ("debug", "info", "warn", "error").
	LogLevel string
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:              envOr("OLLANTA_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("OLLANTA_DATABASE_URL"),
		MeilisearchURL:    envOr("OLLANTA_MEILISEARCH_URL", "http://localhost:7700"),
		MeilisearchAPIKey: os.Getenv("OLLANTA_MEILISEARCH_KEY"),
		LogLevel:          envOr("OLLANTA_LOG_LEVEL", "info"),
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
