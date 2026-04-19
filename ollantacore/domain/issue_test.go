package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/scovl/ollanta/ollantacore/domain"
)

func TestNewIssue_RequiredFields(t *testing.T) {
	issue := domain.NewIssue("go:no-large-functions", "pkg/server.go", 42)
	if issue.RuleKey != "go:no-large-functions" {
		t.Errorf("RuleKey: got %q", issue.RuleKey)
	}
	if issue.ComponentPath != "pkg/server.go" {
		t.Errorf("ComponentPath: got %q", issue.ComponentPath)
	}
	if issue.Line != 42 {
		t.Errorf("Line: got %d", issue.Line)
	}
	if issue.Status != domain.StatusOpen {
		t.Errorf("Status: expected StatusOpen, got %q", issue.Status)
	}
}

func TestNewIssue_AllFields(t *testing.T) {
	issue := domain.NewIssue("js:no-large-functions", "src/app.js", 10)
	issue.Message = "function too large"
	issue.Severity = domain.SeverityMajor
	issue.Type = domain.TypeCodeSmell
	issue.Column = 1
	issue.EndLine = 80
	issue.EndColumn = 1
	issue.EffortMinutes = 30
	issue.Tags = []string{"size", "maintainability"}

	if issue.Message != "function too large" {
		t.Errorf("Message: got %q", issue.Message)
	}
	if issue.Severity != domain.SeverityMajor {
		t.Errorf("Severity: got %q", issue.Severity)
	}
	if issue.Type != domain.TypeCodeSmell {
		t.Errorf("Type: got %q", issue.Type)
	}
	if len(issue.Tags) != 2 {
		t.Errorf("Tags: expected 2, got %d", len(issue.Tags))
	}
}

func TestComputeLineHash_NormalLine(t *testing.T) {
	content := "line one\nline two\nline three"
	h := domain.ComputeLineHash(content, 2)
	if h == "" {
		t.Fatal("expected non-empty hash")
	}
	// Deterministic
	if domain.ComputeLineHash(content, 2) != h {
		t.Error("hash not deterministic")
	}
}

func TestComputeLineHash_DifferentLines(t *testing.T) {
	content := "line one\nline two\nline three"
	h1 := domain.ComputeLineHash(content, 1)
	h2 := domain.ComputeLineHash(content, 2)
	if h1 == h2 {
		t.Error("different lines should produce different hashes")
	}
}

func TestComputeLineHash_EmptyContent(t *testing.T) {
	if domain.ComputeLineHash("", 1) != "" {
		t.Error("expected empty hash for empty content")
	}
}

func TestComputeLineHash_OutOfRange(t *testing.T) {
	if domain.ComputeLineHash("only one line", 5) != "" {
		t.Error("expected empty hash for out-of-range line")
	}
}

func TestComputeLineHash_ZeroLine(t *testing.T) {
	if domain.ComputeLineHash("content", 0) != "" {
		t.Error("expected empty hash for line 0")
	}
}

func TestComputeLineHash_WhitespaceTrimmed(t *testing.T) {
	// Same logical content with different surrounding whitespace → same hash
	h1 := domain.ComputeLineHash("  func foo() {  ", 1)
	h2 := domain.ComputeLineHash("func foo() {", 1)
	if h1 != h2 {
		t.Error("trim: hashes should match after whitespace normalization")
	}
}

func TestIssue_JSONRoundtrip(t *testing.T) {
	issue := domain.NewIssue("go:no-naked-returns", "main.go", 15)
	issue.Message = "avoid naked returns"
	issue.Severity = domain.SeverityMinor
	issue.Type = domain.TypeCodeSmell

	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var restored domain.Issue
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if restored.RuleKey != issue.RuleKey {
		t.Errorf("RuleKey mismatch: got %q", restored.RuleKey)
	}
	if restored.Line != issue.Line {
		t.Errorf("Line mismatch: got %d", restored.Line)
	}
	if restored.Severity != issue.Severity {
		t.Errorf("Severity mismatch: got %q", restored.Severity)
	}
}

func TestNewIssue_EngineIDDefault(t *testing.T) {
	issue := domain.NewIssue("go:test", "main.go", 1)
	if issue.EngineID != "ollanta" {
		t.Errorf("EngineID: expected \"ollanta\", got %q", issue.EngineID)
	}
}

func TestNewIssue_SecondaryLocationsEmpty(t *testing.T) {
	issue := domain.NewIssue("go:test", "main.go", 1)
	if issue.SecondaryLocations == nil {
		t.Fatal("SecondaryLocations should not be nil")
	}
	if len(issue.SecondaryLocations) != 0 {
		t.Errorf("SecondaryLocations: expected empty, got %d", len(issue.SecondaryLocations))
	}
}

func TestSecondaryLocation_JSONRoundtrip(t *testing.T) {
	locs := []domain.SecondaryLocation{
		{FilePath: "a.go", Message: "related", StartLine: 10, StartColumn: 5, EndLine: 10, EndColumn: 20},
		{FilePath: "b.go", Message: "also related", StartLine: 20},
	}
	data, err := json.Marshal(locs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored []domain.SecondaryLocation
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(restored) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(restored))
	}
	if restored[0].FilePath != "a.go" {
		t.Errorf("FilePath: got %q", restored[0].FilePath)
	}
	if restored[0].EndColumn != 20 {
		t.Errorf("EndColumn: got %d", restored[0].EndColumn)
	}
}

func TestIssue_EngineID_JSONRoundtrip(t *testing.T) {
	issue := domain.NewIssue("go:test", "main.go", 1)
	issue.EngineID = "external-tool"
	issue.SecondaryLocations = []domain.SecondaryLocation{
		{FilePath: "main.go", Message: "see also", StartLine: 5},
	}
	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored domain.Issue
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if restored.EngineID != "external-tool" {
		t.Errorf("EngineID: got %q", restored.EngineID)
	}
	if len(restored.SecondaryLocations) != 1 {
		t.Fatalf("SecondaryLocations: expected 1, got %d", len(restored.SecondaryLocations))
	}
	if restored.SecondaryLocations[0].StartLine != 5 {
		t.Errorf("StartLine: got %d", restored.SecondaryLocations[0].StartLine)
	}
}
