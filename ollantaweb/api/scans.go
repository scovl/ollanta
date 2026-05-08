package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/ingest"
)

type scanJobSubmitter interface {
	SubmitWithOptions(ctx context.Context, req *ingest.IngestRequest, opts ingest.ScanJobSubmitOptions) (*ingest.ScanJobSubmitResult, error)
}

type scanDetailResponse struct {
	*postgres.Scan
	TestSignals json.RawMessage `json:"test_signals,omitempty"`
}

// ScansHandler handles scan-related endpoints.
type ScansHandler struct {
	scans        *postgres.ScanRepository
	projects     *postgres.ProjectRepository
	scanJobs     *postgres.ScanJobRepository
	jobs         scanJobSubmitter
	backpressure ingest.ScanBackpressureConfig
}

// Ingest handles POST /api/v1/scans — receives a report.json payload and enqueues durable processing.
// @Summary Ingest scan
// @Description Receive a scan report and enqueue processing
// @Tags scans
// @Accept json
// @Produce json
// @Param body body ingest.IngestRequest true "Scan report"
// @Success 202 {object} postgres.ScanJob
// @Router /api/v1/scans [post]
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
// @Summary Get scan
// @Description Get a scan by ID
// @Tags scans
// @Produce json
// @Param id path int true "Scan ID"
// @Success 200 {object} scanDetailResponse
// @Router /api/v1/scans/{id} [get]
func (h *ScansHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid scan id")
		return
	}
	scan, err := h.scans.GetByID(r.Context(), id)
	if handleNotFound(w, err, "scan not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response := scanDetailResponse{Scan: scan}
	response.TestSignals = h.attachTestSignals(r.Context(), id)
	jsonOK(w, http.StatusOK, response)
}

func (h *ScansHandler) attachTestSignals(ctx context.Context, scanID int64) json.RawMessage {
	if h.scanJobs == nil {
		return nil
	}
	job, err := h.scanJobs.GetByScanID(ctx, scanID)
	if err != nil || job == nil {
		return nil
	}
	var req ingest.IngestRequest
	if err := json.Unmarshal(job.Payload, &req); err != nil {
		slog.Warn("scan test_signals unmarshal failed", "scan_id", scanID, "error", err)
		return nil
	}
	return req.TestSignals
}

type survivedMutantItem struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Mutator     string `json:"mutator"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	EndLine     int    `json:"end_line,omitempty"`
	Description string `json:"description,omitempty"`
	Module      string `json:"module"`
	ChangedCode bool   `json:"changed_code,omitempty"`
}

// SurvivedMutants handles GET /api/v1/scans/{id}/survived-mutants.
// @Summary List survived mutants
// @Description Returns mutation testing survived mutants for a scan
// @Tags scans
// @Produce json
// @Param id path int true "Scan ID"
// @Success 200 {object} survivedMutantsResponse
// @Router /api/v1/scans/{id}/survived-mutants [get]
func (h *ScansHandler) SurvivedMutants(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid scan id")
		return
	}
	_, err = h.scans.GetByID(r.Context(), id)
	if handleNotFound(w, err, "scan not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	mutants := h.fetchSurvivedMutants(r.Context(), id)
	jsonOK(w, http.StatusOK, map[string]interface{}{
		"scan_id":  id,
		"mutants":  mutants,
		"total":    len(mutants),
	})
}

func (h *ScansHandler) fetchSurvivedMutants(ctx context.Context, scanID int64) []survivedMutantItem {
	if h.scanJobs == nil {
		return nil
	}
	job, err := h.scanJobs.GetByScanID(ctx, scanID)
	if err != nil || job == nil {
		return nil
	}
	var req ingest.IngestRequest
	if err := json.Unmarshal(job.Payload, &req); err != nil {
		slog.Warn("survived_mutants payload unmarshal failed", "scan_id", scanID, "error", err)
		return nil
	}
	if req.TestSignals == nil {
		return nil
	}
	var signals struct {
		Modules []struct {
			Name     string `json:"name"`
			Mutation struct {
				SurvivedMutants []survivedMutantItem `json:"survived_mutants"`
			} `json:"mutation"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(req.TestSignals, &signals); err != nil {
		slog.Warn("survived_mutants test_signals unmarshal failed", "scan_id", scanID, "error", err)
		return nil
	}
	var mutants []survivedMutantItem
	for _, module := range signals.Modules {
		for _, mutant := range module.Mutation.SurvivedMutants {
			mutant.Module = module.Name
			mutants = append(mutants, mutant)
		}
	}
	return mutants
}

// ListByProject handles GET /api/v1/projects/{key}/scans.
// @Summary List project scans
// @Description Returns paginated scans for a project
// @Tags scans
// @Produce json
// @Param key path string true "Project key"
// @Param branch query string false "Branch"
// @Param pull_request query string false "Pull request"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset"
// @Success 200 {object} scanListResponse
// @Router /api/v1/projects/{key}/scans [get]
func (h *ScansHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	requested, err := parseScopeQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	resolved, err := resolveProjectScope(r.Context(), h.projects, h.scans, routeParam(r, "key"), requested)
	if handleNotFound(w, err, "project not found") {
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
// @Summary Latest scan
// @Description Returns the latest scan for a project scope
// @Tags scans
// @Produce json
// @Param key path string true "Project key"
// @Param branch query string false "Branch"
// @Param pull_request query string false "Pull request"
// @Success 200 {object} postgres.Scan
// @Router /api/v1/projects/{key}/scans/latest [get]
func (h *ScansHandler) Latest(w http.ResponseWriter, r *http.Request) {
	requested, err := parseScopeQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	resolved, err := resolveProjectScope(r.Context(), h.projects, h.scans, routeParam(r, "key"), requested)
	if handleNotFound(w, err, "project not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	scan, err := h.scans.GetLatestInScope(r.Context(), resolved.Project.ID, resolved.Scope, resolved.DefaultBranch)
	if handleNotFound(w, err, "no scans for project") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, scan)
}
