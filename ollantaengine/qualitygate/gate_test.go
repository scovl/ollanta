package qualitygate_test

import (
	"encoding/json"
	"testing"

	"github.com/scovl/ollanta/ollantaengine/qualitygate"
)

// ── Condition ──────────────────────────────────────────────────────────────

func TestCondition_Fields(t *testing.T) {
	c := qualitygate.Condition{
		MetricKey:      "bugs",
		Operator:       qualitygate.OpGreaterThan,
		ErrorThreshold: 5,
		Description:    "No more than 5 bugs",
	}
	if c.MetricKey != "bugs" {
		t.Errorf("MetricKey: got %q", c.MetricKey)
	}
	if c.Operator != qualitygate.OpGreaterThan {
		t.Errorf("Operator: got %q", c.Operator)
	}
	if c.ErrorThreshold != 5 {
		t.Errorf("ErrorThreshold: got %v", c.ErrorThreshold)
	}
}

func TestCondition_AllOperators(t *testing.T) {
	ops := []qualitygate.Operator{
		qualitygate.OpGreaterThan,
		qualitygate.OpLessThan,
		qualitygate.OpEqual,
		qualitygate.OpGreaterOrEq,
		qualitygate.OpLessOrEq,
	}
	for _, op := range ops {
		c := qualitygate.Condition{MetricKey: "x", Operator: op}
		if c.Operator != op {
			t.Errorf("operator %q not stored correctly", op)
		}
	}
}

func TestCondition_JSON(t *testing.T) {
	c := qualitygate.Condition{
		MetricKey:      "coverage",
		Operator:       qualitygate.OpLessThan,
		ErrorThreshold: 80,
		Description:    "Coverage must be ≥ 80%",
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var c2 qualitygate.Condition
	if err := json.Unmarshal(data, &c2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c2.MetricKey != c.MetricKey || c2.Operator != c.Operator || c2.ErrorThreshold != c.ErrorThreshold {
		t.Errorf("round-trip mismatch: %+v", c2)
	}
}

// ── GateStatus ─────────────────────────────────────────────────────────────

func TestGateStatus_OK(t *testing.T) {
	gs := &qualitygate.GateStatus{Status: qualitygate.GateOK}
	if !gs.Passed() {
		t.Error("GateOK should report Passed()=true")
	}
}

func TestGateStatus_Error(t *testing.T) {
	gs := &qualitygate.GateStatus{Status: qualitygate.GateError}
	if gs.Passed() {
		t.Error("GateError should report Passed()=false")
	}
}

// ── Evaluator ──────────────────────────────────────────────────────────────

func TestEvaluate_AllPass(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "bugs", Operator: qualitygate.OpGreaterThan, ErrorThreshold: 0},
	}
	gs := qualitygate.Evaluate(conds, map[string]float64{"bugs": 0})
	if !gs.Passed() {
		t.Error("expected OK when bugs=0 and threshold is >0")
	}
}

func TestEvaluate_FailGT(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "bugs", Operator: qualitygate.OpGreaterThan, ErrorThreshold: 0},
	}
	gs := qualitygate.Evaluate(conds, map[string]float64{"bugs": 1})
	if gs.Passed() {
		t.Error("expected ERROR when bugs=1 > threshold 0")
	}
}

func TestEvaluate_FailLT(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "coverage", Operator: qualitygate.OpLessThan, ErrorThreshold: 80},
	}
	gs := qualitygate.Evaluate(conds, map[string]float64{"coverage": 70})
	if gs.Passed() {
		t.Error("expected ERROR when coverage=70 < threshold 80")
	}
}

func TestEvaluate_FailEQ(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "files", Operator: qualitygate.OpEqual, ErrorThreshold: 0},
	}
	gs := qualitygate.Evaluate(conds, map[string]float64{"files": 0})
	if gs.Passed() {
		t.Error("expected ERROR when files=0 == threshold 0")
	}
}

func TestEvaluate_FailGTE(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "complexity", Operator: qualitygate.OpGreaterOrEq, ErrorThreshold: 10},
	}
	gs := qualitygate.Evaluate(conds, map[string]float64{"complexity": 10})
	if gs.Passed() {
		t.Error("expected ERROR when complexity=10 >= threshold 10")
	}
}

func TestEvaluate_FailLTE(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "coverage", Operator: qualitygate.OpLessOrEq, ErrorThreshold: 60},
	}
	gs := qualitygate.Evaluate(conds, map[string]float64{"coverage": 60})
	if gs.Passed() {
		t.Error("expected ERROR when coverage=60 <= threshold 60")
	}
}

func TestEvaluate_MissingMetricPasses(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "coverage", Operator: qualitygate.OpLessThan, ErrorThreshold: 80},
	}
	gs := qualitygate.Evaluate(conds, map[string]float64{})
	if !gs.Passed() {
		t.Error("missing metric should not cause failure")
	}
	if gs.Conditions[0].HasValue {
		t.Error("HasValue should be false for missing metric")
	}
}

func TestEvaluate_EmptyConditions(t *testing.T) {
	gs := qualitygate.Evaluate(nil, map[string]float64{"bugs": 5})
	if !gs.Passed() {
		t.Error("no conditions → should pass")
	}
}

func TestEvaluate_ConditionResultFields(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "bugs", Operator: qualitygate.OpGreaterThan, ErrorThreshold: 0},
	}
	gs := qualitygate.Evaluate(conds, map[string]float64{"bugs": 3})
	if len(gs.Conditions) != 1 {
		t.Fatalf("expected 1 condition result")
	}
	cr := gs.Conditions[0]
	if !cr.HasValue {
		t.Error("HasValue should be true")
	}
	if cr.ActualValue != 3 {
		t.Errorf("ActualValue: got %v", cr.ActualValue)
	}
	if cr.Status != qualitygate.ConditionError {
		t.Errorf("Status: got %q", cr.Status)
	}
}

func TestEvaluate_DefaultConditions(t *testing.T) {
	defs := qualitygate.DefaultConditions()
	found := map[string]bool{}
	for _, c := range defs {
		found[c.MetricKey] = true
	}
	if !found["bugs"] {
		t.Error("default conditions should include bugs")
	}
	if !found["vulnerabilities"] {
		t.Error("default conditions should include vulnerabilities")
	}
}

func TestEvaluate_OneFailMakesGlobalError(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "bugs", Operator: qualitygate.OpGreaterThan, ErrorThreshold: 0},
		{MetricKey: "coverage", Operator: qualitygate.OpLessThan, ErrorThreshold: 80},
	}
	// bugs=0 passes, coverage=90 passes
	gs := qualitygate.Evaluate(conds, map[string]float64{"bugs": 0, "coverage": 90})
	if !gs.Passed() {
		t.Error("both conditions pass → should be OK")
	}
}

func TestEvaluate_AllPassGlobalOK(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "bugs", Operator: qualitygate.OpGreaterThan, ErrorThreshold: 0},
		{MetricKey: "coverage", Operator: qualitygate.OpLessThan, ErrorThreshold: 80},
	}
	// bugs=1 fails
	gs := qualitygate.Evaluate(conds, map[string]float64{"bugs": 1, "coverage": 90})
	if gs.Passed() {
		t.Error("one failing condition → global status should be ERROR")
	}
}

func TestEvaluate_NilMeasures(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "bugs", Operator: qualitygate.OpGreaterThan, ErrorThreshold: 0},
	}
	gs := qualitygate.Evaluate(conds, nil)
	if !gs.Passed() {
		t.Error("nil measures → missing metric → should pass")
	}
}

func TestEvaluate_NilConditions(t *testing.T) {
	gs := qualitygate.Evaluate(nil, nil)
	if !gs.Passed() {
		t.Error("nil conditions and measures → should pass")
	}
}

func TestEvaluate_UnknownOperator(t *testing.T) {
	conds := []qualitygate.Condition{
		{MetricKey: "bugs", Operator: "INVALID_OP", ErrorThreshold: 0},
	}
	gs := qualitygate.Evaluate(conds, map[string]float64{"bugs": 99})
	if !gs.Passed() {
		t.Error("unknown operator defaults to not-violated → should pass")
	}
}
