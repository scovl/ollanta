package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// MeasuresHandler handles metric trend endpoints.
type MeasuresHandler struct {
	measures *postgres.MeasureRepository
	projects *postgres.ProjectRepository
}

// Trend handles GET /api/v1/projects/{key}/measures/trend
//
// Query params: metric (required), from (RFC3339), to (RFC3339)
func (h *MeasuresHandler) Trend(w http.ResponseWriter, r *http.Request) {
	key := routeParam(r, "key")
	metricKey := r.URL.Query().Get("metric")
	if metricKey == "" {
		jsonError(w, http.StatusBadRequest, "metric query param is required")
		return
	}

	from := time.Now().AddDate(0, -3, 0)
	to := time.Now()

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}

	project, err := h.projects.GetByKey(r.Context(), key)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	points, err := h.measures.Trend(r.Context(), project.ID, metricKey, from, to)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]interface{}{
		"project": key,
		"metric":  metricKey,
		"from":    from.Format(time.RFC3339),
		"to":      to.Format(time.RFC3339),
		"points":  points,
	})
}
