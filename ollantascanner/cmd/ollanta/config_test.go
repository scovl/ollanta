package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	appscan "github.com/scovl/ollanta/application/scan"
)

const (
	writeFileError  = "WriteFile() error = %v"
	getwdError      = "Getwd() error = %v"
	restoreCWDError = "restore cwd: %v"
	chdirError      = "Chdir() error = %v"
	parseError      = "parseOptions() error = %v"
)

func TestParseOptionsAppliesScannerConfigFile(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, "config.toml")
	config := []byte(`[scanner]
project_dir = "./demo"
sources = ["./cmd/...", "./pkg/..."]
exclusions = ["vendor/**"]
project_key = "demo"
format = "json"
local_ui = true
port = 8888
bind = "0.0.0.0"
server_url = "http://localhost:8080"
server_token = "secret"
server_wait = true
server_wait_timeout = "3m"
server_wait_poll = "5s"
`)
	writeConfigFile(t, configPath, config)

	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf(parseError, err)
	}
	assertScannerConfig(t, opts)
}

func TestParseOptionsFlagsOverrideScannerConfigFile(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, "config.toml")
	config := []byte(`[scanner]
format = "json"
local_ui = true
port = 8888
bind = "0.0.0.0"
server_url = "http://localhost:8080"
server_wait = true
`)
	writeConfigFile(t, configPath, config)

	opts, err := parseOptions([]string{"-format=sarif", "-local-ui=false", "-port=7777", "-bind=127.0.0.1", "-server=http://example.com", "-server-wait=false"})
	if err != nil {
		t.Fatalf(parseError, err)
	}
	if opts.Format != "sarif" {
		t.Fatalf("Format = %q, want sarif", opts.Format)
	}
	if opts.Serve {
		t.Fatal("Serve = true, want false")
	}
	if opts.Port != 7777 {
		t.Fatalf("Port = %d, want 7777", opts.Port)
	}
	if opts.Bind != "127.0.0.1" {
		t.Fatalf("Bind = %q, want 127.0.0.1", opts.Bind)
	}
	if opts.Server != "http://example.com" {
		t.Fatalf("Server = %q, want http://example.com", opts.Server)
	}
	if opts.ServerWait {
		t.Fatal("ServerWait = true, want false")
	}
}

func TestParseOptionsReadsExplicitConfigPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "scanner.toml")
	writeConfigFile(t, configPath, []byte("[scanner]\nport = 9999\n"))

	opts, err := parseOptions([]string{"-config", configPath})
	if err != nil {
		t.Fatalf(parseError, err)
	}
	if opts.Port != 9999 {
		t.Fatalf("Port = %d, want 9999", opts.Port)
	}
}

func TestParseOptionsReadsConfigPathFromEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "scanner.toml")
	writeConfigFile(t, configPath, []byte("[scanner]\nport = 9998\n"))

	t.Setenv("OLLANTA_CONFIG_FILE", configPath)

	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf(parseError, err)
	}
	if opts.Port != 9998 {
		t.Fatalf("Port = %d, want 9998", opts.Port)
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

func assertScannerConfig(t *testing.T, opts *appscan.ScanOptions) {
	t.Helper()
	if opts.ProjectDir != "./demo" {
		t.Fatalf("ProjectDir = %q, want ./demo", opts.ProjectDir)
	}
	if len(opts.Sources) != 2 || opts.Sources[0] != "./cmd/..." || opts.Sources[1] != "./pkg/..." {
		t.Fatalf("Sources = %#v, want config values", opts.Sources)
	}
	if len(opts.Exclusions) != 1 || opts.Exclusions[0] != "vendor/**" {
		t.Fatalf("Exclusions = %#v, want config values", opts.Exclusions)
	}
	if opts.ProjectKey != "demo" {
		t.Fatalf("ProjectKey = %q, want demo", opts.ProjectKey)
	}
	if opts.Format != "json" {
		t.Fatalf("Format = %q, want json", opts.Format)
	}
	if !opts.Serve {
		t.Fatal("Serve = false, want true")
	}
	if opts.Port != 8888 {
		t.Fatalf("Port = %d, want 8888", opts.Port)
	}
	if opts.Bind != "0.0.0.0" {
		t.Fatalf("Bind = %q, want 0.0.0.0", opts.Bind)
	}
	if opts.Server != "http://localhost:8080" {
		t.Fatalf("Server = %q, want http://localhost:8080", opts.Server)
	}
	if opts.ServerToken != "secret" {
		t.Fatalf("ServerToken = %q, want secret", opts.ServerToken)
	}
	if !opts.ServerWait {
		t.Fatal("ServerWait = false, want true")
	}
	if opts.WaitTimeout != 3*time.Minute {
		t.Fatalf("WaitTimeout = %s, want 3m0s", opts.WaitTimeout)
	}
	if opts.WaitPoll != 5*time.Second {
		t.Fatalf("WaitPoll = %s, want 5s", opts.WaitPoll)
	}
}
