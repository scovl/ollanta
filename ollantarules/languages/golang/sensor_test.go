package golang_test

import (
	"strings"
	"testing"

	"github.com/scovl/ollanta/ollantarules/defaults"
	gosensor "github.com/scovl/ollanta/ollantarules/languages/golang"
)

func TestGoSensor_Language(t *testing.T) {
	s := gosensor.NewGoSensor(defaults.NewRegistry())
	if s.Language() != "go" {
		t.Errorf("Language: got %q", s.Language())
	}
}

func TestGoSensor_AnalyzeLargeFunction(t *testing.T) {
	src := `package p
func Big() {
` + strings.Repeat("\t_ = 1\n", 42) + `}`
	s := gosensor.NewGoSensor(defaults.NewRegistry())
	issues, err := s.Analyze("test.go", []byte(src), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) == 0 {
		t.Error("expected issues for large function")
	}
}

func TestGoSensor_AnalyzeClean(t *testing.T) {
	src := `package p
func Add(a, b int) int { return a + b }
`
	s := gosensor.NewGoSensor(defaults.NewRegistry())
	issues, err := s.Analyze("clean.go", []byte(src), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = issues // may or may not have issues; main check is no error
}

func TestGoSensor_AnalyzeParseError(t *testing.T) {
	src := `package p
func Broken( {`
	s := gosensor.NewGoSensor(defaults.NewRegistry())
	_, err := s.Analyze("broken.go", []byte(src), nil)
	if err == nil {
		t.Error("expected error for unparseable source")
	}
}

func TestGoSensor_ActiveRulesFilter(t *testing.T) {
	src := `package p
func Big() {
` + strings.Repeat("\t_ = 1\n", 42) + `}`
	s := gosensor.NewGoSensor(defaults.NewRegistry())
	// Only allow naming-conventions — no-large-functions should be skipped
	activeRules := map[string]bool{"go:naming-conventions": true}
	issues, err := s.Analyze("test.go", []byte(src), activeRules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "go:no-large-functions" {
			t.Error("no-large-functions should have been filtered out")
		}
	}
}

func TestGoSensor_IssueFields(t *testing.T) {
	src := `package p
func Big() {
` + strings.Repeat("\t_ = 1\n", 42) + `}`
	s := gosensor.NewGoSensor(defaults.NewRegistry())
	issues, err := s.Analyze("src/test.go", []byte(src), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "" {
			t.Error("issue RuleKey should not be empty")
		}
		if iss.ComponentPath == "" {
			t.Error("issue ComponentPath should not be empty")
		}
	}
}
