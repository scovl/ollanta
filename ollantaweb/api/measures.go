package api

import (
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
// @Summary Measure trend
// @Description Returns historical measure values for a project
// @Tags measures
// @Produce json
// @Param key path string true "Project key"
// @Param metric query string true "Metric key"
// @Param from query string false "From date (RFC3339)"
// @Param to query string false "To date (RFC3339)"
// @Param component query string false "Component path"
// @Success 200 {object} measureTrendResponse
// @Router /api/v1/projects/{key}/measures/trend [get]
func (h *MeasuresHandler) Trend(w http.ResponseWriter, r *http.Request) {
	key := routeParam(r, "key")
	metricKey := r.URL.Query().Get("metric")
	if metricKey == "" {
		jsonError(w, http.StatusBadRequest, "metric query param is required")
		return
	}
	componentPath := r.URL.Query().Get("component")

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
	if handleNotFound(w, err, "project not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var points []postgres.TrendPoint
	if componentPath != "" {
		points, err = h.measures.TrendForComponent(r.Context(), project.ID, metricKey, componentPath, from, to)
	} else {
		points, err = h.measures.Trend(r.Context(), project.ID, metricKey, from, to)
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response := map[string]interface{}{
		"project": key,
		"metric":  metricKey,
		"from":    from.Format(time.RFC3339),
		"to":      to.Format(time.RFC3339),
		"points":  points,
	}
	if componentPath != "" {
		response["component"] = componentPath
	}
	jsonOK(w, http.StatusOK, response)
}
