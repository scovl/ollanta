package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// NewCodePeriodHandler handles new code period API endpoints.
type NewCodePeriodHandler struct {
	periods  *postgres.NewCodePeriodRepository
	projects *postgres.ProjectRepository
}

// NewNewCodePeriodHandler creates a NewCodePeriodHandler.
func NewNewCodePeriodHandler(periods *postgres.NewCodePeriodRepository, projects *postgres.ProjectRepository) *NewCodePeriodHandler {
	return &NewCodePeriodHandler{periods: periods, projects: projects}
}

// GetGlobal handles GET /api/v1/new-code-periods/global
// @Summary Get global new code period
// @Description Returns the global new code period configuration
// @Tags new-code-periods
// @Produce json
// @Success 200 {object} postgres.NewCodePeriod
// @Router /api/v1/new-code-periods/global [get]
func (h *NewCodePeriodHandler) GetGlobal(w http.ResponseWriter, r *http.Request) {
	ncp, err := h.periods.GetGlobal(r.Context())
	if handleNotFound(w, err, "not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, ncp)
}

// SetGlobal handles PUT /api/v1/new-code-periods/global
// @Summary Set global new code period
// @Description Update the global new code period configuration
// @Tags new-code-periods
// @Accept json
// @Param body body object{strategy=string,value=string} true "New code period data"
// @Success 204
// @Router /api/v1/new-code-periods/global [put]
func (h *NewCodePeriodHandler) SetGlobal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Strategy string `json:"strategy"`
		Value    string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.periods.SetGlobal(r.Context(), req.Strategy, req.Value); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetForProject handles GET /api/v1/projects/{key}/new-code-period
// @Summary Get project new code period
// @Description Returns the new code period for a project
// @Tags new-code-periods
// @Produce json
// @Param key path string true "Project key"
// @Success 200 {object} postgres.NewCodePeriod
// @Router /api/v1/projects/{key}/new-code-period [get]
func (h *NewCodePeriodHandler) GetForProject(w http.ResponseWriter, r *http.Request) {
	project, err := h.resolveProject(r)
	if err != nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	ncp, err := h.periods.GetForProject(r.Context(), project.ID)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonOK(w, http.StatusOK, map[string]any{"strategy": "", "value": "", "scope": "inherited"})
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, ncp)
}

// SetForProject handles PUT /api/v1/projects/{key}/new-code-period
// @Summary Set project new code period
// @Description Update the new code period for a project
// @Tags new-code-periods
// @Accept json
// @Param key path string true "Project key"
// @Param body body object{strategy=string,value=string} true "New code period data"
// @Success 204
// @Router /api/v1/projects/{key}/new-code-period [put]
func (h *NewCodePeriodHandler) SetForProject(w http.ResponseWriter, r *http.Request) {
	project, err := h.resolveProject(r)
	if err != nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	var req struct {
		Strategy string `json:"strategy"`
		Value    string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.periods.SetForProject(r.Context(), project.ID, req.Strategy, req.Value); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteForProject handles DELETE /api/v1/projects/{key}/new-code-period
// @Summary Delete project new code period
// @Description Remove the project-level new code period override
// @Tags new-code-periods
// @Param key path string true "Project key"
// @Success 204
// @Router /api/v1/projects/{key}/new-code-period [delete]
func (h *NewCodePeriodHandler) DeleteForProject(w http.ResponseWriter, r *http.Request) {
	project, err := h.resolveProject(r)
	if err != nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}
	if err := h.periods.DeleteForProject(r.Context(), project.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NewCodePeriodHandler) resolveProject(r *http.Request) (*postgres.Project, error) {
	key := routeParam(r, "key")
	return h.projects.GetByKey(r.Context(), key)
}
