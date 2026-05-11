package model

import "time"

// User is the canonical user record.
type User struct {
	ID           int64      `json:"id"`
	Login        string     `json:"login"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	PasswordHash string     `json:"-"`
	AvatarURL    string     `json:"avatar_url"`
	Provider     string     `json:"provider"`
	ProviderID   string     `json:"provider_id"`
	IsActive     bool       `json:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// OAuthUser holds the identity returned by an OAuth provider during the callback.
type OAuthUser struct {
	ID        string `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}
