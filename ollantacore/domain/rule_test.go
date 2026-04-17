package domain_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantacore/domain"
)

func TestRule_WithoutThreshold(t *testing.T) {
	r := domain.Rule{
		Key:             "go:naming-conventions",
		Name:            "Naming Conventions",
		Language:        "go",
		Type:            domain.TypeCodeSmell,
		DefaultSeverity: domain.SeverityMinor,
	}
	if r.Threshold != nil {
		t.Error("expected nil Threshold")
	}
	if r.Language != "go" {
		t.Errorf("Language: got %q", r.Language)
	}
}

func TestRule_WithThreshold(t *testing.T) {
	r := domain.Rule{
		Key:      "go:no-large-functions",
		Language: "go",
		Threshold: &domain.Threshold{
			MetricKey: "lines",
			Operator:  "gt",
			Value:     100,
			Entity:    "method",
		},
	}
	if r.Threshold == nil {
		t.Fatal("expected non-nil Threshold")
	}
	if r.Threshold.MetricKey != "lines" {
		t.Errorf("MetricKey: got %q", r.Threshold.MetricKey)
	}
	if r.Threshold.Value != 100 {
		t.Errorf("Value: got %f", r.Threshold.Value)
	}
	if r.Threshold.Entity != "method" {
		t.Errorf("Entity: got %q", r.Threshold.Entity)
	}
}

func TestRule_CrossLanguage(t *testing.T) {
	r := domain.Rule{
		Key:      "any:no-fixme-comments",
		Language: "*",
		Type:     domain.TypeCodeSmell,
	}
	if r.Language != "*" {
		t.Errorf("expected \"*\" Language, got %q", r.Language)
	}
}

func TestRule_ParamsSchema(t *testing.T) {
	r := domain.Rule{
		Key: "go:no-large-functions",
		ParamsSchema: map[string]domain.ParamDef{
			"max_lines": {
				Key:          "max_lines",
				Description:  "Maximum number of lines in a function",
				DefaultValue: "100",
				Type:         "int",
			},
		},
	}
	param, ok := r.ParamsSchema["max_lines"]
	if !ok {
		t.Fatal("expected max_lines param")
	}
	if param.Type != "int" {
		t.Errorf("Type: got %q", param.Type)
	}
	if param.DefaultValue != "100" {
		t.Errorf("DefaultValue: got %q", param.DefaultValue)
	}
}
