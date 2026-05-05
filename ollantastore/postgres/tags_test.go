package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/scovl/ollanta/domain/model"
)

func TestTagRepository_CatalogAliasAndAudit(t *testing.T) {
	db, ctx, prefix := openJobRepositoryTestDB(t)
	cleanupTagRepositoryTest(t, db, ctx, prefix)
	repo := NewTagRepository(db)

	key := strings.ReplaceAll(prefix, "_", "-") + "-team"
	created, err := repo.CreateTag(ctx, model.TagCatalogEntry{Key: key, DisplayName: "Team", Color: "#0ea5e9", CreatedBy: 7})
	if err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}
	if created.Key != key || created.Status != model.TagStatusActive {
		t.Fatalf("created = %+v, want active catalog tag", created)
	}
	if _, err := repo.CreateTag(ctx, model.TagCatalogEntry{Key: key}); err == nil {
		t.Fatal("CreateTag duplicate error = nil, want conflict")
	}

	alias := key + "-alias"
	updated, err := repo.UpdateTag(ctx, key, model.TagUpdate{Aliases: []string{alias}, ActorUserID: 7})
	if err != nil {
		t.Fatalf("UpdateTag() error = %v", err)
	}
	if len(updated.Aliases) != 1 || updated.Aliases[0].Alias != alias {
		t.Fatalf("aliases = %+v, want %s", updated.Aliases, alias)
	}
	resolved, err := repo.ResolveTagKey(ctx, alias)
	if err != nil {
		t.Fatalf("ResolveTagKey(alias) error = %v", err)
	}
	if resolved != key {
		t.Fatalf("resolved = %q, want %q", resolved, key)
	}
	audit, total, err := repo.TagAudit(ctx, key, 10, 0)
	if err != nil {
		t.Fatalf("TagAudit() error = %v", err)
	}
	if total < 2 || len(audit) == 0 {
		t.Fatalf("audit total/items = %d/%d, want create and update entries", total, len(audit))
	}
}

func TestTagRepository_BulkEditUsageAndSavedFilters(t *testing.T) {
	db, ctx, prefix := openJobRepositoryTestDB(t)
	cleanupTagRepositoryTest(t, db, ctx, prefix)
	repo := NewTagRepository(db)

	key := strings.ReplaceAll(prefix, "_", "-") + "-security"
	if _, err := repo.CreateTag(ctx, model.TagCatalogEntry{Key: key, DisplayName: "Security"}); err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}
	projectID, scanID := createJobTestProjectAndScan(t, db, ctx, prefix+"-project")
	var issueID int64
	if err := db.Pool.QueryRow(ctx, `
		INSERT INTO issues (scan_id, project_id, rule_key, component_path, type, severity, status, tags)
		VALUES ($1, $2, 'go:test', 'main.go', 'code_smell', 'major', 'open', ARRAY['legacy'])
		RETURNING id`, scanID, projectID).Scan(&issueID); err != nil {
		t.Fatalf("insert issue: %v", err)
	}

	preview, err := repo.PreviewTagBulkEdit(ctx, model.TagBulkEditRequest{TargetType: model.TagTargetIssue, TargetIDs: []int64{issueID}, AddTags: []string{key}})
	if err != nil {
		t.Fatalf("PreviewTagBulkEdit() error = %v", err)
	}
	if preview.TargetCount != 1 || len(preview.Changes) != 1 {
		t.Fatalf("preview = %+v, want one changed issue", preview)
	}
	result, err := repo.ApplyTagBulkEdit(ctx, model.TagBulkEditRequest{TargetType: model.TagTargetIssue, TargetIDs: []int64{issueID}, AddTags: []string{key}, ActorUserID: 3})
	if err != nil {
		t.Fatalf("ApplyTagBulkEdit() error = %v", err)
	}
	if result.UpdatedCount != 1 || result.FailedCount != 0 {
		t.Fatalf("result = %+v, want one updated target", result)
	}
	issue, err := NewIssueRepository(db).GetByID(ctx, issueID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !containsString(issue.Tags, key) {
		t.Fatalf("issue tags = %#v, want %s", issue.Tags, key)
	}

	filter, err := repo.CreateSavedFilter(ctx, model.SavedFilter{Name: prefix + " filter", OwnerUserID: 3, Visibility: model.SavedFilterShared, FilterType: "issues", Criteria: map[string]any{"tag": key}})
	if err != nil {
		t.Fatalf("CreateSavedFilter() error = %v", err)
	}
	if filter.ID == 0 || filter.Criteria["tag"] != key {
		t.Fatalf("saved filter = %+v, want persisted criteria", filter)
	}
	usage, err := repo.TagUsage(ctx, key)
	if err != nil {
		t.Fatalf("TagUsage() error = %v", err)
	}
	if usage.IssueCount != 1 || usage.SavedFilterCount != 1 {
		t.Fatalf("usage = %+v, want issue and saved filter counts", usage)
	}
}

func cleanupTagRepositoryTest(t *testing.T, db *DB, ctx context.Context, prefix string) {
	t.Helper()
	keyPattern := strings.ReplaceAll(prefix, "_", "-") + "%"
	t.Cleanup(func() {
		cleanupTagRepositoryRows(t, db, context.Background(), prefix, keyPattern)
	})
	cleanupTagRepositoryRows(t, db, ctx, prefix, keyPattern)
}

func cleanupTagRepositoryRows(t *testing.T, db *DB, ctx context.Context, prefix, keyPattern string) {
	t.Helper()
	statements := []struct {
		sql string
		arg string
	}{
		{"DELETE FROM saved_filters WHERE name LIKE $1", prefix + "%"},
		{"DELETE FROM tag_audit WHERE tag_key LIKE $1", keyPattern},
		{"DELETE FROM tag_assignments WHERE tag_key LIKE $1", keyPattern},
		{"DELETE FROM tag_aliases WHERE tag_key LIKE $1 OR alias LIKE $1", keyPattern},
		{"DELETE FROM tag_catalog WHERE key LIKE $1", keyPattern},
	}
	for _, statement := range statements {
		if _, err := db.Pool.Exec(ctx, statement.sql, statement.arg); err != nil {
			t.Logf("cleanup %q: %v", statement.sql, err)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
