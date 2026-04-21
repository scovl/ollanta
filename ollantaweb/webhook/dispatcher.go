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
	"log"
	"net/http"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
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
	ClaimNext(ctx context.Context, workerID string) (*postgres.WebhookJob, error)
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
}

// NewDispatcher creates a durable webhook dispatcher.
func NewDispatcher(repo *postgres.WebhookRepository, jobs *postgres.WebhookJobRepository, workerID string) *Dispatcher {
	return &Dispatcher{
		repo:      repo,
		jobs:      jobs,
		workerID:  workerID,
		pollDelay: time.Second,
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
				log.Printf("webhook: process next job: %v", err)
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
}

// Dispatch creates durable webhook jobs for all subscribers of the given event.
func (d *Dispatcher) Dispatch(ctx context.Context, projectID int64, event string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("webhook: marshal payload for event %s: %v", event, err)
		return
	}

	hooks, err := d.repo.ForEvent(ctx, projectID, event)
	if err != nil {
		log.Printf("webhook: query hooks for event %s: %v", event, err)
		return
	}

	for _, wh := range hooks {
		job := &postgres.WebhookJob{
			WebhookID:     wh.ID,
			Event:         event,
			Payload:       data,
			Status:        "accepted",
			NextAttemptAt: time.Now().UTC(),
		}
		if projectID != 0 {
			pid := projectID
			job.ProjectID = &pid
		}
		if err := d.jobs.Create(ctx, job); err != nil {
			log.Printf("webhook: create job for webhook %d event %s: %v", wh.ID, event, err)
		}
	}
}

func (d *Dispatcher) processNext(ctx context.Context) (bool, error) {
	job, err := d.jobs.ClaimNext(ctx, d.workerID)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	wh, err := d.repo.GetByID(ctx, job.WebhookID)
	if err != nil {
		if markErr := d.jobs.MarkFailed(ctx, job.ID, err.Error(), nil, nil); markErr != nil {
			return true, markErr
		}
		return true, nil
	}

	code, body, sendErr := d.send(wh, job.Event, job.Payload)

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
	if err := d.repo.CreateDelivery(ctx, del); err != nil {
		log.Printf("webhook: record delivery attempt %d for webhook %d: %v", job.Attempts, job.WebhookID, err)
	}

	if del.Success {
		if err := d.jobs.MarkCompleted(ctx, job.ID, del.ResponseCode, del.ResponseBody); err != nil {
			return true, err
		}
		return true, nil
	}

	if job.Attempts >= len(retryDelays) {
		log.Printf("webhook: dead-letter webhook %d event %s after %d attempts: %v",
			job.WebhookID, job.Event, job.Attempts, sendErr)
		if err := d.jobs.MarkFailed(ctx, job.ID, errorMessage(sendErr, code), del.ResponseCode, del.ResponseBody); err != nil {
			return true, err
		}
		return true, nil
	}

	delay := retryDelays[job.Attempts-1]
	log.Printf("webhook: delivery failed (attempt %d/%d) webhook %d event %s: %v — retry in %s",
		job.Attempts, len(retryDelays), job.WebhookID, job.Event, sendErr, delay)
	if err := d.jobs.Reschedule(ctx, job.ID, errorMessage(sendErr, code), time.Now().UTC().Add(delay), del.ResponseCode, del.ResponseBody); err != nil {
		return true, err
	}
	return true, nil
}

// send performs a single HTTP POST with HMAC signing.
func (d *Dispatcher) send(wh *postgres.Webhook, event string, payload []byte) (int, string, error) {
	req, err := http.NewRequest(http.MethodPost, wh.URL, bytes.NewReader(payload))
	if err != nil {
		return 0, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ollanta-Event", event)
	req.Header.Set("User-Agent", "ollanta-webhook/1.0")

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
