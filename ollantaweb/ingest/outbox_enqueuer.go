package ingest

import (
	"context"
	"time"

	"github.com/scovl/ollanta/ollantacore/tracectx"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

// IndexJobEnqueuer writes durable search projection jobs without running a worker loop.
type IndexJobEnqueuer struct {
	jobs indexJobStore
}

// NewIndexJobEnqueuer creates an enqueue-only adapter for durable index jobs.
func NewIndexJobEnqueuer(jobs *postgres.IndexJobRepository) *IndexJobEnqueuer {
	return &IndexJobEnqueuer{jobs: jobs}
}

// Enqueue persists a durable search index job.
func (e *IndexJobEnqueuer) Enqueue(ctx context.Context, scanID, projectID int64, projectKey string) error {
	traceParent, traceState := tracectx.Inject(ctx)
	return e.jobs.Create(ctx, &postgres.IndexJob{
		ScanID:        scanID,
		ProjectID:     projectID,
		ProjectKey:    projectKey,
		Status:        "accepted",
		TraceParent:   traceParent,
		TraceState:    traceState,
		NextAttemptAt: time.Now().UTC(),
	})
}

var _ IndexEnqueuer = (*IndexJobEnqueuer)(nil)
