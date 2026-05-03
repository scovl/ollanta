package ingest

import (
	"context"
	"errors"
	"log/slog"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
)

type scanJobProcessor interface {
	ProcessNext(ctx context.Context) (*ScanJob, error)
	CountByStatus(ctx context.Context, status string) (int, error)
}

// ScanJobWorker polls durable scan jobs and delegates processing to the compute processor.
type ScanJobWorker struct {
	processor scanJobProcessor
	idleDelay time.Duration
	metrics   *telemetry.Metrics
}

// NewScanJobWorker creates a background worker loop with the given idle delay.
func NewScanJobWorker(processor scanJobProcessor, idleDelay time.Duration, metrics *telemetry.Metrics) *ScanJobWorker {
	if idleDelay <= 0 {
		idleDelay = time.Second
	}
	return &ScanJobWorker{processor: processor, idleDelay: idleDelay, metrics: metrics}
}

// Start blocks until the context is cancelled.
func (w *ScanJobWorker) Start(ctx context.Context) {
	for w.runOnce(ctx) {
	}
}

func (w *ScanJobWorker) runOnce(ctx context.Context) bool {
	w.refreshQueueMetrics(ctx)
	startedAt := time.Now()
	job, err := w.processor.ProcessNext(ctx)
	if err != nil {
		if w.metrics != nil && !errors.Is(err, context.Canceled) {
			w.metrics.ScanJobsFailed.Inc()
			w.metrics.ObserveIngestStep("process", "failure", time.Since(startedAt))
		}
		return w.handleProcessError(ctx, err)
	}
	if job == nil {
		return w.waitForNextTick(ctx)
	}

	if w.metrics != nil {
		w.metrics.ScansTotal.Inc()
		w.metrics.ScanJobsProcessed.Inc()
		w.metrics.IngestDuration.ObserveDuration(time.Since(startedAt))
		w.metrics.ObserveIngestStep("process", "success", time.Since(startedAt))
	}
	w.refreshQueueMetrics(ctx)
	return true
}

func (w *ScanJobWorker) handleProcessError(ctx context.Context, err error) bool {
	if !errors.Is(err, context.Canceled) {
		slog.ErrorContext(ctx, "process scan job", "error", err)
	}
	return w.waitForNextTick(ctx)
}

func (w *ScanJobWorker) waitForNextTick(ctx context.Context) bool {
	w.refreshQueueMetrics(ctx)
	return waitForNextTick(ctx, w.idleDelay)
}

func (w *ScanJobWorker) refreshQueueMetrics(ctx context.Context) {
	if w == nil || w.metrics == nil || w.processor == nil {
		return
	}

	depth, err := w.processor.CountByStatus(ctx, "accepted")
	if err != nil {
		slog.WarnContext(ctx, "read scan job queue depth", "error", err)
		return
	}
	w.metrics.IngestQueueDepth.Set(int64(depth))
}

func waitForNextTick(ctx context.Context, delay time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(delay):
		return true
	}
}
