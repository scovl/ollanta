package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// IssuesHandler handles issue-related endpoints.
type IssuesHandler struct {
	issues   *postgres.IssueRepository
	projects *postgres.ProjectRepository
}

// List handles GET /api/v1/issues with optional filter query params.
//
// Query params: project_id, scan_id, rule_key, severity, type, status, file, limit, offset
func (h *IssuesHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := postgres.IssueFilter{}

	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}
	if v := q.Get("project_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.ProjectID = &n
		}
	}
	if v := q.Get("scan_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.ScanID = &n
		}
	}
	if v := q.Get("rule_key"); v != "" {
		f.RuleKey = &v
	}
	if v := q.Get("severity"); v != "" {
		f.Severity = &v
	}
	if v := q.Get("type"); v != "" {
		f.Type = &v
	}
	if v := q.Get("status"); v != "" {
		f.Status = &v
	}
	if v := q.Get("file"); v != "" {
		f.FilePath = &v
	}

	issues, total, err := h.issues.Query(r.Context(), f)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]interface{}{
		"items":  issues,
		"total":  total,
		"limit":  f.Limit,
		"offset": f.Offset,
	})
}

// Facets handles GET /api/v1/issues/facets?project_id=1&scan_id=2.
func (h *IssuesHandler) Facets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var projectID, scanID int64
	if v := q.Get("project_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid project_id")
			return
		}
		projectID = n
	}
	if v := q.Get("scan_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid scan_id")
			return
		}
		scanID = n
	}

	facets, err := h.issues.Facets(r.Context(), projectID, scanID)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, facets)
}
