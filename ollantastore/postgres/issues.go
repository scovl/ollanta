package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	domainmodel "github.com/scovl/ollanta/domain/model"
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
	QualityDomain      string          `json:"quality_domain,omitempty"`
	Language           string          `json:"language,omitempty"`
	Status             string          `json:"status"`
	Resolution         string          `json:"resolution"`
	TrackingState      string          `json:"tracking_state"`
	EffortMinutes      int             `json:"effort_minutes"`
	EngineID           string          `json:"engine_id"`
	LineHash           string          `json:"line_hash"`
	Tags               []string        `json:"tags"`
	SecondaryLocations json.RawMessage `json:"secondary_locations"`
	CreatedAt          time.Time       `json:"created_at"`
}

// IssueFilter specifies query parameters for listing issues.
type IssueFilter struct {
	ProjectID        *int64
	ScanID           *int64
	RuleKey          *string
	Severity         *string
	Type             *string
	QualityDomain    *string
	Status           *string
	TrackingState    *string
	Language         *string
	Tag              *string
	SecurityCategory *string
	Directory        *string
	FilePath         *string // applied as LIKE pattern against component_path
	EngineID         *string
	Limit            int // default 100, max 1000
	Offset           int
}

// IssueFacets holds aggregate distributions for the issues index.
type IssueFacets struct {
	BySeverity         map[string]int `json:"by_severity"`
	ByType             map[string]int `json:"by_type"`
	ByQuality          map[string]int `json:"by_quality"`
	ByRule             map[string]int `json:"by_rule"`
	ByStatus           map[string]int `json:"by_status"`
	ByLifecycle        map[string]int `json:"by_lifecycle"`
	ByLanguage         map[string]int `json:"by_language"`
	ByEngineID         map[string]int `json:"by_engine_id"`
	ByFile             map[string]int `json:"by_file"`
	ByDirectory        map[string]int `json:"by_directory"`
	ByTags             map[string]int `json:"by_tags"`
	BySecurityCategory map[string]int `json:"by_security_category"`
}

var testabilityFacetTags = []string{"testability", "unit-test", "coverage-gap", "mutation", "survived-mutant", "failing-test", "flaky-test", "mutation-testing"}

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
			iss.Resolution, iss.TrackingState, iss.EffortMinutes, engineID, iss.LineHash, tags, sl,
		}
	}

	_, err = conn.CopyFrom(
		ctx,
		pgx.Identifier{"issues"},
		[]string{
			"scan_id", "project_id", "rule_key", "component_path",
			"line", "column_num", "end_line", "end_column",
			"message", "type", "severity", "status",
			"resolution", "tracking_state", "effort_minutes", "engine_id", "line_hash", "tags",
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
	if f.QualityDomain != nil {
		conds = append(conds, qualityDomainCondition(*f.QualityDomain, n))
		args = append(args, testabilityFacetTags)
		n++
	}
	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", n))
		args = append(args, *f.Status)
		n++
	}
	if f.TrackingState != nil {
		conds = append(conds, fmt.Sprintf("tracking_state = $%d", n))
		args = append(args, *f.TrackingState)
		n++
	}
	if f.Language != nil {
		conds = append(conds, languageCondition(*f.Language))
	}
	if f.Tag != nil {
		conds = append(conds, fmt.Sprintf("$%d = ANY(tags)", n))
		args = append(args, *f.Tag)
		n++
	}
	if f.SecurityCategory != nil {
		conds = append(conds, fmt.Sprintf("$%d = ANY(tags)", n))
		args = append(args, *f.SecurityCategory)
		n++
	}
	if f.Directory != nil {
		directory := strings.Trim(strings.ReplaceAll(*f.Directory, "\\", "/"), "/")
		conds = append(conds, fmt.Sprintf("(component_path = $%d OR component_path LIKE $%d)", n, n+1))
		args = append(args, directory, directory+"/%")
		n += 2
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

func qualityDomainCondition(quality string, argIndex int) string {
	testTagPredicate := fmt.Sprintf("tags && $%d::text[]", argIndex)
	switch quality {
	case string(domainmodel.QualitySecurity):
		return fmt.Sprintf("(NOT (%s) AND type IN ('vulnerability', 'security_hotspot'))", testTagPredicate)
	case string(domainmodel.QualityReliability):
		return fmt.Sprintf("(NOT (%s) AND type = 'bug')", testTagPredicate)
	case string(domainmodel.QualityTestability):
		return testTagPredicate
	default:
		return fmt.Sprintf("(NOT (%s) AND type = 'code_smell')", testTagPredicate)
	}
}

func languageCondition(language string) string {
	pathExpr := "LOWER(REPLACE(component_path, '\\\\', '/'))"
	switch language {
	case domainmodel.LangGo:
		return pathExpr + " LIKE '%.go'"
	case domainmodel.LangJavaScript:
		return "(" + pathExpr + " LIKE '%.js' OR " + pathExpr + " LIKE '%.mjs')"
	case domainmodel.LangTypeScript:
		return "(" + pathExpr + " LIKE '%.ts' OR " + pathExpr + " LIKE '%.tsx')"
	case domainmodel.LangPython:
		return pathExpr + " LIKE '%.py'"
	case domainmodel.LangRust:
		return pathExpr + " LIKE '%.rs'"
	default:
		return "NOT (" + pathExpr + " LIKE '%.go' OR " + pathExpr + " LIKE '%.js' OR " + pathExpr + " LIKE '%.mjs' OR " + pathExpr + " LIKE '%.ts' OR " + pathExpr + " LIKE '%.tsx' OR " + pathExpr + " LIKE '%.py' OR " + pathExpr + " LIKE '%.rs')"
	}
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
		       type, severity, status, resolution, tracking_state, effort_minutes,
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
			&iss.Type, &iss.Severity, &iss.Status, &iss.Resolution, &iss.TrackingState, &iss.EffortMinutes,
			&iss.EngineID, &iss.LineHash, &iss.Tags, &iss.SecondaryLocations, &iss.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		issues = append(issues, iss)
	}
	for _, issue := range issues {
		enrichIssueRow(issue)
	}
	return issues, total, rows.Err()
}

// UpdateTrackingStates backfills tracking_state for existing issues.
// Only rows still marked as unknown are updated so the operation is safe to rerun.
func (r *IssueRepository) UpdateTrackingStates(ctx context.Context, states map[int64]string) (int64, error) {
	if len(states) == 0 {
		return 0, nil
	}

	ids := make([]int64, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	batch := &pgx.Batch{}
	for _, id := range ids {
		batch.Queue(`
			UPDATE issues
			SET tracking_state = $2
			WHERE id = $1 AND tracking_state = 'unknown'`, id, states[id])
	}

	results := r.db.Pool.SendBatch(ctx, batch)
	var updated int64
	for range ids {
		ct, err := results.Exec()
		if err != nil {
			_ = results.Close()
			return updated, err
		}
		updated += ct.RowsAffected()
	}
	if err := results.Close(); err != nil {
		return updated, err
	}
	return updated, nil
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
	return r.FacetsForFilter(ctx, IssueFilter{ProjectID: &projectID, ScanID: &scanID})
}

func (r *IssueRepository) FacetsForFilter(ctx context.Context, filter IssueFilter) (*IssueFacets, error) {
	facets := &IssueFacets{
		BySeverity:         make(map[string]int),
		ByType:             make(map[string]int),
		ByQuality:          make(map[string]int),
		ByRule:             make(map[string]int),
		ByStatus:           make(map[string]int),
		ByLifecycle:        make(map[string]int),
		ByLanguage:         make(map[string]int),
		ByEngineID:         make(map[string]int),
		ByFile:             make(map[string]int),
		ByDirectory:        make(map[string]int),
		ByTags:             make(map[string]int),
		BySecurityCategory: make(map[string]int),
	}
	conds, args := buildIssueFilter(filter)
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	query := `
		SELECT type, severity, status, tracking_state, rule_key, component_path, engine_id, tags
		FROM issues ` + where
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query issue facets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		issue := &IssueRow{}
		if err := rows.Scan(&issue.Type, &issue.Severity, &issue.Status, &issue.TrackingState, &issue.RuleKey, &issue.ComponentPath, &issue.EngineID, &issue.Tags); err != nil {
			return nil, fmt.Errorf("scan issue facet row: %w", err)
		}
		enrichIssueRow(issue)
		increment(facets.BySeverity, issue.Severity)
		increment(facets.ByType, issue.Type)
		increment(facets.ByQuality, issue.QualityDomain)
		increment(facets.ByRule, issue.RuleKey)
		increment(facets.ByStatus, issue.Status)
		increment(facets.ByLifecycle, issue.TrackingState)
		increment(facets.ByLanguage, issue.Language)
		increment(facets.ByEngineID, issue.EngineID)
		increment(facets.ByFile, issue.ComponentPath)
		increment(facets.ByDirectory, issueDirectory(issue.ComponentPath))
		for _, tag := range issue.Tags {
			increment(facets.ByTags, tag)
		}
		for _, category := range domainmodel.SecurityCategories(issue.Tags) {
			increment(facets.BySecurityCategory, category)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue facets: %w", err)
	}

	return facets, nil
}

func increment(target map[string]int, key string) {
	if key == "" {
		return
	}
	target[key]++
}

func issueDirectory(path string) string {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	idx := strings.LastIndex(normalized, "/")
	if idx <= 0 {
		return "./"
	}
	return normalized[:idx]
}

func enrichIssueRow(issue *IssueRow) {
	if issue == nil {
		return
	}
	if issue.Language == "" {
		issue.Language = domainmodel.LanguageFromPath(issue.ComponentPath)
	}
	if issue.QualityDomain == "" {
		issue.QualityDomain = string(domainmodel.DeriveIssueQualityDomain(domainmodel.IssueType(issue.Type), issue.Tags))
	}
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
		       type, severity, status, resolution, tracking_state, effort_minutes,
		       engine_id, line_hash, tags, secondary_locations, created_at
		FROM issues WHERE id = $1 LIMIT 1`, id,
	).Scan(
		&iss.ID, &iss.ScanID, &iss.ProjectID, &iss.RuleKey, &iss.ComponentPath,
		&iss.Line, &iss.Column, &iss.EndLine, &iss.EndColumn, &iss.Message,
		&iss.Type, &iss.Severity, &iss.Status, &iss.Resolution, &iss.TrackingState, &iss.EffortMinutes,
		&iss.EngineID, &iss.LineHash, &iss.Tags, &iss.SecondaryLocations, &iss.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	enrichIssueRow(iss)
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
