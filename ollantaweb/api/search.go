package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/scovl/ollanta/ollantastore/search"
)

// SearchHandler handles full-text search via a pluggable search backend.
type SearchHandler struct {
	searcher search.ISearcher
}

// Search handles GET /api/v1/search
// @Summary Search
// @Description Full-text search across issues or projects
// @Tags search
// @Produce json
// @Param q query string false "Query string"
// @Param index query string false "Index (issues|projects)"
// @Param severity query string false "Severity filter"
// @Param type query string false "Type filter"
// @Param status query string false "Status filter"
// @Param project_id query string false "Project ID filter"
// @Param rule_key query string false "Rule key filter"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset"
// @Success 200 {object} searchResponse
// @Router /api/v1/search [get]
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	req := search.SearchRequest{
		Query:  q.Get("q"),
		Filter: map[string]string{},
		Facets: []string{"severity", "type", "rule_key"},
		Sort:   []string{"created_at:desc"},
	}

	req.Limit, _ = strconv.Atoi(q.Get("limit"))
	req.Offset, _ = strconv.Atoi(q.Get("offset"))
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// Collect filter params
	filterParams := []string{"severity", "type", "status", "project_id", "rule_key"}
	for _, fp := range filterParams {
		if v := q.Get(fp); v != "" {
			req.Filter[fp] = v
		}
	}

	index := strings.ToLower(q.Get("index"))

	var (
		result *search.SearchResult
		err    error
	)
	switch index {
	case "projects":
		result, err = h.searcher.SearchProjects(r.Context(), req)
	default:
		result, err = h.searcher.SearchIssues(r.Context(), req)
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, result)
}
