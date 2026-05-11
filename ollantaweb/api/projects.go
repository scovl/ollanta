package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// ProjectsHandler handles project-related endpoints.
type ProjectsHandler struct {
	repo *postgres.ProjectRepository
}

// Create handles POST /api/v1/projects — upsert a project by key.
// @Summary Create project
// @Description Create or update a project
// @Tags projects
// @Accept json
// @Produce json
// @Param body body postgres.Project true "Project data"
// @Success 201 {object} postgres.Project
// @Router /api/v1/projects [post]
func (h *ProjectsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p postgres.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if p.Key == "" {
		jsonError(w, http.StatusBadRequest, "key is required")
		return
	}
	if err := h.repo.Upsert(r.Context(), &p); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusCreated, &p)
}

// Get handles GET /api/v1/projects/{key}.
// @Summary Get project
// @Description Get a project by key
// @Tags projects
// @Produce json
// @Param key path string true "Project key"
// @Success 200 {object} postgres.Project
// @Router /api/v1/projects/{key} [get]
func (h *ProjectsHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := routeParam(r, "key")
	p, err := h.repo.GetByKey(r.Context(), key)
	if handleNotFound(w, err, projectNotFoundMessage) {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, p)
}

// List handles GET /api/v1/projects?limit=20&offset=0.
// @Summary List projects
// @Description Returns paginated list of projects
// @Tags projects
// @Produce json
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset"
// @Success 200 {object} projectListResponse
// @Router /api/v1/projects [get]
func (h *ProjectsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	projects, total, err := h.repo.List(r.Context(), limit, offset)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]interface{}{
		"items":  projects,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Update handles PUT /api/v1/projects/{key}.
// @Summary Update project
// @Description Update project metadata
// @Tags projects
// @Accept json
// @Produce json
// @Param key path string true "Project key"
// @Param body body object{name=string,description=string,main_branch=string,tags=[]string} true "Project data"
// @Success 200 {object} postgres.Project
// @Router /api/v1/projects/{key} [put]
func (h *ProjectsHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := routeParam(r, "key")
	p, err := h.repo.GetByKey(r.Context(), key)
	if handleNotFound(w, err, projectNotFoundMessage) {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var req struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		MainBranch  *string  `json:"main_branch"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.MainBranch != nil {
		p.MainBranch = *req.MainBranch
	}
	if req.Tags != nil {
		p.Tags = req.Tags
	}
	if err := h.repo.Upsert(r.Context(), p); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, p)
}

// Delete handles DELETE /api/v1/projects/{key}.
// @Summary Delete project
// @Description Delete a project
// @Tags projects
// @Param key path string true "Project key"
// @Success 204
// @Router /api/v1/projects/{key} [delete]
func (h *ProjectsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	key := routeParam(r, "key")
	p, err := h.repo.GetByKey(r.Context(), key)
	if handleNotFound(w, err, projectNotFoundMessage) {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.repo.Delete(r.Context(), p.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
