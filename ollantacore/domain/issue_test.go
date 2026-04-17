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
