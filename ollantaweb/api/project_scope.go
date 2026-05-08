package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// ProjectScopeHandler exposes branch catalogs, project information, and read-only code browsing.
type ProjectScopeHandler struct {
	projects  *postgres.ProjectRepository
	scans     *postgres.ScanRepository
	snapshots *postgres.CodeSnapshotRepository
	issues    *postgres.IssueRepository
}

// Branches handles GET /api/v1/projects/{key}/branches.
// @Summary List branches
// @Description Returns branches for a project
// @Tags project-scope
// @Produce json
// @Param key path string true "Project key"
// @Success 200 {object} branchesResponse
// @Router /api/v1/projects/{key}/branches [get]
func (h *ProjectScopeHandler) Branches(w http.ResponseWriter, r *http.Request) {
	project, err := h.projects.GetByKey(r.Context(), routeParam(r, "key"))
	if handleNotFound(w, err, "project not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defaultBranch, _, err := h.scans.ResolveDefaultBranch(r.Context(), project.ID, project.MainBranch)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := h.scans.ListBranches(r.Context(), project.ID, defaultBranch)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{
		"default_branch": defaultBranch,
		"items":          items,
	})
}

// PullRequests handles GET /api/v1/projects/{key}/pull-requests.
// @Summary List pull requests
// @Description Returns pull requests for a project
// @Tags project-scope
// @Produce json
// @Param key path string true "Project key"
// @Success 200 {object} pullRequestsResponse
// @Router /api/v1/projects/{key}/pull-requests [get]
func (h *ProjectScopeHandler) PullRequests(w http.ResponseWriter, r *http.Request) {
	project, err := h.projects.GetByKey(r.Context(), routeParam(r, "key"))
	if handleNotFound(w, err, "project not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items, err := h.scans.ListPullRequests(r.Context(), project.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": items})
}

// Information handles GET /api/v1/projects/{key}/information.
// @Summary Project information
// @Description Returns project scope information and latest scan
// @Tags project-scope
// @Produce json
// @Param key path string true "Project key"
// @Param branch query string false "Branch"
// @Param pull_request query string false "Pull request"
// @Success 200 {object} projectInformationResponse
// @Router /api/v1/projects/{key}/information [get]
func (h *ProjectScopeHandler) Information(w http.ResponseWriter, r *http.Request) {
	requested, err := parseScopeQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	resolved, err := resolveProjectScope(r.Context(), h.projects, h.scans, routeParam(r, "key"), requested)
	if handleNotFound(w, err, "project not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	latestScan, err := h.scans.GetLatestInScope(r.Context(), resolved.Project.ID, resolved.Scope, resolved.DefaultBranch)
	if err != nil && !errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var snapshot *postgres.CodeSnapshotScope
	if h.snapshots != nil {
		snapshot, err = h.snapshots.GetScope(r.Context(), resolved.Project.ID, resolved.Scope.Type, resolved.Scope.Key())
		if err != nil && !errors.Is(err, postgres.ErrNotFound) {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if errors.Is(err, postgres.ErrNotFound) {
			snapshot = nil
		}
	}

	metrics := map[string]any{}
	if latestScan != nil {
		metrics["files"] = latestScan.TotalFiles
		metrics["lines"] = latestScan.TotalLines
		metrics["ncloc"] = latestScan.TotalNcloc
		metrics["issues"] = latestScan.TotalIssues
	}

	jsonOK(w, http.StatusOK, map[string]any{
		"project":       resolved.Project,
		"scope":         toScopeResponse(resolved),
		"latest_scan":   latestScan,
		"code_snapshot": snapshot,
		"measures":      metrics,
	})
}

// CodeTree handles GET /api/v1/projects/{key}/code/tree.
// @Summary Code tree
// @Description Returns the file tree for a project scope
// @Tags project-scope
// @Produce json
// @Param key path string true "Project key"
// @Param branch query string false "Branch"
// @Param pull_request query string false "Pull request"
// @Success 200 {object} codeTreeResponse
// @Router /api/v1/projects/{key}/code/tree [get]
func (h *ProjectScopeHandler) CodeTree(w http.ResponseWriter, r *http.Request) {
	requested, err := parseScopeQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	resolved, err := resolveProjectScope(r.Context(), h.projects, h.scans, routeParam(r, "key"), requested)
	if handleNotFound(w, err, "project not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	snapshot, err := h.snapshots.GetScope(r.Context(), resolved.Project.ID, resolved.Scope.Type, resolved.Scope.Key())
	if errors.Is(err, postgres.ErrNotFound) {
		jsonOK(w, http.StatusOK, map[string]any{
			"scope":        toScopeResponse(resolved),
			"code_snapshot": nil,
			"items":        []any{},
		})
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	files, err := h.snapshots.ListFiles(r.Context(), resolved.Project.ID, resolved.Scope.Type, resolved.Scope.Key())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]*postgres.CodeSnapshotFile, 0, len(files))
	for _, file := range files {
		copyFile := *file
		copyFile.Content = ""
		items = append(items, &copyFile)
	}
	jsonOK(w, http.StatusOK, map[string]any{
		"scope":         toScopeResponse(resolved),
		"code_snapshot": snapshot,
		"items":         items,
	})
}

// CodeFile handles GET /api/v1/projects/{key}/code/file?path=... .
// @Summary Code file
// @Description Returns a single file with issues for a project scope
// @Tags project-scope
// @Produce json
// @Param key path string true "Project key"
// @Param path query string true "File path"
// @Param branch query string false "Branch"
// @Param pull_request query string false "Pull request"
// @Success 200 {object} codeFileResponse
// @Router /api/v1/projects/{key}/code/file [get]
func (h *ProjectScopeHandler) CodeFile(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		jsonError(w, http.StatusBadRequest, "path query param is required")
		return
	}
	requested, err := parseScopeQuery(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	resolved, err := resolveProjectScope(r.Context(), h.projects, h.scans, routeParam(r, "key"), requested)
	if handleNotFound(w, err, "project not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	file, err := h.snapshots.GetFile(r.Context(), resolved.Project.ID, resolved.Scope.Type, resolved.Scope.Key(), path)
	if handleNotFound(w, err, "code file not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var issues []*postgres.IssueRow
	latestScan, err := h.scans.GetLatestInScope(r.Context(), resolved.Project.ID, resolved.Scope, resolved.DefaultBranch)
	if err != nil && !errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if latestScan != nil {
		projectID := resolved.Project.ID
		scanID := latestScan.ID
		issues, _, err = h.issues.Query(r.Context(), postgres.IssueFilter{
			ProjectID: &projectID,
			ScanID:    &scanID,
			FilePath:  &path,
			Limit:     1000,
		})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if issues == nil {
		issues = []*postgres.IssueRow{}
	}

	jsonOK(w, http.StatusOK, map[string]any{
		"scope": toScopeResponse(resolved),
		"file":  file,
		"issues": issues,
	})
}