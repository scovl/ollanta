// Package port defines the inbound and outbound interfaces (Ports) of the domain layer.
package port

import (
	"context"

	"github.com/scovl/ollanta/domain/model"
)

// IGateRepo is the outbound port for quality gate persistence.
type IGateRepo interface {
	List(ctx context.Context) ([]*model.QualityGate, error)
	GetByID(ctx context.Context, id int64) (*model.QualityGate, error)
	Create(ctx context.Context, g *model.QualityGate) error
	Update(ctx context.Context, g *model.QualityGate) error
	Delete(ctx context.Context, id int64) error
	Conditions(ctx context.Context, gateID int64) ([]model.GateCondition, error)
	AddCondition(ctx context.Context, c *model.GateCondition) error
	UpdateCondition(ctx context.Context, c *model.GateCondition) error
	RemoveCondition(ctx context.Context, conditionID int64) error
	Copy(ctx context.Context, sourceID int64, newName string) (*model.QualityGate, error)
	SetDefault(ctx context.Context, id int64) error
	AssignToProject(ctx context.Context, projectID, gateID int64) error
	ForProject(ctx context.Context, projectID int64) (*model.QualityGate, error)
}
