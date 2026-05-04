package api

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

//go:embed rules_data
var rulesFS embed.FS

type ruleDetail struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Language    string   `json:"language"`
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Tags        []string `json:"tags,omitempty"`
	Params      []struct {
		Key          string `json:"key"`
		Description  string `json:"description"`
		DefaultValue string `json:"default_value"`
		Type         string `json:"type"`
	} `json:"params,omitempty"`
	Rationale        string `json:"rationale,omitempty"`
	NoncompliantCode string `json:"noncompliant_code,omitempty"`
	CompliantCode    string `json:"compliant_code,omitempty"`
	Origin           string `json:"origin,omitempty"`
	PackName         string `json:"pack_name,omitempty"`
	VersionHash      string `json:"version_hash,omitempty"`
	Lifecycle        string `json:"lifecycle,omitempty"`
}

// RulesHandler serves rule metadata for the issue detail panel.
type RulesHandler struct {
	byKey       map[string]*ruleDetail
	all         []*ruleDetail
	customRules *postgres.CustomRuleRepository
}

// NewRulesHandler creates a RulesHandler by loading embedded rule JSON files.
func NewRulesHandler(customRules *postgres.CustomRuleRepository) *RulesHandler {
	h := &RulesHandler{byKey: make(map[string]*ruleDetail), customRules: customRules}
	_ = fs.WalkDir(rulesFS, "rules_data", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := fs.ReadFile(rulesFS, path)
		if err != nil {
			return nil
		}
		var r ruleDetail
		if json.Unmarshal(data, &r) == nil && r.Key != "" {
			h.byKey[r.Key] = &r
			h.all = append(h.all, &r)
		}
		return nil
	})
	return h
}

// Get handles GET /api/v1/rules/* — returns the full metadata for a single rule.
func (h *RulesHandler) Get(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	key, _ := url.PathUnescape(raw)
	if key == "" {
		jsonError(w, http.StatusBadRequest, "missing rule key")
		return
	}
	rule, ok := h.byKey[key]
	if !ok {
		if customRule := h.getCustomRule(r, key); customRule != nil {
			jsonOK(w, http.StatusOK, customRule)
			return
		}
		jsonError(w, http.StatusNotFound, "rule not found")
		return
	}
	jsonOK(w, http.StatusOK, rule)
}

// List handles GET /api/v1/rules — returns metadata for all registered rules.
func (h *RulesHandler) List(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("language")
	if lang == "" {
		all := append([]*ruleDetail(nil), h.all...)
		all = append(all, h.listCustomRules(r)...)
		jsonOK(w, http.StatusOK, all)
		return
	}
	var filtered []*ruleDetail
	for _, rule := range h.all {
		if rule.Language == lang || rule.Language == "*" {
			filtered = append(filtered, rule)
		}
	}
	for _, rule := range h.listCustomRules(r) {
		if rule.Language == lang || rule.Language == "*" {
			filtered = append(filtered, rule)
		}
	}
	jsonOK(w, http.StatusOK, filtered)
}

func (h *RulesHandler) listCustomRules(r *http.Request) []*ruleDetail {
	if h.customRules == nil {
		return nil
	}
	snapshot, err := h.customRules.PublishedCatalogSnapshot(r.Context())
	if err != nil {
		return nil
	}
	out := make([]*ruleDetail, 0, len(snapshot.Rules))
	for _, rule := range snapshot.Rules {
		out = append(out, customRuleDetail(rule))
	}
	return out
}

func (h *RulesHandler) getCustomRule(r *http.Request, key string) *ruleDetail {
	if h.customRules == nil {
		return nil
	}
	rule, err := h.customRules.PublishedRuleByKey(r.Context(), key)
	if err != nil {
		return nil
	}
	return customRuleDetail(*rule)
}

func customRuleDetail(rule model.CustomRuleDefinition) *ruleDetail {
	params := make([]struct {
		Key          string `json:"key"`
		Description  string `json:"description"`
		DefaultValue string `json:"default_value"`
		Type         string `json:"type"`
	}, 0, len(rule.ParamsSchema))
	for _, param := range rule.ParamsSchema {
		params = append(params, struct {
			Key          string `json:"key"`
			Description  string `json:"description"`
			DefaultValue string `json:"default_value"`
			Type         string `json:"type"`
		}{Key: param.Key, Description: param.Description, DefaultValue: param.DefaultValue, Type: param.Type})
	}
	return &ruleDetail{
		Key:         rule.RuleKey,
		Name:        rule.Name,
		Description: rule.Description,
		Language:    rule.Language,
		Type:        string(rule.Type),
		Severity:    string(rule.DefaultSeverity),
		Tags:        rule.Tags,
		Params:      params,
		Rationale:   rule.Message,
		Origin:      "custom",
		PackName:    rule.PackName,
		VersionHash: rule.VersionHash,
		Lifecycle:   string(rule.Lifecycle),
	}
}
