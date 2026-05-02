package postgres

import "time"

// JobListFilter contains bounded filters shared by durable job repositories.
type JobListFilter struct {
	Status        string
	ProjectKey    string
	ScanID        *int64
	WorkerID      string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Limit         int
	Offset        int
}

func boundedJobLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 500 {
		return 500
	}
	return limit
}
