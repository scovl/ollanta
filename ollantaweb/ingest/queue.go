package ingest

import (
	"errors"
	"sync/atomic"
)

// ErrQueueFull is returned when the ingest queue is at capacity.
var ErrQueueFull = errors.New("ingest queue full")

// IngestQueue is a bounded channel of IngestRequests with backpressure.
// When the queue is full, Enqueue returns ErrQueueFull (caller returns HTTP 429).
type IngestQueue struct {
	ch    chan *IngestRequest
	depth atomic.Int64
}

// NewIngestQueue creates a bounded queue with the given maximum depth.
func NewIngestQueue(maxDepth int) *IngestQueue {
	return &IngestQueue{
		ch: make(chan *IngestRequest, maxDepth),
	}
}

// Enqueue adds a request to the queue without blocking.
// Returns ErrQueueFull if the queue has no capacity.
func (q *IngestQueue) Enqueue(req *IngestRequest) error {
	select {
	case q.ch <- req:
		q.depth.Add(1)
		return nil
	default:
		return ErrQueueFull
	}
}

// Drain returns the channel for consumers to read from.
func (q *IngestQueue) Drain() <-chan *IngestRequest {
	return q.ch
}

// Depth returns the current number of items waiting in the queue.
func (q *IngestQueue) Depth() int64 {
	return q.depth.Load()
}

// Consumed decrements the internal depth counter. Must be called after each item is processed.
func (q *IngestQueue) Consumed() {
	q.depth.Add(-1)
}
