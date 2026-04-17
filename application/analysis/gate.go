// Package analysis provides use cases for quality gate evaluation.
// The pure evaluation logic lives in domain/service; this package wires it
// to the persistence ports for database-backed gate conditions.
package analysis

import (
	"context"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	"github.com/scovl/ollanta/domain/service"
)

// EvaluateGateUseCase evaluates a quality gate for a given scan.
type EvaluateGateUseCase struct {
	gates port.IGateRepo
}

// NewEvaluateGateUseCase creates a use case backed by the provided gate repository.
func NewEvaluateGateUseCase(gates port.IGateRepo) *EvaluateGateUseCase {
	return &EvaluateGateUseCase{gates: gates}
}

// EvaluateDefault evaluates the default built-in conditions against the provided measures.
// This is a fast path that does not require a database round-trip.
func (uc *EvaluateGateUseCase) EvaluateDefault(measures map[string]float64) *service.GateStatus {
	return service.Evaluate(service.DefaultConditions(), measures)
}

// EvaluateForProject loads gate conditions from the repository and evaluates them.
// Falls back to default conditions if no gate is configured for the project.
func (uc *EvaluateGateUseCase) EvaluateForProject(
	ctx context.Context,
	projectID int64,
	totalMeasures, newMeasures map[string]float64,
	changedLines, smallChangesetLines int,
) (*service.GateStatus, error) {
	gate, err := uc.gates.ForProject(ctx, projectID)
	if err != nil {
		// No gate configured — use built-in defaults.
		return service.Evaluate(service.DefaultConditions(), totalMeasures), nil
	}

	rawConds, err := uc.gates.Conditions(ctx, gate.ID)
	if err != nil {
		return service.Evaluate(service.DefaultConditions(), totalMeasures), nil
	}

	conds := make([]service.PersistentCondition, len(rawConds))
	for i, c := range rawConds {
		conds[i] = service.PersistentCondition{
			ID:        c.ID,
			GateID:    c.GateID,
			MetricKey: c.Metric,
			Op:        service.Operator(c.Operator),
			Threshold: c.Threshold,
			OnNewCode: c.OnNewCode,
		}
	}

	req := service.EvalRequest{
		TotalMeasures:       totalMeasures,
		NewMeasures:         newMeasures,
		ChangedLines:        changedLines,
		SmallChangesetLines: smallChangesetLines,
	}
	return service.EvaluatePersistent(conds, req), nil
}

// MeasuresFromScan builds the measures map from a Scan record for gate evaluation.
func MeasuresFromScan(s *model.Scan) map[string]float64 {
	return map[string]float64{
		"bugs":            float64(s.TotalBugs),
		"vulnerabilities": float64(s.TotalVulnerabilities),
		"code_smells":     float64(s.TotalCodeSmells),
		"files":           float64(s.TotalFiles),
		"lines":           float64(s.TotalLines),
		"ncloc":           float64(s.TotalNcloc),
	}
}
