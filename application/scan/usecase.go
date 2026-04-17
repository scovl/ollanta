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
	ProjectDir  string
	Sources     []string // source directory patterns (Go-style ./... accepted)
	Exclusions  []string // glob patterns relative to ProjectDir
	ProjectKey  string
	Format      string // "summary" | "json" | "sarif" | "all"
	Debug       bool
	Serve       bool   // open local web UI after scan
	Port        int    // port for -serve (default 7777)
	Bind        string // bind address for -serve (default 127.0.0.1)
	Server      string // URL of ollantaweb server for push mode (empty = disabled)
	ServerToken string // Bearer token for authenticating with ollantaweb
}

// ParseFlags parses args (typically os.Args[1:]) into ScanOptions.
// Returns an error if flag parsing fails (e.g. unknown flag).
func ParseFlags(args []string) (*ScanOptions, error) {
	fs := flag.NewFlagSet("ollanta", flag.ContinueOnError)

	projectDir := fs.String("project-dir", ".", "Root directory to scan")
	sources := fs.String("sources", "./...", "Comma-separated source patterns")
	exclusions := fs.String("exclusions", "", "Comma-separated glob patterns to exclude")
	projectKey := fs.String("project-key", "", "Project identifier (default: directory base name)")
	format := fs.String("format", "all", "Output format: summary, json, sarif, all")
	debug := fs.Bool("debug", false, "Enable debug output")
	serve := fs.Bool("serve", false, "Open interactive web UI after scan")
	port := fs.Int("port", 7777, "Port for -serve")
	bind := fs.String("bind", "127.0.0.1", "Bind address for -serve (use 0.0.0.0 inside Docker)")
	serverURL := fs.String("server", "", "URL of ollantaweb server to push results to (e.g. http://localhost:8080)")
	serverToken := fs.String("server-token", "", "API token for authenticating with ollantaweb (Bearer)")

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
		ProjectDir:  *projectDir,
		Format:      *format,
		Debug:       *debug,
		Serve:       *serve,
		Port:        *port,
		Bind:        *bind,
		Server:      *serverURL,
		ServerToken: *serverToken,
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
	return Build(opts.ProjectKey, files, issues, time.Since(start)), nil
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
