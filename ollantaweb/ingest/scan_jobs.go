package ingest

import (
	"context"

	appingest "github.com/scovl/ollanta/application/ingest"
	"github.com/scovl/ollanta/domain/model"
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

// Get returns a stored scan job.
func (s *ScanJobService) Get(ctx context.Context, id int64) (*ScanJob, error) {
	return s.inner.Get(ctx, id)
}

// ScanJobProcessor runs accepted jobs through the ingest use case.
type ScanJobProcessor struct {
	inner *appingest.ScanJobProcessor
}

// NewScanJobProcessor creates a background job processor for the compute role.
func NewScanJobProcessor(
	workerID string,
	jobs *postgres.ScanJobRepository,
	projects *postgres.ProjectRepository,
	scans *postgres.ScanRepository,
	issues *postgres.IssueRepository,
	measures *postgres.MeasureRepository,
	enqueuer IndexEnqueuer,
	dispatcher *webhook.Dispatcher,
) *ScanJobProcessor {
	var searchEnqueuer appingest.ISearchEnqueuer
	if enqueuer != nil {
		searchEnqueuer = searchEnqueuerAdapter{inner: enqueuer}
	}

	var webhookDispatcher appingest.IWebhookDispatcher
	if dispatcher != nil {
		webhookDispatcher = webhookDispatcherAdapter{inner: dispatcher}
	}

	ingestUseCase := appingest.NewIngestUseCase(
		&projectRepoAdapter{inner: projects},
		&scanRepoAdapter{inner: scans},
		&issueRepoAdapter{inner: issues},
		&measureRepoAdapter{inner: measures},
		searchEnqueuer,
		webhookDispatcher,
	)

	return &ScanJobProcessor{
		inner: appingest.NewScanJobProcessor(workerID, &scanJobRepoAdapter{inner: jobs}, ingestUseCase),
	}
}

// ProcessNext claims and processes the next accepted job.
func (p *ScanJobProcessor) ProcessNext(ctx context.Context) (*ScanJob, error) {
	return p.inner.ProcessNext(ctx)
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
