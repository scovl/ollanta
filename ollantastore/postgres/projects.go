package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/scovl/ollanta/domain/model"
)

// Project is the canonical project record stored in PostgreSQL.
type Project struct {
	ID          int64     `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MainBranch  string    `json:"main_branch"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = model.ErrNotFound

// ProjectRepository provides CRUD access to the projects table.
type ProjectRepository struct {
	db *DB
}

// NewProjectRepository creates a ProjectRepository backed by db.
func NewProjectRepository(db *DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

// Upsert inserts a new project or updates name/description/tags/updated_at on key conflict.
// The ID field is populated on return.
func (r *ProjectRepository) Upsert(ctx context.Context, p *Project) error {
	if p.Tags == nil {
		p.Tags = []string{}
	}
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO projects (key, name, description, main_branch, tags, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (key) DO UPDATE
		  SET name        = EXCLUDED.name,
		      description = EXCLUDED.description,
		      main_branch = EXCLUDED.main_branch,
		      tags        = EXCLUDED.tags,
		      updated_at  = now()
		RETURNING id, created_at, updated_at`,
		p.Key, p.Name, p.Description, p.MainBranch, p.Tags,
	)
	return row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

// Create inserts a new project and populates its ID and timestamps.
func (r *ProjectRepository) Create(ctx context.Context, p *Project) error {
	if p.Tags == nil {
		p.Tags = []string{}
	}
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO projects (key, name, description, main_branch, tags)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		p.Key, p.Name, p.Description, p.MainBranch, p.Tags,
	)
	return row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

// GetByKey retrieves a project by its unique key. Returns ErrNotFound when absent.
func (r *ProjectRepository) GetByKey(ctx context.Context, key string) (*Project, error) {
	p := &Project{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, key, name, description, main_branch, tags, created_at, updated_at
		FROM projects WHERE key = $1`, key,
	).Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.MainBranch, &p.Tags, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// GetByID retrieves a project by its primary key. Returns ErrNotFound when absent.
func (r *ProjectRepository) GetByID(ctx context.Context, id int64) (*Project, error) {
	p := &Project{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, key, name, description, main_branch, tags, created_at, updated_at
		FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.MainBranch, &p.Tags, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// List returns a page of projects ordered by created_at DESC, plus the total count.
func (r *ProjectRepository) List(ctx context.Context, limit, offset int) ([]*Project, int, error) {
	if limit <= 0 {
		limit = 20
	}

	var total int
	if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM projects").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, key, name, description, main_branch, tags, created_at, updated_at
		FROM projects
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		p := &Project{}
		if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.MainBranch,
			&p.Tags, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		projects = append(projects, p)
	}
	return projects, total, rows.Err()
}

// Delete removes a project and cascades to scans and issues.
func (r *ProjectRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Pool.Exec(ctx, "DELETE FROM projects WHERE id = $1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
