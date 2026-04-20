package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// IssueRow is the database representation of a single issue.
type IssueRow struct {
	ID                 int64           `json:"id"`
	ScanID             int64           `json:"scan_id"`
	ProjectID          int64           `json:"project_id"`
	RuleKey            string          `json:"rule_key"`
	ComponentPath      string          `json:"component_path"`
	Line               int             `json:"line"`
	Column             int             `json:"column"`
	EndLine            int             `json:"end_line"`
	EndColumn          int             `json:"end_column"`
	Message            string          `json:"message"`
	Type               string          `json:"type"`
	Severity           string          `json:"severity"`
	Status             string          `json:"status"`
	Resolution         string          `json:"resolution"`
	EffortMinutes      int             `json:"effort_minutes"`
	EngineID           string          `json:"engine_id"`
	LineHash           string          `json:"line_hash"`
	Tags               []string        `json:"tags"`
	SecondaryLocations json.RawMessage `json:"secondary_locations"`
	CreatedAt          time.Time       `json:"created_at"`
}

// IssueFilter specifies query parameters for listing issues.
type IssueFilter struct {
	ProjectID *int64
	ScanID    *int64
	RuleKey   *string
	Severity  *string
	Type      *string
	Status    *string
	FilePath  *string // applied as LIKE pattern against component_path
	EngineID  *string
	Limit     int // default 100, max 1000
	Offset    int
}

// IssueFacets holds aggregate distributions for the issues index.
type IssueFacets struct {
	BySeverity map[string]int `json:"by_severity"`
	ByType     map[string]int `json:"by_type"`
	ByRule     map[string]int `json:"by_rule"`
	ByStatus   map[string]int `json:"by_status"`
	ByEngineID map[string]int `json:"by_engine_id"`
	ByFile     map[string]int `json:"by_file"`
	ByTags     map[string]int `json:"by_tags"`
}

// IssueRepository provides access to the issues table.
type IssueRepository struct {
	db *DB
}

// NewIssueRepository creates an IssueRepository backed by db.
func NewIssueRepository(db *DB) *IssueRepository {
	return &IssueRepository{db: db}
}

// BulkInsert inserts issues using the PostgreSQL COPY protocol for maximum throughput.
func (r *IssueRepository) BulkInsert(ctx context.Context, issues []IssueRow) error {
	if len(issues) == 0 {
		return nil
	}

	conn, err := r.db.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for bulk insert: %w", err)
	}
	defer conn.Release()

	rows := make([][]interface{}, len(issues))
	for i, iss := range issues {
		tags := iss.Tags
		if tags == nil {
			tags = []string{}
		}
		sl := iss.SecondaryLocations
		if len(sl) == 0 {
			sl = json.RawMessage("[]")
		}
		engineID := iss.EngineID
		if engineID == "" {
			engineID = "ollanta"
		}
		rows[i] = []interface{}{
			iss.ScanID, iss.ProjectID, iss.RuleKey, iss.ComponentPath,
			iss.Line, iss.Column, iss.EndLine, iss.EndColumn,
			iss.Message, iss.Type, iss.Severity, iss.Status,
			iss.Resolution, iss.EffortMinutes, engineID, iss.LineHash, tags, sl,
		}
	}

	_, err = conn.CopyFrom(
		ctx,
		pgx.Identifier{"issues"},
		[]string{
			"scan_id", "project_id", "rule_key", "component_path",
			"line", "column_num", "end_line", "end_column",
			"message", "type", "severity", "status",
			"resolution", "effort_minutes", "engine_id", "line_hash", "tags",
			"secondary_locations",
		},
		pgx.CopyFromRows(rows),
	)
	return err
}

// buildIssueFilter constructs WHERE clause conditions and argument values from f.
func buildIssueFilter(f IssueFilter) (conds []string, args []interface{}) {
	n := 1
	if f.ProjectID != nil {
		conds = append(conds, fmt.Sprintf("project_id = $%d", n))
		args = append(args, *f.ProjectID)
		n++
	}
	if f.ScanID != nil {
		conds = append(conds, fmt.Sprintf("scan_id = $%d", n))
		args = append(args, *f.ScanID)
		n++
	}
	if f.RuleKey != nil {
		conds = append(conds, fmt.Sprintf("rule_key = $%d", n))
		args = append(args, *f.RuleKey)
		n++
	}
	if f.Severity != nil {
		conds = append(conds, fmt.Sprintf("severity = $%d", n))
		args = append(args, *f.Severity)
		n++
	}
	if f.Type != nil {
		conds = append(conds, fmt.Sprintf("type = $%d", n))
		args = append(args, *f.Type)
		n++
	}
	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", n))
		args = append(args, *f.Status)
		n++
	}
	if f.FilePath != nil {
		conds = append(conds, fmt.Sprintf("component_path LIKE $%d", n))
		args = append(args, "%"+*f.FilePath+"%")
		n++
	}
	if f.EngineID != nil {
		conds = append(conds, fmt.Sprintf("engine_id = $%d", n))
		args = append(args, *f.EngineID)
		n++
	}
	return
}

// Query returns issues matching the filter, plus the total count before LIMIT/OFFSET.
func (r *IssueRepository) Query(ctx context.Context, f IssueFilter) ([]*IssueRow, int, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	if f.Limit > 1000 {
		f.Limit = 1000
	}

	conds, args := buildIssueFilter(f)

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM issues " + where
	var total int
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count issues: %w", err)
	}

	n := len(args) + 1
	args = append(args, f.Limit, f.Offset)
	listQuery := fmt.Sprintf(`
		SELECT id, scan_id, project_id, rule_key, component_path,
		       line, column_num, end_line, end_column, message,
		       type, severity, status, resolution, effort_minutes,
		       engine_id, line_hash, tags, secondary_locations, created_at
		FROM issues %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d`, where, n, n+1)

	rows, err := r.db.Pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var issues []*IssueRow
	for rows.Next() {
		iss := &IssueRow{}
		if err := rows.Scan(
			&iss.ID, &iss.ScanID, &iss.ProjectID, &iss.RuleKey, &iss.ComponentPath,
			&iss.Line, &iss.Column, &iss.EndLine, &iss.EndColumn, &iss.Message,
			&iss.Type, &iss.Severity, &iss.Status, &iss.Resolution, &iss.EffortMinutes,
			&iss.EngineID, &iss.LineHash, &iss.Tags, &iss.SecondaryLocations, &iss.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		issues = append(issues, iss)
	}
	return issues, total, rows.Err()
}

// facetRow holds a single facet result: a column value and its issue count.
type facetRow struct {
	key   string
	count int
}

// scanFacetRows reads (key, count) pairs from rows into a facetRow slice.
func scanFacetRows(rows pgx.Rows) ([]facetRow, error) {
	defer rows.Close()
	var out []facetRow
	for rows.Next() {
		var fr facetRow
		if err := rows.Scan(&fr.key, &fr.count); err != nil {
			return nil, err
		}
		out = append(out, fr)
	}
	return out, rows.Err()
}

// applyFacet copies facetRow results into a string→int map.
func applyFacet(dst map[string]int, rows []facetRow) {
	for _, r := range rows {
		dst[r.key] = r.count
	}
}

// queryFacet counts issues grouped by column for the given project and scan.
func (r *IssueRepository) queryFacet(ctx context.Context, column string, projectID, scanID int64) ([]facetRow, error) {
	q := fmt.Sprintf(`
		SELECT %s, COUNT(*) FROM issues
		WHERE project_id = $1 AND scan_id = $2
		GROUP BY %s`, column, column)
	rows, err := r.db.Pool.Query(ctx, q, projectID, scanID)
	if err != nil {
		return nil, err
	}
	return scanFacetRows(rows)
}

// queryTopFacet counts issues grouped by column, returning only the top-N results.
func (r *IssueRepository) queryTopFacet(ctx context.Context, column string, limit int, projectID, scanID int64) ([]facetRow, error) {
	q := fmt.Sprintf(`
		SELECT %s, COUNT(*) AS cnt FROM issues
		WHERE project_id = $1 AND scan_id = $2
		GROUP BY %s
		ORDER BY cnt DESC
		LIMIT %d`, column, column, limit)
	rows, err := r.db.Pool.Query(ctx, q, projectID, scanID)
	if err != nil {
		return nil, err
	}
	return scanFacetRows(rows)
}

// Facets returns count distributions by severity, type, rule, status,
// engine_id, file (top 10), and tags for a given scan.
// Inspired by SonarQube's sticky faceted search pattern.
func (r *IssueRepository) Facets(ctx context.Context, projectID, scanID int64) (*IssueFacets, error) {
	facets := &IssueFacets{
		BySeverity: make(map[string]int),
		ByType:     make(map[string]int),
		ByRule:     make(map[string]int),
		ByStatus:   make(map[string]int),
		ByEngineID: make(map[string]int),
		ByFile:     make(map[string]int),
		ByTags:     make(map[string]int),
	}

	sev, err := r.queryFacet(ctx, "severity", projectID, scanID)
	if err != nil {
		return nil, fmt.Errorf("facet severity: %w", err)
	}
	applyFacet(facets.BySeverity, sev)

	typ, err := r.queryFacet(ctx, "type", projectID, scanID)
	if err != nil {
		return nil, fmt.Errorf("facet type: %w", err)
	}
	applyFacet(facets.ByType, typ)

	rule, err := r.queryTopFacet(ctx, "rule_key", 20, projectID, scanID)
	if err != nil {
		return nil, fmt.Errorf("facet rule: %w", err)
	}
	applyFacet(facets.ByRule, rule)

	st, err := r.queryFacet(ctx, "status", projectID, scanID)
	if err != nil {
		return nil, fmt.Errorf("facet status: %w", err)
	}
	applyFacet(facets.ByStatus, st)

	eng, err := r.queryFacet(ctx, "engine_id", projectID, scanID)
	if err != nil {
		return nil, fmt.Errorf("facet engine_id: %w", err)
	}
	applyFacet(facets.ByEngineID, eng)

	file, err := r.queryTopFacet(ctx, "component_path", 10, projectID, scanID)
	if err != nil {
		return nil, fmt.Errorf("facet file: %w", err)
	}
	applyFacet(facets.ByFile, file)

	// Tags facet uses unnest since tags is an array column.
	tagRows, err := r.db.Pool.Query(ctx, `
		SELECT t, COUNT(*) AS cnt
		FROM issues, unnest(tags) AS t
		WHERE project_id = $1 AND scan_id = $2
		GROUP BY t
		ORDER BY cnt DESC
		LIMIT 20`, projectID, scanID)
	if err != nil {
		return nil, fmt.Errorf("facet tags: %w", err)
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var tag string
		var cnt int
		if err := tagRows.Scan(&tag, &cnt); err != nil {
			return nil, fmt.Errorf("facet tags scan: %w", err)
		}
		facets.ByTags[tag] = cnt
	}
	if err := tagRows.Err(); err != nil {
		return nil, fmt.Errorf("facet tags iter: %w", err)
	}

	return facets, nil
}

// CountByProject returns the total number of issues for a project.
func (r *IssueRepository) CountByProject(ctx context.Context, projectID int64) (int, error) {
	var n int
	err := r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM issues WHERE project_id = $1", projectID,
	).Scan(&n)
	return n, err
}

// GetByID returns a single issue by ID.
func (r *IssueRepository) GetByID(ctx context.Context, id int64) (*IssueRow, error) {
	iss := &IssueRow{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, scan_id, project_id, rule_key, component_path,
		       line, column_num, end_line, end_column, message,
		       type, severity, status, resolution, effort_minutes,
		       engine_id, line_hash, tags, secondary_locations, created_at
		FROM issues WHERE id = $1 LIMIT 1`, id,
	).Scan(
		&iss.ID, &iss.ScanID, &iss.ProjectID, &iss.RuleKey, &iss.ComponentPath,
		&iss.Line, &iss.Column, &iss.EndLine, &iss.EndColumn, &iss.Message,
		&iss.Type, &iss.Severity, &iss.Status, &iss.Resolution, &iss.EffortMinutes,
		&iss.EngineID, &iss.LineHash, &iss.Tags, &iss.SecondaryLocations, &iss.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return iss, err
}

// IssueTransition represents a status change for an issue.
type IssueTransition struct {
	ID         int64     `json:"id"`
	IssueID    int64     `json:"issue_id"`
	UserID     int64     `json:"user_id"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Resolution string    `json:"resolution"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
}

// validTransitions maps allowed from→to status transitions.
var validTransitions = map[string][]string{
	"open":   {"closed", "confirmed"},
	"closed": {"open"},
}

// validResolutions are the allowed resolution values for closing issues.
var validResolutions = map[string]bool{
	"false_positive": true,
	"wont_fix":       true,
	"confirmed":      true,
	"fixed":          true,
	"":               true, // reopen
}

// Transition applies a status change to an issue and records the transition history.
func (r *IssueRepository) Transition(ctx context.Context, issueID, userID int64, toStatus, resolution, comment string) error {
	if !validResolutions[resolution] {
		return fmt.Errorf("invalid resolution: %s", resolution)
	}

	iss, err := r.GetByID(ctx, issueID)
	if err != nil {
		return err
	}

	allowed := validTransitions[iss.Status]
	found := false
	for _, s := range allowed {
		if s == toStatus {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("transition from %s to %s is not allowed", iss.Status, toStatus)
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		UPDATE issues SET status = $1, resolution = $2 WHERE id = $3`,
		toStatus, resolution, issueID)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO issue_transitions (issue_id, user_id, from_status, to_status, resolution, comment)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		issueID, userID, iss.Status, toStatus, resolution, comment)
	if err != nil {
		return fmt.Errorf("insert transition: %w", err)
	}

	return tx.Commit(ctx)
}
