package model

import (
	"errors"
	"time"
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("not found")

// Project is the canonical project record.
type Project struct {
	ID          int64     `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MainBranch  string    `json:"main_branch"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
