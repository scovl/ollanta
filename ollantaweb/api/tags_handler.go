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

func (h *TagsHandler) Get(w http.ResponseWriter, r *http.Request) {
	detail, err := h.service.TagDetail(r.Context(), tagKeyParam(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, detail)
}

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

func (h *TagsHandler) Audit(w http.ResponseWriter, r *http.Request) {
	detail, err := h.service.TagDetail(r.Context(), tagKeyParam(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": detail.Audit, "total": len(detail.Audit)})
}

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

func (h *TagsHandler) ListSavedFilters(w http.ResponseWriter, r *http.Request) {
	items, total, err := h.service.ListSavedFilters(r.Context(), actorUserID(r), model.SavedFilterVisibility(r.URL.Query().Get("visibility")), r.URL.Query().Get("filter_type"), parseOptionalInt(r.URL.Query(), "limit"), parseOptionalInt(r.URL.Query(), "offset"))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

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

func (h *TagsHandler) GetSavedFilter(w http.ResponseWriter, r *http.Request) {
	filter, err := h.service.GetSavedFilter(r.Context(), savedFilterIDParam(r))
	if tagError(w, err) {
		return
	}
	jsonOK(w, http.StatusOK, filter)
}

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

func (h *TagsHandler) DeleteSavedFilter(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteSavedFilter(r.Context(), savedFilterIDParam(r), actorUserID(r)); tagError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

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
	if errors.Is(err, postgres.ErrNotFound) || errors.Is(err, model.ErrNotFound) {
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
