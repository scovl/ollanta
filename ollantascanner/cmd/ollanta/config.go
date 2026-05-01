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
		if err := applyScannerConfig(opts, cfg.Scanner, provided); err != nil {
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
