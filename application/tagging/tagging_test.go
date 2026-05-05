package tagging

import (
	"context"
	"testing"

	"github.com/scovl/ollanta/domain/model"
)

func TestServiceCreateTagNormalizesAndDefaultsMetadata(t *testing.T) {
	t.Parallel()

	repo := &fakeTagRepo{tags: map[string]model.TagCatalogEntry{}}
	service := NewService(repo, repo, repo)
	tag, err := service.CreateTag(context.Background(), CreateTagRequest{Key: " Team API "}, 42)
	if err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}
	if tag.Key != "team-api" || tag.DisplayName != "Team Api" || tag.Scope != "global" || tag.CreatedBy != 42 {
		t.Fatalf("tag = %+v, want normalized defaults", tag)
	}
}

func TestServiceBulkEditRejectsDeprecatedTag(t *testing.T) {
	t.Parallel()

	repo := &fakeTagRepo{tags: map[string]model.TagCatalogEntry{
		"legacy": {Key: "legacy", Status: model.TagStatusDeprecated},
	}}
	service := NewService(repo, repo, repo)
	_, err := service.PreviewBulkEdit(context.Background(), model.TagBulkEditRequest{TargetType: model.TagTargetIssue, TargetIDs: []int64{1}, AddTags: []string{"legacy"}}, 7)
	if err == nil {
		t.Fatal("PreviewBulkEdit() error = nil, want deprecated tag error")
	}
}

func TestServiceSavedFilterResolvesTagAlias(t *testing.T) {
	t.Parallel()

	repo := &fakeTagRepo{tags: map[string]model.TagCatalogEntry{
		"team-api": {Key: "team-api", Status: model.TagStatusActive},
	}, aliases: map[string]string{"api-team": "team-api"}}
	service := NewService(repo, repo, repo)
	filter, err := service.CreateSavedFilter(context.Background(), SavedFilterRequest{
		Name:     "API debt",
		Criteria: map[string]any{"tag": "api-team"},
	}, 9)
	if err != nil {
		t.Fatalf("CreateSavedFilter() error = %v", err)
	}
	if filter.Criteria["tag"] != "team-api" {
		t.Fatalf("criteria tag = %#v, want canonical key", filter.Criteria["tag"])
	}
}

type fakeTagRepo struct {
	tags    map[string]model.TagCatalogEntry
	aliases map[string]string
	filters []model.SavedFilter
}

func (r *fakeTagRepo) CreateTag(_ context.Context, tag model.TagCatalogEntry) (*model.TagCatalogEntry, error) {
	r.tags[tag.Key] = tag
	return &tag, nil
}

func (r *fakeTagRepo) UpdateTag(_ context.Context, key string, update model.TagUpdate) (*model.TagCatalogEntry, error) {
	tag := r.tags[key]
	if update.DisplayName != nil {
		tag.DisplayName = *update.DisplayName
	}
	r.tags[key] = tag
	return &tag, nil
}

func (r *fakeTagRepo) DeprecateTag(_ context.Context, key, replacementKey string, _ int64) (*model.TagCatalogEntry, error) {
	tag := r.tags[key]
	tag.Status = model.TagStatusDeprecated
	tag.ReplacementKey = replacementKey
	r.tags[key] = tag
	return &tag, nil
}

func (r *fakeTagRepo) MergeTag(_ context.Context, sourceKey, targetKey string, _ int64) (*model.TagCatalogEntry, error) {
	if r.aliases == nil {
		r.aliases = map[string]string{}
	}
	r.aliases[sourceKey] = targetKey
	tag := r.tags[targetKey]
	return &tag, nil
}

func (r *fakeTagRepo) GetTag(_ context.Context, keyOrAlias string) (*model.TagCatalogEntry, error) {
	key := keyOrAlias
	if target, ok := r.aliases[keyOrAlias]; ok {
		key = target
	}
	tag := r.tags[key]
	return &tag, nil
}

func (r *fakeTagRepo) ListTags(context.Context, model.TagFilter) ([]model.TagCatalogEntry, int, error) {
	return nil, 0, nil
}

func (r *fakeTagRepo) DiscoverTags(context.Context, []string, model.TagSource) error { return nil }

func (r *fakeTagRepo) ResolveTagKey(_ context.Context, keyOrAlias string) (string, error) {
	key := model.NormalizeTagKey(keyOrAlias)
	if target, ok := r.aliases[key]; ok {
		return target, nil
	}
	return key, nil
}

func (r *fakeTagRepo) TagUsage(context.Context, string) (model.TagUsageSummary, error) {
	return model.TagUsageSummary{}, nil
}

func (r *fakeTagRepo) TagAudit(context.Context, string, int, int) ([]model.TagAuditEntry, int, error) {
	return nil, 0, nil
}

func (r *fakeTagRepo) PreviewTagBulkEdit(_ context.Context, req model.TagBulkEditRequest) (*model.TagBulkEditPreview, error) {
	return &model.TagBulkEditPreview{TargetType: req.TargetType, TargetCount: len(req.TargetIDs), AddTags: req.AddTags}, nil
}

func (r *fakeTagRepo) ApplyTagBulkEdit(_ context.Context, req model.TagBulkEditRequest) (*model.TagBulkEditResult, error) {
	return &model.TagBulkEditResult{TagBulkEditPreview: model.TagBulkEditPreview{TargetType: req.TargetType, TargetCount: len(req.TargetIDs)}}, nil
}

func (r *fakeTagRepo) CreateSavedFilter(_ context.Context, filter model.SavedFilter) (*model.SavedFilter, error) {
	r.filters = append(r.filters, filter)
	return &filter, nil
}

func (r *fakeTagRepo) UpdateSavedFilter(_ context.Context, filter model.SavedFilter) (*model.SavedFilter, error) {
	return &filter, nil
}

func (r *fakeTagRepo) GetSavedFilter(context.Context, int64) (*model.SavedFilter, error) {
	return nil, nil
}

func (r *fakeTagRepo) ListSavedFilters(context.Context, int64, model.SavedFilterVisibility, string, int, int) ([]model.SavedFilter, int, error) {
	return r.filters, len(r.filters), nil
}

func (r *fakeTagRepo) DeleteSavedFilter(context.Context, int64, int64) error { return nil }
