package scan

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	mutationStatusKilled       = "killed"
	mutationStatusSurvived     = "survived"
	mutationStatusNoCoverage   = "no-coverage"
	mutationStatusTimeout      = "timeout"
	mutationStatusSkipped      = "skipped"
	mutationStatusEquivalent   = "equivalent"
	mutationStatusIgnored      = "ignored"
	mutationStatusNonViable    = "non-viable"
	mutationStatusRuntimeError = "runtime-error"
	mutationStatusParserError  = "parser-error"
)

func normalizeModuleMutationSignals(projectDir string, module *TestModuleSignal, opts TestOptions, diagnostics *[]TestSignalDiagnostic) {
	if module.Mutation == nil {
		module.Mutation = &TestMutationSummary{Status: "unavailable", Confidence: "low"}
	}
	for _, report := range module.Reports {
		if report.Kind != "mutation" && report.Kind != "native_mutation" {
			continue
		}
		if report.Freshness == "stale" {
			module.Mutation.Stale = true
		}
		module.Mutation.Reports = append(module.Mutation.Reports, report)
		path := filepath.Join(projectDir, filepath.FromSlash(report.Path))
		if report.Kind == "native_mutation" || filepath.Base(path) == nativeMutationFile {
			mergeNativeMutationReport(projectDir, module, path, opts, diagnostics)
			continue
		}
		mergeMutationReport(projectDir, module, path, opts, diagnostics)
	}
	finalizeMutationSummary(module)
}

func mergeMutationReport(projectDir string, module *TestModuleSignal, path string, opts TestOptions, diagnostics *[]TestSignalDiagnostic) {
	data, err := os.ReadFile(path)
	if err != nil {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "mutation_report_unreadable", Message: "mutation report could not be read", Module: module.Name, Path: cleanRel(projectDir, path)})
		return
	}
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "<") {
		if parsePITMutationXML(projectDir, module, data, cleanRel(projectDir, path), opts, diagnostics) {
			return
		}
	} else if parseStrykerMutationJSON(projectDir, module, data, cleanRel(projectDir, path), opts, diagnostics) {
		return
	}
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "mutation_report_unsupported", Message: "mutation report format is not supported yet", Module: module.Name, Path: cleanRel(projectDir, path)})
}

func mergeNativeMutationReport(projectDir string, module *TestModuleSignal, path string, opts TestOptions, diagnostics *[]TestSignalDiagnostic) {
	data, err := os.ReadFile(path)
	if err != nil {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "native_mutation_report_unreadable", Message: "native mutation report could not be read", Module: module.Name, Path: cleanRel(projectDir, path)})
		return
	}
	var payload struct {
		Mutation *TestMutationSummary `json:"mutation"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.Mutation == nil {
		var direct TestMutationSummary
		if err := json.Unmarshal(data, &direct); err != nil {
			*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "native_mutation_report_invalid", Message: "native mutation report could not be parsed", Module: module.Name, Path: cleanRel(projectDir, path)})
			return
		}
		payload.Mutation = &direct
	}
	mergeMutationSummary(module, *payload.Mutation, diagnostics)
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "native_mutation_report_loaded", Message: "native mutation report loaded", Module: module.Name, Path: cleanRel(projectDir, path)})
}

func parseStrykerMutationJSON(projectDir string, module *TestModuleSignal, data []byte, reportPath string, opts TestOptions, diagnostics *[]TestSignalDiagnostic) bool {
	var payload struct {
		MutationScore *float64 `json:"mutationScore"`
		Files         map[string]struct {
			Mutants []struct {
				ID          string `json:"id"`
				Status      string `json:"status"`
				MutatorName string `json:"mutatorName"`
				Replacement string `json:"replacement"`
				Description string `json:"description"`
				Location    struct {
					Start struct {
						Line int `json:"line"`
					} `json:"start"`
					End struct {
						Line int `json:"line"`
					} `json:"end"`
				} `json:"location"`
			} `json:"mutants"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || len(payload.Files) == 0 {
		return false
	}
	summary := TestMutationSummary{Tool: mutationToolStryker, Status: "available", Confidence: "high", Score: payload.MutationScore}
	seenMutants := map[string]bool{}
	for filePath, file := range payload.Files {
		normalizedPath := mappedMutationPath(projectDir, module, filePath, opts, diagnostics)
		for _, mutant := range file.Mutants {
			key := mutant.ID
			if key == "" {
				key = stableMutantID(normalizedPath, mutant.MutatorName, mutant.Replacement, mutant.Location.Start.Line, 0)
			}
			if seenMutants[key] {
				*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_duplicate_mutant", Message: "duplicate mutant skipped", Module: module.Name, Path: normalizedPath})
				continue
			}
			seenMutants[key] = true
			normalizedStatus := normalizeMutationStatus(mutant.Status)
			addMutationStatus(&summary, mutant.Status)
			if normalizedStatus == mutationStatusSurvived || normalizedStatus == mutationStatusNoCoverage {
				summary.SurvivedMutants = append(summary.SurvivedMutants, TestMutantSignal{ID: mutant.ID, Status: mutationStatusSurvived, Mutator: mutant.MutatorName, File: normalizedPath, Line: mutant.Location.Start.Line, EndLine: mutant.Location.End.Line, Replacement: mutant.Replacement, Description: mutant.Description, Confidence: "high"})
			}
		}
	}
	summary.Suites = append(summary.Suites, TestMutationSuite{Name: "stryker", Tool: mutationToolStryker, Report: reportPath, Total: summary.Total, Killed: summary.Killed, Survived: summary.Survived, Timeout: summary.Timeout, Skipped: summary.Skipped, Errors: summary.Errors})
	mergeMutationSummary(module, summary, diagnostics)
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "stryker_mutation_report_loaded", Message: "Stryker mutation report loaded", Module: module.Name, Path: reportPath})
	return true
}

func parsePITMutationXML(projectDir string, module *TestModuleSignal, data []byte, reportPath string, opts TestOptions, diagnostics *[]TestSignalDiagnostic) bool {
	var payload struct {
		Mutations []struct {
			Detected     string `xml:"detected,attr"`
			Status       string `xml:"status,attr"`
			SourceFile   string `xml:"sourceFile"`
			MutatedClass string `xml:"mutatedClass"`
			Mutator      string `xml:"mutator"`
			LineNumber   int    `xml:"lineNumber"`
			Description  string `xml:"description"`
		} `xml:"mutation"`
	}
	if err := xml.Unmarshal(data, &payload); err != nil || len(payload.Mutations) == 0 {
		return false
	}
	summary := TestMutationSummary{Tool: mutationToolPIT, Status: "available", Confidence: "high"}
	seenMutants := map[string]bool{}
	for _, mutant := range payload.Mutations {
		key := stableMutantID(mutant.MutatedClass, mutant.Mutator, mutant.SourceFile, mutant.LineNumber)
		if seenMutants[key] {
			*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_duplicate_mutant", Message: "duplicate mutant skipped", Module: module.Name, Path: mutant.SourceFile})
			continue
		}
		seenMutants[key] = true
		status := mutant.Status
		if strings.EqualFold(mutant.Detected, "true") && status == "" {
			status = "killed"
		}
		normalizedStatus := normalizeMutationStatus(status)
		addMutationStatus(&summary, status)
		if normalizedStatus == mutationStatusSurvived || normalizedStatus == mutationStatusNoCoverage {
			path := mappedMutationPath(projectDir, module, mutant.SourceFile, opts, diagnostics)
			summary.SurvivedMutants = append(summary.SurvivedMutants, TestMutantSignal{ID: key, Status: mutationStatusSurvived, Mutator: mutant.Mutator, File: path, Line: mutant.LineNumber, Description: mutant.Description, Confidence: "medium"})
		}
	}
	summary.Suites = append(summary.Suites, TestMutationSuite{Name: "pit", Tool: mutationToolPIT, Report: reportPath, Total: summary.Total, Killed: summary.Killed, Survived: summary.Survived, Timeout: summary.Timeout, Skipped: summary.Skipped, Errors: summary.Errors})
	mergeMutationSummary(module, summary, diagnostics)
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "pit_mutation_report_loaded", Message: "PIT mutation report loaded", Module: module.Name, Path: reportPath})
	return true
}

func mergeMutationSummary(module *TestModuleSignal, next TestMutationSummary, diagnostics *[]TestSignalDiagnostic) {
	if module.Mutation == nil {
		module.Mutation = &TestMutationSummary{}
	}
	current := module.Mutation
	if next.Tool != "" {
		current.Tool = next.Tool
	}
	if next.Status != "" {
		current.Status = next.Status
	}
	if next.Confidence != "" {
		current.Confidence = next.Confidence
	}
	if next.Score != nil {
		current.Score = next.Score
	}
	if next.ChangedCodeScore != nil {
		current.ChangedCodeScore = next.ChangedCodeScore
	}
	current.Total += next.Total
	current.Testable += next.Testable
	current.Killed += next.Killed
	current.Survived += next.Survived
	current.NoCoverage += next.NoCoverage
	current.Timeout += next.Timeout
	current.Skipped += next.Skipped
	current.Errors += next.Errors
	current.NonViable += next.NonViable
	current.RuntimeErrors += next.RuntimeErrors
	current.ParserErrors += next.ParserErrors
	current.Equivalent += next.Equivalent
	current.Ignored += next.Ignored
	current.ChangedTotal += next.ChangedTotal
	current.ChangedTestable += next.ChangedTestable
	current.ChangedKilled += next.ChangedKilled
	current.ChangedSurvived += next.ChangedSurvived
	current.ChangedNoCoverage += next.ChangedNoCoverage
	if len(next.StatusCounts) > 0 {
		if current.StatusCounts == nil {
			current.StatusCounts = map[string]int{}
		}
		for status, count := range next.StatusCounts {
			current.StatusCounts[status] += count
		}
	}
	current.Suites = append(current.Suites, next.Suites...)
	current.SurvivedMutants = append(current.SurvivedMutants, next.SurvivedMutants...)
	deduplicateSurvivedMutants(current, diagnostics, module.Name)
}

func finalizeMutationSummary(module *TestModuleSignal) {
	if module.Mutation == nil {
		return
	}
	mutation := module.Mutation
	fillMutationScores(mutation)
	fillMutationDefaults(mutation)
	sortSurvivedMutants(mutation)
}

func fillMutationScores(mutation *TestMutationSummary) {
	if mutation.Testable == 0 && mutation.Killed+mutation.Survived > 0 {
		mutation.Testable = mutation.Killed + mutation.Survived
	}
	if mutation.Total == 0 {
		mutation.Total = mutation.Killed + mutation.Survived + mutation.NoCoverage + mutation.Timeout + mutation.Skipped + mutation.Errors + mutation.NonViable + mutation.RuntimeErrors + mutation.Equivalent + mutation.Ignored
	}
	if mutation.Testable > 0 && mutation.Score == nil {
		score := float64(mutation.Killed) * 100 / float64(mutation.Testable)
		mutation.Score = &score
	}
	if mutation.ChangedTestable == 0 && mutation.ChangedKilled+mutation.ChangedSurvived > 0 {
		mutation.ChangedTestable = mutation.ChangedKilled + mutation.ChangedSurvived
	}
	if mutation.ChangedTestable > 0 && mutation.ChangedCodeScore == nil {
		score := float64(mutation.ChangedKilled) * 100 / float64(mutation.ChangedTestable)
		mutation.ChangedCodeScore = &score
	}
}

func fillMutationDefaults(mutation *TestMutationSummary) {
	if mutation.Status == "" {
		mutation.Status = "available"
	}
	if mutation.Confidence == "" {
		mutation.Confidence = "medium"
	}
	if mutation.Availability == "" {
		if mutation.Testable > 0 || mutation.Total > 0 || mutation.Score != nil || len(mutation.Reports) > 0 {
			mutation.Availability = EvidenceAvailabilityAvailable
		} else {
			mutation.Availability = EvidenceAvailabilityUnavailable
		}
	}
}

func sortSurvivedMutants(mutation *TestMutationSummary) {
	sort.Slice(mutation.SurvivedMutants, func(i, j int) bool {
		left := mutation.SurvivedMutants[i]
		right := mutation.SurvivedMutants[j]
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.ID < right.ID
	})
}

func deduplicateSurvivedMutants(summary *TestMutationSummary, diagnostics *[]TestSignalDiagnostic, moduleName string) {
	seen := map[string]bool{}
	unique := summary.SurvivedMutants[:0]
	for _, mutant := range summary.SurvivedMutants {
		key := mutant.ID
		if key == "" {
			key = stableMutantID(mutant.File, mutant.Mutator, mutant.Original+mutant.Replacement, mutant.Line, 0)
		}
		if seen[key] {
			*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "mutation_duplicate_mutant", Message: "duplicate survived mutant skipped", Module: moduleName, Path: mutant.File})
			continue
		}
		seen[key] = true
		unique = append(unique, mutant)
	}
	summary.SurvivedMutants = unique
}

func addMutationStatus(summary *TestMutationSummary, status string) {
	normalized := normalizeMutationStatus(status)
	if summary.StatusCounts == nil {
		summary.StatusCounts = map[string]int{}
	}
	summary.StatusCounts[normalized]++
	summary.Total++
	switch normalized {
	case mutationStatusKilled:
		summary.Killed++
		summary.Testable++
	case mutationStatusSurvived:
		summary.Survived++
		summary.Testable++
	case mutationStatusNoCoverage:
		summary.NoCoverage++
		summary.Survived++
		summary.Testable++
	case mutationStatusTimeout:
		summary.Timeout++
	case mutationStatusSkipped:
		summary.Skipped++
	case mutationStatusEquivalent:
		summary.Equivalent++
	case mutationStatusIgnored:
		summary.Ignored++
	case mutationStatusNonViable:
		summary.NonViable++
	case mutationStatusRuntimeError:
		summary.RuntimeErrors++
	case mutationStatusParserError:
		summary.ParserErrors++
	default:
		summary.Errors++
	}
}

func normalizeMutationStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	status = strings.ReplaceAll(status, "_", "-")
	compact := strings.NewReplacer("-", "", " ", "", ".", "").Replace(status)
	switch compact {
	case "killed", "covered", "detected":
		return mutationStatusKilled
	case "survived", "notkilled", "undetected", "live":
		return mutationStatusSurvived
	case "nocoverage", "notcovered":
		return mutationStatusNoCoverage
	case "timedout", "timeout", "timeouted":
		return mutationStatusTimeout
	case "ignored", "static":
		return mutationStatusIgnored
	case "skipped", "notrun", "pending":
		return mutationStatusSkipped
	case "equivalent":
		return mutationStatusEquivalent
	case "nonviable", "compileerror", "syntaxerror", "incompetent":
		return mutationStatusNonViable
	case "runtimeerror", "runerror", "memoryerror":
		return mutationStatusRuntimeError
	case "parsererror":
		return mutationStatusParserError
	default:
		return status
	}
}

func mappedMutationPath(projectDir string, module *TestModuleSignal, reportPath string, opts TestOptions, diagnostics *[]TestSignalDiagnostic) string {
	path, ok := normalizeReportPath(projectDir, module, reportPath, opts, diagnostics)
	if !ok {
		return filepath.ToSlash(filepath.Clean(reportPath))
	}
	return path
}

func stableMutantID(parts ...any) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		values = append(values, strings.TrimSpace(strings.ReplaceAll(strings.ToLower(toString(part)), " ", "-")))
	}
	return strings.Join(values, ":")
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}
