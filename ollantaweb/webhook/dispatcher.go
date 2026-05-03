// Package webhook implements async webhook delivery with exponential retry,
// HMAC-SHA256 signing, and dead-letter logging.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantacore/tracectx"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"go.opentelemetry.io/otel/trace"
)

// Event names recognised by the dispatcher.
const (
	EventScanCompleted  = "scan.completed"
	EventGateChanged    = "gate.changed"
	EventProjectCreated = "project.created"
	EventProjectDeleted = "project.deleted"
)

// retryDelays defines the exponential back-off schedule (3 attempts).
var retryDelays = []time.Duration{1 * time.Minute, 5 * time.Minute, 30 * time.Minute}

type webhookStore interface {
	ForEvent(ctx context.Context, projectID int64, event string) ([]*postgres.Webhook, error)
	GetByID(ctx context.Context, id int64) (*postgres.Webhook, error)
	CreateDelivery(ctx context.Context, d *postgres.WebhookDelivery) error
}

type webhookJobStore interface {
	Create(ctx context.Context, job *postgres.WebhookJob) error
	GetActiveByIdentity(ctx context.Context, webhookID int64, event, payloadHash string) (*postgres.WebhookJob, error)
	ClaimNext(ctx context.Context, workerID string) (*postgres.WebhookJob, error)
	CountByStatus(ctx context.Context, status string) (int, error)
	Reschedule(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time, responseCode *int, responseBody *string) error
	MarkCompleted(ctx context.Context, id int64, responseCode *int, responseBody *string) error
	MarkFailed(ctx context.Context, id int64, lastError string, responseCode *int, responseBody *string) error
}

// Dispatcher enqueues and delivers webhook jobs using durable PostgreSQL state.
type Dispatcher struct {
	repo      webhookStore
	jobs      webhookJobStore
	client    *http.Client
	workerID  string
	pollDelay time.Duration
	metrics   *telemetry.Metrics
}

// NewDispatcher creates a durable webhook dispatcher.
func NewDispatcher(repo *postgres.WebhookRepository, jobs *postgres.WebhookJobRepository, workerID string, metrics *telemetry.Metrics) *Dispatcher {
	return &Dispatcher{
		repo:      repo,
		jobs:      jobs,
		workerID:  workerID,
		pollDelay: time.Second,
		metrics:   metrics,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start polls and delivers durable webhook jobs until ctx is cancelled.
func (d *Dispatcher) Start(ctx context.Context) {
	for {
		processed, err := d.processNext(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				slog.ErrorContext(ctx, "process next webhook job", "worker_id", d.workerID, "error", err)
			}
			if !waitForNextTick(ctx, d.pollDelay) {
				return
			}
			continue
		}
		if processed {
			continue
		}
		if !waitForNextTick(ctx, d.pollDelay) {
			return
		}
	}
}

// Stop is a no-op for the durable dispatcher. The Start context controls shutdown.
func (d *Dispatcher) Stop() {
	// Shutdown is controlled by the context passed to Start.
}

// Dispatch creates durable webhook jobs for all subscribers of the given event.
func (d *Dispatcher) Dispatch(ctx context.Context, projectID int64, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "marshal webhook payload", telemetry.WithTraceAttrs(ctx, "event", event, "error", err)...)
		return
	}

	hooks, err := d.repo.ForEvent(ctx, projectID, event)
	if err != nil {
		slog.ErrorContext(ctx, "query webhooks for event", telemetry.WithTraceAttrs(ctx, "event", event, "project_id", projectID, "error", err)...)
		return
	}

	for _, wh := range hooks {
		payloadHash := postgres.HashPayload(data)
		if existing, err := d.jobs.GetActiveByIdentity(ctx, wh.ID, event, payloadHash); err == nil {
			slog.DebugContext(ctx, "dedupe active webhook job", "webhook_id", wh.ID, "event", event, "job_id", existing.ID)
			continue
		} else if !errors.Is(err, postgres.ErrNotFound) {
			slog.ErrorContext(ctx, "lookup active webhook job", telemetry.WithTraceAttrs(ctx, "webhook_id", wh.ID, "event", event, "error", err)...)
			continue
		}

		traceParent, traceState := tracectx.Inject(ctx)
		job := &postgres.WebhookJob{
			WebhookID:     wh.ID,
			Event:         event,
			Payload:       data,
			PayloadHash:   payloadHash,
			Status:        "accepted",
			TraceParent:   traceParent,
			TraceState:    traceState,
			NextAttemptAt: time.Now().UTC(),
		}
		if projectID != 0 {
			pid := projectID
			job.ProjectID = &pid
		}
		if err := d.jobs.Create(ctx, job); err != nil {
			if existing, findErr := d.jobs.GetActiveByIdentity(ctx, wh.ID, event, payloadHash); findErr == nil {
				slog.DebugContext(ctx, "dedupe raced webhook job", "webhook_id", wh.ID, "event", event, "job_id", existing.ID)
				continue
			}
			slog.ErrorContext(ctx, "create webhook job", telemetry.WithTraceAttrs(ctx, "webhook_id", wh.ID, "event", event, "error", err)...)
		}
	}
}

func (d *Dispatcher) processNext(ctx context.Context) (bool, error) {
	job, err := d.jobs.ClaimNext(ctx, d.workerID)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			d.refreshQueueMetrics(ctx)
			return false, nil
		}
		return false, err
	}
	d.refreshQueueMetrics(ctx)

	spanCtx := tracectx.Extract(ctx, job.TraceParent, job.TraceState)
	spanCtx, span := telemetry.StartSpan(spanCtx, "webhook.process")
	defer span.End()

	wh, err := d.repo.GetByID(spanCtx, job.WebhookID)
	if err != nil {
		return d.failLookup(spanCtx, span, job, err)
	}

	del, sendErr := d.deliver(spanCtx, wh, job)
	if d.metrics != nil {
		d.metrics.WebhookDeliveries.Inc()
	}
	if err := d.repo.CreateDelivery(spanCtx, del); err != nil {
		slog.WarnContext(spanCtx, "record webhook delivery attempt", telemetry.WithTraceAttrs(spanCtx, "webhook_id", job.WebhookID, "attempt", job.Attempts, "error", err)...)
	}

	if del.Success {
		return d.complete(spanCtx, job, del)
	}

	if job.Attempts >= len(retryDelays) {
		return d.deadLetter(spanCtx, span, job, del, sendErr)
	}

	return d.retry(spanCtx, span, job, del, sendErr)
}

func (d *Dispatcher) failLookup(ctx context.Context, span trace.Span, job *postgres.WebhookJob, err error) (bool, error) {
	span.RecordError(err)
	if markErr := d.jobs.MarkFailed(ctx, job.ID, err.Error(), nil, nil); markErr != nil {
		return true, markErr
	}
	d.refreshQueueMetrics(ctx)
	return true, nil
}

func (d *Dispatcher) deliver(ctx context.Context, wh *postgres.Webhook, job *postgres.WebhookJob) (*postgres.WebhookDelivery, error) {
	code, body, sendErr := d.send(ctx, wh, job.Event, job.Payload)
	del := &postgres.WebhookDelivery{
		WebhookID: job.WebhookID,
		Event:     job.Event,
		Payload:   job.Payload,
		Success:   sendErr == nil && code >= 200 && code < 300,
		Attempt:   job.Attempts,
	}
	if code > 0 {
		del.ResponseCode = &code
	}
	if body != "" {
		del.ResponseBody = &body
	}
	return del, sendErr
}

func (d *Dispatcher) complete(ctx context.Context, job *postgres.WebhookJob, del *postgres.WebhookDelivery) (bool, error) {
	if err := d.jobs.MarkCompleted(ctx, job.ID, del.ResponseCode, del.ResponseBody); err != nil {
		return true, err
	}
	d.refreshQueueMetrics(ctx)
	return true, nil
}

func (d *Dispatcher) deadLetter(ctx context.Context, span trace.Span, job *postgres.WebhookJob, del *postgres.WebhookDelivery, sendErr error) (bool, error) {
	span.RecordError(sendErr)
	slog.ErrorContext(ctx, "webhook dead-lettered",
		telemetry.WithTraceAttrs(ctx,
			"webhook_id", job.WebhookID,
			"event", job.Event,
			"attempt", job.Attempts,
			"error", sendErr,
		)...,
	)
	if err := d.jobs.MarkFailed(ctx, job.ID, errorMessage(sendErr, derefInt(del.ResponseCode)), del.ResponseCode, del.ResponseBody); err != nil {
		return true, err
	}
	d.refreshQueueMetrics(ctx)
	return true, nil
}

func (d *Dispatcher) retry(ctx context.Context, span trace.Span, job *postgres.WebhookJob, del *postgres.WebhookDelivery, sendErr error) (bool, error) {
	delay := retryDelays[job.Attempts-1]
	span.RecordError(sendErr)
	slog.WarnContext(ctx, "webhook delivery failed; retry scheduled",
		telemetry.WithTraceAttrs(ctx,
			"webhook_id", job.WebhookID,
			"event", job.Event,
			"attempt", job.Attempts,
			"max_attempts", len(retryDelays),
			"delay", delay.String(),
			"error", sendErr,
		)...,
	)
	if err := d.jobs.Reschedule(ctx, job.ID, errorMessage(sendErr, derefInt(del.ResponseCode)), time.Now().UTC().Add(delay), del.ResponseCode, del.ResponseBody); err != nil {
		return true, err
	}
	d.refreshQueueMetrics(ctx)
	return true, nil
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func (d *Dispatcher) refreshQueueMetrics(ctx context.Context) {
	if d == nil || d.metrics == nil || d.jobs == nil {
		return
	}

	depth, err := d.jobs.CountByStatus(ctx, "accepted")
	if err != nil {
		slog.WarnContext(ctx, "read webhook job queue depth", "worker_id", d.workerID, "error", err)
		return
	}
	d.metrics.WebhookQueueDepth.Set(int64(depth))
}

// send performs a single HTTP POST with HMAC signing.
func (d *Dispatcher) send(ctx context.Context, wh *postgres.Webhook, event string, payload []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(payload))
	if err != nil {
		return 0, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ollanta-Event", event)
	req.Header.Set("User-Agent", "ollanta-webhook/1.0")
	if traceParent, traceState := tracectx.Inject(ctx); traceParent != "" || traceState != "" {
		if traceParent != "" {
			req.Header.Set("traceparent", traceParent)
		}
		if traceState != "" {
			req.Header.Set("tracestate", traceState)
		}
	}

	if wh.Secret != "" {
		sig := sign(payload, wh.Secret)
		req.Header.Set("X-Ollanta-Signature-256", "sha256="+sig)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	var bodyBuf bytes.Buffer
	if resp.ContentLength < 4096 {
		_, _ = bodyBuf.ReadFrom(resp.Body)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, bodyBuf.String(),
			fmt.Errorf("non-2xx response: %d", resp.StatusCode)
	}
	return resp.StatusCode, bodyBuf.String(), nil
}

// sign computes HMAC-SHA256 of payload using the given secret.
func sign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func waitForNextTick(ctx context.Context, delay time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(delay):
		return true
	}
}

func errorMessage(err error, code int) string {
	if err != nil {
		return err.Error()
	}
	if code > 0 {
		return fmt.Sprintf("non-2xx response: %d", code)
	}
	return "unknown delivery error"
}
