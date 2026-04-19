package model

import (
	"encoding/json"
	"time"
)

// IssueRow is the database representation of a single issue.
type IssueRow struct {
	ID                 int64           `json:"id"`
	ScanID             int64           `json:"scan_id"`
	ProjectID          int64           `json:"project_id"`
	RuleKey            string          `json:"rule_key"`
	ComponentPath      string          `json:"component_path"`
	Line               int             `json:"line"`
	Column             int             `json:"column"`
	EndLine            int             `json:"end_line"`
	EndColumn          int             `json:"end_column"`
	Message            string          `json:"message"`
	Type               string          `json:"type"`
	Severity           string          `json:"severity"`
	Status             string          `json:"status"`
	Resolution         string          `json:"resolution"`
	EffortMinutes      int             `json:"effort_minutes"`
	EngineID           string          `json:"engine_id"`
	LineHash           string          `json:"line_hash"`
	Tags               []string        `json:"tags"`
	SecondaryLocations json.RawMessage `json:"secondary_locations"`
	CreatedAt          time.Time       `json:"created_at"`
}

// IssueFilter specifies query parameters for listing issues.
type IssueFilter struct {
	ProjectID *int64
	ScanID    *int64
	RuleKey   *string
	Severity  *string
	Type      *string
	Status    *string
	FilePath  *string // applied as LIKE pattern against component_path
	EngineID  *string
	Limit     int // default 100, max 1000
	Offset    int
}

// IssueFacets holds aggregate distributions for the issues index.
type IssueFacets struct {
	BySeverity map[string]int `json:"by_severity"`
	ByType     map[string]int `json:"by_type"`
	ByRule     map[string]int `json:"by_rule"`
	ByStatus   map[string]int `json:"by_status"`
	ByEngineID map[string]int `json:"by_engine_id"`
	ByFile     map[string]int `json:"by_file"`
	ByTags     map[string]int `json:"by_tags"`
}
