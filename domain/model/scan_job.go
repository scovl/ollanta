package model

import "time"

// ScanJobStatus identifies the durable state of an asynchronous scan submission.
type ScanJobStatus string

const (
	ScanJobStatusAccepted  ScanJobStatus = "accepted"
	ScanJobStatusRunning   ScanJobStatus = "running"
	ScanJobStatusCompleted ScanJobStatus = "completed"
	ScanJobStatusFailed    ScanJobStatus = "failed"
)

// ScanJob stores a durably accepted scan payload until background processing finishes.
type ScanJob struct {
	ID             int64         `json:"id"`
	ProjectKey     string        `json:"project_key"`
	Status         ScanJobStatus `json:"status"`
	Payload        []byte        `json:"-"`
	IdempotencyKey string        `json:"-"`
	PayloadHash    string        `json:"-"`
	TraceParent    string        `json:"-"`
	TraceState     string        `json:"-"`
	ScanID         *int64        `json:"scan_id,omitempty"`
	WorkerID       string        `json:"worker_id,omitempty"`
	Attempts       int           `json:"attempts"`
	LastError      string        `json:"last_error,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	StartedAt      *time.Time    `json:"started_at,omitempty"`
	CompletedAt    *time.Time    `json:"completed_at,omitempty"`
}

// ScanQueuePressure summarizes durable scan queue state for intake backpressure.
type ScanQueuePressure struct {
	Accepted          int
	Running           int
	OldestAcceptedAge time.Duration
}
