package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/config"
)

func TestRunJobRecoveryUsesStaleCutoffAndRecordsMetrics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	repo := &fakeStaleJobRepository{result: postgres.JobRecoveryResult{Requeued: 2, Failed: 1}}
	reg := telemetry.NewRegistry()
	metrics := telemetry.NewMetrics(reg)

	runJobRecovery(context.Background(), "scan", repo, config.JobRecoveryConfig{
		StaleAfter:  15 * time.Minute,
		MaxAttempts: 3,
		Interval:    time.Minute,
	}, metrics, now)

	if !repo.staleBefore.Equal(now.Add(-15*time.Minute)) || repo.maxAttempts != 3 {
		t.Fatalf("recovery args = %s/%d, want cutoff and max attempts", repo.staleBefore, repo.maxAttempts)
	}
	body := readMetricsBody(t, reg)
	if !strings.Contains(body, "ollanta_scan_jobs_recovered_total 2") || !strings.Contains(body, "ollanta_scan_jobs_failed_by_recovery_total 1") {
		t.Fatalf("metrics missing scan recovery counters: %s", body)
	}
}

func TestJobRecoveryEnabledRequiresCompletePositiveConfig(t *testing.T) {
	t.Parallel()

	if !jobRecoveryEnabled(config.JobRecoveryConfig{StaleAfter: time.Minute, MaxAttempts: 1, Interval: time.Second}) {
		t.Fatal("jobRecoveryEnabled() = false, want true")
	}
	if jobRecoveryEnabled(config.JobRecoveryConfig{StaleAfter: 0, MaxAttempts: 1, Interval: time.Second}) {
		t.Fatal("jobRecoveryEnabled() = true with zero stale after, want false")
	}
	if jobRecoveryEnabled(config.JobRecoveryConfig{StaleAfter: time.Minute, MaxAttempts: 0, Interval: time.Second}) {
		t.Fatal("jobRecoveryEnabled() = true with zero max attempts, want false")
	}
	if jobRecoveryEnabled(config.JobRecoveryConfig{StaleAfter: time.Minute, MaxAttempts: 1, Interval: 0}) {
		t.Fatal("jobRecoveryEnabled() = true with zero interval, want false")
	}
}

func TestRunJobRecoveryDoesNotRecordMetricsOnError(t *testing.T) {
	t.Parallel()

	repo := &fakeStaleJobRepository{err: errors.New("database down")}
	reg := telemetry.NewRegistry()
	metrics := telemetry.NewMetrics(reg)

	runJobRecovery(context.Background(), "webhook", repo, config.JobRecoveryConfig{
		StaleAfter:  time.Minute,
		MaxAttempts: 3,
		Interval:    time.Minute,
	}, metrics, time.Now().UTC())

	body := readMetricsBody(t, reg)
	if strings.Contains(body, "ollanta_webhook_jobs_recovered_total 1") {
		t.Fatalf("unexpected recovery metric after error: %s", body)
	}
}

type fakeStaleJobRepository struct {
	result      postgres.JobRecoveryResult
	err         error
	staleBefore time.Time
	maxAttempts int
}

func (r *fakeStaleJobRepository) RecoverStale(_ context.Context, staleBefore time.Time, maxAttempts int, _ string) (postgres.JobRecoveryResult, error) {
	r.staleBefore = staleBefore
	r.maxAttempts = maxAttempts
	return r.result, r.err
}
