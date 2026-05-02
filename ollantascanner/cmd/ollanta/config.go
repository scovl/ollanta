package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	appscan "github.com/scovl/ollanta/application/scan"
	"github.com/scovl/ollanta/ollantacore/configfile"
)

type rootConfig struct {
	Scanner scannerConfig `toml:"scanner"`
	Tests   testsConfig   `toml:"tests"`
}

type scannerConfig struct {
	ProjectDir        string   `toml:"project_dir"`
	Sources           []string `toml:"sources"`
	Exclusions        []string `toml:"exclusions"`
	ProjectKey        string   `toml:"project_key"`
	Branch            string   `toml:"branch"`
	CommitSHA         string   `toml:"commit_sha"`
	PullRequestKey    string   `toml:"pull_request_key"`
	PullRequestBranch string   `toml:"pull_request_branch"`
	PullRequestBase   string   `toml:"pull_request_base"`
	Format            string   `toml:"format"`
	Debug             *bool    `toml:"debug"`
	LocalUI           *bool    `toml:"local_ui"`
	Port              *int     `toml:"port"`
	Bind              string   `toml:"bind"`
	ServerURL         string   `toml:"server_url"`
	ServerToken       string   `toml:"server_token"`
	ServerWait        *bool    `toml:"server_wait"`
	ServerWaitTimeout string   `toml:"server_wait_timeout"`
	ServerWaitPoll    string   `toml:"server_wait_poll"`
}

type testsConfig struct {
	Enabled        *bool               `toml:"enabled"`
	Mode           string              `toml:"mode"`
	Discover       *bool               `toml:"discover"`
	Run            *bool               `toml:"run"`
	MaxReportAge   string              `toml:"max_report_age"`
	Exclusions     []string            `toml:"exclusions"`
	MaxDepth       int                 `toml:"max_depth"`
	MaxCandidates  int                 `toml:"max_candidates"`
	MaxReportBytes int64               `toml:"max_report_bytes"`
	CommandPolicy  string              `toml:"command_policy"`
	PathMappings   []testsPathMapping  `toml:"path_mapping"`
	Modules        []testsModuleConfig `toml:"modules"`
}

type testsPathMapping struct {
	From string `toml:"from"`
	To   string `toml:"to"`
}

type testsModuleConfig struct {
	Name                 string   `toml:"name"`
	Root                 string   `toml:"root"`
	Language             string   `toml:"language"`
	ArchitectureRole     string   `toml:"architecture_role"`
	TestPolicy           string   `toml:"test_policy"`
	IgnoreReason         string   `toml:"ignore_reason"`
	Command              string   `toml:"command"`
	ArtifactRoot         string   `toml:"artifact_root"`
	ReportRoot           string   `toml:"report_root"`
	CoverageReports      []string `toml:"coverage_reports"`
	TestReports          []string `toml:"test_reports"`
	MutationReports      []string `toml:"mutation_reports"`
	NativeReports        []string `toml:"native_reports"`
	CoverageThreshold    *float64 `toml:"coverage_threshold"`
	NewCoverageThreshold *float64 `toml:"new_coverage_threshold"`
	MutationThreshold    *float64 `toml:"mutation_threshold"`
	Owner                string   `toml:"owner"`
	Team                 string   `toml:"team"`
	IntegrationRequired  *bool    `toml:"integration_required"`
}

func parseOptions(args []string) (*appscan.ScanOptions, error) {
	filteredArgs, configPath, err := extractConfigPath(args)
	if err != nil {
		return nil, err
	}

	provided := providedFlags(filteredArgs)
	opts, err := appscan.ParseFlags(filteredArgs)
	if err != nil {
		return nil, err
	}

	var cfg rootConfig
	if _, found, err := configfile.Load(configPath, &cfg); err != nil {
		return nil, err
	} else if found {
		opts.ConfigPath = configPath
		if err := applyScannerConfig(opts, cfg.Scanner, provided); err != nil {
			return nil, err
		}
		if err := applyTestsConfig(opts, cfg.Tests, provided); err != nil {
			return nil, err
		}
	}

	return opts, nil
}

func extractConfigPath(args []string) ([]string, string, error) {
	filtered := make([]string, 0, len(args))
	var configPath string

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-config" || arg == "--config":
			if index+1 >= len(args) {
				return nil, "", fmt.Errorf("missing value for %s", arg)
			}
			configPath = args[index+1]
			index++
		case strings.HasPrefix(arg, "-config="):
			configPath = strings.TrimPrefix(arg, "-config=")
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		default:
			filtered = append(filtered, arg)
		}
	}

	if configPath == "" {
		configPath = os.Getenv("OLLANTA_CONFIG_FILE")
	}
	return filtered, configPath, nil
}

func providedFlags(args []string) map[string]bool {
	provided := make(map[string]bool)
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name := strings.TrimLeft(arg, "-")
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			name = name[:idx]
		}
		if name != "" {
			provided[name] = true
		}
	}
	return provided
}

func applyScannerConfig(opts *appscan.ScanOptions, cfg scannerConfig, provided map[string]bool) error {
	applyScannerProjectDir(opts, cfg, provided)
	applyStringSliceFlag(&opts.Sources, cfg.Sources, provided, "sources")
	applyStringSliceFlag(&opts.Exclusions, cfg.Exclusions, provided, "exclusions")
	applyStringFlag(&opts.ProjectKey, cfg.ProjectKey, provided, "project-key")
	applyStringFlag(&opts.Branch, cfg.Branch, provided, "branch")
	applyStringFlag(&opts.CommitSHA, cfg.CommitSHA, provided, "commit-sha")
	applyStringFlag(&opts.PullRequestKey, cfg.PullRequestKey, provided, "pull-request-key")
	applyStringFlag(&opts.PullRequestBranch, cfg.PullRequestBranch, provided, "pull-request-branch")
	applyStringFlag(&opts.PullRequestBase, cfg.PullRequestBase, provided, "pull-request-base")
	applyStringFlag(&opts.Format, cfg.Format, provided, "format")
	applyBoolFlag(&opts.Debug, cfg.Debug, provided, "debug")
	applyBoolFlag(&opts.Serve, cfg.LocalUI, provided, "local-ui")
	applyIntFlag(&opts.Port, cfg.Port, provided, "port")
	applyStringFlag(&opts.Bind, cfg.Bind, provided, "bind")
	applyStringFlag(&opts.Server, cfg.ServerURL, provided, "server")
	applyStringFlag(&opts.ServerToken, cfg.ServerToken, provided, "server-token")
	applyBoolFlag(&opts.ServerWait, cfg.ServerWait, provided, "server-wait")
	if err := applyDurationFlag(&opts.WaitTimeout, cfg.ServerWaitTimeout, provided, "server-wait-timeout", "scanner.server_wait_timeout"); err != nil {
		return err
	}
	if err := applyDurationFlag(&opts.WaitPoll, cfg.ServerWaitPoll, provided, "server-wait-poll", "scanner.server_wait_poll"); err != nil {
		return err
	}
	return nil
}

func applyTestsConfig(opts *appscan.ScanOptions, cfg testsConfig, provided map[string]bool) error {
	applyBoolFlag(&opts.Tests.Enabled, cfg.Enabled, provided, "with-tests")
	applyStringFlag(&opts.Tests.Mode, cfg.Mode, provided, "tests-mode")
	applyBoolFlag(&opts.Tests.Discover, cfg.Discover, provided, "tests-discover")
	applyBoolFlag(&opts.Tests.Run, cfg.Run, provided, "tests-run")
	if opts.Tests.Mode == appscan.TestModeRun {
		opts.Tests.Run = true
	}
	if cfg.MaxReportAge != "" {
		duration, err := time.ParseDuration(cfg.MaxReportAge)
		if err != nil {
			return fmt.Errorf("parse tests.max_report_age: %w", err)
		}
		opts.Tests.MaxReportAge = duration
	}
	if cfg.Exclusions != nil {
		opts.Tests.Exclusions = append([]string(nil), cfg.Exclusions...)
	}
	if cfg.MaxDepth > 0 {
		opts.Tests.MaxDepth = cfg.MaxDepth
	}
	if cfg.MaxCandidates > 0 {
		opts.Tests.MaxCandidates = cfg.MaxCandidates
	}
	if cfg.MaxReportBytes > 0 {
		opts.Tests.MaxReportBytes = cfg.MaxReportBytes
	}
	if cfg.CommandPolicy != "" {
		opts.Tests.CommandPolicy = cfg.CommandPolicy
	}
	opts.Tests.PathMappings = make([]appscan.TestPathMapping, 0, len(cfg.PathMappings))
	for _, mapping := range cfg.PathMappings {
		opts.Tests.PathMappings = append(opts.Tests.PathMappings, appscan.TestPathMapping{From: mapping.From, To: mapping.To})
	}
	opts.Tests.Modules = make([]appscan.TestModuleConfig, 0, len(cfg.Modules))
	for _, module := range cfg.Modules {
		opts.Tests.Modules = append(opts.Tests.Modules, toAppTestModule(module))
	}
	return nil
}

func toAppTestModule(module testsModuleConfig) appscan.TestModuleConfig {
	integrationRequired := false
	if module.IntegrationRequired != nil {
		integrationRequired = *module.IntegrationRequired
	}
	return appscan.TestModuleConfig{
		Name:                 module.Name,
		Root:                 module.Root,
		Language:             module.Language,
		ArchitectureRole:     module.ArchitectureRole,
		TestPolicy:           module.TestPolicy,
		IgnoreReason:         module.IgnoreReason,
		Command:              module.Command,
		ArtifactRoot:         module.ArtifactRoot,
		ReportRoot:           module.ReportRoot,
		CoverageReports:      append([]string(nil), module.CoverageReports...),
		TestReports:          append([]string(nil), module.TestReports...),
		MutationReports:      append([]string(nil), module.MutationReports...),
		NativeReports:        append([]string(nil), module.NativeReports...),
		CoverageThreshold:    module.CoverageThreshold,
		NewCoverageThreshold: module.NewCoverageThreshold,
		MutationThreshold:    module.MutationThreshold,
		Owner:                module.Owner,
		Team:                 module.Team,
		IntegrationRequired:  integrationRequired,
	}
}

func applyScannerProjectDir(opts *appscan.ScanOptions, cfg scannerConfig, provided map[string]bool) {
	if cfg.ProjectDir == "" || provided["project-dir"] {
		return
	}
	opts.ProjectDir = cfg.ProjectDir
	if provided["project-key"] || cfg.ProjectKey != "" {
		return
	}
	abs, err := filepath.Abs(cfg.ProjectDir)
	if err != nil {
		abs = cfg.ProjectDir
	}
	opts.ProjectKey = filepath.Base(abs)
}

func applyStringFlag(dst *string, value string, provided map[string]bool, flag string) {
	if value == "" || provided[flag] {
		return
	}
	*dst = value
}

func applyStringSliceFlag(dst *[]string, value []string, provided map[string]bool, flag string) {
	if value == nil || provided[flag] {
		return
	}
	*dst = append([]string(nil), value...)
}

func applyBoolFlag(dst *bool, value *bool, provided map[string]bool, flag string) {
	if value == nil || provided[flag] {
		return
	}
	*dst = *value
}

func applyIntFlag(dst *int, value *int, provided map[string]bool, flag string) {
	if value == nil || provided[flag] {
		return
	}
	*dst = *value
}

func applyDurationFlag(dst *time.Duration, value string, provided map[string]bool, flag, label string) error {
	if value == "" || provided[flag] {
		return nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("parse %s: %w", label, err)
	}
	*dst = duration
	return nil
}
