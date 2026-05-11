// Package model defines the language-agnostic data model used across all Ollanta modules.
package model

// ParamDef describes a configurable parameter accepted by a Rule.
type ParamDef struct {
	Key          string `json:"key"`
	Description  string `json:"description"`
	DefaultValue string `json:"default_value"`
	// Type is one of "int", "float", "string", "bool".
	Type string `json:"type"`
}

// Threshold defines a metric-based condition for rules that fire when a measured value
// crosses a configurable limit. Inspired by the MetricHunter pattern from OSA.
type Threshold struct {
	MetricKey string `json:"metric_key"`
	// Operator is one of "gt", "lt", "eq", "gte", "lte".
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
	// Entity is the unit of measurement: "method", "class", "package", "file".
	Entity string `json:"entity"`
}

// Rule represents the static metadata of an analysis rule.
// When Language is "*", the rule applies to all languages.
type Rule struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Language is the canonical language ID ("go", "javascript", etc.) or "*" for
	// cross-language rules that apply regardless of the file's language.
	Language        string    `json:"language"`
	Type            IssueType `json:"type"`
	DefaultSeverity Severity  `json:"default_severity"`
	Tags            []string  `json:"tags,omitempty"`
	// ParamsSchema describes the configurable parameters accepted by this rule.
	ParamsSchema map[string]ParamDef `json:"params_schema,omitempty"`
	// Threshold is non-nil for metric-threshold rules (MetricHunter pattern).
	// Nil for pattern-matching rules.
	Threshold *Threshold `json:"threshold,omitempty"`
	// ReferenceURL is an optional link to official documentation, CWE, or standard.
	ReferenceURL string `json:"reference_url,omitempty"`
}
