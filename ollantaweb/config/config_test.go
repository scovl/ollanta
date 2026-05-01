package config

import (
	"os"
	"path/filepath"
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

[database]
url = "postgres://file@localhost:5432/ollanta?sslmode=disable"

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
	config := []byte(`[database]
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

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadReadsConfigPathFromEnv(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "server.toml")
	config := []byte("[server]\naddr = \":38080\"\n\n[database]\nurl = \"postgres://envfile@localhost:5432/ollanta?sslmode=disable\"\n")
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
	if cfg.Addr != ":18080" {
		t.Fatalf("Addr = %q, want :18080", cfg.Addr)
	}
	if cfg.AdminAddr != ":19090" {
		t.Fatalf("AdminAddr = %q, want :19090", cfg.AdminAddr)
	}
	if cfg.DatabaseURL != "postgres://file@localhost:5432/ollanta?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q, want file value", cfg.DatabaseURL)
	}
	if cfg.ZincSearchURL != "http://localhost:4081" {
		t.Fatalf("ZincSearchURL = %q, want http://localhost:4081", cfg.ZincSearchURL)
	}
	if cfg.SearchBackend != "postgres" {
		t.Fatalf("SearchBackend = %q, want postgres", cfg.SearchBackend)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want debug", cfg.LogLevel)
	}
	if cfg.JWTSecret != "file-secret" {
		t.Fatalf("JWTSecret = %q, want file-secret", cfg.JWTSecret)
	}
	if cfg.JWTExpiry != 30*time.Minute {
		t.Fatalf("JWTExpiry = %s, want 30m0s", cfg.JWTExpiry)
	}
	if cfg.RefreshExpiry != 48*time.Hour {
		t.Fatalf("RefreshExpiry = %s, want 48h0m0s", cfg.RefreshExpiry)
	}
	if cfg.ScannerToken != "scanner-secret" {
		t.Fatalf("ScannerToken = %q, want scanner-secret", cfg.ScannerToken)
	}
}
