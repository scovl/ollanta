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
