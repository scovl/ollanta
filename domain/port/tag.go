package port

import (
	"context"

	"github.com/scovl/ollanta/domain/model"
)

type ITagCatalogRepo interface {
	CreateTag(ctx context.Context, tag model.TagCatalogEntry) (*model.TagCatalogEntry, error)
	UpdateTag(ctx context.Context, key string, update model.TagUpdate) (*model.TagCatalogEntry, error)
	DeprecateTag(ctx context.Context, key, replacementKey string, actorUserID int64) (*model.TagCatalogEntry, error)
	MergeTag(ctx context.Context, sourceKey, targetKey string, actorUserID int64) (*model.TagCatalogEntry, error)
	GetTag(ctx context.Context, keyOrAlias string) (*model.TagCatalogEntry, error)
	ListTags(ctx context.Context, filter model.TagFilter) ([]model.TagCatalogEntry, int, error)
	DiscoverTags(ctx context.Context, keys []string, source model.TagSource) error
	ResolveTagKey(ctx context.Context, keyOrAlias string) (string, error)
	TagUsage(ctx context.Context, key string) (model.TagUsageSummary, error)
	TagAudit(ctx context.Context, key string, limit, offset int) ([]model.TagAuditEntry, int, error)
}

type ITagAssignmentRepo interface {
	PreviewTagBulkEdit(ctx context.Context, req model.TagBulkEditRequest) (*model.TagBulkEditPreview, error)
	ApplyTagBulkEdit(ctx context.Context, req model.TagBulkEditRequest) (*model.TagBulkEditResult, error)
}

type ISavedFilterRepo interface {
	CreateSavedFilter(ctx context.Context, filter model.SavedFilter) (*model.SavedFilter, error)
	UpdateSavedFilter(ctx context.Context, filter model.SavedFilter) (*model.SavedFilter, error)
	GetSavedFilter(ctx context.Context, id int64) (*model.SavedFilter, error)
	ListSavedFilters(ctx context.Context, ownerUserID int64, visibility model.SavedFilterVisibility, filterType string, limit, offset int) ([]model.SavedFilter, int, error)
	DeleteSavedFilter(ctx context.Context, id, actorUserID int64) error
}
