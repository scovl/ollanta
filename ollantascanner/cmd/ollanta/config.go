package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	appscan "github.com/scovl/ollanta/application/scan"
	"github.com/scovl/ollanta/ollantacore/configfile"
)

type rootConfig struct {
	Scanner   scannerConfig   `toml:"scanner"`
	Tests     testsConfig     `toml:"tests"`
	Mutations mutationsConfig `toml:"mutations"`
}

type scannerConfig struct {
	ProjectDir          string   `toml:"project_dir"`
	Sources             []string `toml:"sources"`
	Exclusions          []string `toml:"exclusions"`
	ProjectKey          string   `toml:"project_key"`
	Branch              string   `toml:"branch"`
	CommitSHA           string   `toml:"commit_sha"`
	PullRequestKey      string   `toml:"pull_request_key"`
	PullRequestBranch   string   `toml:"pull_request_branch"`
	PullRequestBase     string   `toml:"pull_request_base"`
	Format              string   `toml:"format"`
	Debug               *bool    `toml:"debug"`
	LocalUI             *bool    `toml:"local_ui"`
	Port                *int     `toml:"port"`
	Bind                string   `toml:"bind"`
	ServerURL           string   `toml:"server_url"`
	ServerToken         string   `toml:"server_token"`
	ServerWait          *bool    `toml:"server_wait"`
	ServerWaitTimeout   string   `toml:"server_wait_timeout"`
	ServerWaitPoll      string   `toml:"server_wait_poll"`
	ProfileSource       string   `toml:"profile_source"`
	ProfileFile         string   `toml:"profile_file"`
	ProfileStrict       *bool    `toml:"profile_strict"`
	ProfileFetchTimeout string   `toml:"profile_fetch_timeout"`
	Proxy               string   `toml:"proxy"`
	Skip                *bool    `toml:"skip"`
}

type testsConfig struct {
	Enabled                *bool               `toml:"enabled"`
	Mode                   string              `toml:"mode"`
	Discover               *bool               `toml:"discover"`
	Run                    *bool               `toml:"run"`
	MaxRuntime             string              `toml:"max_runtime"`
	FailOnTimeout          *bool               `toml:"fail_on_timeout"`
	MaxReportAge           string              `toml:"max_report_age"`
	Exclusions             []string            `toml:"exclusions"`
	MaxDepth               int                 `toml:"max_depth"`
	MaxCandidates          int                 `toml:"max_candidates"`
	MaxReportBytes         int64               `toml:"max_report_bytes"`
	CommandPolicy          string              `toml:"command_policy"`
	AllowExternalArtifacts *bool               `toml:"allow_external_artifacts"`
	PathMappings           []testsPathMapping  `toml:"path_mapping"`
	Modules                []testsModuleConfig `toml:"modules"`
}

type testsPathMapping struct {
	From string `toml:"from"`
	To   string `toml:"to"`
}

type testsModuleConfig struct {
	Name                   string   `toml:"name"`
	Root                   string   `toml:"root"`
	Language               string   `toml:"language"`
	ArchitectureRole       string   `toml:"architecture_role"`
	TestPolicy             string   `toml:"test_policy"`
	IgnoreReason           string   `toml:"ignore_reason"`
	SuiteKind              string   `toml:"suite_kind"`
	EvidenceConfidence     string   `toml:"evidence_confidence"`
	Command                string   `toml:"command"`
	ArtifactRoot           string   `toml:"artifact_root"`
	ReportRoot             string   `toml:"report_root"`
	AllowExternalArtifacts *bool    `toml:"allow_external_artifacts"`
	CoverageReports        []string `toml:"coverage_reports"`
	TestReports            []string `toml:"test_reports"`
	MutationReports        []string `toml:"mutation_reports"`
	NativeReports          []string `toml:"native_reports"`
	CoverageThreshold      *float64 `toml:"coverage_threshold"`
	NewCoverageThreshold   *float64 `toml:"new_coverage_threshold"`
	MutationThreshold      *float64 `toml:"mutation_threshold"`
	Owner                  string   `toml:"owner"`
	Team                   string   `toml:"team"`
	IntegrationRequired    *bool    `toml:"integration_required"`
}

type mutationsConfig struct {
	Enabled                *bool                   `toml:"enabled"`
	Mode                   string                  `toml:"mode"`
	Discover               *bool                   `toml:"discover"`
	Run                    *bool                   `toml:"run"`
	ChangedOnly            *bool                   `toml:"changed_only"`
	MaxRuntime             string                  `toml:"max_runtime"`
	MaxMutants             int                     `toml:"max_mutants"`
	Exclusions             []string                `toml:"exclusions"`
	MaxReportAge           string                  `toml:"max_report_age"`
	MaxDepth               int                     `toml:"max_depth"`
	MaxCandidates          int                     `toml:"max_candidates"`
	MaxReportBytes         int64                   `toml:"max_report_bytes"`
	CommandPolicy          string                  `toml:"command_policy"`
	FailOnTimeout          *bool                   `toml:"fail_on_timeout"`
	AllowExternalArtifacts *bool                   `toml:"allow_external_artifacts"`
	PathMappings           []testsPathMapping      `toml:"path_mapping"`
	Modules                []mutationsModuleConfig `toml:"modules"`
}

type mutationsModuleConfig struct {
	Name                   string             `toml:"name"`
	Root                   string             `toml:"root"`
	Language               string             `toml:"language"`
	ArchitectureRole       string             `toml:"architecture_role"`
	Tool                   string             `toml:"tool"`
	Command                string             `toml:"command"`
	SuiteKind              string             `toml:"suite_kind"`
	EvidenceConfidence     string             `toml:"evidence_confidence"`
	ArtifactRoot           string             `toml:"artifact_root"`
	ReportRoot             string             `toml:"report_root"`
	AllowExternalArtifacts *bool              `toml:"allow_external_artifacts"`
	ReportPaths            []string           `toml:"report_paths"`
	NativeReportPaths      []string           `toml:"native_report_paths"`
	PathMappings           []testsPathMapping `toml:"path_mapping"`
	Threshold              *float64           `toml:"threshold"`
	ChangedCodeThreshold   *float64           `toml:"changed_code_threshold"`
	Owner                  string             `toml:"owner"`
	Team                   string             `toml:"team"`
	MutationPolicy         string             `toml:"mutation_policy"`
	IgnoreReason           string             `toml:"ignore_reason"`
	ChangedOnly            *bool              `toml:"changed_only"`
	MaxRuntime             string             `toml:"max_runtime"`
	MaxMutants             int                `toml:"max_mutants"`
	Exclusions             []string           `toml:"exclusions"`
	FailOnTimeout          *bool              `toml:"fail_on_timeout"`
}

func parseOptions(args []string) (*appscan.ScanOptions, error) {
	filteredArgs, configPath, globalConfigPath, err := extractConfigPaths(args)
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
		if err := resolvePlaceholders(&cfg); err != nil {
			return nil, fmt.Errorf("config interpolation: %w", err)
		}
		if err := applyScannerConfig(opts, cfg.Scanner, provided); err != nil {
			return nil, err
		}
		if err := applyTestsConfig(opts, cfg.Tests, provided); err != nil {
			return nil, err
		}
		if err := applyMutationsConfig(opts, cfg.Mutations, provided); err != nil {
			return nil, err
		}
	}

	if globalConfigPath != "" {
		var globalCfg rootConfig
		if _, found, err := configfile.Load(globalConfigPath, &globalCfg); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
		} else if found {
			if err := resolvePlaceholders(&globalCfg); err != nil {
				return nil, fmt.Errorf("global config interpolation: %w", err)
			}
			applyGlobalConfig(opts, globalCfg.Scanner, provided)
		}
	}

	if err := appscan.ValidateOptions(opts); err != nil {
		return nil, err
	}
	return opts, nil
}

func applyMutationsConfig(opts *appscan.ScanOptions, cfg mutationsConfig, provided map[string]bool) error {
	applyBoolFlag(&opts.Mutations.Enabled, cfg.Enabled, provided, "with-mutations")
	applyStringFlag(&opts.Mutations.Mode, cfg.Mode, provided, "mutations-mode")
	applyBoolFlag(&opts.Mutations.Discover, cfg.Discover, provided, "mutations-discover")
	applyBoolFlag(&opts.Mutations.Run, cfg.Run, provided, "mutations-run")
	applyBoolFlag(&opts.Mutations.ChangedOnly, cfg.ChangedOnly, provided, "mutations-changed-only")
	applyBoolFlag(&opts.Mutations.FailOnTimeout, cfg.FailOnTimeout, provided, "mutations-fail-on-timeout")
	applyBoolFlag(&opts.Mutations.AllowExternalArtifacts, cfg.AllowExternalArtifacts, provided, "mutations-allow-external-artifacts")
	if opts.Mutations.Mode == appscan.MutationModeRun {
		opts.Mutations.Run = true
	}
	if err := applyDurationFlag(&opts.Mutations.MaxRuntime, cfg.MaxRuntime, provided, "mutations-max-runtime", "mutations.max_runtime"); err != nil {
		return err
	}
	if err := applyDurationFlag(&opts.Mutations.MaxReportAge, cfg.MaxReportAge, provided, "mutations-max-report-age", "mutations.max_report_age"); err != nil {
		return err
	}
	if cfg.MaxMutants > 0 {
		opts.Mutations.MaxMutants = cfg.MaxMutants
	}
	if cfg.Exclusions != nil {
		opts.Mutations.Exclusions = append([]string(nil), cfg.Exclusions...)
	}
	if cfg.MaxDepth > 0 {
		opts.Mutations.MaxDepth = cfg.MaxDepth
	}
	if cfg.MaxCandidates > 0 {
		opts.Mutations.MaxCandidates = cfg.MaxCandidates
	}
	if cfg.MaxReportBytes > 0 {
		opts.Mutations.MaxReportBytes = cfg.MaxReportBytes
	}
	applyStringFlag(&opts.Mutations.CommandPolicy, cfg.CommandPolicy, provided, "mutations-command-policy")
	opts.Mutations.PathMappings = make([]appscan.TestPathMapping, 0, len(cfg.PathMappings))
	for _, mapping := range cfg.PathMappings {
		opts.Mutations.PathMappings = append(opts.Mutations.PathMappings, appscan.TestPathMapping{From: mapping.From, To: mapping.To})
	}
	opts.Mutations.Modules = make([]appscan.MutationModuleConfig, 0, len(cfg.Modules))
	for _, module := range cfg.Modules {
		appModule, err := toAppMutationModule(module)
		if err != nil {
			return err
		}
		opts.Mutations.Modules = append(opts.Mutations.Modules, appModule)
	}
	return nil
}

func toAppMutationModule(module mutationsModuleConfig) (appscan.MutationModuleConfig, error) {
	appModule := appscan.MutationModuleConfig{
		Name:                   module.Name,
		Root:                   module.Root,
		Language:               module.Language,
		ArchitectureRole:       module.ArchitectureRole,
		Tool:                   module.Tool,
		Command:                module.Command,
		SuiteKind:              module.SuiteKind,
		EvidenceConfidence:     module.EvidenceConfidence,
		ArtifactRoot:           module.ArtifactRoot,
		ReportRoot:             module.ReportRoot,
		AllowExternalArtifacts: module.AllowExternalArtifacts,
		ReportPaths:            append([]string(nil), module.ReportPaths...),
		NativeReportPaths:      append([]string(nil), module.NativeReportPaths...),
		Threshold:              module.Threshold,
		ChangedCodeThreshold:   module.ChangedCodeThreshold,
		Owner:                  module.Owner,
		Team:                   module.Team,
		MutationPolicy:         module.MutationPolicy,
		IgnoreReason:           module.IgnoreReason,
		ChangedOnly:            module.ChangedOnly,
		MaxMutants:             module.MaxMutants,
		Exclusions:             append([]string(nil), module.Exclusions...),
		FailOnTimeout:          module.FailOnTimeout,
	}
	for _, mapping := range module.PathMappings {
		appModule.PathMappings = append(appModule.PathMappings, appscan.TestPathMapping{From: mapping.From, To: mapping.To})
	}
	if module.MaxRuntime != "" {
		duration, err := time.ParseDuration(module.MaxRuntime)
		if err != nil {
			return appscan.MutationModuleConfig{}, fmt.Errorf("parse mutations.modules.max_runtime: %w", err)
		}
		appModule.MaxRuntime = duration
	}
	return appModule, nil
}

func extractConfigPaths(args []string) ([]string, string, string, error) {
	filtered := make([]string, 0, len(args))
	var configPath, globalConfigPath string

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "-config" || arg == "--config":
			if index+1 >= len(args) {
				return nil, "", "", fmt.Errorf("missing value for %s", arg)
			}
			configPath = args[index+1]
			index++
		case strings.HasPrefix(arg, "-config="):
			configPath = strings.TrimPrefix(arg, "-config=")
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case arg == "-global-config" || arg == "--global-config":
			if index+1 >= len(args) {
				return nil, "", "", fmt.Errorf("missing value for %s", arg)
			}
			globalConfigPath = args[index+1]
			index++
		case strings.HasPrefix(arg, "-global-config="):
			globalConfigPath = strings.TrimPrefix(arg, "-global-config=")
		case strings.HasPrefix(arg, "--global-config="):
			globalConfigPath = strings.TrimPrefix(arg, "--global-config=")
		default:
			filtered = append(filtered, arg)
		}
	}

	if configPath == "" {
		configPath = os.Getenv("OLLANTA_CONFIG_FILE")
	}
	if globalConfigPath == "" {
		globalConfigPath = defaultGlobalConfigPath()
	}
	return filtered, configPath, globalConfigPath, nil
}

func defaultGlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ollanta", "config.toml")
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
	applyStringFlag(&opts.Profiles.Source, cfg.ProfileSource, provided, "profile-source")
	applyStringFlag(&opts.Profiles.FilePath, cfg.ProfileFile, provided, "profile-file")
	applyBoolFlag(&opts.Profiles.Strict, cfg.ProfileStrict, provided, "profile-strict")
	if err := applyDurationFlag(&opts.Profiles.FetchTimeout, cfg.ProfileFetchTimeout, provided, "profile-fetch-timeout", "scanner.profile_fetch_timeout"); err != nil {
		return err
	}
	applyStringFlag(&opts.Proxy, cfg.Proxy, provided, "proxy")
	applyBoolFlag(&opts.Skip, cfg.Skip, provided, "skip")
	return nil
}

func applyGlobalConfig(opts *appscan.ScanOptions, globalCfg scannerConfig, provided map[string]bool) {
	if !provided["server"] {
		applyStringFlagNoOverride(&opts.Server, globalCfg.ServerURL)
	}
	if !provided["server-token"] {
		applyStringFlagNoOverride(&opts.ServerToken, globalCfg.ServerToken)
	}
	if !provided["proxy"] {
		applyStringFlagNoOverride(&opts.Proxy, globalCfg.Proxy)
	}
}

func applyStringFlagNoOverride(dst *string, value string) {
	if value != "" && *dst == "" {
		*dst = value
	}
}

// ── placeholder interpolation ────────────────────────────────────────────────

func resolvePlaceholders(cfg *rootConfig) error {
	env := envMap()
	resolveScannerPlaceholders(&cfg.Scanner, env)
	resolveTestPlaceholders(&cfg.Tests, env)
	resolveMutationPlaceholders(&cfg.Mutations, env)
	return nil
}

func resolveScannerPlaceholders(cfg *scannerConfig, env map[string]string) {
	cfg.ProjectDir = expandValue(cfg.ProjectDir, env)
	cfg.ProjectKey = expandValue(cfg.ProjectKey, env)
	cfg.ServerURL = expandValue(cfg.ServerURL, env)
	cfg.ServerToken = expandValue(cfg.ServerToken, env)
	cfg.Proxy = expandValue(cfg.Proxy, env)
	cfg.Branch = expandValue(cfg.Branch, env)
	cfg.CommitSHA = expandValue(cfg.CommitSHA, env)
	cfg.Bind = expandValue(cfg.Bind, env)
	for i, s := range cfg.Sources {
		cfg.Sources[i] = expandValue(s, env)
	}
	for i, e := range cfg.Exclusions {
		cfg.Exclusions[i] = expandValue(e, env)
	}
}

func resolveTestPlaceholders(cfg *testsConfig, env map[string]string) {
	for i, m := range cfg.Modules {
		cfg.Modules[i].Root = expandValue(m.Root, env)
		cfg.Modules[i].Command = expandValue(m.Command, env)
	}
}

func resolveMutationPlaceholders(cfg *mutationsConfig, env map[string]string) {
	for i, m := range cfg.Modules {
		cfg.Modules[i].Root = expandValue(m.Root, env)
		cfg.Modules[i].Command = expandValue(m.Command, env)
	}
}

func expandValue(value string, env map[string]string) string {
	return os.Expand(value, func(key string) string {
		if key == "" {
			return "$"
		}
		if strings.HasPrefix(key, "env.") {
			k := strings.TrimPrefix(key, "env.")
			if idx := strings.Index(k, ":-"); idx >= 0 {
				name := k[:idx]
				def := k[idx+2:]
				if v, ok := env[name]; ok {
					return v
				}
				return def
			}
			return env[k]
		}
		if idx := strings.Index(key, ":-"); idx >= 0 {
			name := key[:idx]
			def := key[idx+2:]
			if v, ok := env[name]; ok {
				return v
			}
			return def
		}
		return env[key]
	})
}

func envMap() map[string]string {
	raw := os.Environ()
	m := make(map[string]string, len(raw))
	for _, pair := range raw {
		if idx := strings.IndexByte(pair, '='); idx >= 0 {
			m[pair[:idx]] = pair[idx+1:]
		}
	}
	return m
}

func applyTestsConfig(opts *appscan.ScanOptions, cfg testsConfig, provided map[string]bool) error {
	applyBoolFlag(&opts.Tests.Enabled, cfg.Enabled, provided, "with-tests")
	applyStringFlag(&opts.Tests.Mode, cfg.Mode, provided, "tests-mode")
	applyBoolFlag(&opts.Tests.Discover, cfg.Discover, provided, "tests-discover")
	applyBoolFlag(&opts.Tests.Run, cfg.Run, provided, "tests-run")
	applyBoolFlag(&opts.Tests.FailOnTimeout, cfg.FailOnTimeout, provided, "tests-fail-on-timeout")
	applyBoolFlag(&opts.Tests.AllowExternalArtifacts, cfg.AllowExternalArtifacts, provided, "tests-allow-external-artifacts")
	if opts.Tests.Mode == appscan.TestModeRun {
		opts.Tests.Run = true
	}
	if err := applyDurationFlag(&opts.Tests.MaxRuntime, cfg.MaxRuntime, provided, "tests-max-runtime", "tests.max_runtime"); err != nil {
		return err
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
	applyStringFlag(&opts.Tests.CommandPolicy, cfg.CommandPolicy, provided, "tests-command-policy")
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
		Name:                   module.Name,
		Root:                   module.Root,
		Language:               module.Language,
		ArchitectureRole:       module.ArchitectureRole,
		TestPolicy:             module.TestPolicy,
		IgnoreReason:           module.IgnoreReason,
		SuiteKind:              module.SuiteKind,
		EvidenceConfidence:     module.EvidenceConfidence,
		Command:                module.Command,
		ArtifactRoot:           module.ArtifactRoot,
		ReportRoot:             module.ReportRoot,
		AllowExternalArtifacts: module.AllowExternalArtifacts,
		CoverageReports:        append([]string(nil), module.CoverageReports...),
		TestReports:            append([]string(nil), module.TestReports...),
		MutationReports:        append([]string(nil), module.MutationReports...),
		NativeReports:          append([]string(nil), module.NativeReports...),
		CoverageThreshold:      module.CoverageThreshold,
		NewCoverageThreshold:   module.NewCoverageThreshold,
		MutationThreshold:      module.MutationThreshold,
		Owner:                  module.Owner,
		Team:                   module.Team,
		IntegrationRequired:    integrationRequired,
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
