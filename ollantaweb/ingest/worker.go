package ingest

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
)

type indexJobStore interface {
	Create(ctx context.Context, job *postgres.IndexJob) error
	ClaimNext(ctx context.Context, workerID string) (*postgres.IndexJob, error)
	Reschedule(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time) error
	MarkCompleted(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, lastError string) error
}

type issueQueryer interface {
	Query(ctx context.Context, filter postgres.IssueFilter) ([]*postgres.IssueRow, int, error)
}

// Worker polls durable index jobs and applies them to the configured search backend.
type Worker struct {
	indexer    search.IIndexer
	issues     issueQueryer
	jobs       indexJobStore
	workerID   string
	pollDelay  time.Duration
	maxRetries int
	batchSize  int
}

// NewWorker creates a durable search projection worker.
func NewWorker(
	indexer search.IIndexer,
	issues *postgres.IssueRepository,
	jobs *postgres.IndexJobRepository,
	workerID string,
) *Worker {
	return &Worker{
		indexer:    indexer,
		issues:     issues,
		jobs:       jobs,
		workerID:   workerID,
		pollDelay:  time.Second,
		maxRetries: 3,
		batchSize:  1000,
	}
}

// compile-time interface check
var _ IndexEnqueuer = (*Worker)(nil)

// Enqueue persists a durable search index job.
func (w *Worker) Enqueue(ctx context.Context, scanID, projectID int64, projectKey string) error {
	return w.jobs.Create(ctx, &postgres.IndexJob{
		ScanID:        scanID,
		ProjectID:     projectID,
		ProjectKey:    projectKey,
		Status:        "accepted",
		NextAttemptAt: time.Now().UTC(),
	})
}

// Start begins polling and processing durable jobs until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	for {
		processed, err := w.processNext(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("ollantaweb/worker: process next index job: %v", err)
			}
			if !waitForNextTick(ctx, w.pollDelay) {
				return
			}
			continue
		}
		if processed {
			continue
		}
		if !waitForNextTick(ctx, w.pollDelay) {
			return
		}
	}
}

// Stop is a no-op for the durable worker. The context passed to Start controls shutdown.
func (w *Worker) Stop() {
}

func (w *Worker) processNext(ctx context.Context) (bool, error) {
	job, err := w.jobs.ClaimNext(ctx, w.workerID)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	if err := w.doIndex(ctx, job); err != nil {
		if job.Attempts >= w.maxRetries {
			if markErr := w.jobs.MarkFailed(ctx, job.ID, err.Error()); markErr != nil {
				return true, markErr
			}
			return true, nil
		}

		backoff := time.Duration(job.Attempts) * 2 * time.Second
		log.Printf("ollantaweb/worker: index attempt %d/%d for scan %d failed: %v (retry in %s)",
			job.Attempts, w.maxRetries, job.ScanID, err, backoff)
		if rescheduleErr := w.jobs.Reschedule(ctx, job.ID, err.Error(), time.Now().UTC().Add(backoff)); rescheduleErr != nil {
			return true, rescheduleErr
		}
		return true, nil
	}

	if err := w.jobs.MarkCompleted(ctx, job.ID); err != nil {
		return true, err
	}
	return true, nil
}

func (w *Worker) doIndex(ctx context.Context, job *postgres.IndexJob) error {
	sid := job.ScanID
	pid := job.ProjectID
	offset := 0

	for {
		issues, _, err := w.issues.Query(ctx, postgres.IssueFilter{
			ScanID:    &sid,
			ProjectID: &pid,
			Limit:     w.batchSize,
			Offset:    offset,
		})
		if err != nil {
			return err
		}
		if len(issues) == 0 {
			return nil
		}

		rows := make([]postgres.IssueRow, len(issues))
		for i, iss := range issues {
			rows[i] = *iss
		}
		if err := w.indexer.IndexIssues(ctx, job.ProjectKey, rows); err != nil {
			return err
		}

		offset += len(issues)
		if len(issues) < w.batchSize {
			return nil
		}
	}
}
