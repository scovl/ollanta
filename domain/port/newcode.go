// Package port defines the inbound and outbound interfaces (Ports) of the domain layer.
package port

import (
	"context"

	"github.com/scovl/ollanta/domain/model"
)

// ScanLister is satisfied by any repository that can list scans for a project.
// It is used by the newcode resolver to locate the baseline scan.
type ScanLister interface {
	// ListByProject returns scans for a project ordered by analysis_date DESC.
	ListByProject(ctx context.Context, projectID int64) ([]*model.Scan, error)
}

// INewCodeRepo is the outbound port for new-code period persistence.
type INewCodeRepo interface {
	GetGlobal(ctx context.Context) (*model.NewCodePeriod, error)
	GetForProject(ctx context.Context, projectID int64) (*model.NewCodePeriod, error)
	GetForBranch(ctx context.Context, projectID int64, branch string) (*model.NewCodePeriod, error)
	Resolve(ctx context.Context, projectID int64, branch string) (*model.NewCodePeriod, error)
	SetGlobal(ctx context.Context, strategy, value string) error
	SetForProject(ctx context.Context, projectID int64, strategy, value string) error
	SetForBranch(ctx context.Context, projectID int64, branch, strategy, value string) error
	DeleteForProject(ctx context.Context, projectID int64) error
	DeleteForBranch(ctx context.Context, projectID int64, branch string) error
}
