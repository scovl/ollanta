package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// IndexJob is the durable search projection job stored in PostgreSQL.
type IndexJob struct {
	ID            int64      `json:"id"`
	ScanID        int64      `json:"scan_id"`
	ProjectID     int64      `json:"project_id"`
	ProjectKey    string     `json:"project_key"`
	Status        string     `json:"status"`
	WorkerID      string     `json:"worker_id,omitempty"`
	Attempts      int        `json:"attempts"`
	LastError     string     `json:"last_error,omitempty"`
	NextAttemptAt time.Time  `json:"next_attempt_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// IndexJobRepository provides durable queue semantics for search indexing jobs.
type IndexJobRepository struct {
	db *DB
}

// NewIndexJobRepository creates an IndexJobRepository backed by db.
func NewIndexJobRepository(db *DB) *IndexJobRepository {
	return &IndexJobRepository{db: db}
}

// Create inserts a new accepted index job.
func (r *IndexJobRepository) Create(ctx context.Context, job *IndexJob) error {
	if job.NextAttemptAt.IsZero() {
		job.NextAttemptAt = time.Now().UTC()
	}
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO index_jobs (scan_id, project_id, project_key, status, worker_id, attempts, last_error, next_attempt_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`,
		job.ScanID, job.ProjectID, job.ProjectKey, job.Status, job.WorkerID, job.Attempts, job.LastError, job.NextAttemptAt,
	)
	return row.Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)
}

// GetByID returns an index job by id.
func (r *IndexJobRepository) GetByID(ctx context.Context, id int64) (*IndexJob, error) {
	return scanIndexJobRow(r.db.Pool.QueryRow(ctx, `
		SELECT id, scan_id, project_id, project_key, status, worker_id, attempts, last_error,
		       next_attempt_at, created_at, updated_at, started_at, completed_at
		FROM index_jobs WHERE id = $1`, id,
	))
}

// List returns index jobs filtered by status.
func (r *IndexJobRepository) List(ctx context.Context, status string, limit, offset int) ([]*IndexJob, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	countQuery := "SELECT COUNT(*) FROM index_jobs"
	listQuery := `
		SELECT id, scan_id, project_id, project_key, status, worker_id, attempts, last_error,
		       next_attempt_at, created_at, updated_at, started_at, completed_at
		FROM index_jobs`
	args := []any{}
	if status != "" {
		countQuery += " WHERE status = $1"
		listQuery += " WHERE status = $1"
		args = append(args, status)
	}

	var total int
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery += " ORDER BY created_at DESC, id DESC"
	args = append(args, limit, offset)
	listQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []*IndexJob
	for rows.Next() {
		job, err := scanIndexJobRow(rows)
		if err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, job)
	}
	return jobs, total, rows.Err()
}

// ClaimNext marks the next due accepted job as running and increments attempts.
func (r *IndexJobRepository) ClaimNext(ctx context.Context, workerID string) (*IndexJob, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	job, err := scanIndexJobRow(tx.QueryRow(ctx, `
		WITH next_job AS (
			SELECT id
			FROM index_jobs
			WHERE status = 'accepted'
			  AND next_attempt_at <= now()
			ORDER BY next_attempt_at ASC, id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE index_jobs AS j
		SET status = 'running',
		    worker_id = $1,
		    attempts = j.attempts + 1,
		    started_at = COALESCE(j.started_at, now()),
		    updated_at = now()
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING j.id, j.scan_id, j.project_id, j.project_key, j.status, j.worker_id, j.attempts,
		          j.last_error, j.next_attempt_at, j.created_at, j.updated_at, j.started_at, j.completed_at`, workerID,
	))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim tx: %w", err)
	}
	return job, nil
}

// Reschedule moves a job back to accepted with a future retry time.
func (r *IndexJobRepository) Reschedule(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE index_jobs
		SET status = 'accepted',
		    last_error = $2,
		    next_attempt_at = $3,
		    updated_at = now()
		WHERE id = $1`, id, lastError, nextAttemptAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkCompleted marks a job as successfully completed.
func (r *IndexJobRepository) MarkCompleted(ctx context.Context, id int64) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE index_jobs
		SET status = 'completed',
		    last_error = '',
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1`, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkFailed marks a job as permanently failed.
func (r *IndexJobRepository) MarkFailed(ctx context.Context, id int64, lastError string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE index_jobs
		SET status = 'failed',
		    last_error = $2,
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1`, id, lastError,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Retry resets a failed job so it can be claimed again.
func (r *IndexJobRepository) Retry(ctx context.Context, id int64) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE index_jobs
		SET status = 'accepted',
		    worker_id = '',
		    last_error = '',
		    next_attempt_at = now(),
		    started_at = NULL,
		    completed_at = NULL,
		    updated_at = now()
		WHERE id = $1`, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanIndexJobRow(row pgx.Row) (*IndexJob, error) {
	job := &IndexJob{}
	var startedAt sql.NullTime
	var completedAt sql.NullTime
	err := row.Scan(
		&job.ID,
		&job.ScanID,
		&job.ProjectID,
		&job.ProjectKey,
		&job.Status,
		&job.WorkerID,
		&job.Attempts,
		&job.LastError,
		&job.NextAttemptAt,
		&job.CreatedAt,
		&job.UpdatedAt,
		&startedAt,
		&completedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		value := startedAt.Time
		job.StartedAt = &value
	}
	if completedAt.Valid {
		value := completedAt.Time
		job.CompletedAt = &value
	}
	return job, nil
}
