// Package model defines the language-agnostic data model used across all Ollanta modules.
package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Severity represents the impact level of an Issue.
type Severity string

const (
	// SeverityBlocker is the highest severity: must be fixed immediately.
	SeverityBlocker Severity = "blocker"
	// SeverityCritical indicates a serious defect likely to cause problems.
	SeverityCritical Severity = "critical"
	// SeverityMajor indicates a significant quality issue.
	SeverityMajor Severity = "major"
	// SeverityMinor indicates a minor quality issue.
	SeverityMinor Severity = "minor"
	// SeverityInfo is an informational finding only.
	SeverityInfo Severity = "info"
)

// IssueType classifies an Issue into one of the four quality model categories.
type IssueType string

const (
	// TypeBug represents a code defect that may cause incorrect behaviour.
	TypeBug IssueType = "bug"
	// TypeVulnerability represents a security weakness in the code.
	TypeVulnerability IssueType = "vulnerability"
	// TypeCodeSmell represents a maintainability issue.
	TypeCodeSmell IssueType = "code_smell"
	// TypeSecurityHotspot represents code that requires a manual security review.
	TypeSecurityHotspot IssueType = "security_hotspot"
)

// Status represents the lifecycle state of an Issue across successive scans.
type Status string

const (
	// StatusOpen means the issue was detected in the current scan.
	StatusOpen Status = "open"
	// StatusConfirmed means the issue has been reviewed and acknowledged.
	StatusConfirmed Status = "confirmed"
	// StatusClosed means the issue is no longer detected.
	StatusClosed Status = "closed"
	// StatusReopened means a previously closed issue has reappeared.
	StatusReopened Status = "reopened"
)

// SecondaryLocation represents an auxiliary source location that contextualises an issue.
type SecondaryLocation struct {
	FilePath    string `json:"file_path,omitempty"`
	Message     string `json:"message,omitempty"`
	StartLine   int    `json:"start_line,omitempty"`
	StartColumn int    `json:"start_column,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
	EndColumn   int    `json:"end_column,omitempty"`
}

// Issue represents a single quality finding produced by a rule during analysis.
// It is language-agnostic: the same struct is used for Go, JavaScript, Python, and any
// other language supported by Ollanta.
type Issue struct {
	RuleKey       string    `json:"rule_key"`
	ComponentPath string    `json:"component_path"`
	Line          int       `json:"line"`
	Column        int       `json:"column"`
	EndLine       int       `json:"end_line"`
	EndColumn     int       `json:"end_column"`
	Message       string    `json:"message"`
	Type          IssueType `json:"type"`
	Severity      Severity  `json:"severity"`
	Status        Status    `json:"status"`
	Resolution    string    `json:"resolution,omitempty"`
	EffortMinutes int       `json:"effort_minutes,omitempty"`
	EngineID      string    `json:"engine_id,omitempty"`
	// LineHash is the SHA-256 hex digest of the trimmed source line. Used by the
	// issue tracker to match the same logical issue across scans
	// even when its line number shifts due to edits elsewhere in the file.
	LineHash           string              `json:"line_hash,omitempty"`
	Tags               []string            `json:"tags,omitempty"`
	SecondaryLocations []SecondaryLocation `json:"secondary_locations"`
}

// NewIssue creates an Issue with the required identifying fields and sane defaults.
// Callers set additional fields (Message, Severity, Type, etc.) after construction.
func NewIssue(ruleKey, componentPath string, line int) *Issue {
	return &Issue{
		RuleKey:            ruleKey,
		ComponentPath:      componentPath,
		Line:               line,
		Status:             StatusOpen,
		EngineID:           "ollanta",
		SecondaryLocations: []SecondaryLocation{},
	}
}

// ComputeLineHash returns the SHA-256 hex digest of the trimmed source line at the given
// 1-based line number. Returns an empty string when the line is out of range or blank.
// Used to set Issue.LineHash for stable identity across scans.
func ComputeLineHash(fileContent string, line int) string {
	if fileContent == "" || line <= 0 {
		return ""
	}
	lines := strings.Split(fileContent, "\n")
	if line > len(lines) {
		return ""
	}
	trimmed := strings.TrimSpace(lines[line-1])
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}
