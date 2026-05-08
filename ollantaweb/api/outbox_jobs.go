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
// @Summary List index jobs
// @Description Returns paginated index jobs
// @Tags admin
// @Produce json
// @Param status query string false "Status filter"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} outboxJobListResponse
// @Router /api/v1/admin/index-jobs [get]
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
// @Summary Retry index job
// @Description Retry a failed index job
// @Tags admin
// @Produce json
// @Param id path int true "Job ID"
// @Success 202 {object} idStatusResponse
// @Router /api/v1/admin/index-jobs/{id}/retry [post]
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
// @Summary List webhook jobs
// @Description Returns paginated webhook jobs
// @Tags admin
// @Produce json
// @Param status query string false "Status filter"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} outboxJobListResponse
// @Router /api/v1/admin/webhook-jobs [get]
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
// @Summary Retry webhook job
// @Description Retry a failed webhook job
// @Tags admin
// @Produce json
// @Param id path int true "Job ID"
// @Success 202 {object} idStatusResponse
// @Router /api/v1/admin/webhook-jobs/{id}/retry [post]
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
