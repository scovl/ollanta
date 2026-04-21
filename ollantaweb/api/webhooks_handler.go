package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/webhook"
)

// WebhooksHandler handles webhook API endpoints.
type WebhooksHandler struct {
	webhooks   *postgres.WebhookRepository
	projects   *postgres.ProjectRepository
	dispatcher *webhook.Dispatcher
}

// NewWebhooksHandler creates a WebhooksHandler.
func NewWebhooksHandler(
	webhooks *postgres.WebhookRepository,
	projects *postgres.ProjectRepository,
	dispatcher *webhook.Dispatcher,
) *WebhooksHandler {
	return &WebhooksHandler{webhooks: webhooks, projects: projects, dispatcher: dispatcher}
}

// List handles GET /api/v1/webhooks?project_key=
func (h *WebhooksHandler) List(w http.ResponseWriter, r *http.Request) {
	var projectID int64
	if key := r.URL.Query().Get("project_key"); key != "" {
		p, err := h.projects.GetByKey(r.Context(), key)
		if errors.Is(err, postgres.ErrNotFound) {
			jsonError(w, http.StatusNotFound, "project not found")
			return
		}
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		projectID = p.ID
	}
	list, err := h.webhooks.List(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// Create handles POST /api/v1/webhooks
func (h *WebhooksHandler) Create(w http.ResponseWriter, r *http.Request) {
	var wh postgres.Webhook
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	wh.Enabled = true
	if err := h.webhooks.Create(r.Context(), &wh); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusCreated, wh)
}

// Update handles PUT /api/v1/webhooks/{id}
func (h *WebhooksHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var wh postgres.Webhook
	if err := json.NewDecoder(r.Body).Decode(&wh); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	wh.ID = id
	if err := h.webhooks.Update(r.Context(), &wh); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, wh)
}

// Delete handles DELETE /api/v1/webhooks/{id}
func (h *WebhooksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.webhooks.Delete(r.Context(), id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Deliveries handles GET /api/v1/webhooks/{id}/deliveries
func (h *WebhooksHandler) Deliveries(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	list, err := h.webhooks.ListDeliveries(r.Context(), id, limit)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// Test handles POST /api/v1/webhooks/{id}/test — fires a test event.
func (h *WebhooksHandler) Test(w http.ResponseWriter, r *http.Request) {
	if h.dispatcher == nil {
		jsonError(w, http.StatusServiceUnavailable, "webhook dispatcher is not running in the web role")
		return
	}

	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	wh, err := h.webhooks.GetByID(r.Context(), id)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "webhook not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	payload := map[string]any{"test": true, "webhook_id": wh.ID}
	h.dispatcher.Dispatch(r.Context(), 0, "test.ping", payload)
	jsonOK(w, http.StatusOK, map[string]string{"status": "queued"})
}
