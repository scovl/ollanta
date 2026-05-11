package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// NewCodePeriod represents a new code period setting at global, project, or branch scope.
type NewCodePeriod struct {
	ID        int64     `json:"id"`
	Scope     string    `json:"scope"` // global | project | branch
	ProjectID *int64    `json:"project_id,omitempty"`
	Branch    *string   `json:"branch,omitempty"`
	Strategy  string    `json:"strategy"` // auto | previous_version | number_of_days | specific_analysis | reference_branch
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewCodePeriodRepository provides CRUD and hierarchical resolution for new_code_periods.
type NewCodePeriodRepository struct {
	db *DB
}

// NewNewCodePeriodRepository creates a NewCodePeriodRepository backed by db.
func NewNewCodePeriodRepository(db *DB) *NewCodePeriodRepository {
	return &NewCodePeriodRepository{db: db}
}

// GetGlobal returns the global new code period setting.
func (r *NewCodePeriodRepository) GetGlobal(ctx context.Context) (*NewCodePeriod, error) {
	return r.getByScope(ctx, "global", nil, nil)
}

// GetForProject returns the project-level setting (without branch).
func (r *NewCodePeriodRepository) GetForProject(ctx context.Context, projectID int64) (*NewCodePeriod, error) {
	return r.getByScope(ctx, "project", &projectID, nil)
}

// GetForBranch returns the branch-level setting.
func (r *NewCodePeriodRepository) GetForBranch(ctx context.Context, projectID int64, branch string) (*NewCodePeriod, error) {
	return r.getByScope(ctx, "branch", &projectID, &branch)
}

// Resolve returns the most specific applicable setting: branch > project > global.
func (r *NewCodePeriodRepository) Resolve(ctx context.Context, projectID int64, branch string) (*NewCodePeriod, error) {
	if branch != "" {
		p, err := r.GetForBranch(ctx, projectID, branch)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	p, err := r.GetForProject(ctx, projectID)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	return r.GetGlobal(ctx)
}

// SetGlobal upserts the global setting.
func (r *NewCodePeriodRepository) SetGlobal(ctx context.Context, strategy, value string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO new_code_periods (scope, strategy, value)
		VALUES ('global', $1, $2)
		ON CONFLICT (scope, project_id, branch) DO UPDATE
		  SET strategy = EXCLUDED.strategy, value = EXCLUDED.value, updated_at = now()`,
		strategy, value)
	return err
}

// SetForProject upserts a project-level setting.
func (r *NewCodePeriodRepository) SetForProject(ctx context.Context, projectID int64, strategy, value string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO new_code_periods (scope, project_id, strategy, value)
		VALUES ('project', $1, $2, $3)
		ON CONFLICT (scope, project_id, branch) DO UPDATE
		  SET strategy = EXCLUDED.strategy, value = EXCLUDED.value, updated_at = now()`,
		projectID, strategy, value)
	return err
}

// SetForBranch upserts a branch-level setting.
func (r *NewCodePeriodRepository) SetForBranch(ctx context.Context, projectID int64, branch, strategy, value string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO new_code_periods (scope, project_id, branch, strategy, value)
		VALUES ('branch', $1, $2, $3, $4)
		ON CONFLICT (scope, project_id, branch) DO UPDATE
		  SET strategy = EXCLUDED.strategy, value = EXCLUDED.value, updated_at = now()`,
		projectID, branch, strategy, value)
	return err
}

// DeleteForProject removes a project-level override (falls back to global).
func (r *NewCodePeriodRepository) DeleteForProject(ctx context.Context, projectID int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM new_code_periods WHERE scope = 'project' AND project_id = $1`, projectID)
	return err
}

// DeleteForBranch removes a branch-level override.
func (r *NewCodePeriodRepository) DeleteForBranch(ctx context.Context, projectID int64, branch string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM new_code_periods WHERE scope = 'branch' AND project_id = $1 AND branch = $2`,
		projectID, branch)
	return err
}

func (r *NewCodePeriodRepository) getByScope(ctx context.Context, scope string, projectID *int64, branch *string) (*NewCodePeriod, error) {
	ncp := &NewCodePeriod{}
	var row pgx.Row
	switch scope {
	case "global":
		row = r.db.Pool.QueryRow(ctx, `
			SELECT id, scope, project_id, branch, strategy, value, created_at, updated_at
			FROM new_code_periods WHERE scope = 'global' LIMIT 1`)
	case "project":
		row = r.db.Pool.QueryRow(ctx, `
			SELECT id, scope, project_id, branch, strategy, value, created_at, updated_at
			FROM new_code_periods WHERE scope = 'project' AND project_id = $1 LIMIT 1`,
			*projectID)
	default: // branch
		row = r.db.Pool.QueryRow(ctx, `
			SELECT id, scope, project_id, branch, strategy, value, created_at, updated_at
			FROM new_code_periods WHERE scope = 'branch' AND project_id = $1 AND branch = $2 LIMIT 1`,
			*projectID, *branch)
	}
	err := row.Scan(&ncp.ID, &ncp.Scope, &ncp.ProjectID, &ncp.Branch,
		&ncp.Strategy, &ncp.Value, &ncp.CreatedAt, &ncp.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ncp, err
}
