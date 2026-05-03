package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantaweb/ingest"
)

func TestScansIngestAcceptedUsesIdempotencyHeader(t *testing.T) {
	jobs := &fakeScanSubmitter{result: &ingest.ScanJobSubmitResult{Job: &model.ScanJob{ID: 1, ProjectKey: "demo", Status: model.ScanJobStatusAccepted}}}
	handler := &ScansHandler{jobs: jobs}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", strings.NewReader(`{"metadata":{"project_key":"demo"}}`))
	req.Header.Set("Idempotency-Key", "ci-run-1")
	rr := httptest.NewRecorder()

	handler.Ingest(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusAccepted)
	}
	if jobs.opts.IdempotencyKey != "ci-run-1" {
		t.Fatalf("IdempotencyKey = %q, want ci-run-1", jobs.opts.IdempotencyKey)
	}
}

func TestScansIngestDuplicateReturnsExistingJob(t *testing.T) {
	jobs := &fakeScanSubmitter{result: &ingest.ScanJobSubmitResult{Job: &model.ScanJob{ID: 42, ProjectKey: "demo", Status: model.ScanJobStatusAccepted}, Duplicate: true}}
	handler := &ScansHandler{jobs: jobs}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", strings.NewReader(`{"metadata":{"project_key":"demo"}}`))
	rr := httptest.NewRecorder()

	handler.Ingest(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), `"id":42`) {
		t.Fatalf("body = %q, want existing job id", rr.Body.String())
	}
}

func TestScansIngestIdempotencyConflictReturns409(t *testing.T) {
	jobs := &fakeScanSubmitter{err: ingest.ErrScanJobIdempotencyConflict}
	handler := &ScansHandler{jobs: jobs}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", strings.NewReader(`{"metadata":{"project_key":"demo"}}`))
	rr := httptest.NewRecorder()

	handler.Ingest(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusConflict)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

func TestScansIngestBackpressureReturns429(t *testing.T) {
	jobs := &fakeScanSubmitter{err: &ingest.ScanJobBackpressureError{Reason: "accepted scan job limit reached", RetryAfter: 15 * time.Second}}
	handler := &ScansHandler{jobs: jobs}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", strings.NewReader(`{"metadata":{"project_key":"demo"}}`))
	rr := httptest.NewRecorder()

	handler.Ingest(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusTooManyRequests)
	}
	if got := rr.Header().Get("Retry-After"); got != "15" {
		t.Fatalf("Retry-After = %q, want 15", got)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
}

type fakeScanSubmitter struct {
	result *ingest.ScanJobSubmitResult
	err    error
	req    *ingest.IngestRequest
	opts   ingest.ScanJobSubmitOptions
}

func (f *fakeScanSubmitter) SubmitWithOptions(_ context.Context, req *ingest.IngestRequest, opts ingest.ScanJobSubmitOptions) (*ingest.ScanJobSubmitResult, error) {
	f.req = req
	f.opts = opts
	if f.err != nil {
		return nil, f.err
	}
	if f.result == nil {
		return nil, errors.New("missing fake result")
	}
	return f.result, nil
}
