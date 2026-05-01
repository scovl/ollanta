// Package scan provides the top-level scan use case: flag parsing, pipeline
// execution via injected IParser/IAnalyzer ports, and terminal output.
package scan

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/scovl/ollanta/domain/port"
)

// ScanOptions holds every parameter that controls a scan run.
type ScanOptions struct {
	ProjectDir        string
	Sources           []string // source directory patterns (Go-style ./... accepted)
	Exclusions        []string // glob patterns relative to ProjectDir
	ProjectKey        string
	Branch            string
	CommitSHA         string
	PullRequestKey    string
	PullRequestBranch string
	PullRequestBase   string
	Format            string // "summary" | "json" | "sarif" | "all"
	Debug             bool
	Serve             bool          // open local web UI after scan
	Port              int           // port for -local-ui (default 7777)
	Bind              string        // bind address for -local-ui (default 127.0.0.1)
	Server            string        // URL of ollantaweb server for push mode (empty = disabled)
	ServerToken       string        // Bearer token for authenticating with ollantaweb
	ServerWait        bool          // wait for accepted server job until completion
	WaitTimeout       time.Duration // maximum time to wait for server-side job completion
	WaitPoll          time.Duration // polling interval while waiting for server-side job completion
}

// ParseFlags parses args (typically os.Args[1:]) into ScanOptions.
// Returns an error if flag parsing fails (e.g. unknown flag).
func ParseFlags(args []string) (*ScanOptions, error) {
	fs := flag.NewFlagSet("ollanta", flag.ContinueOnError)

	projectDir := fs.String("project-dir", ".", "Root directory to scan")
	sources := fs.String("sources", "./...", "Comma-separated source patterns")
	exclusions := fs.String("exclusions", "", "Comma-separated glob patterns to exclude")
	projectKey := fs.String("project-key", "", "Project identifier (default: directory base name)")
	branch := fs.String("branch", "", "Explicit branch override for the analysis scope")
	commitSHA := fs.String("commit-sha", "", "Explicit commit SHA override for the analysis scope")
	pullRequestKey := fs.String("pull-request-key", "", "Explicit pull request key for pull request analysis")
	pullRequestBranch := fs.String("pull-request-branch", "", "Explicit source branch for pull request analysis")
	pullRequestBase := fs.String("pull-request-base", "", "Explicit target/base branch for pull request analysis")
	format := fs.String("format", "all", "Output format: summary, json, sarif, all")
	debug := fs.Bool("debug", false, "Enable debug output")
	localUI := fs.Bool("local-ui", false, "Open the embedded local web UI after scan")
	port := fs.Int("port", 7777, "Port for -local-ui")
	bind := fs.String("bind", "127.0.0.1", "Bind address for -local-ui (use 0.0.0.0 inside Docker)")
	serverURL := fs.String("server", "", "URL of ollantaweb server to push results to (e.g. http://localhost:8080)")
	serverToken := fs.String("server-token", "", "API token for authenticating with ollantaweb (Bearer)")
	serverWait := fs.Bool("server-wait", false, "Wait for an accepted server-side scan job to complete")
	waitTimeout := fs.Duration("server-wait-timeout", 10*time.Minute, "Maximum time to wait for a server-side scan job")
	waitPoll := fs.Duration("server-wait-poll", 2*time.Second, "Polling interval while waiting for a server-side scan job")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	switch *format {
	case "summary", "json", "sarif", "all":
		// valid
	default:
		return nil, fmt.Errorf("unknown format %q: expected summary, json, sarif, or all", *format)
	}

	opts := &ScanOptions{
		ProjectDir:        *projectDir,
		Branch:            *branch,
		CommitSHA:         *commitSHA,
		PullRequestKey:    *pullRequestKey,
		PullRequestBranch: *pullRequestBranch,
		PullRequestBase:   *pullRequestBase,
		Format:            *format,
		Debug:             *debug,
		Serve:             *localUI,
		Port:              *port,
		Bind:              *bind,
		Server:            *serverURL,
		ServerToken:       *serverToken,
		ServerWait:        *serverWait,
		WaitTimeout:       *waitTimeout,
		WaitPoll:          *waitPoll,
	}

	for _, s := range strings.Split(*sources, ",") {
		if s := strings.TrimSpace(s); s != "" {
			opts.Sources = append(opts.Sources, s)
		}
	}
	for _, s := range strings.Split(*exclusions, ",") {
		if s := strings.TrimSpace(s); s != "" {
			opts.Exclusions = append(opts.Exclusions, s)
		}
	}

	if *projectKey != "" {
		opts.ProjectKey = *projectKey
	} else {
		abs, err := filepath.Abs(*projectDir)
		if err != nil {
			abs = *projectDir
		}
		opts.ProjectKey = filepath.Base(abs)
	}

	return opts, nil
}

// ScanUseCase orchestrates: discover → parse/analyse → report.
// All concrete dependencies (parser, analyzers) are injected at construction time.
type ScanUseCase struct {
	executor *Executor
}

// NewScanUseCase creates a ScanUseCase backed by the provided parser and analyzers.
func NewScanUseCase(p port.IParser, analyzers []port.IAnalyzer) *ScanUseCase {
	return &ScanUseCase{executor: NewExecutor(p, analyzers)}
}

// Run executes the full scan pipeline: discover → analyze → report.
// It returns the assembled Report so callers can save or inspect it.
func (uc *ScanUseCase) Run(ctx context.Context, opts *ScanOptions) (*Report, error) {
	start := time.Now()

	abs, err := filepath.Abs(opts.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}
	opts.ProjectDir = abs

	if opts.Debug {
		fmt.Fprintf(os.Stderr, "[debug] project dir: %s\n", opts.ProjectDir)
	}

	scmCtx, err := resolveSCMContext(opts)
	if err != nil {
		return nil, err
	}
	if opts.Debug && (scmCtx.Branch != "" || scmCtx.CommitSHA != "" || scmCtx.PullRequestKey != "") {
		fmt.Fprintf(os.Stderr, "[debug] scope=%s branch=%s commit=%s pr=%s\n",
			scmCtx.ScopeType, scmCtx.Branch, scmCtx.CommitSHA, scmCtx.PullRequestKey)
	}

	// 1. Discover files
	files, err := Discover(opts.ProjectDir, opts.Sources, opts.Exclusions)
	if err != nil {
		return nil, fmt.Errorf("discovery: %w", err)
	}
	if opts.Debug {
		fmt.Fprintf(os.Stderr, "[debug] discovered %d files\n", len(files))
	}

	// 2. Analyze in parallel
	issues, err := uc.executor.Run(ctx, files)
	if err != nil {
		return nil, fmt.Errorf("executor: %w", err)
	}

	// 3. Assemble report
	return Build(opts.ProjectKey, opts.ProjectDir, files, issues, time.Since(start), Metadata{
		ProjectKey:      opts.ProjectKey,
		Version:         Version,
		ElapsedMs:       time.Since(start).Milliseconds(),
		ScopeType:       scmCtx.ScopeType,
		Branch:          scmCtx.Branch,
		CommitSHA:       scmCtx.CommitSHA,
		PullRequestKey:  scmCtx.PullRequestKey,
		PullRequestBase: scmCtx.PullRequestBase,
	}), nil
}

// PrintSummary writes a human-readable scan summary to stdout.
func PrintSummary(r *Report) {
	fmt.Printf("\nOllanta Scanner %s\n", r.Metadata.Version)
	fmt.Printf("Project:  %s\n", r.Metadata.ProjectKey)
	fmt.Printf("Date:     %s\n\n", r.Metadata.AnalysisDate)

	fmt.Printf("Files:         %d\n", r.Measures.Files)
	fmt.Printf("Lines:         %d\n", r.Measures.Lines)
	fmt.Printf("NCLOC:         %d\n", r.Measures.Ncloc)
	fmt.Printf("Comment lines: %d\n", r.Measures.Comments)
	fmt.Printf("Issues:        %d\n", len(r.Issues))
	if r.Measures.Bugs > 0 || r.Measures.CodeSmells > 0 || r.Measures.Vulnerabilities > 0 {
		fmt.Printf("Bugs: %d  Code Smells: %d  Vulnerabilities: %d\n",
			r.Measures.Bugs, r.Measures.CodeSmells, r.Measures.Vulnerabilities)
	}

	if len(r.Measures.ByLang) > 0 {
		fmt.Println("\nBy language:")
		langs := make([]string, 0, len(r.Measures.ByLang))
		for l := range r.Measures.ByLang {
			langs = append(langs, l)
		}
		sort.Strings(langs)
		for _, l := range langs {
			fmt.Printf("  %-15s %d files\n", l, r.Measures.ByLang[l])
		}
	}
}
