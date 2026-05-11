// Package model defines the language-agnostic data model used across all Ollanta modules.
package model

// Measure holds the computed value of a single metric for a specific component.
type Measure struct {
	MetricKey     string  `json:"metric_key"`
	ComponentPath string  `json:"component_path"`
	Value         float64 `json:"value"`
	TextValue     string  `json:"text_value,omitempty"`
}
