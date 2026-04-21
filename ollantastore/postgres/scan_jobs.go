package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ScanJob is the durable intake record stored in PostgreSQL.
type ScanJob struct {
	ID          int64      `json:"id"`
	ProjectKey  string     `json:"project_key"`
	Status      string     `json:"status"`
	Payload     []byte     `json:"-"`
	ScanID      *int64     `json:"scan_id,omitempty"`
	WorkerID    string     `json:"worker_id,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ScanJobRepository provides durable intake operations for scan jobs.
type ScanJobRepository struct {
	db *DB
}

// NewScanJobRepository creates a ScanJobRepository backed by db.
func NewScanJobRepository(db *DB) *ScanJobRepository {
	return &ScanJobRepository{db: db}
}

// Create inserts a new accepted scan job.
func (r *ScanJobRepository) Create(ctx context.Context, job *ScanJob) error {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO scan_jobs (project_key, status, payload, worker_id, last_error)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		job.ProjectKey, job.Status, job.Payload, job.WorkerID, job.LastError,
	)
	return row.Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)
}

// GetByID retrieves a scan job by primary key.
func (r *ScanJobRepository) GetByID(ctx context.Context, id int64) (*ScanJob, error) {
	return scanJobFromRow(r.db.Pool.QueryRow(ctx, `
		SELECT id, project_key, status, payload, scan_id, worker_id, last_error,
		       created_at, updated_at, started_at, completed_at
		FROM scan_jobs
		WHERE id = $1`, id,
	))
}

// ClaimNext marks the oldest accepted job as running and returns it.
func (r *ScanJobRepository) ClaimNext(ctx context.Context, workerID string) (*ScanJob, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	job, err := scanJobFromRow(tx.QueryRow(ctx, `
		WITH next_job AS (
			SELECT id
			FROM scan_jobs
			WHERE status = 'accepted'
			ORDER BY created_at ASC, id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE scan_jobs AS j
		SET status = 'running',
		    worker_id = $1,
		    last_error = '',
		    started_at = COALESCE(j.started_at, now()),
		    updated_at = now()
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING j.id, j.project_key, j.status, j.payload, j.scan_id, j.worker_id, j.last_error,
		          j.created_at, j.updated_at, j.started_at, j.completed_at`, workerID,
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

// MarkCompleted records the linked scan for a finished job.
func (r *ScanJobRepository) MarkCompleted(ctx context.Context, id, scanID int64) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE scan_jobs
		SET status = 'completed',
		    scan_id = $2,
		    last_error = '',
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1`, id, scanID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkFailed records a durable failure state for a claimed job.
func (r *ScanJobRepository) MarkFailed(ctx context.Context, id int64, lastError string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE scan_jobs
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

func scanJobFromRow(row pgx.Row) (*ScanJob, error) {
	job := &ScanJob{}
	var scanID sql.NullInt64
	var startedAt sql.NullTime
	var completedAt sql.NullTime

	err := row.Scan(
		&job.ID,
		&job.ProjectKey,
		&job.Status,
		&job.Payload,
		&scanID,
		&job.WorkerID,
		&job.LastError,
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
	if scanID.Valid {
		value := scanID.Int64
		job.ScanID = &value
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
