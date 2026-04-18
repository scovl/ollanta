// pgfts.go implements the search.ISearcher and search.IIndexer interfaces
// using PostgreSQL full-text search (tsvector/tsquery with GIN indexes).
//
// This backend eliminates the need for an external search service, making
// the system simpler to deploy and operate. It uses the same database that
// is already the source of truth for issues and projects.
//
// Trade-offs vs Meilisearch:
//   - No additional pod / process to manage
//   - Scales with the database (read replicas when needed)
//   - Slightly slower for large result sets, but adequate for typical workloads
//   - No typo tolerance (Meilisearch has fuzzy matching)
package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

// PgFTSBackend implements ISearcher and IIndexer using PostgreSQL FTS.
// Indexing operations are no-ops because PostgreSQL queries live data directly.
type PgFTSBackend struct {
	pool *pgxpool.Pool
}

// compile-time interface checks
var (
	_ ISearcher = (*PgFTSBackend)(nil)
	_ IIndexer  = (*PgFTSBackend)(nil)
)

// NewPgFTSBackend creates a Postgres FTS backend using the given pool.
func NewPgFTSBackend(pool *pgxpool.Pool) *PgFTSBackend {
	return &PgFTSBackend{pool: pool}
}

// Health returns nil because the pool is managed externally by the DB layer.
func (b *PgFTSBackend) Health(ctx context.Context) error {
	return b.pool.Ping(ctx)
}

// ConfigureIndexes creates the GIN indexes needed for full-text search if
// they don't already exist. Safe to call on every startup.
func (b *PgFTSBackend) ConfigureIndexes(ctx context.Context) error {
	stmts := []string{
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issues_fts
		 ON issues USING GIN (to_tsvector('english', message || ' ' || rule_key || ' ' || component_path))`,
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_projects_fts
		 ON projects USING GIN (to_tsvector('english', name || ' ' || COALESCE(description, '')))`,
	}
	for _, sql := range stmts {
		if _, err := b.pool.Exec(ctx, sql); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("pgfts configure: %w", err)
			}
		}
	}
	return nil
}

// IndexIssues is a no-op — Postgres queries live data directly.
func (b *PgFTSBackend) IndexIssues(_ context.Context, _ string, _ []postgres.IssueRow) error {
	return nil
}

// IndexProject is a no-op — Postgres queries live data directly.
func (b *PgFTSBackend) IndexProject(_ context.Context, _ *postgres.Project) error {
	return nil
}

// DeleteScanIssues is a no-op — issues are deleted via CASCADE or direct SQL.
func (b *PgFTSBackend) DeleteScanIssues(_ context.Context, _ int64) error {
	return nil
}

// ReindexAll is a no-op — Postgres FTS reads directly from the tables.
func (b *PgFTSBackend) ReindexAll(_ context.Context, _ *postgres.IssueRepository, _ *postgres.ProjectRepository) error {
	return nil
}

// SearchIssues performs a full-text search against the issues table.
func (b *PgFTSBackend) SearchIssues(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	where := []string{"1=1"}
	args := []interface{}{}
	argN := 1

	if req.Query != "" {
		where = append(where, fmt.Sprintf(
			"to_tsvector('english', message || ' ' || rule_key || ' ' || component_path) @@ plainto_tsquery('english', $%d)", argN))
		args = append(args, req.Query)
		argN++
	}

	for k, v := range req.Filter {
		where = append(where, fmt.Sprintf("%s = $%d", sanitiseColumn(k), argN))
		args = append(args, v)
		argN++
	}

	whereClause := strings.Join(where, " AND ")

	// Count total
	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM issues WHERE %s", whereClause)
	if err := b.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("pgfts count issues: %w", err)
	}

	// Fetch page
	orderBy := "created_at DESC"
	if len(req.Sort) > 0 {
		orderBy = pgSortClause(req.Sort)
	}

	dataSQL := fmt.Sprintf(`
		SELECT id, scan_id, project_id, rule_key, component_path, line,
		       message, type, severity, status, tags, created_at
		FROM issues
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, whereClause, orderBy, argN, argN+1)
	args = append(args, limit, req.Offset)

	rows, err := b.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("pgfts query issues: %w", err)
	}
	defer rows.Close()

	hits := []map[string]interface{}{}
	for rows.Next() {
		var (
			id, scanID, projectID int64
			ruleKey, path, msg    string
			line                  int
			issType, sev, status  string
			tags                  []string
			createdAt             interface{}
		)
		if err := rows.Scan(&id, &scanID, &projectID, &ruleKey, &path, &line,
			&msg, &issType, &sev, &status, &tags, &createdAt); err != nil {
			return nil, err
		}
		hits = append(hits, map[string]interface{}{
			"id": id, "scan_id": scanID, "project_id": projectID,
			"rule_key": ruleKey, "component_path": path, "line": line,
			"message": msg, "type": issType, "severity": sev, "status": status,
			"tags": tags, "created_at": createdAt,
		})
	}

	// Facets
	facets := map[string]map[string]int{}
	for _, facet := range req.Facets {
		col := sanitiseColumn(facet)
		facetSQL := fmt.Sprintf(
			"SELECT %s, COUNT(*) FROM issues WHERE %s GROUP BY %s", col, whereClause, col)
		frows, err := b.pool.Query(ctx, facetSQL, args[:len(args)-2]...) // exclude LIMIT/OFFSET
		if err != nil {
			continue
		}
		m := map[string]int{}
		for frows.Next() {
			var k string
			var c int
			if err := frows.Scan(&k, &c); err == nil {
				m[k] = c
			}
		}
		frows.Close()
		facets[facet] = m
	}

	return &SearchResult{
		Hits:              hits,
		TotalHits:         total,
		FacetDistribution: facets,
	}, nil
}

// SearchProjects performs a full-text search against the projects table.
func (b *PgFTSBackend) SearchProjects(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	where := []string{"1=1"}
	args := []interface{}{}
	argN := 1

	if req.Query != "" {
		where = append(where, fmt.Sprintf(
			"to_tsvector('english', name || ' ' || COALESCE(description, '')) @@ plainto_tsquery('english', $%d)", argN))
		args = append(args, req.Query)
		argN++
	}

	for k, v := range req.Filter {
		where = append(where, fmt.Sprintf("%s = $%d", sanitiseColumn(k), argN))
		args = append(args, v)
		argN++
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM projects WHERE %s", whereClause)
	if err := b.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("pgfts count projects: %w", err)
	}

	dataSQL := fmt.Sprintf(`
		SELECT id, key, name, description, tags, created_at
		FROM projects
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, argN, argN+1)
	args = append(args, limit, req.Offset)

	rows, err := b.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("pgfts query projects: %w", err)
	}
	defer rows.Close()

	hits := []map[string]interface{}{}
	for rows.Next() {
		var (
			id        int64
			key, name string
			desc      *string
			tags      []string
			createdAt interface{}
		)
		if err := rows.Scan(&id, &key, &name, &desc, &tags, &createdAt); err != nil {
			return nil, err
		}
		d := ""
		if desc != nil {
			d = *desc
		}
		hits = append(hits, map[string]interface{}{
			"id": id, "key": key, "name": name, "description": d,
			"tags": tags, "created_at": createdAt,
		})
	}

	return &SearchResult{
		Hits:      hits,
		TotalHits: total,
	}, nil
}

// sanitiseColumn maps filter/facet keys to safe column names to prevent SQL injection.
var allowedColumns = map[string]string{
	"severity":       "severity",
	"type":           "type",
	"status":         "status",
	"rule_key":       "rule_key",
	"project_id":     "project_id",
	"scan_id":        "scan_id",
	"component_path": "component_path",
	"key":            "key",
}

func sanitiseColumn(name string) string {
	if col, ok := allowedColumns[name]; ok {
		return col
	}
	return "id" // safe fallback — never inject raw user input
}

// pgSortClause converts search sort directives ("field:asc"/"field:desc") to SQL.
func pgSortClause(sorts []string) string {
	parts := make([]string, 0, len(sorts))
	for _, s := range sorts {
		tokens := strings.SplitN(s, ":", 2)
		col := sanitiseColumn(tokens[0])
		dir := "ASC"
		if len(tokens) == 2 && strings.EqualFold(tokens[1], "desc") {
			dir = "DESC"
		}
		parts = append(parts, col+" "+dir)
	}
	if len(parts) == 0 {
		return "id DESC"
	}
	return strings.Join(parts, ", ")
}
