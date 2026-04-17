package ingest

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ErrorStrategy determines how a step timeout/failure is handled.
type ErrorStrategy int

const (
	// StrategyAbort fails the entire ingest on step error.
	StrategyAbort ErrorStrategy = iota
	// StrategySkip logs a warning and continues the pipeline.
	StrategySkip
	// StrategyRetry retries the step up to maxRetries times with linear backoff.
	StrategyRetry
)

// Step describes a single pipeline step with an independent timeout and error strategy.
type Step struct {
	Name     string
	Timeout  time.Duration
	Strategy ErrorStrategy
	MaxRetry int // only used with StrategyRetry
}

// run executes fn within the step's timeout, applying the error strategy.
// Returns an error only when strategy is Abort.
func (s Step) run(ctx context.Context, fn func(ctx context.Context) error) error {
	attempt := func() error {
		stepCtx, cancel := context.WithTimeout(ctx, s.Timeout)
		defer cancel()
		return fn(stepCtx)
	}

	switch s.Strategy {
	case StrategyAbort:
		if err := attempt(); err != nil {
			return fmt.Errorf("step %s: %w", s.Name, err)
		}
		return nil

	case StrategySkip:
		if err := attempt(); err != nil {
			log.Printf("ingest: step %s skipped: %v", s.Name, err)
		}
		return nil

	case StrategyRetry:
		maxR := s.MaxRetry
		if maxR <= 0 {
			maxR = 3
		}
		var lastErr error
		for i := 1; i <= maxR; i++ {
			if err := attempt(); err != nil {
				lastErr = err
				backoff := time.Duration(i) * 500 * time.Millisecond
				log.Printf("ingest: step %s attempt %d/%d failed: %v (retry in %s)",
					s.Name, i, maxR, err, backoff)
				select {
				case <-ctx.Done():
					return fmt.Errorf("step %s: context cancelled: %w", s.Name, ctx.Err())
				case <-time.After(backoff):
				}
				continue
			}
			return nil
		}
		// After all retries, abort.
		return fmt.Errorf("step %s: all %d retries failed: %w", s.Name, maxR, lastErr)
	}
	return nil
}

// pipelineSteps defines the canonical step configuration for the ingest pipeline.
var pipelineSteps = struct {
	upsertProject  Step
	fetchPrevScan  Step
	trackIssues    Step
	evaluateGate   Step
	insertScan     Step
	bulkInsIssues  Step
	bulkInsMeasures Step
	indexSearch    Step
	fireWebhooks   Step
}{
	upsertProject:   Step{Name: "upsert_project",   Timeout: 10 * time.Second, Strategy: StrategyAbort},
	fetchPrevScan:   Step{Name: "fetch_prev_scan",   Timeout: 10 * time.Second, Strategy: StrategySkip},
	trackIssues:     Step{Name: "track_issues",      Timeout: 5 * time.Second,  Strategy: StrategyAbort},
	evaluateGate:    Step{Name: "evaluate_gate",     Timeout: 10 * time.Second, Strategy: StrategySkip},
	insertScan:      Step{Name: "insert_scan",       Timeout: 15 * time.Second, Strategy: StrategyAbort},
	bulkInsIssues:   Step{Name: "persist_issues",    Timeout: 30 * time.Second, Strategy: StrategyAbort},
	bulkInsMeasures: Step{Name: "persist_measures",  Timeout: 15 * time.Second, Strategy: StrategyAbort},
	indexSearch:     Step{Name: "index_search",      Timeout: 10 * time.Second, Strategy: StrategySkip},
	fireWebhooks:    Step{Name: "fire_webhooks",     Timeout: 2 * time.Second,  Strategy: StrategySkip},
}
