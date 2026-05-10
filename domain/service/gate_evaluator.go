// Package service contains pure domain logic with zero external dependencies.
// Gate evaluation is inspired by the MetricHunter threshold system from OpenStaticAnalyzer.
package service

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
	// GateWarn means at least one condition hit the warning threshold but no condition hit the error threshold.
	GateWarn GateStatusValue = "WARN"
	// GateError means at least one condition was violated at the error level.
	GateError GateStatusValue = "ERROR"
)

// ConditionStatus is the per-condition evaluation result.
type ConditionStatus string

const (
	// ConditionOK means this condition was not violated.
	ConditionOK ConditionStatus = "OK"
	// ConditionWarn means this condition hit the warning threshold but not the error threshold.
	ConditionWarn ConditionStatus = "WARN"
	// ConditionError means this condition was violated at the error level.
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

// Passed reports whether the gate verdict is OK or WARN (not blocking).
func (g *GateStatus) Passed() bool { return g.Status == GateOK || g.Status == GateWarn }

// IsWarning reports whether the gate verdict is WARN.
func (g *GateStatus) IsWarning() bool { return g.Status == GateWarn }

// Failed reports whether the gate verdict is ERROR (blocking).
func (g *GateStatus) Failed() bool { return g.Status == GateError }

// FailedConditions returns conditions with ERROR or WARN status.
func (g *GateStatus) FailedConditions() []ConditionResult {
	var out []ConditionResult
	for _, c := range g.Conditions {
		if c.Status == ConditionError || c.Status == ConditionWarn {
			out = append(out, c)
		}
	}
	return out
}

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

// PersistentCondition is a gate condition loaded from the database.
// It extends the in-memory Condition with on_new_code and warning threshold.
type PersistentCondition struct {
	ID               int64    `json:"id"`
	GateID           int64    `json:"gate_id"`
	MetricKey        string   `json:"metric_key"`
	Op               Operator `json:"operator"`
	Threshold        float64  `json:"threshold"`
	WarningThreshold *float64 `json:"warning_threshold,omitempty"`
	OnNewCode        bool     `json:"on_new_code"`
}

// EvalRequest carries all data needed for a persistent gate evaluation.
type EvalRequest struct {
	// TotalMeasures are aggregated over the full scan history.
	TotalMeasures map[string]float64
	// NewMeasures are measured only on new/changed code in this scan.
	NewMeasures map[string]float64
	// ChangedLines is the number of lines changed in this scan (used for small changeset).
	ChangedLines int
	// SmallChangesetLines is the threshold below which new_coverage and new_duplications are skipped.
	SmallChangesetLines int
}

// smallChangesetExcludes are metric keys skipped on small changesets.
var smallChangesetExcludes = map[string]bool{
	"new_coverage":     true,
	"new_duplications": true,
}

// EvaluatePersistent assesses a set of database-sourced conditions.
// Conditions with on_new_code=true use NewMeasures; others use TotalMeasures.
// Small changeset skips new_coverage and new_duplications automatically.
// Warning thresholds are evaluated: if warning is violated but error is not, condition is WARN.
func EvaluatePersistent(conditions []PersistentCondition, req EvalRequest) *GateStatus {
	results := make([]ConditionResult, 0, len(conditions))
	anyError := false
	anyWarn := false
	isSmall := req.SmallChangesetLines > 0 && req.ChangedLines > 0 &&
		req.ChangedLines < req.SmallChangesetLines

	for _, pc := range conditions {
		cond := Condition{
			MetricKey:      pc.MetricKey,
			Operator:       pc.Op,
			ErrorThreshold: pc.Threshold,
		}

		if isSmall && pc.OnNewCode && smallChangesetExcludes[pc.MetricKey] {
			results = append(results, ConditionResult{
				Condition: cond,
				Status:    ConditionOK,
				HasValue:  false,
			})
			continue
		}

		measures := req.TotalMeasures
		if pc.OnNewCode {
			measures = req.NewMeasures
		}

		cr := evalPersistentCondition(cond, pc, measures)
		switch cr.Status {
		case ConditionError:
			anyError = true
		case ConditionWarn:
			anyWarn = true
		}
		results = append(results, cr)
	}

	status := GateOK
	if anyError {
		status = GateError
	} else if anyWarn {
		status = GateWarn
	}
	return &GateStatus{Status: status, Conditions: results}
}

// evalPersistentCondition evaluates a single condition against measures.
// Returns ConditionError if the error threshold is violated,
// ConditionWarn if the warning threshold is violated,
// or ConditionOK otherwise.
func evalPersistentCondition(cond Condition, pc PersistentCondition, measures map[string]float64) ConditionResult {
	actual, ok := measures[pc.MetricKey]
	cr := ConditionResult{
		Condition:   cond,
		ActualValue: actual,
		HasValue:    ok,
	}
	if !ok {
		cr.Status = ConditionOK
	} else if ollantacore.Violated(actual, string(pc.Op), pc.Threshold) {
		cr.Status = ConditionError
	} else if pc.WarningThreshold != nil && ollantacore.Violated(actual, string(pc.Op), *pc.WarningThreshold) {
		cr.Status = ConditionWarn
	} else {
		cr.Status = ConditionOK
	}
	return cr
}
