package ingest

import (
	"context"
	"fmt"
	"log/slog"
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
	switch s.Strategy {
	case StrategyAbort:
		return s.runAbort(ctx, fn)
	case StrategySkip:
		return s.runSkip(ctx, fn)
	case StrategyRetry:
		return s.runRetry(ctx, fn)
	}
	return nil
}

func (s Step) runAbort(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := s.runAttempt(ctx, fn); err != nil {
		return fmt.Errorf("step %s: %w", s.Name, err)
	}
	return nil
}

func (s Step) runSkip(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := s.runAttempt(ctx, fn); err != nil {
		slog.Warn("step skipped", "step", s.Name, "error", err)
	}
	return nil
}

func (s Step) runRetry(ctx context.Context, fn func(ctx context.Context) error) error {
	maxR := s.MaxRetry
	if maxR <= 0 {
		maxR = 3
	}
	var lastErr error
	for i := 1; i <= maxR; i++ {
		if err := s.runAttempt(ctx, fn); err != nil {
			lastErr = err
			backoff := time.Duration(i) * 500 * time.Millisecond
			slog.Warn("step attempt failed", "step", s.Name, "attempt", i, "max_attempts", maxR, "error", err, "retry_in", backoff)
			select {
			case <-ctx.Done():
				return fmt.Errorf("step %s: context cancelled: %w", s.Name, ctx.Err())
			case <-time.After(backoff):
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("step %s: all %d retries failed: %w", s.Name, maxR, lastErr)
}

func (s Step) runAttempt(ctx context.Context, fn func(ctx context.Context) error) error {
	stepCtx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	return fn(stepCtx)
}

// pipelineSteps defines the canonical step configuration for the ingest pipeline.
var pipelineSteps = struct {
	upsertProject   Step
	fetchPrevScan   Step
	trackIssues     Step
	evaluateGate    Step
	insertScan      Step
	bulkInsIssues   Step
	bulkInsMeasures Step
	indexSearch     Step
	fireWebhooks    Step
}{
	upsertProject:   Step{Name: "upsert_project", Timeout: 10 * time.Second, Strategy: StrategyAbort},
	fetchPrevScan:   Step{Name: "fetch_prev_scan", Timeout: 10 * time.Second, Strategy: StrategySkip},
	trackIssues:     Step{Name: "track_issues", Timeout: 5 * time.Second, Strategy: StrategyAbort},
	evaluateGate:    Step{Name: "evaluate_gate", Timeout: 10 * time.Second, Strategy: StrategySkip},
	insertScan:      Step{Name: "insert_scan", Timeout: 15 * time.Second, Strategy: StrategyAbort},
	bulkInsIssues:   Step{Name: "persist_issues", Timeout: 30 * time.Second, Strategy: StrategyAbort},
	bulkInsMeasures: Step{Name: "persist_measures", Timeout: 15 * time.Second, Strategy: StrategyAbort},
	indexSearch:     Step{Name: "index_search", Timeout: 10 * time.Second, Strategy: StrategySkip},
	fireWebhooks:    Step{Name: "fire_webhooks", Timeout: 2 * time.Second, Strategy: StrategySkip},
}
