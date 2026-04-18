package ingest

import (
	"context"
	"log"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
)

// IndexJob represents a unit of work for the background search indexer.
type IndexJob struct {
	ScanID     int64
	ProjectID  int64
	ProjectKey string
}

// Worker drains an IndexJob channel and calls the search indexer.
// If the indexer is unavailable, jobs are retried up to maxRetries times.
type Worker struct {
	indexer    search.IIndexer
	issues     *postgres.IssueRepository
	queue      chan IndexJob
	maxRetries int
}

// NewWorker creates a Worker with a buffered job queue of the given size.
func NewWorker(
	indexer search.IIndexer,
	issues *postgres.IssueRepository,
	bufferSize int,
) *Worker {
	return &Worker{
		indexer:    indexer,
		issues:     issues,
		queue:      make(chan IndexJob, bufferSize),
		maxRetries: 3,
	}
}

// compile-time interface check
var _ IndexEnqueuer = (*Worker)(nil)

// Enqueue submits a job to the queue without blocking.
// If the queue is full, the job is dropped and a warning is logged.
func (w *Worker) Enqueue(_ context.Context, scanID, projectID int64, projectKey string) error {
	job := IndexJob{ScanID: scanID, ProjectID: projectID, ProjectKey: projectKey}
	select {
	case w.queue <- job:
		return nil
	default:
		log.Printf("ollantaweb/worker: queue full, dropping index job for scan %d", scanID)
		return nil
	}
}

// Start begins processing jobs from the queue until ctx is cancelled.
// It should be called in a goroutine.
func (w *Worker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-w.queue:
			if !ok {
				return
			}
			w.process(ctx, job)
		}
	}
}

// Stop drains remaining jobs and closes the queue.
func (w *Worker) Stop() {
	close(w.queue)
}

func (w *Worker) process(ctx context.Context, job IndexJob) {
	var lastErr error
	for attempt := 1; attempt <= w.maxRetries; attempt++ {
		if err := w.doIndex(ctx, job); err != nil {
			lastErr = err
			backoff := time.Duration(attempt) * 2 * time.Second
			log.Printf("ollantaweb/worker: index attempt %d/%d for scan %d failed: %v (retry in %s)",
				attempt, w.maxRetries, job.ScanID, err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}
		return
	}
	log.Printf("ollantaweb/worker: gave up indexing scan %d after %d attempts: %v",
		job.ScanID, w.maxRetries, lastErr)
}

func (w *Worker) doIndex(ctx context.Context, job IndexJob) error {
	sid := job.ScanID
	pid := job.ProjectID

	issues, _, err := w.issues.Query(ctx, postgres.IssueFilter{
		ScanID:    &sid,
		ProjectID: &pid,
		Limit:     10000,
	})
	if err != nil {
		return err
	}
	rows := make([]postgres.IssueRow, len(issues))
	for i, iss := range issues {
		rows[i] = *iss
	}
	return w.indexer.IndexIssues(ctx, job.ProjectKey, rows)
}
