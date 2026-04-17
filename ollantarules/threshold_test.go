package ollantarules_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

func makeComponent(name string, compType string, metrics map[string]float64) *domain.Component {
	return &domain.Component{
		Name:    name,
		Type:    domain.ComponentType(compType),
		Path:    "pkg/" + name,
		Metrics: metrics,
	}
}

func TestCheckThresholds_FunctionLinesTooLong(t *testing.T) {
	c := makeComponent("bigFn", "function", map[string]float64{"lines": 150})
	issues := ollantarules.CheckThresholds(c, ollantarules.DefaultThresholds())
	if len(issues) == 0 {
		t.Error("expected threshold violation for lines > 100")
	}
}

func TestCheckThresholds_WithinLimit(t *testing.T) {
	c := makeComponent("smallFn", "function", map[string]float64{"lines": 50})
	issues := ollantarules.CheckThresholds(c, ollantarules.DefaultThresholds())
	if len(issues) != 0 {
		t.Errorf("expected no violations, got %d", len(issues))
	}
}

func TestCheckThresholds_WrongEntity(_ *testing.T) {
	// complexity threshold is for "function" entity — a "file" component should not trigger it
	c := makeComponent("main.go", "file", map[string]float64{"complexity": 100})
	issues := ollantarules.CheckThresholds(c, ollantarules.DefaultThresholds())
	// Only file lines threshold should fire if exceeded
	_ = issues // just verify no panic
}

func TestCheckThresholds_MultipleViolations(t *testing.T) {
	c := makeComponent("bigFn", "function", map[string]float64{
		"lines":         200, // > 100
		"complexity":    15,  // > 10
		"nesting_level": 6,   // > 4
	})
	issues := ollantarules.CheckThresholds(c, ollantarules.DefaultThresholds())
	if len(issues) != 3 {
		t.Errorf("expected 3 violations, got %d", len(issues))
	}
}

func TestCheckThresholds_MissingMetric(t *testing.T) {
	// Component has no metrics at all — no violations
	c := makeComponent("emptyFn", "function", map[string]float64{})
	issues := ollantarules.CheckThresholds(c, ollantarules.DefaultThresholds())
	if len(issues) != 0 {
		t.Errorf("expected no violations for missing metrics, got %d", len(issues))
	}
}

func TestDefaultThresholds_Count(t *testing.T) {
	thresholds := ollantarules.DefaultThresholds()
	if len(thresholds) != 5 {
		t.Errorf("expected 5 default thresholds, got %d", len(thresholds))
	}
}

func TestCheckThresholds_MessageTemplate(t *testing.T) {
	c := makeComponent("heavyFn", "function", map[string]float64{"lines": 150})
	issues := ollantarules.CheckThresholds(c, ollantarules.DefaultThresholds())
	if len(issues) == 0 {
		t.Fatal("expected an issue")
	}
	if issues[0].Message == "" {
		t.Error("issue message should not be empty")
	}
}
