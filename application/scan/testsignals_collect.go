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

const commandOutputLimit = 16 * 1024

func executeTestCommand(projectDir string, module TestModuleSignal, diagnostics *[]TestSignalDiagnostic) *TestExecutionStatus {
	if module.Command == "" {
		return nil
	}
	started := time.Now()
	workingDir := filepath.Join(projectDir, filepath.FromSlash(module.Root))
	if module.Root == "." {
		workingDir = projectDir
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, commandShell(), commandShellArg(), module.Command)
	cmd.Dir = workingDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	status := &TestExecutionStatus{
		Mode:       TestModeRun,
		Command:    module.Command,
		WorkingDir: cleanRel(projectDir, workingDir),
		DurationMs: time.Since(started).Milliseconds(),
		Stdout:     limitOutput(stdout.String()),
		Stderr:     limitOutput(stderr.String()),
	}
	if err != nil {
		status.ExitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			status.ExitCode = exitErr.ExitCode()
		}
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "command_failed", Message: fmt.Sprintf("test command exited with status %d", status.ExitCode), Module: module.Name, Path: module.Root})
		return status
	}
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "command_executed", Message: "configured test command executed", Module: module.Name, Path: module.Root})
	return status
}

func commandShell() string {
	if os.PathSeparator == '\\' {
		return "cmd"
	}
	return "sh"
}

func commandShellArg() string {
	if os.PathSeparator == '\\' {
		return "/C"
	}
	return "-c"
}

func limitOutput(value string) string {
	if len(value) <= commandOutputLimit {
		return value
	}
	return value[:commandOutputLimit]
}

func boundedFallbackReports(projectDir string, module TestModuleSignal, opts TestOptions, scanStarted time.Time, diagnostics *[]TestSignalDiagnostic) []TestReportProvenance {
	candidates := collectFallbackCandidates(projectDir, fallbackRoot(projectDir, module), opts)
	if len(candidates) > opts.MaxCandidates {
		candidates = candidates[:opts.MaxCandidates]
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "report_candidate_limit", Message: "bounded fallback report search reached max_candidates", Module: module.Name, Path: module.Root})
	}
	reports := fallbackReportsFromCandidates(projectDir, module, opts, scanStarted, candidates, diagnostics)
	if len(reports) > 0 {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "fallback_reports_found", Message: "bounded fallback report search found candidates", Module: module.Name, Path: module.Root})
	}
	return reports
}

func fallbackRoot(projectDir string, module TestModuleSignal) string {
	if module.Root == "." {
		return projectDir
	}
	return filepath.Join(projectDir, filepath.FromSlash(module.Root))
}

func collectFallbackCandidates(projectDir, root string, opts TestOptions) []string {
	var candidates []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relRoot := cleanRel(root, path)
		if entry.IsDir() {
			if shouldSkipFallbackDir(projectDir, path, relRoot, entry.Name(), opts) {
				return filepath.SkipDir
			}
			return nil
		}
		if isFallbackReportCandidate(entry, opts) {
			candidates = append(candidates, path)
		}
		return nil
	})
	sort.Strings(candidates)
	return candidates
}

func shouldSkipFallbackDir(projectDir, path, relRoot, name string, opts TestOptions) bool {
	if relRoot == "." {
		return false
	}
	return shouldSkipReportDir(cleanRel(projectDir, path), name, opts) || depth(relRoot) > opts.MaxDepth
}

func isFallbackReportCandidate(entry os.DirEntry, opts TestOptions) bool {
	if !isKnownReportName(entry.Name()) {
		return false
	}
	info, err := entry.Info()
	return err == nil && info.Size() <= opts.MaxReportBytes
}

func fallbackReportsFromCandidates(projectDir string, module TestModuleSignal, opts TestOptions, scanStarted time.Time, candidates []string, diagnostics *[]TestSignalDiagnostic) []TestReportProvenance {
	reports := make([]TestReportProvenance, 0, len(candidates))
	for _, candidate := range candidates {
		report, ok := fallbackReportFromCandidate(projectDir, module, opts, scanStarted, candidate, diagnostics)
		if ok {
			reports = append(reports, report)
		}
	}
	return reports
}

func fallbackReportFromCandidate(projectDir string, module TestModuleSignal, opts TestOptions, scanStarted time.Time, candidate string, diagnostics *[]TestSignalDiagnostic) (TestReportProvenance, bool) {
	info, err := os.Stat(candidate)
	if err != nil {
		return TestReportProvenance{}, false
	}
	age := scanStarted.Sub(info.ModTime())
	freshness := "fresh"
	if age > opts.MaxReportAge {
		freshness = "stale"
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "report_stale", Message: "fallback report candidate is older than max_report_age", Module: module.Name, Path: cleanRel(projectDir, candidate)})
	}
	return TestReportProvenance{Kind: reportKindForPath(candidate), Path: cleanRel(projectDir, candidate), SourceMode: "fallback", Freshness: freshness, AgeMs: age.Milliseconds(), SizeBytes: info.Size()}, true
}

func shouldSkipReportDir(rel, name string, opts TestOptions) bool {
	if name == "build" || name == "target" || name == "coverage" {
		return false
	}
	return shouldSkipTestDir(rel, name, opts)
}

func isKnownReportName(name string) bool {
	lower := strings.ToLower(name)
	if lower == "lcov.info" || lower == "coverage.out" || lower == "cover.out" || lower == "coverage.xml" || lower == "coverage.json" || lower == "junit.xml" || lower == "test-results.xml" || lower == "cobertura.xml" || lower == "jacoco.xml" || lower == "ollanta-tests.json" {
		return true
	}
	return strings.HasPrefix(lower, "test-") && strings.HasSuffix(lower, ".xml")
}

func reportKindForPath(path string) string {
	lower := strings.ToLower(filepath.Base(path))
	switch {
	case lower == "lcov.info", lower == "coverage.out", lower == "cover.out", lower == "coverage.xml", lower == "coverage.json", lower == "cobertura.xml", lower == "jacoco.xml":
		return "coverage"
	case lower == "ollanta-tests.json":
		return "native"
	default:
		return "test"
	}
}
