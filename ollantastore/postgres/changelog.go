package postgres

import (
	"context"
	"fmt"
	"time"
)

// ChangelogEntry represents a single field change on an issue.
// Inspired by SonarQube's api/issues/changelog which provides
// structured diffs per field change with user/date attribution.
type ChangelogEntry struct {
	ID        int64     `json:"id"`
	IssueID   int64     `json:"issue_id"`
	UserID    int64     `json:"user_id,omitempty"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	CreatedAt time.Time `json:"created_at"`
}

// ChangelogRepository manages issue change history.
type ChangelogRepository struct {
	db *DB
}

// NewChangelogRepository creates a new ChangelogRepository.
func NewChangelogRepository(db *DB) *ChangelogRepository {
	return &ChangelogRepository{db: db}
}

// Insert records a single field change for an issue.
func (r *ChangelogRepository) Insert(ctx context.Context, entry *ChangelogEntry) error {
	return r.db.Pool.QueryRow(ctx, `
		INSERT INTO issue_changelog (issue_id, user_id, field, old_value, new_value)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		entry.IssueID, entry.UserID, entry.Field, entry.OldValue, entry.NewValue,
	).Scan(&entry.ID, &entry.CreatedAt)
}

// InsertBatch records multiple field changes in a single transaction.
func (r *ChangelogRepository) InsertBatch(ctx context.Context, entries []ChangelogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for i := range entries {
		e := &entries[i]
		if err := tx.QueryRow(ctx, `
			INSERT INTO issue_changelog (issue_id, user_id, field, old_value, new_value)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, created_at`,
			e.IssueID, e.UserID, e.Field, e.OldValue, e.NewValue,
		).Scan(&e.ID, &e.CreatedAt); err != nil {
			return fmt.Errorf("insert changelog: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// ListByIssue returns all changelog entries for an issue, most recent first.
func (r *ChangelogRepository) ListByIssue(ctx context.Context, issueID int64) ([]ChangelogEntry, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, issue_id, user_id, field, old_value, new_value, created_at
		FROM issue_changelog
		WHERE issue_id = $1
		ORDER BY created_at DESC`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ChangelogEntry
	for rows.Next() {
		var e ChangelogEntry
		if err := rows.Scan(&e.ID, &e.IssueID, &e.UserID, &e.Field, &e.OldValue, &e.NewValue, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
