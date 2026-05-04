package model

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// QualityProfile is the canonical quality profile record.
type QualityProfile struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Language   string    `json:"language"`
	ParentID   *int64    `json:"parent_id,omitempty"`
	IsDefault  bool      `json:"is_default"`
	IsBuiltin  bool      `json:"is_builtin"`
	RuleCount  int       `json:"rule_count"`
	HasRules   bool      `json:"has_rules"`
	ParserOnly bool      `json:"parser_only"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProfileRule associates a rule activation to a quality profile.
type ProfileRule struct {
	ID        int64             `json:"id"`
	ProfileID int64             `json:"profile_id"`
	RuleKey   string            `json:"rule_key"`
	Severity  string            `json:"severity"`
	Params    map[string]string `json:"params,omitempty"`
}

// EffectiveRule is the resolved rule configuration (after profile inheritance).
type EffectiveRule struct {
	RuleKey           string            `json:"rule_key"`
	Severity          string            `json:"severity"`
	Params            map[string]string `json:"params,omitempty"`
	RuleVersionHash   string            `json:"rule_version_hash,omitempty"`
	Origin            RuleOrigin        `json:"origin,omitempty"`
	Disabled          bool              `json:"disabled,omitempty"`
	SourceProfileID   int64             `json:"source_profile_id,omitempty"`
	SourceProfileName string            `json:"source_profile_name,omitempty"`
}

type RuleOrigin string

const (
	RuleOriginLocal      RuleOrigin = "local"
	RuleOriginInherited  RuleOrigin = "inherited"
	RuleOriginOverridden RuleOrigin = "overridden"
	RuleOriginDisabled   RuleOrigin = "disabled"
)

type ProfileSource string

const (
	ProfileSourceAssigned ProfileSource = "assigned"
	ProfileSourceDefault  ProfileSource = "default"
	ProfileSourceLocal    ProfileSource = "local"
	ProfileSourceRemote   ProfileSource = "remote"
	ProfileSourceBuiltin  ProfileSource = "builtin"
	ProfileSourceUnknown  ProfileSource = "unknown"
)

// ProjectQualityProfile describes the profile active for a project language.
type ProjectQualityProfile struct {
	Language string          `json:"language"`
	Profile  *QualityProfile `json:"profile,omitempty"`
	Source   ProfileSource   `json:"source"`
}

// EffectiveQualityProfile is the fully resolved policy for one project language.
type EffectiveQualityProfile struct {
	Language          string              `json:"language"`
	ProfileID         int64               `json:"profile_id,omitempty"`
	ProfileName       string              `json:"profile_name,omitempty"`
	Source            ProfileSource       `json:"source"`
	Rules             []*EffectiveRule    `json:"rules"`
	ActiveRuleCount   int                 `json:"active_rule_count"`
	RulesHash         string              `json:"rules_hash"`
	CustomCatalogHash string              `json:"custom_catalog_hash,omitempty"`
	HasRules          bool                `json:"has_rules"`
	ParserOnly        bool                `json:"parser_only"`
	Diagnostics       []ProfileDiagnostic `json:"diagnostics,omitempty"`
}

type ProfileDiagnostic struct {
	Level    string `json:"level"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Language string `json:"language,omitempty"`
}

// ProfileSnapshot records the policy snapshot attached to a scan report.
type ProfileSnapshot struct {
	Language          string              `json:"language"`
	ProfileID         int64               `json:"profile_id,omitempty"`
	ProfileName       string              `json:"profile_name,omitempty"`
	Source            ProfileSource       `json:"source"`
	ActiveRuleCount   int                 `json:"active_rule_count"`
	RulesHash         string              `json:"rules_hash"`
	CustomCatalogHash string              `json:"custom_catalog_hash,omitempty"`
	MetadataAvailable bool                `json:"metadata_available"`
	Diagnostics       []ProfileDiagnostic `json:"diagnostics,omitempty"`
}

// ProfileChangelogEntry records administrative changes to quality profiles.
type ProfileChangelogEntry struct {
	ID          int64     `json:"id"`
	ProfileID   int64     `json:"profile_id,omitempty"`
	ProjectID   int64     `json:"project_id,omitempty"`
	Language    string    `json:"language,omitempty"`
	Action      string    `json:"action"`
	RuleKey     string    `json:"rule_key,omitempty"`
	OldValue    string    `json:"old_value,omitempty"`
	NewValue    string    `json:"new_value,omitempty"`
	ActorUserID int64     `json:"actor_user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProfileHashChange summarizes a policy hash change between two scans.
type ProfileHashChange struct {
	Language     string `json:"language"`
	CurrentHash  string `json:"current_hash"`
	PreviousHash string `json:"previous_hash"`
}

// ProfileYAMLEntry is used for bulk-loading profile rules from YAML.
type ProfileYAMLEntry struct {
	RuleKey  string            `json:"rule_key" yaml:"rule_key"`
	Rule     string            `json:"rule,omitempty" yaml:"rule,omitempty"`
	Severity string            `json:"severity" yaml:"severity"`
	Params   map[string]string `json:"params,omitempty" yaml:"params,omitempty"`
	Activate bool              `json:"activate" yaml:"activate"`
}

// HashEffectiveRules returns a stable hash for normalized active rule configuration.
func HashEffectiveRules(rules []*EffectiveRule) string {
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		if rule == nil {
			continue
		}
		params := make([]string, 0, len(rule.Params))
		for key, value := range rule.Params {
			params = append(params, key+"="+value)
		}
		sort.Strings(params)
		disabled := "active"
		if rule.Disabled || strings.EqualFold(rule.Severity, "OFF") {
			disabled = "disabled"
		}
		parts = append(parts, rule.RuleKey+"|"+rule.Severity+"|"+disabled+"|"+rule.RuleVersionHash+"|"+strings.Join(params, ","))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}
