// Package search defines the search port interfaces and types that decouple the
// application from any concrete search backend (ZincSearch, Postgres FTS, etc.).
package search

import "context"

const (
	indexIssues   = "issues"
	indexProjects = "projects"
)

// SearchRequest describes a full-text search query with optional filters and facets.
type SearchRequest struct {
	Query  string
	Filter map[string]string
	Facets []string
	Sort   []string
	Limit  int
	Offset int
}

// SearchResult is the unified response from a search backend.
type SearchResult struct {
	Hits              []map[string]interface{}  `json:"hits"`
	TotalHits         int64                     `json:"total_hits"`
	FacetDistribution map[string]map[string]int `json:"facet_distribution"`
	ProcessingTimeMs  int64                     `json:"processing_time_ms"`
}

// IndexIssue is the search-index representation of an issue.
type IndexIssue struct {
	ID            int64    `json:"id"`
	ScanID        int64    `json:"scan_id"`
	ProjectID     int64    `json:"project_id"`
	ProjectKey    string   `json:"project_key"`
	RuleKey       string   `json:"rule_key"`
	ComponentPath string   `json:"component_path"`
	Message       string   `json:"message"`
	Type          string   `json:"type"`
	Severity      string   `json:"severity"`
	Status        string   `json:"status"`
	Line          int      `json:"line"`
	Tags          []string `json:"tags,omitempty"`
	CreatedAt     string   `json:"created_at"`
}

// IndexProject is the search-index representation of a project.
type IndexProject struct {
	ID        int64  `json:"id"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// ISearcher executes full-text queries against the search index.
type ISearcher interface {
	SearchIssues(ctx context.Context, req SearchRequest) (*SearchResult, error)
	SearchProjects(ctx context.Context, req SearchRequest) (*SearchResult, error)
}

// IIndexer writes data into the search index and manages its lifecycle.
type IIndexer interface {
	Health(ctx context.Context) error
	ConfigureIndexes(ctx context.Context) error
	IndexIssues(ctx context.Context, projectKey string, issues []IndexIssue) error
	IndexProject(ctx context.Context, p IndexProject) error
	DeleteScanIssues(ctx context.Context, scanID int64) error
}
