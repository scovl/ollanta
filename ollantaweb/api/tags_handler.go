package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/scovl/ollanta/application/tagging"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

type TagsHandler struct {
	service *tagging.Service
	perms   *postgres.PermissionRepository
}

const (
	tagErrorInsufficientPermissions = "insufficient permissions"
	tagErrorInvalidJSON             = "invalid JSON"
)

func NewTagsHandler(service *tagging.Service, perms *postgres.PermissionRepository) *TagsHandler {
	return &TagsHandler{service: service, perms: perms}
}

// List handles GET /api/v1/tags.
// @Summary List tags
// @Description Returns tags with optional filters
// @Tags tags
// @Produce json
// @Param search query string false "Search query"
// @Param status query string false "Status"
// @Param owner query string false "Owner"
// @Param scope query string false "Scope"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} tagListResponse
// @Router /api/v1/tags [get]
func (h *TagsHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := model.TagFilter{
		Query:  r.URL.Query().Get("search"),
		Status: model.TagStatus(r.URL.Query().Get("status")),
		Owner:  r.URL.Query().Get("owner"),
		Scope:  r.URL.Query().Get("scope"),
		Limit:  parseOptionalInt(r.URL.Query(), "limit"),
		Offset: parseOptionalInt(r.URL.Query(), "offset"),
	}
	items, total, err := h.service.ListTags(r.Context(), filter)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": filter.Limit, "offset": filter.Offset})
}

// Create handles POST /api/v1/tags.
// @Summary Create tag
// @Description Create a new tag
// @Tags tags
// @Accept json
// @Produce json
// @Param body body tagging.CreateTagRequest true "Tag data"
// @Success 201 {object} model.TagCatalogEntry
// @Router /api/v1/tags [post]
func (h *TagsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.hasAdmin(r) {
		jsonError(w, http.StatusForbidden, tagErrorInsufficientPermissions)
		return
	}
	var req tagging.CreateTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, tagErrorInvalidJSON)
		return
	}
	tag, err := h.service.CreateTag(r.Context(), req, actorUserID(r))
	writeTagMutationResult(w, tag, err, http.StatusCreated)
}

// Get handles GET /api/v1/tags/{key}.
// @Summary Get tag
// @Description Returns tag details by key
// @Tags tags
// @Produce json
// @Param key path string true "Tag key"
// @Success 200 {object} model.TagCatalogEntry
// @Router /api/v1/tags/{key} [get]
func (h *TagsHandler) Get(w http.ResponseWriter, r *http.Request) {
	detail, err := h.service.TagDetail(r.Context(), tagKeyParam(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, detail)
}

// Update handles PUT /api/v1/tags/{key}.
// @Summary Update tag
// @Description Update a tag
// @Tags tags
// @Accept json
// @Produce json
// @Param key path string true "Tag key"
// @Param body body tagging.UpdateTagRequest true "Tag data"
// @Success 200 {object} model.TagCatalogEntry
// @Router /api/v1/tags/{key} [put]
func (h *TagsHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := tagKeyParam(r)
	if !h.canGovernTag(r, key) {
		jsonError(w, http.StatusForbidden, tagErrorInsufficientPermissions)
		return
	}
	var req tagging.UpdateTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, tagErrorInvalidJSON)
		return
	}
	tag, err := h.service.UpdateTag(r.Context(), key, req, actorUserID(r))
	writeTagMutationResult(w, tag, err, http.StatusOK)
}

// Deprecate handles POST /api/v1/tags/{key}/deprecate.
// @Summary Deprecate tag
// @Description Deprecate a tag
// @Tags tags
// @Accept json
// @Produce json
// @Param key path string true "Tag key"
// @Param body body object{replacement_key=string} true "Deprecate data"
// @Success 200 {object} model.TagCatalogEntry
// @Router /api/v1/tags/{key}/deprecate [post]
func (h *TagsHandler) Deprecate(w http.ResponseWriter, r *http.Request) {
	key := tagKeyParam(r)
	if !h.canGovernTag(r, key) {
		jsonError(w, http.StatusForbidden, tagErrorInsufficientPermissions)
		return
	}
	var req struct {
		ReplacementKey string `json:"replacement_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		jsonError(w, http.StatusBadRequest, tagErrorInvalidJSON)
		return
	}
	tag, err := h.service.DeprecateTag(r.Context(), key, req.ReplacementKey, actorUserID(r))
	writeTagMutationResult(w, tag, err, http.StatusOK)
}

// Merge handles POST /api/v1/tags/{key}/merge.
// @Summary Merge tag
// @Description Merge a tag into another
// @Tags tags
// @Accept json
// @Produce json
// @Param key path string true "Tag key"
// @Param body body object{target_key=string} true "Merge data"
// @Success 200 {object} model.TagCatalogEntry
// @Router /api/v1/tags/{key}/merge [post]
func (h *TagsHandler) Merge(w http.ResponseWriter, r *http.Request) {
	key := tagKeyParam(r)
	if !h.canGovernTag(r, key) {
		jsonError(w, http.StatusForbidden, tagErrorInsufficientPermissions)
		return
	}
	var req struct {
		TargetKey string `json:"target_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, tagErrorInvalidJSON)
		return
	}
	tag, err := h.service.MergeTag(r.Context(), key, req.TargetKey, actorUserID(r))
	writeTagMutationResult(w, tag, err, http.StatusOK)
}

// Audit handles GET /api/v1/tags/{key}/audit.
// @Summary Tag audit
// @Description Returns audit log for a tag
// @Tags tags
// @Produce json
// @Param key path string true "Tag key"
// @Success 200 {object} itemsOnlyResponse
// @Router /api/v1/tags/{key}/audit [get]
func (h *TagsHandler) Audit(w http.ResponseWriter, r *http.Request) {
	detail, err := h.service.TagDetail(r.Context(), tagKeyParam(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": detail.Audit, "total": len(detail.Audit)})
}

// BulkPreview handles POST /api/v1/tags/bulk/preview.
// @Summary Bulk preview
// @Description Preview bulk tag edit results
// @Tags tags
// @Accept json
// @Produce json
// @Param body body model.TagBulkEditRequest true "Bulk edit request"
// @Success 200 {object} model.TagBulkEditPreview
// @Router /api/v1/tags/bulk/preview [post]
func (h *TagsHandler) BulkPreview(w http.ResponseWriter, r *http.Request) {
	var req model.TagBulkEditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, tagErrorInvalidJSON)
		return
	}
	preview, err := h.service.PreviewBulkEdit(r.Context(), req, actorUserID(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, preview)
}

// BulkApply handles POST /api/v1/tags/bulk/apply.
// @Summary Bulk apply
// @Description Apply bulk tag edit
// @Tags tags
// @Accept json
// @Produce json
// @Param body body model.TagBulkEditRequest true "Bulk edit request"
// @Success 200 {object} model.TagBulkEditResult
// @Router /api/v1/tags/bulk/apply [post]
func (h *TagsHandler) BulkApply(w http.ResponseWriter, r *http.Request) {
	var req model.TagBulkEditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, tagErrorInvalidJSON)
		return
	}
	result, err := h.service.ApplyBulkEdit(r.Context(), req, actorUserID(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, result)
}

// ListSavedFilters handles GET /api/v1/saved-filters.
// @Summary List saved filters
// @Description Returns saved filters for the current user
// @Tags saved-filters
// @Produce json
// @Param visibility query string false "Visibility"
// @Param filter_type query string false "Filter type"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} savedFilterListResponse
// @Router /api/v1/saved-filters [get]
func (h *TagsHandler) ListSavedFilters(w http.ResponseWriter, r *http.Request) {
	items, total, err := h.service.ListSavedFilters(r.Context(), actorUserID(r), model.SavedFilterVisibility(r.URL.Query().Get("visibility")), r.URL.Query().Get("filter_type"), parseOptionalInt(r.URL.Query(), "limit"), parseOptionalInt(r.URL.Query(), "offset"))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

// CreateSavedFilter handles POST /api/v1/saved-filters.
// @Summary Create saved filter
// @Description Create a new saved filter
// @Tags saved-filters
// @Accept json
// @Produce json
// @Param body body tagging.SavedFilterRequest true "Filter data"
// @Success 201 {object} model.SavedFilter
// @Router /api/v1/saved-filters [post]
func (h *TagsHandler) CreateSavedFilter(w http.ResponseWriter, r *http.Request) {
	var req tagging.SavedFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, tagErrorInvalidJSON)
		return
	}
	filter, err := h.service.CreateSavedFilter(r.Context(), req, actorUserID(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusCreated, filter)
}

// GetSavedFilter handles GET /api/v1/saved-filters/{id}.
// @Summary Get saved filter
// @Description Returns a saved filter by ID
// @Tags saved-filters
// @Produce json
// @Param id path int true "Filter ID"
// @Success 200 {object} model.SavedFilter
// @Router /api/v1/saved-filters/{id} [get]
func (h *TagsHandler) GetSavedFilter(w http.ResponseWriter, r *http.Request) {
	filter, err := h.service.GetSavedFilter(r.Context(), savedFilterIDParam(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, filter)
}

// UpdateSavedFilter handles PUT /api/v1/saved-filters/{id}.
// @Summary Update saved filter
// @Description Update a saved filter
// @Tags saved-filters
// @Accept json
// @Produce json
// @Param id path int true "Filter ID"
// @Param body body tagging.SavedFilterRequest true "Filter data"
// @Success 200 {object} model.SavedFilter
// @Router /api/v1/saved-filters/{id} [put]
func (h *TagsHandler) UpdateSavedFilter(w http.ResponseWriter, r *http.Request) {
	var req tagging.SavedFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, tagErrorInvalidJSON)
		return
	}
	filter, err := h.service.UpdateSavedFilter(r.Context(), savedFilterIDParam(r), req, actorUserID(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, filter)
}

// DeleteSavedFilter handles DELETE /api/v1/saved-filters/{id}.
// @Summary Delete saved filter
// @Description Delete a saved filter
// @Tags saved-filters
// @Param id path int true "Filter ID"
// @Success 204
// @Router /api/v1/saved-filters/{id} [delete]
func (h *TagsHandler) DeleteSavedFilter(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteSavedFilter(r.Context(), savedFilterIDParam(r), actorUserID(r)); tagError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ApplySavedFilter handles POST /api/v1/saved-filters/{id}/apply.
// @Summary Apply saved filter
// @Description Apply a saved filter and return criteria
// @Tags saved-filters
// @Produce json
// @Param id path int true "Filter ID"
// @Success 200 {object} savedFilterCriteriaResponse
// @Router /api/v1/saved-filters/{id}/apply [post]
func (h *TagsHandler) ApplySavedFilter(w http.ResponseWriter, r *http.Request) {
	filter, err := h.service.GetSavedFilter(r.Context(), savedFilterIDParam(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"criteria": filter.Criteria, "filter": filter})
}

func (h *TagsHandler) hasAdmin(r *http.Request) bool {
	user := UserFromContext(r.Context())
	if user == nil || h.perms == nil {
		return false
	}
	ok, err := h.perms.HasGlobal(r.Context(), user.ID, "admin")
	return err == nil && ok
}

func (h *TagsHandler) canGovernTag(r *http.Request, key string) bool {
	if h.hasAdmin(r) {
		return true
	}
	user := UserFromContext(r.Context())
	if user == nil {
		return false
	}
	detail, err := h.service.TagDetail(r.Context(), key)
	if err != nil {
		return false
	}
	return detail.Tag.OwnerType == model.TagOwnerUser && detail.Tag.OwnerID == user.ID
}

func writeTagMutationResult(w http.ResponseWriter, tag *model.TagCatalogEntry, err error, status int) {
	if tagError(w, err) {
		return
	}
	jsonOK(w, status, tag)
}

func tagError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "not found")
		return true
	}
	if errors.Is(err, model.ErrInvalidTagKey) || errors.Is(err, model.ErrInvalidTagColor) {
		jsonError(w, http.StatusBadRequest, err.Error())
		return true
	}
	jsonError(w, http.StatusBadRequest, err.Error())
	return true
}

func tagKeyParam(r *http.Request) string {
	raw := routeParam(r, "key")
	value, err := url.PathUnescape(raw)
	if err != nil {
		return raw
	}
	return value
}

func savedFilterIDParam(r *http.Request) int64 {
	id, err := strconv.ParseInt(routeParam(r, "id"), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func actorUserID(r *http.Request) int64 {
	if user := UserFromContext(r.Context()); user != nil {
		return user.ID
	}
	return 0
}
