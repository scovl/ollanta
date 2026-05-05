package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
)

type TagRepository struct {
	db *DB
}

var _ port.ITagCatalogRepo = (*TagRepository)(nil)
var _ port.ITagAssignmentRepo = (*TagRepository)(nil)
var _ port.ISavedFilterRepo = (*TagRepository)(nil)

type tagExec interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type tagTarget struct {
	ID   int64
	Key  string
	Tags []string
}

func NewTagRepository(db *DB) *TagRepository {
	return &TagRepository{db: db}
}

func (r *TagRepository) CreateTag(ctx context.Context, tag model.TagCatalogEntry) (*model.TagCatalogEntry, error) {
	tag.Key = model.NormalizeTagKey(tag.Key)
	if err := model.ValidateTagKey(tag.Key); err != nil {
		return nil, err
	}
	if tag.DisplayName == "" {
		tag.DisplayName = model.DefaultTagDisplayName(tag.Key)
	}
	if tag.Scope == "" {
		tag.Scope = "global"
	}
	if tag.Status == "" {
		tag.Status = model.TagStatusActive
	}
	if tag.Source == "" {
		tag.Source = model.TagSourceManual
	}

	created, err := scanTag(r.db.Pool.QueryRow(ctx, `
		INSERT INTO tag_catalog (key, display_name, description, color, owner_type, owner_id, owner_name, scope, status, source, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING `+tagColumns(),
		tag.Key, tag.DisplayName, tag.Description, tag.Color, tag.OwnerType, tag.OwnerID, tag.OwnerName, tag.Scope, tag.Status, tag.Source, tag.CreatedBy,
	))
	if err != nil {
		return nil, err
	}
	if err := r.recordTagAudit(ctx, r.db.Pool, model.TagAuditEntry{TagKey: created.Key, Action: "create", ActorUserID: tag.CreatedBy, NewState: tagAuditState(*created)}); err != nil {
		return nil, err
	}
	return r.withTagExtras(ctx, created)
}

func (r *TagRepository) UpdateTag(ctx context.Context, key string, update model.TagUpdate) (*model.TagCatalogEntry, error) {
	key = model.NormalizeTagKey(key)
	existing, err := r.GetTag(ctx, key)
	if err != nil {
		return nil, err
	}

	next := applyTagUpdate(*existing, update)

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	updated, err := scanTag(tx.QueryRow(ctx, `
		UPDATE tag_catalog
		SET display_name = $2, description = $3, color = $4, owner_type = $5, owner_id = $6,
		    owner_name = $7, scope = $8, status = $9, replacement_key = $10, updated_at = now()
		WHERE key = $1
		RETURNING `+tagColumns(),
		next.Key, next.DisplayName, next.Description, next.Color, next.OwnerType, next.OwnerID, next.OwnerName, next.Scope, next.Status, next.ReplacementKey,
	))
	if err != nil {
		return nil, err
	}

	if update.Aliases != nil {
		if err := r.replaceTagAliases(ctx, tx, updated.Key, update.Aliases); err != nil {
			return nil, err
		}
	}

	if err := r.recordTagAudit(ctx, tx, model.TagAuditEntry{TagKey: updated.Key, Action: "update", ActorUserID: update.ActorUserID, OldState: tagAuditState(*existing), NewState: tagAuditState(*updated)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.withTagExtras(ctx, updated)
}

func applyTagUpdate(existing model.TagCatalogEntry, update model.TagUpdate) model.TagCatalogEntry {
	next := existing
	if update.DisplayName != nil {
		next.DisplayName = *update.DisplayName
	}
	if update.Description != nil {
		next.Description = *update.Description
	}
	if update.Color != nil {
		next.Color = *update.Color
	}
	if update.OwnerType != nil {
		next.OwnerType = *update.OwnerType
	}
	if update.OwnerID != nil {
		next.OwnerID = *update.OwnerID
	}
	if update.OwnerName != nil {
		next.OwnerName = *update.OwnerName
	}
	if update.Scope != nil {
		next.Scope = *update.Scope
	}
	if update.Status != nil {
		next.Status = *update.Status
	}
	if update.ReplacementKey != nil {
		next.ReplacementKey = *update.ReplacementKey
	}
	return next
}

func (r *TagRepository) replaceTagAliases(ctx context.Context, tx pgx.Tx, tagKey string, aliases []string) error {
	if _, err := tx.Exec(ctx, "DELETE FROM tag_aliases WHERE tag_key = $1", tagKey); err != nil {
		return err
	}
	for _, alias := range aliases {
		alias = model.NormalizeTagKey(alias)
		if alias == "" || alias == tagKey {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO tag_aliases (tag_key, alias)
			VALUES ($1, $2)
			ON CONFLICT (alias) DO UPDATE SET tag_key = EXCLUDED.tag_key`, tagKey, alias); err != nil {
			return err
		}
	}
	return nil
}

func (r *TagRepository) DeprecateTag(ctx context.Context, key, replacementKey string, actorUserID int64) (*model.TagCatalogEntry, error) {
	status := model.TagStatusDeprecated
	return r.UpdateTag(ctx, key, model.TagUpdate{Status: &status, ReplacementKey: &replacementKey, ActorUserID: actorUserID})
}

func (r *TagRepository) MergeTag(ctx context.Context, sourceKey, targetKey string, actorUserID int64) (*model.TagCatalogEntry, error) {
	sourceKey = model.NormalizeTagKey(sourceKey)
	targetKey = model.NormalizeTagKey(targetKey)
	target, err := r.GetTag(ctx, targetKey)
	if err != nil {
		return nil, err
	}
	existing, err := r.GetTag(ctx, sourceKey)
	if err != nil {
		return nil, err
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `
		INSERT INTO tag_aliases (tag_key, alias)
		VALUES ($1, $2)
		ON CONFLICT (alias) DO UPDATE SET tag_key = EXCLUDED.tag_key`, targetKey, sourceKey); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tag_assignments (target_type, target_id, target_key, tag_key, source, actor_user_id)
		SELECT target_type, target_id, target_key, $2, source, $3
		FROM tag_assignments WHERE tag_key = $1
		ON CONFLICT (target_type, target_id, target_key, tag_key) DO NOTHING`, sourceKey, targetKey, actorUserID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM tag_assignments WHERE tag_key = $1", sourceKey); err != nil {
		return nil, err
	}
	for _, table := range []string{"issues", "projects", "custom_rule_versions"} {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s
			SET tags = ARRAY(SELECT DISTINCT CASE WHEN value = $1 THEN $2 ELSE value END FROM unnest(tags) value ORDER BY 1)
			WHERE $1 = ANY(tags)`, table), sourceKey, targetKey); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tag_catalog
		SET status = 'deprecated', replacement_key = $2, updated_at = now()
		WHERE key = $1`, sourceKey, targetKey); err != nil {
		return nil, err
	}
	if err := r.recordTagAudit(ctx, tx, model.TagAuditEntry{TagKey: sourceKey, Action: "merge", ActorUserID: actorUserID, OldState: tagAuditState(*existing), NewState: map[string]any{"replacement_key": targetKey, "status": string(model.TagStatusDeprecated)}}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.withTagExtras(ctx, target)
}

func (r *TagRepository) GetTag(ctx context.Context, keyOrAlias string) (*model.TagCatalogEntry, error) {
	key, err := r.ResolveTagKey(ctx, keyOrAlias)
	if err != nil {
		return nil, err
	}
	tag, err := scanTag(r.db.Pool.QueryRow(ctx, "SELECT "+tagColumns()+" FROM tag_catalog WHERE key = $1", key))
	if err != nil {
		return nil, err
	}
	return r.withTagExtras(ctx, tag)
}

func (r *TagRepository) ListTags(ctx context.Context, filter model.TagFilter) ([]model.TagCatalogEntry, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 500 {
		filter.Limit = 500
	}
	conds := []string{}
	args := []any{}
	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		conds = append(conds, fmt.Sprintf("(lower(key) LIKE $%d OR lower(display_name) LIKE $%d OR lower(description) LIKE $%d)", len(args), len(args), len(args)))
	}
	if filter.Status != "" {
		args = append(args, string(filter.Status))
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Owner != "" {
		args = append(args, "%"+strings.ToLower(filter.Owner)+"%")
		conds = append(conds, fmt.Sprintf("lower(owner_name) LIKE $%d", len(args)))
	}
	if filter.Scope != "" {
		args = append(args, filter.Scope)
		conds = append(conds, fmt.Sprintf("scope = $%d", len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM tag_catalog"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Pool.Query(ctx, fmt.Sprintf("SELECT %s FROM tag_catalog%s ORDER BY key LIMIT $%d OFFSET $%d", tagColumns(), where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var tags []model.TagCatalogEntry
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, 0, err
		}
		withExtras, err := r.withTagExtras(ctx, tag)
		if err != nil {
			return nil, 0, err
		}
		tags = append(tags, *withExtras)
	}
	return tags, total, rows.Err()
}

func (r *TagRepository) DiscoverTags(ctx context.Context, keys []string, source model.TagSource) error {
	if source == "" {
		source = model.TagSourceScan
	}
	for _, key := range model.NormalizeTagKeys(keys) {
		if err := model.ValidateTagKey(key); err != nil {
			continue
		}
		_, err := r.db.Pool.Exec(ctx, `
			INSERT INTO tag_catalog (key, display_name, status, source)
			VALUES ($1, $2, 'discovered', $3)
			ON CONFLICT (key) DO NOTHING`, key, model.DefaultTagDisplayName(key), source)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *TagRepository) ResolveTagKey(ctx context.Context, keyOrAlias string) (string, error) {
	key := model.NormalizeTagKey(keyOrAlias)
	if key == "" {
		return "", ErrNotFound
	}
	var resolved string
	err := r.db.Pool.QueryRow(ctx, `
		SELECT key FROM tag_catalog WHERE key = $1
		UNION ALL
		SELECT tag_key FROM tag_aliases WHERE alias = $1
		LIMIT 1`, key).Scan(&resolved)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return resolved, err
}

func (r *TagRepository) TagUsage(ctx context.Context, key string) (model.TagUsageSummary, error) {
	key = model.NormalizeTagKey(key)
	usage := model.TagUsageSummary{}
	queries := []struct {
		target *int
		sql    string
	}{
		{&usage.IssueCount, `SELECT COUNT(DISTINCT id) FROM (SELECT id FROM issues WHERE $1 = ANY(tags) UNION SELECT target_id AS id FROM tag_assignments WHERE tag_key = $1 AND target_type = 'issue') s`},
		{&usage.ProjectCount, `SELECT COUNT(DISTINCT id) FROM (SELECT id FROM projects WHERE $1 = ANY(tags) UNION SELECT target_id AS id FROM tag_assignments WHERE tag_key = $1 AND target_type = 'project') s`},
		{&usage.CustomRuleCount, `SELECT COUNT(DISTINCT id) FROM (SELECT id FROM custom_rule_versions WHERE $1 = ANY(tags) UNION SELECT target_id AS id FROM tag_assignments WHERE tag_key = $1 AND target_type = 'custom_rule') s`},
		{&usage.RuleCount, `SELECT COUNT(DISTINCT target_key) FROM tag_assignments WHERE tag_key = $1 AND target_type = 'rule'`},
		{&usage.SavedFilterCount, `SELECT COUNT(*) FROM saved_filters WHERE criteria->>'tag' = $1 OR criteria->'tags' ? $1`},
	}
	for _, query := range queries {
		if err := r.db.Pool.QueryRow(ctx, query.sql, key).Scan(query.target); err != nil {
			return usage, err
		}
	}
	return usage, nil
}

func (r *TagRepository) TagAudit(ctx context.Context, key string, limit, offset int) ([]model.TagAuditEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}
	var total int
	if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM tag_audit WHERE tag_key = $1", model.NormalizeTagKey(key)).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, tag_key, action, target_type, target_id, target_key, actor_user_id, old_state, new_state, summary, created_at
		FROM tag_audit WHERE tag_key = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, model.NormalizeTagKey(key), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	entries, err := scanTagAuditRows(rows)
	return entries, total, err
}

func (r *TagRepository) PreviewTagBulkEdit(ctx context.Context, req model.TagBulkEditRequest) (*model.TagBulkEditPreview, error) {
	preview := &model.TagBulkEditPreview{
		TargetType: req.TargetType,
		AddTags:    append([]string(nil), req.AddTags...),
		RemoveTags: append([]string(nil), req.RemoveTags...),
	}
	if req.TargetType == "" {
		preview.ValidationErrors = append(preview.ValidationErrors, "target_type is required")
		return preview, nil
	}
	if len(req.AddTags) == 0 && len(req.RemoveTags) == 0 {
		preview.ValidationErrors = append(preview.ValidationErrors, "add_tags or remove_tags is required")
	}
	targets, err := r.loadTargets(ctx, req)
	if err != nil {
		return nil, err
	}
	preview.TargetCount = len(targets)
	if len(targets) == 0 {
		preview.ValidationErrors = append(preview.ValidationErrors, "no targets matched the request")
	}
	for _, target := range targets {
		change := buildTagChange(target, req.AddTags, req.RemoveTags)
		if !sameStrings(change.Before, change.After) {
			preview.Changes = append(preview.Changes, change)
		}
	}
	return preview, nil
}

func (r *TagRepository) ApplyTagBulkEdit(ctx context.Context, req model.TagBulkEditRequest) (*model.TagBulkEditResult, error) {
	preview, err := r.PreviewTagBulkEdit(ctx, req)
	if err != nil {
		return nil, err
	}
	result := &model.TagBulkEditResult{TagBulkEditPreview: *preview}
	if len(preview.ValidationErrors) > 0 {
		result.FailedCount = preview.TargetCount
		return result, nil
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, change := range preview.Changes {
		if err := r.applyTagChange(ctx, tx, req, change); err != nil {
			result.FailedCount++
			continue
		}
		if err := r.recordBulkEditAudits(ctx, tx, req, change); err != nil {
			return nil, err
		}
		result.UpdatedCount++
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *TagRepository) applyTagChange(ctx context.Context, tx pgx.Tx, req model.TagBulkEditRequest, change model.TagBulkEditChange) error {
	if err := r.updateTargetTags(ctx, tx, req.TargetType, change.TargetID, change.TargetKey, change.After); err != nil {
		return err
	}
	for _, tag := range req.AddTags {
		if _, err := tx.Exec(ctx, `
			INSERT INTO tag_assignments (target_type, target_id, target_key, tag_key, source, actor_user_id)
			VALUES ($1, $2, $3, $4, 'manual', $5)
			ON CONFLICT (target_type, target_id, target_key, tag_key) DO NOTHING`, req.TargetType, change.TargetID, change.TargetKey, tag, req.ActorUserID); err != nil {
			return err
		}
	}
	for _, tag := range req.RemoveTags {
		if _, err := tx.Exec(ctx, `
			DELETE FROM tag_assignments
			WHERE target_type = $1 AND target_id = $2 AND target_key = $3 AND tag_key = $4`, req.TargetType, change.TargetID, change.TargetKey, tag); err != nil {
			return err
		}
	}
	return nil
}

func (r *TagRepository) recordBulkEditAudits(ctx context.Context, tx pgx.Tx, req model.TagBulkEditRequest, change model.TagBulkEditChange) error {
	changedTags := append(append([]string{}, req.AddTags...), req.RemoveTags...)
	for _, tag := range changedTags {
		if err := r.recordTagAudit(ctx, tx, model.TagAuditEntry{
			TagKey: tag, Action: "bulk_edit", TargetType: req.TargetType, TargetID: change.TargetID, TargetKey: change.TargetKey,
			ActorUserID: req.ActorUserID, OldState: map[string]any{"tags": change.Before}, NewState: map[string]any{"tags": change.After}, Summary: map[string]any{"reason": req.Reason},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *TagRepository) CreateSavedFilter(ctx context.Context, filter model.SavedFilter) (*model.SavedFilter, error) {
	if filter.Visibility == "" {
		filter.Visibility = model.SavedFilterPrivate
	}
	if filter.FilterType == "" {
		filter.FilterType = "issues"
	}
	criteria, err := json.Marshal(filter.Criteria)
	if err != nil {
		return nil, err
	}
	created, err := scanSavedFilter(r.db.Pool.QueryRow(ctx, `
		INSERT INTO saved_filters (name, description, owner_user_id, visibility, filter_type, criteria)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, description, owner_user_id, visibility, filter_type, criteria, created_at, updated_at`,
		filter.Name, filter.Description, filter.OwnerUserID, filter.Visibility, filter.FilterType, criteria,
	))
	if err != nil {
		return nil, err
	}
	if err := r.recordTagAudit(ctx, r.db.Pool, model.TagAuditEntry{Action: "saved_filter_create", ActorUserID: filter.OwnerUserID, NewState: map[string]any{"name": filter.Name}}); err != nil {
		return nil, err
	}
	return created, nil
}

func (r *TagRepository) UpdateSavedFilter(ctx context.Context, filter model.SavedFilter) (*model.SavedFilter, error) {
	criteria, err := json.Marshal(filter.Criteria)
	if err != nil {
		return nil, err
	}
	updated, err := scanSavedFilter(r.db.Pool.QueryRow(ctx, `
		UPDATE saved_filters
		SET name = $2, description = $3, visibility = $4, filter_type = $5, criteria = $6, updated_at = now()
		WHERE id = $1 AND (owner_user_id = $7 OR $7 = 0)
		RETURNING id, name, description, owner_user_id, visibility, filter_type, criteria, created_at, updated_at`,
		filter.ID, filter.Name, filter.Description, filter.Visibility, filter.FilterType, criteria, filter.OwnerUserID,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return updated, err
}

func (r *TagRepository) GetSavedFilter(ctx context.Context, id int64) (*model.SavedFilter, error) {
	filter, err := scanSavedFilter(r.db.Pool.QueryRow(ctx, `
		SELECT id, name, description, owner_user_id, visibility, filter_type, criteria, created_at, updated_at
		FROM saved_filters WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return filter, err
}

func (r *TagRepository) ListSavedFilters(ctx context.Context, ownerUserID int64, visibility model.SavedFilterVisibility, filterType string, limit, offset int) ([]model.SavedFilter, int, error) {
	if limit <= 0 {
		limit = 50
	}
	conds := []string{}
	args := []any{}
	if ownerUserID > 0 {
		args = append(args, ownerUserID)
		conds = append(conds, fmt.Sprintf("(owner_user_id = $%d OR visibility = 'shared')", len(args)))
	}
	if visibility != "" {
		args = append(args, string(visibility))
		conds = append(conds, fmt.Sprintf("visibility = $%d", len(args)))
	}
	if filterType != "" {
		args = append(args, filterType)
		conds = append(conds, fmt.Sprintf("filter_type = $%d", len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM saved_filters"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, description, owner_user_id, visibility, filter_type, criteria, created_at, updated_at
		FROM saved_filters%s ORDER BY updated_at DESC, id DESC LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var filters []model.SavedFilter
	for rows.Next() {
		filter, err := scanSavedFilter(rows)
		if err != nil {
			return nil, 0, err
		}
		filters = append(filters, *filter)
	}
	return filters, total, rows.Err()
}

func (r *TagRepository) DeleteSavedFilter(ctx context.Context, id, actorUserID int64) error {
	tag, err := r.db.Pool.Exec(ctx, "DELETE FROM saved_filters WHERE id = $1 AND (owner_user_id = $2 OR $2 = 0)", id, actorUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func tagColumns() string {
	return "id, key, display_name, description, color, owner_type, owner_id, owner_name, scope, status, source, replacement_key, created_by, created_at, updated_at"
}

func scanTag(row pgx.Row) (*model.TagCatalogEntry, error) {
	tag := &model.TagCatalogEntry{}
	err := row.Scan(&tag.ID, &tag.Key, &tag.DisplayName, &tag.Description, &tag.Color, &tag.OwnerType, &tag.OwnerID, &tag.OwnerName, &tag.Scope, &tag.Status, &tag.Source, &tag.ReplacementKey, &tag.CreatedBy, &tag.CreatedAt, &tag.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return tag, err
}

func (r *TagRepository) withTagExtras(ctx context.Context, tag *model.TagCatalogEntry) (*model.TagCatalogEntry, error) {
	aliases, err := r.loadAliases(ctx, tag.Key)
	if err != nil {
		return nil, err
	}
	usage, err := r.TagUsage(ctx, tag.Key)
	if err != nil {
		return nil, err
	}
	tag.Aliases = aliases
	tag.Usage = usage
	return tag, nil
}

func (r *TagRepository) loadAliases(ctx context.Context, key string) ([]model.TagAlias, error) {
	rows, err := r.db.Pool.Query(ctx, "SELECT id, tag_key, alias, created_at FROM tag_aliases WHERE tag_key = $1 ORDER BY alias", key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var aliases []model.TagAlias
	for rows.Next() {
		alias := model.TagAlias{}
		if err := rows.Scan(&alias.ID, &alias.TagKey, &alias.Alias, &alias.CreatedAt); err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

func tagAuditState(tag model.TagCatalogEntry) map[string]any {
	return map[string]any{
		"key": tag.Key, "display_name": tag.DisplayName, "description": tag.Description, "color": tag.Color,
		"owner_type": string(tag.OwnerType), "owner_id": tag.OwnerID, "owner_name": tag.OwnerName,
		"scope": tag.Scope, "status": string(tag.Status), "replacement_key": tag.ReplacementKey,
	}
}

func (r *TagRepository) recordTagAudit(ctx context.Context, exec tagExec, entry model.TagAuditEntry) error {
	oldState, err := json.Marshal(emptyMapIfNil(entry.OldState))
	if err != nil {
		return err
	}
	newState, err := json.Marshal(emptyMapIfNil(entry.NewState))
	if err != nil {
		return err
	}
	summary, err := json.Marshal(emptyMapIfNil(entry.Summary))
	if err != nil {
		return err
	}
	_, err = exec.Exec(ctx, `
		INSERT INTO tag_audit (tag_key, action, target_type, target_id, target_key, actor_user_id, old_state, new_state, summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, entry.TagKey, entry.Action, entry.TargetType, entry.TargetID, entry.TargetKey, entry.ActorUserID, oldState, newState, summary)
	return err
}

func emptyMapIfNil(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func scanTagAuditRows(rows pgx.Rows) ([]model.TagAuditEntry, error) {
	var entries []model.TagAuditEntry
	for rows.Next() {
		entry := model.TagAuditEntry{}
		var oldState, newState, summary []byte
		if err := rows.Scan(&entry.ID, &entry.TagKey, &entry.Action, &entry.TargetType, &entry.TargetID, &entry.TargetKey, &entry.ActorUserID, &oldState, &newState, &summary, &entry.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(oldState, &entry.OldState); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(newState, &entry.NewState); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(summary, &entry.Summary); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *TagRepository) loadTargets(ctx context.Context, req model.TagBulkEditRequest) ([]tagTarget, error) {
	switch req.TargetType {
	case model.TagTargetIssue:
		if len(req.TargetIDs) == 0 {
			return nil, nil
		}
		return r.queryTargets(ctx, "SELECT id, '' AS key, tags FROM issues WHERE id = ANY($1) ORDER BY id", req.TargetIDs)
	case model.TagTargetProject:
		return r.loadProjectTargets(ctx, req)
	case model.TagTargetCustomRule:
		return r.loadCustomRuleTargets(ctx, req)
	case model.TagTargetRule:
		return r.loadRuleTargets(ctx, req.TargetKeys)
	default:
		return nil, nil
	}
}

func (r *TagRepository) queryTargets(ctx context.Context, query string, arg any) ([]tagTarget, error) {
	rows, err := r.db.Pool.Query(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var targets []tagTarget
	for rows.Next() {
		target := tagTarget{}
		if err := rows.Scan(&target.ID, &target.Key, &target.Tags); err != nil {
			return nil, err
		}
		target.Tags = model.NormalizeTagKeys(target.Tags)
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (r *TagRepository) loadProjectTargets(ctx context.Context, req model.TagBulkEditRequest) ([]tagTarget, error) {
	if len(req.TargetIDs) > 0 {
		return r.queryTargets(ctx, "SELECT id, key, tags FROM projects WHERE id = ANY($1) ORDER BY id", req.TargetIDs)
	}
	if len(req.TargetKeys) > 0 {
		return r.queryTargets(ctx, "SELECT id, key, tags FROM projects WHERE key = ANY($1) ORDER BY key", req.TargetKeys)
	}
	return nil, nil
}

func (r *TagRepository) loadCustomRuleTargets(ctx context.Context, req model.TagBulkEditRequest) ([]tagTarget, error) {
	if len(req.TargetIDs) > 0 {
		return r.queryTargets(ctx, "SELECT id, rule_key, tags FROM custom_rule_versions WHERE id = ANY($1) ORDER BY id", req.TargetIDs)
	}
	if len(req.TargetKeys) > 0 {
		return r.queryTargets(ctx, "SELECT id, rule_key, tags FROM custom_rule_versions WHERE rule_key = ANY($1) ORDER BY rule_key, version DESC", req.TargetKeys)
	}
	return nil, nil
}

func (r *TagRepository) loadRuleTargets(ctx context.Context, keys []string) ([]tagTarget, error) {
	keys = model.NormalizeTagKeys(keys)
	if len(keys) == 0 {
		return nil, nil
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT target_key, array_agg(tag_key ORDER BY tag_key)
		FROM tag_assignments
		WHERE target_type = 'rule' AND target_key = ANY($1)
		GROUP BY target_key`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byKey := map[string][]string{}
	for rows.Next() {
		var key string
		var tags []string
		if err := rows.Scan(&key, &tags); err != nil {
			return nil, err
		}
		byKey[key] = tags
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	targets := make([]tagTarget, 0, len(keys))
	for _, key := range keys {
		targets = append(targets, tagTarget{Key: key, Tags: model.NormalizeTagKeys(byKey[key])})
	}
	return targets, nil
}

func buildTagChange(target tagTarget, addTags, removeTags []string) model.TagBulkEditChange {
	before := model.NormalizeTagKeys(target.Tags)
	set := map[string]bool{}
	for _, tag := range before {
		set[tag] = true
	}
	for _, tag := range addTags {
		set[tag] = true
	}
	for _, tag := range removeTags {
		delete(set, tag)
	}
	after := make([]string, 0, len(set))
	for tag := range set {
		after = append(after, tag)
	}
	sort.Strings(after)
	return model.TagBulkEditChange{
		TargetID:    target.ID,
		TargetKey:   target.Key,
		Before:      before,
		After:       after,
		AddedTags:   diffStrings(after, before),
		RemovedTags: diffStrings(before, after),
	}
}

func (r *TagRepository) updateTargetTags(ctx context.Context, tx pgx.Tx, targetType model.TagTargetType, id int64, key string, tags []string) error {
	switch targetType {
	case model.TagTargetIssue:
		_, err := tx.Exec(ctx, "UPDATE issues SET tags = $2 WHERE id = $1", id, tags)
		return err
	case model.TagTargetProject:
		_, err := tx.Exec(ctx, "UPDATE projects SET tags = $2, updated_at = now() WHERE id = $1", id, tags)
		return err
	case model.TagTargetCustomRule:
		_, err := tx.Exec(ctx, "UPDATE custom_rule_versions SET tags = $2, updated_at = now() WHERE id = $1", id, tags)
		return err
	case model.TagTargetRule:
		return nil
	default:
		return fmt.Errorf("unsupported target_type %q", targetType)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func diffStrings(left, right []string) []string {
	rightSet := map[string]bool{}
	for _, value := range right {
		rightSet[value] = true
	}
	var out []string
	for _, value := range left {
		if !rightSet[value] {
			out = append(out, value)
		}
	}
	return out
}

func scanSavedFilter(row pgx.Row) (*model.SavedFilter, error) {
	filter := &model.SavedFilter{}
	var criteria []byte
	err := row.Scan(&filter.ID, &filter.Name, &filter.Description, &filter.OwnerUserID, &filter.Visibility, &filter.FilterType, &criteria, &filter.CreatedAt, &filter.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(criteria) > 0 {
		if err := json.Unmarshal(criteria, &filter.Criteria); err != nil {
			return nil, err
		}
	}
	if filter.Criteria == nil {
		filter.Criteria = map[string]any{}
	}
	return filter, nil
}
