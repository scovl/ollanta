// Package search provides Meilisearch integration for fast full-text search and
// faceted filtering of issues and projects. The Meilisearch index is always
// reconstructible from PostgreSQL via ReindexAll — it is never the source of truth.
package search

import (
	"context"
	"fmt"
	"time"

	meilisearch "github.com/meilisearch/meilisearch-go"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
)

const (
	indexIssues   = "issues"
	indexProjects = "projects"
)

// IndexerConfig holds the connection parameters for Meilisearch.
type IndexerConfig struct {
	Host   string // e.g. "http://localhost:7700"
	APIKey string
}

// IssueDocument is the Meilisearch document shape for issues.
type IssueDocument struct {
	ID            int64     `json:"id"`
	ScanID        int64     `json:"scan_id"`
	ProjectID     int64     `json:"project_id"`
	ProjectKey    string    `json:"project_key"`
	RuleKey       string    `json:"rule_key"`
	ComponentPath string    `json:"component_path"`
	Line          int       `json:"line"`
	Message       string    `json:"message"`
	Type          string    `json:"type"`
	Severity      string    `json:"severity"`
	Status        string    `json:"status"`
	Tags          []string  `json:"tags"`
	CreatedAt     time.Time `json:"created_at"`
}

// MeilisearchIndexer synchronises PostgreSQL data into Meilisearch indexes.
type MeilisearchIndexer struct {
	client meilisearch.ServiceManager
}

// NewMeilisearchIndexer creates an indexer connected to the given Meilisearch instance.
func NewMeilisearchIndexer(cfg IndexerConfig) (*MeilisearchIndexer, error) {
	client := meilisearch.New(cfg.Host, meilisearch.WithAPIKey(cfg.APIKey))
	return &MeilisearchIndexer{client: client}, nil
}

// Health returns nil if the Meilisearch server is reachable.
func (idx *MeilisearchIndexer) Health(_ context.Context) error {
	if !idx.client.IsHealthy() {
		return fmt.Errorf("meilisearch is not healthy")
	}
	return nil
}

// ConfigureIndexes sets filterable and sortable attributes on the indexes.
// Safe to call on every startup — Meilisearch is idempotent for settings updates.
func (idx *MeilisearchIndexer) ConfigureIndexes(_ context.Context) error {
	issuesFilterable := []string{
		"project_id", "scan_id", "rule_key", "severity", "type", "status", "component_path",
	}
	issuesSortable := []string{"created_at", "line"}

	if _, err := idx.client.Index(indexIssues).UpdateSettings(&meilisearch.Settings{
		FilterableAttributes: issuesFilterable,
		SortableAttributes:   issuesSortable,
	}); err != nil {
		return fmt.Errorf("configure issues index: %w", err)
	}

	projectsFilterable := []string{"key", "tags"}
	projectsSortable := []string{"created_at", "name"}

	if _, err := idx.client.Index(indexProjects).UpdateSettings(&meilisearch.Settings{
		FilterableAttributes: projectsFilterable,
		SortableAttributes:   projectsSortable,
	}); err != nil {
		return fmt.Errorf("configure projects index: %w", err)
	}
	return nil
}

// IndexIssues adds a batch of issues (linked to scanID) into the issues index.
func (idx *MeilisearchIndexer) IndexIssues(_ context.Context, projectKey string, issues []model.IssueRow) error {
	if len(issues) == 0 {
		return nil
	}
	docs := make([]IssueDocument, len(issues))
	for i, iss := range issues {
		tags := iss.Tags
		if tags == nil {
			tags = []string{}
		}
		docs[i] = IssueDocument{
			ID:            iss.ID,
			ScanID:        iss.ScanID,
			ProjectID:     iss.ProjectID,
			ProjectKey:    projectKey,
			RuleKey:       iss.RuleKey,
			ComponentPath: iss.ComponentPath,
			Line:          iss.Line,
			Message:       iss.Message,
			Type:          iss.Type,
			Severity:      iss.Severity,
			Status:        iss.Status,
			Tags:          tags,
			CreatedAt:     iss.CreatedAt,
		}
	}
	_, err := idx.client.Index(indexIssues).AddDocuments(docs, "id")
	return err
}

// IndexProject adds or updates a project in the projects index.
func (idx *MeilisearchIndexer) IndexProject(_ context.Context, p *model.Project) error {
	doc := map[string]interface{}{
		"id":          p.ID,
		"key":         p.Key,
		"name":        p.Name,
		"description": p.Description,
		"tags":        p.Tags,
		"created_at":  p.CreatedAt,
	}
	_, err := idx.client.Index(indexProjects).AddDocuments([]map[string]interface{}{doc}, "id")
	return err
}

// DeleteScanIssues removes all indexed documents belonging to a specific scan.
func (idx *MeilisearchIndexer) DeleteScanIssues(_ context.Context, scanID int64) error {
	_, err := idx.client.Index(indexIssues).DeleteDocumentsByFilter(
		fmt.Sprintf("scan_id = %d", scanID),
	)
	return err
}

// reindexProject pages through all issues for a single project and indexes them.
func (idx *MeilisearchIndexer) reindexProject(ctx context.Context, p *model.Project, issueRepo port.IIssueRepo) error {
	pid := p.ID
	for offset := 0; ; offset += 1000 {
		issues, _, err := issueRepo.Query(ctx, model.IssueFilter{
			ProjectID: &pid,
			Limit:     1000,
			Offset:    offset,
		})
		if err != nil {
			return fmt.Errorf("query issues for reindex project %s: %w", p.Key, err)
		}
		if len(issues) == 0 {
			break
		}
		rows := make([]model.IssueRow, len(issues))
		for i, iss := range issues {
			rows[i] = *iss
		}
		if err := idx.IndexIssues(ctx, p.Key, rows); err != nil {
			return fmt.Errorf("index issues for project %s: %w", p.Key, err)
		}
		if len(issues) < 1000 {
			break
		}
	}
	return nil
}

// ReindexAll rebuilds the issues index from the database.
func (idx *MeilisearchIndexer) ReindexAll(ctx context.Context, issueRepo port.IIssueRepo, projectRepo port.IProjectRepo) error {
	if _, err := idx.client.Index(indexIssues).DeleteAllDocuments(); err != nil {
		return fmt.Errorf("clear issues index: %w", err)
	}

	projects, err := projectRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("list projects for reindex: %w", err)
	}

	for _, p := range projects {
		if err := idx.reindexProject(ctx, p, issueRepo); err != nil {
			return err
		}
	}
	return nil
}
