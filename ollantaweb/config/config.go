// Package config loads ollantaweb server configuration from config.toml and environment variables.
// All fields have sensible defaults; database connectivity remains required.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scovl/ollanta/ollantacore/configfile"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

type fileConfig struct {
	Server   serverFileConfig   `toml:"server"`
	Database databaseFileConfig `toml:"database"`
	Search   searchFileConfig   `toml:"search"`
	UI       uiFileConfig       `toml:"ui"`
}

type serverFileConfig struct {
	Addr                       string   `toml:"addr"`
	AdminAddr                  string   `toml:"admin_addr"`
	LogLevel                   string   `toml:"log_level"`
	JWTSecret                  string   `toml:"jwt_secret"`
	AllowRandomJWTSecret       bool     `toml:"allow_random_jwt_secret"`
	JWTExpiry                  string   `toml:"jwt_expiry"`
	RefreshExpiry              string   `toml:"refresh_expiry"`
	OAuthRedirectBase          string   `toml:"oauth_redirect_base"`
	GitHubClientID             string   `toml:"github_client_id"`
	GitHubClientSecret         string   `toml:"github_client_secret"`
	GitLabClientID             string   `toml:"gitlab_client_id"`
	GitLabClientSecret         string   `toml:"gitlab_client_secret"`
	GoogleClientID             string   `toml:"google_client_id"`
	GoogleClientSecret         string   `toml:"google_client_secret"`
	ScannerToken               string   `toml:"scanner_token"`
	CORSAllowedOrigins         []string `toml:"cors_allowed_origins"`
	CORSAllowUnsafeWildcard    bool     `toml:"cors_allow_unsafe_wildcard"`
	HTTPMaxBodyBytes           int64    `toml:"http_max_body_bytes"`
	AutoMigrate                *bool    `toml:"auto_migrate"`
	ScanQueueMaxAccepted       int      `toml:"scan_queue_max_accepted"`
	ScanQueueMaxRunning        int      `toml:"scan_queue_max_running"`
	ScanQueueMaxAge            string   `toml:"scan_queue_max_oldest_accepted_age"`
	ScanQueueRetryAfter        string   `toml:"scan_queue_retry_after"`
	ScanJobStaleAfter          string   `toml:"scan_job_stale_after"`
	ScanJobMaxAttempts         int      `toml:"scan_job_max_attempts"`
	ScanJobRecoveryInterval    string   `toml:"scan_job_recovery_interval"`
	IndexJobStaleAfter         string   `toml:"index_job_stale_after"`
	IndexJobMaxAttempts        int      `toml:"index_job_max_attempts"`
	IndexJobRecoveryInterval   string   `toml:"index_job_recovery_interval"`
	WebhookJobStaleAfter       string   `toml:"webhook_job_stale_after"`
	WebhookJobMaxAttempts      int      `toml:"webhook_job_max_attempts"`
	WebhookJobRecoveryInterval string   `toml:"webhook_job_recovery_interval"`
}

type databaseFileConfig struct {
	URL                 string `toml:"url"`
	Host                string `toml:"host"`
	Port                int    `toml:"port"`
	Name                string `toml:"name"`
	User                string `toml:"user"`
	Password            string `toml:"password"`
	SSLMode             string `toml:"sslmode"`
	PoolMaxConns        int    `toml:"pool_max_conns"`
	PoolMinConns        int    `toml:"pool_min_conns"`
	PoolMaxConnLifetime string `toml:"pool_max_conn_lifetime"`
	PoolMaxConnIdleTime string `toml:"pool_max_conn_idle_time"`
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

// JobRecoveryConfig controls automatic stale durable job recovery for one job type.
type JobRecoveryConfig struct {
	StaleAfter  time.Duration
	MaxAttempts int
	Interval    time.Duration
}

const invalidConfigValuePrefix = "invalid "

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
	// Required unless AllowRandomJWTSecret is enabled for local development.
	JWTSecret string

	// AllowRandomJWTSecret permits generating an unsafe process-local secret for development.
	AllowRandomJWTSecret bool

	// RandomJWTSecretGenerated is true when JWTSecret was generated by the development opt-in.
	RandomJWTSecretGenerated bool

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

	// CORSAllowedOrigins is the allowlist used for cross-origin browser requests.
	CORSAllowedOrigins []string

	// CORSAllowUnsafeWildcard permits "*" in CORSAllowedOrigins for development only.
	CORSAllowUnsafeWildcard bool

	// HTTPMaxBodyBytes limits request bodies accepted by the HTTP router.
	HTTPMaxBodyBytes int64

	// PostgresPool controls PostgreSQL connection pool sizing and lifetimes.
	PostgresPool postgres.PoolConfig

	// AutoMigrate controls whether API and worker roles apply migrations during startup.
	AutoMigrate bool

	// ScanQueueMaxAccepted rejects new intake when accepted scan jobs reach this limit. Zero disables the limit.
	ScanQueueMaxAccepted int

	// ScanQueueMaxRunning rejects new intake when running scan jobs reach this limit. Zero disables the limit.
	ScanQueueMaxRunning int

	// ScanQueueMaxOldestAcceptedAge rejects intake when the oldest accepted job exceeds this age. Zero disables the limit.
	ScanQueueMaxOldestAcceptedAge time.Duration

	// ScanQueueRetryAfter is returned as Retry-After when intake backpressure rejects a request.
	ScanQueueRetryAfter time.Duration

	// ScanJobRecovery controls stale scan job requeue/fail behavior.
	ScanJobRecovery JobRecoveryConfig

	// IndexJobRecovery controls stale index job requeue/fail behavior.
	IndexJobRecovery JobRecoveryConfig

	// WebhookJobRecovery controls stale webhook job requeue/fail behavior.
	WebhookJobRecovery JobRecoveryConfig

	// ObservabilityLinks are optional external links shown in the admin navigation.
	ObservabilityLinks []ObservabilityLink
}

// Load reads configuration from config.toml and environment variables, then validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:                ":8080",
		AdminAddr:           ":9090",
		ZincSearchURL:       "http://localhost:4080",
		ZincSearchUser:      "admin",
		ZincSearchPassword:  "admin",
		SearchBackend:       "zincsearch",
		LogLevel:            "info",
		JWTExpiry:           15 * time.Minute,
		RefreshExpiry:       30 * 24 * time.Hour,
		HTTPMaxBodyBytes:    10 << 20,
		PostgresPool:        postgres.DefaultPoolConfig(),
		AutoMigrate:         true,
		ScanQueueRetryAfter: 30 * time.Second,
		ScanJobRecovery:     defaultJobRecoveryConfig(),
		IndexJobRecovery:    defaultJobRecoveryConfig(),
		WebhookJobRecovery:  defaultJobRecoveryConfig(),
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

	if cfg.DatabaseURL == "" {
		return nil, errors.New("database url is required (set [database].url, explicit [database] fields, or OLLANTA_DATABASE_URL)")
	}
	if err := validateRuntimeConfig(cfg); err != nil {
		return nil, err
	}

	if cfg.JWTSecret == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, errors.New("could not generate JWT secret")
		}
		cfg.JWTSecret = hex.EncodeToString(b)
		cfg.RandomJWTSecretGenerated = true
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

// LogStartupWarnings emits warnings for explicit development-only configuration.
func (cfg *Config) LogStartupWarnings() {
	if cfg.RandomJWTSecretGenerated {
		slog.Warn("generated random JWT secret for development; set OLLANTA_JWT_SECRET for production and multi-replica deployments")
	}
	if cfg.CORSAllowUnsafeWildcard && containsWildcardOrigin(cfg.CORSAllowedOrigins) {
		slog.Warn("unsafe wildcard CORS origin enabled; configure OLLANTA_CORS_ALLOWED_ORIGINS explicitly for production")
	}
}

func parseDuration(s string, fallback time.Duration) (time.Duration, error) {
	if s == "" {
		return fallback, nil
	}
	return time.ParseDuration(s)
}

func defaultJobRecoveryConfig() JobRecoveryConfig {
	return JobRecoveryConfig{
		StaleAfter:  15 * time.Minute,
		MaxAttempts: 3,
		Interval:    time.Minute,
	}
}

func applyFileConfig(cfg *Config, file fileConfig) error {
	if err := applyServerFileConfig(cfg, file.Server); err != nil {
		return err
	}
	if err := applyDatabaseFileConfig(cfg, file.Database); err != nil {
		return err
	}
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
	if file.AllowRandomJWTSecret {
		cfg.AllowRandomJWTSecret = true
	}
	applyStringValue(&cfg.OAuthRedirectBase, file.OAuthRedirectBase)
	applyStringValue(&cfg.GitHubClientID, file.GitHubClientID)
	applyStringValue(&cfg.GitHubClientSecret, file.GitHubClientSecret)
	applyStringValue(&cfg.GitLabClientID, file.GitLabClientID)
	applyStringValue(&cfg.GitLabClientSecret, file.GitLabClientSecret)
	applyStringValue(&cfg.GoogleClientID, file.GoogleClientID)
	applyStringValue(&cfg.GoogleClientSecret, file.GoogleClientSecret)
	applyStringValue(&cfg.ScannerToken, file.ScannerToken)
	applyStringListValue(&cfg.CORSAllowedOrigins, file.CORSAllowedOrigins)
	if file.CORSAllowUnsafeWildcard {
		cfg.CORSAllowUnsafeWildcard = true
	}
	if file.HTTPMaxBodyBytes != 0 {
		cfg.HTTPMaxBodyBytes = file.HTTPMaxBodyBytes
	}
	if file.AutoMigrate != nil {
		cfg.AutoMigrate = *file.AutoMigrate
	}
	if file.ScanQueueMaxAccepted != 0 {
		cfg.ScanQueueMaxAccepted = file.ScanQueueMaxAccepted
	}
	if file.ScanQueueMaxRunning != 0 {
		cfg.ScanQueueMaxRunning = file.ScanQueueMaxRunning
	}
	if err := applyDurationValue(&cfg.ScanQueueMaxOldestAcceptedAge, file.ScanQueueMaxAge, "server.scan_queue_max_oldest_accepted_age"); err != nil {
		return err
	}
	if err := applyDurationValue(&cfg.ScanQueueRetryAfter, file.ScanQueueRetryAfter, "server.scan_queue_retry_after"); err != nil {
		return err
	}
	if err := applyJobRecoveryFileConfig(&cfg.ScanJobRecovery, file.ScanJobStaleAfter, file.ScanJobMaxAttempts, file.ScanJobRecoveryInterval, "server.scan_job"); err != nil {
		return err
	}
	if err := applyJobRecoveryFileConfig(&cfg.IndexJobRecovery, file.IndexJobStaleAfter, file.IndexJobMaxAttempts, file.IndexJobRecoveryInterval, "server.index_job"); err != nil {
		return err
	}
	if err := applyJobRecoveryFileConfig(&cfg.WebhookJobRecovery, file.WebhookJobStaleAfter, file.WebhookJobMaxAttempts, file.WebhookJobRecoveryInterval, "server.webhook_job"); err != nil {
		return err
	}
	if err := applyDurationValue(&cfg.JWTExpiry, file.JWTExpiry, "server.jwt_expiry"); err != nil {
		return err
	}
	if err := applyDurationValue(&cfg.RefreshExpiry, file.RefreshExpiry, "server.refresh_expiry"); err != nil {
		return err
	}
	return nil
}

func applyJobRecoveryFileConfig(cfg *JobRecoveryConfig, staleAfter string, maxAttempts int, interval string, keyPrefix string) error {
	if err := applyDurationValue(&cfg.StaleAfter, staleAfter, keyPrefix+"_stale_after"); err != nil {
		return err
	}
	if maxAttempts != 0 {
		cfg.MaxAttempts = maxAttempts
	}
	if err := applyDurationValue(&cfg.Interval, interval, keyPrefix+"_recovery_interval"); err != nil {
		return err
	}
	return nil
}

func applyDatabaseFileConfig(cfg *Config, file databaseFileConfig) error {
	applyInt32Value(&cfg.PostgresPool.MaxConns, file.PoolMaxConns)
	applyInt32Value(&cfg.PostgresPool.MinConns, file.PoolMinConns)
	if err := applyDurationValue(&cfg.PostgresPool.MaxConnLifetime, file.PoolMaxConnLifetime, "database.pool_max_conn_lifetime"); err != nil {
		return err
	}
	if err := applyDurationValue(&cfg.PostgresPool.MaxConnIdleTime, file.PoolMaxConnIdleTime, "database.pool_max_conn_idle_time"); err != nil {
		return err
	}

	if file.URL != "" {
		cfg.DatabaseURL = file.URL
		return nil
	}

	if !hasDatabaseParts(file) {
		return nil
	}

	cfg.DatabaseURL = buildDatabaseURL(file)
	return nil
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
	applyAddressEnvOverrides(cfg)
	if err := applyDatabaseEnvOverrides(cfg); err != nil {
		return err
	}
	if err := applySecurityEnvOverrides(cfg); err != nil {
		return err
	}
	if err := applyRuntimeEnvOverrides(cfg); err != nil {
		return err
	}
	if err := applyJobEnvOverrides(cfg); err != nil {
		return err
	}
	if err := applyPostgresPoolEnvOverrides(cfg); err != nil {
		return err
	}
	if err := applyEnvObservabilityLinks(cfg, "OLLANTA_OBSERVABILITY_LINKS"); err != nil {
		return err
	}
	return applyTokenExpiryEnvOverrides(cfg)
}

func applyAddressEnvOverrides(cfg *Config) {
	applyEnvStringValue(&cfg.Addr, "OLLANTA_ADDR")
	applyEnvStringValue(&cfg.AdminAddr, "OLLANTA_ADMIN_ADDR")
	applyEnvStringValue(&cfg.DatabaseURL, "OLLANTA_DATABASE_URL")
	applyEnvStringValue(&cfg.ZincSearchURL, "OLLANTA_ZINCSEARCH_URL")
	applyEnvStringValue(&cfg.ZincSearchUser, "OLLANTA_ZINCSEARCH_USER")
	applyEnvStringValue(&cfg.ZincSearchPassword, "OLLANTA_ZINCSEARCH_PASSWORD")
	applyEnvStringValue(&cfg.SearchBackend, "OLLANTA_SEARCH_BACKEND")
	applyEnvStringValue(&cfg.LogLevel, "OLLANTA_LOG_LEVEL")
	applyEnvStringValue(&cfg.OAuthRedirectBase, "OLLANTA_OAUTH_REDIRECT_BASE")
	applyEnvStringValue(&cfg.GitHubClientID, "OLLANTA_GITHUB_CLIENT_ID")
	applyEnvStringValue(&cfg.GitHubClientSecret, "OLLANTA_GITHUB_CLIENT_SECRET")
	applyEnvStringValue(&cfg.GitLabClientID, "OLLANTA_GITLAB_CLIENT_ID")
	applyEnvStringValue(&cfg.GitLabClientSecret, "OLLANTA_GITLAB_CLIENT_SECRET")
	applyEnvStringValue(&cfg.GoogleClientID, "OLLANTA_GOOGLE_CLIENT_ID")
	applyEnvStringValue(&cfg.GoogleClientSecret, "OLLANTA_GOOGLE_CLIENT_SECRET")
	applyEnvStringValue(&cfg.ScannerToken, "OLLANTA_SCANNER_TOKEN")
}

func applyDatabaseEnvOverrides(cfg *Config) error {
	if strings.TrimSpace(os.Getenv("OLLANTA_DATABASE_URL")) != "" {
		return nil
	}
	file := databaseFileConfig{
		Host:     strings.TrimSpace(os.Getenv("OLLANTA_POSTGRES_HOST")),
		Name:     strings.TrimSpace(os.Getenv("OLLANTA_POSTGRES_DB")),
		User:     strings.TrimSpace(os.Getenv("OLLANTA_POSTGRES_USER")),
		Password: strings.TrimSpace(os.Getenv("OLLANTA_POSTGRES_PASSWORD")),
		SSLMode:  strings.TrimSpace(os.Getenv("OLLANTA_POSTGRES_SSLMODE")),
	}
	if value := strings.TrimSpace(os.Getenv("OLLANTA_POSTGRES_PORT")); value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return errors.New("invalid OLLANTA_POSTGRES_PORT")
		}
		file.Port = port
	}
	if !hasDatabaseParts(file) {
		return nil
	}
	if file.Host == "" {
		file.Host = "localhost"
	}
	if file.Name == "" {
		file.Name = "ollanta"
	}
	if file.User == "" {
		file.User = "ollanta"
	}
	cfg.DatabaseURL = buildDatabaseURL(file)
	return nil
}

func applySecurityEnvOverrides(cfg *Config) error {
	applyEnvStringValue(&cfg.JWTSecret, "OLLANTA_JWT_SECRET")
	if err := applyEnvBoolValue(&cfg.AllowRandomJWTSecret, "OLLANTA_ALLOW_RANDOM_JWT_SECRET"); err != nil {
		return err
	}
	applyEnvStringListValue(&cfg.CORSAllowedOrigins, "OLLANTA_CORS_ALLOWED_ORIGINS")
	return applyEnvBoolValue(&cfg.CORSAllowUnsafeWildcard, "OLLANTA_CORS_ALLOW_UNSAFE_WILDCARD")
}

func applyRuntimeEnvOverrides(cfg *Config) error {
	if err := applyEnvInt64Value(&cfg.HTTPMaxBodyBytes, "OLLANTA_HTTP_MAX_BODY_BYTES"); err != nil {
		return err
	}
	if err := applyEnvBoolValue(&cfg.AutoMigrate, "OLLANTA_AUTO_MIGRATE"); err != nil {
		return err
	}
	if err := applyEnvIntValue(&cfg.ScanQueueMaxAccepted, "OLLANTA_SCAN_QUEUE_MAX_ACCEPTED"); err != nil {
		return err
	}
	if err := applyEnvIntValue(&cfg.ScanQueueMaxRunning, "OLLANTA_SCAN_QUEUE_MAX_RUNNING"); err != nil {
		return err
	}
	if err := applyEnvDurationValue(&cfg.ScanQueueMaxOldestAcceptedAge, "OLLANTA_SCAN_QUEUE_MAX_OLDEST_ACCEPTED_AGE"); err != nil {
		return errors.New("invalid OLLANTA_SCAN_QUEUE_MAX_OLDEST_ACCEPTED_AGE")
	}
	if err := applyEnvDurationValue(&cfg.ScanQueueRetryAfter, "OLLANTA_SCAN_QUEUE_RETRY_AFTER"); err != nil {
		return errors.New("invalid OLLANTA_SCAN_QUEUE_RETRY_AFTER")
	}
	return nil
}

func applyJobEnvOverrides(cfg *Config) error {
	if err := applyJobRecoveryEnvConfig(&cfg.ScanJobRecovery, "OLLANTA_SCAN_JOB"); err != nil {
		return err
	}
	if err := applyJobRecoveryEnvConfig(&cfg.IndexJobRecovery, "OLLANTA_INDEX_JOB"); err != nil {
		return err
	}
	if err := applyJobRecoveryEnvConfig(&cfg.WebhookJobRecovery, "OLLANTA_WEBHOOK_JOB"); err != nil {
		return err
	}
	return nil
}

func applyPostgresPoolEnvOverrides(cfg *Config) error {
	if err := applyEnvInt32Value(&cfg.PostgresPool.MaxConns, "OLLANTA_POSTGRES_MAX_CONNS"); err != nil {
		return err
	}
	if err := applyEnvInt32Value(&cfg.PostgresPool.MinConns, "OLLANTA_POSTGRES_MIN_CONNS"); err != nil {
		return err
	}
	if err := applyEnvDurationValue(&cfg.PostgresPool.MaxConnLifetime, "OLLANTA_POSTGRES_MAX_CONN_LIFETIME"); err != nil {
		return errors.New("invalid OLLANTA_POSTGRES_MAX_CONN_LIFETIME")
	}
	if err := applyEnvDurationValue(&cfg.PostgresPool.MaxConnIdleTime, "OLLANTA_POSTGRES_MAX_CONN_IDLE_TIME"); err != nil {
		return errors.New("invalid OLLANTA_POSTGRES_MAX_CONN_IDLE_TIME")
	}
	return nil
}

func applyTokenExpiryEnvOverrides(cfg *Config) error {
	if err := applyEnvDurationValue(&cfg.JWTExpiry, "OLLANTA_JWT_EXPIRY"); err != nil {
		return errors.New("invalid OLLANTA_JWT_EXPIRY")
	}
	if err := applyEnvDurationValue(&cfg.RefreshExpiry, "OLLANTA_REFRESH_EXPIRY"); err != nil {
		return errors.New("invalid OLLANTA_REFRESH_EXPIRY")
	}
	return nil
}

func applyJobRecoveryEnvConfig(cfg *JobRecoveryConfig, prefix string) error {
	if err := applyEnvDurationValue(&cfg.StaleAfter, prefix+"_STALE_AFTER"); err != nil {
		return errors.New(invalidConfigValuePrefix + prefix + "_STALE_AFTER")
	}
	if err := applyEnvIntValue(&cfg.MaxAttempts, prefix+"_MAX_ATTEMPTS"); err != nil {
		return err
	}
	if err := applyEnvDurationValue(&cfg.Interval, prefix+"_RECOVERY_INTERVAL"); err != nil {
		return errors.New(invalidConfigValuePrefix + prefix + "_RECOVERY_INTERVAL")
	}
	return nil
}

func validateRuntimeConfig(cfg *Config) error {
	if cfg.JWTSecret == "" && !cfg.AllowRandomJWTSecret {
		return errors.New("jwt secret is required (set OLLANTA_JWT_SECRET or OLLANTA_ALLOW_RANDOM_JWT_SECRET=true for local development)")
	}
	if cfg.HTTPMaxBodyBytes <= 0 {
		return errors.New("http max body bytes must be greater than zero")
	}
	if cfg.ScanQueueMaxAccepted < 0 {
		return errors.New("scan queue max accepted cannot be negative")
	}
	if cfg.ScanQueueMaxRunning < 0 {
		return errors.New("scan queue max running cannot be negative")
	}
	if cfg.ScanQueueMaxOldestAcceptedAge < 0 {
		return errors.New("scan queue max oldest accepted age cannot be negative")
	}
	if cfg.ScanQueueRetryAfter < 0 {
		return errors.New("scan queue retry after cannot be negative")
	}
	if err := validateJobRecoveryConfig("scan job recovery", cfg.ScanJobRecovery); err != nil {
		return err
	}
	if err := validateJobRecoveryConfig("index job recovery", cfg.IndexJobRecovery); err != nil {
		return err
	}
	if err := validateJobRecoveryConfig("webhook job recovery", cfg.WebhookJobRecovery); err != nil {
		return err
	}
	origins, err := validateCORSOrigins(cfg.CORSAllowedOrigins, cfg.CORSAllowUnsafeWildcard)
	if err != nil {
		return err
	}
	cfg.CORSAllowedOrigins = origins
	if err := cfg.PostgresPool.Validate(); err != nil {
		return fmt.Errorf("invalid postgres pool config: %w", err)
	}
	return nil
}

func validateJobRecoveryConfig(name string, cfg JobRecoveryConfig) error {
	if cfg.StaleAfter < 0 {
		return fmt.Errorf("%s stale after cannot be negative", name)
	}
	if cfg.Interval < 0 {
		return fmt.Errorf("%s interval cannot be negative", name)
	}
	if cfg.MaxAttempts < 0 {
		return fmt.Errorf("%s max attempts cannot be negative", name)
	}
	if cfg.StaleAfter == 0 || cfg.Interval == 0 {
		return nil
	}
	if cfg.MaxAttempts == 0 {
		return fmt.Errorf("%s max attempts must be greater than zero when recovery is enabled", name)
	}
	return nil
}

func validateCORSOrigins(origins []string, allowUnsafeWildcard bool) ([]string, error) {
	if len(origins) == 0 {
		return nil, nil
	}
	valid := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		origin, err := normalizeCORSOrigin(origin, allowUnsafeWildcard)
		if err != nil {
			return nil, err
		}
		if origin == "" {
			continue
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		valid = append(valid, origin)
	}
	return valid, nil
}

func normalizeCORSOrigin(origin string, allowUnsafeWildcard bool) (string, error) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", nil
	}
	if origin == "*" {
		if !allowUnsafeWildcard {
			return "", errors.New("wildcard CORS origin requires OLLANTA_CORS_ALLOW_UNSAFE_WILDCARD=true")
		}
		return origin, nil
	}
	return normalizeHTTPCORSOrigin(origin)
}

func normalizeHTTPCORSOrigin(origin string) (string, error) {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("CORS allowed origins must be absolute http or https origins")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("CORS allowed origins must use http or https")
	}
	if (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("CORS allowed origins must not include path, query, or fragment")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func applyEnvObservabilityLinks(cfg *Config, key string) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	links, err := parseObservabilityLinks(value)
	if err != nil {
		return errors.New(invalidConfigValuePrefix + key)
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

func applyStringListValue(dst *[]string, value []string) {
	if len(value) == 0 {
		return
	}
	*dst = value
}

func applyInt32Value(dst *int32, value int) {
	if value == 0 {
		return
	}
	*dst = int32(value)
}

func applyDurationValue(dst *time.Duration, value, label string) error {
	if value == "" {
		return nil
	}
	duration, err := parseDuration(value, *dst)
	if err != nil {
		return errors.New(invalidConfigValuePrefix + label)
	}
	*dst = duration
	return nil
}

func applyEnvStringValue(dst *string, key string) {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		*dst = value
	}
}

func applyEnvStringListValue(dst *[]string, key string) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return
	}
	if strings.TrimSpace(value) == "" {
		*dst = nil
		return
	}
	*dst = splitList(value)
}

func applyEnvBoolValue(dst *bool, key string) error {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return errors.New(invalidConfigValuePrefix + key)
	}
	*dst = parsed
	return nil
}

func applyEnvInt64Value(dst *int64, key string) error {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return errors.New(invalidConfigValuePrefix + key)
	}
	*dst = parsed
	return nil
}

func applyEnvIntValue(dst *int, key string) error {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return errors.New(invalidConfigValuePrefix + key)
	}
	*dst = parsed
	return nil
}

func applyEnvInt32Value(dst *int32, key string) error {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return errors.New(invalidConfigValuePrefix + key)
	}
	*dst = int32(parsed)
	return nil
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

func splitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func containsWildcardOrigin(origins []string) bool {
	for _, origin := range origins {
		if origin == "*" {
			return true
		}
	}
	return false
}
