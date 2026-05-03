package ingest

import (
	"context"
	"log/slog"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/config"
)

type staleJobRepository interface {
	RecoverStale(ctx context.Context, staleBefore time.Time, maxAttempts int, failureMessage string) (postgres.JobRecoveryResult, error)
}

// StartJobRecoveryLoop periodically recovers stale durable jobs for a worker role.
func StartJobRecoveryLoop(ctx context.Context, jobType string, repo staleJobRepository, cfg config.JobRecoveryConfig, metrics *telemetry.Metrics) {
	if !jobRecoveryEnabled(cfg) {
		slog.InfoContext(ctx, "stale job recovery disabled", "job_type", jobType)
		return
	}

	go func() {
		runJobRecovery(ctx, jobType, repo, cfg, metrics, time.Now().UTC())
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				runJobRecovery(ctx, jobType, repo, cfg, metrics, now.UTC())
			}
		}
	}()
}

func runJobRecovery(ctx context.Context, jobType string, repo staleJobRepository, cfg config.JobRecoveryConfig, metrics *telemetry.Metrics, now time.Time) {
	result, err := repo.RecoverStale(ctx, now.Add(-cfg.StaleAfter), cfg.MaxAttempts, "job exceeded stale recovery attempts")
	if err != nil {
		slog.ErrorContext(ctx, "recover stale jobs", "job_type", jobType, "error", err)
		return
	}
	if metrics != nil {
		metrics.ObserveJobRecovery(jobType, result.Requeued, result.Failed)
	}
	if result.Requeued > 0 || result.Failed > 0 {
		slog.InfoContext(ctx, "recovered stale jobs", "job_type", jobType, "requeued", result.Requeued, "failed", result.Failed)
	}
}

func jobRecoveryEnabled(cfg config.JobRecoveryConfig) bool {
	return cfg.StaleAfter > 0 && cfg.Interval > 0 && cfg.MaxAttempts > 0
}
