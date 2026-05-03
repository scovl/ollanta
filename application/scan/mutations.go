package scan

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	mutationToolNative    = "native"
	mutationToolStryker   = "stryker"
	mutationToolPIT       = "pit"
	mutationToolMutmut    = "mutmut"
	mutationToolCosmic    = "cosmic-ray"
	mutationToolInfection = "infection"

	mutationReportInfection = "infection-log.json"
	mutationReportMutmut    = "mutmut-report.json"
	mutationReportCosmic    = "cosmic-ray.json"
)

// CollectMutationSignals discovers mutation modules and collects existing mutation reports.
func CollectMutationSignals(projectDir string, opts MutationOptions, scanStarted time.Time) (*TestSignalReport, error) {
	if !opts.Enabled {
		return nil, nil
	}
	applyMutationDefaults(&opts)
	if err := ValidateMutationOptions(opts); err != nil {
		return nil, err
	}

	report := &TestSignalReport{
		Summary:      TestSignalSummary{Enabled: true},
		PathMappings: append([]TestPathMapping(nil), opts.PathMappings...),
	}
	modulesByRoot := map[string]int{}
	if err := addConfiguredMutationModules(projectDir, opts, scanStarted, report, modulesByRoot); err != nil {
		return nil, err
	}
	if opts.Discover {
		if err := addDiscoveredMutationModules(projectDir, opts, scanStarted, report, modulesByRoot); err != nil {
			return nil, err
		}
	}
	summarizeTestSignalReport(report)
	sort.Slice(report.Modules, func(i, j int) bool { return report.Modules[i].Root < report.Modules[j].Root })
	evaluateTestHealth(report)
	return report, nil
}

func addConfiguredMutationModules(projectDir string, opts MutationOptions, scanStarted time.Time, report *TestSignalReport, modulesByRoot map[string]int) error {
	for _, moduleConfig := range opts.Modules {
		effective := effectiveMutationOptions(opts, moduleConfig)
		module := moduleFromMutationConfig(projectDir, moduleConfig)
		moduleContext := configuredMutationContext{projectDir: projectDir, opts: opts, effective: effective, scanStarted: scanStarted, report: report, modulesByRoot: modulesByRoot}
		if err := collectConfiguredMutationModule(moduleContext, moduleConfig, module); err != nil {
			return err
		}
	}
	return nil
}

type configuredMutationContext struct {
	projectDir    string
	opts          MutationOptions
	effective     MutationOptions
	scanStarted   time.Time
	report        *TestSignalReport
	modulesByRoot map[string]int
}

func collectConfiguredMutationModule(context configuredMutationContext, moduleConfig MutationModuleConfig, module TestModuleSignal) error {
	module.Source = TestSourceConfigured
	if module.Mutation == nil {
		module.Mutation = mutationSummaryFromConfig(context.effective, moduleConfig)
	}
	if isMutationModuleIgnored(moduleConfig) {
		context.report.Diagnostics = append(context.report.Diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_module_ignored", Message: "configured module ignored for mutation health", Module: module.Name, Path: module.Root})
		addModule(context.report, context.modulesByRoot, module)
		return nil
	}
	if context.opts.Mode == MutationModeDoctor {
		appendMutationDoctorDiagnostics(module, configuredMutationReportPaths(moduleConfig), &context.report.Diagnostics)
	}
	if module.Command != "" && (!context.opts.Run || context.opts.Mode == MutationModeDoctor) {
		context.report.Diagnostics = append(context.report.Diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_command_not_executed", Message: "configured mutation command was not executed because mutation execution is not enabled", Module: module.Name, Path: module.Root})
	}
	var executionErr error
	if context.opts.Run && context.opts.Mode != MutationModeDoctor && module.Command != "" {
		execution, err := executeMutationCommand(context.projectDir, module, context.effective, &context.report.Diagnostics)
		module.MutationExecution = execution
		executionErr = err
	}
	module.Reports = collectConfiguredMutationReports(context.projectDir, module, moduleConfig, context.effective, context.scanStarted, &context.report.Diagnostics)
	normalizeModuleMutationSignals(context.projectDir, &module, mutationTestOptions(context.effective, moduleConfig), &context.report.Diagnostics)
	addModule(context.report, context.modulesByRoot, module)
	return executionErr
}

func addDiscoveredMutationModules(projectDir string, opts MutationOptions, scanStarted time.Time, report *TestSignalReport, modulesByRoot map[string]int) error {
	discovered, diagnostics, err := DiscoverMutationModules(projectDir, opts)
	if err != nil {
		return err
	}
	report.Diagnostics = append(report.Diagnostics, diagnostics...)
	for _, module := range discovered {
		if existingIndex, exists := modulesByRoot[module.Root]; exists {
			report.Diagnostics = append(report.Diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_module_duplicate", Message: "discovered mutation module skipped because configuration already defines the root", Module: report.Modules[existingIndex].Name, Path: module.Root})
			continue
		}
		if opts.Mode == MutationModeDoctor {
			appendMutationDoctorDiagnostics(module, defaultMutationReportPaths(module), &report.Diagnostics)
		}
		if module.Command != "" && (!opts.Run || opts.Mode == MutationModeDoctor) {
			report.Diagnostics = append(report.Diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_command_not_executed", Message: "discovered mutation command was not executed because mutation execution is not enabled", Module: module.Name, Path: module.Root})
		}
		var executionErr error
		if opts.Run && opts.Mode != MutationModeDoctor && module.Command != "" {
			execution, err := executeMutationCommand(projectDir, module, opts, &report.Diagnostics)
			module.MutationExecution = execution
			executionErr = err
		}
		module.Reports = collectDefaultMutationReports(projectDir, module, opts, scanStarted, &report.Diagnostics)
		normalizeModuleMutationSignals(projectDir, &module, mutationTestOptions(opts, MutationModuleConfig{}), &report.Diagnostics)
		addModule(report, modulesByRoot, module)
		if executionErr != nil {
			return executionErr
		}
	}
	return nil
}

// DiscoverMutationModules finds mutation-capable modules without executing mutation tools.
func DiscoverMutationModules(projectDir string, opts MutationOptions) ([]TestModuleSignal, []TestSignalDiagnostic, error) {
	applyMutationDefaults(&opts)
	testOpts := TestOptions{Exclusions: opts.Exclusions, MaxDepth: opts.MaxDepth, MaxCandidates: opts.MaxCandidates, MaxReportBytes: opts.MaxReportBytes, PathMappings: opts.PathMappings}
	modules, diagnostics, err := DiscoverTestModules(projectDir, testOpts)
	if err != nil {
		return nil, diagnostics, err
	}
	out := make([]TestModuleSignal, 0, len(modules))
	for _, module := range modules {
		tool := detectMutationTool(projectDir, module)
		if tool.Name == "" {
			diagnostics = append(diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_tool_missing", Message: "no supported mutation tool or native mutation report detected", Module: module.Name, Path: module.Root})
			continue
		}
		module.Command = tool.Command
		module.Mutation = &TestMutationSummary{Tool: tool.Name, Status: "available", Confidence: tool.Confidence, ChangedOnly: opts.ChangedOnly, MaxRuntime: opts.MaxRuntime.String(), MaxMutants: opts.MaxMutants}
		diagnostics = append(diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_tool_detected", Message: "mutation tool detected", Module: module.Name, Path: tool.Name})
		out = append(out, module)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Root < out[j].Root })
	return out, diagnostics, nil
}

func applyMutationDefaults(opts *MutationOptions) {
	if opts.Mode == "" {
		opts.Mode = MutationModeCollect
	}
	if opts.Mode == MutationModeRun {
		opts.Run = true
	}
	if opts.MaxReportAge == 0 {
		opts.MaxReportAge = 24 * time.Hour
	}
	if opts.MaxRuntime == 0 {
		opts.MaxRuntime = 10 * time.Minute
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
		opts.CommandPolicy = CommandPolicyExplicit
	}
}

func effectiveMutationOptions(opts MutationOptions, cfg MutationModuleConfig) MutationOptions {
	applyMutationDefaults(&opts)
	if cfg.ChangedOnly != nil {
		opts.ChangedOnly = *cfg.ChangedOnly
	}
	if cfg.MaxRuntime > 0 {
		opts.MaxRuntime = cfg.MaxRuntime
	}
	if cfg.MaxMutants > 0 {
		opts.MaxMutants = cfg.MaxMutants
	}
	if len(cfg.Exclusions) > 0 {
		opts.Exclusions = append(append([]string(nil), opts.Exclusions...), cfg.Exclusions...)
	}
	if cfg.FailOnTimeout != nil {
		opts.FailOnTimeout = *cfg.FailOnTimeout
	}
	if cfg.AllowExternalArtifacts != nil {
		opts.AllowExternalArtifacts = *cfg.AllowExternalArtifacts
	}
	if len(cfg.PathMappings) > 0 {
		opts.PathMappings = append(append([]TestPathMapping(nil), opts.PathMappings...), cfg.PathMappings...)
	}
	return opts
}

func moduleFromMutationConfig(projectDir string, cfg MutationModuleConfig) TestModuleSignal {
	root := cleanConfiguredRoot(projectDir, cfg.Root)
	name := cfg.Name
	if name == "" {
		name = moduleName(root)
	}
	policy := cfg.MutationPolicy
	if policy == MutationPolicyIgnored || policy == MutationPolicyDisabled {
		policy = TestPolicyIgnored
	} else if policy == "" {
		policy = TestPolicyRequired
	}
	return TestModuleSignal{
		Name:                     name,
		Root:                     root,
		Language:                 cfg.Language,
		ArchitectureRole:         firstNonEmpty(cfg.ArchitectureRole, inferArchitectureRole(root)),
		Source:                   TestSourceConfigured,
		TestPolicy:               policy,
		IgnoreReason:             cfg.IgnoreReason,
		Command:                  cfg.Command,
		ArtifactRoot:             cfg.ArtifactRoot,
		ReportRoot:               cfg.ReportRoot,
		SuiteKind:                normalizedSuiteKind(cfg.SuiteKind),
		EvidenceConfidence:       cfg.EvidenceConfidence,
		AllowExternalArtifacts:   boolValue(cfg.AllowExternalArtifacts),
		MutationThreshold:        cfg.Threshold,
		ChangedMutationThreshold: cfg.ChangedCodeThreshold,
		Owner:                    cfg.Owner,
		Team:                     cfg.Team,
	}
}

func mutationSummaryFromConfig(opts MutationOptions, cfg MutationModuleConfig) *TestMutationSummary {
	changedOnly := opts.ChangedOnly
	if cfg.ChangedOnly != nil {
		changedOnly = *cfg.ChangedOnly
	}
	maxRuntime := opts.MaxRuntime
	if cfg.MaxRuntime > 0 {
		maxRuntime = cfg.MaxRuntime
	}
	maxMutants := opts.MaxMutants
	if cfg.MaxMutants > 0 {
		maxMutants = cfg.MaxMutants
	}
	changedOnlyEnforcement := ""
	if changedOnly {
		changedOnlyEnforcement = "advisory"
	}
	maxMutantsEnforcement := ""
	if maxMutants > 0 {
		maxMutantsEnforcement = "advisory"
	}
	return &TestMutationSummary{Tool: cfg.Tool, Status: "configured", Confidence: firstNonEmpty(cfg.EvidenceConfidence, EvidenceConfidenceMedium), SuiteKind: normalizedSuiteKind(cfg.SuiteKind), Availability: EvidenceAvailabilityUnavailable, ChangedCodeThreshold: cfg.ChangedCodeThreshold, ChangedOnly: changedOnly, ChangedOnlyEnforcement: changedOnlyEnforcement, MaxRuntime: maxRuntime.String(), MaxMutants: maxMutants, MaxMutantsEnforcement: maxMutantsEnforcement}
}

func isMutationModuleIgnored(cfg MutationModuleConfig) bool {
	return cfg.MutationPolicy == MutationPolicyIgnored || cfg.MutationPolicy == MutationPolicyDisabled
}

func configuredMutationReportPaths(cfg MutationModuleConfig) []string {
	paths := append([]string(nil), cfg.ReportPaths...)
	paths = append(paths, cfg.NativeReportPaths...)
	return paths
}

func collectConfiguredMutationReports(projectDir string, module TestModuleSignal, cfg MutationModuleConfig, opts MutationOptions, scanStarted time.Time, diagnostics *[]TestSignalDiagnostic) []TestReportProvenance {
	collector := mutationReportCollector{projectDir: projectDir, opts: opts, scanStarted: scanStarted, diagnostics: diagnostics}
	var reports []TestReportProvenance
	reports = append(reports, collector.collectReportList(module, "mutation", cfg.ReportPaths, "configured")...)
	reports = append(reports, collector.collectReportList(module, "native_mutation", cfg.NativeReportPaths, "configured")...)
	return reports
}

func collectDefaultMutationReports(projectDir string, module TestModuleSignal, opts MutationOptions, scanStarted time.Time, diagnostics *[]TestSignalDiagnostic) []TestReportProvenance {
	collector := mutationReportCollector{projectDir: projectDir, opts: opts, scanStarted: scanStarted, diagnostics: diagnostics}
	reports := collector.collectReportList(module, "mutation", defaultMutationReportPaths(module), "discovered")
	if len(reports) == 0 {
		reports = boundedFallbackMutationReports(projectDir, module, opts, scanStarted, diagnostics)
	}
	return reports
}

type mutationReportCollector struct {
	projectDir  string
	opts        MutationOptions
	scanStarted time.Time
	diagnostics *[]TestSignalDiagnostic
}

func (collector mutationReportCollector) collectReportList(module TestModuleSignal, kind string, paths []string, sourceMode string) []TestReportProvenance {
	reports := make([]TestReportProvenance, 0, len(paths))
	for _, configuredPath := range paths {
		if len(reports) >= collector.opts.MaxCandidates {
			break
		}
		fullPath, ok := resolveModulePath(collector.projectDir, module, configuredPath, collector.opts.AllowExternalArtifacts, collector.diagnostics)
		if !ok {
			continue
		}
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Size() > collector.opts.MaxReportBytes {
			*collector.diagnostics = append(*collector.diagnostics, TestSignalDiagnostic{Level: "warn", Code: "mutation_report_too_large", Message: "mutation report candidate exceeds configured size limit", Module: module.Name, Path: cleanRel(collector.projectDir, fullPath)})
			continue
		}
		age := collector.scanStarted.Sub(info.ModTime())
		freshness := "fresh"
		if age > collector.opts.MaxReportAge {
			freshness = "stale"
			*collector.diagnostics = append(*collector.diagnostics, TestSignalDiagnostic{Level: "warn", Code: "mutation_report_stale", Message: "mutation report candidate is older than max_report_age", Module: module.Name, Path: cleanRel(collector.projectDir, fullPath)})
		}
		if filepath.Base(fullPath) == nativeMutationFile {
			kind = "native_mutation"
		}
		reports = append(reports, TestReportProvenance{Kind: kind, Path: cleanRel(collector.projectDir, fullPath), SourceMode: sourceMode, Freshness: freshness, AgeMs: age.Milliseconds(), SizeBytes: info.Size()})
	}
	return reports
}

func boundedFallbackMutationReports(projectDir string, module TestModuleSignal, opts MutationOptions, scanStarted time.Time, diagnostics *[]TestSignalDiagnostic) []TestReportProvenance {
	candidates := collectFallbackMutationCandidates(projectDir, fallbackRoot(projectDir, module), module, opts, diagnostics)
	if len(candidates) > opts.MaxCandidates {
		candidates = candidates[:opts.MaxCandidates]
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "mutation_report_candidate_limit", Message: "bounded fallback mutation report search reached max_candidates", Module: module.Name, Path: module.Root})
	}
	collector := mutationReportCollector{projectDir: projectDir, opts: opts, scanStarted: scanStarted, diagnostics: diagnostics}
	reports := collector.collectReportList(module, "mutation", relCandidatePaths(projectDir, candidates), "fallback")
	if len(reports) > 0 {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_fallback_reports_found", Message: "bounded fallback mutation report search found candidates", Module: module.Name, Path: module.Root})
	}
	return reports
}

func collectFallbackMutationCandidates(projectDir, root string, module TestModuleSignal, opts MutationOptions, diagnostics *[]TestSignalDiagnostic) []string {
	var candidates []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "mutation_fallback_walk_error", Message: "bounded fallback mutation report search hit an inaccessible path", Module: module.Name, Path: cleanRel(projectDir, path)})
			return nil
		}
		relRoot := cleanRel(root, path)
		if entry.IsDir() {
			if relRoot != "." && (shouldSkipTestDir(cleanRel(projectDir, path), entry.Name(), TestOptions{Exclusions: opts.Exclusions}) || depth(relRoot) > opts.MaxDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		if isMutationReportName(entry.Name()) {
			if info, err := entry.Info(); err == nil && info.Size() <= opts.MaxReportBytes {
				candidates = append(candidates, path)
			}
		}
		return nil
	})
	sort.Strings(candidates)
	return candidates
}

func relCandidatePaths(projectDir string, candidates []string) []string {
	paths := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		paths = append(paths, cleanRel(projectDir, candidate))
	}
	return paths
}

func isMutationReportName(name string) bool {
	lower := strings.ToLower(name)
	return lower == nativeMutationFile || lower == "mutation.json" || lower == "mutations.json" || lower == "mutations.xml" || lower == "pitest.xml" || lower == mutationReportInfection || lower == mutationReportMutmut || lower == mutationReportCosmic
}

func defaultMutationReportPaths(module TestModuleSignal) []string {
	switch module.MutationTool() {
	case mutationToolStryker:
		return []string{"reports/mutation/mutation.json", "mutation/mutation.json", "coverage/mutation/mutation.json", nativeMutationFile}
	case mutationToolPIT:
		return []string{"target/pit-reports/mutations.xml", "build/reports/pitest/mutations.xml", nativeMutationFile}
	case mutationToolMutmut:
		return []string{mutationReportMutmut, "html/mutmut-report.json", nativeMutationFile}
	case mutationToolCosmic:
		return []string{mutationReportCosmic, ".cosmic-ray/results.json", nativeMutationFile}
	case mutationToolInfection:
		return []string{mutationReportInfection, "build/infection/infection-log.json", nativeMutationFile}
	default:
		switch module.Language {
		case "javascript", "typescript":
			return []string{"reports/mutation/mutation.json", "coverage/mutation/mutation.json", nativeMutationFile}
		case "java", "kotlin":
			return []string{"target/pit-reports/mutations.xml", nativeMutationFile}
		case "python":
			return []string{mutationReportMutmut, mutationReportCosmic, nativeMutationFile}
		case "php":
			return []string{mutationReportInfection, nativeMutationFile}
		default:
			return []string{nativeMutationFile, "mutation.json", "mutations.json"}
		}
	}
}

func (module TestModuleSignal) MutationTool() string {
	if module.Mutation != nil {
		return module.Mutation.Tool
	}
	return ""
}

type mutationToolDetection struct {
	Name       string
	Command    string
	Confidence string
}

func detectMutationTool(projectDir string, module TestModuleSignal) mutationToolDetection {
	root := filepath.Join(projectDir, filepath.FromSlash(module.Root))
	if module.Root == "." {
		root = projectDir
	}
	if fileExists(filepath.Join(root, nativeMutationFile)) {
		return mutationToolDetection{Name: mutationToolNative, Confidence: "high"}
	}
	if detectsStryker(root) {
		return mutationToolDetection{Name: mutationToolStryker, Command: "npx stryker run", Confidence: "high"}
	}
	if detectsPIT(root) {
		return mutationToolDetection{Name: mutationToolPIT, Command: pitCommand(root), Confidence: "high"}
	}
	if detectsMutmut(root) {
		return mutationToolDetection{Name: mutationToolMutmut, Command: "mutmut run", Confidence: "medium"}
	}
	if fileExists(filepath.Join(root, ".cosmic-ray.toml")) {
		return mutationToolDetection{Name: mutationToolCosmic, Command: "cosmic-ray exec .cosmic-ray.toml", Confidence: "medium"}
	}
	if detectsInfection(root) {
		return mutationToolDetection{Name: mutationToolInfection, Command: "vendor/bin/infection", Confidence: "medium"}
	}
	return mutationToolDetection{}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func detectsStryker(root string) bool {
	patterns := []string{"stryker.conf.*", "stryker.config.*", ".stryker.*"}
	for _, pattern := range patterns {
		if matches, _ := filepath.Glob(filepath.Join(root, pattern)); len(matches) > 0 {
			return true
		}
	}
	return fileContainsAny(filepath.Join(root, packageJSONFile), "@stryker-mutator", "stryker")
}

func detectsPIT(root string) bool {
	return fileContainsAny(filepath.Join(root, "pom.xml"), "pitest", "pitest-maven") || fileContainsAny(filepath.Join(root, "build.gradle"), "pitest") || fileContainsAny(filepath.Join(root, "build.gradle.kts"), "pitest")
}

func pitCommand(root string) string {
	if fileExists(filepath.Join(root, "pom.xml")) {
		return "mvn test-compile org.pitest:pitest-maven:mutationCoverage"
	}
	return "./gradlew pitest"
}

func detectsMutmut(root string) bool {
	return fileContainsAny(filepath.Join(root, "pyproject.toml"), "mutmut") || fileContainsAny(filepath.Join(root, "setup.cfg"), "mutmut")
}

func detectsInfection(root string) bool {
	return fileExists(filepath.Join(root, "infection.json")) || fileContainsAny(filepath.Join(root, "composer.json"), "infection/infection")
}

func fileContainsAny(path string, needles ...string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func appendMutationDoctorDiagnostics(module TestModuleSignal, reportCandidates []string, diagnostics *[]TestSignalDiagnostic) {
	if module.Mutation != nil && module.Mutation.Tool != "" {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_doctor_tool", Message: "mutation tool available without executing commands", Module: module.Name, Path: module.Mutation.Tool})
	}
	if module.Command != "" {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_doctor_command_candidate", Message: "candidate mutation command discovered", Module: module.Name, Path: module.Command})
	}
	for _, candidate := range reportCandidates {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_doctor_report_candidate", Message: "candidate mutation report path", Module: module.Name, Path: candidate})
	}
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_doctor_collect_only", Message: "prefer report-only CI collection before enabling opt-in mutation execution", Module: module.Name, Path: module.Root})
}

func executeMutationCommand(projectDir string, module TestModuleSignal, opts MutationOptions, diagnostics *[]TestSignalDiagnostic) (*TestExecutionStatus, error) {
	if !commandAllowed(opts.CommandPolicy, module.Source) {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "mutation_command_policy_denied", Message: "mutation command was not executed because command_policy denied this command source", Module: module.Name, Path: module.Root})
		return &TestExecutionStatus{Mode: MutationModeRun, Command: module.Command, CommandPolicy: opts.CommandPolicy, Shell: commandShell(), Partial: true}, nil
	}
	start := time.Now()
	workingDir := filepath.Join(projectDir, filepath.FromSlash(module.Root))
	if module.Root == "." {
		workingDir = projectDir
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.MaxRuntime)
	defer cancel()
	cmd := exec.CommandContext(ctx, commandShell(), commandShellArg(), module.Command)
	cmd.Dir = workingDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	stdoutValue, stdoutTruncated := limitOutput(stdout.String())
	stderrValue, stderrTruncated := limitOutput(stderr.String())
	status := &TestExecutionStatus{Mode: MutationModeRun, Command: module.Command, CommandPolicy: opts.CommandPolicy, Shell: commandShell(), WorkingDir: cleanRel(projectDir, workingDir), MaxRuntime: opts.MaxRuntime.String(), DurationMs: time.Since(start).Milliseconds(), Stdout: stdoutValue, Stderr: stderrValue, StdoutTruncated: stdoutTruncated, StderrTruncated: stderrTruncated}
	appendOutputTruncationDiagnostics(module, status, diagnostics, "mutation_command_output_truncated")
	if ctx.Err() == context.DeadlineExceeded {
		status.ExitCode = 124
		status.Timeout = true
		status.Partial = true
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "mutation_command_timeout", Message: "mutation command timed out; readable reports will still be collected when present", Module: module.Name, Path: module.Root})
		if opts.FailOnTimeout {
			return status, fmt.Errorf("mutation command timed out for module %s", module.Name)
		}
		return status, nil
	}
	if err != nil {
		status.ExitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			status.ExitCode = exitErr.ExitCode()
		}
		status.Partial = true
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "mutation_command_failed", Message: fmt.Sprintf("mutation command exited with status %d; readable reports will still be collected when present", status.ExitCode), Module: module.Name, Path: module.Root})
		return status, nil
	}
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_command_executed", Message: "configured mutation command executed", Module: module.Name, Path: module.Root})
	return status, nil
}

func mutationTestOptions(opts MutationOptions, module MutationModuleConfig) TestOptions {
	pathMappings := append([]TestPathMapping(nil), opts.PathMappings...)
	pathMappings = append(pathMappings, module.PathMappings...)
	return TestOptions{Exclusions: opts.Exclusions, MaxReportAge: opts.MaxReportAge, MaxDepth: opts.MaxDepth, MaxCandidates: opts.MaxCandidates, MaxReportBytes: opts.MaxReportBytes, PathMappings: pathMappings}
}

func mergeMutationSignals(existing, mutationSignals *TestSignalReport) *TestSignalReport {
	if existing == nil {
		return mutationSignals
	}
	if mutationSignals == nil {
		return existing
	}
	byRoot := map[string]int{}
	for index, module := range existing.Modules {
		byRoot[module.Root] = index
	}
	for _, module := range mutationSignals.Modules {
		if index, ok := byRoot[module.Root]; ok {
			existing.Modules[index].Mutation = module.Mutation
			existing.Modules[index].MutationExecution = module.MutationExecution
			existing.Modules[index].Reports = append(existing.Modules[index].Reports, module.Reports...)
			continue
		}
		existing.Modules = append(existing.Modules, module)
	}
	existing.Diagnostics = append(existing.Diagnostics, mutationSignals.Diagnostics...)
	existing.PathMappings = append(existing.PathMappings, mutationSignals.PathMappings...)
	existing.Summary = TestSignalSummary{Enabled: true}
	summarizeTestSignalReport(existing)
	evaluateTestHealth(existing)
	return existing
}
