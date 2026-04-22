package scan_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/scovl/ollanta/ollantascanner/scan"
)

func TestParseFlags_ConfigFileDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "ollanta.toml")
	if err := os.WriteFile(configPath, []byte(`
[scanner]
project_key = "from-config"
format = "json"
serve = true
port = 9090
bind = "0.0.0.0"
server = "http://localhost:8080"
server_token = "scanner-token"
server_wait = true
server_wait_timeout = "30s"
server_wait_poll = "500ms"
sources = ["./cmd/...", "./pkg/..."]
exclusions = ["vendor/**", "*_test.go"]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	opts, err := scan.ParseFlags([]string{"-config", configPath})
	if err != nil {
		t.Fatal(err)
	}

	if opts.ProjectKey != "from-config" || opts.Format != "json" || !opts.Serve {
		t.Fatalf("unexpected config defaults: %+v", opts)
	}
	if opts.Port != 9090 || opts.Bind != "0.0.0.0" || opts.ServerToken != "scanner-token" || !opts.ServerWait {
		t.Fatalf("unexpected scanner network defaults: %+v", opts)
	}
	if opts.WaitTimeout != 30*time.Second || opts.WaitPoll != 500*time.Millisecond {
		t.Fatalf("unexpected wait defaults: timeout=%s poll=%s", opts.WaitTimeout, opts.WaitPoll)
	}
	if len(opts.Sources) != 2 || len(opts.Exclusions) != 2 {
		t.Fatalf("unexpected list defaults: %+v", opts)
	}
}

func TestParseFlags_ExplicitFlagOverridesConfigFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "ollanta.toml")
	if err := os.WriteFile(configPath, []byte(`
[scanner]
format = "json"
port = 9090
`), 0o600); err != nil {
		t.Fatal(err)
	}

	opts, err := scan.ParseFlags([]string{"-config", configPath, "-format", "sarif", "-port", "7778"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Format != "sarif" || opts.Port != 7778 {
		t.Fatalf("expected explicit flags to win, got %+v", opts)
	}
}

func TestParseFlags_UsesSharedServerURLWhenScannerServerMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "ollanta.toml")
	if err := os.WriteFile(configPath, []byte(`
[server]
host = "localhost"
port = 9091
`), 0o600); err != nil {
		t.Fatal(err)
	}

	opts, err := scan.ParseFlags([]string{"-config", configPath})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Server != "http://localhost:9091" {
		t.Fatalf("expected shared server url, got %q", opts.Server)
	}
}

func TestParseFlags_MapsWildcardServerHostToLocalhost(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "ollanta.toml")
	if err := os.WriteFile(configPath, []byte(`
[server]
host = "0.0.0.0"
port = 8080
`), 0o600); err != nil {
		t.Fatal(err)
	}

	opts, err := scan.ParseFlags([]string{"-config", configPath})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Server != "http://localhost:8080" {
		t.Fatalf("expected localhost server url, got %q", opts.Server)
	}
}
