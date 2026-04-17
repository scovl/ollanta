package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/scovl/ollanta/ollantastore/search"
)

// SearchHandler handles full-text search via Meilisearch.
type SearchHandler struct {
	searcher *search.MeilisearchSearcher
}

// Search handles GET /api/v1/search
//
// Query params: q (query string), index (issues|projects, default issues),
//
//	severity, type, project_id (filters), limit, offset
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
