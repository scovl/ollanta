package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/scovl/ollanta/application/tagging"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

func TestTagsHandlerCreateAndList(t *testing.T) {
	repo := newAPITagRepo()
	handler := NewTagsHandler(tagging.NewService(repo, repo, repo), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tags", strings.NewReader(`{"key":"Team API","color":"#0ea5e9"}`))
	req = WithUser(req, &postgres.User{ID: 1, Login: "admin", IsActive: true})
	// No permission repository in this unit test; exercise the service path directly.
	tag, err := handler.service.CreateTag(req.Context(), tagging.CreateTagRequest{Key: "Team API", Color: "#0ea5e9"}, 1)
	if err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}
	if tag.Key != "team-api" {
		t.Fatalf("tag key = %q, want team-api", tag.Key)
	}

	rr := httptest.NewRecorder()
	handler.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tags?search=team", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", rr.Code, rr.Body.String())
	}
	var response struct {
		Items []model.TagCatalogEntry `json:"items"`
		Total int                     `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if response.Total != 1 || response.Items[0].Key != "team-api" {
		t.Fatalf("response = %+v, want listed tag", response)
	}
}

func TestTagsHandlerSavedFilterResolvesAlias(t *testing.T) {
	repo := newAPITagRepo()
	repo.tags["team-api"] = model.TagCatalogEntry{Key: "team-api", DisplayName: "Team API", Status: model.TagStatusActive}
	repo.aliases["api-team"] = "team-api"
	handler := NewTagsHandler(tagging.NewService(repo, repo, repo), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/saved-filters", strings.NewReader(`{"name":"API issues","criteria":{"tag":"api-team"}}`))
	req = WithUser(req, &postgres.User{ID: 5, Login: "dev", IsActive: true})
	rr := httptest.NewRecorder()

	handler.CreateSavedFilter(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s, want 201", rr.Code, rr.Body.String())
	}
	var filter model.SavedFilter
	if err := json.NewDecoder(rr.Body).Decode(&filter); err != nil {
		t.Fatalf("decode filter: %v", err)
	}
	if filter.Criteria["tag"] != "team-api" {
		t.Fatalf("criteria = %+v, want canonical tag", filter.Criteria)
	}
}

func TestIssuesHandlerResolvesTagAlias(t *testing.T) {
	repo := newAPITagRepo()
	repo.tags["team-api"] = model.TagCatalogEntry{Key: "team-api", Status: model.TagStatusActive}
	repo.aliases["api-team"] = "team-api"
	handler := &IssuesHandler{tags: tagging.NewService(repo, repo, repo)}
	filter := postgres.IssueFilter{Tag: stringPtr("api-team")}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues?tag=api-team", nil)

	if err := handler.resolveTagFilter(req, &filter); err != nil {
		t.Fatalf("resolveTagFilter() error = %v", err)
	}
	if filter.Tag == nil || *filter.Tag != "team-api" {
		t.Fatalf("filter.Tag = %#v, want team-api", filter.Tag)
	}
}

func tagRequestWithKey(method, target, key, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("key", key)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func stringPtr(value string) *string { return &value }

type apiTagRepo struct {
	tags    map[string]model.TagCatalogEntry
	aliases map[string]string
	filters []model.SavedFilter
}

func newAPITagRepo() *apiTagRepo {
	return &apiTagRepo{tags: map[string]model.TagCatalogEntry{}, aliases: map[string]string{}}
}

func (r *apiTagRepo) CreateTag(_ context.Context, tag model.TagCatalogEntry) (*model.TagCatalogEntry, error) {
	r.tags[tag.Key] = tag
	return &tag, nil
}

func (r *apiTagRepo) UpdateTag(_ context.Context, key string, update model.TagUpdate) (*model.TagCatalogEntry, error) {
	tag := r.tags[key]
	if update.DisplayName != nil {
		tag.DisplayName = *update.DisplayName
	}
	if update.Aliases != nil {
		for _, alias := range update.Aliases {
			r.aliases[alias] = key
		}
	}
	r.tags[key] = tag
	return &tag, nil
}

func (r *apiTagRepo) DeprecateTag(_ context.Context, key, replacementKey string, _ int64) (*model.TagCatalogEntry, error) {
	tag := r.tags[key]
	tag.Status = model.TagStatusDeprecated
	tag.ReplacementKey = replacementKey
	r.tags[key] = tag
	return &tag, nil
}

func (r *apiTagRepo) MergeTag(_ context.Context, sourceKey, targetKey string, _ int64) (*model.TagCatalogEntry, error) {
	r.aliases[sourceKey] = targetKey
	tag := r.tags[targetKey]
	return &tag, nil
}

func (r *apiTagRepo) GetTag(_ context.Context, keyOrAlias string) (*model.TagCatalogEntry, error) {
	key, err := r.ResolveTagKey(context.Background(), keyOrAlias)
	if err != nil {
		return nil, err
	}
	tag := r.tags[key]
	return &tag, nil
}

func (r *apiTagRepo) ListTags(_ context.Context, filter model.TagFilter) ([]model.TagCatalogEntry, int, error) {
	var out []model.TagCatalogEntry
	query := strings.ToLower(filter.Query)
	for _, tag := range r.tags {
		if query == "" || strings.Contains(tag.Key, query) || strings.Contains(strings.ToLower(tag.DisplayName), query) {
			out = append(out, tag)
		}
	}
	return out, len(out), nil
}

func (r *apiTagRepo) DiscoverTags(context.Context, []string, model.TagSource) error { return nil }

func (r *apiTagRepo) ResolveTagKey(_ context.Context, keyOrAlias string) (string, error) {
	key := model.NormalizeTagKey(keyOrAlias)
	if target, ok := r.aliases[key]; ok {
		return target, nil
	}
	if _, ok := r.tags[key]; !ok {
		return "", postgres.ErrNotFound
	}
	return key, nil
}

func (r *apiTagRepo) TagUsage(context.Context, string) (model.TagUsageSummary, error) {
	return model.TagUsageSummary{}, nil
}

func (r *apiTagRepo) TagAudit(context.Context, string, int, int) ([]model.TagAuditEntry, int, error) {
	return nil, 0, nil
}

func (r *apiTagRepo) PreviewTagBulkEdit(_ context.Context, req model.TagBulkEditRequest) (*model.TagBulkEditPreview, error) {
	return &model.TagBulkEditPreview{TargetType: req.TargetType, TargetCount: len(req.TargetIDs), AddTags: req.AddTags}, nil
}

func (r *apiTagRepo) ApplyTagBulkEdit(_ context.Context, req model.TagBulkEditRequest) (*model.TagBulkEditResult, error) {
	return &model.TagBulkEditResult{TagBulkEditPreview: model.TagBulkEditPreview{TargetType: req.TargetType, TargetCount: len(req.TargetIDs)}, UpdatedCount: len(req.TargetIDs)}, nil
}

func (r *apiTagRepo) CreateSavedFilter(_ context.Context, filter model.SavedFilter) (*model.SavedFilter, error) {
	filter.ID = int64(len(r.filters) + 1)
	r.filters = append(r.filters, filter)
	return &filter, nil
}

func (r *apiTagRepo) UpdateSavedFilter(_ context.Context, filter model.SavedFilter) (*model.SavedFilter, error) {
	return &filter, nil
}

func (r *apiTagRepo) GetSavedFilter(_ context.Context, id int64) (*model.SavedFilter, error) {
	for _, filter := range r.filters {
		if filter.ID == id {
			return &filter, nil
		}
	}
	return nil, postgres.ErrNotFound
}

func (r *apiTagRepo) ListSavedFilters(context.Context, int64, model.SavedFilterVisibility, string, int, int) ([]model.SavedFilter, int, error) {
	return r.filters, len(r.filters), nil
}

func (r *apiTagRepo) DeleteSavedFilter(context.Context, int64, int64) error { return nil }
