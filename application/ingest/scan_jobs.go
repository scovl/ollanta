package ingest

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	"github.com/scovl/ollanta/ollantacore/tracectx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// ErrScanJobIdempotencyConflict is returned when a key is reused with a different payload.
var ErrScanJobIdempotencyConflict = errors.New("idempotency key reused with different scan payload")

// ScanJobBackpressureError is returned when durable queue pressure rejects intake.
type ScanJobBackpressureError struct {
	Reason     string
	RetryAfter time.Duration
}

func (e *ScanJobBackpressureError) Error() string {
	if e == nil || e.Reason == "" {
		return "scan intake backpressure limit exceeded"
	}
	return "scan intake backpressure limit exceeded: " + e.Reason
}

// ScanBackpressureConfig defines durable queue limits for accepting new scans.
type ScanBackpressureConfig struct {
	MaxAccepted          int
	MaxRunning           int
	MaxOldestAcceptedAge time.Duration
	RetryAfter           time.Duration
}

// ScanJobSubmitOptions controls idempotency and backpressure behavior during scan intake.
type ScanJobSubmitOptions struct {
	IdempotencyKey string
	Backpressure   ScanBackpressureConfig
	Now            time.Time
}

// ScanJobSubmitResult describes whether intake created a new job or returned an existing one.
type ScanJobSubmitResult struct {
	Job       *model.ScanJob
	Duplicate bool
}

// ScanJobService persists accepted scan submissions before background processing begins.
type ScanJobService struct {
	jobs port.IScanJobRepo
}

// NewScanJobService creates a durable intake service backed by a scan job repository.
func NewScanJobService(jobs port.IScanJobRepo) *ScanJobService {
	return &ScanJobService{jobs: jobs}
}

// Submit validates and persists a scan submission as an accepted job.
func (s *ScanJobService) Submit(ctx context.Context, req *IngestRequest) (*model.ScanJob, error) {
	result, err := s.SubmitWithOptions(ctx, req, ScanJobSubmitOptions{})
	if err != nil {
		return nil, err
	}
	return result.Job, nil
}

// SubmitWithOptions validates and persists a scan submission with idempotency and backpressure controls.
func (s *ScanJobService) SubmitWithOptions(ctx context.Context, req *IngestRequest, opts ScanJobSubmitOptions) (*ScanJobSubmitResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.Metadata.ProjectKey == "" {
		return nil, fmt.Errorf("project_key is required")
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal job payload: %w", err)
	}
	payloadHash := hashScanPayload(payload)
	idempotencyKey := scanIdempotencyKey(opts.IdempotencyKey, payloadHash)

	dup, err := s.checkIdempotency(ctx, req.Metadata.ProjectKey, idempotencyKey, payloadHash)
	if err != nil {
		return nil, err
	}
	if dup != nil {
		return dup, nil
	}

	if err := s.checkBackpressure(ctx, req.Metadata.ProjectKey, opts); err != nil {
		return nil, err
	}

	job, err := s.createJob(ctx, req, payload, idempotencyKey, payloadHash)
	if err != nil {
		return nil, err
	}
	return &ScanJobSubmitResult{Job: job}, nil
}

func (s *ScanJobService) checkIdempotency(ctx context.Context, projectKey, idempotencyKey, payloadHash string) (*ScanJobSubmitResult, error) {
	existing, err := s.jobs.FindByIdempotencyKey(ctx, projectKey, idempotencyKey)
	if err == nil {
		if !payloadHashesEqual(existing.PayloadHash, payloadHash) {
			return nil, ErrScanJobIdempotencyConflict
		}
		return &ScanJobSubmitResult{Job: existing, Duplicate: true}, nil
	}
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("find scan job by idempotency key: %w", err)
	}
	return nil, nil
}

func (s *ScanJobService) createJob(ctx context.Context, req *IngestRequest, payload []byte, idempotencyKey, payloadHash string) (*model.ScanJob, error) {
	job := &model.ScanJob{
		ProjectKey:     req.Metadata.ProjectKey,
		Status:         model.ScanJobStatusAccepted,
		Payload:        payload,
		IdempotencyKey: idempotencyKey,
		PayloadHash:    payloadHash,
	}
	job.TraceParent, job.TraceState = tracectx.Inject(ctx)
	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create scan job: %w", err)
	}
	return job, nil
}

func (s *ScanJobService) checkBackpressure(ctx context.Context, projectKey string, opts ScanJobSubmitOptions) error {
	limits := opts.Backpressure
	if limits.MaxAccepted <= 0 && limits.MaxRunning <= 0 && limits.MaxOldestAcceptedAge <= 0 {
		return nil
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	pressure, err := s.jobs.QueuePressure(ctx, projectKey, now)
	if err != nil {
		return fmt.Errorf("read scan queue pressure: %w", err)
	}
	if limits.MaxAccepted > 0 && pressure.Accepted >= limits.MaxAccepted {
		return &ScanJobBackpressureError{Reason: "accepted scan job limit reached", RetryAfter: limits.RetryAfter}
	}
	if limits.MaxRunning > 0 && pressure.Running >= limits.MaxRunning {
		return &ScanJobBackpressureError{Reason: "running scan job limit reached", RetryAfter: limits.RetryAfter}
	}
	if limits.MaxOldestAcceptedAge > 0 && pressure.OldestAcceptedAge >= limits.MaxOldestAcceptedAge {
		return &ScanJobBackpressureError{Reason: "oldest accepted scan job age limit reached", RetryAfter: limits.RetryAfter}
	}
	return nil
}

func scanIdempotencyKey(clientKey, payloadHash string) string {
	clientKey = strings.TrimSpace(clientKey)
	if clientKey != "" {
		return clientKey
	}
	return "payload:" + payloadHash
}

func hashScanPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func payloadHashesEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

// Get returns a durable scan job by id.
func (s *ScanJobService) Get(ctx context.Context, id int64) (*model.ScanJob, error) {
	return s.jobs.GetByID(ctx, id)
}

// ScanJobProcessor claims accepted jobs and runs the ingest workflow in the background.
type ScanJobProcessor struct {
	workerID string
	jobs     port.IScanJobRepo
	ingest   *IngestUseCase
}

// NewScanJobProcessor creates a background job processor bound to a worker identity.
func NewScanJobProcessor(workerID string, jobs port.IScanJobRepo, ingest *IngestUseCase) *ScanJobProcessor {
	return &ScanJobProcessor{workerID: workerID, jobs: jobs, ingest: ingest}
}

// ProcessNext claims and processes the next accepted job.
// Returns (nil, nil) when there is no pending work.
func (p *ScanJobProcessor) ProcessNext(ctx context.Context) (*model.ScanJob, error) {
	job, err := p.jobs.ClaimNext(ctx, p.workerID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	ctx = tracectx.Extract(ctx, job.TraceParent, job.TraceState)
	ctx, span := otel.Tracer("github.com/scovl/ollanta/application/ingest").Start(ctx, "scan_job.process")
	defer span.End()

	var req IngestRequest
	if err := json.Unmarshal(job.Payload, &req); err != nil {
		return p.failJob(ctx, span, job, "decode payload: "+err.Error()), err
	}

	result, err := p.ingest.Ingest(ctx, &req)
	if err != nil {
		return p.failJob(ctx, span, job, err.Error()), err
	}

	if err := p.jobs.MarkCompleted(ctx, job.ID, result.ScanID); err != nil {
		span.RecordError(err)
		return job, fmt.Errorf("mark completed: %w", err)
	}

	job.Status = model.ScanJobStatusCompleted
	job.ScanID = &result.ScanID
	job.LastError = ""
	return job, nil
}

func (p *ScanJobProcessor) failJob(ctx context.Context, span trace.Span, job *model.ScanJob, reason string) *model.ScanJob {
	span.RecordError(errors.New(reason))
	markErr := p.jobs.MarkFailed(ctx, job.ID, reason)
	if markErr != nil {
		slog.Error("mark job failed", "job_id", job.ID, "error", markErr)
	}
	job.Status = model.ScanJobStatusFailed
	job.LastError = reason
	return job
}
