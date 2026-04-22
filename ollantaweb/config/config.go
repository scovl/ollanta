// Package config loads ollantaweb runtime configuration from environment variables
// and an optional shared TOML file.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
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

type fileConfig struct {
	Server struct {
		Addr              string `toml:"addr"`
		Host              string `toml:"host"`
		Port              int    `toml:"port"`
		PublicURL         string `toml:"public_url"`
		DatabaseURL       string `toml:"database_url"`
		SearchBackend     string `toml:"search_backend"`
		LogLevel          string `toml:"log_level"`
		JWTSecret         string `toml:"jwt_secret"`
		JWTExpiry         string `toml:"jwt_expiry"`
		RefreshExpiry     string `toml:"refresh_expiry"`
		OAuthRedirectBase string `toml:"oauth_redirect_base"`
		ScannerToken      string `toml:"scanner_token"`
	} `toml:"server"`
	Database struct {
		URL      string `toml:"url"`
		Host     string `toml:"host"`
		Port     int    `toml:"port"`
		Name     string `toml:"name"`
		User     string `toml:"user"`
		Password string `toml:"password"`
		SSLMode  string `toml:"sslmode"`
	} `toml:"database"`
	Search struct {
		URL      string `toml:"url"`
		Scheme   string `toml:"scheme"`
		Host     string `toml:"host"`
		Port     int    `toml:"port"`
		User     string `toml:"user"`
		Password string `toml:"password"`
		Backend  string `toml:"backend"`
	} `toml:"search"`
	ZincSearch struct {
		URL      string `toml:"url"`
		User     string `toml:"user"`
		Password string `toml:"password"`
	} `toml:"zincsearch"`
	OAuth struct {
		GitHub struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
		} `toml:"github"`
		GitLab struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
		} `toml:"gitlab"`
		Google struct {
			ClientID     string `toml:"client_id"`
			ClientSecret string `toml:"client_secret"`
		} `toml:"google"`
	} `toml:"oauth"`
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	fileCfg, err := loadFileConfig(os.Getenv("OLLANTA_CONFIG_FILE"))
	if err != nil {
		return nil, err
	}

	jwtSecret := envOrFile("OLLANTA_JWT_SECRET", fileCfg.Server.JWTSecret)
	if jwtSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, errors.New("could not generate JWT secret")
		}
		jwtSecret = hex.EncodeToString(b)
	}

	jwtExpiry, err := parseDuration(envOrFile("OLLANTA_JWT_EXPIRY", fileCfg.Server.JWTExpiry), 15*time.Minute)
	if err != nil {
		return nil, errors.New("invalid OLLANTA_JWT_EXPIRY")
	}
	refreshExpiry, err := parseDuration(envOrFile("OLLANTA_REFRESH_EXPIRY", fileCfg.Server.RefreshExpiry), 30*24*time.Hour)
	if err != nil {
		return nil, errors.New("invalid OLLANTA_REFRESH_EXPIRY")
	}

	addr := resolveServerAddr(fileCfg.Server.Addr, fileCfg.Server.Host, fileCfg.Server.Port)
	databaseURL := firstNonEmpty(fileCfg.Server.DatabaseURL, fileCfg.Database.URL, buildDatabaseURL(fileCfg.Database))
	zincURL := firstNonEmpty(fileCfg.ZincSearch.URL, fileCfg.Search.URL, buildHTTPURL(fileCfg.Search.Scheme, fileCfg.Search.Host, fileCfg.Search.Port))
	zincUser := firstNonEmpty(fileCfg.ZincSearch.User, fileCfg.Search.User)
	zincPassword := firstNonEmpty(fileCfg.ZincSearch.Password, fileCfg.Search.Password)
	searchBackend := firstNonEmpty(fileCfg.Server.SearchBackend, fileCfg.Search.Backend)

	cfg := &Config{
		Addr:               envOrFileOr("OLLANTA_ADDR", addr, ":8080"),
		DatabaseURL:        envOrFile("OLLANTA_DATABASE_URL", databaseURL),
		ZincSearchURL:      envOrFileOr("OLLANTA_ZINCSEARCH_URL", zincURL, "http://localhost:4080"),
		ZincSearchUser:     envOrFileOr("OLLANTA_ZINCSEARCH_USER", zincUser, "admin"),
		ZincSearchPassword: envOrFileOr("OLLANTA_ZINCSEARCH_PASSWORD", zincPassword, "admin"),
		SearchBackend:      envOrFileOr("OLLANTA_SEARCH_BACKEND", searchBackend, "zincsearch"),
		LogLevel:           envOrFileOr("OLLANTA_LOG_LEVEL", fileCfg.Server.LogLevel, "info"),
		JWTSecret:          jwtSecret,
		JWTExpiry:          jwtExpiry,
		RefreshExpiry:      refreshExpiry,
		OAuthRedirectBase:  envOrFile("OLLANTA_OAUTH_REDIRECT_BASE", fileCfg.Server.OAuthRedirectBase),
		GitHubClientID:     envOrFile("OLLANTA_GITHUB_CLIENT_ID", fileCfg.OAuth.GitHub.ClientID),
		GitHubClientSecret: envOrFile("OLLANTA_GITHUB_CLIENT_SECRET", fileCfg.OAuth.GitHub.ClientSecret),
		GitLabClientID:     envOrFile("OLLANTA_GITLAB_CLIENT_ID", fileCfg.OAuth.GitLab.ClientID),
		GitLabClientSecret: envOrFile("OLLANTA_GITLAB_CLIENT_SECRET", fileCfg.OAuth.GitLab.ClientSecret),
		GoogleClientID:     envOrFile("OLLANTA_GOOGLE_CLIENT_ID", fileCfg.OAuth.Google.ClientID),
		GoogleClientSecret: envOrFile("OLLANTA_GOOGLE_CLIENT_SECRET", fileCfg.OAuth.Google.ClientSecret),
		ScannerToken:       envOrFile("OLLANTA_SCANNER_TOKEN", fileCfg.Server.ScannerToken),
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

func loadFileConfig(path string) (*fileConfig, error) {
	cfg := &fileConfig{}
	if path == "" {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("load ollantaweb config %q: %w", path, err)
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrFile(key, fileValue string) string {
	return envOr(key, fileValue)
}

func envOrFileOr(key, fileValue, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if fileValue != "" {
		return fileValue
	}
	return fallback
}

func parseDuration(s string, fallback time.Duration) (time.Duration, error) {
	if s == "" {
		return fallback, nil
	}
	return time.ParseDuration(s)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func resolveServerAddr(addr, host string, port int) string {
	if strings.TrimSpace(addr) != "" {
		return addr
	}
	if port <= 0 {
		return ""
	}
	if strings.TrimSpace(host) == "" {
		return ":" + strconv.Itoa(port)
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func buildHTTPURL(scheme, host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" || port <= 0 {
		return ""
	}
	scheme = strings.TrimSpace(scheme)
	if scheme == "" {
		scheme = "http"
	}
	return (&url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
	}).String()
}

func buildDatabaseURL(cfg struct {
	URL      string `toml:"url"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Name     string `toml:"name"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	SSLMode  string `toml:"sslmode"`
}) string {
	host := strings.TrimSpace(cfg.Host)
	userName := strings.TrimSpace(cfg.User)
	databaseName := strings.TrimSpace(cfg.Name)
	if host == "" || userName == "" || databaseName == "" {
		return ""
	}
	port := cfg.Port
	if port <= 0 {
		port = 5432
	}
	password := strings.TrimSpace(cfg.Password)
	user := url.User(userName)
	if password != "" {
		user = url.UserPassword(userName, password)
	}
	sslMode := strings.TrimSpace(cfg.SSLMode)
	if sslMode == "" {
		sslMode = "disable"
	}
	return (&url.URL{
		Scheme:   "postgres",
		User:     user,
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		Path:     "/" + strings.TrimPrefix(databaseName, "/"),
		RawQuery: "sslmode=" + url.QueryEscape(sslMode),
	}).String()
}
