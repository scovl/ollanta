package qualitygate_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantaengine/qualitygate"
)

func TestEvaluatePersistentTotalMeasures(t *testing.T) {
	t.Parallel()
	conditions := []qualitygate.PersistentCondition{
		{MetricKey: "bugs", Op: qualitygate.OpGreaterThan, Threshold: 0, OnNewCode: false},
	}
	req := qualitygate.EvalRequest{
		TotalMeasures: map[string]float64{"bugs": 3},
		NewMeasures:   map[string]float64{},
	}
	status := qualitygate.EvaluatePersistent(conditions, req)
	if status.Status != qualitygate.GateError {
		t.Errorf("expected ERROR, got %s", status.Status)
	}
}

func TestEvaluatePersistentOnNewCode(t *testing.T) {
	t.Parallel()
	conditions := []qualitygate.PersistentCondition{
		{MetricKey: "new_bugs", Op: qualitygate.OpGreaterThan, Threshold: 0, OnNewCode: true},
	}
	req := qualitygate.EvalRequest{
		TotalMeasures: map[string]float64{"new_bugs": 5}, // should not be used
		NewMeasures:   map[string]float64{"new_bugs": 0},
	}
	status := qualitygate.EvaluatePersistent(conditions, req)
	if status.Status != qualitygate.GateOK {
		t.Errorf("expected OK, got %s", status.Status)
	}
}

func TestEvaluatePersistentSmallChangesetSkipsNewCoverage(t *testing.T) {
	t.Parallel()
	conditions := []qualitygate.PersistentCondition{
		{MetricKey: "new_coverage", Op: qualitygate.OpLessThan, Threshold: 80, OnNewCode: true},
	}
	req := qualitygate.EvalRequest{
		TotalMeasures:       map[string]float64{},
		NewMeasures:         map[string]float64{"new_coverage": 10}, // would fail if evaluated
		ChangedLines:        5,
		SmallChangesetLines: 20,
	}
	status := qualitygate.EvaluatePersistent(conditions, req)
	// Small changeset: new_coverage is skipped → gate should OK
	if status.Status != qualitygate.GateOK {
		t.Errorf("expected OK on small changeset, got %s", status.Status)
	}
}

func TestEvaluatePersistentSmallChangesetBugsNotSkipped(t *testing.T) {
	t.Parallel()
	conditions := []qualitygate.PersistentCondition{
		{MetricKey: "new_bugs", Op: qualitygate.OpGreaterThan, Threshold: 0, OnNewCode: true},
	}
	req := qualitygate.EvalRequest{
		TotalMeasures:       map[string]float64{},
		NewMeasures:         map[string]float64{"new_bugs": 2},
		ChangedLines:        5,
		SmallChangesetLines: 20,
	}
	status := qualitygate.EvaluatePersistent(conditions, req)
	// new_bugs is not in exclusion list → still evaluated → should ERROR
	if status.Status != qualitygate.GateError {
		t.Errorf("expected ERROR for new_bugs on small changeset, got %s", status.Status)
	}
}
