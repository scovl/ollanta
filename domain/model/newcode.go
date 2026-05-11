package model

import "time"

// NewCodePeriod defines the baseline strategy for new code analysis on a given scope.
type NewCodePeriod struct {
	ID        int64     `json:"id"`
	Scope     string    `json:"scope"` // "global", "project", "branch"
	ProjectID *int64    `json:"project_id,omitempty"`
	Branch    *string   `json:"branch,omitempty"`
	Strategy  string    `json:"strategy"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
