package ingest

import (
	"context"
	"errors"
	"log"
	"time"
)

// ScanJobWorker polls durable scan jobs and delegates processing to the compute processor.
type ScanJobWorker struct {
	processor *ScanJobProcessor
	idleDelay time.Duration
}

// NewScanJobWorker creates a background worker loop with the given idle delay.
func NewScanJobWorker(processor *ScanJobProcessor, idleDelay time.Duration) *ScanJobWorker {
	if idleDelay <= 0 {
		idleDelay = time.Second
	}
	return &ScanJobWorker{processor: processor, idleDelay: idleDelay}
}

// Start blocks until the context is cancelled.
func (w *ScanJobWorker) Start(ctx context.Context) {
	for {
		job, err := w.processor.ProcessNext(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("ollantaworker: process job: %v", err)
			}
			if !waitForNextTick(ctx, w.idleDelay) {
				return
			}
			continue
		}

		if job == nil {
			if !waitForNextTick(ctx, w.idleDelay) {
				return
			}
			continue
		}
	}
}

func waitForNextTick(ctx context.Context, delay time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(delay):
		return true
	}
}
