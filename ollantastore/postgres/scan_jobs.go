package postgres

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const statusWhereClause = " WHERE status = $1"

const scanJobSelectColumns = `id, project_key, status, payload, idempotency_key, payload_hash,
       trace_parent, trace_state, scan_id, worker_id, attempts, last_error,
       created_at, updated_at, started_at, completed_at`

// JobRecoveryResult summarizes stale job recovery outcomes.
type JobRecoveryResult struct {
	Requeued int64
	Failed   int64
}

// ScanQueuePressure summarizes durable scan queue pressure for backpressure decisions.
type ScanQueuePressure struct {
	Accepted          int
	Running           int
	OldestAcceptedAge time.Duration
}

// ScanJob is the durable intake record stored in PostgreSQL.
type ScanJob struct {
	ID             int64      `json:"id"`
	ProjectKey     string     `json:"project_key"`
	Status         string     `json:"status"`
	Payload        []byte     `json:"-"`
	IdempotencyKey string     `json:"-"`
	PayloadHash    string     `json:"-"`
	TraceParent    string     `json:"-"`
	TraceState     string     `json:"-"`
	ScanID         *int64     `json:"scan_id,omitempty"`
	WorkerID       string     `json:"worker_id,omitempty"`
	Attempts       int        `json:"attempts"`
	LastError      string     `json:"last_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
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
		INSERT INTO scan_jobs (project_key, status, payload, idempotency_key, payload_hash, trace_parent, trace_state, worker_id, attempts, last_error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`,
		job.ProjectKey, job.Status, job.Payload, job.IdempotencyKey, job.PayloadHash,
		job.TraceParent, job.TraceState, job.WorkerID, job.Attempts, job.LastError,
	)
	return row.Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)
}

// GetByID retrieves a scan job by primary key.
func (r *ScanJobRepository) GetByID(ctx context.Context, id int64) (*ScanJob, error) {
	return scanJobFromRow(r.db.Pool.QueryRow(ctx, `
		SELECT `+scanJobSelectColumns+`
		FROM scan_jobs
		WHERE id = $1`, id,
	))
}

// GetByScanID retrieves the completed scan job that produced a scan.
func (r *ScanJobRepository) GetByScanID(ctx context.Context, scanID int64) (*ScanJob, error) {
	return scanJobFromRow(r.db.Pool.QueryRow(ctx, `
		SELECT `+scanJobSelectColumns+`
		FROM scan_jobs
		WHERE scan_id = $1
		ORDER BY completed_at DESC NULLS LAST, updated_at DESC, id DESC
		LIMIT 1`, scanID,
	))
}

// FindByIdempotencyKey retrieves a scan job by project-scoped idempotency identity.
func (r *ScanJobRepository) FindByIdempotencyKey(ctx context.Context, projectKey, idempotencyKey string) (*ScanJob, error) {
	if projectKey == "" || idempotencyKey == "" {
		return nil, ErrNotFound
	}
	return scanJobFromRow(r.db.Pool.QueryRow(ctx, `
		SELECT `+scanJobSelectColumns+`
		FROM scan_jobs
		WHERE project_key = $1 AND idempotency_key = $2
		LIMIT 1`, projectKey, idempotencyKey,
	))
}

// CountByStatus returns the number of durable scan jobs in the given state.
func (r *ScanJobRepository) CountByStatus(ctx context.Context, status string) (int, error) {
	query := "SELECT COUNT(*) FROM scan_jobs"
	args := []any{}
	if status != "" {
		query += statusWhereClause
		args = append(args, status)
	}

	var total int
	err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

// QueuePressure returns accepted/running counts and oldest accepted age for durable backpressure.
func (r *ScanJobRepository) QueuePressure(ctx context.Context, projectKey string, now time.Time) (ScanQueuePressure, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'accepted'),
			COUNT(*) FILTER (WHERE status = 'running'),
			EXTRACT(EPOCH FROM ($2::timestamptz - MIN(created_at) FILTER (WHERE status = 'accepted')))
		FROM scan_jobs
		WHERE ($1 = '' OR project_key = $1)`

	var pressure ScanQueuePressure
	var oldestAgeSeconds sql.NullFloat64
	if err := r.db.Pool.QueryRow(ctx, query, projectKey, now).Scan(&pressure.Accepted, &pressure.Running, &oldestAgeSeconds); err != nil {
		return ScanQueuePressure{}, err
	}
	if oldestAgeSeconds.Valid && oldestAgeSeconds.Float64 > 0 {
		pressure.OldestAcceptedAge = time.Duration(oldestAgeSeconds.Float64 * float64(time.Second))
	}
	return pressure, nil
}

// RecoverStale requeues or fails running scan jobs that have not updated recently.
func (r *ScanJobRepository) RecoverStale(ctx context.Context, staleBefore time.Time, maxAttempts int, failureMessage string) (JobRecoveryResult, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return JobRecoveryResult{}, fmt.Errorf("begin scan job recovery tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	requeued, err := tx.Exec(ctx, `
		UPDATE scan_jobs
		SET status = 'accepted',
		    worker_id = '',
		    last_error = '',
		    started_at = NULL,
		    updated_at = now()
		WHERE status = 'running'
		  AND updated_at < $1
		  AND attempts < $2`, staleBefore, maxAttempts,
	)
	if err != nil {
		return JobRecoveryResult{}, err
	}

	failed, err := tx.Exec(ctx, `
		UPDATE scan_jobs
		SET status = 'failed',
		    last_error = $3,
		    completed_at = now(),
		    updated_at = now()
		WHERE status = 'running'
		  AND updated_at < $1
		  AND attempts >= $2`, staleBefore, maxAttempts, failureMessage,
	)
	if err != nil {
		return JobRecoveryResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return JobRecoveryResult{}, fmt.Errorf("commit scan job recovery tx: %w", err)
	}
	return JobRecoveryResult{Requeued: requeued.RowsAffected(), Failed: failed.RowsAffected()}, nil
}

// List returns scan jobs matching the provided filter.
func (r *ScanJobRepository) List(ctx context.Context, filter JobListFilter) ([]*ScanJob, int, error) {
	filter.Limit = boundedJobLimit(filter.Limit)
	where, args := buildScanJobWhere(filter)

	var total int
	if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM scan_jobs"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT ` + scanJobSelectColumns + `
		FROM scan_jobs` + where + " ORDER BY created_at DESC, id DESC"
	args = append(args, filter.Limit, filter.Offset)
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	jobs := []*ScanJob{}
	for rows.Next() {
		job, err := scanJobFromRow(rows)
		if err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, job)
	}
	return jobs, total, rows.Err()
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
		    attempts = j.attempts + 1,
		    last_error = '',
		    started_at = COALESCE(j.started_at, now()),
		    updated_at = now()
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING j.id, j.project_key, j.status, j.payload, j.idempotency_key, j.payload_hash,
		          j.trace_parent, j.trace_state, j.scan_id, j.worker_id, j.attempts, j.last_error,
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

// Retry resets a failed or cancelled scan job so it can be claimed again.
func (r *ScanJobRepository) Retry(ctx context.Context, id int64) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE scan_jobs
		SET status = 'accepted',
		    worker_id = '',
		    last_error = '',
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

// CancelQueued marks an accepted scan job as cancelled so workers will not claim it.
func (r *ScanJobRepository) CancelQueued(ctx context.Context, id int64) error {
	tag, err := r.db.Pool.Exec(ctx, `
		UPDATE scan_jobs
		SET status = 'cancelled',
		    last_error = '',
		    completed_at = now(),
		    updated_at = now()
		WHERE id = $1 AND status = 'accepted'`, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func buildScanJobWhere(filter JobListFilter) (string, []any) {
	clauses := []string{}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.Status != "" {
		add("status = $%d", filter.Status)
	}
	if filter.ProjectKey != "" {
		add("project_key = $%d", filter.ProjectKey)
	}
	if filter.ScanID != nil {
		add("scan_id = $%d", *filter.ScanID)
	}
	if filter.WorkerID != "" {
		add("worker_id = $%d", filter.WorkerID)
	}
	if filter.CreatedAfter != nil {
		add("created_at >= $%d", *filter.CreatedAfter)
	}
	if filter.CreatedBefore != nil {
		add("created_at <= $%d", *filter.CreatedBefore)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func scanJobFromRow(row pgx.Row) (*ScanJob, error) {
	job := &ScanJob{}
	var traceParent sql.NullString
	var traceState sql.NullString
	var scanID sql.NullInt64
	var startedAt sql.NullTime
	var completedAt sql.NullTime

	err := row.Scan(
		&job.ID,
		&job.ProjectKey,
		&job.Status,
		&job.Payload,
		&job.IdempotencyKey,
		&job.PayloadHash,
		&traceParent,
		&traceState,
		&scanID,
		&job.WorkerID,
		&job.Attempts,
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
	if traceParent.Valid {
		job.TraceParent = traceParent.String
	}
	if traceState.Valid {
		job.TraceState = traceState.String
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

// HashPayload returns the canonical SHA-256 hex hash used for idempotency checks.
func HashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// PayloadHashesEqual compares payload hashes without short-circuiting on content.
func PayloadHashesEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
