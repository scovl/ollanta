package ingest

import (
	"context"
	"errors"
	"log/slog"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantacore/tracectx"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
)

type indexJobStore interface {
	Create(ctx context.Context, job *postgres.IndexJob) error
	ClaimNext(ctx context.Context, workerID string) (*postgres.IndexJob, error)
	CountByStatus(ctx context.Context, status string) (int, error)
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
	metrics    *telemetry.Metrics
}

// NewWorker creates a durable search projection worker.
func NewWorker(
	indexer search.IIndexer,
	issues *postgres.IssueRepository,
	jobs *postgres.IndexJobRepository,
	workerID string,
	metrics *telemetry.Metrics,
) *Worker {
	return &Worker{
		indexer:    indexer,
		issues:     issues,
		jobs:       jobs,
		workerID:   workerID,
		pollDelay:  time.Second,
		maxRetries: 3,
		batchSize:  1000,
		metrics:    metrics,
	}
}

// compile-time interface check
var _ IndexEnqueuer = (*Worker)(nil)

// Enqueue persists a durable search index job.
func (w *Worker) Enqueue(ctx context.Context, scanID, projectID int64, projectKey string) error {
	traceParent, traceState := tracectx.Inject(ctx)
	return w.jobs.Create(ctx, &postgres.IndexJob{
		ScanID:        scanID,
		ProjectID:     projectID,
		ProjectKey:    projectKey,
		Status:        "accepted",
		TraceParent:   traceParent,
		TraceState:    traceState,
		NextAttemptAt: time.Now().UTC(),
	})
}

// Start begins polling and processing durable jobs until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	for {
		processed, err := w.processNext(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				slog.ErrorContext(ctx, "process next index job", "worker_id", w.workerID, "error", err)
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
	// Shutdown is controlled by the context passed to Start.
}

func (w *Worker) processNext(ctx context.Context) (bool, error) {
	job, err := w.jobs.ClaimNext(ctx, w.workerID)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			w.refreshQueueMetrics(ctx)
			return false, nil
		}
		return false, err
	}
	w.refreshQueueMetrics(ctx)

	spanCtx := tracectx.Extract(ctx, job.TraceParent, job.TraceState)
	spanCtx, span := telemetry.StartSpan(spanCtx, "index_job.process")
	defer span.End()

	if err := w.doIndex(spanCtx, job); err != nil {
		if job.Attempts >= w.maxRetries {
			span.RecordError(err)
			if markErr := w.jobs.MarkFailed(spanCtx, job.ID, err.Error()); markErr != nil {
				return true, markErr
			}
			w.refreshQueueMetrics(spanCtx)
			return true, nil
		}

		backoff := time.Duration(job.Attempts) * 2 * time.Second
		if w.metrics != nil {
			w.metrics.IndexJobRetries.Inc()
		}
		slog.WarnContext(spanCtx, "index job retry scheduled",
			telemetry.WithTraceAttrs(spanCtx,
				"worker_id", w.workerID,
				"attempt", job.Attempts,
				"max_attempts", w.maxRetries,
				"scan_id", job.ScanID,
				"backoff", backoff.String(),
				"error", err,
			)...,
		)
		span.RecordError(err)
		if rescheduleErr := w.jobs.Reschedule(spanCtx, job.ID, err.Error(), time.Now().UTC().Add(backoff)); rescheduleErr != nil {
			return true, rescheduleErr
		}
		w.refreshQueueMetrics(spanCtx)
		return true, nil
	}

	if err := w.jobs.MarkCompleted(spanCtx, job.ID); err != nil {
		return true, err
	}
	if w.metrics != nil {
		w.metrics.IndexJobsProcessed.Inc()
	}
	w.refreshQueueMetrics(spanCtx)
	return true, nil
}

func (w *Worker) refreshQueueMetrics(ctx context.Context) {
	if w == nil || w.metrics == nil || w.jobs == nil {
		return
	}

	depth, err := w.jobs.CountByStatus(ctx, "accepted")
	if err != nil {
		slog.WarnContext(ctx, "read index job queue depth", "worker_id", w.workerID, "error", err)
		return
	}
	w.metrics.IndexQueueDepth.Set(int64(depth))
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
