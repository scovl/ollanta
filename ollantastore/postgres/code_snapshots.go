package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/scovl/ollanta/domain/model"
)

// CodeSnapshotScope stores latest-per-scope snapshot metadata.
type CodeSnapshotScope struct {
	ProjectID       int64     `json:"project_id"`
	ScopeType       string    `json:"scope_type"`
	ScopeKey        string    `json:"scope_key"`
	ScanID          int64     `json:"scan_id"`
	Branch          string    `json:"branch"`
	PullRequestKey  string    `json:"pull_request_key,omitempty"`
	PullRequestBase string    `json:"pull_request_base,omitempty"`
	TotalFiles      int       `json:"total_files"`
	StoredFiles     int       `json:"stored_files"`
	TruncatedFiles  int       `json:"truncated_files"`
	OmittedFiles    int       `json:"omitted_files"`
	StoredBytes     int       `json:"stored_bytes"`
	MaxFileBytes    int       `json:"max_file_bytes"`
	MaxTotalBytes   int       `json:"max_total_bytes"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CodeSnapshotFile stores a single latest snapshot file for a scope.
type CodeSnapshotFile struct {
	Path          string    `json:"path"`
	Language      string    `json:"language"`
	Content       string    `json:"content,omitempty"`
	SizeBytes     int       `json:"size_bytes"`
	LineCount     int       `json:"line_count"`
	IsTruncated   bool      `json:"is_truncated"`
	IsOmitted     bool      `json:"is_omitted"`
	OmittedReason string    `json:"omitted_reason,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CodeSnapshotManifest is the full latest snapshot payload returned by the API layer.
type CodeSnapshotManifest struct {
	Scope *CodeSnapshotScope  `json:"scope"`
	Files []*CodeSnapshotFile `json:"files"`
}

// CodeSnapshotRepository provides latest-per-scope snapshot persistence.
type CodeSnapshotRepository struct {
	db *DB
}

// NewCodeSnapshotRepository creates a CodeSnapshotRepository backed by db.
func NewCodeSnapshotRepository(db *DB) *CodeSnapshotRepository {
	return &CodeSnapshotRepository{db: db}
}

// Replace upserts the scope metadata and replaces all file rows for that scope in one transaction.
func (r *CodeSnapshotRepository) Replace(ctx context.Context, state *model.CodeSnapshotState) error {
	if state == nil {
		return nil
	}
	scope := state.Scope.Normalize()
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		INSERT INTO code_snapshot_scopes (
			project_id, scope_type, scope_key, scan_id, branch,
			pull_request_key, pull_request_base,
			total_files, stored_files, truncated_files, omitted_files,
			stored_bytes, max_file_bytes, max_total_bytes, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, now()
		)
		ON CONFLICT (project_id, scope_type, scope_key) DO UPDATE
		SET scan_id = EXCLUDED.scan_id,
		    branch = EXCLUDED.branch,
		    pull_request_key = EXCLUDED.pull_request_key,
		    pull_request_base = EXCLUDED.pull_request_base,
		    total_files = EXCLUDED.total_files,
		    stored_files = EXCLUDED.stored_files,
		    truncated_files = EXCLUDED.truncated_files,
		    omitted_files = EXCLUDED.omitted_files,
		    stored_bytes = EXCLUDED.stored_bytes,
		    max_file_bytes = EXCLUDED.max_file_bytes,
		    max_total_bytes = EXCLUDED.max_total_bytes,
		    updated_at = now()`,
		state.ProjectID, scope.Type, scope.Key(), state.ScanID, scope.Branch,
		scope.PullRequestKey, scope.PullRequestBase,
		state.Snapshot.TotalFiles, state.Snapshot.StoredFiles, state.Snapshot.TruncatedFiles, state.Snapshot.OmittedFiles,
		state.Snapshot.StoredBytes, state.Snapshot.MaxFileBytes, state.Snapshot.MaxTotalBytes,
	)
	if err != nil {
		return fmt.Errorf("upsert code snapshot scope: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM code_snapshot_files
		WHERE project_id = $1 AND scope_type = $2 AND scope_key = $3`,
		state.ProjectID, scope.Type, scope.Key(),
	); err != nil {
		return fmt.Errorf("delete code snapshot files: %w", err)
	}

	for _, file := range state.Snapshot.Files {
		if _, err := tx.Exec(ctx, `
			INSERT INTO code_snapshot_files (
				project_id, scope_type, scope_key, path, language, content,
				size_bytes, line_count, is_truncated, is_omitted, omitted_reason, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, $11, now()
			)`,
			state.ProjectID, scope.Type, scope.Key(), file.Path, file.Language, file.Content,
			file.SizeBytes, file.LineCount, file.IsTruncated, file.IsOmitted, file.OmittedReason,
		); err != nil {
			return fmt.Errorf("insert code snapshot file %s: %w", file.Path, err)
		}
	}

	return tx.Commit(ctx)
}

// GetManifest returns the scope metadata and all files for the requested latest snapshot.
func (r *CodeSnapshotRepository) GetManifest(ctx context.Context, projectID int64, scopeType, scopeKey string) (*CodeSnapshotManifest, error) {
	scope, err := r.GetScope(ctx, projectID, scopeType, scopeKey)
	if err != nil {
		return nil, err
	}
	manifest := &CodeSnapshotManifest{Scope: scope}
	files, err := r.ListFiles(ctx, projectID, scopeType, scopeKey)
	if err != nil {
		return nil, err
	}
	manifest.Files = files
	return manifest, nil
}

// GetScope returns only the scope metadata for the latest snapshot.
func (r *CodeSnapshotRepository) GetScope(ctx context.Context, projectID int64, scopeType, scopeKey string) (*CodeSnapshotScope, error) {
	scope := &CodeSnapshotScope{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT project_id, scope_type, scope_key, scan_id, branch,
		       pull_request_key, pull_request_base,
		       total_files, stored_files, truncated_files, omitted_files,
		       stored_bytes, max_file_bytes, max_total_bytes, updated_at
		FROM code_snapshot_scopes
		WHERE project_id = $1 AND scope_type = $2 AND scope_key = $3`,
		projectID, scopeType, scopeKey,
	).Scan(
		&scope.ProjectID, &scope.ScopeType, &scope.ScopeKey, &scope.ScanID, &scope.Branch,
		&scope.PullRequestKey, &scope.PullRequestBase,
		&scope.TotalFiles, &scope.StoredFiles, &scope.TruncatedFiles, &scope.OmittedFiles,
		&scope.StoredBytes, &scope.MaxFileBytes, &scope.MaxTotalBytes, &scope.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return scope, err
}

// ListFiles returns all latest snapshot files for the requested scope, ordered by path.
func (r *CodeSnapshotRepository) ListFiles(ctx context.Context, projectID int64, scopeType, scopeKey string) ([]*CodeSnapshotFile, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT path, language, content, size_bytes, line_count,
		       is_truncated, is_omitted, omitted_reason, updated_at
		FROM code_snapshot_files
		WHERE project_id = $1 AND scope_type = $2 AND scope_key = $3
		ORDER BY path ASC`,
		projectID, scopeType, scopeKey,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*CodeSnapshotFile
	for rows.Next() {
		file := &CodeSnapshotFile{}
		if err := rows.Scan(
			&file.Path, &file.Language, &file.Content, &file.SizeBytes, &file.LineCount,
			&file.IsTruncated, &file.IsOmitted, &file.OmittedReason, &file.UpdatedAt,
		); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if files == nil {
		files = []*CodeSnapshotFile{}
	}
	return files, rows.Err()
}

// GetFile returns a single latest snapshot file by relative path.
func (r *CodeSnapshotRepository) GetFile(ctx context.Context, projectID int64, scopeType, scopeKey, path string) (*CodeSnapshotFile, error) {
	file := &CodeSnapshotFile{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT path, language, content, size_bytes, line_count,
		       is_truncated, is_omitted, omitted_reason, updated_at
		FROM code_snapshot_files
		WHERE project_id = $1 AND scope_type = $2 AND scope_key = $3 AND path = $4`,
		projectID, scopeType, scopeKey, path,
	).Scan(
		&file.Path, &file.Language, &file.Content, &file.SizeBytes, &file.LineCount,
		&file.IsTruncated, &file.IsOmitted, &file.OmittedReason, &file.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return file, err
}
