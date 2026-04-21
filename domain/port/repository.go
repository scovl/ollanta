// Package port defines the inbound and outbound interfaces (Ports) of the domain layer.
package port

import (
	"context"
	"time"

	"github.com/scovl/ollanta/domain/model"
)

// IProjectRepo is the outbound port for project persistence.
type IProjectRepo interface {
	Create(ctx context.Context, p *model.Project) error
	Upsert(ctx context.Context, p *model.Project) error
	GetByKey(ctx context.Context, key string) (*model.Project, error)
	GetByID(ctx context.Context, id int64) (*model.Project, error)
	List(ctx context.Context) ([]*model.Project, error)
	Delete(ctx context.Context, id int64) error
}

// IScanRepo is the outbound port for scan persistence.
type IScanRepo interface {
	Create(ctx context.Context, s *model.Scan) error
	Update(ctx context.Context, s *model.Scan) error
	GetByID(ctx context.Context, id int64) (*model.Scan, error)
	GetLatest(ctx context.Context, projectID int64) (*model.Scan, error)
	ListByProject(ctx context.Context, projectID int64) ([]*model.Scan, error)
}

// IScanJobRepo is the outbound port for durable scan intake state.
type IScanJobRepo interface {
	Create(ctx context.Context, job *model.ScanJob) error
	GetByID(ctx context.Context, id int64) (*model.ScanJob, error)
	ClaimNext(ctx context.Context, workerID string) (*model.ScanJob, error)
	MarkCompleted(ctx context.Context, id, scanID int64) error
	MarkFailed(ctx context.Context, id int64, lastError string) error
}

// IIssueRepo is the outbound port for issue persistence.
type IIssueRepo interface {
	BulkInsert(ctx context.Context, issues []model.IssueRow) error
	Query(ctx context.Context, filter model.IssueFilter) ([]*model.IssueRow, int, error)
	Facets(ctx context.Context, projectID, scanID int64) (*model.IssueFacets, error)
	CountByProject(ctx context.Context, projectID int64) (int, error)
	GetByID(ctx context.Context, id int64) (*model.IssueRow, error)
	Transition(ctx context.Context, issueID, userID int64, toStatus, resolution, comment string) error
}

// IMeasureRepo is the outbound port for measure persistence.
type IMeasureRepo interface {
	BulkInsert(ctx context.Context, measures []model.MeasureRow) error
	GetLatest(ctx context.Context, projectID int64, metricKey string) (*model.MeasureRow, error)
	Trend(ctx context.Context, projectID int64, metricKey string, from, to time.Time) ([]model.TrendPoint, error)
}
