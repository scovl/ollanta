package ingest_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantaweb/ingest"
)

func TestIngestQueueEnqueueAndDepth(t *testing.T) {
	t.Parallel()
	q := ingest.NewIngestQueue(3)
	for i := 0; i < 3; i++ {
		if err := q.Enqueue(&ingest.IngestRequest{}); err != nil {
			t.Fatalf("enqueue %d failed: %v", i, err)
		}
	}
	if q.Depth() != 3 {
		t.Errorf("expected depth 3, got %d", q.Depth())
	}
}

func TestIngestQueueFullReturnsError(t *testing.T) {
	t.Parallel()
	q := ingest.NewIngestQueue(1)
	if err := q.Enqueue(&ingest.IngestRequest{}); err != nil {
		t.Fatalf("first enqueue failed: %v", err)
	}
	err := q.Enqueue(&ingest.IngestRequest{})
	if err == nil {
		t.Fatal("expected error when queue full")
	}
}

func TestIngestQueueDrain(t *testing.T) {
	t.Parallel()
	q := ingest.NewIngestQueue(10)
	_ = q.Enqueue(&ingest.IngestRequest{})
	_ = q.Enqueue(&ingest.IngestRequest{})

	ch := q.Drain()
	var drained int
	for i := 0; i < 2; i++ {
		<-ch
		q.Consumed()
		drained++
	}
	if drained != 2 {
		t.Errorf("expected 2 drained items, got %d", drained)
	}
	if q.Depth() != 0 {
		t.Errorf("expected depth 0 after drain, got %d", q.Depth())
	}
}
