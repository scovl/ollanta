package tagging

import (
	"context"
	"errors"
	"fmt"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
)

type Service struct {
	catalog     port.ITagCatalogRepo
	assignments port.ITagAssignmentRepo
	filters     port.ISavedFilterRepo
}

type CreateTagRequest struct {
	Key         string             `json:"key"`
	DisplayName string             `json:"display_name"`
	Description string             `json:"description"`
	Color       string             `json:"color"`
	OwnerType   model.TagOwnerType `json:"owner_type,omitempty"`
	OwnerID     int64              `json:"owner_id,omitempty"`
	OwnerName   string             `json:"owner_name,omitempty"`
	Scope       string             `json:"scope,omitempty"`
}

type UpdateTagRequest struct {
	DisplayName    *string             `json:"display_name,omitempty"`
	Description    *string             `json:"description,omitempty"`
	Color          *string             `json:"color,omitempty"`
	OwnerType      *model.TagOwnerType `json:"owner_type,omitempty"`
	OwnerID        *int64              `json:"owner_id,omitempty"`
	OwnerName      *string             `json:"owner_name,omitempty"`
	Scope          *string             `json:"scope,omitempty"`
	Status         *model.TagStatus    `json:"status,omitempty"`
	ReplacementKey *string             `json:"replacement_key,omitempty"`
	Aliases        []string            `json:"aliases,omitempty"`
}

type SavedFilterRequest struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description,omitempty"`
	Visibility  model.SavedFilterVisibility `json:"visibility,omitempty"`
	FilterType  string                      `json:"filter_type,omitempty"`
	Criteria    map[string]any              `json:"criteria"`
}

func NewService(catalog port.ITagCatalogRepo, assignments port.ITagAssignmentRepo, filters port.ISavedFilterRepo) *Service {
	return &Service{catalog: catalog, assignments: assignments, filters: filters}
}

func (s *Service) CreateTag(ctx context.Context, req CreateTagRequest, actorUserID int64) (*model.TagCatalogEntry, error) {
	if s.catalog == nil {
		return nil, errors.New("tag catalog repository is not configured")
	}
	key := model.NormalizeTagKey(req.Key)
	if err := model.ValidateTagKey(key); err != nil {
		return nil, err
	}
	if err := model.ValidateTagColor(req.Color); err != nil {
		return nil, err
	}
	displayName := req.DisplayName
	if displayName == "" {
		displayName = model.DefaultTagDisplayName(key)
	}
	scope := req.Scope
	if scope == "" {
		scope = "global"
	}
	return s.catalog.CreateTag(ctx, model.TagCatalogEntry{
		Key:         key,
		DisplayName: displayName,
		Description: req.Description,
		Color:       req.Color,
		OwnerType:   req.OwnerType,
		OwnerID:     req.OwnerID,
		OwnerName:   req.OwnerName,
		Scope:       scope,
		Status:      model.TagStatusActive,
		Source:      model.TagSourceManual,
		CreatedBy:   actorUserID,
	})
}

func (s *Service) UpdateTag(ctx context.Context, key string, req UpdateTagRequest, actorUserID int64) (*model.TagCatalogEntry, error) {
	if s.catalog == nil {
		return nil, errors.New("tag catalog repository is not configured")
	}
	normalized := model.NormalizeTagKey(key)
	if err := model.ValidateTagKey(normalized); err != nil {
		return nil, err
	}
	if req.Color != nil {
		if err := model.ValidateTagColor(*req.Color); err != nil {
			return nil, err
		}
	}
	if req.ReplacementKey != nil && *req.ReplacementKey != "" {
		replacement, err := s.catalog.ResolveTagKey(ctx, *req.ReplacementKey)
		if err != nil {
			return nil, err
		}
		req.ReplacementKey = &replacement
	}
	var aliases []string
	if req.Aliases != nil {
		aliases = model.NormalizeTagKeys(req.Aliases)
		for _, alias := range aliases {
			if err := model.ValidateTagKey(alias); err != nil {
				return nil, fmt.Errorf("alias %q: %w", alias, err)
			}
		}
	}
	return s.catalog.UpdateTag(ctx, normalized, model.TagUpdate{
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		Color:          req.Color,
		OwnerType:      req.OwnerType,
		OwnerID:        req.OwnerID,
		OwnerName:      req.OwnerName,
		Scope:          req.Scope,
		Status:         req.Status,
		ReplacementKey: req.ReplacementKey,
		Aliases:        aliases,
		ActorUserID:    actorUserID,
	})
}

func (s *Service) DeprecateTag(ctx context.Context, key, replacementKey string, actorUserID int64) (*model.TagCatalogEntry, error) {
	if s.catalog == nil {
		return nil, errors.New("tag catalog repository is not configured")
	}
	normalized := model.NormalizeTagKey(key)
	if err := model.ValidateTagKey(normalized); err != nil {
		return nil, err
	}
	if replacementKey != "" {
		resolved, err := s.catalog.ResolveTagKey(ctx, replacementKey)
		if err != nil {
			return nil, err
		}
		replacementKey = resolved
	}
	return s.catalog.DeprecateTag(ctx, normalized, replacementKey, actorUserID)
}

func (s *Service) MergeTag(ctx context.Context, sourceKey, targetKey string, actorUserID int64) (*model.TagCatalogEntry, error) {
	if s.catalog == nil {
		return nil, errors.New("tag catalog repository is not configured")
	}
	source := model.NormalizeTagKey(sourceKey)
	target, err := s.catalog.ResolveTagKey(ctx, targetKey)
	if err != nil {
		return nil, err
	}
	if source == target {
		return nil, errors.New("source and target tags must be different")
	}
	return s.catalog.MergeTag(ctx, source, target, actorUserID)
}

func (s *Service) ListTags(ctx context.Context, filter model.TagFilter) ([]model.TagCatalogEntry, int, error) {
	return s.catalog.ListTags(ctx, filter)
}

func (s *Service) TagDetail(ctx context.Context, key string) (*model.TagDetail, error) {
	tag, err := s.catalog.GetTag(ctx, key)
	if err != nil {
		return nil, err
	}
	usage, err := s.catalog.TagUsage(ctx, tag.Key)
	if err != nil {
		return nil, err
	}
	tag.Usage = usage
	audit, _, err := s.catalog.TagAudit(ctx, tag.Key, 20, 0)
	if err != nil {
		return nil, err
	}
	return &model.TagDetail{Tag: *tag, Audit: audit}, nil
}

func (s *Service) ResolveTagKey(ctx context.Context, keyOrAlias string) (string, error) {
	if s.catalog == nil {
		return model.NormalizeTagKey(keyOrAlias), nil
	}
	return s.catalog.ResolveTagKey(ctx, keyOrAlias)
}

func (s *Service) PreviewBulkEdit(ctx context.Context, req model.TagBulkEditRequest, actorUserID int64) (*model.TagBulkEditPreview, error) {
	if err := s.normalizeBulkEditRequest(ctx, &req, actorUserID); err != nil {
		return nil, err
	}
	return s.assignments.PreviewTagBulkEdit(ctx, req)
}

func (s *Service) ApplyBulkEdit(ctx context.Context, req model.TagBulkEditRequest, actorUserID int64) (*model.TagBulkEditResult, error) {
	if err := s.normalizeBulkEditRequest(ctx, &req, actorUserID); err != nil {
		return nil, err
	}
	return s.assignments.ApplyTagBulkEdit(ctx, req)
}

func (s *Service) CreateSavedFilter(ctx context.Context, req SavedFilterRequest, actorUserID int64) (*model.SavedFilter, error) {
	if s.filters == nil {
		return nil, errors.New("saved filter repository is not configured")
	}
	filter, err := s.savedFilterFromRequest(ctx, req, actorUserID, 0)
	if err != nil {
		return nil, err
	}
	return s.filters.CreateSavedFilter(ctx, filter)
}

func (s *Service) UpdateSavedFilter(ctx context.Context, id int64, req SavedFilterRequest, actorUserID int64) (*model.SavedFilter, error) {
	filter, err := s.savedFilterFromRequest(ctx, req, actorUserID, id)
	if err != nil {
		return nil, err
	}
	return s.filters.UpdateSavedFilter(ctx, filter)
}

func (s *Service) ListSavedFilters(ctx context.Context, ownerUserID int64, visibility model.SavedFilterVisibility, filterType string, limit, offset int) ([]model.SavedFilter, int, error) {
	return s.filters.ListSavedFilters(ctx, ownerUserID, visibility, filterType, limit, offset)
}

func (s *Service) GetSavedFilter(ctx context.Context, id int64) (*model.SavedFilter, error) {
	return s.filters.GetSavedFilter(ctx, id)
}

func (s *Service) DeleteSavedFilter(ctx context.Context, id, actorUserID int64) error {
	return s.filters.DeleteSavedFilter(ctx, id, actorUserID)
}

func (s *Service) normalizeBulkEditRequest(ctx context.Context, req *model.TagBulkEditRequest, actorUserID int64) error {
	if s.assignments == nil {
		return errors.New("tag assignment repository is not configured")
	}
	req.ActorUserID = actorUserID
	req.AddTags = model.NormalizeTagKeys(req.AddTags)
	req.RemoveTags = model.NormalizeTagKeys(req.RemoveTags)
	for index, tagKey := range req.AddTags {
		resolved, err := s.catalog.ResolveTagKey(ctx, tagKey)
		if err != nil {
			return err
		}
		tag, err := s.catalog.GetTag(ctx, resolved)
		if err != nil {
			return err
		}
		if !model.CanApplyTag(tag.Status) {
			return fmt.Errorf("tag %s is deprecated", tag.Key)
		}
		req.AddTags[index] = resolved
	}
	for index, tagKey := range req.RemoveTags {
		resolved, err := s.catalog.ResolveTagKey(ctx, tagKey)
		if err == nil {
			req.RemoveTags[index] = resolved
		}
	}
	return nil
}

func (s *Service) savedFilterFromRequest(ctx context.Context, req SavedFilterRequest, actorUserID, id int64) (model.SavedFilter, error) {
	if req.Name == "" {
		return model.SavedFilter{}, errors.New("name is required")
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = model.SavedFilterPrivate
	}
	filterType := req.FilterType
	if filterType == "" {
		filterType = "issues"
	}
	criteria, err := s.normalizeCriteriaTags(ctx, req.Criteria)
	if err != nil {
		return model.SavedFilter{}, err
	}
	return model.SavedFilter{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		OwnerUserID: actorUserID,
		Visibility:  visibility,
		FilterType:  filterType,
		Criteria:    criteria,
	}, nil
}

func (s *Service) normalizeCriteriaTags(ctx context.Context, criteria map[string]any) (map[string]any, error) {
	if criteria == nil {
		criteria = map[string]any{}
	}
	out := make(map[string]any, len(criteria))
	for key, value := range criteria {
		out[key] = value
	}
	if tag, ok := out["tag"].(string); ok && tag != "" && tag != "all" {
		resolved, err := s.catalog.ResolveTagKey(ctx, tag)
		if err != nil {
			return nil, err
		}
		out["tag"] = resolved
	}
	if rawTags, ok := out["tags"]; ok {
		resolvedTags, err := s.resolveCriteriaTagList(ctx, rawTags)
		if err != nil {
			return nil, err
		}
		out["tags"] = resolvedTags
	}
	return out, nil
}

func (s *Service) resolveCriteriaTagList(ctx context.Context, raw any) ([]string, error) {
	var tags []string
	switch values := raw.(type) {
	case []string:
		tags = values
	case []any:
		for _, value := range values {
			if text, ok := value.(string); ok {
				tags = append(tags, text)
			}
		}
	default:
		return []string{}, nil
	}
	tags = model.NormalizeTagKeys(tags)
	for index, tag := range tags {
		resolved, err := s.catalog.ResolveTagKey(ctx, tag)
		if err != nil {
			return nil, err
		}
		tags[index] = resolved
	}
	return tags, nil
}
