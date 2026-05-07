// Package qualitygate evaluates metric-based conditions to produce a pass/fail verdict
// for a project scan. Inspired by the MetricHunter threshold system from OpenStaticAnalyzer.
package qualitygate

import "github.com/scovl/ollanta/ollantacore"

// Operator is a comparison operator used in a gate Condition.
type Operator string

const (
	// OpGreaterThan triggers when actual > threshold.
	OpGreaterThan Operator = "gt"
	// OpLessThan triggers when actual < threshold.
	OpLessThan Operator = "lt"
	// OpEqual triggers when actual == threshold.
	OpEqual Operator = "eq"
	// OpGreaterOrEq triggers when actual >= threshold.
	OpGreaterOrEq Operator = "gte"
	// OpLessOrEq triggers when actual <= threshold.
	OpLessOrEq Operator = "lte"
)

// Condition defines a single quality gate rule: a metric key, a comparison operator,
// and an error threshold. When the condition is violated the gate status becomes ERROR.
type Condition struct {
	MetricKey      string   `json:"metric_key"`
	Operator       Operator `json:"operator"`
	ErrorThreshold float64  `json:"error_threshold"`
	Description    string   `json:"description"`
}

// GateStatusValue is the top-level pass/fail verdict.
type GateStatusValue string

const (
	// GateOK means all conditions passed.
	GateOK GateStatusValue = "OK"
	// GateError means at least one condition was violated.
	GateError GateStatusValue = "ERROR"
)

// ConditionStatus is the per-condition evaluation result.
type ConditionStatus string

const (
	// ConditionOK means this condition was not violated.
	ConditionOK ConditionStatus = "OK"
	// ConditionError means this condition was violated.
	ConditionError ConditionStatus = "ERROR"
)

// ConditionResult holds the evaluation outcome for a single Condition.
type ConditionResult struct {
	Condition   Condition       `json:"condition"`
	Status      ConditionStatus `json:"status"`
	ActualValue float64         `json:"actual_value"`
	HasValue    bool            `json:"has_value"`
}

// GateStatus is the overall evaluation result containing per-condition details.
type GateStatus struct {
	Status     GateStatusValue   `json:"status"`
	Conditions []ConditionResult `json:"conditions"`
}

// Passed reports whether the gate verdict is OK (no conditions violated).
func (g *GateStatus) Passed() bool { return g.Status == GateOK }

// Evaluate assesses all conditions against the provided measures map and returns
// the aggregated GateStatus. A missing metric does not cause a failure (HasValue=false).
func Evaluate(conditions []Condition, measures map[string]float64) *GateStatus {
	results := make([]ConditionResult, 0, len(conditions))
	anyError := false

	for _, c := range conditions {
		actual, ok := measures[c.MetricKey]
		cr := ConditionResult{
			Condition:   c,
			ActualValue: actual,
			HasValue:    ok,
		}
		if ok && ollantacore.Violated(actual, string(c.Operator), c.ErrorThreshold) {
			cr.Status = ConditionError
			anyError = true
		} else {
			cr.Status = ConditionOK
		}
		results = append(results, cr)
	}

	status := GateOK
	if anyError {
		status = GateError
	}
	return &GateStatus{Status: status, Conditions: results}
}

// DefaultConditions returns the built-in "Ollanta way" quality gate:
// zero tolerance for bugs and vulnerabilities.
func DefaultConditions() []Condition {
	return []Condition{
		{
			MetricKey:      "bugs",
			Operator:       OpGreaterThan,
			ErrorThreshold: 0,
			Description:    "No bugs allowed",
		},
		{
			MetricKey:      "vulnerabilities",
			Operator:       OpGreaterThan,
			ErrorThreshold: 0,
			Description:    "No vulnerabilities allowed",
		},
	}
}
