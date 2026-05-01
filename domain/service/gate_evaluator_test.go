package service

import (
	"testing"

	"github.com/scovl/ollanta/domain/model"
)

func TestEvaluate_TestAndMutationMetrics(t *testing.T) {
	t.Parallel()

	status := Evaluate([]Condition{
		{MetricKey: model.MetricTestFailures, Operator: OpGreaterThan, ErrorThreshold: 0},
		{MetricKey: model.MetricMutationScore, Operator: OpLessThan, ErrorThreshold: 80},
	}, map[string]float64{
		model.MetricTestFailures:  1,
		model.MetricMutationScore: 79.5,
	})

	if status.Status != GateError {
		t.Fatalf("status = %s, want ERROR", status.Status)
	}
	if len(status.Conditions) != 2 {
		t.Fatalf("conditions = %d, want 2", len(status.Conditions))
	}
	for _, condition := range status.Conditions {
		if condition.Status != ConditionError || !condition.HasValue {
			t.Fatalf("condition result = %+v, want error with value", condition)
		}
	}
}

func TestEvaluate_MissingOptionalTestMetricPasses(t *testing.T) {
	t.Parallel()

	status := Evaluate([]Condition{
		{MetricKey: model.MetricMutationScore, Operator: OpLessThan, ErrorThreshold: 80},
	}, map[string]float64{})

	if status.Status != GateOK {
		t.Fatalf("status = %s, want OK", status.Status)
	}
	if len(status.Conditions) != 1 || status.Conditions[0].HasValue {
		t.Fatalf("condition result = %+v, want missing value", status.Conditions)
	}
}
