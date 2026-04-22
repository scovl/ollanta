package scan

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const scannerConfigFileEnv = "OLLANTA_CONFIG_FILE"

type scannerFlagDefaults struct {
	ProjectDir        string
	Sources           string
	Exclusions        string
	ProjectKey        string
	Branch            string
	CommitSHA         string
	PullRequestKey    string
	PullRequestBranch string
	PullRequestBase   string
	Format            string
	Debug             bool
	Serve             bool
	Port              int
	Bind              string
	Server            string
	ServerToken       string
	ServerWait        bool
	WaitTimeout       time.Duration
	WaitPoll          time.Duration
}

type scannerFileConfig struct {
	Scanner struct {
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
		Serve             *bool    `toml:"serve"`
		Port              *int     `toml:"port"`
		Bind              string   `toml:"bind"`
		Server            string   `toml:"server"`
		ServerToken       string   `toml:"server_token"`
		ServerWait        *bool    `toml:"server_wait"`
		WaitTimeout       string   `toml:"server_wait_timeout"`
		WaitPoll          string   `toml:"server_wait_poll"`
	} `toml:"scanner"`
	Server struct {
		Addr      string `toml:"addr"`
		Host      string `toml:"host"`
		Port      int    `toml:"port"`
		PublicURL string `toml:"public_url"`
	} `toml:"server"`
}

func defaultScannerFlagDefaults() scannerFlagDefaults {
	return scannerFlagDefaults{
		ProjectDir:  ".",
		Sources:     "./...",
		Exclusions:  "",
		Format:      "all",
		Port:        7777,
		Bind:        "127.0.0.1",
		WaitTimeout: 10 * time.Minute,
		WaitPoll:    2 * time.Second,
	}
}

func loadScannerFlagDefaults(configPath string) (scannerFlagDefaults, error) {
	defaults := defaultScannerFlagDefaults()
	if configPath == "" {
		return defaults, nil
	}

	var cfg scannerFileConfig
	if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
		return scannerFlagDefaults{}, fmt.Errorf("load scanner config %q: %w", configPath, err)
	}

	if cfg.Scanner.ProjectDir != "" {
		defaults.ProjectDir = cfg.Scanner.ProjectDir
	}
	if len(cfg.Scanner.Sources) > 0 {
		defaults.Sources = strings.Join(cfg.Scanner.Sources, ",")
	}
	if len(cfg.Scanner.Exclusions) > 0 {
		defaults.Exclusions = strings.Join(cfg.Scanner.Exclusions, ",")
	}
	if cfg.Scanner.ProjectKey != "" {
		defaults.ProjectKey = cfg.Scanner.ProjectKey
	}
	if cfg.Scanner.Branch != "" {
		defaults.Branch = cfg.Scanner.Branch
	}
	if cfg.Scanner.CommitSHA != "" {
		defaults.CommitSHA = cfg.Scanner.CommitSHA
	}
	if cfg.Scanner.PullRequestKey != "" {
		defaults.PullRequestKey = cfg.Scanner.PullRequestKey
	}
	if cfg.Scanner.PullRequestBranch != "" {
		defaults.PullRequestBranch = cfg.Scanner.PullRequestBranch
	}
	if cfg.Scanner.PullRequestBase != "" {
		defaults.PullRequestBase = cfg.Scanner.PullRequestBase
	}
	if cfg.Scanner.Format != "" {
		defaults.Format = cfg.Scanner.Format
	}
	if cfg.Scanner.Debug != nil {
		defaults.Debug = *cfg.Scanner.Debug
	}
	if cfg.Scanner.Serve != nil {
		defaults.Serve = *cfg.Scanner.Serve
	}
	if cfg.Scanner.Port != nil {
		defaults.Port = *cfg.Scanner.Port
	}
	if cfg.Scanner.Bind != "" {
		defaults.Bind = cfg.Scanner.Bind
	}
	if cfg.Scanner.Server != "" {
		defaults.Server = cfg.Scanner.Server
	} else if serverURL := resolveScannerServerURL(cfg.Server.Addr, cfg.Server.Host, cfg.Server.Port, cfg.Server.PublicURL); serverURL != "" {
		defaults.Server = serverURL
	}
	if cfg.Scanner.ServerToken != "" {
		defaults.ServerToken = cfg.Scanner.ServerToken
	}
	if cfg.Scanner.ServerWait != nil {
		defaults.ServerWait = *cfg.Scanner.ServerWait
	}
	if cfg.Scanner.WaitTimeout != "" {
		value, err := time.ParseDuration(cfg.Scanner.WaitTimeout)
		if err != nil {
			return scannerFlagDefaults{}, fmt.Errorf("parse scanner.server_wait_timeout: %w", err)
		}
		defaults.WaitTimeout = value
	}
	if cfg.Scanner.WaitPoll != "" {
		value, err := time.ParseDuration(cfg.Scanner.WaitPoll)
		if err != nil {
			return scannerFlagDefaults{}, fmt.Errorf("parse scanner.server_wait_poll: %w", err)
		}
		defaults.WaitPoll = value
	}

	return defaults, nil
}

func detectScannerConfigPath(args []string) string {
	configPath := os.Getenv(scannerConfigFileEnv)
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-config" || arg == "--config":
			if i+1 < len(args) {
				configPath = args[i+1]
			}
		case strings.HasPrefix(arg, "-config="):
			configPath = strings.TrimPrefix(arg, "-config=")
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		}
	}
	return configPath
}

func resolveScannerServerURL(addr, host string, port int, publicURL string) string {
	if publicURL != "" {
		return publicURL
	}
	if host != "" && port > 0 {
		return "http://" + net.JoinHostPort(normalizeScannerServerHost(host), strconv.Itoa(port))
	}
	if addr == "" {
		return ""
	}
	resolvedHost, resolvedPort, err := net.SplitHostPort(addr)
	if err != nil || resolvedPort == "" {
		return ""
	}
	return "http://" + net.JoinHostPort(normalizeScannerServerHost(resolvedHost), resolvedPort)
}

func normalizeScannerServerHost(host string) string {
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0", "::":
		return "localhost"
	default:
		return host
	}
}
