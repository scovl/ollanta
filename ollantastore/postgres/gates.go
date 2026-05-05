package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// QualityGate is a named set of metric conditions.
type QualityGate struct {
	ID                  int64     `json:"id"`
	Name                string    `json:"name"`
	IsDefault           bool      `json:"is_default"`
	IsBuiltin           bool      `json:"is_builtin"`
	SmallChangesetLines int       `json:"small_changeset_lines"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// GateCondition is a single metric threshold condition attached to a gate.
type GateCondition struct {
	ID               int64    `json:"id"`
	GateID           int64    `json:"gate_id"`
	Metric           string   `json:"metric"`
	Operator         string   `json:"operator"` // GT, LT, GTE, LTE, EQ, NE
	Threshold        float64  `json:"threshold"`
	WarningThreshold *float64 `json:"warning_threshold,omitempty"`
	OnNewCode        bool     `json:"on_new_code"`
}

// GateRepository provides CRUD access to quality_gates and related tables.
type GateRepository struct {
	db *DB
}

// NewGateRepository creates a GateRepository backed by db.
func NewGateRepository(db *DB) *GateRepository {
	return &GateRepository{db: db}
}

// List returns all quality gates.
func (r *GateRepository) List(ctx context.Context) ([]*QualityGate, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, name, is_default, is_builtin, small_changeset_lines, created_at, updated_at
		FROM quality_gates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGates(rows)
}

// GetByID returns a gate by its ID.
func (r *GateRepository) GetByID(ctx context.Context, id int64) (*QualityGate, error) {
	g := &QualityGate{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, name, is_default, is_builtin, small_changeset_lines, created_at, updated_at
		FROM quality_gates WHERE id = $1`, id,
	).Scan(&g.ID, &g.Name, &g.IsDefault, &g.IsBuiltin, &g.SmallChangesetLines, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return g, err
}

// Create inserts a new quality gate.
func (r *GateRepository) Create(ctx context.Context, g *QualityGate) error {
	return r.db.Pool.QueryRow(ctx, `
		INSERT INTO quality_gates (name, is_default, small_changeset_lines)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`,
		g.Name, g.IsDefault, g.SmallChangesetLines,
	).Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
}

// Update updates a non-builtin gate.
func (r *GateRepository) Update(ctx context.Context, g *QualityGate) error {
	if g.IsBuiltin {
		return fmt.Errorf("cannot update builtin gate")
	}
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE quality_gates
		SET name = $1, is_default = $2, small_changeset_lines = $3, updated_at = now()
		WHERE id = $4`,
		g.Name, g.IsDefault, g.SmallChangesetLines, g.ID)
	return err
}

// Delete removes a non-builtin gate.
func (r *GateRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM quality_gates WHERE id = $1 AND is_builtin = FALSE`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("gate not found or is builtin")
	}
	return nil
}

// Conditions returns all conditions for a gate.
func (r *GateRepository) Conditions(ctx context.Context, gateID int64) ([]*GateCondition, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, gate_id, metric, operator, threshold, warning_threshold, on_new_code
		FROM gate_conditions WHERE gate_id = $1 ORDER BY metric`, gateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConditions(rows)
}

// AddCondition inserts a new condition to a gate.
func (r *GateRepository) AddCondition(ctx context.Context, c *GateCondition) error {
	return r.db.Pool.QueryRow(ctx, `
		INSERT INTO gate_conditions (gate_id, metric, operator, threshold, warning_threshold, on_new_code)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		c.GateID, c.Metric, c.Operator, c.Threshold, c.WarningThreshold, c.OnNewCode,
	).Scan(&c.ID)
}

// RemoveCondition deletes a condition by ID.
func (r *GateRepository) RemoveCondition(ctx context.Context, conditionID int64) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM gate_conditions WHERE id = $1`, conditionID)
	return err
}

// UpdateCondition updates an existing condition.
func (r *GateRepository) UpdateCondition(ctx context.Context, c *GateCondition) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE gate_conditions
		SET metric = $1, operator = $2, threshold = $3, warning_threshold = $4, on_new_code = $5
		WHERE id = $6`,
		c.Metric, c.Operator, c.Threshold, c.WarningThreshold, c.OnNewCode, c.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Copy duplicates a gate with a new name, including all its conditions.
func (r *GateRepository) Copy(ctx context.Context, sourceID int64, newName string) (*QualityGate, error) {
	src, err := r.GetByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	newGate := &QualityGate{
		Name:                newName,
		SmallChangesetLines: src.SmallChangesetLines,
	}
	if err := r.Create(ctx, newGate); err != nil {
		return nil, fmt.Errorf("create copy: %w", err)
	}
	conditions, err := r.Conditions(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("read conditions: %w", err)
	}
	for _, c := range conditions {
		dup := &GateCondition{
			GateID:           newGate.ID,
			Metric:           c.Metric,
			Operator:         c.Operator,
			Threshold:        c.Threshold,
			WarningThreshold: c.WarningThreshold,
			OnNewCode:        c.OnNewCode,
		}
		if err := r.AddCondition(ctx, dup); err != nil {
			return nil, fmt.Errorf("copy condition: %w", err)
		}
	}
	return newGate, nil
}

// SetDefault atomically sets a gate as default and clears all other defaults.
func (r *GateRepository) SetDefault(ctx context.Context, id int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE quality_gates SET is_default = (id = $1), updated_at = now()
		 WHERE is_default = TRUE OR id = $1`, id)
	return err
}

// AssignToProject sets the active gate for a project.
func (r *GateRepository) AssignToProject(ctx context.Context, projectID, gateID int64) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO project_gates (project_id, gate_id)
		VALUES ($1, $2)
		ON CONFLICT (project_id) DO UPDATE SET gate_id = EXCLUDED.gate_id`,
		projectID, gateID)
	return err
}

// ForProject returns the gate assigned to a project, falling back to the default gate.
func (r *GateRepository) ForProject(ctx context.Context, projectID int64) (*QualityGate, []*GateCondition, error) {
	var gateID int64
	err := r.db.Pool.QueryRow(ctx,
		`SELECT gate_id FROM project_gates WHERE project_id = $1`, projectID,
	).Scan(&gateID)
	if errors.Is(err, pgx.ErrNoRows) {
		// fall back to global default
		err = r.db.Pool.QueryRow(ctx,
			`SELECT id FROM quality_gates WHERE is_default = TRUE LIMIT 1`,
		).Scan(&gateID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrNotFound
		}
		if err != nil {
			return nil, nil, err
		}
	} else if err != nil {
		return nil, nil, err
	}

	gate, err := r.GetByID(ctx, gateID)
	if err != nil {
		return nil, nil, err
	}
	conditions, err := r.Conditions(ctx, gateID)
	return gate, conditions, err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func scanGates(rows pgx.Rows) ([]*QualityGate, error) {
	var out []*QualityGate
	for rows.Next() {
		g := &QualityGate{}
		if err := rows.Scan(&g.ID, &g.Name, &g.IsDefault, &g.IsBuiltin,
			&g.SmallChangesetLines, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func scanConditions(rows pgx.Rows) ([]*GateCondition, error) {
	var out []*GateCondition
	for rows.Next() {
		c := &GateCondition{}
		if err := rows.Scan(&c.ID, &c.GateID, &c.Metric, &c.Operator, &c.Threshold, &c.WarningThreshold, &c.OnNewCode); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
