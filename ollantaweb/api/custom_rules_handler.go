package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/scovl/ollanta/application/customrules"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

type CustomRulesHandler struct {
	rules *postgres.CustomRuleRepository
}

const (
	customRuleInvalidIDMessage = "invalid id"
	customRuleNotFoundMessage  = "custom rule not found"
)

func NewCustomRulesHandler(rules *postgres.CustomRuleRepository) *CustomRulesHandler {
	return &CustomRulesHandler{rules: rules}
}

// Engines handles GET /api/v1/rule-engines.
// @Summary List rule engines
// @Description Returns available custom rule engines
// @Tags custom-rules
// @Produce json
// @Success 200 {object} enginesResponse
// @Router /api/v1/rule-engines [get]
func (h *CustomRulesHandler) Engines(w http.ResponseWriter, _ *http.Request) {
	jsonOK(w, http.StatusOK, customrules.EngineCapabilities())
}

// List handles GET /api/v1/custom-rules.
// @Summary List custom rules
// @Description Returns all custom rules
// @Tags custom-rules
// @Produce json
// @Success 200 {object} customRuleListResponse
// @Router /api/v1/custom-rules [get]
func (h *CustomRulesHandler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.rules.List(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": rules})
}

// Get handles GET /api/v1/custom-rules/{id}.
// @Summary Get custom rule
// @Description Returns a custom rule by ID
// @Tags custom-rules
// @Produce json
// @Param id path int true "Rule ID"
// @Success 200 {object} model.CustomRuleDefinition
// @Router /api/v1/custom-rules/{id} [get]
func (h *CustomRulesHandler) Get(w http.ResponseWriter, r *http.Request) {
	rule, ok := h.ruleByID(w, r)
	if !ok {
		return
	}
	jsonOK(w, http.StatusOK, rule)
}

// Create handles POST /api/v1/custom-rules.
// @Summary Create custom rules
// @Description Import a custom rule pack document
// @Tags custom-rules
// @Accept json
// @Produce json
// @Param body body model.CustomRulePackDocument true "Rule pack document"
// @Success 201 {object} customRuleListResponse
// @Router /api/v1/custom-rules [post]
func (h *CustomRulesHandler) Create(w http.ResponseWriter, r *http.Request) {
	doc, err := decodeCustomRulePackRequest(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.rules.ImportDocument(r.Context(), doc)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, http.StatusCreated, map[string]any{"items": created})
}

// Update handles PUT /api/v1/custom-rules/{id}.
// @Summary Update custom rule
// @Description Update a custom rule draft
// @Tags custom-rules
// @Accept json
// @Produce json
// @Param id path int true "Rule ID"
// @Param body body model.CustomRuleDefinition true "Rule data"
// @Success 200 {object} model.CustomRuleDefinition
// @Router /api/v1/custom-rules/{id} [put]
func (h *CustomRulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, customRuleInvalidIDMessage)
		return
	}
	var rule model.CustomRuleDefinition
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	updated, err := h.rules.UpdateDraft(r.Context(), id, rule)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, updated)
}

// Validate handles POST /api/v1/custom-rules/{id}/validate.
// @Summary Validate custom rule
// @Description Validate a custom rule definition
// @Tags custom-rules
// @Produce json
// @Param id path int true "Rule ID"
// @Success 200 {object} model.CustomRuleDefinition
// @Router /api/v1/custom-rules/{id}/validate [post]
func (h *CustomRulesHandler) Validate(w http.ResponseWriter, r *http.Request) {
	rule, ok := h.ruleByID(w, r)
	if !ok {
		return
	}
	result := customrules.Validate(r.Context(), *rule, customrules.ValidationContext{AllowExistingRuleKey: rule.RuleKey})
	updated, err := h.rules.StoreValidation(r.Context(), rule.ID, result)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, updated)
}

// Preview handles POST /api/v1/custom-rules/{id}/preview.
// @Summary Preview custom rule
// @Description Preview a custom rule against sample code
// @Tags custom-rules
// @Accept json
// @Produce json
// @Param id path int true "Rule ID"
// @Param body body object{file_path=string,source=string} true "Preview data"
// @Success 200 {object} customRulePreviewResponse
// @Router /api/v1/custom-rules/{id}/preview [post]
func (h *CustomRulesHandler) Preview(w http.ResponseWriter, r *http.Request) {
	rule, ok := h.ruleByID(w, r)
	if !ok {
		return
	}
	var req struct {
		FilePath string `json:"file_path"`
		Source   string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.FilePath == "" {
		req.FilePath = rule.RuleKey + ".example"
	}
	matches, diagnostics := customrules.Evaluate(*rule, req.FilePath, []byte(req.Source))
	jsonOK(w, http.StatusOK, model.CustomRulePreviewResult{
		RuleKey:      rule.RuleKey,
		FilesScanned: 1,
		MatchCount:   len(matches),
		Matches:      matches,
		Diagnostics:  diagnostics,
	})
}

// Publish handles POST /api/v1/custom-rules/{id}/publish.
// @Summary Publish custom rule
// @Description Publish a custom rule draft
// @Tags custom-rules
// @Produce json
// @Param id path int true "Rule ID"
// @Success 200 {object} model.CustomRuleDefinition
// @Router /api/v1/custom-rules/{id}/publish [post]
func (h *CustomRulesHandler) Publish(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.rules.Publish)
}

// Disable handles POST /api/v1/custom-rules/{id}/disable.
// @Summary Disable custom rule
// @Description Disable a published custom rule
// @Tags custom-rules
// @Produce json
// @Param id path int true "Rule ID"
// @Success 200 {object} model.CustomRuleDefinition
// @Router /api/v1/custom-rules/{id}/disable [post]
func (h *CustomRulesHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.rules.Disable)
}

// Deprecate handles POST /api/v1/custom-rules/{id}/deprecate.
// @Summary Deprecate custom rule
// @Description Deprecate a custom rule
// @Tags custom-rules
// @Produce json
// @Param id path int true "Rule ID"
// @Success 200 {object} model.CustomRuleDefinition
// @Router /api/v1/custom-rules/{id}/deprecate [post]
func (h *CustomRulesHandler) Deprecate(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.rules.Deprecate)
}

// Import handles POST /api/v1/custom-rules/import.
// @Summary Import custom rules
// @Description Import custom rules from a document
// @Tags custom-rules
// @Accept json
// @Produce json
// @Param body body model.CustomRulePackDocument true "Rule pack document"
// @Success 201 {object} customRuleListResponse
// @Router /api/v1/custom-rules/import [post]
func (h *CustomRulesHandler) Import(w http.ResponseWriter, r *http.Request) {
	h.Create(w, r)
}

// Export handles GET /api/v1/custom-rules/{id}/export.
// @Summary Export custom rule
// @Description Export a custom rule as a document
// @Tags custom-rules
// @Produce json
// @Param id path int true "Rule ID"
// @Success 200 {object} model.CustomRulePackDocument
// @Router /api/v1/custom-rules/{id}/export [get]
func (h *CustomRulesHandler) Export(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, customRuleInvalidIDMessage)
		return
	}
	doc, err := h.rules.ExportDocument(r.Context(), id)
	if handleNotFound(w, err, customRuleNotFoundMessage) {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, doc)
}

// Audit handles GET /api/v1/custom-rules/{id}/audit.
// @Summary Custom rule audit
// @Description Returns audit log for a custom rule
// @Tags custom-rules
// @Produce json
// @Param id path int true "Rule ID"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} paginatedItemsResponse
// @Router /api/v1/custom-rules/{id}/audit [get]
func (h *CustomRulesHandler) Audit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, customRuleInvalidIDMessage)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	entries, total, err := h.rules.Audit(r.Context(), id, limit, offset)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": entries, "total": total, "limit": limit, "offset": offset})
}

// Catalog handles GET /api/v1/custom-rules/catalog.
// @Summary Custom rules catalog
// @Description Returns the published custom rules catalog
// @Tags custom-rules
// @Produce json
// @Success 200 {object} model.CustomRuleCatalogSnapshot
// @Router /api/v1/custom-rules/catalog [get]
func (h *CustomRulesHandler) Catalog(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.rules.PublishedCatalogSnapshot(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, snapshot)
}

func (h *CustomRulesHandler) transition(w http.ResponseWriter, r *http.Request, fn func(context.Context, int64) (*model.CustomRuleDefinition, error)) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, customRuleInvalidIDMessage)
		return
	}
	rule, err := fn(r.Context(), id)
	if handleNotFound(w, err, customRuleNotFoundMessage) {
		return
	}
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, rule)
}

func (h *CustomRulesHandler) ruleByID(w http.ResponseWriter, r *http.Request) (*model.CustomRuleDefinition, bool) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, customRuleInvalidIDMessage)
		return nil, false
	}
	rule, err := h.rules.Get(r.Context(), id)
	if handleNotFound(w, err, customRuleNotFoundMessage) {
		return nil, false
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	return rule, true
}

func decodeCustomRulePackRequest(r *http.Request) (model.CustomRulePackDocument, error) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return model.CustomRulePackDocument{}, err
	}
	return customrules.DecodeDocument(data)
}
