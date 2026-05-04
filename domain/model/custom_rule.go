package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const CustomRulePackSchemaVersion = 1

type CustomRuleEngine string

const (
	CustomRuleEngineAuto       CustomRuleEngine = "auto"
	CustomRuleEngineText       CustomRuleEngine = "text"
	CustomRuleEngineGoAST      CustomRuleEngine = "go-ast"
	CustomRuleEngineTreeSitter CustomRuleEngine = "tree-sitter"
)

type CustomRuleLifecycle string

const (
	CustomRuleDraft      CustomRuleLifecycle = "draft"
	CustomRuleValid      CustomRuleLifecycle = "valid"
	CustomRulePublished  CustomRuleLifecycle = "published"
	CustomRuleDisabled   CustomRuleLifecycle = "disabled"
	CustomRuleDeprecated CustomRuleLifecycle = "deprecated"
	CustomRuleInvalid    CustomRuleLifecycle = "invalid"
)

type CustomRuleValidationStatus string

const (
	CustomRuleValidationNone            CustomRuleValidationStatus = "none"
	CustomRuleValidationPassed          CustomRuleValidationStatus = "passed"
	CustomRuleValidationFailed          CustomRuleValidationStatus = "failed"
	CustomRuleValidationRequiresRuntime CustomRuleValidationStatus = "requires_runtime"
)

type CustomRulePack struct {
	ID          int64     `json:"id,omitempty" yaml:"-"`
	Name        string    `json:"name" yaml:"name"`
	Namespace   string    `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	SourceHash  string    `json:"source_hash,omitempty" yaml:"source_hash,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty" yaml:"-"`
	UpdatedAt   time.Time `json:"updated_at,omitempty" yaml:"-"`
}

type CustomRulePackDocument struct {
	Version int                    `json:"version" yaml:"version"`
	Pack    CustomRulePack         `json:"pack" yaml:"pack"`
	Rules   []CustomRuleDefinition `json:"rules" yaml:"rules"`
}

type CustomRuleDefinition struct {
	ID                  int64                       `json:"id,omitempty" yaml:"-"`
	PackID              int64                       `json:"pack_id,omitempty" yaml:"-"`
	PackName            string                      `json:"pack_name,omitempty" yaml:"-"`
	RuleKey             string                      `json:"key" yaml:"key"`
	Version             int                         `json:"version,omitempty" yaml:"version,omitempty"`
	Name                string                      `json:"name" yaml:"name"`
	Description         string                      `json:"description,omitempty" yaml:"description,omitempty"`
	Language            string                      `json:"language" yaml:"language"`
	Type                IssueType                   `json:"type" yaml:"type"`
	DefaultSeverity     Severity                    `json:"severity" yaml:"severity"`
	Tags                []string                    `json:"tags,omitempty" yaml:"tags,omitempty"`
	ParamsSchema        map[string]ParamDef         `json:"params_schema,omitempty" yaml:"params_schema,omitempty"`
	Engine              CustomRuleEngine            `json:"engine" yaml:"engine"`
	EngineConfig        map[string]string           `json:"engine_config,omitempty" yaml:"engine_config,omitempty"`
	Message             string                      `json:"message,omitempty" yaml:"message,omitempty"`
	Examples            []CustomRuleExample         `json:"examples,omitempty" yaml:"examples,omitempty"`
	Limits              CustomRuleLimits            `json:"limits,omitempty" yaml:"limits,omitempty"`
	Lifecycle           CustomRuleLifecycle         `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	VersionHash         string                      `json:"version_hash,omitempty" yaml:"version_hash,omitempty"`
	ValidationStatus    CustomRuleValidationStatus  `json:"validation_status,omitempty" yaml:"validation_status,omitempty"`
	ValidationHash      string                      `json:"validation_hash,omitempty" yaml:"validation_hash,omitempty"`
	ValidationTimestamp time.Time                   `json:"validation_timestamp,omitempty" yaml:"-"`
	ValidationResult    *CustomRuleValidationResult `json:"validation_result,omitempty" yaml:"validation_result,omitempty"`
	PublishedAt         *time.Time                  `json:"published_at,omitempty" yaml:"-"`
	CreatedAt           time.Time                   `json:"created_at,omitempty" yaml:"-"`
	UpdatedAt           time.Time                   `json:"updated_at,omitempty" yaml:"-"`
}

type CustomRuleExample struct {
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
	Code       string `json:"code" yaml:"code"`
	Compliant  bool   `json:"compliant" yaml:"compliant"`
	Language   string `json:"language,omitempty" yaml:"language,omitempty"`
	WantLine   int    `json:"want_line,omitempty" yaml:"want_line,omitempty"`
	WantColumn int    `json:"want_column,omitempty" yaml:"want_column,omitempty"`
}

type CustomRuleLimits struct {
	MaxFileBytes int `json:"max_file_bytes,omitempty" yaml:"max_file_bytes,omitempty"`
	MaxMatches   int `json:"max_matches,omitempty" yaml:"max_matches,omitempty"`
	MaxFiles     int `json:"max_files,omitempty" yaml:"max_files,omitempty"`
	TimeoutMs    int `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
}

type CustomRuleEngineCapability struct {
	Engine                CustomRuleEngine `json:"engine"`
	Name                  string           `json:"name"`
	Languages             []string         `json:"languages"`
	RequiredFields        []string         `json:"required_fields"`
	SupportedPatterns     []string         `json:"supported_patterns,omitempty"`
	RequiresRuntime       bool             `json:"requires_runtime"`
	SupportsExampleTests  bool             `json:"supports_example_tests"`
	SupportsSourcePreview bool             `json:"supports_source_preview"`
	DefaultLimits         CustomRuleLimits `json:"default_limits"`
}

type CustomRuleDiagnostic struct {
	Level   string `json:"level" yaml:"level"`
	Code    string `json:"code" yaml:"code"`
	Field   string `json:"field,omitempty" yaml:"field,omitempty"`
	Message string `json:"message" yaml:"message"`
}

type CustomRuleValidationResult struct {
	Status                CustomRuleValidationStatus `json:"status" yaml:"status"`
	RuleKey               string                     `json:"rule_key,omitempty" yaml:"rule_key,omitempty"`
	VersionHash           string                     `json:"version_hash,omitempty" yaml:"version_hash,omitempty"`
	ValidatorCapabilities []string                   `json:"validator_capabilities,omitempty" yaml:"validator_capabilities,omitempty"`
	Diagnostics           []CustomRuleDiagnostic     `json:"diagnostics,omitempty" yaml:"diagnostics,omitempty"`
	CheckedAt             time.Time                  `json:"checked_at,omitempty" yaml:"-"`
}

type CustomRulePreviewResult struct {
	RuleKey      string                   `json:"rule_key"`
	FilesScanned int                      `json:"files_scanned"`
	MatchCount   int                      `json:"match_count"`
	Limited      bool                     `json:"limited,omitempty"`
	Matches      []CustomRulePreviewMatch `json:"matches,omitempty"`
	Diagnostics  []CustomRuleDiagnostic   `json:"diagnostics,omitempty"`
}

type CustomRulePreviewMatch struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message"`
	Snippet  string `json:"snippet,omitempty"`
}

type CustomRuleCatalogSnapshot struct {
	Source      string                 `json:"source,omitempty"`
	SnapshotID  string                 `json:"snapshot_id,omitempty"`
	Hash        string                 `json:"hash"`
	RuleCount   int                    `json:"rule_count"`
	PackIDs     []int64                `json:"pack_ids,omitempty"`
	Rules       []CustomRuleDefinition `json:"rules,omitempty"`
	Diagnostics []CustomRuleDiagnostic `json:"diagnostics,omitempty"`
	ResolvedAt  time.Time              `json:"resolved_at,omitempty"`
}

type CustomRuleAuditEntry struct {
	ID          int64               `json:"id"`
	PackID      int64               `json:"pack_id,omitempty"`
	RuleID      int64               `json:"rule_id,omitempty"`
	RuleKey     string              `json:"rule_key,omitempty"`
	VersionHash string              `json:"version_hash,omitempty"`
	Action      string              `json:"action"`
	OldState    CustomRuleLifecycle `json:"old_state,omitempty"`
	NewState    CustomRuleLifecycle `json:"new_state,omitempty"`
	ActorUserID int64               `json:"actor_user_id,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
}

func NormalizeCustomRuleKey(namespace, key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" || strings.Contains(key, ":") {
		return key
	}
	namespace = strings.TrimSpace(strings.ToLower(namespace))
	if namespace == "" {
		namespace = "custom"
	}
	return namespace + ":" + key
}

func NormalizeCustomRuleDefinition(rule CustomRuleDefinition) CustomRuleDefinition {
	rule.RuleKey = NormalizeCustomRuleKey("", rule.RuleKey)
	rule.Language = strings.TrimSpace(strings.ToLower(rule.Language))
	rule.Engine = CustomRuleEngine(strings.TrimSpace(strings.ToLower(string(rule.Engine))))
	if rule.Engine == "" {
		rule.Engine = CustomRuleEngineAuto
	}
	if rule.DefaultSeverity == "" {
		rule.DefaultSeverity = SeverityMajor
	}
	if rule.Type == "" {
		rule.Type = TypeCodeSmell
	}
	if rule.Lifecycle == "" {
		rule.Lifecycle = CustomRuleDraft
	}
	if rule.EngineConfig == nil {
		rule.EngineConfig = map[string]string{}
	}
	if rule.ParamsSchema == nil {
		rule.ParamsSchema = map[string]ParamDef{}
	}
	if rule.Tags == nil {
		rule.Tags = []string{}
	}
	sort.Strings(rule.Tags)
	sort.Slice(rule.Examples, func(i, j int) bool {
		if rule.Examples[i].Name == rule.Examples[j].Name {
			return rule.Examples[i].Code < rule.Examples[j].Code
		}
		return rule.Examples[i].Name < rule.Examples[j].Name
	})
	return rule
}

func HashCustomRuleDefinition(rule CustomRuleDefinition) string {
	rule = NormalizeCustomRuleDefinition(rule)
	rule.ID = 0
	rule.PackID = 0
	rule.PackName = ""
	rule.Version = 0
	rule.VersionHash = ""
	rule.ValidationStatus = ""
	rule.ValidationHash = ""
	rule.ValidationTimestamp = time.Time{}
	rule.ValidationResult = nil
	rule.PublishedAt = nil
	rule.Lifecycle = ""
	rule.CreatedAt = time.Time{}
	rule.UpdatedAt = time.Time{}
	data, err := json.Marshal(rule)
	if err != nil {
		panic("hash custom rule definition: " + err.Error())
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func HashCustomRuleCatalog(rules []CustomRuleDefinition) string {
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		if rule.VersionHash == "" {
			rule.VersionHash = HashCustomRuleDefinition(rule)
		}
		parts = append(parts, rule.RuleKey+"|"+rule.VersionHash+"|"+string(rule.Lifecycle))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func CanPublishCustomRule(rule CustomRuleDefinition) bool {
	return rule.ValidationStatus == CustomRuleValidationPassed && rule.ValidationHash != "" && rule.ValidationHash == HashCustomRuleDefinition(rule)
}
