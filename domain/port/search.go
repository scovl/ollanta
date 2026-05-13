package port

import "context"

// ISearchEnqueuer is an optional outbound port for async search indexing.
// Implementations live in the outer layer chosen by the active runtime.
type ISearchEnqueuer interface {
	Enqueue(ctx context.Context, scanID, projectID int64, projectKey string)
}
