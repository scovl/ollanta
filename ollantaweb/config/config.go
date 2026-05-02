// Package config loads ollantaweb server configuration from config.toml and environment variables.
// All fields have sensible defaults; database connectivity remains required.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scovl/ollanta/ollantacore/configfile"
)

type fileConfig struct {
	Server   serverFileConfig   `toml:"server"`
	Database databaseFileConfig `toml:"database"`
	Search   searchFileConfig   `toml:"search"`
	UI       uiFileConfig       `toml:"ui"`
}

type serverFileConfig struct {
	Addr               string `toml:"addr"`
	AdminAddr          string `toml:"admin_addr"`
	LogLevel           string `toml:"log_level"`
	JWTSecret          string `toml:"jwt_secret"`
	JWTExpiry          string `toml:"jwt_expiry"`
	RefreshExpiry      string `toml:"refresh_expiry"`
	OAuthRedirectBase  string `toml:"oauth_redirect_base"`
	GitHubClientID     string `toml:"github_client_id"`
	GitHubClientSecret string `toml:"github_client_secret"`
	GitLabClientID     string `toml:"gitlab_client_id"`
	GitLabClientSecret string `toml:"gitlab_client_secret"`
	GoogleClientID     string `toml:"google_client_id"`
	GoogleClientSecret string `toml:"google_client_secret"`
	ScannerToken       string `toml:"scanner_token"`
}

type databaseFileConfig struct {
	URL      string `toml:"url"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Name     string `toml:"name"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	SSLMode  string `toml:"sslmode"`
}

type searchFileConfig struct {
	Backend  string `toml:"backend"`
	URL      string `toml:"url"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
}

type uiFileConfig struct {
	ObservabilityLinks []ObservabilityLink `toml:"observability_links"`
}

// ObservabilityLink describes an optional external observability tool link shown in the web UI.
type ObservabilityLink struct {
	Label string `toml:"label" json:"label"`
	URL   string `toml:"url" json:"url"`
}

// Config holds all runtime configuration for the ollantaweb server.
type Config struct {
	// Addr is the TCP address the HTTP server listens on (e.g. ":8080").
	Addr string

	// AdminAddr is the TCP address exposed by long-running worker roles for health and metrics.
	AdminAddr string

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

	// ObservabilityLinks are optional external links shown in the admin navigation.
	ObservabilityLinks []ObservabilityLink
}

// Load reads configuration from config.toml and environment variables, then validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:               ":8080",
		AdminAddr:          ":9090",
		ZincSearchURL:      "http://localhost:4080",
		ZincSearchUser:     "admin",
		ZincSearchPassword: "admin",
		SearchBackend:      "zincsearch",
		LogLevel:           "info",
		JWTExpiry:          15 * time.Minute,
		RefreshExpiry:      30 * 24 * time.Hour,
	}

	var fileCfg fileConfig
	if _, found, err := configfile.Load(os.Getenv("OLLANTA_CONFIG_FILE"), &fileCfg); err != nil {
		return nil, err
	} else if found {
		if err := applyFileConfig(cfg, fileCfg); err != nil {
			return nil, err
		}
	}

	if err := applyServerEnvOverrides(cfg); err != nil {
		return nil, err
	}

	if cfg.JWTSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, errors.New("could not generate JWT secret")
		}
		cfg.JWTSecret = hex.EncodeToString(b)
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("database url is required (set [database].url, explicit [database] fields, or OLLANTA_DATABASE_URL)")
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

func parseDuration(s string, fallback time.Duration) (time.Duration, error) {
	if s == "" {
		return fallback, nil
	}
	return time.ParseDuration(s)
}

func applyFileConfig(cfg *Config, file fileConfig) error {
	if err := applyServerFileConfig(cfg, file.Server); err != nil {
		return err
	}
	applyDatabaseFileConfig(cfg, file.Database)
	applySearchFileConfig(cfg, file.Search)
	if err := applyUIFileConfig(cfg, file.UI); err != nil {
		return err
	}
	return nil
}

func applyServerFileConfig(cfg *Config, file serverFileConfig) error {
	applyStringValue(&cfg.Addr, file.Addr)
	applyStringValue(&cfg.AdminAddr, file.AdminAddr)
	applyStringValue(&cfg.LogLevel, file.LogLevel)
	applyStringValue(&cfg.JWTSecret, file.JWTSecret)
	applyStringValue(&cfg.OAuthRedirectBase, file.OAuthRedirectBase)
	applyStringValue(&cfg.GitHubClientID, file.GitHubClientID)
	applyStringValue(&cfg.GitHubClientSecret, file.GitHubClientSecret)
	applyStringValue(&cfg.GitLabClientID, file.GitLabClientID)
	applyStringValue(&cfg.GitLabClientSecret, file.GitLabClientSecret)
	applyStringValue(&cfg.GoogleClientID, file.GoogleClientID)
	applyStringValue(&cfg.GoogleClientSecret, file.GoogleClientSecret)
	applyStringValue(&cfg.ScannerToken, file.ScannerToken)
	if err := applyDurationValue(&cfg.JWTExpiry, file.JWTExpiry, "server.jwt_expiry"); err != nil {
		return err
	}
	if err := applyDurationValue(&cfg.RefreshExpiry, file.RefreshExpiry, "server.refresh_expiry"); err != nil {
		return err
	}
	return nil
}

func applyDatabaseFileConfig(cfg *Config, file databaseFileConfig) {
	if file.URL != "" {
		cfg.DatabaseURL = file.URL
		return
	}

	if !hasDatabaseParts(file) {
		return
	}

	cfg.DatabaseURL = buildDatabaseURL(file)
}

func applySearchFileConfig(cfg *Config, file searchFileConfig) {
	applyStringValue(&cfg.SearchBackend, file.Backend)
	if file.URL != "" {
		cfg.ZincSearchURL = file.URL
	} else if file.Host != "" {
		cfg.ZincSearchURL = buildSearchURL(file)
	}
	applyStringValue(&cfg.ZincSearchUser, file.User)
	applyStringValue(&cfg.ZincSearchPassword, file.Password)
}

func applyUIFileConfig(cfg *Config, file uiFileConfig) error {
	links, err := validateObservabilityLinks(file.ObservabilityLinks)
	if err != nil {
		return err
	}
	cfg.ObservabilityLinks = links
	return nil
}

func hasDatabaseParts(file databaseFileConfig) bool {
	return file.Host != "" || file.Port != 0 || file.Name != "" || file.User != "" || file.Password != "" || file.SSLMode != ""
}

func buildDatabaseURL(file databaseFileConfig) string {
	port := file.Port
	if port == 0 {
		port = 5432
	}
	sslMode := file.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	databaseURL := &url.URL{
		Scheme:   "postgres",
		Host:     net.JoinHostPort(file.Host, strconv.Itoa(port)),
		Path:     file.Name,
		RawQuery: "sslmode=" + url.QueryEscape(sslMode),
	}
	if file.User != "" {
		if file.Password != "" {
			databaseURL.User = url.UserPassword(file.User, file.Password)
		} else {
			databaseURL.User = url.User(file.User)
		}
	}
	return databaseURL.String()
}

func buildSearchURL(file searchFileConfig) string {
	port := file.Port
	if port == 0 {
		port = 4080
	}
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(file.Host, strconv.Itoa(port)),
	}).String()
}

func applyServerEnvOverrides(cfg *Config) error {
	applyEnvStringValue(&cfg.Addr, "OLLANTA_ADDR")
	applyEnvStringValue(&cfg.AdminAddr, "OLLANTA_ADMIN_ADDR")
	applyEnvStringValue(&cfg.DatabaseURL, "OLLANTA_DATABASE_URL")
	applyEnvStringValue(&cfg.ZincSearchURL, "OLLANTA_ZINCSEARCH_URL")
	applyEnvStringValue(&cfg.ZincSearchUser, "OLLANTA_ZINCSEARCH_USER")
	applyEnvStringValue(&cfg.ZincSearchPassword, "OLLANTA_ZINCSEARCH_PASSWORD")
	applyEnvStringValue(&cfg.SearchBackend, "OLLANTA_SEARCH_BACKEND")
	applyEnvStringValue(&cfg.LogLevel, "OLLANTA_LOG_LEVEL")
	applyEnvStringValue(&cfg.JWTSecret, "OLLANTA_JWT_SECRET")
	applyEnvStringValue(&cfg.OAuthRedirectBase, "OLLANTA_OAUTH_REDIRECT_BASE")
	applyEnvStringValue(&cfg.GitHubClientID, "OLLANTA_GITHUB_CLIENT_ID")
	applyEnvStringValue(&cfg.GitHubClientSecret, "OLLANTA_GITHUB_CLIENT_SECRET")
	applyEnvStringValue(&cfg.GitLabClientID, "OLLANTA_GITLAB_CLIENT_ID")
	applyEnvStringValue(&cfg.GitLabClientSecret, "OLLANTA_GITLAB_CLIENT_SECRET")
	applyEnvStringValue(&cfg.GoogleClientID, "OLLANTA_GOOGLE_CLIENT_ID")
	applyEnvStringValue(&cfg.GoogleClientSecret, "OLLANTA_GOOGLE_CLIENT_SECRET")
	applyEnvStringValue(&cfg.ScannerToken, "OLLANTA_SCANNER_TOKEN")
	if err := applyEnvObservabilityLinks(cfg, "OLLANTA_OBSERVABILITY_LINKS"); err != nil {
		return err
	}
	if err := applyEnvDurationValue(&cfg.JWTExpiry, "OLLANTA_JWT_EXPIRY"); err != nil {
		return errors.New("invalid OLLANTA_JWT_EXPIRY")
	}
	if err := applyEnvDurationValue(&cfg.RefreshExpiry, "OLLANTA_REFRESH_EXPIRY"); err != nil {
		return errors.New("invalid OLLANTA_REFRESH_EXPIRY")
	}
	return nil
}

func applyEnvObservabilityLinks(cfg *Config, key string) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	links, err := parseObservabilityLinks(value)
	if err != nil {
		return errors.New("invalid " + key)
	}
	cfg.ObservabilityLinks = links
	return nil
}

func parseObservabilityLinks(value string) ([]ObservabilityLink, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ";")
	links := make([]ObservabilityLink, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		label, linkURL, found := strings.Cut(part, "=")
		if !found {
			return nil, errors.New("missing label or url")
		}
		links = append(links, ObservabilityLink{Label: strings.TrimSpace(label), URL: strings.TrimSpace(linkURL)})
	}
	return validateObservabilityLinks(links)
}

func validateObservabilityLinks(links []ObservabilityLink) ([]ObservabilityLink, error) {
	if len(links) == 0 {
		return nil, nil
	}
	valid := make([]ObservabilityLink, 0, len(links))
	for _, link := range links {
		label := strings.TrimSpace(link.Label)
		linkURL := strings.TrimSpace(link.URL)
		if label == "" || linkURL == "" {
			return nil, errors.New("observability links require label and url")
		}
		parsed, err := url.Parse(linkURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, errors.New("observability links require absolute urls")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, errors.New("observability links support http and https urls")
		}
		valid = append(valid, ObservabilityLink{Label: label, URL: linkURL})
	}
	return valid, nil
}

func applyStringValue(dst *string, value string) {
	if value == "" {
		return
	}
	*dst = value
}

func applyDurationValue(dst *time.Duration, value, label string) error {
	if value == "" {
		return nil
	}
	duration, err := parseDuration(value, *dst)
	if err != nil {
		return errors.New("invalid " + label)
	}
	*dst = duration
	return nil
}

func applyEnvStringValue(dst *string, key string) {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		*dst = value
	}
}

func applyEnvDurationValue(dst *time.Duration, key string) error {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return nil
	}
	duration, err := parseDuration(value, *dst)
	if err != nil {
		return err
	}
	*dst = duration
	return nil
}
