package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/scovl/ollanta/application/tagging"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

// IssuesHandler handles issue-related endpoints.
type IssuesHandler struct {
	issues    *postgres.IssueRepository
	projects  *postgres.ProjectRepository
	scans     *postgres.ScanRepository
	changelog *postgres.ChangelogRepository
	tags      *tagging.Service
}

type scopedIssueSelection struct {
	projectID int64
	scanID    int64
	found     bool
}

func (h *IssuesHandler) resolveIssueProject(r *http.Request, projectID *int64, projectKey string) (*postgres.Project, error) {
	switch {
	case projectKey != "":
		return h.projects.GetByKey(r.Context(), projectKey)
	case projectID != nil:
		return h.projects.GetByID(r.Context(), *projectID)
	default:
		return nil, errors.New("project_id or project_key is required when filtering by branch or pull_request")
	}
}

func (h *IssuesHandler) resolveScopedIssueSelection(r *http.Request, projectID *int64, projectKey string) (*scopedIssueSelection, error) {
	if r.URL.Query().Get("branch") == "" && r.URL.Query().Get("pull_request") == "" {
		return nil, nil
	}
	requested, err := parseScopeQuery(r)
	if err != nil {
		return nil, err
	}
	project, err := h.resolveIssueProject(r, projectID, projectKey)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveProjectScopeForProject(r.Context(), project, h.scans, requested)
	if err != nil {
		return nil, err
	}
	scan, err := h.scans.GetLatestInScope(r.Context(), project.ID, resolved.Scope, resolved.DefaultBranch)
	if errors.Is(err, postgres.ErrNotFound) {
		return &scopedIssueSelection{projectID: project.ID, found: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return &scopedIssueSelection{projectID: project.ID, scanID: scan.ID, found: true}, nil
}

func emptyFacets() *postgres.IssueFacets {
	return &postgres.IssueFacets{
		BySeverity:         map[string]int{},
		ByType:             map[string]int{},
		ByQuality:          map[string]int{},
		ByRule:             map[string]int{},
		ByStatus:           map[string]int{},
		ByLifecycle:        map[string]int{},
		ByLanguage:         map[string]int{},
		ByEngineID:         map[string]int{},
		ByFile:             map[string]int{},
		ByDirectory:        map[string]int{},
		ByTags:             map[string]int{},
		BySecurityCategory: map[string]int{},
	}
}

func parseOptionalInt(query url.Values, key string) int {
	value := query.Get(key)
	if value == "" {
		return 0
	}
	number, _ := strconv.Atoi(value)
	return number
}

func parseOptionalInt64(query url.Values, key string) *int64 {
	value := query.Get(key)
	if value == "" {
		return nil
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil
	}
	return &number
}

func assignOptionalString(value string, target **string) {
	if value == "" {
		return
	}
	*target = &value
}

func parseIssueFilter(q url.Values) (postgres.IssueFilter, string) {
	projectKey := q.Get("project_key")
	f := postgres.IssueFilter{
		Limit:     parseOptionalInt(q, "limit"),
		Offset:    parseOptionalInt(q, "offset"),
		ProjectID: parseOptionalInt64(q, "project_id"),
		ScanID:    parseOptionalInt64(q, "scan_id"),
	}
	assignOptionalString(q.Get("rule_key"), &f.RuleKey)
	assignOptionalString(q.Get("severity"), &f.Severity)
	assignOptionalString(q.Get("type"), &f.Type)
	assignOptionalString(q.Get("quality"), &f.QualityDomain)
	assignOptionalString(q.Get("status"), &f.Status)
	assignOptionalString(q.Get("tracking_state"), &f.TrackingState)
	assignOptionalString(q.Get("language"), &f.Language)
	assignOptionalString(q.Get("tag"), &f.Tag)
	assignOptionalString(q.Get("security_category"), &f.SecurityCategory)
	assignOptionalString(q.Get("directory"), &f.Directory)
	assignOptionalString(q.Get("file"), &f.FilePath)
	assignOptionalString(q.Get("engine_id"), &f.EngineID)
	return f, projectKey
}

func writeEmptyIssuesResponse(w http.ResponseWriter, filter postgres.IssueFilter) {
	jsonOK(w, http.StatusOK, map[string]interface{}{
		"items":  []*postgres.IssueRow{},
		"total":  0,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// List handles GET /api/v1/issues with optional filter query params.
// @Summary List issues
// @Description Returns paginated issues with optional filters
// @Tags issues
// @Produce json
// @Param project_id query int false "Project ID"
// @Param scan_id query int false "Scan ID"
// @Param rule_key query string false "Rule key"
// @Param severity query string false "Severity"
// @Param type query string false "Issue type"
// @Param quality query string false "Quality domain"
// @Param status query string false "Status"
// @Param tracking_state query string false "Tracking state"
// @Param language query string false "Language"
// @Param tag query string false "Tag"
// @Param security_category query string false "Security category"
// @Param directory query string false "Directory"
// @Param file query string false "File path"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} issueListResponse
// @Router /api/v1/issues [get]
func (h *IssuesHandler) List(w http.ResponseWriter, r *http.Request) {
	f, projectKey := parseIssueFilter(r.URL.Query())
	if err := h.resolveTagFilter(r, &f); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	selection, err := h.resolveScopedIssueSelection(r, f.ProjectID, projectKey)
	if err != nil {
		if handleNotFound(w, err, projectNotFoundMessage) {
			return
		}
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if selection != nil {
		if f.ScanID != nil {
			jsonError(w, http.StatusBadRequest, "scan_id cannot be combined with branch or pull_request")
			return
		}
		if !selection.found {
			writeEmptyIssuesResponse(w, f)
			return
		}
		projectID := selection.projectID
		scanID := selection.scanID
		f.ProjectID = &projectID
		f.ScanID = &scanID
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
// @Summary Issue facets
// @Description Returns aggregated issue counts by dimension
// @Tags issues
// @Produce json
// @Param project_id query int false "Project ID"
// @Param scan_id query int false "Scan ID"
// @Success 200 {object} postgres.IssueFacets
// @Router /api/v1/issues/facets [get]
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
	selection, err := h.resolveScopedIssueSelection(r, optionalInt64(projectID), q.Get("project_key"))
	if err != nil {
		if handleNotFound(w, err, projectNotFoundMessage) {
			return
		}
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if selection != nil {
		if !selection.found {
			jsonOK(w, http.StatusOK, emptyFacets())
			return
		}
		projectID = selection.projectID
		scanID = selection.scanID
	}

	filter, _ := parseIssueFilter(q)
	if err := h.resolveTagFilter(r, &filter); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	filter.ProjectID = optionalInt64(projectID)
	filter.ScanID = optionalInt64(scanID)
	facets, err := h.issues.FacetsForFilter(r.Context(), filter)
	if handleNotFound(w, err, "not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, facets)
}

func (h *IssuesHandler) resolveTagFilter(r *http.Request, filter *postgres.IssueFilter) error {
	if h.tags == nil || filter == nil || filter.Tag == nil {
		return nil
	}
	resolved, err := h.tags.ResolveTagKey(r.Context(), *filter.Tag)
	if err == nil {
		*filter.Tag = resolved
		return nil
	}
	if errors.Is(err, postgres.ErrNotFound) {
		normalized := model.NormalizeTagKey(*filter.Tag)
		*filter.Tag = normalized
		return nil
	}
	return err
}

func optionalInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

// Transition handles POST /api/v1/issues/{id}/transition.
// Allowed resolutions: false_positive, wont_fix, confirmed, fixed, "" (reopen).
// @Summary Transition issue
// @Description Change issue status/resolution
// @Tags issues
// @Accept json
// @Param id path int true "Issue ID"
// @Param body body object{resolution=string,comment=string} true "Transition data"
// @Success 204
// @Router /api/v1/issues/{id}/transition [post]
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
		if handleNotFound(w, err, "issue not found") {
			return
		}
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Record changelog entries for the transition (SonarQube-style audit trail).
	if h.changelog != nil {
		var entries []postgres.ChangelogEntry
		entries = append(entries, postgres.ChangelogEntry{
			IssueID:  id,
			UserID:   userID,
			Field:    "status",
			OldValue: "", // unknown from here; the DB has the old value
			NewValue: toStatus,
		})
		if req.Resolution != "" {
			entries = append(entries, postgres.ChangelogEntry{
				IssueID:  id,
				UserID:   userID,
				Field:    "resolution",
				OldValue: "",
				NewValue: req.Resolution,
			})
		}
		if req.Comment != "" {
			entries = append(entries, postgres.ChangelogEntry{
				IssueID:  id,
				UserID:   userID,
				Field:    "comment",
				NewValue: req.Comment,
			})
		}
		_ = h.changelog.InsertBatch(r.Context(), entries) // best-effort
	}

	w.WriteHeader(http.StatusNoContent)
}

// Changelog handles GET /api/v1/issues/{id}/changelog.
// Returns the complete change history for an issue, most recent first.
// @Summary Issue changelog
// @Description Returns change history for an issue
// @Tags issues
// @Produce json
// @Param id path int true "Issue ID"
// @Success 200 {object} issueChangelogResponse
// @Router /api/v1/issues/{id}/changelog [get]
func (h *IssuesHandler) Changelog(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid issue id")
		return
	}

	if h.changelog == nil {
		jsonOK(w, http.StatusOK, map[string]interface{}{"items": []struct{}{}})
		return
	}

	entries, err := h.changelog.ListByIssue(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []postgres.ChangelogEntry{}
	}
	jsonOK(w, http.StatusOK, map[string]interface{}{"items": entries})
}
