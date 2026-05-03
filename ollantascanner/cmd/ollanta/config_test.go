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

func TestParseOptionsExplicitConfigAllowsFlagOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "scanner.toml")
	writeConfigFile(t, configPath, []byte("[scanner]\nformat = \"all\"\nsources = [\"./...\"]\nserver_wait = false\n"))

	opts, err := parseOptions([]string{"--config=" + configPath, "-format=json", "-sources=application/scan,application/ingest", "-server=http://localhost:8080", "-server-wait=true"})
	if err != nil {
		t.Fatalf(parseError, err)
	}
	if opts.Format != "json" {
		t.Fatalf("Format = %q, want json", opts.Format)
	}
	if len(opts.Sources) != 2 || opts.Sources[0] != "application/scan" || opts.Sources[1] != "application/ingest" {
		t.Fatalf("Sources = %#v, want flag override", opts.Sources)
	}
	if opts.Server != "http://localhost:8080" {
		t.Fatalf("Server = %q, want flag override", opts.Server)
	}
	if !opts.ServerWait {
		t.Fatal("ServerWait = false, want flag override")
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

func TestParseOptionsAppliesTestsConfigFile(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, "config.toml")
	config := []byte(`[tests]
enabled = true
mode = "collect"
discover = true
run = false
max_report_age = "2h"
exclusions = ["fixtures/**"]
max_depth = 4
max_candidates = 25
max_report_bytes = 4096
max_runtime = "90s"
command_policy = "explicit"
fail_on_timeout = true
allow_external_artifacts = true

[[tests.path_mapping]]
from = "/workspace/app"
to = "."

[[tests.modules]]
name = "core-domain"
root = "domain"
language = "go"
architecture_role = "domain"
test_policy = "required"
command = "go test ./..."
coverage_reports = ["coverage.out"]
test_reports = ["junit.xml"]
native_reports = ["ollanta-tests.json"]
coverage_threshold = 85.5
new_coverage_threshold = 90.0
mutation_threshold = 70.0
owner = "platform"
team = "quality"
integration_required = true
suite_kind = "contract"
evidence_confidence = "medium"
allow_external_artifacts = false
`)
	writeConfigFile(t, configPath, config)

	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf(parseError, err)
	}
	assertTestsConfigValues(t, opts.Tests)
	assertTestsModuleValues(t, opts.Tests.Modules)
}

func assertTestsConfigValues(t *testing.T, opts appscan.TestOptions) {
	t.Helper()
	if !opts.Enabled || opts.Mode != appscan.TestModeCollect || opts.Run {
		t.Fatalf("Tests = %+v, want enabled collect without run", opts)
	}
	if opts.MaxReportAge != 2*time.Hour {
		t.Fatalf("Tests.MaxReportAge = %s, want 2h", opts.MaxReportAge)
	}
	if opts.MaxRuntime != 90*time.Second || !opts.FailOnTimeout || !opts.AllowExternalArtifacts {
		t.Fatalf("Tests timeout/artifact options = %+v, want configured values", opts)
	}
	if len(opts.PathMappings) != 1 || opts.PathMappings[0].From != "/workspace/app" {
		t.Fatalf("Tests.PathMappings = %#v, want configured mapping", opts.PathMappings)
	}
}

func assertTestsModuleValues(t *testing.T, modules []appscan.TestModuleConfig) {
	t.Helper()
	if len(modules) != 1 {
		t.Fatalf("Tests.Modules length = %d, want 1", len(modules))
	}
	module := modules[0]
	assertTestsModuleIdentity(t, module)
	if module.CoverageThreshold == nil || *module.CoverageThreshold != 85.5 {
		t.Fatalf("CoverageThreshold = %v, want 85.5", module.CoverageThreshold)
	}
	if !module.IntegrationRequired {
		t.Fatal("IntegrationRequired = false, want true")
	}
	if module.AllowExternalArtifacts == nil || *module.AllowExternalArtifacts {
		t.Fatalf("AllowExternalArtifacts = %v, want false module override", module.AllowExternalArtifacts)
	}
}

func assertTestsModuleIdentity(t *testing.T, module appscan.TestModuleConfig) {
	t.Helper()
	if module.Name != "core-domain" || module.Root != "domain" || module.ArchitectureRole != "domain" {
		t.Fatalf("Tests.Modules[0] = %+v, want configured module", module)
	}
	if module.SuiteKind != appscan.SuiteKindContract || module.EvidenceConfidence != appscan.EvidenceConfidenceMedium {
		t.Fatalf("suite evidence = %q/%q, want contract/medium", module.SuiteKind, module.EvidenceConfidence)
	}
}

func TestParseOptionsTestsFlagsOverrideConfigFile(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, "config.toml")
	writeConfigFile(t, configPath, []byte("[tests]\nenabled = true\nmode = \"collect\"\nrun = false\ncommand_policy = \"explicit\"\n"))

	opts, err := parseOptions([]string{"-with-tests=false", "-tests-mode=run", "-tests-run=true", "-tests-command-policy=never", "-tests-max-runtime=10s", "-tests-fail-on-timeout=true", "-tests-allow-external-artifacts=true"})
	if err != nil {
		t.Fatalf(parseError, err)
	}
	if opts.Tests.Enabled {
		t.Fatal("Tests.Enabled = true, want false from flag override")
	}
	if opts.Tests.Mode != appscan.TestModeRun {
		t.Fatalf("Tests.Mode = %q, want run", opts.Tests.Mode)
	}
	if !opts.Tests.Run {
		t.Fatal("Tests.Run = false, want true from flag override")
	}
	if opts.Tests.CommandPolicy != appscan.CommandPolicyNever || opts.Tests.MaxRuntime != 10*time.Second || !opts.Tests.FailOnTimeout || !opts.Tests.AllowExternalArtifacts {
		t.Fatalf("Tests flags = %+v, want command policy, runtime, timeout, and artifact overrides", opts.Tests)
	}
}

func TestParseOptionsRejectsInvalidTestEvidenceConfig(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, "config.toml")
	writeConfigFile(t, configPath, []byte("[tests]\nenabled = true\nmode = \"mystery\"\n"))

	if _, err := parseOptions(nil); err == nil {
		t.Fatal("parseOptions() error = nil, want invalid test mode error")
	}
}

func TestParseOptionsAppliesMutationsConfigFile(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, "config.toml")
	config := []byte(`[mutations]
enabled = true
mode = "collect"
discover = true
run = false
changed_only = true
max_runtime = "8m"
max_mutants = 250
max_report_age = "12h"
exclusions = ["generated/**"]
max_depth = 5
max_candidates = 30
max_report_bytes = 8192
command_policy = "explicit"
fail_on_timeout = false
allow_external_artifacts = true

[[mutations.path_mapping]]
from = "/workspace/app"
to = "."

[[mutations.modules]]
name = "domain"
root = "domain"
language = "go"
architecture_role = "domain"
tool = "native"
command = "go test ./... && go-mutesting ./..."
report_paths = ["reports/mutation.json"]
native_report_paths = ["ollanta-mutations.json"]
threshold = 75.0
changed_code_threshold = 85.0
owner = "platform"
team = "quality"
mutation_policy = "required"
suite_kind = "component"
evidence_confidence = "medium"
allow_external_artifacts = false
changed_only = false
max_runtime = "3m"
max_mutants = 50
exclusions = ["fixtures/**"]
fail_on_timeout = true
`)
	writeConfigFile(t, configPath, config)

	opts, err := parseOptions(nil)
	if err != nil {
		t.Fatalf(parseError, err)
	}
	assertMutationsConfigValues(t, opts.Mutations)
	assertMutationsModuleValues(t, opts.Mutations.Modules)
}

func assertMutationsConfigValues(t *testing.T, opts appscan.MutationOptions) {
	t.Helper()
	if !opts.Enabled || opts.Mode != appscan.MutationModeCollect || opts.Run {
		t.Fatalf("Mutations = %+v, want enabled collect without run", opts)
	}
	if opts.MaxRuntime != 8*time.Minute || opts.MaxReportAge != 12*time.Hour || opts.MaxMutants != 250 {
		t.Fatalf("Mutations limits = %+v, want configured durations and mutant limit", opts)
	}
	if !opts.AllowExternalArtifacts {
		t.Fatalf("AllowExternalArtifacts = false, want configured true")
	}
	if len(opts.PathMappings) != 1 || opts.PathMappings[0].From != "/workspace/app" {
		t.Fatalf("Mutations.PathMappings = %#v, want configured mapping", opts.PathMappings)
	}
}

func assertMutationsModuleValues(t *testing.T, modules []appscan.MutationModuleConfig) {
	t.Helper()
	if len(modules) != 1 {
		t.Fatalf("Mutations.Modules length = %d, want 1", len(modules))
	}
	module := modules[0]
	if module.Name != "domain" || module.Tool != "native" || module.MaxRuntime != 3*time.Minute || module.MaxMutants != 50 {
		t.Fatalf("Mutations.Modules[0] = %+v, want configured module", module)
	}
	if module.SuiteKind != appscan.SuiteKindComponent || module.EvidenceConfidence != appscan.EvidenceConfidenceMedium {
		t.Fatalf("suite evidence = %q/%q, want component/medium", module.SuiteKind, module.EvidenceConfidence)
	}
	if module.AllowExternalArtifacts == nil || *module.AllowExternalArtifacts {
		t.Fatalf("AllowExternalArtifacts = %v, want false module override", module.AllowExternalArtifacts)
	}
	if module.ChangedOnly == nil || *module.ChangedOnly {
		t.Fatalf("ChangedOnly = %v, want false override", module.ChangedOnly)
	}
	if module.FailOnTimeout == nil || !*module.FailOnTimeout {
		t.Fatalf("FailOnTimeout = %v, want true override", module.FailOnTimeout)
	}
}

func TestParseOptionsMutationsFlagsOverrideConfigFile(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, "config.toml")
	writeConfigFile(t, configPath, []byte("[mutations]\nenabled = true\nmode = \"collect\"\nrun = false\ncommand_policy = \"explicit\"\n"))

	opts, err := parseOptions([]string{"-with-mutations=false", "-mutations-mode=run", "-mutations-run=true", "-mutations-command-policy=never", "-mutations-allow-external-artifacts=true"})
	if err != nil {
		t.Fatalf(parseError, err)
	}
	if opts.Mutations.Enabled {
		t.Fatal("Mutations.Enabled = true, want false from flag override")
	}
	if opts.Mutations.Mode != appscan.MutationModeRun || !opts.Mutations.Run {
		t.Fatalf("Mutations = %+v, want run mode and run flag override", opts.Mutations)
	}
	if opts.Mutations.CommandPolicy != appscan.CommandPolicyNever || !opts.Mutations.AllowExternalArtifacts {
		t.Fatalf("Mutations flags = %+v, want command policy and artifact overrides", opts.Mutations)
	}
}

func TestParseOptionsRejectsInvalidMutationEvidenceConfig(t *testing.T) {
	dir := t.TempDir()
	enterDir(t, dir)
	configPath := filepath.Join(dir, "config.toml")
	writeConfigFile(t, configPath, []byte("[mutations]\nenabled = true\ncommand_policy = \"surprise\"\n"))

	if _, err := parseOptions(nil); err == nil {
		t.Fatal("parseOptions() error = nil, want invalid mutation command policy error")
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
