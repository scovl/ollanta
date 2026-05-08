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
	ConfigPath        string
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
	Serve             bool           // open local web UI after scan
	Port              int            // port for -local-ui (default 7777)
	Bind              string         // bind address for -local-ui (default 127.0.0.1)
	Server            string         // URL of ollantaweb server for push mode (empty = disabled)
	ServerToken       string         // Bearer token for authenticating with ollantaweb
	ServerWait        bool           // wait for accepted server job until completion
	WaitTimeout       time.Duration  // maximum time to wait for server-side job completion
	WaitPoll          time.Duration  // polling interval while waiting for server-side job completion
	Proxy             string         // HTTP(S) proxy URL for server push
	Skip              bool           // exit immediately without scanning
	Profiles          ProfileOptions // effective quality profile loading and enforcement
	CustomRules       CustomRuleOptions
	Tests             TestOptions     // optional test-signal discovery and collection
	Mutations         MutationOptions // optional mutation-signal discovery and collection
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
	profileSource := fs.String("profile-source", ProfileSourceAuto, "Quality profile source: auto, local, server, or builtin")
	profileFile := fs.String("profile-file", "", "Path to a local profile-as-code JSON file")
	profileStrict := fs.Bool("profile-strict", false, "Fail the scan when the requested quality profile cannot be loaded")
	profileFetchTimeout := fs.Duration("profile-fetch-timeout", 10*time.Second, "Maximum time to fetch effective profiles from ollantaweb")
	withTests := fs.Bool("with-tests", false, "Enable test signal discovery and report collection without running tests")
	testsMode := fs.String("tests-mode", TestModeCollect, "Test signal mode: collect, run, or doctor")
	testsRun := fs.Bool("tests-run", false, "Explicitly allow configured test commands to run")
	testsMaxRuntime := fs.Duration("tests-max-runtime", 10*time.Minute, "Maximum time for an opt-in test command")
	testsFailOnTimeout := fs.Bool("tests-fail-on-timeout", false, "Treat test command timeout as a failed scan instead of collecting partial reports")
	testsCommandPolicy := fs.String("tests-command-policy", CommandPolicyExplicit, "Test command policy: explicit, never, configured_only, discovered, or trusted_shell")
	testsAllowExternalArtifacts := fs.Bool("tests-allow-external-artifacts", false, "Allow configured test reports outside the project directory")
	withMutations := fs.Bool("with-mutations", false, "Enable mutation signal discovery and report collection without running mutation tools")
	mutationsMode := fs.String("mutations-mode", MutationModeCollect, "Mutation signal mode: collect, run, or doctor")
	mutationsRun := fs.Bool("mutations-run", false, "Explicitly allow configured mutation commands to run")
	mutationsDiscover := fs.Bool("mutations-discover", true, "Discover mutation tools and report candidates")
	mutationsChangedOnly := fs.Bool("mutations-changed-only", true, "Prefer changed-code mutation scope when executing mutation commands")
	mutationsMaxRuntime := fs.Duration("mutations-max-runtime", 10*time.Minute, "Maximum time for an opt-in mutation command")
	mutationsMaxMutants := fs.Int("mutations-max-mutants", 0, "Maximum mutants for opt-in mutation execution when supported by the tool")
	mutationsMaxReportAge := fs.Duration("mutations-max-report-age", 24*time.Hour, "Maximum age for mutation reports before they are marked stale")
	mutationsCommandPolicy := fs.String("mutations-command-policy", CommandPolicyExplicit, "Mutation command policy: explicit, never, configured_only, discovered, or trusted_shell")
	mutationsFailOnTimeout := fs.Bool("mutations-fail-on-timeout", false, "Treat mutation command timeout as a failed scan instead of collecting partial reports")
	mutationsAllowExternalArtifacts := fs.Bool("mutations-allow-external-artifacts", false, "Allow configured mutation reports outside the project directory")
	proxy := fs.String("proxy", "", "HTTP(S) proxy URL for server push (e.g. http://proxy:3128)")
	skip := fs.Bool("skip", false, "Exit immediately without scanning")

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
		Proxy:             *proxy,
		Skip:              *skip,
		Profiles: ProfileOptions{
			Source:       *profileSource,
			FilePath:     *profileFile,
			Strict:       *profileStrict,
			FetchTimeout: *profileFetchTimeout,
		},
		Tests: TestOptions{
			Enabled:                *withTests,
			Mode:                   *testsMode,
			Discover:               true,
			Run:                    *testsRun || *testsMode == TestModeRun,
			MaxRuntime:             *testsMaxRuntime,
			FailOnTimeout:          *testsFailOnTimeout,
			CommandPolicy:          *testsCommandPolicy,
			AllowExternalArtifacts: *testsAllowExternalArtifacts,
		},
		Mutations: MutationOptions{
			Enabled:                *withMutations,
			Mode:                   *mutationsMode,
			Discover:               *mutationsDiscover,
			Run:                    *mutationsRun || *mutationsMode == MutationModeRun,
			ChangedOnly:            *mutationsChangedOnly,
			MaxRuntime:             *mutationsMaxRuntime,
			MaxMutants:             *mutationsMaxMutants,
			MaxReportAge:           *mutationsMaxReportAge,
			CommandPolicy:          *mutationsCommandPolicy,
			FailOnTimeout:          *mutationsFailOnTimeout,
			AllowExternalArtifacts: *mutationsAllowExternalArtifacts,
		},
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

	if err := ValidateOptions(opts); err != nil {
		return nil, err
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

	profilePolicy, err := ResolveProfilePolicy(ctx, opts, discoveredLanguages(files))
	if err != nil {
		return nil, fmt.Errorf("quality profiles: %w", err)
	}
	if opts.Debug {
		for _, diagnostic := range profilePolicy.Diagnostics() {
			fmt.Fprintf(os.Stderr, "[debug] profile %s: %s\n", diagnostic.Language, diagnostic.Message)
		}
	}

	// 2. Analyze in parallel
	issues, err := uc.executor.Run(ctx, files, profilePolicy)
	if err != nil {
		return nil, fmt.Errorf("executor: %w", err)
	}

	// 3. Assemble report
	report := Build(opts.ProjectKey, opts.ProjectDir, files, issues, time.Since(start), Metadata{
		ProjectKey:      opts.ProjectKey,
		Version:         Version,
		ElapsedMs:       time.Since(start).Milliseconds(),
		ScopeType:       scmCtx.ScopeType,
		Branch:          scmCtx.Branch,
		CommitSHA:       scmCtx.CommitSHA,
		PullRequestKey:  scmCtx.PullRequestKey,
		PullRequestBase: scmCtx.PullRequestBase,
	})
	report.QualityProfiles = profilePolicy.Snapshots()
	report.ScannerOptions = scannerOptionsFromScanOptions(opts)
	if err := collectReportTestSignals(report, opts, start); err != nil {
		return nil, err
	}
	return report, nil
}

func collectReportTestSignals(report *Report, opts *ScanOptions, scanStarted time.Time) error {
	if opts.Tests.Enabled {
		testSignals, err := CollectTestSignals(opts.ProjectDir, opts.Tests, scanStarted)
		if err != nil {
			return fmt.Errorf("test signals: %w", err)
		}
		report.TestSignals = testSignals
	}
	if opts.Mutations.Enabled {
		if opts.Mutations.ChangedOnly && len(opts.Mutations.ChangedFiles) == 0 {
			if files, err := ResolveChangedFiles(opts.ProjectDir, opts.PullRequestBase); err == nil && len(files) > 0 {
				opts.Mutations.ChangedFiles = files
			}
		}
		mutationSignals, err := CollectMutationSignals(opts.ProjectDir, opts.Mutations, scanStarted)
		if err != nil {
			return fmt.Errorf("mutation signals: %w", err)
		}
		report.TestSignals = mergeMutationSignals(report.TestSignals, mutationSignals)
	}
	if report.TestSignals != nil {
		applyTestSignalMeasures(report, report.TestSignals)
	}
	return nil
}

func scannerOptionsFromScanOptions(opts *ScanOptions) ScannerOptions {
	if opts == nil {
		return ScannerOptions{}
	}
	return ScannerOptions{
		ConfigPath:        opts.ConfigPath,
		ProjectDir:        opts.ProjectDir,
		Sources:           append([]string(nil), opts.Sources...),
		Exclusions:        append([]string(nil), opts.Exclusions...),
		ProjectKey:        opts.ProjectKey,
		Branch:            opts.Branch,
		CommitSHA:         opts.CommitSHA,
		PullRequestKey:    opts.PullRequestKey,
		PullRequestBranch: opts.PullRequestBranch,
		PullRequestBase:   opts.PullRequestBase,
		Format:            opts.Format,
		Debug:             opts.Debug,
		LocalUI:           opts.Serve,
		Port:              opts.Port,
		Bind:              opts.Bind,
		Server:            opts.Server,
		ServerWait:        opts.ServerWait,
		WaitTimeout:       opts.WaitTimeout.String(),
		WaitPoll:          opts.WaitPoll.String(),
		Profiles:          scannerProfileOptionsFromProfileOptions(opts.Profiles),
		CustomRules:       scannerCustomRuleOptionsFromCustomRuleOptions(opts.CustomRules),
		Tests:             scannerTestOptionsFromTestOptions(opts.Tests),
		Mutations:         scannerMutationOptionsFromMutationOptions(opts.Mutations),
	}
}

func scannerCustomRuleOptionsFromCustomRuleOptions(opts CustomRuleOptions) ScannerCustomRuleOptions {
	return ScannerCustomRuleOptions{CatalogHash: opts.CatalogHash, RuleCount: len(opts.Rules), Sources: append([]string(nil), opts.Sources...)}
}

func scannerProfileOptionsFromProfileOptions(opts ProfileOptions) ScannerProfileOptions {
	profileOptions := ScannerProfileOptions{
		Source:   opts.Source,
		FilePath: opts.FilePath,
		Strict:   opts.Strict,
	}
	if opts.FetchTimeout > 0 {
		profileOptions.FetchTimeout = opts.FetchTimeout.String()
	}
	return profileOptions
}

func discoveredLanguages(files []DiscoveredFile) []string {
	seen := map[string]bool{}
	for _, file := range files {
		if file.Language != "" && !seen[file.Language] {
			seen[file.Language] = true
		}
	}
	out := make([]string, 0, len(seen))
	for language := range seen {
		out = append(out, language)
	}
	sort.Strings(out)
	return out
}

func scannerTestOptionsFromTestOptions(opts TestOptions) ScannerTestOptions {
	testOptions := ScannerTestOptions{
		Enabled:                opts.Enabled,
		Mode:                   opts.Mode,
		Discover:               opts.Discover,
		Run:                    opts.Run,
		FailOnTimeout:          opts.FailOnTimeout,
		Exclusions:             append([]string(nil), opts.Exclusions...),
		MaxDepth:               opts.MaxDepth,
		MaxCandidates:          opts.MaxCandidates,
		MaxReportBytes:         opts.MaxReportBytes,
		CommandPolicy:          opts.CommandPolicy,
		AllowExternalArtifacts: opts.AllowExternalArtifacts,
		PathMappings:           append([]TestPathMapping(nil), opts.PathMappings...),
		Modules:                make([]ScannerTestModuleOptions, 0, len(opts.Modules)),
	}
	if opts.MaxReportAge > 0 {
		testOptions.MaxReportAge = opts.MaxReportAge.String()
	}
	if opts.MaxRuntime > 0 {
		testOptions.MaxRuntime = opts.MaxRuntime.String()
	}
	for _, module := range opts.Modules {
		testOptions.Modules = append(testOptions.Modules, ScannerTestModuleOptions{
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
			IntegrationRequired:    module.IntegrationRequired,
		})
	}
	return testOptions
}

func scannerMutationOptionsFromMutationOptions(opts MutationOptions) ScannerMutationOptions {
	if !opts.Enabled {
		return ScannerMutationOptions{}
	}
	applyMutationDefaults(&opts)
	mutationOptions := ScannerMutationOptions{
		Enabled:                opts.Enabled,
		Mode:                   opts.Mode,
		Discover:               opts.Discover,
		Run:                    opts.Run,
		ChangedOnly:            opts.ChangedOnly,
		MaxMutants:             opts.MaxMutants,
		Exclusions:             append([]string(nil), opts.Exclusions...),
		MaxDepth:               opts.MaxDepth,
		MaxCandidates:          opts.MaxCandidates,
		MaxReportBytes:         opts.MaxReportBytes,
		CommandPolicy:          opts.CommandPolicy,
		FailOnTimeout:          opts.FailOnTimeout,
		AllowExternalArtifacts: opts.AllowExternalArtifacts,
		PathMappings:           append([]TestPathMapping(nil), opts.PathMappings...),
		Modules:                make([]ScannerMutationModuleOptions, 0, len(opts.Modules)),
	}
	if opts.MaxRuntime > 0 {
		mutationOptions.MaxRuntime = opts.MaxRuntime.String()
	}
	if opts.MaxReportAge > 0 {
		mutationOptions.MaxReportAge = opts.MaxReportAge.String()
	}
	for _, module := range opts.Modules {
		moduleOptions := ScannerMutationModuleOptions{
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
			PathMappings:           append([]TestPathMapping(nil), module.PathMappings...),
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
		if module.MaxRuntime > 0 {
			moduleOptions.MaxRuntime = module.MaxRuntime.String()
		}
		mutationOptions.Modules = append(mutationOptions.Modules, moduleOptions)
	}
	return mutationOptions
}

func applyTestSignalMeasures(report *Report, testSignals *TestSignalReport) {
	if report == nil || testSignals == nil {
		return
	}
	report.Measures.Coverage = testSignals.Summary.Coverage
	report.Measures.Tests = testSignals.Summary.Tests
	report.Measures.TestFailures = testSignals.Summary.TestFailures
	report.Measures.TestErrors = testSignals.Summary.TestErrors
	report.Measures.TestSkipped = testSignals.Summary.TestSkipped
	report.Measures.TestDurationMs = testSignals.Summary.TestDurationMs
	report.Measures.MutationScore = testSignals.Summary.MutationScore
	report.Measures.MutantsTotal = testSignals.Summary.MutantsTotal
	report.Measures.MutantsKilled = testSignals.Summary.MutantsKilled
	report.Measures.MutantsSurvived = testSignals.Summary.MutantsSurvived
	report.Measures.MutantsTimeout = testSignals.Summary.MutantsTimeout
	report.Measures.MutantsSkipped = testSignals.Summary.MutantsSkipped
	report.Measures.MutantsError = testSignals.Summary.MutantsError
	report.Measures.ChangedMutationScore = testSignals.Summary.ChangedMutationScore
	report.Measures.ChangedMutantsTotal = testSignals.Summary.ChangedMutantsTotal
	report.Measures.ChangedMutantsKilled = testSignals.Summary.ChangedMutantsKilled
	report.Measures.ChangedMutantsSurvived = testSignals.Summary.ChangedMutantsSurvived
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
	if r.Measures.Coverage != nil {
		fmt.Printf("Coverage:      %.1f%%\n", *r.Measures.Coverage)
	}
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
