package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"gopkg.in/yaml.v3"
)

const (
	profileInvalidIDMessage       = "invalid id"
	profileInvalidJSONMessage     = "invalid json"
	profileInvalidDocumentMessage = "invalid profile document"
	profileNotFoundMessage        = "profile not found"
)

// ProfilesHandler handles quality profile API endpoints.
type ProfilesHandler struct {
	profiles port.IProfileRepo
	projects *postgres.ProjectRepository
}

type profileCodeDocument struct {
	Version  int                   `json:"version" yaml:"version"`
	Language string                `json:"language,omitempty" yaml:"language,omitempty"`
	Name     string                `json:"name,omitempty" yaml:"name,omitempty"`
	Rules    []profileCodeRule     `json:"rules,omitempty" yaml:"rules,omitempty"`
	Profiles []profileCodeLanguage `json:"profiles,omitempty" yaml:"profiles,omitempty"`
}

type profileCodeLanguage struct {
	Language string            `json:"language" yaml:"language"`
	Name     string            `json:"name,omitempty" yaml:"name,omitempty"`
	Rules    []profileCodeRule `json:"rules" yaml:"rules"`
}

type profileCodeRule struct {
	Key      string            `json:"key,omitempty" yaml:"key,omitempty"`
	RuleKey  string            `json:"rule_key,omitempty" yaml:"rule_key,omitempty"`
	Rule     string            `json:"rule,omitempty" yaml:"rule,omitempty"`
	Severity string            `json:"severity,omitempty" yaml:"severity,omitempty"`
	Params   map[string]string `json:"params,omitempty" yaml:"params,omitempty"`
	Active   *bool             `json:"active,omitempty" yaml:"active,omitempty"`
	Activate *bool             `json:"activate,omitempty" yaml:"activate,omitempty"`
}

// NewProfilesHandler creates a ProfilesHandler.
func NewProfilesHandler(profiles port.IProfileRepo, projects *postgres.ProjectRepository) *ProfilesHandler {
	return &ProfilesHandler{profiles: profiles, projects: projects}
}

// List handles GET /api/v1/profiles?language=go
// @Summary List quality profiles
// @Description Returns quality profiles, optionally filtered by language
// @Tags quality-profiles
// @Produce json
// @Param language query string false "Language"
// @Success 200 {array} model.QualityProfile
// @Router /api/v1/profiles [get]
func (h *ProfilesHandler) List(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("language")
	list, err := h.profiles.List(r.Context(), lang)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, list)
}

// Get handles GET /api/v1/profiles/{id}
// @Summary Get quality profile
// @Description Returns a quality profile by ID
// @Tags quality-profiles
// @Produce json
// @Param id path int true "Profile ID"
// @Success 200 {object} model.QualityProfile
// @Router /api/v1/profiles/{id} [get]
func (h *ProfilesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	p, err := h.profiles.GetByID(r.Context(), id)
	if handleNotFound(w, err, profileNotFoundMessage) {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, p)
}

// Create handles POST /api/v1/profiles
// @Summary Create quality profile
// @Description Create a new quality profile
// @Tags quality-profiles
// @Accept json
// @Produce json
// @Param body body model.QualityProfile true "Profile data"
// @Success 201 {object} model.QualityProfile
// @Router /api/v1/profiles [post]
func (h *ProfilesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p model.QualityProfile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidJSONMessage)
		return
	}
	if err := h.profiles.Create(r.Context(), &p); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, http.StatusCreated, p)
}

// Update handles PUT /api/v1/profiles/{id}
// @Summary Update quality profile
// @Description Update a quality profile
// @Tags quality-profiles
// @Accept json
// @Produce json
// @Param id path int true "Profile ID"
// @Param body body model.QualityProfile true "Profile data"
// @Success 200 {object} model.QualityProfile
// @Router /api/v1/profiles/{id} [put]
func (h *ProfilesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	var p model.QualityProfile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidJSONMessage)
		return
	}
	p.ID = id
	if err := h.profiles.Update(r.Context(), &p); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, p)
}

// Delete handles DELETE /api/v1/profiles/{id}
// @Summary Delete quality profile
// @Description Delete a quality profile
// @Tags quality-profiles
// @Param id path int true "Profile ID"
// @Success 204
// @Router /api/v1/profiles/{id} [delete]
func (h *ProfilesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	if err := h.profiles.Delete(r.Context(), id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ActivateRule handles POST /api/v1/profiles/{id}/rules
// @Summary Activate rule
// @Description Activate a rule in a quality profile
// @Tags quality-profiles
// @Accept json
// @Param id path int true "Profile ID"
// @Param body body object{rule_key=string,severity=string,params=map[string]string} true "Activation data"
// @Success 204
// @Router /api/v1/profiles/{id}/rules [post]
func (h *ProfilesHandler) ActivateRule(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	var req struct {
		RuleKey  string            `json:"rule_key"`
		Severity string            `json:"severity"`
		Params   map[string]string `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidJSONMessage)
		return
	}
	if err := h.profiles.ActivateRule(r.Context(), id, req.RuleKey, req.Severity, req.Params); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeactivateRule handles DELETE /api/v1/profiles/{id}/rules/{rule}
// @Summary Deactivate rule
// @Description Deactivate a rule in a quality profile
// @Tags quality-profiles
// @Param id path int true "Profile ID"
// @Param rule path string true "Rule key"
// @Success 204
// @Router /api/v1/profiles/{id}/rules/{rule} [delete]
func (h *ProfilesHandler) DeactivateRule(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	rule := routeParam(r, "rule")
	if err := h.profiles.DeactivateRule(r.Context(), id, rule); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// EffectiveRules handles GET /api/v1/profiles/{id}/effective-rules
// @Summary Effective rules
// @Description Returns effective rules for a quality profile
// @Tags quality-profiles
// @Produce json
// @Param id path int true "Profile ID"
// @Success 200 {array} model.EffectiveRule
// @Router /api/v1/profiles/{id}/effective-rules [get]
func (h *ProfilesHandler) EffectiveRules(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	rules, err := h.profiles.ResolveEffectiveRules(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, rules)
}

// Import handles POST /api/v1/profiles/{id}/import.
// @Summary Import rules
// @Description Import rules into a quality profile from YAML/JSON
// @Tags quality-profiles
// @Accept json
// @Produce json
// @Param id path int true "Profile ID"
// @Param body body profileCodeDocument true "Profile document"
// @Success 200 {object} profileImportResponse
// @Router /api/v1/profiles/{id}/import [post]
func (h *ProfilesHandler) Import(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	profile, err := h.profiles.GetByID(r.Context(), id)
	if handleNotFound(w, err, profileNotFoundMessage) {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	doc, err := decodeProfileCodeDocument(r)
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidDocumentMessage)
		return
	}
	rules, err := profileImportRules(doc, profile.Language)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries := make([]model.ProfileYAMLEntry, 0, len(rules))
	for _, rule := range rules {
		entries = append(entries, model.ProfileYAMLEntry{RuleKey: profileCodeRuleKey(rule), Severity: rule.Severity, Params: rule.Params, Activate: profileCodeRuleActive(rule)})
	}
	if err := h.profiles.ApplyProfileRules(r.Context(), id, entries); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"imported_rules": len(rules)})
}

// Export handles GET /api/v1/profiles/{id}/export.
// @Summary Export profile
// @Description Export a quality profile as JSON
// @Tags quality-profiles
// @Produce json
// @Param id path int true "Profile ID"
// @Success 200 {object} profileCodeDocument
// @Router /api/v1/profiles/{id}/export [get]
func (h *ProfilesHandler) Export(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	profile, err := h.profiles.GetByID(r.Context(), id)
	if handleNotFound(w, err, profileNotFoundMessage) {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rules, err := h.profiles.ResolveEffectiveRules(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	doc := profileCodeDocument{Version: 1, Language: profile.Language, Name: profile.Name, Rules: make([]profileCodeRule, 0, len(rules))}
	for _, rule := range rules {
		active := !rule.Disabled && !strings.EqualFold(rule.Severity, "off")
		doc.Rules = append(doc.Rules, profileCodeRule{Key: rule.RuleKey, Severity: rule.Severity, Params: rule.Params, Active: &active})
	}
	jsonOK(w, http.StatusOK, doc)
}

// Changelog handles GET /api/v1/profiles/{id}/changelog.
// @Summary Profile changelog
// @Description Returns changelog entries for a quality profile
// @Tags quality-profiles
// @Produce json
// @Param id path int true "Profile ID"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} profileChangelogResponse
// @Router /api/v1/profiles/{id}/changelog [get]
func (h *ProfilesHandler) Changelog(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	entries, total, err := h.profiles.ProfileChangelog(r.Context(), id, limit, offset)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, map[string]any{"items": entries, "total": total, "limit": limit, "offset": offset})
}

// AssignToProject handles POST /api/v1/projects/{key}/profiles
// @Summary Assign profile to project
// @Description Assign a quality profile to a project
// @Tags quality-profiles
// @Accept json
// @Param key path string true "Project key"
// @Param body body object{language=string,profile_id=int64} true "Assignment data"
// @Success 204
// @Router /api/v1/projects/{key}/profiles [post]
func (h *ProfilesHandler) AssignToProject(w http.ResponseWriter, r *http.Request) {
	key := routeParam(r, "key")
	project, err := h.projects.GetByKey(r.Context(), key)
	if handleNotFound(w, err, "project not found") {
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var req struct {
		Language  string `json:"language"`
		ProfileID int64  `json:"profile_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidJSONMessage)
		return
	}
	if err := h.profiles.AssignToProject(r.Context(), project.ID, req.Language, req.ProfileID); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ProjectProfiles handles GET /api/v1/projects/{key}/profiles.
// @Summary Project profiles
// @Description Returns profiles assigned to a project
// @Tags quality-profiles
// @Produce json
// @Param key path string true "Project key"
// @Success 200 {array} model.ProjectQualityProfile
// @Router /api/v1/projects/{key}/profiles [get]
func (h *ProfilesHandler) ProjectProfiles(w http.ResponseWriter, r *http.Request) {
	project, ok := h.resolveProject(w, r)
	if !ok {
		return
	}
	profiles, err := h.profiles.ProjectProfiles(r.Context(), project.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, profiles)
}

// ProjectEffectiveProfiles handles GET /api/v1/projects/{key}/profiles/effective.
// @Summary Project effective profiles
// @Description Returns effective profiles for a project
// @Tags quality-profiles
// @Produce json
// @Param key path string true "Project key"
// @Success 200 {array} model.EffectiveQualityProfile
// @Router /api/v1/projects/{key}/profiles/effective [get]
func (h *ProfilesHandler) ProjectEffectiveProfiles(w http.ResponseWriter, r *http.Request) {
	project, ok := h.resolveProject(w, r)
	if !ok {
		return
	}
	profiles, err := h.profiles.ProjectEffectiveProfiles(r.Context(), project.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusOK, profiles)
}

// Copy handles POST /api/v1/profiles/{id}/copy
// @Summary Copy profile
// @Description Copy a quality profile
// @Tags quality-profiles
// @Accept json
// @Produce json
// @Param id path int true "Profile ID"
// @Param body body object{name=string} true "Copy data"
// @Success 201 {object} model.QualityProfile
// @Router /api/v1/profiles/{id}/copy [post]
func (h *ProfilesHandler) Copy(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	profile, err := h.profiles.Copy(r.Context(), id, req.Name)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, http.StatusCreated, profile)
}

// SetDefault handles POST /api/v1/profiles/{id}/set-default
// @Summary Set default profile
// @Description Set a quality profile as the default for its language
// @Tags quality-profiles
// @Param id path int true "Profile ID"
// @Success 204
// @Router /api/v1/profiles/{id}/set-default [post]
func (h *ProfilesHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, profileInvalidIDMessage)
		return
	}
	if err := h.profiles.SetDefault(r.Context(), id); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfilesHandler) resolveProject(w http.ResponseWriter, r *http.Request) (*postgres.Project, bool) {
	key := routeParam(r, "key")
	project, err := h.projects.GetByKey(r.Context(), key)
	if errors.Is(err, postgres.ErrNotFound) {
		jsonError(w, http.StatusNotFound, "project not found")
		return nil, false
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	return project, true
}

func profileImportRules(doc profileCodeDocument, language string) ([]profileCodeRule, error) {
	if doc.Version != 0 && doc.Version != 1 {
		return nil, fmt.Errorf("unsupported profile schema version %d", doc.Version)
	}
	if doc.Language == language || (doc.Language == "" && len(doc.Profiles) == 0) {
		return nonEmptyProfileRules(doc.Rules)
	}
	for _, profile := range doc.Profiles {
		if profile.Language == language {
			return nonEmptyProfileRules(profile.Rules)
		}
	}
	return nil, fmt.Errorf("profile document does not contain language %q", language)
}

func decodeProfileCodeDocument(r *http.Request) (profileCodeDocument, error) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return profileCodeDocument{}, err
	}
	var doc profileCodeDocument
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "yaml") {
		return doc, yaml.Unmarshal(data, &doc)
	}
	if err := json.Unmarshal(data, &doc); err == nil {
		return doc, nil
	}
	return doc, yaml.Unmarshal(data, &doc)
}

func nonEmptyProfileRules(rules []profileCodeRule) ([]profileCodeRule, error) {
	if len(rules) == 0 {
		return nil, fmt.Errorf("profile document contains no rules")
	}
	for _, rule := range rules {
		if profileCodeRuleKey(rule) == "" {
			return nil, fmt.Errorf("profile rule key is required")
		}
	}
	return rules, nil
}

func profileCodeRuleKey(rule profileCodeRule) string {
	if rule.Key != "" {
		return rule.Key
	}
	if rule.RuleKey != "" {
		return rule.RuleKey
	}
	return rule.Rule
}

func profileCodeRuleActive(rule profileCodeRule) bool {
	active := true
	if rule.Active != nil {
		active = *rule.Active
	}
	if rule.Activate != nil {
		active = *rule.Activate
	}
	return active && !strings.EqualFold(rule.Severity, "off")
}
