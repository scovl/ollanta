// Package ingest keeps the existing ollantaweb ingest API while delegating
// the workflow to the application layer.
package ingest

import (
	"context"
	"errors"

	"github.com/scovl/ollanta/application/analysis"
	appingest "github.com/scovl/ollanta/application/ingest"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

// IngestMetadata mirrors the Metadata field of report.Report for JSON decoding.
type IngestMetadata = appingest.IngestMetadata

// IngestMeasures mirrors the Measures field of report.Report for JSON decoding.
type IngestMeasures = appingest.IngestMeasures

// IngestRequest is the payload accepted by POST /api/v1/scans.
type IngestRequest = appingest.IngestRequest

// IngestResult is the response returned after a successful ingest.
type IngestResult = appingest.IngestResult

// ScanBackpressureConfig configures durable queue pressure limits for scan intake.
type ScanBackpressureConfig = appingest.ScanBackpressureConfig

// ScanJobSubmitOptions controls scan intake idempotency and backpressure.
type ScanJobSubmitOptions = appingest.ScanJobSubmitOptions

// ScanJobSubmitResult describes whether scan intake created or reused a job.
type ScanJobSubmitResult = appingest.ScanJobSubmitResult

// ScanJobBackpressureError is returned when durable queue pressure rejects intake.
type ScanJobBackpressureError = appingest.ScanJobBackpressureError

// ErrScanJobIdempotencyConflict is returned when an idempotency key is reused with a different payload.
var ErrScanJobIdempotencyConflict = appingest.ErrScanJobIdempotencyConflict

// IndexEnqueuer abstracts the mechanism for enqueuing durable search index jobs.
type IndexEnqueuer interface {
	Enqueue(ctx context.Context, scanID, projectID int64, projectKey string) error
}

// Pipeline preserves the existing ollantaweb API surface while delegating to the
// hexagonal ingest use case and repository adapters.
type Pipeline struct {
	inner *appingest.IngestUseCase
}

// NewPipeline creates an ingest pipeline backed by application/ingest.
// enqueuer may be nil to disable async search indexing.
func NewPipeline(
	repos IngestRepositories,
	enqueuer IndexEnqueuer,
) *Pipeline {
	var searchEnqueuer appingest.ISearchEnqueuer
	if enqueuer != nil {
		searchEnqueuer = searchEnqueuerAdapter{inner: enqueuer}
	}

	ingestUC := appingest.NewIngestUseCase(
		&projectRepoAdapter{inner: repos.Projects},
		&scanRepoAdapter{inner: repos.Scans},
		&issueRepoAdapter{inner: repos.Issues},
		&measureRepoAdapter{inner: repos.Measures},
		&codeSnapshotRepoAdapter{inner: repos.Snapshots},
		searchEnqueuer,
		nil,
	)
	wireGateEvaluator(ingestUC, repos.Gates)
	ingestUC.SetProfileSnapshotRepo(repos.Profiles)
	ingestUC.SetTagCatalogRepo(repos.Tags)

	return &Pipeline{
		inner: ingestUC,
	}
}

func wireGateEvaluator(ingestUC *appingest.IngestUseCase, gateRepo *postgres.GateRepository) {
	if gateRepo == nil {
		return
	}
	ingestUC.SetGateEvaluator(analysis.NewEvaluateGateUseCase(&gateRepoAdapter{inner: gateRepo}))
}

// Ingest persists a scan report and returns a summary of the results.
func (p *Pipeline) Ingest(ctx context.Context, req *IngestRequest) (*IngestResult, error) {
	return p.inner.Ingest(ctx, req)
}

func mapStoreErr(err error) error {
	if errors.Is(err, postgres.ErrNotFound) {
		return model.ErrNotFound
	}
	return err
}
