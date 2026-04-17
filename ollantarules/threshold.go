package ollantarules

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
)

// ThresholdDef defines a single threshold condition (MetricHunter pattern from OSA).
// When a component's metric exceeds the threshold, an issue is generated automatically.
type ThresholdDef struct {
	MetricKey string
	Relation  string // "gt", "lt", "gte", "lte"
	Value     float64
	Entity    string // "function", "file", "package", "project"
	Severity  domain.Severity
	// Message is a format string using %s for name, first %g for actual value, second %g for threshold.
	Message string
}

// DefaultThresholds returns the built-in threshold definitions (inspired by OSA MetricHunter).
func DefaultThresholds() []ThresholdDef {
	return []ThresholdDef{
		{
			MetricKey: "lines",
			Relation:  "gt",
			Value:     100,
			Entity:    "function",
			Severity:  domain.SeverityMajor,
			Message:   "Function '%s' has %g lines (threshold: %g)",
		},
		{
			MetricKey: "complexity",
			Relation:  "gt",
			Value:     10,
			Entity:    "function",
			Severity:  domain.SeverityMajor,
			Message:   "Function '%s' has cyclomatic complexity %g (threshold: %g)",
		},
		{
			MetricKey: "nesting_level",
			Relation:  "gt",
			Value:     4,
			Entity:    "function",
			Severity:  domain.SeverityMinor,
			Message:   "Function '%s' has nesting level %g (threshold: %g)",
		},
		{
			MetricKey: "lines",
			Relation:  "gt",
			Value:     500,
			Entity:    "file",
			Severity:  domain.SeverityMajor,
			Message:   "File '%s' has %g lines (threshold: %g)",
		},
		{
			MetricKey: "coupling",
			Relation:  "gt",
			Value:     10,
			Entity:    "package",
			Severity:  domain.SeverityMajor,
			Message:   "Package '%s' has coupling %g (threshold: %g)",
		},
	}
}

// CheckThresholds evaluates all thresholds against a component and returns issues
// for any that are violated. Only metrics present in component.Metrics are evaluated.
func CheckThresholds(component *domain.Component, thresholds []ThresholdDef) []*domain.Issue {
	var issues []*domain.Issue
	for _, t := range thresholds {
		if string(component.Type) != t.Entity {
			continue
		}
		val, ok := component.Metrics[t.MetricKey]
		if !ok {
			continue
		}
		if violated(val, t.Relation, t.Value) {
			msg := fmt.Sprintf(t.Message, component.Name, val, t.Value)
			issue := domain.NewIssue("threshold:"+t.MetricKey, component.Path, 0)
			issue.Message = msg
			issue.Severity = t.Severity
			issue.Type = domain.TypeCodeSmell
			issues = append(issues, issue)
		}
	}
	return issues
}

func violated(actual float64, relation string, threshold float64) bool {
	switch relation {
	case "gt":
		return actual > threshold
	case "lt":
		return actual < threshold
	case "gte":
		return actual >= threshold
	case "lte":
		return actual <= threshold
	}
	return false
}
