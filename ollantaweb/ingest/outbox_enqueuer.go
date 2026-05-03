package ingest

import (
	"context"

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
	return enqueueIndexJob(ctx, e.jobs, scanID, projectID, projectKey)
}

var _ IndexEnqueuer = (*IndexJobEnqueuer)(nil)
