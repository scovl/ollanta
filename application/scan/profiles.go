package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/scovl/ollanta/domain/model"
	coredomain "github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantacore/rulecatalog"
	"gopkg.in/yaml.v3"
)

const (
	ProfileSourceAuto    = "auto"
	ProfileSourceLocal   = "local"
	ProfileSourceServer  = "server"
	ProfileSourceBuiltin = "builtin"
)

// ProfileOptions controls how the scanner loads and enforces quality profiles.
type ProfileOptions struct {
	Source       string
	FilePath     string
	Strict       bool
	FetchTimeout time.Duration
}

type CustomRuleOptions struct {
	CatalogHash string
	Rules       []model.CustomRuleDefinition
	Sources     []string
}

// ProfilePolicy is the effective rule policy used by the executor.
type ProfilePolicy struct {
	profiles          map[string]*model.EffectiveQualityProfile
	diagnostics       []model.ProfileDiagnostic
	customCatalogHash string
	allowAll          bool
}

// AllowAllProfilePolicy preserves legacy behavior for tests or callers that do not pass a policy.
func AllowAllProfilePolicy() *ProfilePolicy {
	return &ProfilePolicy{allowAll: true}
}

// ResolveProfilePolicy loads the effective profile policy using local, server, or built-in sources.
func ResolveProfilePolicy(ctx context.Context, opts *ScanOptions, languages []string) (*ProfilePolicy, error) {
	if opts == nil {
		return NewProfilePolicy(builtinEffectiveProfiles(languages), nil), nil
	}
	profileOpts := normalizeProfileOptions(opts.Profiles)
	if profileOpts.Source == ProfileSourceLocal || (profileOpts.Source == ProfileSourceAuto && profileOpts.FilePath != "") {
		return resolveLocalProfilePolicy(profileOpts, languages, opts.CustomRules)
	}
	if profileOpts.Source == ProfileSourceServer || (profileOpts.Source == ProfileSourceAuto && opts.Server != "") {
		return resolveServerProfilePolicy(ctx, opts, profileOpts, languages)
	}
	return newProfilePolicyWithCustomCatalog(builtinEffectiveProfiles(languages), nil, opts.CustomRules.CatalogHash), nil
}

// NewProfilePolicy normalizes effective profiles for executor lookup and reporting.
func NewProfilePolicy(profiles []*model.EffectiveQualityProfile, diagnostics []model.ProfileDiagnostic) *ProfilePolicy {
	return newProfilePolicyWithCustomCatalog(profiles, diagnostics, "")
}

func newProfilePolicyWithCustomCatalog(profiles []*model.EffectiveQualityProfile, diagnostics []model.ProfileDiagnostic, customCatalogHash string) *ProfilePolicy {
	policy := &ProfilePolicy{profiles: map[string]*model.EffectiveQualityProfile{}, diagnostics: diagnostics}
	policy.customCatalogHash = customCatalogHash
	for _, profile := range profiles {
		if profile == nil || profile.Language == "" {
			continue
		}
		if policy.customCatalogHash == "" && profile.CustomCatalogHash != "" {
			policy.customCatalogHash = profile.CustomCatalogHash
		}
		normalizeEffectiveProfile(profile)
		policy.profiles[profile.Language] = profile
	}
	return policy
}

// Rule returns the active effective configuration for a rule key in a language.
func (p *ProfilePolicy) Rule(language, ruleKey string) (*model.EffectiveRule, bool) {
	if p == nil || p.allowAll {
		return &model.EffectiveRule{RuleKey: ruleKey, Params: map[string]string{}}, true
	}
	profile := p.profiles[language]
	if profile == nil {
		return nil, false
	}
	for _, rule := range profile.Rules {
		if rule.RuleKey == ruleKey {
			if rule.Disabled || strings.EqualFold(rule.Severity, "OFF") {
				return nil, false
			}
			return rule, true
		}
	}
	return nil, false
}

// Snapshots returns report-safe profile snapshots sorted by language.
func (p *ProfilePolicy) Snapshots() []model.ProfileSnapshot {
	if p == nil || p.allowAll {
		return nil
	}
	languages := make([]string, 0, len(p.profiles))
	for language := range p.profiles {
		languages = append(languages, language)
	}
	sort.Strings(languages)
	out := make([]model.ProfileSnapshot, 0, len(languages))
	for _, language := range languages {
		profile := p.profiles[language]
		out = append(out, model.ProfileSnapshot{
			Language:          profile.Language,
			ProfileID:         profile.ProfileID,
			ProfileName:       profile.ProfileName,
			Source:            profile.Source,
			ActiveRuleCount:   profile.ActiveRuleCount,
			RulesHash:         profile.RulesHash,
			CustomCatalogHash: p.customCatalogHash,
			MetadataAvailable: true,
			Diagnostics:       append([]model.ProfileDiagnostic(nil), profile.Diagnostics...),
		})
	}
	return out
}

// Diagnostics returns profile resolution diagnostics sorted by language and code.
func (p *ProfilePolicy) Diagnostics() []model.ProfileDiagnostic {
	if p == nil {
		return nil
	}
	out := append([]model.ProfileDiagnostic(nil), p.diagnostics...)
	for _, profile := range p.profiles {
		out = append(out, profile.Diagnostics...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Language == out[j].Language {
			return out[i].Code < out[j].Code
		}
		return out[i].Language < out[j].Language
	})
	return out
}

func normalizeProfileOptions(opts ProfileOptions) ProfileOptions {
	if opts.Source == "" {
		opts.Source = ProfileSourceAuto
	}
	opts.Source = strings.ToLower(opts.Source)
	if opts.FetchTimeout <= 0 {
		opts.FetchTimeout = 10 * time.Second
	}
	return opts
}

func resolveLocalProfilePolicy(opts ProfileOptions, languages []string, customRules CustomRuleOptions) (*ProfilePolicy, error) {
	profiles, err := loadLocalProfileDocument(opts.FilePath, languages, customRules)
	if err == nil {
		return newProfilePolicyWithCustomCatalog(mergeMissingBuiltinProfiles(profiles, languages), nil, customRules.CatalogHash), nil
	}
	if opts.Strict {
		return nil, err
	}
	diagnostic := model.ProfileDiagnostic{Level: "warning", Code: "local_profile_load_failed", Message: err.Error()}
	return newProfilePolicyWithCustomCatalog(builtinEffectiveProfiles(languages), []model.ProfileDiagnostic{diagnostic}, customRules.CatalogHash), nil
}

func resolveServerProfilePolicy(ctx context.Context, opts *ScanOptions, profileOpts ProfileOptions, languages []string) (*ProfilePolicy, error) {
	profiles, err := fetchServerEffectiveProfiles(ctx, opts.Server, opts.ServerToken, opts.ProjectKey, profileOpts.FetchTimeout)
	if err == nil {
		return newProfilePolicyWithCustomCatalog(mergeMissingBuiltinProfiles(profiles, languages), nil, opts.CustomRules.CatalogHash), nil
	}
	if profileOpts.Strict {
		return nil, err
	}
	diagnostic := model.ProfileDiagnostic{Level: "warning", Code: "server_profile_load_failed", Message: err.Error()}
	return newProfilePolicyWithCustomCatalog(builtinEffectiveProfiles(languages), []model.ProfileDiagnostic{diagnostic}, opts.CustomRules.CatalogHash), nil
}

func fetchServerEffectiveProfiles(ctx context.Context, serverURL, token, projectKey string, timeout time.Duration) ([]*model.EffectiveQualityProfile, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("server URL is required for profile-source=server")
	}
	if projectKey == "" {
		return nil, fmt.Errorf("project key is required for server profile loading")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint := strings.TrimRight(serverURL, "/") + "/api/v1/projects/" + url.PathEscape(projectKey) + "/profiles/effective"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build profile request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch effective profiles: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch effective profiles returned %d", resp.StatusCode)
	}
	var profiles []*model.EffectiveQualityProfile
	if err := json.NewDecoder(resp.Body).Decode(&profiles); err != nil {
		return nil, fmt.Errorf("decode effective profiles: %w", err)
	}
	return profiles, nil
}

type profileDocument struct {
	Version  int                       `json:"version" yaml:"version"`
	Language string                    `json:"language" yaml:"language"`
	Name     string                    `json:"name" yaml:"name"`
	Rules    []profileDocumentRule     `json:"rules" yaml:"rules"`
	Profiles []profileDocumentLanguage `json:"profiles" yaml:"profiles"`
}

type profileDocumentLanguage struct {
	Language string                `json:"language" yaml:"language"`
	Name     string                `json:"name" yaml:"name"`
	Rules    []profileDocumentRule `json:"rules" yaml:"rules"`
}

type profileDocumentRule struct {
	Key      string            `json:"key" yaml:"key"`
	RuleKey  string            `json:"rule_key" yaml:"rule_key"`
	Rule     string            `json:"rule" yaml:"rule"`
	Severity string            `json:"severity" yaml:"severity"`
	Params   map[string]string `json:"params" yaml:"params"`
	Active   *bool             `json:"active" yaml:"active"`
	Activate *bool             `json:"activate" yaml:"activate"`
}

func loadLocalProfileDocument(path string, languages []string, customRules CustomRuleOptions) ([]*model.EffectiveQualityProfile, error) {
	if path == "" {
		return nil, fmt.Errorf("profile file is required for profile-source=local")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile file: %w", err)
	}
	var doc profileDocument
	if err := decodeProfileDocument(data, &doc); err != nil {
		return nil, fmt.Errorf("parse profile file: %w", err)
	}
	return documentEffectiveProfiles(doc, languages, customRulesByKey(customRules.Rules))
}

func decodeProfileDocument(data []byte, doc *profileDocument) error {
	if err := json.Unmarshal(data, doc); err == nil {
		return nil
	}
	return yaml.Unmarshal(data, doc)
}

func documentEffectiveProfiles(doc profileDocument, languages []string, customCatalog map[string]model.CustomRuleDefinition) ([]*model.EffectiveQualityProfile, error) {
	if doc.Version != 0 && doc.Version != 1 {
		return nil, fmt.Errorf("unsupported profile schema version %d", doc.Version)
	}
	profiles := doc.Profiles
	if len(profiles) == 0 && doc.Language != "" {
		profiles = []profileDocumentLanguage{{Language: doc.Language, Name: doc.Name, Rules: doc.Rules}}
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("profile file must define at least one profile")
	}
	out := make([]*model.EffectiveQualityProfile, 0, len(profiles))
	for _, profile := range profiles {
		effective, err := documentLanguageProfile(profile, customCatalog)
		if err != nil {
			return nil, err
		}
		out = append(out, effective)
	}
	return filterProfilesByLanguage(out, languages), nil
}

func documentLanguageProfile(profile profileDocumentLanguage, customCatalog map[string]model.CustomRuleDefinition) (*model.EffectiveQualityProfile, error) {
	language, ok := rulecatalog.LanguageByKey(profile.Language)
	if !ok {
		return nil, fmt.Errorf("unsupported profile language %q", profile.Language)
	}
	rules := make([]*model.EffectiveRule, 0, len(profile.Rules))
	for _, entry := range profile.Rules {
		rule, active, err := documentRule(profile.Language, entry, customCatalog)
		if err != nil {
			return nil, err
		}
		if active {
			rules = append(rules, rule)
		}
	}
	name := profile.Name
	if name == "" {
		name = "Local Profile"
	}
	effective := &model.EffectiveQualityProfile{
		Language:    profile.Language,
		ProfileName: name,
		Source:      model.ProfileSourceLocal,
		Rules:       rules,
		HasRules:    language.HasRules,
		ParserOnly:  language.ParserOnly,
	}
	if language.ParserOnly {
		effective.Diagnostics = append(effective.Diagnostics, model.ProfileDiagnostic{Level: "info", Code: "parser_only_language", Message: "language is parsed but has no bundled rules", Language: profile.Language})
	}
	return effective, nil
}

func documentRule(language string, entry profileDocumentRule, customCatalog map[string]model.CustomRuleDefinition) (*model.EffectiveRule, bool, error) {
	ruleKey := firstProfileField(entry.Key, entry.RuleKey, entry.Rule)
	rule, ok := rulecatalog.ByKey(ruleKey)
	ruleVersionHash := ""
	if !ok {
		customRule, found := customCatalog[ruleKey]
		if !found {
			return nil, false, fmt.Errorf("unknown rule %q", ruleKey)
		}
		rule = customRuleCatalogRule(customRule)
		ruleVersionHash = customRule.VersionHash
	}
	if rule.Language != language && rule.Language != "*" {
		return nil, false, fmt.Errorf("rule %q belongs to language %q, not %q", ruleKey, rule.Language, language)
	}
	severity := entry.Severity
	if severity == "" {
		severity = string(rule.DefaultSeverity)
	}
	active := true
	if entry.Active != nil {
		active = *entry.Active
	}
	if entry.Activate != nil {
		active = *entry.Activate
	}
	if strings.EqualFold(severity, "off") {
		active = false
	}
	if !validProfileSeverity(severity) {
		return nil, false, fmt.Errorf("invalid severity %q for rule %q", severity, ruleKey)
	}
	params, err := effectiveRuleParams(rule, entry.Params)
	if err != nil {
		return nil, false, err
	}
	return &model.EffectiveRule{RuleKey: ruleKey, Severity: severity, Params: params, RuleVersionHash: ruleVersionHash, Origin: model.RuleOriginLocal}, active, nil
}

func customRulesByKey(rules []model.CustomRuleDefinition) map[string]model.CustomRuleDefinition {
	out := make(map[string]model.CustomRuleDefinition, len(rules))
	for _, rule := range rules {
		if rule.RuleKey != "" {
			out[rule.RuleKey] = rule
		}
	}
	return out
}

func customRuleCatalogRule(rule model.CustomRuleDefinition) *coredomain.Rule {
	params := make(map[string]coredomain.ParamDef, len(rule.ParamsSchema))
	for key, param := range rule.ParamsSchema {
		params[key] = coredomain.ParamDef(param)
	}
	return &coredomain.Rule{
		Key:             rule.RuleKey,
		Name:            rule.Name,
		Description:     rule.Description,
		Language:        rule.Language,
		Type:            coredomain.IssueType(rule.Type),
		DefaultSeverity: coredomain.Severity(rule.DefaultSeverity),
		Tags:            append([]string(nil), rule.Tags...),
		ParamsSchema:    params,
	}
}

func effectiveRuleParams(rule *coredomain.Rule, overrides map[string]string) (map[string]string, error) {
	params := rulecatalog.DefaultParams(rule)
	for key, value := range overrides {
		param, ok := rule.ParamsSchema[key]
		if !ok {
			return nil, fmt.Errorf("unknown parameter %q for rule %q", key, rule.Key)
		}
		if err := validateProfileParamValue(param.Type, value); err != nil {
			return nil, fmt.Errorf("invalid parameter %q for rule %q: %w", key, rule.Key, err)
		}
		params[key] = value
	}
	return params, nil
}

func builtinEffectiveProfiles(languages []string) []*model.EffectiveQualityProfile {
	targets := targetLanguages(languages)
	out := make([]*model.EffectiveQualityProfile, 0, len(targets))
	for _, language := range targets {
		languageMeta, ok := rulecatalog.LanguageByKey(language)
		if !ok {
			continue
		}
		rules := make([]*model.EffectiveRule, 0, len(rulecatalog.ByLanguage(language)))
		for _, rule := range rulecatalog.ByLanguage(language) {
			rules = append(rules, &model.EffectiveRule{RuleKey: rule.Key, Severity: string(rule.DefaultSeverity), Params: rulecatalog.DefaultParams(rule), Origin: model.RuleOriginLocal})
		}
		profile := &model.EffectiveQualityProfile{
			Language:    language,
			ProfileName: "Ollanta Way",
			Source:      model.ProfileSourceBuiltin,
			Rules:       rules,
			HasRules:    languageMeta.HasRules,
			ParserOnly:  languageMeta.ParserOnly,
		}
		if languageMeta.ParserOnly {
			profile.Diagnostics = append(profile.Diagnostics, model.ProfileDiagnostic{Level: "info", Code: "parser_only_language", Message: "language is parsed but has no bundled rules", Language: language})
		}
		out = append(out, profile)
	}
	return out
}

func mergeMissingBuiltinProfiles(profiles []*model.EffectiveQualityProfile, languages []string) []*model.EffectiveQualityProfile {
	seen := map[string]bool{}
	for _, profile := range profiles {
		if profile != nil {
			seen[profile.Language] = true
		}
	}
	out := append([]*model.EffectiveQualityProfile(nil), profiles...)
	for _, profile := range builtinEffectiveProfiles(languages) {
		if !seen[profile.Language] {
			out = append(out, profile)
		}
	}
	return out
}

func normalizeEffectiveProfile(profile *model.EffectiveQualityProfile) {
	if profile.Rules == nil {
		profile.Rules = []*model.EffectiveRule{}
	}
	sort.Slice(profile.Rules, func(i, j int) bool { return profile.Rules[i].RuleKey < profile.Rules[j].RuleKey })
	profile.ActiveRuleCount = profileActiveRuleCount(profile.Rules)
	profile.RulesHash = model.HashEffectiveRules(profile.Rules)
	if language, ok := rulecatalog.LanguageByKey(profile.Language); ok {
		profile.HasRules = language.HasRules
		profile.ParserOnly = language.ParserOnly
	}
}

func profileActiveRuleCount(rules []*model.EffectiveRule) int {
	count := 0
	for _, rule := range rules {
		if rule != nil && !rule.Disabled && !strings.EqualFold(rule.Severity, "OFF") {
			count++
		}
	}
	return count
}

func targetLanguages(languages []string) []string {
	if len(languages) == 0 {
		all := rulecatalog.SupportedLanguages()
		out := make([]string, 0, len(all))
		for _, language := range all {
			out = append(out, language.Key)
		}
		return out
	}
	seen := map[string]bool{}
	for _, language := range languages {
		if language != "" {
			seen[language] = true
		}
	}
	out := make([]string, 0, len(seen))
	for language := range seen {
		out = append(out, language)
	}
	sort.Strings(out)
	return out
}

func filterProfilesByLanguage(profiles []*model.EffectiveQualityProfile, languages []string) []*model.EffectiveQualityProfile {
	if len(languages) == 0 {
		return profiles
	}
	wanted := map[string]bool{}
	for _, language := range languages {
		wanted[language] = true
	}
	out := make([]*model.EffectiveQualityProfile, 0, len(profiles))
	for _, profile := range profiles {
		if profile != nil && wanted[profile.Language] {
			out = append(out, profile)
		}
	}
	return out
}

func firstProfileField(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func validProfileSeverity(severity string) bool {
	switch strings.ToLower(severity) {
	case "blocker", "critical", "major", "minor", "info", "off":
		return true
	default:
		return false
	}
}

func validateProfileParamValue(paramType, value string) error {
	switch paramType {
	case "int":
		_, err := strconv.Atoi(value)
		return err
	case "float":
		_, err := strconv.ParseFloat(value, 64)
		return err
	case "bool":
		_, err := strconv.ParseBool(value)
		return err
	case "string", "":
		return nil
	default:
		return fmt.Errorf("unsupported parameter type %q", paramType)
	}
}
