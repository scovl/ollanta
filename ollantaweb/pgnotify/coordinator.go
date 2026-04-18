// Package pgnotify provides a Postgres LISTEN/NOTIFY based coordinator for
// distributed index jobs. It replaces the in-process channel queue used by
// ingest.Worker so that multiple ollantaweb replicas can coordinate indexing
// without duplicating work.
//
// Architecture:
//
//	Producer (any replica)                    Consumer (one replica wins)
//	    │                                           │
//	    ├─ INSERT INTO search_index_jobs ...         │
//	    ├─ NOTIFY search_index_ready                 │
//	    │                                           ├─ LISTEN search_index_ready
//	    │                                           ├─ SELECT ... FOR UPDATE SKIP LOCKED
//	    │                                           ├─ index the issues
//	    │                                           └─ DELETE FROM search_index_jobs
//
// Only one replica processes each job because of the advisory lock via
// FOR UPDATE SKIP LOCKED. Jobs survive pod restarts because they live in
// Postgres — not in an in-memory channel.
package pgnotify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
)

const (
	channel = "search_index_ready"
)

// Coordinator listens for Postgres NOTIFY events and processes search index
// jobs one at a time, using FOR UPDATE SKIP LOCKED to prevent duplicate work
// across replicas.
type Coordinator struct {
	pool       *pgxpool.Pool
	indexer    search.IIndexer
	issues     *postgres.IssueRepository
	maxRetries int
}

// IndexJob is the row stored in the search_index_jobs table.
type IndexJob struct {
	ID         int64  `json:"id"`
	ScanID     int64  `json:"scan_id"`
	ProjectID  int64  `json:"project_id"`
	ProjectKey string `json:"project_key"`
}

// NewCoordinator creates a coordinator backed by the given pool and indexer.
func NewCoordinator(pool *pgxpool.Pool, indexer search.IIndexer, issues *postgres.IssueRepository) *Coordinator {
	return &Coordinator{
		pool:       pool,
		indexer:    indexer,
		issues:     issues,
		maxRetries: 3,
	}
}

// EnsureTable creates the search_index_jobs table if it does not exist.
// Safe to call on every startup.
func (c *Coordinator) EnsureTable(ctx context.Context) error {
	_, err := c.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS search_index_jobs (
			id          BIGSERIAL PRIMARY KEY,
			scan_id     BIGINT  NOT NULL,
			project_id  BIGINT  NOT NULL,
			project_key TEXT    NOT NULL,
			attempts    INT     NOT NULL DEFAULT 0,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	return err
}

// Enqueue inserts a job into the table and sends a NOTIFY on the channel.
// This is called by the ingest pipeline after persisting a scan.
func (c *Coordinator) Enqueue(ctx context.Context, scanID, projectID int64, projectKey string) error {
	_, err := c.pool.Exec(ctx, `
		INSERT INTO search_index_jobs (scan_id, project_id, project_key)
		VALUES ($1, $2, $3);
		NOTIFY `+channel, scanID, projectID, projectKey)
	return err
}

// Start listens for notifications and processes jobs until ctx is cancelled.
// Must be called in a goroutine.
func (c *Coordinator) Start(ctx context.Context) {
	for {
		if err := c.listenLoop(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("pgnotify: listen loop error: %v (reconnecting in 5s)", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (c *Coordinator) listenLoop(ctx context.Context) error {
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return fmt.Errorf("LISTEN: %w", err)
	}

	// Drain any pending jobs on startup before waiting for notifications.
	c.drainJobs(ctx)

	for {
		// WaitForNotification blocks until a notification arrives or ctx is cancelled.
		_, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		c.drainJobs(ctx)
	}
}

// drainJobs processes all available jobs using FOR UPDATE SKIP LOCKED.
func (c *Coordinator) drainJobs(ctx context.Context) {
	for {
		processed, err := c.processOne(ctx)
		if err != nil {
			log.Printf("pgnotify: process job error: %v", err)
			return
		}
		if !processed {
			return
		}
	}
}

// processOne claims and processes a single job. Returns false if no job was available.
func (c *Coordinator) processOne(ctx context.Context) (bool, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var job IndexJob
	err = tx.QueryRow(ctx, `
		SELECT id, scan_id, project_id, project_key
		FROM search_index_jobs
		ORDER BY id
		LIMIT 1
		FOR UPDATE SKIP LOCKED`).Scan(&job.ID, &job.ScanID, &job.ProjectID, &job.ProjectKey)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return false, nil
		}
		return false, fmt.Errorf("select job: %w", err)
	}

	// Process the indexing
	if err := c.indexJob(ctx, job); err != nil {
		// Increment attempts; if too many, move to dead-letter log and delete
		var attempts int
		_ = tx.QueryRow(ctx, `
			UPDATE search_index_jobs SET attempts = attempts + 1
			WHERE id = $1 RETURNING attempts`, job.ID).Scan(&attempts)

		if attempts >= c.maxRetries {
			log.Printf("pgnotify: dead-letter job %d (scan %d) after %d attempts: %v",
				job.ID, job.ScanID, attempts, err)
			_, _ = tx.Exec(ctx, "DELETE FROM search_index_jobs WHERE id = $1", job.ID)
		}
		_ = tx.Commit(ctx)
		return true, nil
	}

	// Success — remove the job
	_, _ = tx.Exec(ctx, "DELETE FROM search_index_jobs WHERE id = $1", job.ID)
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

func (c *Coordinator) indexJob(ctx context.Context, job IndexJob) error {
	sid := job.ScanID
	pid := job.ProjectID

	issues, _, err := c.issues.Query(ctx, postgres.IssueFilter{
		ScanID:    &sid,
		ProjectID: &pid,
		Limit:     10000,
	})
	if err != nil {
		return fmt.Errorf("query issues for scan %d: %w", job.ScanID, err)
	}

	rows := make([]postgres.IssueRow, len(issues))
	for i, iss := range issues {
		rows[i] = *iss
	}
	return c.indexer.IndexIssues(ctx, job.ProjectKey, rows)
}

// EnqueuePayload is used by the ingest pipeline to fire-and-forget.
// It serialises the job to JSON for the NOTIFY payload (future use).
func EnqueuePayload(scanID, projectID int64, projectKey string) ([]byte, error) {
	return json.Marshal(IndexJob{
		ScanID:     scanID,
		ProjectID:  projectID,
		ProjectKey: projectKey,
	})
}
