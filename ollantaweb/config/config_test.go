package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	configFileName  = "config.toml"
	writeFileError  = "WriteFile() error = %v"
	getwdError      = "Getwd() error = %v"
	restoreCWDError = "restore cwd: %v"
	chdirError      = "Chdir() error = %v"
	loadError       = "Load() error = %v"
)

func TestLoadReadsServerConfigFromConfigToml(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
addr = ":18080"
admin_addr = ":19090"
log_level = "debug"
jwt_secret = "file-secret"
jwt_expiry = "30m"
refresh_expiry = "48h"
oauth_redirect_base = "http://localhost:8080"
scanner_token = "scanner-secret"
cors_allowed_origins = ["https://ui.example.com", "http://localhost:3000"]
http_max_body_bytes = 2097152
auto_migrate = false
scan_job_stale_after = "20m"
scan_job_max_attempts = 4
scan_job_recovery_interval = "2m"
index_job_stale_after = "10m"
index_job_max_attempts = 5
index_job_recovery_interval = "90s"
webhook_job_stale_after = "30m"
webhook_job_max_attempts = 6
webhook_job_recovery_interval = "3m"

[[ui.observability_links]]
label = "Grafana"
url = "https://grafana.example.com/d/ollanta"

[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"
pool_max_conns = 12
pool_min_conns = 2
pool_max_conn_lifetime = "45m"
pool_max_conn_idle_time = "10m"

[search]
backend = "postgres"
url = "http://localhost:4081"
user = "file-user"
password = "file-pass"
`)
	writeConfigFile(t, configPath, config)

	cfg, err := Load()
	if err != nil {
		t.Fatalf(loadError, err)
	}
	assertServerFileConfig(t, cfg)
}

func TestLoadBuildsDatabaseAndSearchConfigFromExplicitFields(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
addr = ":18080"
	jwt_secret = "file-secret"

[database]
host = "db.internal"
port = 15432
name = "ollanta"
user = "dbuser"
password = "dbpass"
sslmode = "require"

[search]
host = "search.internal"
port = 14080
user = "search-user"
password = "search-pass"
`)
	writeConfigFile(t, configPath, config)

	cfg, err := Load()
	if err != nil {
		t.Fatalf(loadError, err)
	}
	if cfg.DatabaseURL != "postgres://dbuser:dbpass@db.internal:15432/ollanta?sslmode=require" {
		t.Fatalf("DatabaseURL = %q, want explicit field value", cfg.DatabaseURL)
	}
	if cfg.ZincSearchURL != "http://search.internal:14080" {
		t.Fatalf("ZincSearchURL = %q, want explicit field value", cfg.ZincSearchURL)
	}
	if cfg.ZincSearchUser != "search-user" {
		t.Fatalf("ZincSearchUser = %q, want search-user", cfg.ZincSearchUser)
	}
	if cfg.ZincSearchPassword != "search-pass" {
		t.Fatalf("ZincSearchPassword = %q, want search-pass", cfg.ZincSearchPassword)
	}
}

func TestLoadURLOverridesExplicitDatabaseAndSearchFields(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
jwt_secret = "file-secret"

[database]
url = "postgres://override@localhost:5432/ollanta?sslmode=disable"
host = "db.internal"
port = 15432
name = "ignored"
user = "ignored"
password = "ignored"

[search]
url = "http://search-override:4081"
host = "search.internal"
port = 14080
user = "search-user"
password = "search-pass"
`)
	writeConfigFile(t, configPath, config)

	cfg, err := Load()
	if err != nil {
		t.Fatalf(loadError, err)
	}
	if cfg.DatabaseURL != "postgres://override@localhost:5432/ollanta?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q, want url override", cfg.DatabaseURL)
	}
	if cfg.ZincSearchURL != "http://search-override:4081" {
		t.Fatalf("ZincSearchURL = %q, want url override", cfg.ZincSearchURL)
	}
}

func TestLoadEnvOverridesConfigToml(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
addr = ":18080"
log_level = "info"
	jwt_secret = "file-secret"

[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"
`)
	writeConfigFile(t, configPath, config)

	t.Setenv("OLLANTA_ADDR", ":28080")
	t.Setenv("OLLANTA_DATABASE_URL", "postgres://env@localhost:5432/ollanta?sslmode=disable")
	t.Setenv("OLLANTA_LOG_LEVEL", "warn")

	cfg, err := Load()
	if err != nil {
		t.Fatalf(loadError, err)
	}
	if cfg.Addr != ":28080" {
		t.Fatalf("Addr = %q, want :28080", cfg.Addr)
	}
	if cfg.DatabaseURL != "postgres://env@localhost:5432/ollanta?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q, want env value", cfg.DatabaseURL)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("LogLevel = %q, want warn", cfg.LogLevel)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	t.Setenv("OLLANTA_JWT_SECRET", "env-secret")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadReadsConfigPathFromEnv(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "server.toml")
	config := []byte("[server]\naddr = \":38080\"\njwt_secret = \"file-secret\"\n\n[database]\nurl = \"postgres://envfile@localhost:5432/ollanta?sslmode=disable\"\n")
	writeConfigFile(t, configPath, config)

	t.Setenv("OLLANTA_CONFIG_FILE", configPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf(loadError, err)
	}
	if cfg.Addr != ":38080" {
		t.Fatalf("Addr = %q, want :38080", cfg.Addr)
	}
	if cfg.DatabaseURL != "postgres://envfile@localhost:5432/ollanta?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q, want envfile value", cfg.DatabaseURL)
	}
}

func writeConfigFile(t *testing.T, path string, contents []byte) {
	t.Helper()
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf(writeFileError, err)
	}
}

func enterDir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf(getwdError, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf(restoreCWDError, err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf(chdirError, err)
	}
}

func assertServerFileConfig(t *testing.T, cfg *Config) {
	t.Helper()
	assertServerCoreFileConfig(t, cfg)
	assertServerRuntimeFileConfig(t, cfg)
	assertServerRecoveryFileConfig(t, cfg)
	assertObservabilityLinks(t, cfg.ObservabilityLinks)
}

func assertServerCoreFileConfig(t *testing.T, cfg *Config) {
	t.Helper()
	assertEqual(t, "Addr", cfg.Addr, ":18080")
	assertEqual(t, "AdminAddr", cfg.AdminAddr, ":19090")
	assertEqual(t, "DatabaseURL", cfg.DatabaseURL, "postgres://file@localhost:5432/ollanta?sslmode=disable")
	assertEqual(t, "ZincSearchURL", cfg.ZincSearchURL, "http://localhost:4081")
	assertEqual(t, "SearchBackend", cfg.SearchBackend, "postgres")
	assertEqual(t, "LogLevel", cfg.LogLevel, "debug")
	assertEqual(t, "JWTSecret", cfg.JWTSecret, "file-secret")
	assertEqual(t, "JWTExpiry", cfg.JWTExpiry, 30*time.Minute)
	assertEqual(t, "RefreshExpiry", cfg.RefreshExpiry, 48*time.Hour)
	assertEqual(t, "ScannerToken", cfg.ScannerToken, "scanner-secret")
}

func assertServerRuntimeFileConfig(t *testing.T, cfg *Config) {
	t.Helper()
	assertEqual(t, "CORSAllowedOrigins", cfg.CORSAllowedOrigins, []string{"https://ui.example.com", "http://localhost:3000"})
	assertEqual(t, "HTTPMaxBodyBytes", cfg.HTTPMaxBodyBytes, int64(2097152))
	assertEqual(t, "AutoMigrate", cfg.AutoMigrate, false)
	assertEqual(t, "PostgresPool.MaxConns", cfg.PostgresPool.MaxConns, int32(12))
	assertEqual(t, "PostgresPool.MinConns", cfg.PostgresPool.MinConns, int32(2))
	assertEqual(t, "PostgresPool.MaxConnLifetime", cfg.PostgresPool.MaxConnLifetime, 45*time.Minute)
	assertEqual(t, "PostgresPool.MaxConnIdleTime", cfg.PostgresPool.MaxConnIdleTime, 10*time.Minute)
}

func assertServerRecoveryFileConfig(t *testing.T, cfg *Config) {
	t.Helper()
	assertEqual(t, "ScanJobRecovery", cfg.ScanJobRecovery, JobRecoveryConfig{StaleAfter: 20 * time.Minute, MaxAttempts: 4, Interval: 2 * time.Minute})
	assertEqual(t, "IndexJobRecovery", cfg.IndexJobRecovery, JobRecoveryConfig{StaleAfter: 10 * time.Minute, MaxAttempts: 5, Interval: 90 * time.Second})
	assertEqual(t, "WebhookJobRecovery", cfg.WebhookJobRecovery, JobRecoveryConfig{StaleAfter: 30 * time.Minute, MaxAttempts: 6, Interval: 3 * time.Minute})
}

func assertObservabilityLinks(t *testing.T, links []ObservabilityLink) {
	t.Helper()
	assertEqual(t, "ObservabilityLinks", links, []ObservabilityLink{{Label: "Grafana", URL: "https://grafana.example.com/d/ollanta"}})
}

func TestLoadEnvOverridesObservabilityLinks(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
jwt_secret = "file-secret"

[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"

[[ui.observability_links]]
label = "Prometheus"
url = "http://localhost:9091/targets"
`)
	writeConfigFile(t, configPath, config)

	t.Setenv("OLLANTA_OBSERVABILITY_LINKS", "Datadog=https://app.datadoghq.com/dashboard/abc;Grafana=https://grafana.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf(loadError, err)
	}
	if len(cfg.ObservabilityLinks) != 2 {
		t.Fatalf("ObservabilityLinks length = %d, want 2", len(cfg.ObservabilityLinks))
	}
	if cfg.ObservabilityLinks[0].Label != "Datadog" || cfg.ObservabilityLinks[0].URL != "https://app.datadoghq.com/dashboard/abc" {
		t.Fatalf("ObservabilityLinks[0] = %+v, want Datadog link", cfg.ObservabilityLinks[0])
	}
	if cfg.ObservabilityLinks[1].Label != "Grafana" || cfg.ObservabilityLinks[1].URL != "https://grafana.example.com" {
		t.Fatalf("ObservabilityLinks[1] = %+v, want Grafana link", cfg.ObservabilityLinks[1])
	}
}

func TestLoadRejectsInvalidObservabilityLinks(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
jwt_secret = "file-secret"

[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"

[[ui.observability_links]]
label = "Broken"
url = "localhost:9091"
`)
	writeConfigFile(t, configPath, config)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid observability link error")
	}
}

func TestLoadRejectsMissingJWTSecret(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"
`)
	writeConfigFile(t, configPath, config)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want missing JWT secret error")
	}
	if !strings.Contains(err.Error(), "OLLANTA_JWT_SECRET") {
		t.Fatalf("Load() error = %q, want OLLANTA_JWT_SECRET guidance", err.Error())
	}
}

func TestLoadAllowsRandomJWTSecretWithOptIn(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
allow_random_jwt_secret = true

[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"
`)
	writeConfigFile(t, configPath, config)

	cfg, err := Load()
	if err != nil {
		t.Fatalf(loadError, err)
	}
	if cfg.JWTSecret == "" {
		t.Fatal("JWTSecret is empty, want generated secret")
	}
	if !cfg.RandomJWTSecretGenerated {
		t.Fatal("RandomJWTSecretGenerated = false, want true")
	}
}

func TestLoadReadsRuntimeEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
jwt_secret = "file-secret"

[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"
`)
	writeConfigFile(t, configPath, config)

	t.Setenv("OLLANTA_CORS_ALLOWED_ORIGINS", "https://app.example.com,http://localhost:5173")
	t.Setenv("OLLANTA_HTTP_MAX_BODY_BYTES", "4096")
	t.Setenv("OLLANTA_AUTO_MIGRATE", "false")
	t.Setenv("OLLANTA_POSTGRES_MAX_CONNS", "9")
	t.Setenv("OLLANTA_POSTGRES_MIN_CONNS", "3")
	t.Setenv("OLLANTA_POSTGRES_MAX_CONN_LIFETIME", "20m")
	t.Setenv("OLLANTA_POSTGRES_MAX_CONN_IDLE_TIME", "5m")
	t.Setenv("OLLANTA_SCAN_QUEUE_MAX_ACCEPTED", "7")
	t.Setenv("OLLANTA_SCAN_QUEUE_MAX_RUNNING", "2")
	t.Setenv("OLLANTA_SCAN_QUEUE_MAX_OLDEST_ACCEPTED_AGE", "3m")
	t.Setenv("OLLANTA_SCAN_QUEUE_RETRY_AFTER", "45s")
	t.Setenv("OLLANTA_SCAN_JOB_STALE_AFTER", "11m")
	t.Setenv("OLLANTA_SCAN_JOB_MAX_ATTEMPTS", "4")
	t.Setenv("OLLANTA_SCAN_JOB_RECOVERY_INTERVAL", "70s")
	t.Setenv("OLLANTA_INDEX_JOB_STALE_AFTER", "12m")
	t.Setenv("OLLANTA_INDEX_JOB_MAX_ATTEMPTS", "5")
	t.Setenv("OLLANTA_INDEX_JOB_RECOVERY_INTERVAL", "80s")
	t.Setenv("OLLANTA_WEBHOOK_JOB_STALE_AFTER", "13m")
	t.Setenv("OLLANTA_WEBHOOK_JOB_MAX_ATTEMPTS", "6")
	t.Setenv("OLLANTA_WEBHOOK_JOB_RECOVERY_INTERVAL", "90s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf(loadError, err)
	}
	assertRuntimeEnvOverrides(t, cfg)
}

func assertRuntimeEnvOverrides(t *testing.T, cfg *Config) {
	t.Helper()
	assertEqual(t, "HTTPMaxBodyBytes", cfg.HTTPMaxBodyBytes, int64(4096))
	assertEqual(t, "AutoMigrate", cfg.AutoMigrate, false)
	assertEqual(t, "CORSAllowedOrigins", cfg.CORSAllowedOrigins, []string{"https://app.example.com", "http://localhost:5173"})
	assertEqual(t, "PostgresPool.MaxConns", cfg.PostgresPool.MaxConns, int32(9))
	assertEqual(t, "PostgresPool.MinConns", cfg.PostgresPool.MinConns, int32(3))
	assertEqual(t, "PostgresPool.MaxConnLifetime", cfg.PostgresPool.MaxConnLifetime, 20*time.Minute)
	assertEqual(t, "PostgresPool.MaxConnIdleTime", cfg.PostgresPool.MaxConnIdleTime, 5*time.Minute)
	assertEqual(t, "ScanQueueMaxAccepted", cfg.ScanQueueMaxAccepted, 7)
	assertEqual(t, "ScanQueueMaxRunning", cfg.ScanQueueMaxRunning, 2)
	assertEqual(t, "ScanQueueMaxOldestAcceptedAge", cfg.ScanQueueMaxOldestAcceptedAge, 3*time.Minute)
	assertEqual(t, "ScanQueueRetryAfter", cfg.ScanQueueRetryAfter, 45*time.Second)
	assertEqual(t, "ScanJobRecovery", cfg.ScanJobRecovery, JobRecoveryConfig{StaleAfter: 11 * time.Minute, MaxAttempts: 4, Interval: 70 * time.Second})
	assertEqual(t, "IndexJobRecovery", cfg.IndexJobRecovery, JobRecoveryConfig{StaleAfter: 12 * time.Minute, MaxAttempts: 5, Interval: 80 * time.Second})
	assertEqual(t, "WebhookJobRecovery", cfg.WebhookJobRecovery, JobRecoveryConfig{StaleAfter: 13 * time.Minute, MaxAttempts: 6, Interval: 90 * time.Second})
}

func assertEqual(t *testing.T, name string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}

func TestLoadRejectsWildcardCORSWithoutOptIn(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
jwt_secret = "file-secret"
cors_allowed_origins = ["*"]

[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"
`)
	writeConfigFile(t, configPath, config)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want wildcard CORS error")
	}
	if !strings.Contains(err.Error(), "wildcard CORS") {
		t.Fatalf("Load() error = %q, want wildcard CORS guidance", err.Error())
	}
}

func TestLoadRejectsInvalidPostgresPool(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, configFileName)
	config := []byte(`[server]
jwt_secret = "file-secret"

[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"
`)
	writeConfigFile(t, configPath, config)
	t.Setenv("OLLANTA_POSTGRES_MAX_CONNS", "2")
	t.Setenv("OLLANTA_POSTGRES_MIN_CONNS", "3")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid pool error")
	}
	if !strings.Contains(err.Error(), "min connections cannot exceed max connections") {
		t.Fatalf("Load() error = %q, want pool validation guidance", err.Error())
	}
}
