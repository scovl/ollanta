package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/scovl/ollanta/adapter/secondary/postgres"
	"github.com/scovl/ollanta/domain/model"
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
	f := issueFilterFromQuery(r.URL.Query())

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

func issueFilterFromQuery(q url.Values) model.IssueFilter {
	f := model.IssueFilter{}
	if n, ok := queryInt(q, "limit"); ok {
		f.Limit = n
	}
	if n, ok := queryInt(q, "offset"); ok {
		f.Offset = n
	}
	f.ProjectID = queryInt64Ptr(q, "project_id")
	f.ScanID = queryInt64Ptr(q, "scan_id")
	f.RuleKey = queryStrPtr(q, "rule_key")
	f.Severity = queryStrPtr(q, "severity")
	f.Type = queryStrPtr(q, "type")
	f.Status = queryStrPtr(q, "status")
	f.FilePath = queryStrPtr(q, "file")
	return f
}

func queryInt(q url.Values, key string) (int, bool) {
	if v := q.Get(key); v != "" {
		n, err := strconv.Atoi(v)
		return n, err == nil
	}
	return 0, false
}

func queryInt64Ptr(q url.Values, key string) *int64 {
	if v := q.Get(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return &n
		}
	}
	return nil
}

func queryStrPtr(q url.Values, key string) *string {
	if v := q.Get(key); v != "" {
		return &v
	}
	return nil
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
	if errors.Is(err, model.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, facets)
}

// Transition handles POST /api/v1/issues/{id}/transition.
// Allowed resolutions: false_positive, wont_fix, confirmed, fixed, "" (reopen).
func (h *IssuesHandler) Transition(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	var req struct {
		Resolution string `json:"resolution"`
		Comment    string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}

	// Determine target status from resolution.
	toStatus := "closed"
	if req.Resolution == "" {
		toStatus = "open" // reopen
	}

	user := UserFromContext(r.Context())
	var userID int64
	if user != nil {
		userID = user.ID
	}

	if err := h.issues.Transition(r.Context(), id, userID, toStatus, req.Resolution, req.Comment); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			jsonError(w, http.StatusNotFound, "issue not found")
			return
		}
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
