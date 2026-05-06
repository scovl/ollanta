package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/ingest"
)

type scanJobSubmitter interface {
	SubmitWithOptions(ctx context.Context, req *ingest.IngestRequest, opts ingest.ScanJobSubmitOptions) (*ingest.ScanJobSubmitResult, error)
}

// ScansHandler handles scan-related endpoints.
type ScansHandler struct {
	scans        *postgres.ScanRepository
	projects     *postgres.ProjectRepository
	jobs         scanJobSubmitter
	backpressure ingest.ScanBackpressureConfig
}

// Ingest handles POST /api/v1/scans — receives a report.json payload and enqueues durable processing.
func (h *ScansHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	body := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(body)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid gzip: "+err.Error())
			return
		}
		defer gr.Close()
		body = gr
	}
	var req ingest.IngestRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			jsonError(w, http.StatusRequestEntityTooLarge, "scan report exceeds configured request body limit")
			return
		}
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Metadata.ProjectKey == "" {
		jsonError(w, http.StatusBadRequest, "project_key is required")
		return
	}

	result, err := h.jobs.SubmitWithOptions(r.Context(), &req, ingest.ScanJobSubmitOptions{
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
		Backpressure:   h.backpressure,
	})
	if errors.Is(err, ingest.ErrScanJobIdempotencyConflict) {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}
	var backpressureErr *ingest.ScanJobBackpressureError
	if errors.As(err, &backpressureErr) {
		if backpressureErr.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(backpressureErr.RetryAfter.Seconds())))
		}
		jsonError(w, http.StatusTooManyRequests, backpressureErr.Error())
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := http.StatusAccepted
	if result.Duplicate {
		status = http.StatusOK
	}
	jsonOK(w, status, result.Job)
}

// Get handles GET /api/v1/scans/{id}.
func (h *ScansHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid scan id")
		return
	}
	scan, err := h.scans.GetByID(r.Context(), id)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "scan not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, scan)
}

// ListByProject handles GET /api/v1/projects/{key}/scans.
func (h *ScansHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	requested, err := parseScopeQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	resolved, err := resolveProjectScope(r.Context(), h.projects, h.scans, routeParam(r, "key"), requested)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	items, err := h.scans.ListByProjectInScope(r.Context(), resolved.Project.ID, resolved.Scope, resolved.DefaultBranch)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	scans := items[offset:end]
	jsonOK(w, http.StatusOK, map[string]interface{}{
		"items":  scans,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"scope":  toScopeResponse(resolved),
	})
}

// Latest handles GET /api/v1/projects/{key}/scans/latest.
func (h *ScansHandler) Latest(w http.ResponseWriter, r *http.Request) {
	requested, err := parseScopeQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	resolved, err := resolveProjectScope(r.Context(), h.projects, h.scans, routeParam(r, "key"), requested)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	scan, err := h.scans.GetLatestInScope(r.Context(), resolved.Project.ID, resolved.Scope, resolved.DefaultBranch)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "no scans for project")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, scan)
}
