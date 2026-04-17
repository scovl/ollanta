package treesitter_test

import (
	"strings"
	"testing"

	parlanguages "github.com/scovl/ollanta/ollantaparser/languages"
	"github.com/scovl/ollanta/ollantarules/defaults"
	"github.com/scovl/ollanta/ollantarules/languages/treesitter"
)

func defaultSensor() *treesitter.TreeSitterSensor {
	return treesitter.NewTreeSitterSensor(defaults.NewRegistry(), parlanguages.DefaultRegistry())
}

func TestTreeSitterSensor_JS_LargeFunction(t *testing.T) {
	src := []byte("function bigFunc() {\n" + strings.Repeat("  const x = 1;\n", 42) + "}\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) == 0 {
		t.Error("expected issue for large JS function")
	}
}

func TestTreeSitterSensor_JS_SmallFunction(t *testing.T) {
	src := []byte("function small() { return 1; }\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:no-large-functions" {
			t.Error("small function should not be flagged")
		}
	}
}

func TestTreeSitterSensor_Python_LargeFunction(t *testing.T) {
	src := []byte("def big_func():\n" + strings.Repeat("    x = 1\n", 42) + "\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.py", src, "python", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) == 0 {
		t.Error("expected issue for large Python function")
	}
}

func TestTreeSitterSensor_UnknownLanguage(t *testing.T) {
	s := defaultSensor()
	_, err := s.Analyze("test.rb", []byte("puts 'hello'"), "ruby", nil)
	if err == nil {
		t.Error("expected error for unsupported language")
	}
}

func TestTreeSitterSensor_IssueHasPositions(t *testing.T) {
	src := []byte("function bigFunc() {\n" + strings.Repeat("  const x = 1;\n", 42) + "}\n")
	s := defaultSensor()
	issues, err := s.Analyze("test.js", src, "javascript", nil)
	if err != nil || len(issues) == 0 {
		t.Fatalf("setup failed: err=%v issues=%d", err, len(issues))
	}
	if issues[0].Line <= 0 {
		t.Error("issue should have positive start line")
	}
	if issues[0].EndLine <= 0 {
		t.Error("issue should have positive end line")
	}
}

func TestTreeSitterSensor_CustomMaxLines(t *testing.T) {
	src := []byte("function medium() {\n" + strings.Repeat("  const x = 1;\n", 10) + "}\n")
	s := defaultSensor()
	// Filter to only js rule; cannot easily pass params via sensor directly,
	// but can verify the sensor honors active rules
	activeRules := map[string]bool{"js:no-large-functions": true}
	// 10 lines < default 40 — no issue
	issues, err := s.Analyze("test.js", src, "javascript", activeRules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, iss := range issues {
		if iss.RuleKey == "js:no-large-functions" {
			t.Errorf("10-line function should not be flagged with default threshold: %v", iss.Message)
		}
	}
}
