package scan

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func normalizeModuleSignals(projectDir string, module *TestModuleSignal, opts TestOptions, diagnostics *[]TestSignalDiagnostic) {
	for _, report := range module.Reports {
		fullPath := filepath.Join(projectDir, filepath.FromSlash(report.Path))
		switch report.Kind {
		case "native":
			mergeNativeReport(fullPath, module, diagnostics)
		case "test":
			mergeJUnitReport(fullPath, module, diagnostics)
		case "coverage", "candidate":
			mergeCoverageReport(projectDir, fullPath, module, opts, diagnostics)
		default:
			mergeCoverageReport(projectDir, fullPath, module, opts, diagnostics)
			mergeJUnitReport(fullPath, module, diagnostics)
		}
	}
	mergeSuites(module)
	summarizeModuleSignals(module)
}

func mergeNativeReport(path string, module *TestModuleSignal, diagnostics *[]TestSignalDiagnostic) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var native TestModuleSignal
	if err := json.Unmarshal(data, &native); err != nil {
		var wrapped struct {
			Module TestModuleSignal `json:"module"`
		}
		if wrappedErr := json.Unmarshal(data, &wrapped); wrappedErr != nil {
			*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "native_report_invalid", Message: "native Ollanta test-signal JSON could not be decoded", Module: module.Name, Path: path})
			return
		}
		native = wrapped.Module
	}
	if native.Coverage != nil {
		module.Coverage = native.Coverage
	}
	module.Files = append(module.Files, native.Files...)
	module.Suites = append(module.Suites, native.Suites...)
	if native.Mutation != nil {
		module.Mutation = native.Mutation
	}
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "native_report_loaded", Message: "native Ollanta test-signal JSON loaded", Module: module.Name, Path: path})
}

func mergeCoverageReport(projectDir, path string, module *TestModuleSignal, opts TestOptions, diagnostics *[]TestSignalDiagnostic) {
	lower := strings.ToLower(filepath.Base(path))
	switch {
	case lower == "lcov.info":
		mergeLCOVReport(projectDir, path, module, opts, diagnostics)
	case lower == "coverage.out" || lower == "cover.out":
		mergeGoCoverReport(projectDir, path, module, opts, diagnostics)
	case strings.HasSuffix(lower, ".xml"):
		mergeCoberturaLikeReport(projectDir, path, module, opts, diagnostics)
	}
}

func mergeGoCoverReport(projectDir, path string, module *TestModuleSignal, opts TestOptions, diagnostics *[]TestSignalDiagnostic) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	covered := map[string]map[int]bool{}
	uncovered := map[string]map[int]bool{}
	normalizedPaths := map[string]string{}
	unmappedPaths := map[string]bool{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		count, _ := strconv.Atoi(fields[2])
		fileRange := fields[0]
		colon := strings.LastIndex(fileRange, ":")
		comma := strings.Index(fileRange, ",")
		if colon < 0 || comma < colon {
			continue
		}
		reportPath := fileRange[:colon]
		startLine := parseLineNumber(fileRange[colon+1 : comma])
		endLine := parseLineNumber(fileRange[comma+1:])
		normalized, ok := normalizeCachedReportPath(projectDir, module, reportPath, opts, diagnostics, normalizedPaths, unmappedPaths)
		if !ok {
			continue
		}
		if count > 0 {
			addLineRange(covered, normalized, startLine, endLine, true)
		} else {
			addLineRange(uncovered, normalized, startLine, endLine, true)
		}
	}
	appendCoverageFiles(module, covered, uncovered)
}

func normalizeCachedReportPath(projectDir string, module *TestModuleSignal, reportPath string, opts TestOptions, diagnostics *[]TestSignalDiagnostic, normalizedPaths map[string]string, unmappedPaths map[string]bool) (string, bool) {
	if normalized, ok := normalizedPaths[reportPath]; ok {
		return normalized, true
	}
	if unmappedPaths[reportPath] {
		return "", false
	}
	normalized, ok := normalizeReportPath(projectDir, module, reportPath, opts, diagnostics)
	if ok {
		normalizedPaths[reportPath] = normalized
		return normalized, true
	}
	unmappedPaths[reportPath] = true
	return "", false
}

func mergeLCOVReport(projectDir, path string, module *TestModuleSignal, opts TestOptions, diagnostics *[]TestSignalDiagnostic) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	covered := map[string]map[int]bool{}
	uncovered := map[string]map[int]bool{}
	currentPath := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "SF:") {
			var ok bool
			currentPath, ok = normalizeReportPath(projectDir, module, strings.TrimPrefix(line, "SF:"), opts, diagnostics)
			if !ok {
				currentPath = ""
			}
			continue
		}
		if currentPath == "" || !strings.HasPrefix(line, "DA:") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
		if len(parts) < 2 {
			continue
		}
		lineNumber, _ := strconv.Atoi(parts[0])
		count, _ := strconv.Atoi(parts[1])
		if count > 0 {
			addLineRange(covered, currentPath, lineNumber, lineNumber, true)
		} else {
			addLineRange(uncovered, currentPath, lineNumber, lineNumber, true)
		}
	}
	appendCoverageFiles(module, covered, uncovered)
}

func mergeCoberturaLikeReport(projectDir, path string, module *TestModuleSignal, opts TestOptions, diagnostics *[]TestSignalDiagnostic) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var doc struct {
		Packages []struct {
			Classes []struct {
				Filename string `xml:"filename,attr"`
				Lines    []struct {
					Number int `xml:"number,attr"`
					Hits   int `xml:"hits,attr"`
				} `xml:"lines>line"`
			} `xml:"classes>class"`
		} `xml:"packages>package"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return
	}
	covered := map[string]map[int]bool{}
	uncovered := map[string]map[int]bool{}
	state := coberturaMergeState{projectDir: projectDir, module: module, opts: opts, diagnostics: diagnostics, covered: covered, uncovered: uncovered}
	for _, pkg := range doc.Packages {
		for _, class := range pkg.Classes {
			state.mergeClass(class.Filename, class.Lines)
		}
	}
	appendCoverageFiles(module, covered, uncovered)
}

type coberturaMergeState struct {
	projectDir  string
	module      *TestModuleSignal
	opts        TestOptions
	diagnostics *[]TestSignalDiagnostic
	covered     map[string]map[int]bool
	uncovered   map[string]map[int]bool
}

func (state coberturaMergeState) mergeClass(filename string, lines []struct {
	Number int `xml:"number,attr"`
	Hits   int `xml:"hits,attr"`
}) {
	path, ok := normalizeReportPath(state.projectDir, state.module, filename, state.opts, state.diagnostics)
	if !ok {
		return
	}
	for _, line := range lines {
		if line.Hits > 0 {
			addLineRange(state.covered, path, line.Number, line.Number, true)
			continue
		}
		addLineRange(state.uncovered, path, line.Number, line.Number, true)
	}
}

func mergeJUnitReport(path string, module *TestModuleSignal, diagnostics *[]TestSignalDiagnostic) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var suites junitSuites
	if err := xml.Unmarshal(data, &suites); err != nil || len(suites.Suites) == 0 {
		var suite junitSuite
		if suiteErr := xml.Unmarshal(data, &suite); suiteErr != nil {
			return
		}
		suites.Suites = []junitSuite{suite}
	}
	for _, suite := range suites.Suites {
		module.Suites = append(module.Suites, suite.toSignal())
	}
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "info", Code: "junit_report_loaded", Message: "JUnit XML test report loaded", Module: module.Name, Path: path})
}

type junitSuites struct {
	Name   string       `xml:"name,attr"`
	Suites []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      string          `xml:"time,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name      string  `xml:"name,attr"`
	ClassName string  `xml:"classname,attr"`
	File      string  `xml:"file,attr"`
	Time      string  `xml:"time,attr"`
	Failure   *string `xml:"failure"`
	Error     *string `xml:"error"`
	Skipped   *string `xml:"skipped"`
}

func (s junitSuite) toSignal() TestSuiteSignal {
	signal := TestSuiteSignal{Name: s.Name, Kind: "unit", Tests: s.Tests, Failures: s.Failures, Errors: s.Errors, Skipped: s.Skipped, DurationMs: secondsStringToMs(s.Time)}
	for _, testCase := range s.TestCases {
		status := "passed"
		message := ""
		if testCase.Failure != nil {
			status = "failed"
			message = *testCase.Failure
		} else if testCase.Error != nil {
			status = "errored"
			message = *testCase.Error
		} else if testCase.Skipped != nil {
			status = "skipped"
			message = *testCase.Skipped
		}
		signal.Cases = append(signal.Cases, TestCaseSignal{Name: testCase.Name, ClassName: testCase.ClassName, File: filepath.ToSlash(testCase.File), Status: status, DurationMs: secondsStringToMs(testCase.Time), Message: strings.TrimSpace(message)})
	}
	if signal.Tests == 0 && len(signal.Cases) > 0 {
		signal.Tests = len(signal.Cases)
	}
	signal.Passed = signal.Tests - signal.Failures - signal.Errors - signal.Skipped
	if signal.Passed < 0 {
		signal.Passed = 0
	}
	return signal
}

func parseLineNumber(value string) int {
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		value = value[:dot]
	}
	line, _ := strconv.Atoi(value)
	return line
}

func addLineRange(target map[string]map[int]bool, path string, startLine, endLine int, value bool) {
	if startLine <= 0 {
		return
	}
	if endLine < startLine {
		endLine = startLine
	}
	if target[path] == nil {
		target[path] = map[int]bool{}
	}
	for line := startLine; line <= endLine; line++ {
		target[path][line] = value
	}
}

func appendCoverageFiles(module *TestModuleSignal, covered, uncovered map[string]map[int]bool) {
	paths := map[string]bool{}
	for path := range covered {
		paths[path] = true
	}
	for path := range uncovered {
		paths[path] = true
	}
	for path := range paths {
		file := TestFileCoverage{Path: path, CoveredLines: len(covered[path])}
		for line := range covered[path] {
			file.CoveredLineNumbers = append(file.CoveredLineNumbers, line)
		}
		sort.Ints(file.CoveredLineNumbers)
		for line := range uncovered[path] {
			if !covered[path][line] {
				file.UncoveredLines = append(file.UncoveredLines, line)
			}
		}
		sort.Ints(file.UncoveredLines)
		file.LinesToCover = file.CoveredLines + len(file.UncoveredLines)
		module.Files = append(module.Files, file)
	}
}

func summarizeModuleSignals(module *TestModuleSignal) {
	mergeCoverageFiles(module)
	linesToCover := 0
	coveredLines := 0
	uncoveredLines := 0
	for _, file := range module.Files {
		linesToCover += file.LinesToCover
		coveredLines += file.CoveredLines
		uncoveredLines += len(file.UncoveredLines)
	}
	if linesToCover > 0 {
		coverage := float64(coveredLines) * 100 / float64(linesToCover)
		module.Coverage = &TestCoverageSummary{LinesToCover: linesToCover, CoveredLines: coveredLines, UncoveredLines: uncoveredLines, Coverage: &coverage}
	}
	if len(module.Suites) == 0 {
		return
	}
	for i := range module.Suites {
		if module.Suites[i].Passed == 0 && module.Suites[i].Tests > 0 {
			module.Suites[i].Passed = module.Suites[i].Tests - module.Suites[i].Failures - module.Suites[i].Errors - module.Suites[i].Skipped
			if module.Suites[i].Passed < 0 {
				module.Suites[i].Passed = 0
			}
		}
	}
}

func mergeCoverageFiles(module *TestModuleSignal) {
	byPath := map[string]TestFileCoverage{}
	for _, file := range module.Files {
		byPath[file.Path] = mergeCoverageFile(byPath[file.Path], file)
	}
	module.Files = module.Files[:0]
	for _, file := range byPath {
		module.Files = append(module.Files, file)
	}
	sort.Slice(module.Files, func(i, j int) bool { return module.Files[i].Path < module.Files[j].Path })
}

func mergeCoverageFile(existing, next TestFileCoverage) TestFileCoverage {
	existing.Path = next.Path
	covered := lineSet(existing.CoveredLineNumbers, next.CoveredLineNumbers)
	uncovered := lineSet(existing.UncoveredLines, next.UncoveredLines)
	for line := range covered {
		delete(uncovered, line)
	}
	existing.CoveredLineNumbers = sortedLines(covered)
	existing.UncoveredLines = sortedLines(uncovered)
	if len(existing.CoveredLineNumbers) > 0 {
		existing.CoveredLines = len(existing.CoveredLineNumbers)
	} else {
		existing.CoveredLines += next.CoveredLines
	}
	existing.LinesToCover = existing.CoveredLines + len(existing.UncoveredLines)
	return existing
}

func lineSet(lineGroups ...[]int) map[int]bool {
	set := map[int]bool{}
	for _, lines := range lineGroups {
		for _, line := range lines {
			set[line] = true
		}
	}
	return set
}

func sortedLines(set map[int]bool) []int {
	lines := make([]int, 0, len(set))
	for line := range set {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	return lines
}

func mergeSuites(module *TestModuleSignal) {
	if len(module.Suites) < 2 {
		return
	}
	seenCases := map[string]bool{}
	merged := make([]TestSuiteSignal, 0, len(module.Suites))
	for _, suite := range module.Suites {
		uniqueCases := uniqueSuiteCases(suite, seenCases)
		suite.Cases = uniqueCases
		if len(uniqueCases) > 0 {
			recountSuiteCases(&suite)
		}
		merged = append(merged, suite)
	}
	module.Suites = merged
}

func uniqueSuiteCases(suite TestSuiteSignal, seenCases map[string]bool) []TestCaseSignal {
	uniqueCases := suite.Cases[:0]
	for _, testCase := range suite.Cases {
		key := testCase.ID
		if key == "" {
			key = suite.Name + ":" + testCase.ClassName + ":" + testCase.Name
		}
		if seenCases[key] {
			continue
		}
		seenCases[key] = true
		uniqueCases = append(uniqueCases, testCase)
	}
	return uniqueCases
}

func recountSuiteCases(suite *TestSuiteSignal) {
	suite.Tests = len(suite.Cases)
	suite.Failures, suite.Errors, suite.Skipped = 0, 0, 0
	for _, testCase := range suite.Cases {
		switch testCase.Status {
		case "failed":
			suite.Failures++
		case "errored":
			suite.Errors++
		case "skipped":
			suite.Skipped++
		}
	}
	suite.Passed = suite.Tests - suite.Failures - suite.Errors - suite.Skipped
}

func normalizeReportPath(projectDir string, module *TestModuleSignal, reportPath string, opts TestOptions, diagnostics *[]TestSignalDiagnostic) (string, bool) {
	path := filepath.Clean(reportPath)
	for _, mapping := range opts.PathMappings {
		from := filepath.Clean(mapping.From)
		if strings.HasPrefix(path, from) {
			mapped := filepath.Join(projectDir, filepath.FromSlash(mapping.To), strings.TrimPrefix(path, from))
			return cleanRel(projectDir, mapped), true
		}
	}
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(projectDir, path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(filepath.Clean(rel)), true
		}
		return suffixMatchPath(projectDir, module, path, diagnostics)
	}
	candidate := filepath.Join(projectDir, filepath.FromSlash(module.Root), path)
	if module.Root == "." {
		candidate = filepath.Join(projectDir, path)
	}
	if _, err := os.Stat(candidate); err == nil {
		return cleanRel(projectDir, candidate), true
	}
	if normalized, ok := directProjectFilePath(projectDir, path); ok {
		return normalized, true
	}
	return suffixMatchPath(projectDir, module, path, diagnostics)
}

func directProjectFilePath(projectDir, reportPath string) (string, bool) {
	suffix := filepath.ToSlash(filepath.Clean(reportPath))
	if suffix == "." || strings.HasPrefix(suffix, "../") {
		return "", false
	}
	for suffix != "" {
		candidate := filepath.Join(projectDir, filepath.FromSlash(suffix))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.ToSlash(filepath.Clean(suffix)), true
		}
		slash := strings.IndexByte(suffix, '/')
		if slash < 0 {
			break
		}
		suffix = suffix[slash+1:]
	}
	return "", false
}

func suffixMatchPath(projectDir string, module *TestModuleSignal, reportPath string, diagnostics *[]TestSignalDiagnostic) (string, bool) {
	suffix := filepath.ToSlash(filepath.Clean(reportPath))
	var matches []string
	_ = filepath.WalkDir(projectDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := cleanRel(projectDir, path)
		if entry.IsDir() {
			if rel != "." && isOutOfProjectOrGenerated(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if isOutOfProjectOrGenerated(rel) {
			return nil
		}
		if strings.HasSuffix(filepath.ToSlash(rel), suffix) || strings.HasSuffix(suffix, filepath.ToSlash(rel)) {
			matches = append(matches, rel)
		}
		return nil
	})
	if len(matches) == 1 {
		return matches[0], true
	}
	if len(matches) > 1 {
		*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "path_mapping_ambiguous", Message: "report path matched multiple project files", Module: module.Name, Path: reportPath})
		return "", false
	}
	*diagnostics = append(*diagnostics, TestSignalDiagnostic{Level: "warn", Code: "path_out_of_project", Message: "report path could not be mapped to a project file", Module: module.Name, Path: reportPath})
	return "", false
}

func isOutOfProjectOrGenerated(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if defaultTestExcludedDirs[part] || part == "generated" {
			return true
		}
	}
	return false
}

func secondsStringToMs(value string) int64 {
	if value == "" {
		return 0
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int64(seconds * 1000)
}

func marshalTestSignals(report *TestSignalReport) string {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", report)
	}
	return string(data)
}
