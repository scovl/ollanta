package report_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantascanner/report"
)

func TestSaveSARIF_Schema(t *testing.T) {
	dir := t.TempDir()
	r := report.Build("p", nil, nil, 0)
	path, err := r.SaveSARIF(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var sarif map[string]interface{}
	if err := json.Unmarshal(data, &sarif); err != nil {
		t.Fatalf("invalid SARIF JSON: %v", err)
	}
	if sarif["$schema"] == nil {
		t.Error("SARIF missing $schema")
	}
}

func TestSaveSARIF_Version(t *testing.T) {
	dir := t.TempDir()
	r := report.Build("p", nil, nil, 0)
	path, _ := r.SaveSARIF(dir)
	data, _ := os.ReadFile(path)
	var sarif map[string]interface{}
	if err := json.Unmarshal(data, &sarif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sarif["version"] != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %v", sarif["version"])
	}
}

func TestSaveSARIF_IssueMapping(t *testing.T) {
	dir := t.TempDir()
	iss := []*domain.Issue{{
		RuleKey:       "go:no-large-functions",
		ComponentPath: "main.go",
		Line:          5,
		EndLine:       50,
		Message:       "function too long",
		Severity:      domain.SeverityMajor,
		Status:        domain.StatusOpen,
	}}
	r := report.Build("p", nil, iss, time.Millisecond)
	path, err := r.SaveSARIF(dir)
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	var sarif map[string]interface{}
	if err := json.Unmarshal(data, &sarif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	runs := sarif["runs"].([]interface{})
	run := runs[0].(map[string]interface{})
	results := run["results"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	res := results[0].(map[string]interface{})
	if res["ruleId"] != "go:no-large-functions" {
		t.Errorf("unexpected ruleId: %v", res["ruleId"])
	}
}

func TestSaveSARIF_Path(t *testing.T) {
	dir := t.TempDir()
	r := report.Build("p", nil, nil, 0)
	path, err := r.SaveSARIF(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "report.sarif" {
		t.Errorf("expected report.sarif, got %s", filepath.Base(path))
	}
}

func TestSaveSARIF_RuleDedup(t *testing.T) {
	dir := t.TempDir()
	iss := []*domain.Issue{
		{RuleKey: "go:rule-a", Severity: domain.SeverityMinor, Status: domain.StatusOpen},
		{RuleKey: "go:rule-a", Severity: domain.SeverityMinor, Status: domain.StatusOpen},
		{RuleKey: "go:rule-b", Severity: domain.SeverityMajor, Status: domain.StatusOpen},
	}
	r := report.Build("p", nil, iss, 0)
	path, _ := r.SaveSARIF(dir)
	data, _ := os.ReadFile(path)
	var sarif map[string]interface{}
	if err := json.Unmarshal(data, &sarif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	runs := sarif["runs"].([]interface{})
	run := runs[0].(map[string]interface{})
	tool := run["tool"].(map[string]interface{})
	driver := tool["driver"].(map[string]interface{})
	rules := driver["rules"].([]interface{})
	if len(rules) != 2 {
		t.Errorf("expected 2 deduplicated rules, got %d", len(rules))
	}
}
