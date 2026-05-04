package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
)

// ProfileSnapshotRepository persists the quality profile policy used by a scan.
type ProfileSnapshotRepository struct {
	db *DB
}

var _ port.IProfileSnapshotRepo = (*ProfileSnapshotRepository)(nil)

// NewProfileSnapshotRepository creates a repository for scan profile snapshots.
func NewProfileSnapshotRepository(db *DB) *ProfileSnapshotRepository {
	return &ProfileSnapshotRepository{db: db}
}

// Replace replaces all profile snapshots for a scan. Empty snapshots mark legacy/unavailable metadata.
func (r *ProfileSnapshotRepository) Replace(ctx context.Context, projectID, scanID int64, scope model.AnalysisScope, snapshots []model.ProfileSnapshot) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin profile snapshot replace: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM scan_profile_snapshots WHERE scan_id = $1`, scanID); err != nil {
		return fmt.Errorf("delete profile snapshots: %w", err)
	}
	scope = scope.Normalize()
	if len(snapshots) == 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO scan_profile_snapshots (project_id, scan_id, scope_type, branch, pull_request_key, metadata_available, source)
			VALUES ($1, $2, $3, $4, $5, FALSE, $6)`,
			projectID, scanID, scope.Type, scope.Branch, scope.PullRequestKey, string(model.ProfileSourceUnknown)); err != nil {
			return fmt.Errorf("insert unavailable profile snapshot marker: %w", err)
		}
		return tx.Commit(ctx)
	}

	for _, snapshot := range snapshots {
		diagnostics, err := json.Marshal(snapshot.Diagnostics)
		if err != nil {
			return fmt.Errorf("marshal profile diagnostics: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO scan_profile_snapshots (
				project_id, scan_id, scope_type, branch, pull_request_key, language,
				profile_id, profile_name, source, active_rule_count, rules_hash,
				custom_catalog_hash, metadata_available, diagnostics
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,TRUE,$13)`,
			projectID, scanID, scope.Type, scope.Branch, scope.PullRequestKey, snapshot.Language,
			nullZeroInt64(snapshot.ProfileID), snapshot.ProfileName, string(snapshot.Source), snapshot.ActiveRuleCount,
			snapshot.RulesHash, snapshot.CustomCatalogHash, diagnostics); err != nil {
			return fmt.Errorf("insert profile snapshot %s: %w", snapshot.Language, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit profile snapshot replace: %w", err)
	}
	return nil
}

// ListByScan returns snapshots for a scan and whether profile metadata was available.
func (r *ProfileSnapshotRepository) ListByScan(ctx context.Context, scanID int64) ([]model.ProfileSnapshot, bool, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT language, COALESCE(profile_id, 0), profile_name, source, active_rule_count, rules_hash, custom_catalog_hash,
		       metadata_available, diagnostics
		FROM scan_profile_snapshots
		WHERE scan_id = $1
		ORDER BY language`, scanID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	available := false
	seenRows := false
	snapshots := []model.ProfileSnapshot{}
	for rows.Next() {
		seenRows = true
		var snapshot model.ProfileSnapshot
		var source string
		var metadataAvailable bool
		var diagnosticsRaw []byte
		if err := rows.Scan(&snapshot.Language, &snapshot.ProfileID, &snapshot.ProfileName, &source, &snapshot.ActiveRuleCount, &snapshot.RulesHash, &snapshot.CustomCatalogHash, &metadataAvailable, &diagnosticsRaw); err != nil {
			return nil, false, err
		}
		if !metadataAvailable {
			continue
		}
		available = true
		snapshot.Source = model.ProfileSource(source)
		snapshot.MetadataAvailable = true
		if len(diagnosticsRaw) > 0 {
			_ = json.Unmarshal(diagnosticsRaw, &snapshot.Diagnostics)
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if !seenRows {
		return snapshots, false, nil
	}
	return snapshots, available, nil
}

// HashChanges compares profile rule hashes between two scans.
func (r *ProfileSnapshotRepository) HashChanges(ctx context.Context, currentScanID, previousScanID int64) ([]model.ProfileHashChange, error) {
	current, currentAvailable, err := r.ListByScan(ctx, currentScanID)
	if err != nil {
		return nil, err
	}
	previous, previousAvailable, err := r.ListByScan(ctx, previousScanID)
	if err != nil {
		return nil, err
	}
	if !currentAvailable || !previousAvailable {
		return nil, nil
	}
	previousByLanguage := map[string]string{}
	for _, snapshot := range previous {
		previousByLanguage[snapshot.Language] = snapshot.RulesHash
	}
	changes := []model.ProfileHashChange{}
	for _, snapshot := range current {
		previousHash, ok := previousByLanguage[snapshot.Language]
		if ok && previousHash != snapshot.RulesHash {
			changes = append(changes, model.ProfileHashChange{Language: snapshot.Language, CurrentHash: snapshot.RulesHash, PreviousHash: previousHash})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Language < changes[j].Language })
	return changes, nil
}

func nullZeroInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
