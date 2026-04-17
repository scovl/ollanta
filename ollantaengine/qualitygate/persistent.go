package qualitygate

// PersistentCondition is a gate condition loaded from the database.
// It extends the in-memory Condition with on_new_code awareness.
type PersistentCondition struct {
	ID         int64    `json:"id"`
	GateID     int64    `json:"gate_id"`
	MetricKey  string   `json:"metric_key"`
	Op         Operator `json:"operator"`
	Threshold  float64  `json:"threshold"`
	OnNewCode  bool     `json:"on_new_code"`
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
func EvaluatePersistent(conditions []PersistentCondition, req EvalRequest) *GateStatus {
	results := make([]ConditionResult, 0, len(conditions))
	anyError := false
	isSmall := req.SmallChangesetLines > 0 && req.ChangedLines > 0 &&
		req.ChangedLines < req.SmallChangesetLines

	for _, pc := range conditions {
		cond := Condition{
			MetricKey:      pc.MetricKey,
			Operator:       pc.Op,
			ErrorThreshold: pc.Threshold,
		}

		// Skip on small changeset.
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

		actual, ok := measures[pc.MetricKey]
		cr := ConditionResult{
			Condition:   cond,
			ActualValue: actual,
			HasValue:    ok,
		}
		if ok && violated(actual, pc.Op, pc.Threshold) {
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
