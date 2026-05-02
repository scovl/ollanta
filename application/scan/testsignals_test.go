package scan

import (
	"os"
	"path/filepath"
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
