// Package scan provides the top-level scan orchestration: flag parsing, pipeline
// execution, and terminal output.  There are no external dependencies — only the
// standard library flag package is used for CLI argument parsing.
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

	parlanguages "github.com/scovl/ollanta/ollantaparser/languages"
	"github.com/scovl/ollanta/ollantarules/defaults"
	gosensor "github.com/scovl/ollanta/ollantarules/languages/golang"
	tssensor "github.com/scovl/ollanta/ollantarules/languages/treesitter"
	"github.com/scovl/ollanta/ollantascanner/discovery"
	"github.com/scovl/ollanta/ollantascanner/executor"
	"github.com/scovl/ollanta/ollantascanner/report"
)

// ScanOptions holds every parameter that controls a scan run.
type ScanOptions struct {
	ProjectDir string
	Sources    []string // source directory patterns (Go-style ./... accepted)
	Exclusions []string // glob patterns relative to ProjectDir
	ProjectKey string
	Format     string // "summary" | "json" | "sarif" | "all"
	Debug      bool
	Serve      bool   // open local web UI after scan
	Port       int    // port for -serve (default 7777)
	Bind       string // bind address for -serve (default 127.0.0.1)
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
	serverURL   := fs.String("server", "", "URL of ollantaweb server to push results to (e.g. http://localhost:8080)")
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
		ProjectDir: *projectDir,
		Format:     *format,
		Debug:      *debug,
		Serve:      *serve,
		Port:       *port,
		Bind:       *bind,
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

// Run executes the full scan pipeline: discover → analyze → report.
// It returns the assembled Report so callers can save or inspect it.
func Run(ctx context.Context, opts *ScanOptions) (*report.Report, error) {
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
	files, err := discovery.Discover(opts.ProjectDir, opts.Sources, opts.Exclusions)
	if err != nil {
		return nil, fmt.Errorf("discovery: %w", err)
	}
	if opts.Debug {
		fmt.Fprintf(os.Stderr, "[debug] discovered %d files\n", len(files))
	}

	// 2. Build sensors
	reg := defaults.NewRegistry()
	parserReg := parlanguages.DefaultRegistry()
	goS := gosensor.NewGoSensor(reg)
	tsS := tssensor.NewTreeSitterSensor(reg, parserReg)

	// 3. Analyze in parallel
	exec := executor.New(goS, tsS)
	issues, err := exec.Run(ctx, files)
	if err != nil {
		return nil, fmt.Errorf("executor: %w", err)
	}

	// 4. Assemble report
	return report.Build(opts.ProjectKey, files, issues, time.Since(start)), nil
}

// PrintSummary writes a human-readable scan summary to w (usually os.Stdout).
func PrintSummary(r *report.Report) {
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
		fmt.Println("\nFiles by language:")
		langs := make([]string, 0, len(r.Measures.ByLang))
		for l := range r.Measures.ByLang {
			langs = append(langs, l)
		}
		sort.Strings(langs)
		for _, l := range langs {
			fmt.Printf("  %-15s %d\n", l+":", r.Measures.ByLang[l])
		}
	}

	if len(r.Issues) > 0 {
		bySev := map[string]int{}
		for _, iss := range r.Issues {
			bySev[string(iss.Severity)]++
		}
		fmt.Println("\nIssues by severity:")
		for _, sev := range []string{"blocker", "critical", "major", "minor", "info"} {
			if n := bySev[sev]; n > 0 {
				fmt.Printf("  %-10s %d\n", sev+":", n)
			}
		}
	}

	fmt.Printf("\nElapsed: %dms\n", r.Metadata.ElapsedMs)
}
