package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantascanner/discovery"
	"github.com/scovl/ollanta/ollantascanner/report"
)

func sampleFiles() []discovery.DiscoveredFile {
	return []discovery.DiscoveredFile{
		{Path: "a.go", Language: "go"},
		{Path: "b.js", Language: "javascript"},
	}
}

func sampleIssue() *domain.Issue {
	return &domain.Issue{
		RuleKey:       "go:no-large-functions",
		ComponentPath: "a.go",
		Line:          10,
		EndLine:       55,
		Message:       "function too long",
		Severity:      domain.SeverityMajor,
		Type:          domain.TypeCodeSmell,
		Status:        domain.StatusOpen,
	}
}

func TestBuild_Metadata(t *testing.T) {
	r := report.Build("myproject", sampleFiles(), nil, 123*time.Millisecond)
	if r.Metadata.ProjectKey != "myproject" {
		t.Errorf("ProjectKey: got %q", r.Metadata.ProjectKey)
	}
	if r.Metadata.Version == "" {
		t.Error("Version should not be empty")
	}
	if r.Metadata.AnalysisDate == "" {
		t.Error("AnalysisDate should not be empty")
	}
	if r.Metadata.ElapsedMs < 100 {
		t.Errorf("ElapsedMs too low: %d", r.Metadata.ElapsedMs)
	}
}

func TestBuild_Measures_FileCount(t *testing.T) {
	files := sampleFiles()
	r := report.Build("p", files, nil, 0)
	if r.Measures.Files != len(files) {
		t.Errorf("Files: got %d, want %d", r.Measures.Files, len(files))
	}
	if r.Measures.ByLang["go"] != 1 {
		t.Errorf("ByLang[go]: got %d, want 1", r.Measures.ByLang["go"])
	}
	if r.Measures.ByLang["javascript"] != 1 {
		t.Errorf("ByLang[javascript]: got %d, want 1", r.Measures.ByLang["javascript"])
	}
}

func TestBuild_Issues(t *testing.T) {
	iss := []*domain.Issue{sampleIssue()}
	r := report.Build("p", nil, iss, 0)
	if len(r.Issues) != 1 {
		t.Errorf("Issues: got %d, want 1", len(r.Issues))
	}
}

func TestBuild_SetsEngineIDAndSecondaryLocations(t *testing.T) {
	iss := []*domain.Issue{sampleIssue()}
	r := report.Build("p", nil, iss, 0)
	if r.Issues[0].EngineID != "ollanta" {
		t.Errorf("EngineID: expected \"ollanta\", got %q", r.Issues[0].EngineID)
	}
	if r.Issues[0].SecondaryLocations == nil {
		t.Fatal("SecondaryLocations should not be nil")
	}
	if len(r.Issues[0].SecondaryLocations) != 0 {
		t.Errorf("SecondaryLocations: expected empty, got %d", len(r.Issues[0].SecondaryLocations))
	}
}

func TestBuild_PreservesExistingEngineID(t *testing.T) {
	iss := sampleIssue()
	iss.EngineID = "semgrep"
	r := report.Build("p", nil, []*domain.Issue{iss}, 0)
	if r.Issues[0].EngineID != "semgrep" {
		t.Errorf("EngineID: expected \"semgrep\", got %q", r.Issues[0].EngineID)
	}
}

func TestSaveJSON_Valid(t *testing.T) {
	dir := t.TempDir()
	iss := []*domain.Issue{sampleIssue()}
	r := report.Build("p", sampleFiles(), iss, 10*time.Millisecond)

	path, err := r.SaveJSON(dir)
	if err != nil {
		t.Fatalf("SaveJSON error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("report.json not found: %v", err)
	}

	data, _ := os.ReadFile(path)
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out["metadata"] == nil {
		t.Error("JSON missing 'metadata'")
	}
	if out["issues"] == nil {
		t.Error("JSON missing 'issues'")
	}
}

func TestSaveJSON_Path(t *testing.T) {
	dir := t.TempDir()
	r := report.Build("p", nil, nil, 0)
	path, err := r.SaveJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "report.json" {
		t.Errorf("expected report.json, got %s", filepath.Base(path))
	}
}
