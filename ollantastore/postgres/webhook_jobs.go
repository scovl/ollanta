package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// WebhookJob is the durable notification delivery job stored in PostgreSQL.
type WebhookJob struct {
	ID               int64      `json:"id"`
	WebhookID        int64      `json:"webhook_id"`
	ProjectID        *int64     `json:"project_id,omitempty"`
	Event            string     `json:"event"`
	Payload          []byte     `json:"-"`
	Status           string     `json:"status"`
	WorkerID         string     `json:"worker_id,omitempty"`
	Attempts         int        `json:"attempts"`
	LastError        string     `json:"last_error,omitempty"`
	LastResponseCode *int       `json:"last_response_code,omitempty"`
	LastResponseBody *string    `json:"last_response_body,omitempty"`
	NextAttemptAt    time.Time  `json:"next_attempt_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// WebhookJobRepository provides durable queue semantics for webhook delivery jobs.
type WebhookJobRepository struct {
	db *DB
}

// NewWebhookJobRepository creates a WebhookJobRepository backed by db.
func NewWebhookJobRepository(db *DB) *WebhookJobRepository {
	return &WebhookJobRepository{db: db}
}

// Create inserts a new accepted webhook job.
func (r *WebhookJobRepository) Create(ctx context.Context, job *WebhookJob) error {
	if job.NextAttemptAt.IsZero() {
		job.NextAttemptAt = time.Now().UTC()
	}
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO webhook_jobs (
			webhook_id, project_id, event, payload, status, worker_id, attempts,
			last_error, last_response_code, last_response_body, next_attempt_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`,
		job.WebhookID, job.ProjectID, job.Event, job.Payload, job.Status, job.WorkerID,
		job.Attempts, job.LastError, job.LastResponseCode, job.LastResponseBody, job.NextAttemptAt,
	)
	return row.Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)
}

// GetByID returns a webhook job by id.
func (r *WebhookJobRepository) GetByID(ctx context.Context, id int64) (*WebhookJob, error) {
	return scanWebhookJobRow(r.db.Pool.QueryRow(ctx, `
		SELECT id, webhook_id, project_id, event, payload, status, worker_id, attempts,
		       last_error, last_response_code, last_response_body, next_attempt_at,
		       created_at, updated_at, started_at, completed_at
		FROM webhook_jobs WHERE id = $1`, id,
	))
}

// List returns webhook jobs filtered by status.
func (r *WebhookJobRepository) List(ctx context.Context, status string, limit, offset int) ([]*WebhookJob, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	countQuery := "SELECT COUNT(*) FROM webhook_jobs"
	listQuery := `
		SELECT id, webhook_id, project_id, event, payload, status, worker_id, attempts,
		       last_error, last_response_code, last_response_body, next_attempt_at,
		       created_at, updated_at, started_at, completed_at
		FROM webhook_jobs`
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

	var jobs []*WebhookJob
	for rows.Next() {
		job, err := scanWebhookJobRow(rows)
		if err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, job)
	}
	return jobs, total, rows.Err()
}

// ClaimNext marks the next due accepted webhook job as running and increments attempts.
func (r *WebhookJobRepository) ClaimNext(ctx context.Context, workerID string) (*WebhookJob, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin claim tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	job, err := scanWebhookJobRow(tx.QueryRow(ctx, `
		WITH next_job AS (
			SELECT id
			FROM webhook_jobs
			WHERE status = 'accepted'
			  AND next_attempt_at <= now()
			ORDER BY next_attempt_at ASC, id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE webhook_jobs AS j
		SET status = 'running',
		    worker_id = $1,
		    attempts = j.attempts + 1,
		    started_at = COALESCE(j.started_at, now()),
		    updated_at = now()
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING j.id, j.webhook_id, j.project_id, j.event, j.payload, j.status, j.worker_id,
		          j.attempts, j.last_error, j.last_response_code, j.last_response_body,
		          j.next_attempt_at, j.created_at, j.updated_at, j.started_at, j.completed_at`, workerID,
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
func (r *WebhookJobRepository) Reschedule(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time, responseCode *int, responseBody *string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE webhook_jobs
		SET status = 'accepted',
		    last_error = $2,
		    last_response_code = $3,
		    last_response_body = $4,
		    next_attempt_at = $5,
		    updated_at = now()
		WHERE id = $1`, id, lastError, responseCode, responseBody, nextAttemptAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkCompleted marks a webhook job as successfully delivered.
func (r *WebhookJobRepository) MarkCompleted(ctx context.Context, id int64, responseCode *int, responseBody *string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE webhook_jobs
		SET status = 'completed',
		    last_error = '',
		    last_response_code = $2,
		    last_response_body = $3,
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1`, id, responseCode, responseBody,
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
func (r *WebhookJobRepository) MarkFailed(ctx context.Context, id int64, lastError string, responseCode *int, responseBody *string) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE webhook_jobs
		SET status = 'failed',
		    last_error = $2,
		    last_response_code = $3,
		    last_response_body = $4,
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1`, id, lastError, responseCode, responseBody,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Retry resets a failed webhook job so it can be claimed again.
func (r *WebhookJobRepository) Retry(ctx context.Context, id int64) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE webhook_jobs
		SET status = 'accepted',
		    worker_id = '',
		    last_error = '',
		    last_response_code = NULL,
		    last_response_body = NULL,
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

func scanWebhookJobRow(row pgx.Row) (*WebhookJob, error) {
	job := &WebhookJob{}
	var projectID sql.NullInt64
	var responseCode sql.NullInt64
	var responseBody sql.NullString
	var startedAt sql.NullTime
	var completedAt sql.NullTime
	err := row.Scan(
		&job.ID,
		&job.WebhookID,
		&projectID,
		&job.Event,
		&job.Payload,
		&job.Status,
		&job.WorkerID,
		&job.Attempts,
		&job.LastError,
		&responseCode,
		&responseBody,
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
	if projectID.Valid {
		value := projectID.Int64
		job.ProjectID = &value
	}
	if responseCode.Valid {
		value := int(responseCode.Int64)
		job.LastResponseCode = &value
	}
	if responseBody.Valid {
		value := responseBody.String
		job.LastResponseBody = &value
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
