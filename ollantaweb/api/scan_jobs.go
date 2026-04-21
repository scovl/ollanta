package api

import (
	"errors"
	"net/http"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantaweb/ingest"
)

// ScanJobsHandler handles durable scan-job endpoints.
type ScanJobsHandler struct {
	jobs *ingest.ScanJobService
}

// Get handles GET /api/v1/scan-jobs/{id}.
func (h *ScanJobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid scan job id")
		return
	}

	job, err := h.jobs.Get(r.Context(), id)
	if errors.Is(err, model.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "scan job not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, http.StatusOK, job)
}
