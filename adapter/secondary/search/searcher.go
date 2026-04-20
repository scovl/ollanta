package search

import (
	"context"
	"fmt"
	"strings"

	meilisearch "github.com/meilisearch/meilisearch-go"
)

// SearchRequest describes a full-text search query with optional filters and facets.
type SearchRequest struct {
	Query  string
	Filter map[string]string // field → value; joined as AND filters
	Facets []string
	Sort   []string
	Limit  int
	Offset int
}

// SearchResult is the unified response from a Meilisearch search call.
type SearchResult struct {
	Hits              []map[string]interface{}  `json:"hits"`
	TotalHits         int64                     `json:"total_hits"`
	FacetDistribution map[string]map[string]int `json:"facet_distribution"`
	ProcessingTimeMs  int64                     `json:"processing_time_ms"`
}

// MeilisearchSearcher executes full-text queries against Meilisearch indexes.
type MeilisearchSearcher struct {
	client meilisearch.ServiceManager
}

// NewMeilisearchSearcher creates a searcher connected to the given Meilisearch instance.
func NewMeilisearchSearcher(cfg IndexerConfig) (*MeilisearchSearcher, error) {
	client := meilisearch.New(cfg.Host, meilisearch.WithAPIKey(cfg.APIKey))
	return &MeilisearchSearcher{client: client}, nil
}

// SearchIssues executes a query against the issues index.
func (s *MeilisearchSearcher) SearchIssues(_ context.Context, req SearchRequest) (*SearchResult, error) {
	return s.search(indexIssues, req)
}

// SearchProjects executes a query against the projects index.
func (s *MeilisearchSearcher) SearchProjects(_ context.Context, req SearchRequest) (*SearchResult, error) {
	return s.search(indexProjects, req)
}

// buildFilter joins a field→value map into a Meilisearch AND filter string.
func buildFilter(f map[string]string) string {
	parts := make([]string, 0, len(f))
	for k, v := range f {
		parts = append(parts, fmt.Sprintf("%s = %q", k, v))
	}
	return strings.Join(parts, " AND ")
}

// convertHits casts each raw Meilisearch hit to map[string]interface{}.
func convertHits(raw []interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, len(raw))
	for i, h := range raw {
		if m, ok := h.(map[string]interface{}); ok {
			out[i] = m
		} else {
			out[i] = map[string]interface{}{"_raw": h}
		}
	}
	return out
}

// convertFacetDistribution converts the untyped Meilisearch facet payload to
// a typed map[facet]map[value]count.
func convertFacetDistribution(raw interface{}) map[string]map[string]int {
	fd, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]map[string]int, len(fd))
	for facetName, facetVals := range fd {
		vals, ok := facetVals.(map[string]interface{})
		if !ok {
			continue
		}
		m := make(map[string]int, len(vals))
		for k, v := range vals {
			if n, ok := v.(float64); ok {
				m[k] = int(n)
			}
		}
		result[facetName] = m
	}
	return result
}

func (s *MeilisearchSearcher) search(index string, req SearchRequest) (*SearchResult, error) {
	limit := int64(req.Limit)
	if limit <= 0 {
		limit = 20
	}

	sr := &meilisearch.SearchRequest{
		Limit:  limit,
		Offset: int64(req.Offset),
	}

	if len(req.Filter) > 0 {
		sr.Filter = buildFilter(req.Filter)
	}

	if len(req.Facets) > 0 {
		sr.Facets = req.Facets
	}

	if len(req.Sort) > 0 {
		sr.Sort = req.Sort
	}

	resp, err := s.client.Index(index).Search(req.Query, sr)
	if err != nil {
		return nil, fmt.Errorf("meilisearch search %s: %w", index, err)
	}

	return &SearchResult{
		TotalHits:         resp.EstimatedTotalHits,
		ProcessingTimeMs:  resp.ProcessingTimeMs,
		Hits:              convertHits(resp.Hits),
		FacetDistribution: convertFacetDistribution(resp.FacetDistribution),
	}, nil
}
