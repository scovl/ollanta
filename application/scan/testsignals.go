package scan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	TestModeCollect = "collect"
	TestModeRun     = "run"
	TestModeDoctor  = "doctor"

	TestPolicyRequired = "required"
	TestPolicyOptional = "optional"
	TestPolicyIgnored  = "ignored"

	TestSourceConfigured = "configured"
	TestSourceDiscovered = "discovered"

	packageJSONFile      = "package.json"
	junitReportFile      = "junit.xml"
	nativeTestReportFile = "ollanta-tests.json"
)

var defaultTestExcludedDirs = map[string]bool{
	".git":          true,
	".hg":           true,
	".svn":          true,
	".ollanta":      true,
	"node_modules":  true,
	"vendor":        true,
	"dist":          true,
	"build":         true,
	"target":        true,
	"coverage":      false,
	"__pycache__":   true,
	".pytest_cache": true,
	".mypy_cache":   true,
}

// TestOptions controls optional test-signal discovery and collection.
type TestOptions struct {
	Enabled        bool
	Mode           string
	Discover       bool
	Run            bool
	MaxReportAge   time.Duration
	Exclusions     []string
	MaxDepth       int
	MaxCandidates  int
	MaxReportBytes int64
	CommandPolicy  string
	PathMappings   []TestPathMapping
	Modules        []TestModuleConfig
}

// TestPathMapping maps paths found in reports back to the scanner workspace.
type TestPathMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// TestModuleConfig is an explicit module override from configuration.
type TestModuleConfig struct {
	Name                 string
	Root                 string
	Language             string
	ArchitectureRole     string
	TestPolicy           string
	IgnoreReason         string
	Command              string
	ArtifactRoot         string
	ReportRoot           string
	CoverageReports      []string
	TestReports          []string
	MutationReports      []string
	NativeReports        []string
	CoverageThreshold    *float64
	NewCoverageThreshold *float64
	MutationThreshold    *float64
	Owner                string
	Team                 string
	IntegrationRequired  bool
}

// TestSignalReport is the normalized optional test payload emitted by the scanner.
type TestSignalReport struct {
	Summary      TestSignalSummary      `json:"summary"`
	Modules      []TestModuleSignal     `json:"modules"`
	Health       *TestHealthSummary     `json:"health,omitempty"`
	Diagnostics  []TestSignalDiagnostic `json:"diagnostics,omitempty"`
	PathMappings []TestPathMapping      `json:"path_mappings,omitempty"`
}

// TestSignalSummary aggregates scanner-side test discovery state.
type TestSignalSummary struct {
	Enabled             bool     `json:"enabled"`
	Modules             int      `json:"modules"`
	IgnoredModules      int      `json:"ignored_modules,omitempty"`
	ConfiguredModules   int      `json:"configured_modules,omitempty"`
	DiscoveredModules   int      `json:"discovered_modules,omitempty"`
	ReportCandidates    int      `json:"report_candidates,omitempty"`
	StaleReports        int      `json:"stale_reports,omitempty"`
	Tests               int      `json:"tests,omitempty"`
	TestFailures        int      `json:"test_failures,omitempty"`
	TestErrors          int      `json:"test_errors,omitempty"`
	TestSkipped         int      `json:"test_skipped,omitempty"`
	TestDurationMs      int64    `json:"test_duration_ms,omitempty"`
	ModulesWithCoverage int      `json:"modules_with_coverage,omitempty"`
	LinesToCover        int      `json:"lines_to_cover,omitempty"`
	CoveredLines        int      `json:"covered_lines,omitempty"`
	Coverage            *float64 `json:"coverage,omitempty"`
	NewLinesToCover     int      `json:"new_lines_to_cover,omitempty"`
	NewCoveredLines     int      `json:"new_covered_lines,omitempty"`
	NewCodeCoverage     *float64 `json:"new_code_coverage,omitempty"`
	MutantsTotal        int      `json:"mutants_total,omitempty"`
	MutantsKilled       int      `json:"mutants_killed,omitempty"`
	MutantsSurvived     int      `json:"mutants_survived,omitempty"`
	MutantsTimeout      int      `json:"mutants_timeout,omitempty"`
	MutantsError        int      `json:"mutants_error,omitempty"`
	MutationScore       *float64 `json:"mutation_score,omitempty"`
}

// TestModuleSignal describes one discovered or configured test module.
type TestModuleSignal struct {
	Name                 string                 `json:"name"`
	Root                 string                 `json:"root"`
	Language             string                 `json:"language,omitempty"`
	ArchitectureRole     string                 `json:"architecture_role,omitempty"`
	Source               string                 `json:"source"`
	TestPolicy           string                 `json:"test_policy,omitempty"`
	IgnoreReason         string                 `json:"ignore_reason,omitempty"`
	Command              string                 `json:"command,omitempty"`
	ArtifactRoot         string                 `json:"artifact_root,omitempty"`
	ReportRoot           string                 `json:"report_root,omitempty"`
	Reports              []TestReportProvenance `json:"reports,omitempty"`
	Coverage             *TestCoverageSummary   `json:"coverage,omitempty"`
	Files                []TestFileCoverage     `json:"files,omitempty"`
	Suites               []TestSuiteSignal      `json:"suites,omitempty"`
	Mutation             *TestMutationSummary   `json:"mutation,omitempty"`
	Execution            *TestExecutionStatus   `json:"execution,omitempty"`
	Health               *TestModuleHealth      `json:"health,omitempty"`
	CoverageThreshold    *float64               `json:"coverage_threshold,omitempty"`
	NewCoverageThreshold *float64               `json:"new_coverage_threshold,omitempty"`
	MutationThreshold    *float64               `json:"mutation_threshold,omitempty"`
	Owner                string                 `json:"owner,omitempty"`
	Team                 string                 `json:"team,omitempty"`
	IntegrationRequired  bool                   `json:"integration_required,omitempty"`
}

// TestHealthSummary is the project-level architecture-aware test-health result.
type TestHealthSummary struct {
	Status          string   `json:"status"`
	Score           int      `json:"score"`
	Modules         int      `json:"modules"`
	ModulesAtRisk   int      `json:"modules_at_risk,omitempty"`
	PartialModules  int      `json:"partial_modules,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// TestModuleHealth describes module-level test-health evaluation.
type TestModuleHealth struct {
	Status          string   `json:"status"`
	Score           int      `json:"score"`
	Confidence      string   `json:"confidence"`
	Partial         bool     `json:"partial,omitempty"`
	Reasons         []string `json:"reasons,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// TestCoverageSummary is a normalized coverage aggregate for one module.
type TestCoverageSummary struct {
	LinesToCover      int      `json:"lines_to_cover,omitempty"`
	CoveredLines      int      `json:"covered_lines,omitempty"`
	UncoveredLines    int      `json:"uncovered_lines,omitempty"`
	Coverage          *float64 `json:"coverage,omitempty"`
	NewLinesToCover   int      `json:"new_lines_to_cover,omitempty"`
	NewCoveredLines   int      `json:"new_covered_lines,omitempty"`
	NewCodeCoverage   *float64 `json:"new_code_coverage,omitempty"`
	Partial           bool     `json:"partial,omitempty"`
	UnavailableReason string   `json:"unavailable_reason,omitempty"`
}

// TestFileCoverage is normalized file-level coverage for project files.
type TestFileCoverage struct {
	Path               string `json:"path"`
	LinesToCover       int    `json:"lines_to_cover,omitempty"`
	CoveredLines       int    `json:"covered_lines,omitempty"`
	CoveredLineNumbers []int  `json:"covered_line_numbers,omitempty"`
	UncoveredLines     []int  `json:"uncovered_lines,omitempty"`
	BranchConditions   int    `json:"branch_conditions,omitempty"`
	CoveredBranches    int    `json:"covered_branches,omitempty"`
}

// TestSuiteSignal preserves suite-level unit-test results.
type TestSuiteSignal struct {
	ID         string           `json:"id,omitempty"`
	Name       string           `json:"name,omitempty"`
	Kind       string           `json:"kind,omitempty"`
	Tests      int              `json:"tests,omitempty"`
	Passed     int              `json:"passed,omitempty"`
	Failures   int              `json:"failures,omitempty"`
	Errors     int              `json:"errors,omitempty"`
	Skipped    int              `json:"skipped,omitempty"`
	DurationMs int64            `json:"duration_ms,omitempty"`
	Cases      []TestCaseSignal `json:"cases,omitempty"`
}

// TestCaseSignal is a normalized test-case result.
type TestCaseSignal struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	ClassName  string `json:"class_name,omitempty"`
	File       string `json:"file,omitempty"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Message    string `json:"message,omitempty"`
}

// TestMutationSummary is a normalized optional mutation-test aggregate.
type TestMutationSummary struct {
	Score    *float64 `json:"score,omitempty"`
	Total    int      `json:"total,omitempty"`
	Killed   int      `json:"killed,omitempty"`
	Survived int      `json:"survived,omitempty"`
	Timeout  int      `json:"timeout,omitempty"`
	Errors   int      `json:"errors,omitempty"`
}

// TestExecutionStatus records opt-in command execution state when commands run.
type TestExecutionStatus struct {
	Mode       string `json:"mode"`
	Command    string `json:"command,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
}

// TestReportProvenance records an existing report candidate and its freshness.
type TestReportProvenance struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	SourceMode string `json:"source_mode"`
	Freshness  string `json:"freshness"`
	AgeMs      int64  `json:"age_ms,omitempty"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
}

// TestSignalDiagnostic explains discovery, overrides, ignored modules, and report collection decisions.
type TestSignalDiagnostic struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Module  string `json:"module,omitempty"`
	Path    string `json:"path,omitempty"`
}

// CollectTestSignals discovers configured and automatic test modules without executing commands.
func CollectTestSignals(projectDir string, opts TestOptions, scanStarted time.Time) (*TestSignalReport, error) {
	if !opts.Enabled {
		return nil, nil
	}
	applyTestDefaults(&opts)

	report := &TestSignalReport{
		Summary:      TestSignalSummary{Enabled: true},
		PathMappings: append([]TestPathMapping(nil), opts.PathMappings...),
	}
	modulesByRoot := map[string]int{}
	addConfiguredTestModules(projectDir, opts, scanStarted, report, modulesByRoot)
	if opts.Discover {
		if err := addDiscoveredTestModules(projectDir, opts, scanStarted, report, modulesByRoot); err != nil {
			return nil, err
		}
	}
	summarizeTestSignalReport(report)
	sort.Slice(report.Modules, func(i, j int) bool { return report.Modules[i].Root < report.Modules[j].Root })
	evaluateTestHealth(report)
	return report, nil
}

func addConfiguredTestModules(projectDir string, opts TestOptions, scanStarted time.Time, report *TestSignalReport, modulesByRoot map[string]int) {
	for _, moduleConfig := range opts.Modules {
		module := moduleFromConfig(projectDir, moduleConfig)
		module.Source = TestSourceConfigured
		if module.TestPolicy == "" {
			module.TestPolicy = TestPolicyRequired
		}
		if module.TestPolicy == TestPolicyIgnored {
			report.Diagnostics = append(report.Diagnostics, TestSignalDiagnostic{Level: "info", Code: "module_ignored", Message: "configured module ignored for test health", Module: module.Name, Path: module.Root})
		}
		if opts.Mode == TestModeDoctor {
			appendDoctorDiagnostics(module, defaultReportPaths(module), &report.Diagnostics)
		}
		if module.Command != "" && (!opts.Run || opts.Mode == TestModeDoctor) {
			report.Diagnostics = append(report.Diagnostics, TestSignalDiagnostic{Level: "info", Code: "command_not_executed", Message: "configured test command was not executed because test execution is not enabled", Module: module.Name, Path: module.Root})
		}
		if opts.Run && opts.Mode != TestModeDoctor && module.Command != "" {
			module.Execution = executeTestCommand(projectDir, module, &report.Diagnostics)
		}
		module.Reports = collectConfiguredReports(projectDir, module, moduleConfig, opts, scanStarted, &report.Diagnostics)
		normalizeModuleSignals(projectDir, &module, opts, &report.Diagnostics)
		addModule(report, modulesByRoot, module)
	}
}

func addDiscoveredTestModules(projectDir string, opts TestOptions, scanStarted time.Time, report *TestSignalReport, modulesByRoot map[string]int) error {
	discovered, diagnostics, err := DiscoverTestModules(projectDir, opts)
	if err != nil {
		return err
	}
	report.Diagnostics = append(report.Diagnostics, diagnostics...)
	for _, module := range discovered {
		if existingIndex, exists := modulesByRoot[module.Root]; exists {
			report.Diagnostics = append(report.Diagnostics, TestSignalDiagnostic{Level: "info", Code: "module_duplicate", Message: "discovered module skipped because configuration already defines the root", Module: report.Modules[existingIndex].Name, Path: module.Root})
			continue
		}
		collectDiscoveredTestModule(projectDir, opts, scanStarted, report, modulesByRoot, module)
	}
	return nil
}

func collectDiscoveredTestModule(projectDir string, opts TestOptions, scanStarted time.Time, report *TestSignalReport, modulesByRoot map[string]int, module TestModuleSignal) {
	if opts.Mode == TestModeDoctor {
		appendDoctorDiagnostics(module, defaultReportPaths(module), &report.Diagnostics)
	}
	if opts.Run && opts.Mode != TestModeDoctor && module.Command != "" {
		module.Execution = executeTestCommand(projectDir, module, &report.Diagnostics)
	}
	module.Reports = collectDefaultReports(projectDir, module, opts, scanStarted, &report.Diagnostics)
	normalizeModuleSignals(projectDir, &module, opts, &report.Diagnostics)
	addModule(report, modulesByRoot, module)
}

func summarizeTestSignalReport(report *TestSignalReport) {
	for _, module := range report.Modules {
		if module.Source == TestSourceConfigured {
			report.Summary.ConfiguredModules++
		} else {
			report.Summary.DiscoveredModules++
		}
		if module.TestPolicy == TestPolicyIgnored {
			report.Summary.IgnoredModules++
		}
		report.Summary.ReportCandidates += len(module.Reports)
		for _, candidate := range module.Reports {
			if candidate.Freshness == "stale" {
				report.Summary.StaleReports++
			}
		}
	}
	report.Summary.Modules = len(report.Modules)
}

// DiscoverTestModules finds modules from workspace manifests and language marker files.
func DiscoverTestModules(projectDir string, opts TestOptions) ([]TestModuleSignal, []TestSignalDiagnostic, error) {
	applyTestDefaults(&opts)
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve project dir: %w", err)
	}

	modules := map[string]TestModuleSignal{}
	var diagnostics []TestSignalDiagnostic
	addGoWorkspaceModules(absProjectDir, modules, &diagnostics)
	addPackageWorkspaceModules(absProjectDir, modules, &diagnostics)
	if err := addMarkerModules(absProjectDir, opts, modules, &diagnostics); err != nil {
		return nil, nil, err
	}
	out := sortedDiscoveredModules(modules, &diagnostics)
	return out, diagnostics, nil
}

func addGoWorkspaceModules(absProjectDir string, modules map[string]TestModuleSignal, diagnostics *[]TestSignalDiagnostic) {
	for _, root := range discoverGoWorkModules(absProjectDir, diagnostics) {
		module := moduleFromRoot(absProjectDir, root, "go", TestSourceDiscovered)
		module.ArchitectureRole = inferArchitectureRole(module.Root)
		modules[module.Root] = module
	}
}

func addPackageWorkspaceModules(absProjectDir string, modules map[string]TestModuleSignal, diagnostics *[]TestSignalDiagnostic) {
	for _, root := range discoverPackageWorkspaceModules(absProjectDir, diagnostics) {
		if _, exists := modules[root]; exists {
			continue
		}
		language := packageJSONLanguage(filepath.Join(absProjectDir, filepath.FromSlash(root), packageJSONFile))
		module := moduleFromRoot(absProjectDir, root, language, TestSourceDiscovered)
		module.ArchitectureRole = inferArchitectureRole(module.Root)
		modules[module.Root] = module
	}
}

func addMarkerModules(absProjectDir string, opts TestOptions, modules map[string]TestModuleSignal, diagnostics *[]TestSignalDiagnostic) error {
	return filepath.WalkDir(absProjectDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := cleanRel(absProjectDir, path)
		if entry.IsDir() {
			return handleMarkerDir(rel, entry.Name(), opts, diagnostics)
		}
		handleMarkerFile(absProjectDir, path, modules)
		return nil
	})
}

func handleMarkerDir(rel, name string, opts TestOptions, diagnostics *[]TestSignalDiagnostic) error {
	if shouldSkipMarkerDir(rel, name, opts, diagnostics) || shouldStopAtDepth(rel, opts.MaxDepth) {
		return filepath.SkipDir
	}
	return nil
}

func handleMarkerFile(absProjectDir, path string, modules map[string]TestModuleSignal) {
	language, ok := markerLanguage(path)
	if !ok {
		return
	}
	root := cleanRel(absProjectDir, filepath.Dir(path))
	if existing, exists := modules[root]; exists {
		if existing.Language == "" {
			existing.Language = language
			modules[root] = existing
		}
		return
	}
	module := moduleFromRoot(absProjectDir, root, language, TestSourceDiscovered)
	module.ArchitectureRole = inferArchitectureRole(module.Root)
	modules[root] = module
}

func shouldSkipMarkerDir(rel, name string, opts TestOptions, diagnostics *[]TestSignalDiagnostic) bool {
	if rel == "." || !shouldSkipTestDir(rel, name, opts) {
		return false
	}
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "module_path_ignored", Message: "directory excluded from test module discovery", Path: rel})
	return true
}

func shouldStopAtDepth(rel string, maxDepth int) bool {
	return rel != "." && depth(rel) > maxDepth
}

func sortedDiscoveredModules(modules map[string]TestModuleSignal, diagnostics *[]TestSignalDiagnostic) []TestModuleSignal {
	out := make([]TestModuleSignal, 0, len(modules))
	for _, module := range modules {
		out = append(out, module)
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "module_discovered", Message: "test module discovered", Module: module.Name, Path: module.Root})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Root < out[j].Root })
	return out
}

func applyTestDefaults(opts *TestOptions) {
	if opts.Mode == "" {
		opts.Mode = TestModeCollect
	}
	if opts.Mode == TestModeRun {
		opts.Run = true
	}
	if opts.MaxReportAge == 0 {
		opts.MaxReportAge = 24 * time.Hour
	}
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 8
	}
	if opts.MaxCandidates <= 0 {
		opts.MaxCandidates = 200
	}
	if opts.MaxReportBytes <= 0 {
		opts.MaxReportBytes = 20 * 1024 * 1024
	}
	if opts.CommandPolicy == "" {
		opts.CommandPolicy = "explicit"
	}
}

func moduleFromConfig(projectDir string, cfg TestModuleConfig) TestModuleSignal {
	root := cfg.Root
	if root == "" {
		root = "."
	}
	root = cleanConfiguredRoot(projectDir, root)
	name := cfg.Name
	if name == "" {
		name = moduleName(root)
	}
	return TestModuleSignal{
		Name:                 name,
		Root:                 root,
		Language:             cfg.Language,
		ArchitectureRole:     firstNonEmpty(cfg.ArchitectureRole, inferArchitectureRole(root)),
		TestPolicy:           cfg.TestPolicy,
		IgnoreReason:         cfg.IgnoreReason,
		Command:              cfg.Command,
		ArtifactRoot:         cfg.ArtifactRoot,
		ReportRoot:           cfg.ReportRoot,
		CoverageThreshold:    cfg.CoverageThreshold,
		NewCoverageThreshold: cfg.NewCoverageThreshold,
		MutationThreshold:    cfg.MutationThreshold,
		Owner:                cfg.Owner,
		Team:                 cfg.Team,
		IntegrationRequired:  cfg.IntegrationRequired,
	}
}

func moduleFromRoot(projectDir, root, language, source string) TestModuleSignal {
	root = cleanConfiguredRoot(projectDir, root)
	return TestModuleSignal{
		Name:       moduleName(root),
		Root:       root,
		Language:   language,
		Source:     source,
		TestPolicy: TestPolicyRequired,
		Command:    candidateTestCommand(language),
	}
}

func addModule(report *TestSignalReport, modulesByRoot map[string]int, module TestModuleSignal) {
	modulesByRoot[module.Root] = len(report.Modules)
	report.Modules = append(report.Modules, module)
}

func discoverGoWorkModules(projectDir string, diagnostics *[]TestSignalDiagnostic) []string {
	path := filepath.Join(projectDir, "go.work")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var roots []string
	inUseBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		moduleLine, nextInUseBlock, ok := normalizeGoWorkUseLine(line, inUseBlock)
		inUseBlock = nextInUseBlock
		if !ok {
			continue
		}
		for _, field := range strings.Fields(moduleLine) {
			field = strings.Trim(field, `"`)
			if field == "use" || strings.HasPrefix(field, "//") {
				break
			}
			root := cleanConfiguredRoot(projectDir, field)
			if root != "" {
				roots = append(roots, root)
				*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "go_work_module", Message: "Go workspace module discovered", Path: root})
			}
		}
	}
	return roots
}

func normalizeGoWorkUseLine(line string, inUseBlock bool) (string, bool, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "//") {
		return "", inUseBlock, false
	}
	if strings.HasPrefix(line, "use (") {
		return cleanGoWorkUseLine(strings.TrimPrefix(line, "use")), true, true
	}
	if line == ")" && inUseBlock {
		return "", false, false
	}
	if strings.HasPrefix(line, "use ") {
		return cleanGoWorkUseLine(strings.TrimPrefix(line, "use")), inUseBlock, true
	}
	if !inUseBlock {
		return "", inUseBlock, false
	}
	return cleanGoWorkUseLine(line), inUseBlock, true
}

func cleanGoWorkUseLine(line string) string {
	return strings.TrimSpace(strings.Trim(line, "()"))
}

func markerLanguage(path string) (string, bool) {
	name := filepath.Base(path)
	switch name {
	case "go.mod":
		return "go", true
	case "pom.xml", "build.gradle", "build.gradle.kts":
		return "java", true
	case "pyproject.toml", "pytest.ini", "tox.ini", "setup.py":
		return "python", true
	case "Cargo.toml":
		return "rust", true
	case packageJSONFile:
		return packageJSONLanguage(path), true
	case "nx.json", "lerna.json", "turbo.json", "pnpm-workspace.yaml":
		return "javascript", true
	default:
		return "", false
	}
}

func packageJSONLanguage(path string) string {
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "tsconfig.json")); err == nil {
		return "typescript"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "javascript"
	}
	var pkg struct {
		Dependencies    map[string]any `json:"dependencies"`
		DevDependencies map[string]any `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "javascript"
	}
	if _, ok := pkg.Dependencies["typescript"]; ok {
		return "typescript"
	}
	if _, ok := pkg.DevDependencies["typescript"]; ok {
		return "typescript"
	}
	return "javascript"
}

func discoverPackageWorkspaceModules(projectDir string, diagnostics *[]TestSignalDiagnostic) []string {
	patterns := packageWorkspacePatterns(filepath.Join(projectDir, packageJSONFile))
	patterns = append(patterns, pnpmWorkspacePatterns(filepath.Join(projectDir, "pnpm-workspace.yaml"))...)
	if len(patterns) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var roots []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(projectDir, filepath.FromSlash(pattern), packageJSONFile))
		for _, match := range matches {
			root := cleanRel(projectDir, filepath.Dir(match))
			if root == "." || seen[root] {
				continue
			}
			seen[root] = true
			roots = append(roots, root)
			*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "workspace_module", Message: "package workspace module discovered", Path: root})
		}
	}
	sort.Strings(roots)
	return roots
}

func packageWorkspacePatterns(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg struct {
		Workspaces any `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	switch workspaces := pkg.Workspaces.(type) {
	case []any:
		return stringList(workspaces)
	case map[string]any:
		if packages, ok := workspaces["packages"].([]any); ok {
			return stringList(packages)
		}
	}
	return nil
}

func pnpmWorkspacePatterns(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "-")
		line = strings.Trim(strings.TrimSpace(line), `"'`)
		if line == "" || strings.HasPrefix(line, "#") || line == "packages:" {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func stringList(items []any) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := item.(string); ok && value != "" {
			out = append(out, value)
		}
	}
	return out
}

func shouldSkipTestDir(rel, name string, opts TestOptions) bool {
	if defaultTestExcludedDirs[name] {
		return true
	}
	return matchesAny(filepath.ToSlash(rel), opts.Exclusions)
}

func inferArchitectureRole(root string) string {
	parts := strings.FieldsFunc(strings.ToLower(filepath.ToSlash(root)), func(r rune) bool {
		return r == '/' || r == '-' || r == '_'
	})
	for _, part := range parts {
		switch part {
		case "domain":
			return "domain"
		case "application", "app":
			return "application"
		case "adapter", "adapters":
			return "adapter"
		case "infrastructure", "infra":
			return "infrastructure"
		case "api", "web":
			return "web"
		case "cmd", "service", "services":
			return "service"
		case "apps":
			return "application"
		case "lib", "libs", "packages", "pkg":
			return "library"
		}
	}
	return "unknown"
}

func collectConfiguredReports(projectDir string, module TestModuleSignal, cfg TestModuleConfig, opts TestOptions, scanStarted time.Time, diagnostics *[]TestSignalDiagnostic) []TestReportProvenance {
	collector := testReportCollector{projectDir: projectDir, opts: opts, scanStarted: scanStarted, diagnostics: diagnostics}
	var reports []TestReportProvenance
	reports = append(reports, collector.collectReportList(module, "coverage", cfg.CoverageReports, "configured")...)
	reports = append(reports, collector.collectReportList(module, "test", cfg.TestReports, "configured")...)
	reports = append(reports, collector.collectReportList(module, "mutation", cfg.MutationReports, "configured")...)
	reports = append(reports, collector.collectReportList(module, "native", cfg.NativeReports, "configured")...)
	return reports
}

func collectDefaultReports(projectDir string, module TestModuleSignal, opts TestOptions, scanStarted time.Time, diagnostics *[]TestSignalDiagnostic) []TestReportProvenance {
	paths := defaultReportPaths(module)
	collector := testReportCollector{projectDir: projectDir, opts: opts, scanStarted: scanStarted, diagnostics: diagnostics}
	reports := collector.collectReportList(module, "candidate", paths, "discovered")
	if len(reports) == 0 {
		reports = boundedFallbackReports(projectDir, module, opts, scanStarted, diagnostics)
	}
	return reports
}

func defaultReportPaths(module TestModuleSignal) []string {
	switch module.Language {
	case "go":
		return []string{"coverage.out", "cover.out", "test-results.xml", junitReportFile, nativeTestReportFile}
	case "javascript", "typescript":
		return []string{"coverage/lcov.info", "coverage/cobertura-coverage.xml", junitReportFile, "test-results/" + junitReportFile, nativeTestReportFile}
	case "java", "kotlin":
		return []string{"target/site/jacoco/jacoco.xml", "build/reports/jacoco/test/jacocoTestReport.xml", "build/test-results/test/TESTS-TestSuites.xml"}
	case "python":
		return []string{"coverage.xml", "coverage.json", junitReportFile, "test-results/" + junitReportFile}
	case "rust":
		return []string{"cobertura.xml", junitReportFile, "target/tarpaulin/cobertura.xml"}
	default:
		return []string{"coverage.xml", "lcov.info", junitReportFile, nativeTestReportFile}
	}
}

func appendDoctorDiagnostics(module TestModuleSignal, reportCandidates []string, diagnostics *[]TestSignalDiagnostic) {
	if module.Command != "" {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "doctor_command_candidate", Message: "candidate test command discovered", Module: module.Name, Path: module.Command})
	}
	for _, candidate := range reportCandidates {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "doctor_report_candidate", Message: "candidate report path", Module: module.Name, Path: candidate})
	}
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "doctor_config_suggestion", Message: "suggested [[tests.modules]] entry can pin root, role, command, and report paths", Module: module.Name, Path: module.Root})
}

func candidateTestCommand(language string) string {
	switch language {
	case "go":
		return "go test ./..."
	case "javascript", "typescript":
		return "npm test"
	case "java", "kotlin":
		return "mvn test"
	case "python":
		return "pytest"
	case "rust":
		return "cargo test"
	default:
		return ""
	}
}

type testReportCollector struct {
	projectDir  string
	opts        TestOptions
	scanStarted time.Time
	diagnostics *[]TestSignalDiagnostic
}

func (collector testReportCollector) collectReportList(module TestModuleSignal, kind string, paths []string, sourceMode string) []TestReportProvenance {
	reports := make([]TestReportProvenance, 0, len(paths))
	for _, configuredPath := range paths {
		if len(reports) >= collector.opts.MaxCandidates {
			break
		}
		fullPath := resolveModulePath(collector.projectDir, module, configuredPath)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Size() > collector.opts.MaxReportBytes {
			*collector.diagnostics = append(*collector.diagnostics, TestSignalDiagnostic{Level: "warn", Code: "report_too_large", Message: "test report candidate exceeds configured size limit", Module: module.Name, Path: cleanRel(collector.projectDir, fullPath)})
			continue
		}
		age := collector.scanStarted.Sub(info.ModTime())
		freshness := "fresh"
		if age > collector.opts.MaxReportAge {
			freshness = "stale"
			*collector.diagnostics = append(*collector.diagnostics, TestSignalDiagnostic{Level: "warn", Code: "report_stale", Message: "test report candidate is older than max_report_age", Module: module.Name, Path: cleanRel(collector.projectDir, fullPath)})
		}
		reports = append(reports, TestReportProvenance{Kind: kind, Path: cleanRel(collector.projectDir, fullPath), SourceMode: sourceMode, Freshness: freshness, AgeMs: age.Milliseconds(), SizeBytes: info.Size()})
	}
	return reports
}

func resolveModulePath(projectDir string, module TestModuleSignal, reportPath string) string {
	if filepath.IsAbs(reportPath) {
		return reportPath
	}
	root := module.ReportRoot
	if root == "" {
		root = module.ArtifactRoot
	}
	if root == "" {
		root = module.Root
	}
	if root == "." {
		return filepath.Join(projectDir, reportPath)
	}
	return filepath.Join(projectDir, filepath.FromSlash(root), reportPath)
}

func cleanConfiguredRoot(projectDir, root string) string {
	if root == "" || root == "." {
		return "."
	}
	if filepath.IsAbs(root) {
		if rel, err := filepath.Rel(projectDir, root); err == nil {
			return filepath.ToSlash(filepath.Clean(rel))
		}
	}
	return filepath.ToSlash(filepath.Clean(root))
}

func cleanRel(projectDir, path string) string {
	rel, err := filepath.Rel(projectDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if rel == "." {
		return "."
	}
	return filepath.ToSlash(filepath.Clean(rel))
}

func moduleName(root string) string {
	if root == "." || root == "" {
		return "root"
	}
	return filepath.Base(filepath.FromSlash(root))
}

func depth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/") + 1
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
