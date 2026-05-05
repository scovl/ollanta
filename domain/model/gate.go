package model

import "time"

// QualityGate is the canonical quality gate record.
type QualityGate struct {
	ID                  int64     `json:"id"`
	Name                string    `json:"name"`
	IsDefault           bool      `json:"is_default"`
	IsBuiltin           bool      `json:"is_builtin"`
	SmallChangesetLines int       `json:"small_changeset_lines"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// GateCondition is a single condition within a quality gate.
type GateCondition struct {
	ID               int64    `json:"id"`
	GateID           int64    `json:"gate_id"`
	Metric           string   `json:"metric"`
	Operator         string   `json:"operator"`
	Threshold        float64  `json:"threshold"`
	WarningThreshold *float64 `json:"warning_threshold,omitempty"`
	OnNewCode        bool     `json:"on_new_code"`
}

// GateResult is a snapshot of a gate evaluation for a specific scan.
type GateResult struct {
	GateID     int64              `json:"gate_id"`
	Status     string             `json:"status"`
	Conditions []GateConditionEval `json:"conditions"`
	EvaluatedAt time.Time         `json:"evaluated_at"`
}

// GateConditionEval is the per-condition evaluation result stored in a GateResult.
type GateConditionEval struct {
	Metric           string  `json:"metric"`
	Operator         string  `json:"operator"`
	Threshold        float64 `json:"threshold"`
	WarningThreshold *float64 `json:"warning_threshold,omitempty"`
	OnNewCode        bool    `json:"on_new_code,omitempty"`
	Actual           float64 `json:"actual"`
	HasValue         bool    `json:"has_value"`
	Status           string  `json:"status"`
}
