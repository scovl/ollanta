package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	domainmodel "github.com/scovl/ollanta/domain/model"
)

// Scan is the canonical scan record stored in PostgreSQL.
type Scan struct {
	ID                   int64     `json:"id"`
	ProjectID            int64     `json:"project_id"`
	Version              string    `json:"version"`
	ScopeType            string    `json:"scope_type"`
	Branch               string    `json:"branch"`
	CommitSHA            string    `json:"commit_sha"`
	PullRequestKey       string    `json:"pull_request_key"`
	PullRequestBase      string    `json:"pull_request_base"`
	Status               string    `json:"status"`
	ElapsedMs            int64     `json:"elapsed_ms"`
	GateStatus           string    `json:"gate_status"`
	AnalysisDate         time.Time `json:"analysis_date"`
	CreatedAt            time.Time `json:"created_at"`
	TotalFiles           int       `json:"total_files"`
	TotalLines           int       `json:"total_lines"`
	TotalNcloc           int       `json:"total_ncloc"`
	TotalComments        int       `json:"total_comments"`
	TotalIssues          int       `json:"total_issues"`
	TotalBugs            int       `json:"total_bugs"`
	TotalCodeSmells      int       `json:"total_code_smells"`
	TotalVulnerabilities int       `json:"total_vulnerabilities"`
	NewIssues            int       `json:"new_issues"`
	ClosedIssues         int       `json:"closed_issues"`
}

// ScanRepository provides access to the scans table.
type ScanRepository struct {
	db *DB
}

// BranchSummary describes the latest branch-scoped analysis known for a project.
type BranchSummary struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	LatestScan *Scan `json:"latest_scan,omitempty"`
}

// PullRequestSummary describes the latest pull request analysis known for a project.
type PullRequestSummary struct {
	Key        string `json:"key"`
	Branch     string `json:"branch"`
	BaseBranch string `json:"base_branch"`
	LatestScan *Scan  `json:"latest_scan,omitempty"`
}

const scanSelectColumns = `
	id, project_id, version, scope_type, branch, commit_sha,
	pull_request_key, pull_request_base, status, elapsed_ms,
	gate_status, analysis_date, created_at,
	total_files, total_lines, total_ncloc, total_comments,
	total_issues, total_bugs, total_code_smells, total_vulnerabilities,
	new_issues, closed_issues`

// NewScanRepository creates a ScanRepository backed by db.
func NewScanRepository(db *DB) *ScanRepository {
	return &ScanRepository{db: db}
}

func scanDest(s *Scan) []any {
	return []any{
		&s.ID, &s.ProjectID, &s.Version, &s.ScopeType, &s.Branch, &s.CommitSHA,
		&s.PullRequestKey, &s.PullRequestBase, &s.Status, &s.ElapsedMs,
		&s.GateStatus, &s.AnalysisDate, &s.CreatedAt,
		&s.TotalFiles, &s.TotalLines, &s.TotalNcloc, &s.TotalComments,
		&s.TotalIssues, &s.TotalBugs, &s.TotalCodeSmells, &s.TotalVulnerabilities,
		&s.NewIssues, &s.ClosedIssues,
	}
}

func buildScopeCondition(scope domainmodel.AnalysisScope, defaultBranch string, start int) (string, []any) {
	scope = scope.Normalize()
	if scope.Type == domainmodel.ScopeTypePullRequest {
		return fmt.Sprintf("scope_type = $%d AND pull_request_key = $%d", start, start+1), []any{domainmodel.ScopeTypePullRequest, scope.PullRequestKey}
	}

	branch := scope.Branch
	if branch == "" {
		branch = defaultBranch
	}
	cond := fmt.Sprintf("scope_type = $%d AND (branch = $%d", start, start+1)
	if branch == defaultBranch {
		cond += " OR branch = ''"
	}
	cond += ")"
	return cond, []any{domainmodel.ScopeTypeBranch, branch}
}

func chooseDefaultBranch(branches []string) string {
	for _, branch := range branches {
		if branch == "main" {
			return branch
		}
	}
	for _, branch := range branches {
		if branch == "master" {
			return branch
		}
	}
	if len(branches) > 0 {
		return branches[0]
	}
	return ""
}

// Create inserts a new scan and populates its ID and CreatedAt.
func (r *ScanRepository) Create(ctx context.Context, s *Scan) error {
	if s.ScopeType == "" {
		s.ScopeType = domainmodel.ScopeTypeBranch
	}
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO scans (
			project_id, version, scope_type, branch, commit_sha,
			pull_request_key, pull_request_base, status, elapsed_ms,
			gate_status, analysis_date,
			total_files, total_lines, total_ncloc, total_comments,
			total_issues, total_bugs, total_code_smells, total_vulnerabilities,
			new_issues, closed_issues
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
		) RETURNING id, created_at`,
		s.ProjectID, s.Version, s.ScopeType, s.Branch, s.CommitSHA,
		s.PullRequestKey, s.PullRequestBase, s.Status, s.ElapsedMs,
		s.GateStatus, s.AnalysisDate,
		s.TotalFiles, s.TotalLines, s.TotalNcloc, s.TotalComments,
		s.TotalIssues, s.TotalBugs, s.TotalCodeSmells, s.TotalVulnerabilities,
		s.NewIssues, s.ClosedIssues,
	)
	return row.Scan(&s.ID, &s.CreatedAt)
}

// Update persists gate_status, new_issues, and closed_issues for an existing scan.
func (r *ScanRepository) Update(ctx context.Context, s *Scan) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE scans
		SET gate_status = $1, new_issues = $2, closed_issues = $3
		WHERE id = $4`,
		s.GateStatus, s.NewIssues, s.ClosedIssues, s.ID,
	)
	return err
}

// GetByID retrieves a scan by its primary key. Returns ErrNotFound when absent.
func (r *ScanRepository) GetByID(ctx context.Context, id int64) (*Scan, error) {
	s := &Scan{}
	err := r.db.Pool.QueryRow(ctx, "SELECT "+scanSelectColumns+" FROM scans WHERE id = $1", id).Scan(scanDest(s)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// GetLatest returns the most recent scan for a project. Returns ErrNotFound when none.
func (r *ScanRepository) GetLatest(ctx context.Context, projectID int64) (*Scan, error) {
	s := &Scan{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT `+scanSelectColumns+`
		FROM scans
		WHERE project_id = $1
		ORDER BY analysis_date DESC, id DESC
		LIMIT 1`, projectID,
	).Scan(scanDest(s)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// GetLatestInScope returns the most recent scan for the resolved branch or pull request scope.
func (r *ScanRepository) GetLatestInScope(ctx context.Context, projectID int64, scope domainmodel.AnalysisScope, defaultBranch string) (*Scan, error) {
	condition, args := buildScopeCondition(scope, defaultBranch, 2)
	args = append([]any{projectID}, args...)
	s := &Scan{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT `+scanSelectColumns+`
		FROM scans
		WHERE project_id = $1 AND `+condition+`
		ORDER BY analysis_date DESC, id DESC
		LIMIT 1`, args...).Scan(scanDest(s)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// ListByProject returns a page of scans for a project ordered by analysis_date DESC,
// plus the total count.
func (r *ScanRepository) ListByProject(ctx context.Context, projectID int64, limit, offset int) ([]*Scan, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	if err := r.db.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM scans WHERE project_id = $1", projectID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count scans: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx, `
		SELECT `+scanSelectColumns+`
		FROM scans
		WHERE project_id = $1
		ORDER BY analysis_date DESC, id DESC
		LIMIT $2 OFFSET $3`, projectID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var scans []*Scan
	for rows.Next() {
		s := &Scan{}
		if err := rows.Scan(scanDest(s)...); err != nil {
			return nil, 0, err
		}
		scans = append(scans, s)
	}
	return scans, total, rows.Err()
}

// ListByProjectInScope returns all scans for a single logical scope ordered by newest first.
func (r *ScanRepository) ListByProjectInScope(ctx context.Context, projectID int64, scope domainmodel.AnalysisScope, defaultBranch string) ([]*Scan, error) {
	condition, args := buildScopeCondition(scope, defaultBranch, 2)
	args = append([]any{projectID}, args...)
	rows, err := r.db.Pool.Query(ctx, `
		SELECT `+scanSelectColumns+`
		FROM scans
		WHERE project_id = $1 AND `+condition+`
		ORDER BY analysis_date DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []*Scan
	for rows.Next() {
		s := &Scan{}
		if err := rows.Scan(scanDest(s)...); err != nil {
			return nil, err
		}
		scans = append(scans, s)
	}
	if scans == nil {
		scans = []*Scan{}
	}
	return scans, rows.Err()
}

// ResolveDefaultBranch returns the configured branch when available, otherwise infers it from historical branch scans.
func (r *ScanRepository) ResolveDefaultBranch(ctx context.Context, projectID int64, configured string) (string, bool, error) {
	if configured != "" {
		return configured, false, nil
	}

	rows, err := r.db.Pool.Query(ctx, `
		SELECT branch
		FROM scans
		WHERE project_id = $1
		  AND scope_type = $2
		  AND branch <> ''
		GROUP BY branch
		ORDER BY MAX(analysis_date) DESC`, projectID, domainmodel.ScopeTypeBranch)
	if err != nil {
		return "", false, err
	}
	defer rows.Close()

	var branches []string
	for rows.Next() {
		var branch string
		if err := rows.Scan(&branch); err != nil {
			return "", false, err
		}
		branches = append(branches, branch)
	}
	if err := rows.Err(); err != nil {
		return "", false, err
	}

	if resolved := chooseDefaultBranch(branches); resolved != "" {
		return resolved, true, nil
	}

	var hasLegacy bool
	if err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM scans
			WHERE project_id = $1
			  AND scope_type = $2
			  AND branch = ''
		)`, projectID, domainmodel.ScopeTypeBranch).Scan(&hasLegacy); err != nil {
		return "", false, err
	}
	if hasLegacy {
		return "", true, nil
	}

	return "", true, nil
}

// ListBranches returns the distinct logical branches known for a project with latest scan metadata.
func (r *ScanRepository) ListBranches(ctx context.Context, projectID int64, defaultBranch string) ([]*BranchSummary, error) {
	rows, err := r.db.Pool.Query(ctx, `
		WITH ranked AS (
			SELECT
				CASE WHEN branch = '' THEN $2 ELSE branch END AS logical_branch,
				`+scanSelectColumns+`,
				ROW_NUMBER() OVER (
					PARTITION BY CASE WHEN branch = '' THEN $2 ELSE branch END
					ORDER BY analysis_date DESC, id DESC
				) AS rn
			FROM scans
			WHERE project_id = $1
			  AND scope_type = $3
		)
		SELECT logical_branch, `+scanSelectColumns+`
		FROM ranked
		WHERE rn = 1
		ORDER BY analysis_date DESC, logical_branch ASC`,
		projectID, defaultBranch, domainmodel.ScopeTypeBranch,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*BranchSummary
	for rows.Next() {
		item := &BranchSummary{}
		item.LatestScan = &Scan{}
		if err := rows.Scan(append([]any{&item.Name}, scanDest(item.LatestScan)...)...); err != nil {
			return nil, err
		}
		item.IsDefault = item.Name == defaultBranch
		items = append(items, item)
	}
	if items == nil {
		items = []*BranchSummary{}
	}
	return items, rows.Err()
}

// ListPullRequests returns the distinct pull requests known for a project with latest scan metadata.
func (r *ScanRepository) ListPullRequests(ctx context.Context, projectID int64) ([]*PullRequestSummary, error) {
	rows, err := r.db.Pool.Query(ctx, `
		WITH ranked AS (
			SELECT
				`+scanSelectColumns+`,
				ROW_NUMBER() OVER (
					PARTITION BY pull_request_key
					ORDER BY analysis_date DESC, id DESC
				) AS rn
			FROM scans
			WHERE project_id = $1
			  AND scope_type = $2
			  AND pull_request_key <> ''
		)
		SELECT pull_request_key, `+scanSelectColumns+`
		FROM ranked
		WHERE rn = 1
		ORDER BY analysis_date DESC, pull_request_key ASC`,
		projectID, domainmodel.ScopeTypePullRequest,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*PullRequestSummary
	for rows.Next() {
		item := &PullRequestSummary{LatestScan: &Scan{}}
		if err := rows.Scan(scanDest(item.LatestScan)...); err != nil {
			return nil, err
		}
		item.Key = item.LatestScan.PullRequestKey
		item.Branch = item.LatestScan.Branch
		item.BaseBranch = item.LatestScan.PullRequestBase
		items = append(items, item)
	}
	if items == nil {
		items = []*PullRequestSummary{}
	}
	return items, rows.Err()
}
