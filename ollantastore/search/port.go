// port.go defines the search port interfaces that decouple the rest of the
// application from any concrete search backend (ZincSearch, Postgres FTS, etc.).
//
// Every search adapter must implement ISearcher and/or IIndexer.
// The Pipeline, Worker, Router, and Health checks depend only on these
// interfaces — never on a concrete struct — so backends can be swapped
// at startup via configuration.
package search

import (
	"context"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// Index names shared by all backends.
const (
	indexIssues   = "issues"
	indexProjects = "projects"
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

// SearchResult is the unified response from a search backend.
type SearchResult struct {
	Hits              []map[string]interface{}  `json:"hits"`
	TotalHits         int64                     `json:"total_hits"`
	FacetDistribution map[string]map[string]int `json:"facet_distribution"`
	ProcessingTimeMs  int64                     `json:"processing_time_ms"`
}

// ISearcher executes full-text queries against the search index.
type ISearcher interface {
	SearchIssues(ctx context.Context, req SearchRequest) (*SearchResult, error)
	SearchProjects(ctx context.Context, req SearchRequest) (*SearchResult, error)
}

// IIndexer writes data into the search index and manages its lifecycle.
type IIndexer interface {
	// Health returns nil when the search backend is reachable.
	Health(ctx context.Context) error

	// ConfigureIndexes ensures filterable/sortable attributes are set.
	// Implementations must be idempotent.
	ConfigureIndexes(ctx context.Context) error

	// IndexIssues upserts a batch of issues into the search index.
	IndexIssues(ctx context.Context, projectKey string, issues []postgres.IssueRow) error

	// IndexProject upserts a project document.
	IndexProject(ctx context.Context, p *postgres.Project) error

	// DeleteScanIssues removes all indexed documents for a given scan.
	DeleteScanIssues(ctx context.Context, scanID int64) error

	// ReindexAll rebuilds the entire search index from the database.
	ReindexAll(ctx context.Context, issueRepo *postgres.IssueRepository, projectRepo *postgres.ProjectRepository) error
}
