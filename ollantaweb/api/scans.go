package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/ingest"
)

// ScansHandler handles scan-related endpoints.
type ScansHandler struct {
	scans    *postgres.ScanRepository
	projects *postgres.ProjectRepository
	jobs     *ingest.ScanJobService
}

// Ingest handles POST /api/v1/scans — receives a report.json payload and enqueues durable processing.
func (h *ScansHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	var req ingest.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Metadata.ProjectKey == "" {
		jsonError(w, http.StatusBadRequest, "project_key is required")
		return
	}

	result, err := h.jobs.Submit(r.Context(), &req)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusAccepted, result)
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
	key := routeParam(r, "key")
	project, err := h.projects.GetByKey(r.Context(), key)
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

	scans, total, err := h.scans.ListByProject(r.Context(), project.ID, limit, offset)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]interface{}{
		"items":  scans,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Latest handles GET /api/v1/projects/{key}/scans/latest.
func (h *ScansHandler) Latest(w http.ResponseWriter, r *http.Request) {
	key := routeParam(r, "key")
	project, err := h.projects.GetByKey(r.Context(), key)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	scan, err := h.scans.GetLatest(r.Context(), project.ID)
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
