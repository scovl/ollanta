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

func (h *CustomRulesHandler) Engines(w http.ResponseWriter, _ *http.Request) {
	jsonOK(w, http.StatusOK, customrules.EngineCapabilities())
}

func (h *CustomRulesHandler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.rules.List(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": rules})
}

func (h *CustomRulesHandler) Get(w http.ResponseWriter, r *http.Request) {
	rule, ok := h.ruleByID(w, r)
	if !ok {
		return
	}
	jsonOK(w, http.StatusOK, rule)
}

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

func (h *CustomRulesHandler) Publish(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.rules.Publish)
}

func (h *CustomRulesHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.rules.Disable)
}

func (h *CustomRulesHandler) Deprecate(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, h.rules.Deprecate)
}

func (h *CustomRulesHandler) Import(w http.ResponseWriter, r *http.Request) {
	h.Create(w, r)
}

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
