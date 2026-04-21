package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
)

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

	job := &model.ScanJob{
		ProjectKey: req.Metadata.ProjectKey,
		Status:     model.ScanJobStatusAccepted,
		Payload:    payload,
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create scan job: %w", err)
	}
	return job, nil
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

	var req IngestRequest
	if err := json.Unmarshal(job.Payload, &req); err != nil {
		markErr := p.jobs.MarkFailed(ctx, job.ID, "decode payload: "+err.Error())
		if markErr != nil {
			return job, fmt.Errorf("decode payload: %v; mark failed: %w", err, markErr)
		}
		job.Status = model.ScanJobStatusFailed
		job.LastError = "decode payload: " + err.Error()
		return job, err
	}

	result, err := p.ingest.Ingest(ctx, &req)
	if err != nil {
		markErr := p.jobs.MarkFailed(ctx, job.ID, err.Error())
		if markErr != nil {
			return job, fmt.Errorf("ingest job: %v; mark failed: %w", err, markErr)
		}
		job.Status = model.ScanJobStatusFailed
		job.LastError = err.Error()
		return job, err
	}

	if err := p.jobs.MarkCompleted(ctx, job.ID, result.ScanID); err != nil {
		return job, fmt.Errorf("mark completed: %w", err)
	}

	job.Status = model.ScanJobStatusCompleted
	job.ScanID = &result.ScanID
	job.LastError = ""
	return job, nil
}
