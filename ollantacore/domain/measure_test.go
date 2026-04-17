package domain_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantacore/domain"
)

func TestMeasure_NumericValue(t *testing.T) {
	m := domain.Measure{
		MetricKey:     "ncloc",
		ComponentPath: "pkg/parser.go",
		Value:         312,
	}
	if m.Value != 312 {
		t.Errorf("Value: got %f", m.Value)
	}
	if m.TextValue != "" {
		t.Errorf("expected empty TextValue, got %q", m.TextValue)
	}
}

func TestMeasure_TextValue(t *testing.T) {
	m := domain.Measure{
		MetricKey:     "profile",
		ComponentPath: "project",
		TextValue:     "Ollanta Way",
	}
	if m.TextValue != "Ollanta Way" {
		t.Errorf("TextValue: got %q", m.TextValue)
	}
	if m.Value != 0 {
		t.Errorf("expected zero Value when only TextValue set, got %f", m.Value)
	}
}
