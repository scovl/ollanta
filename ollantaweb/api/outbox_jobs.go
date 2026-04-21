package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// OutboxJobsHandler exposes inspection and retry endpoints for durable side effects.
type OutboxJobsHandler struct {
	indexJobs   *postgres.IndexJobRepository
	webhookJobs *postgres.WebhookJobRepository
}

// ListIndexJobs handles GET /api/v1/admin/index-jobs.
func (h *OutboxJobsHandler) ListIndexJobs(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.indexJobs.List(r.Context(), status, limit, offset)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// RetryIndexJob handles POST /api/v1/admin/index-jobs/{id}/retry.
func (h *OutboxJobsHandler) RetryIndexJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid index job id")
		return
	}
	if err := h.indexJobs.Retry(r.Context(), id); errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "index job not found")
		return
	} else if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusAccepted, map[string]any{"id": id, "status": "accepted"})
}

// ListWebhookJobs handles GET /api/v1/admin/webhook-jobs.
func (h *OutboxJobsHandler) ListWebhookJobs(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, total, err := h.webhookJobs.List(r.Context(), status, limit, offset)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// RetryWebhookJob handles POST /api/v1/admin/webhook-jobs/{id}/retry.
func (h *OutboxJobsHandler) RetryWebhookJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid webhook job id")
		return
	}
	if err := h.webhookJobs.Retry(r.Context(), id); errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "webhook job not found")
		return
	} else if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusAccepted, map[string]any{"id": id, "status": "accepted"})
}
