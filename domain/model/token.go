package model

import "time"

// Token is the canonical API token record.
type Token struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`
	TokenType  string     `json:"token_type"`
	ProjectID  *int64     `json:"project_id,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
