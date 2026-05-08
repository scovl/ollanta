package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDiscoverTestModulesFromGoWorkspaceAndNestedMarkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.work"), "go 1.21\n\nuse (\n\t./domain\n\t./adapter/http\n)\n")
	writeTestFile(t, filepath.Join(dir, "domain", "go.mod"), "module example/domain\n")
	writeTestFile(t, filepath.Join(dir, "adapter", "http", "go.mod"), "module example/adapter\n")
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"workspaces":["web/*"]}`)
	writeTestFile(t, filepath.Join(dir, "web", "ui", "package.json"), `{"devDependencies":{"typescript":"^5.0.0"}}`)
	writeTestFile(t, filepath.Join(dir, "web", "ui", "tsconfig.json"), `{}`)
	writeTestFile(t, filepath.Join(dir, "node_modules", "ignored", "package.json"), `{}`)

	modules, diagnostics, err := DiscoverTestModules(dir, TestOptions{MaxDepth: 6})
	if err != nil {
		t.Fatalf("DiscoverTestModules() error = %v", err)
	}
	byRoot := modulesByRoot(modules)
	assertModule(t, byRoot, "domain", "go", "domain")
	assertModule(t, byRoot, "adapter/http", "go", "adapter")
	assertModule(t, byRoot, "web/ui", "typescript", "web")
	if _, ok := byRoot["node_modules/ignored"]; ok {
		t.Fatal("node_modules package was discovered, want excluded")
	}
	if !hasDiagnostic(diagnostics, "module_path_ignored") {
		t.Fatalf("diagnostics = %#v, want ignored path diagnostic", diagnostics)
	}
}

func TestCollectTestSignalsReportsIgnoredModulesAndDoesNotRunCommands(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "adapters", "payment", "go.mod"), "module example/payment\n")

	report, err := CollectTestSignals(dir, TestOptions{
		Enabled:  true,
		Discover: false,
		Modules: []TestModuleConfig{{
			Name:         "payment-adapter",
			Root:         "adapters/payment",
			Language:     "go",
			TestPolicy:   TestPolicyIgnored,
			IgnoreReason: "covered by contract tests elsewhere",
			Command:      "go test ./...",
		}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if report.Summary.IgnoredModules != 1 {
		t.Fatalf("IgnoredModules = %d, want 1", report.Summary.IgnoredModules)
	}
	if !hasDiagnostic(report.Diagnostics, "module_ignored") || !hasDiagnostic(report.Diagnostics, "command_not_executed") {
		t.Fatalf("diagnostics = %#v, want ignored module and command diagnostics", report.Diagnostics)
	}
}

func TestCollectTestSignalsCommandPolicyDeniesDiscoveredCommands(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module example/app\n")

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: true, Run: true, CommandPolicy: CommandPolicyConfiguredOnly}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if len(report.Modules) != 1 || report.Modules[0].Execution == nil || !report.Modules[0].Execution.Partial {
		t.Fatalf("Execution = %+v, want denied partial execution status", report.Modules)
	}
	if !hasDiagnostic(report.Diagnostics, "command_policy_denied") {
		t.Fatalf("diagnostics = %#v, want command policy diagnostic", report.Diagnostics)
	}
}

func TestCollectTestSignalsTimeoutRecordsPartialStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	command := "sleep 1"
	if runtime.GOOS == "windows" {
		command = "ping -n 2 127.0.0.1 > nul"
	}

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: false, Run: true, MaxRuntime: time.Nanosecond, Modules: []TestModuleConfig{{Root: ".", Command: command}}}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	execution := report.Modules[0].Execution
	if execution == nil || !execution.Timeout || execution.ExitCode != 124 || !execution.Partial {
		t.Fatalf("Execution = %+v, want timeout partial status", execution)
	}
	if !hasDiagnostic(report.Diagnostics, "command_timeout") {
		t.Fatalf("diagnostics = %#v, want timeout diagnostic", report.Diagnostics)
	}
}

func TestCollectTestSignalsAppliesConfiguredOverridesAndReportFreshness(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "domain", "go.mod"), "module example/domain\n")
	coveragePath := filepath.Join(dir, "domain", "coverage.out")
	writeTestFile(t, coveragePath, "mode: set\n")
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(coveragePath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
	threshold := 85.0

	report, err := CollectTestSignals(dir, TestOptions{
		Enabled:      true,
		Discover:     true,
		MaxReportAge: time.Hour,
		Modules: []TestModuleConfig{{
			Name:              "core-domain",
			Root:              "domain",
			Language:          "go",
			ArchitectureRole:  "domain",
			CoverageReports:   []string{"coverage.out"},
			CoverageThreshold: &threshold,
		}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if report.Summary.Modules != 1 {
		t.Fatalf("Summary.Modules = %d, want 1", report.Summary.Modules)
	}
	module := report.Modules[0]
	if module.Name != "core-domain" || module.ArchitectureRole != "domain" || module.Source != TestSourceConfigured {
		t.Fatalf("module = %+v, want configured domain override", module)
	}
	if module.CoverageThreshold == nil || *module.CoverageThreshold != threshold {
		t.Fatalf("CoverageThreshold = %v, want %v", module.CoverageThreshold, threshold)
	}
	if len(module.Reports) != 1 || module.Reports[0].Freshness != "stale" {
		t.Fatalf("Reports = %#v, want one stale report", module.Reports)
	}
	if report.Summary.StaleReports != 1 {
		t.Fatalf("StaleReports = %d, want 1", report.Summary.StaleReports)
	}
	if !hasDiagnostic(report.Diagnostics, "module_duplicate") || !hasDiagnostic(report.Diagnostics, "report_stale") {
		t.Fatalf("diagnostics = %#v, want duplicate and stale diagnostics", report.Diagnostics)
	}
}

func TestCollectTestSignalsFallbackSearchParsesJUnitAndHealth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")
	writeTestFile(t, filepath.Join(dir, "build", "test-results", "TEST-app.xml"), `<testsuite name="unit" tests="2" failures="1" skipped="1" time="0.25"><testcase classname="Calc" name="adds" time="0.10"/><testcase classname="Calc" name="breaks"><failure>boom</failure></testcase></testsuite>`)

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: true, MaxDepth: 6}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if report.Summary.Tests != 2 || report.Summary.TestFailures != 1 || report.Summary.TestSkipped != 1 {
		t.Fatalf("summary = %+v modules = %+v diagnostics = %#v, want parsed JUnit totals", report.Summary, report.Modules, report.Diagnostics)
	}
	if report.Health == nil || report.Health.Status != "at_risk" {
		t.Fatalf("health = %+v, want at_risk", report.Health)
	}
	if !hasDiagnostic(report.Diagnostics, "fallback_reports_found") || !hasDiagnostic(report.Diagnostics, "junit_report_loaded") {
		t.Fatalf("diagnostics = %#v, want fallback and JUnit diagnostics", report.Diagnostics)
	}
}

func TestCollectTestSignalsDoctorModeDoesNotExecuteCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")

	report, err := CollectTestSignals(dir, TestOptions{
		Enabled: true,
		Mode:    TestModeDoctor,
		Run:     true,
		Modules: []TestModuleConfig{{Root: ".", Language: "go", Command: "go test ./..."}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if report.Modules[0].Execution != nil {
		t.Fatalf("Execution = %+v, want nil in doctor mode", report.Modules[0].Execution)
	}
	if !hasDiagnostic(report.Diagnostics, "doctor_command_candidate") || !hasDiagnostic(report.Diagnostics, "doctor_config_suggestion") {
		t.Fatalf("diagnostics = %#v, want doctor diagnostics", report.Diagnostics)
	}
}

func TestCollectTestSignalsRunCommandCollectsGeneratedGoCoverage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n\ngo 1.21\n")
	writeTestFile(t, filepath.Join(dir, "calc.go"), "package app\n\nfunc Add(a, b int) int { return a + b }\n")
	writeTestFile(t, filepath.Join(dir, "calc_test.go"), "package app\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { if Add(1, 2) != 3 { t.Fatal(\"bad math\") } }\n")

	report, err := CollectTestSignals(dir, TestOptions{
		Enabled:  true,
		Discover: false,
		Run:      true,
		Modules:  []TestModuleConfig{{Root: ".", Language: "go", Command: "go test ./... -coverprofile=coverage.out", CoverageReports: []string{"coverage.out"}}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	module := report.Modules[0]
	if module.Execution == nil || module.Execution.ExitCode != 0 {
		t.Fatalf("Execution = %+v, want successful command", module.Execution)
	}
	if module.Coverage == nil || module.Coverage.Coverage == nil || *module.Coverage.Coverage <= 0 {
		t.Fatalf("Coverage = %+v, want generated Go coverage", module.Coverage)
	}
	if !hasDiagnostic(report.Diagnostics, "command_executed") {
		t.Fatalf("diagnostics = %#v, want command_executed", report.Diagnostics)
	}
}

func TestCollectTestSignalsMapsGoModuleImportCoveragePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "application", "ingest", "queue.go"), "package ingest\n\nfunc Queue() {}\n")
	writeTestFile(t, filepath.Join(dir, "coverage.out"), "mode: set\ngithub.com/scovl/ollanta/application/ingest/queue.go:1.1,1.20 1 1\n")

	report, err := CollectTestSignals(dir, TestOptions{
		Enabled:  true,
		Discover: false,
		Modules:  []TestModuleConfig{{Root: ".", Language: "go", CoverageReports: []string{"coverage.out"}}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	module := report.Modules[0]
	if len(module.Files) != 1 || module.Files[0].Path != "application/ingest/queue.go" {
		t.Fatalf("Files = %#v, want mapped Go module import path", module.Files)
	}
	if hasDiagnostic(report.Diagnostics, "path_out_of_project") || hasDiagnostic(report.Diagnostics, "path_mapping_ambiguous") {
		t.Fatalf("diagnostics = %#v, want direct path mapping without suffix fallback warnings", report.Diagnostics)
	}
}

func TestCollectTestSignalsParsesNativeJSONAndPathMappings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "src", "app.go"), "package src\n\nfunc App() {}\n")
	native := `{"coverage":{"lines_to_cover":2,"covered_lines":1,"coverage":50},"suites":[{"name":"native","tests":1,"passed":1}],"mutation":{"score":80,"total":10,"killed":8,"survived":2}}`
	writeTestFile(t, filepath.Join(dir, "ollanta-tests.json"), native)
	writeTestFile(t, filepath.Join(dir, "coverage.out"), "mode: set\n/ci/work/src/app.go:1.1,1.20 1 1\n/ci/work/src/app.go:3.1,3.20 1 0\n")

	report, err := CollectTestSignals(dir, TestOptions{
		Enabled:      true,
		Discover:     false,
		PathMappings: []TestPathMapping{{From: filepath.Clean("/ci/work"), To: "."}},
		Modules: []TestModuleConfig{{
			Root:            ".",
			Language:        "go",
			NativeReports:   []string{"ollanta-tests.json"},
			CoverageReports: []string{"coverage.out"},
		}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	module := report.Modules[0]
	if report.Summary.Tests != 1 || report.Summary.MutantsKilled != 8 {
		t.Fatalf("summary = %+v, want native test and mutation totals", report.Summary)
	}
	if len(module.Files) != 1 || module.Files[0].Path != "src/app.go" {
		t.Fatalf("Files = %#v, want mapped src/app.go coverage", module.Files)
	}
	if !hasDiagnostic(report.Diagnostics, "native_report_loaded") {
		t.Fatalf("diagnostics = %#v, want native_report_loaded", report.Diagnostics)
	}
}

func TestCollectTestSignalsPathMappingRequiresBoundary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "coverage.out"), "mode: set\n/ci/workspace/src/app.go:1.1,1.20 1 1\n")

	report, err := CollectTestSignals(dir, TestOptions{
		Enabled:      true,
		Discover:     false,
		PathMappings: []TestPathMapping{{From: filepath.Clean("/ci/work"), To: "."}},
		Modules:      []TestModuleConfig{{Root: ".", Language: "go", CoverageReports: []string{"coverage.out"}}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if len(report.Modules[0].Files) != 0 {
		t.Fatalf("Files = %#v, want prefix /ci/work not to match /ci/workspace", report.Modules[0].Files)
	}
	if !hasDiagnostic(report.Diagnostics, "path_out_of_project") {
		t.Fatalf("diagnostics = %#v, want unmapped path diagnostic", report.Diagnostics)
	}
}

func TestCollectTestSignalsReportsUnsupportedCandidates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "coverage.json"), `{"meta":{"tool":"coverage.py"}}`)

	report, err := CollectTestSignals(dir, TestOptions{
		Enabled:  true,
		Discover: false,
		Modules:  []TestModuleConfig{{Root: ".", Language: "python", CoverageReports: []string{"coverage.json"}}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if !hasDiagnostic(report.Diagnostics, "report_format_unsupported") {
		t.Fatalf("diagnostics = %#v, want unsupported report diagnostic", report.Diagnostics)
	}
}

func TestCollectTestSignalsReportsAmbiguousSuffixPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a", "foo.go"), "package a\n")
	writeTestFile(t, filepath.Join(dir, "b", "foo.go"), "package b\n")
	writeTestFile(t, filepath.Join(dir, "coverage.out"), "mode: set\nfoo.go:1.1,1.10 1 1\n")

	report, err := CollectTestSignals(dir, TestOptions{
		Enabled:  true,
		Discover: false,
		Modules:  []TestModuleConfig{{Root: ".", Language: "go", CoverageReports: []string{"coverage.out"}}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if !hasDiagnostic(report.Diagnostics, "path_mapping_ambiguous") {
		t.Fatalf("diagnostics = %#v, want path_mapping_ambiguous", report.Diagnostics)
	}
}

func TestDiscoverMutationModulesDetectsStrykerWithoutExecution(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"devDependencies":{"@stryker-mutator/core":"^8.0.0"}}`)

	modules, diagnostics, err := DiscoverMutationModules(dir, MutationOptions{MaxDepth: 4})
	if err != nil {
		t.Fatalf("DiscoverMutationModules() error = %v", err)
	}
	if len(modules) != 1 || modules[0].Mutation == nil || modules[0].Mutation.Tool != mutationToolStryker {
		t.Fatalf("modules = %#v, want Stryker mutation module", modules)
	}
	if modules[0].MutationExecution != nil {
		t.Fatalf("MutationExecution = %+v, want nil during discovery", modules[0].MutationExecution)
	}
	if !hasDiagnostic(diagnostics, "mutation_tool_detected") {
		t.Fatalf("diagnostics = %#v, want mutation_tool_detected", diagnostics)
	}
}

func TestCollectMutationSignalsDoctorModeDoesNotExecuteCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"devDependencies":{"@stryker-mutator/core":"^8.0.0"}}`)

	report, err := CollectMutationSignals(dir, MutationOptions{Enabled: true, Mode: MutationModeDoctor, Run: true, Discover: true}, time.Now())
	if err != nil {
		t.Fatalf("CollectMutationSignals() error = %v", err)
	}
	if len(report.Modules) != 1 {
		t.Fatalf("Modules = %d, want 1", len(report.Modules))
	}
	if report.Modules[0].MutationExecution != nil {
		t.Fatalf("MutationExecution = %+v, want nil in doctor mode", report.Modules[0].MutationExecution)
	}
	if !hasDiagnostic(report.Diagnostics, "mutation_doctor_collect_only") || !hasDiagnostic(report.Diagnostics, "mutation_command_not_executed") {
		t.Fatalf("diagnostics = %#v, want doctor collect-only and no-exec diagnostics", report.Diagnostics)
	}
}

func TestCollectMutationSignalsConfigOverridesAndIgnoredModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "domain", "go.mod"), "module example/domain\n")
	writeTestFile(t, filepath.Join(dir, "domain", nativeMutationFile), `{"score":80,"total":10,"killed":8,"survived":2}`)
	writeTestFile(t, filepath.Join(dir, "fixtures", nativeMutationFile), `{"total":1,"killed":1}`)
	threshold := 75.0

	report, err := CollectMutationSignals(dir, MutationOptions{
		Enabled:  true,
		Discover: true,
		Modules: []MutationModuleConfig{
			{Root: "domain", Tool: mutationToolNative, NativeReportPaths: []string{nativeMutationFile}, Threshold: &threshold},
			{Name: "fixtures", Root: "fixtures", MutationPolicy: MutationPolicyIgnored, IgnoreReason: "generated fixtures"},
		},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectMutationSignals() error = %v", err)
	}
	if report.Summary.Modules != 2 || report.Summary.ConfiguredModules != 2 || report.Summary.IgnoredModules != 1 {
		t.Fatalf("summary = %+v, want configured domain plus ignored fixtures", report.Summary)
	}
	byRoot := modulesByRoot(report.Modules)
	if byRoot["domain"].Mutation == nil || byRoot["domain"].Mutation.Killed != 8 {
		t.Fatalf("domain mutation = %+v, want parsed native report", byRoot["domain"].Mutation)
	}
	if !hasDiagnostic(report.Diagnostics, "mutation_module_duplicate") || !hasDiagnostic(report.Diagnostics, "mutation_module_ignored") {
		t.Fatalf("diagnostics = %#v, want configured override and ignored module diagnostics", report.Diagnostics)
	}
}

func TestCollectMutationSignalsParsesStrykerAndMapsSurvivedMutants(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "src", "app.ts"), "export const app = 1;\n")
	reportJSON := `{"files":{"/ci/work/src/app.ts":{"mutants":[{"id":"1","status":"Killed","mutatorName":"StringLiteral","location":{"start":{"line":1},"end":{"line":1}}},{"id":"2","status":"Survived","mutatorName":"ArithmeticOperator","replacement":"-","description":"changed + to -","location":{"start":{"line":1},"end":{"line":1}}},{"id":"2","status":"Survived","mutatorName":"ArithmeticOperator","replacement":"-","description":"duplicate","location":{"start":{"line":1},"end":{"line":1}}}]}}}`
	writeTestFile(t, filepath.Join(dir, "reports", "mutation", "mutation.json"), reportJSON)

	report, err := CollectMutationSignals(dir, MutationOptions{
		Enabled:      true,
		Discover:     false,
		PathMappings: []TestPathMapping{{From: filepath.Clean("/ci/work"), To: "."}},
		Modules:      []MutationModuleConfig{{Root: ".", Tool: mutationToolStryker, ReportPaths: []string{"reports/mutation/mutation.json"}}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectMutationSignals() error = %v", err)
	}
	mutation := report.Modules[0].Mutation
	if mutation == nil || mutation.Total != 2 || mutation.Killed != 1 || mutation.Survived != 1 || len(mutation.SurvivedMutants) != 1 {
		t.Fatalf("mutation = %+v, want counted report with one deduped survived mutant", mutation)
	}
	if mutation.SurvivedMutants[0].File != "src/app.ts" {
		t.Fatalf("SurvivedMutants = %#v, want mapped src/app.ts", mutation.SurvivedMutants)
	}
	if !hasDiagnostic(report.Diagnostics, "stryker_mutation_report_loaded") || !hasDiagnostic(report.Diagnostics, "mutation_duplicate_mutant") {
		t.Fatalf("diagnostics = %#v, want Stryker and duplicate diagnostics", report.Diagnostics)
	}
}

func TestCollectMutationSignalsNoCoverageIsNotKilled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "src", "app.ts"), "export const app = 1;\n")
	reportJSON := `{"files":{"src/app.ts":{"mutants":[{"id":"1","status":"NoCoverage","mutatorName":"StringLiteral","location":{"start":{"line":1},"end":{"line":1}}}]}}}`
	writeTestFile(t, filepath.Join(dir, "mutation.json"), reportJSON)

	report, err := CollectMutationSignals(dir, MutationOptions{Enabled: true, Discover: false, Modules: []MutationModuleConfig{{Root: ".", Tool: mutationToolStryker, ReportPaths: []string{"mutation.json"}}}}, time.Now())
	if err != nil {
		t.Fatalf("CollectMutationSignals() error = %v", err)
	}
	mutation := report.Modules[0].Mutation
	if mutation == nil || mutation.Killed != 0 || mutation.Survived != 1 || mutation.NoCoverage != 1 || mutation.Score == nil || *mutation.Score != 0 {
		t.Fatalf("mutation = %+v, want no-coverage counted as not killed", mutation)
	}
}

func TestCollectMutationSignalsMarksStaleReportsPartial(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, nativeMutationFile)
	writeTestFile(t, path, `{"score":50,"total":2,"killed":1,"survived":1}`)
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	report, err := CollectMutationSignals(dir, MutationOptions{Enabled: true, Discover: false, MaxReportAge: time.Hour, Modules: []MutationModuleConfig{{Root: ".", Tool: mutationToolNative, NativeReportPaths: []string{nativeMutationFile}}}}, time.Now())
	if err != nil {
		t.Fatalf("CollectMutationSignals() error = %v", err)
	}
	if report.Modules[0].Mutation == nil || !report.Modules[0].Mutation.Stale {
		t.Fatalf("mutation = %+v, want stale mutation summary", report.Modules[0].Mutation)
	}
	if report.Health == nil || report.Health.PartialModules != 1 {
		t.Fatalf("health = %+v, want partial module due stale report", report.Health)
	}
	if !hasDiagnostic(report.Diagnostics, "mutation_report_stale") {
		t.Fatalf("diagnostics = %#v, want stale diagnostic", report.Diagnostics)
	}
}

func TestCollectMutationSignalsTimeoutStillCollectsReadableReport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, nativeMutationFile), `{"score":100,"total":1,"killed":1}`)
	command := "sleep 1"
	if runtime.GOOS == "windows" {
		command = "ping -n 2 127.0.0.1 > nul"
	}

	report, err := CollectMutationSignals(dir, MutationOptions{
		Enabled:    true,
		Discover:   false,
		Run:        true,
		MaxRuntime: time.Nanosecond,
		Modules:    []MutationModuleConfig{{Root: ".", Tool: mutationToolNative, Command: command, NativeReportPaths: []string{nativeMutationFile}}},
	}, time.Now())
	if err != nil {
		t.Fatalf("CollectMutationSignals() error = %v", err)
	}
	if report.Modules[0].MutationExecution == nil || report.Modules[0].MutationExecution.ExitCode != 124 {
		t.Fatalf("MutationExecution = %+v, want timeout status", report.Modules[0].MutationExecution)
	}
	if report.Summary.MutantsKilled != 1 {
		t.Fatalf("summary = %+v, want readable report collected after timeout", report.Summary)
	}
	if !hasDiagnostic(report.Diagnostics, "mutation_command_timeout") {
		t.Fatalf("diagnostics = %#v, want timeout diagnostic", report.Diagnostics)
	}
}

func TestEvaluateTestHealthUsesChangedCodeMutationWhenAvailable(t *testing.T) {
	t.Parallel()
	mutationScore := 90.0
	changedMutationScore := 50.0
	threshold := 75.0
	report := &TestSignalReport{Summary: TestSignalSummary{Enabled: true}, Modules: []TestModuleSignal{{
		Name:                     "domain",
		Root:                     "domain",
		ArchitectureRole:         "domain",
		MutationThreshold:        &threshold,
		ChangedMutationThreshold: &threshold,
		Suites:                   []TestSuiteSignal{{Name: "unit", Tests: 1, Passed: 1}},
		Coverage:                 &TestCoverageSummary{LinesToCover: 1, CoveredLines: 1, Coverage: &mutationScore},
		Mutation:                 &TestMutationSummary{Score: &mutationScore, ChangedCodeScore: &changedMutationScore, Total: 10, Killed: 9, ChangedTotal: 2, ChangedKilled: 1, ChangedSurvived: 1},
	}}}

	evaluateTestHealth(report)
	moduleHealth := report.Modules[0].Health
	if moduleHealth == nil || !containsReason(moduleHealth.Reasons, "changed-code mutation score") {
		t.Fatalf("health = %+v, want changed-code mutation reason", moduleHealth)
	}
	if report.Summary.ChangedMutationScore == nil || *report.Summary.ChangedMutationScore != 50.0 {
		t.Fatalf("summary = %+v, want changed mutation score", report.Summary)
	}
}

func TestEvaluateTestHealthMissingMutationIsPartialOnlyWithThreshold(t *testing.T) {
	t.Parallel()
	report := &TestSignalReport{Summary: TestSignalSummary{Enabled: true}, Modules: []TestModuleSignal{{Name: "web", Root: "web", ArchitectureRole: "web", Suites: []TestSuiteSignal{{Name: "unit", Tests: 1, Passed: 1}}}}}

	evaluateTestHealth(report)
	if containsReason(report.Modules[0].Health.Reasons, "mutation report unavailable") {
		t.Fatalf("health = %+v, missing mutation should not be failed by default for web role", report.Modules[0].Health)
	}
}

func TestCollectTestSignalsMonorepoHexagonalFixture(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.work"), "go 1.21\n\nuse (\n\t./domain\n\t./application\n\t./adapter/http\n)\n")
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"workspaces":["web/*"]}`)
	writeTestFile(t, filepath.Join(dir, "domain", "go.mod"), "module example.com/domain\n")
	writeTestFile(t, filepath.Join(dir, "domain", "model.go"), "package domain\n\nfunc Model() {}\n")
	writeTestFile(t, filepath.Join(dir, "domain", "coverage.out"), "mode: set\nmodel.go:1.1,1.20 1 1\n")
	writeTestFile(t, filepath.Join(dir, "application", "go.mod"), "module example.com/application\n")
	writeTestFile(t, filepath.Join(dir, "application", "usecase.go"), "package application\n\nfunc Run() {}\n")
	writeTestFile(t, filepath.Join(dir, "application", "junit.xml"), `<testsuite name="application" tests="1"><testcase classname="UseCase" name="runs"/></testsuite>`)
	writeTestFile(t, filepath.Join(dir, "adapter", "http", "go.mod"), "module example.com/adapter/http\n")
	writeTestFile(t, filepath.Join(dir, "adapter", "http", "handler.go"), "package http\n\nfunc Handle() {}\n")
	writeTestFile(t, filepath.Join(dir, "adapter", "http", "build", "test-results", "TEST-adapter.xml"), `<testsuite name="adapter-integration" tests="1"><testcase classname="HTTP" name="serves"/></testsuite>`)
	writeTestFile(t, filepath.Join(dir, "web", "ui", "package.json"), `{"devDependencies":{"typescript":"^5.0.0"}}`)
	writeTestFile(t, filepath.Join(dir, "web", "ui", "tsconfig.json"), `{}`)
	writeTestFile(t, filepath.Join(dir, "web", "ui", "src", "app.ts"), "export const app = 1;\n")
	writeTestFile(t, filepath.Join(dir, "web", "ui", "coverage", "lcov.info"), "TN:\nSF:src/app.ts\nDA:1,1\nend_of_record\n")

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: true, MaxDepth: 8}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	byRoot := modulesByRoot(report.Modules)
	assertModule(t, byRoot, "domain", "go", "domain")
	assertModule(t, byRoot, "application", "go", "application")
	assertModule(t, byRoot, "adapter/http", "go", "adapter")
	assertModule(t, byRoot, "web/ui", "typescript", "web")
	if report.Summary.Tests < 2 {
		t.Fatalf("Tests = %d, want parsed application and adapter suites", report.Summary.Tests)
	}
	if report.Summary.ModulesWithCoverage < 2 {
		t.Fatalf("ModulesWithCoverage = %d, want domain and web coverage", report.Summary.ModulesWithCoverage)
	}
	if report.Health == nil || report.Health.Modules < 4 {
		t.Fatalf("Health = %+v, want project health for the discovered fixture modules", report.Health)
	}
}

func TestCommandPolicyExplicitDeniesWithoutExplicitCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: false, Run: true, CommandPolicy: CommandPolicyExplicit, Modules: []TestModuleConfig{{Root: ".", Command: ""}}}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if len(report.Modules) != 1 {
		t.Fatalf("Modules = %d, want 1", len(report.Modules))
	}
	if report.Modules[0].Execution != nil {
		t.Fatalf("Execution = %+v, want nil when explicit policy denies empty command", report.Modules[0].Execution)
	}
}

func TestCommandPolicyConfiguredOnlyAllowsConfiguredModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/app\n")

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: true, Run: true, CommandPolicy: CommandPolicyConfiguredOnly}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if len(report.Modules) != 1 {
		t.Fatalf("Modules = %d, want 1", len(report.Modules))
	}
	if !hasDiagnostic(report.Diagnostics, "command_policy_denied") {
		t.Fatalf("diagnostics = %#v, want command_policy_denied for discovered module", report.Diagnostics)
	}
}

func TestIntegrationSuiteInferredFromFilePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	junit := `<testsuite name="unit" tests="1"><testcase classname="Adapters" name="connects"/></testsuite>`
	writeTestFile(t, filepath.Join(dir, "build", "integration-results", "junit.xml"), junit)

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: false, Modules: []TestModuleConfig{{Root: ".", TestReports: []string{"build/integration-results/junit.xml"}}}}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if len(report.Modules) != 1 || len(report.Modules[0].Suites) != 1 {
		t.Fatalf("modules = %+v, want suite with integration kind", report.Modules)
	}
	if report.Modules[0].Suites[0].Kind != SuiteKindIntegration {
		t.Fatalf("Suite kind = %q, want integration (inferred from path)", report.Modules[0].Suites[0].Kind)
	}
}

func TestSuiteKindFromPathContractDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	junit := `<testsuite name="test" tests="1"><testcase classname="Pact" name="verifies"/></testsuite>`
	writeTestFile(t, filepath.Join(dir, "contract", "reports", "junit.xml"), junit)

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: false, Modules: []TestModuleConfig{{Root: ".", SuiteKind: SuiteKindUnknown, TestReports: []string{"contract/reports/junit.xml"}}}}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if len(report.Modules) != 1 || len(report.Modules[0].Suites) != 1 {
		t.Fatalf("modules = %+v, want suite", report.Modules)
	}
	if report.Modules[0].Suites[0].Kind != SuiteKindContract {
		t.Fatalf("Suite kind = %q, want contract (inferred from path)", report.Modules[0].Suites[0].Kind)
	}
}

func TestSuiteKindFromE2EPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	junit := `<testsuite name="specs" tests="1"><testcase classname="Flow" name="completes"/></testsuite>`
	writeTestFile(t, filepath.Join(dir, "e2e", "results", "junit.xml"), junit)

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: false, Modules: []TestModuleConfig{{Root: ".", SuiteKind: SuiteKindUnknown, TestReports: []string{"e2e/results/junit.xml"}}}}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if len(report.Modules) != 1 || len(report.Modules[0].Suites) != 1 {
		t.Fatalf("modules = %+v, want suite", report.Modules)
	}
	if report.Modules[0].Suites[0].Kind != SuiteKindE2E {
		t.Fatalf("Suite kind = %q, want e2e (inferred from path)", report.Modules[0].Suites[0].Kind)
	}
}

func TestSuiteKindNotAffectedByGenericPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	junit := `<testsuite name="unit" tests="1"><testcase classname="Calc" name="adds"/></testsuite>`
	writeTestFile(t, filepath.Join(dir, "build", "test-results", "junit.xml"), junit)

	report, err := CollectTestSignals(dir, TestOptions{Enabled: true, Discover: false, Modules: []TestModuleConfig{{Root: ".", TestReports: []string{"build/test-results/junit.xml"}}}}, time.Now())
	if err != nil {
		t.Fatalf("CollectTestSignals() error = %v", err)
	}
	if len(report.Modules) != 1 || len(report.Modules[0].Suites) != 1 {
		t.Fatalf("modules = %+v, want suite", report.Modules)
	}
	if report.Modules[0].Suites[0].Kind != SuiteKindUnknown {
		t.Fatalf("Suite kind = %q, want unknown for generic path", report.Modules[0].Suites[0].Kind)
	}
}

func TestMutationCommandBuilderStrykerChangedOnly(t *testing.T) {
	t.Parallel()
	opts := MutationOptions{ChangedOnly: true}
	result := buildMutationCommand(mutationToolStryker, opts)

	if !strings.Contains(result.Command, "--mutate") {
		t.Fatalf("Command = %q, want --mutate flag", result.Command)
	}
	if result.Enforcement != mutationEnforcementBestEffort {
		t.Fatalf("Enforcement = %q, want best_effort", result.Enforcement)
	}
}

func TestMutationCommandBuilderStrykerWithoutChangedOnly(t *testing.T) {
	t.Parallel()
	opts := MutationOptions{}
	result := buildMutationCommand(mutationToolStryker, opts)

	if strings.Contains(result.Command, "--mutate") {
		t.Fatalf("Command = %q, want no --mutate flag", result.Command)
	}
	if result.Enforcement != mutationEnforcementAdvisory {
		t.Fatalf("Enforcement = %q, want advisory", result.Enforcement)
	}
}

func TestMutationCommandBuilderStrykerMaxMutants(t *testing.T) {
	t.Parallel()
	opts := MutationOptions{MaxMutants: 100}
	result := buildMutationCommand(mutationToolStryker, opts)

	if !strings.Contains(result.Command, "--concurrency=") {
		t.Fatalf("Command = %q, want concurrency limit", result.Command)
	}
}

func TestMutationCommandBuilderInfectionChangedOnly(t *testing.T) {
	t.Parallel()
	opts := MutationOptions{ChangedOnly: true}
	result := buildMutationCommand(mutationToolInfection, opts)

	if !strings.Contains(result.Command, "--git-diff-filter") {
		t.Fatalf("Command = %q, want --git-diff-filter flag", result.Command)
	}
	if result.Enforcement != mutationEnforcementBestEffort {
		t.Fatalf("Enforcement = %q, want best_effort", result.Enforcement)
	}
}

func TestMutationCommandBuilderCosmicRayDefault(t *testing.T) {
	t.Parallel()
	opts := MutationOptions{}
	result := buildMutationCommand(mutationToolCosmic, opts)

	if !strings.Contains(result.Command, ".cosmic-ray.toml") {
		t.Fatalf("Command = %q, want .cosmic-ray.toml config reference", result.Command)
	}
}

func TestMutationCommandBuilderStrykerEnforcedWithChangedFiles(t *testing.T) {
	t.Parallel()
	opts := MutationOptions{ChangedOnly: true, ChangedFiles: []string{"src/app.ts", "src/util.ts"}}
	result := buildMutationCommand(mutationToolStryker, opts)

	if !strings.Contains(result.Command, "--mutate=src/app.ts,src/util.ts") {
		t.Fatalf("Command = %q, want --mutate with file list", result.Command)
	}
	if result.Enforcement != mutationEnforcementEnforced {
		t.Fatalf("Enforcement = %q, want enforced", result.Enforcement)
	}
}

func TestMutationCommandBuilderInfectionEnforcedWithChangedFiles(t *testing.T) {
	t.Parallel()
	opts := MutationOptions{ChangedOnly: true, ChangedFiles: []string{"src/App.php"}}
	result := buildMutationCommand(mutationToolInfection, opts)

	if !strings.Contains(result.Command, "--filter=src/App.php") {
		t.Fatalf("Command = %q, want --filter flag", result.Command)
	}
	if !strings.Contains(result.Command, "--git-diff-filter") {
		t.Fatalf("Command = %q, want --git-diff-filter flag", result.Command)
	}
	if result.Enforcement != mutationEnforcementEnforced {
		t.Fatalf("Enforcement = %q, want enforced", result.Enforcement)
	}
}

func TestResolveChangedFilesWithGitDiff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustGit(t, dir, "init")
	mustGit(t, dir, "config", "user.email", "test@example.com")
	mustGit(t, dir, "config", "user.name", "test")
	writeTestFile(t, filepath.Join(dir, "main.go"), "package main\n")
	mustGit(t, dir, "add", ".")
	mustGit(t, dir, "commit", "-m", "initial")
	writeTestFile(t, filepath.Join(dir, "changed.go"), "package main\nfunc Changed() {}\n")
	mustGit(t, dir, "add", ".")
	mustGit(t, dir, "commit", "-m", "changed")

	files, err := ResolveChangedFiles(dir, "")
	if err != nil {
		t.Fatalf("ResolveChangedFiles() error = %v", err)
	}
	if len(files) == 0 {
		t.Fatal("want at least one changed file")
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %s", args, string(out))
	}
}

func TestWeightedHealthWeightsDomainHigherThanLibrary(t *testing.T) {
	t.Parallel()
	goodCoverage := 90.0
	mutationScore := 80.0
	report := &TestSignalReport{Summary: TestSignalSummary{Enabled: true}, Modules: []TestModuleSignal{
		{Name: "domain", Root: "domain", ArchitectureRole: "domain", Suites: []TestSuiteSignal{{Name: "unit", Tests: 1, Passed: 1}}, Coverage: &TestCoverageSummary{LinesToCover: 1, CoveredLines: 1, Coverage: &goodCoverage}, Mutation: &TestMutationSummary{Score: &mutationScore, Total: 1, Killed: 1}},
		{Name: "lib", Root: "lib", ArchitectureRole: "library", Suites: []TestSuiteSignal{{Name: "unit", Tests: 1, Passed: 1}}, Coverage: &TestCoverageSummary{LinesToCover: 1, CoveredLines: 1, Coverage: &goodCoverage}},
	}}
	evaluateTestHealth(report)

	if report.Modules[0].Health == nil || report.Modules[0].Health.Status != "healthy" {
		t.Fatalf("domain health = %+v, want healthy", report.Modules[0].Health)
	}
	if report.Modules[1].Health == nil || report.Modules[1].Health.Status != "healthy" {
		t.Fatalf("library health = %+v, want healthy", report.Modules[1].Health)
	}
	if report.Health == nil {
		t.Fatal("project health is nil")
	}
}

func TestWeightedHealthAtRiskDomainPullsDownWeightedScore(t *testing.T) {
	t.Parallel()
	badCoverage := 50.0
	goodCoverage := 90.0
	report := &TestSignalReport{Summary: TestSignalSummary{Enabled: true}, Modules: []TestModuleSignal{
		{Name: "domain", Root: "domain", ArchitectureRole: "domain", Suites: []TestSuiteSignal{{Name: "unit", Tests: 1, Passed: 1}}, Coverage: &TestCoverageSummary{LinesToCover: 1, CoveredLines: 1, Coverage: &badCoverage}},
		{Name: "lib", Root: "lib", ArchitectureRole: "library", Suites: []TestSuiteSignal{{Name: "unit", Tests: 1, Passed: 1}}, Coverage: &TestCoverageSummary{LinesToCover: 1, CoveredLines: 1, Coverage: &goodCoverage}},
	}}
	evaluateTestHealth(report)

	if report.Health == nil || report.Health.Status != "at_risk" {
		t.Fatalf("project health = %+v, want at_risk when domain has bad coverage", report.Health)
	}
}

func TestMutationSummaryChangedOnlyEnforcementBestEffortForStryker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"devDependencies":{"@stryker-mutator/core":"^8.0.0"}}`)
	writeTestFile(t, filepath.Join(dir, nativeMutationFile), `{"score":80,"total":1,"killed":1}`)

	report, err := CollectMutationSignals(dir, MutationOptions{Enabled: true, Discover: true, ChangedOnly: true, Modules: []MutationModuleConfig{{Root: ".", MutationPolicy: MutationPolicyIgnored, IgnoreReason: "override"}}}, time.Now())
	if err != nil {
		t.Fatalf("CollectMutationSignals() error = %v", err)
	}
	for _, module := range report.Modules {
		if module.Mutation != nil && module.Mutation.Tool == mutationToolStryker && module.Mutation.ChangedOnlyEnforcement != mutationEnforcementBestEffort {
			t.Fatalf("ChangedOnlyEnforcement = %q, want best_effort for Stryker with changed_only", module.Mutation.ChangedOnlyEnforcement)
		}
	}
}

func TestDiscoverMutationModulesUsesCommandBuilderForChangedOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"devDependencies":{"@stryker-mutator/core":"^8.0.0"}}`)

	modules, _, err := DiscoverMutationModules(dir, MutationOptions{ChangedOnly: true, MaxDepth: 4})
	if err != nil {
		t.Fatalf("DiscoverMutationModules() error = %v", err)
	}
	if len(modules) != 1 {
		t.Fatalf("Modules = %d, want 1", len(modules))
	}
	if !strings.Contains(modules[0].Command, "--mutate") {
		t.Fatalf("Command = %q, want --mutate flag when changed_only=true", modules[0].Command)
	}
	if modules[0].Mutation == nil || modules[0].Mutation.ChangedOnlyEnforcement != mutationEnforcementBestEffort {
		t.Fatalf("Mutation = %+v, want best_effort ChangedOnlyEnforcement", modules[0].Mutation)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func modulesByRoot(modules []TestModuleSignal) map[string]TestModuleSignal {
	out := make(map[string]TestModuleSignal, len(modules))
	for _, module := range modules {
		out[module.Root] = module
	}
	return out
}

func assertModule(t *testing.T, modules map[string]TestModuleSignal, root, language, role string) {
	t.Helper()
	module, ok := modules[root]
	if !ok {
		t.Fatalf("module %q not discovered; got %#v", root, modules)
	}
	if module.Language != language {
		t.Fatalf("module %q language = %q, want %q", root, module.Language, language)
	}
	if module.ArchitectureRole != role {
		t.Fatalf("module %q role = %q, want %q", root, module.ArchitectureRole, role)
	}
}

func hasDiagnostic(diagnostics []TestSignalDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func containsReason(reasons []string, fragment string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, fragment) {
			return true
		}
	}
	return false
}
