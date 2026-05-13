package ingest

import (
	"context"

	appingest "github.com/scovl/ollanta/application/ingest"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/webhook"
)

// ScanJob is the HTTP-facing durable intake record.
type ScanJob = model.ScanJob

// ScanJobService wraps the application durable intake service.
type ScanJobService struct {
	inner *appingest.ScanJobService
}

// NewScanJobService creates a durable intake service backed by PostgreSQL.
func NewScanJobService(jobs *postgres.ScanJobRepository) *ScanJobService {
	return &ScanJobService{inner: appingest.NewScanJobService(&scanJobRepoAdapter{inner: jobs})}
}

// Submit stores a scan report as an accepted job.
func (s *ScanJobService) Submit(ctx context.Context, req *IngestRequest) (*ScanJob, error) {
	return s.inner.Submit(ctx, req)
}

// SubmitWithOptions stores a scan report with idempotency and durable backpressure controls.
func (s *ScanJobService) SubmitWithOptions(ctx context.Context, req *IngestRequest, opts ScanJobSubmitOptions) (*ScanJobSubmitResult, error) {
	return s.inner.SubmitWithOptions(ctx, req, opts)
}

// Get returns a stored scan job.
func (s *ScanJobService) Get(ctx context.Context, id int64) (*ScanJob, error) {
	return s.inner.Get(ctx, id)
}

// ScanJobProcessor runs accepted jobs through the ingest use case.
type ScanJobProcessor struct {
	inner *appingest.ScanJobProcessor
	jobs  *postgres.ScanJobRepository
}

// IngestRepositories groups the repositories required by the durable ingest worker.
type IngestRepositories struct {
	Projects  *postgres.ProjectRepository
	Scans     *postgres.ScanRepository
	Issues    *postgres.IssueRepository
	Measures  *postgres.MeasureRepository
	Snapshots *postgres.CodeSnapshotRepository
	Profiles  *postgres.ProfileSnapshotRepository
	Tags      *postgres.TagRepository
	Gates     *postgres.GateRepository
}

// NewScanJobProcessor creates a background job processor for the compute role.
func NewScanJobProcessor(
	workerID string,
	jobs *postgres.ScanJobRepository,
	repos IngestRepositories,
	enqueuer IndexEnqueuer,
	dispatcher *webhook.Dispatcher,
) *ScanJobProcessor {
	var searchEnqueuer port.ISearchEnqueuer
	if enqueuer != nil {
		searchEnqueuer = searchEnqueuerAdapter{inner: enqueuer}
	}

	var webhookDispatcher port.IWebhookDispatcher
	if dispatcher != nil {
		webhookDispatcher = webhookDispatcherAdapter{inner: dispatcher}
	}

	ingestUseCase := appingest.NewIngestUseCase(
		&projectRepoAdapter{inner: repos.Projects},
		&scanRepoAdapter{inner: repos.Scans},
		&issueRepoAdapter{inner: repos.Issues},
		&measureRepoAdapter{inner: repos.Measures},
		&codeSnapshotRepoAdapter{inner: repos.Snapshots},
		searchEnqueuer,
		webhookDispatcher,
	)
	wireGateEvaluator(ingestUseCase, repos.Gates)
	ingestUseCase.SetProfileSnapshotRepo(repos.Profiles)
	ingestUseCase.SetTagCatalogRepo(repos.Tags)

	return &ScanJobProcessor{
		inner: appingest.NewScanJobProcessor(workerID, &scanJobRepoAdapter{inner: jobs}, ingestUseCase),
		jobs:  jobs,
	}
}

// ProcessNext claims and processes the next accepted job.
func (p *ScanJobProcessor) ProcessNext(ctx context.Context) (*ScanJob, error) {
	return p.inner.ProcessNext(ctx)
}

// CountByStatus returns the number of durable scan jobs in the given state.
func (p *ScanJobProcessor) CountByStatus(ctx context.Context, status string) (int, error) {
	if p == nil || p.jobs == nil {
		return 0, nil
	}
	return p.jobs.CountByStatus(ctx, status)
}

type webhookDispatcherAdapter struct {
	inner *webhook.Dispatcher
}

func (a webhookDispatcherAdapter) Dispatch(ctx context.Context, projectID, scanID int64, event string) error {
	a.inner.Dispatch(ctx, projectID, event, map[string]any{
		"event":      event,
		"project_id": projectID,
		"scan_id":    scanID,
	})
	return nil
}
