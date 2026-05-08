// Package main provides the ollantaworker process, which owns durable background
// processing: scan-job ingestion, webhook delivery, data lifecycle cleanup,
// heartbeat maintenance, and operational observability.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/config"
	"github.com/scovl/ollanta/ollantaweb/ingest"
	appruntime "github.com/scovl/ollanta/ollantaweb/internal/runtime"
	"github.com/scovl/ollanta/ollantaweb/webhook"
)

type workerContext struct {
	cfg         config.Config
	db          *postgres.DB
	workerID    string
	metrics     *telemetry.Metrics
	processor   *ingest.ScanJobProcessor
	scanJobRepo *postgres.ScanJobRepository
	webhook     *webhook.Dispatcher
}

func main() {
	wc, err := setupWorker()
	if err != nil {
		slog.Error("setup worker", "error", err)
		os.Exit(1)
	}
	defer wc.db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startWorkerPool(ctx, wc)
	startHeartbeat(ctx, wc)
	startDataCleanup(ctx, wc)
	slog.Info("started", "worker_id", wc.workerID, "worker_pool", workerPoolSize(wc.cfg), "admin_addr", wc.cfg.AdminAddr)

	<-ctx.Done()
	wc.webhook.Stop()
	slog.Info("stopped")
}

func setupWorker() (*workerContext, error) {
	cfg := config.MustLoad()
	slog.SetDefault(telemetry.SetupLogger(cfg.LogLevel, "service", "ollantaworker", "role", "worker"))
	cfg.LogStartupWarnings()

	ctx := context.Background()
	shutdownTracing, err := telemetry.SetupTracing(ctx, "ollantaworker")
	if err != nil {
		return nil, fmt.Errorf("setup tracing: %w", err)
	}
	defer func() {
		if shutdownErr := shutdownTracing(context.Background()); shutdownErr != nil {
			slog.Warn("shutdown tracing", "error", shutdownErr)
		}
	}()

	db, err := postgres.New(ctx, cfg.DatabaseURL, cfg.PostgresPool)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := appruntime.PrepareDatabase(ctx, db, cfg.AutoMigrate); err != nil {
		db.Close()
		return nil, fmt.Errorf("prepare database (auto_migrate=%v): %w", cfg.AutoMigrate, err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ollantaworker"
	}
	workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	metricsReg := telemetry.NewRegistry()
	appMetrics := telemetry.NewMetrics(metricsReg)
	appruntime.StartDatabaseMetricsLoop(ctx, db, metricsReg, cfg.DatabaseMetricsInterval)
	telemetry.StartAdminServer(ctx, cfg.AdminAddr, metricsReg, appruntime.ReadyCheck(
		appruntime.NamedHealthCheck{Name: "postgres", Check: db},
	))

	repos := initRepositories(db)
	processor, webhookDispatcher := wireServices(db, workerID, appMetrics, *cfg, repos)

	return &workerContext{
		cfg:         *cfg,
		db:          db,
		workerID:    workerID,
		metrics:     appMetrics,
		processor:   processor,
		scanJobRepo: repos.ScanJobs,
		webhook:     webhookDispatcher,
	}, nil
}

func wireServices(db *postgres.DB, workerID string, appMetrics *telemetry.Metrics, cfg config.Config, repos workerRepositories) (*ingest.ScanJobProcessor, *webhook.Dispatcher) {
	indexEnqueuer := ingest.NewIndexJobEnqueuer(repos.IndexJobs)
	webhookDispatcher := webhook.NewDispatcher(repos.Webhooks, repos.WebhookJobs, workerID, appMetrics, cfg.WebhookPollDelay, cfg.WebhookClientTimeout, cfg.WebhookRetryDelays)
	ingest.StartJobRecoveryLoop(context.Background(), "scan", repos.ScanJobs, cfg.ScanJobRecovery, appMetrics)

	processor := ingest.NewScanJobProcessor(
		workerID,
		repos.ScanJobs,
		ingest.IngestRepositories{
			Projects:  repos.Projects,
			Scans:     repos.Scans,
			Issues:    repos.Issues,
			Measures:  repos.Measures,
			Snapshots: repos.Snapshots,
			Profiles:  repos.Profiles,
			Tags:      repos.Tags,
			Gates:     repos.Gates,
		},
		indexEnqueuer,
		webhookDispatcher,
	)
	return processor, webhookDispatcher
}

type workerRepositories struct {
	Projects  *postgres.ProjectRepository
	Scans     *postgres.ScanRepository
	ScanJobs  *postgres.ScanJobRepository
	IndexJobs *postgres.IndexJobRepository
	Issues    *postgres.IssueRepository
	Measures  *postgres.MeasureRepository
	Snapshots *postgres.CodeSnapshotRepository
	Profiles  *postgres.ProfileSnapshotRepository
	Tags      *postgres.TagRepository
	Gates     *postgres.GateRepository
	Webhooks  *postgres.WebhookRepository
	WebhookJobs *postgres.WebhookJobRepository
}

func initRepositories(db *postgres.DB) workerRepositories {
	return workerRepositories{
		Projects:    postgres.NewProjectRepository(db),
		Scans:       postgres.NewScanRepository(db),
		ScanJobs:    postgres.NewScanJobRepository(db),
		IndexJobs:   postgres.NewIndexJobRepository(db),
		Issues:      postgres.NewIssueRepository(db),
		Measures:    postgres.NewMeasureRepository(db),
		Snapshots:   postgres.NewCodeSnapshotRepository(db),
		Profiles:    postgres.NewProfileSnapshotRepository(db),
		Tags:        postgres.NewTagRepository(db),
		Gates:       postgres.NewGateRepository(db),
		Webhooks:    postgres.NewWebhookRepository(db),
		WebhookJobs: postgres.NewWebhookJobRepository(db),
	}
}

func workerPoolSize(cfg config.Config) int {
	if cfg.WorkerPool >= 1 {
		return int(cfg.WorkerPool)
	}
	return 4
}

func startWorkerPool(ctx context.Context, wc *workerContext) {
	wp := workerPoolSize(wc.cfg)
	for i := 0; i < wp; i++ {
		go func(id int) {
			wrk := ingest.NewScanJobWorker(wc.processor, wc.cfg.WorkerPollDelay, wc.metrics)
			slog.Debug("worker goroutine started", "id", id)
			wrk.Start(ctx)
		}(i)
	}
}

func startHeartbeat(ctx context.Context, wc *workerContext) {
	go func() {
		ticker := time.NewTicker(wc.cfg.WorkerHeartbeatInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := wc.scanJobRepo.UpdateHeartbeat(context.Background(), wc.workerID); err != nil {
				slog.Warn("heartbeat failed", "error", err)
			}
		}
	}()
}

func startDataCleanup(ctx context.Context, wc *workerContext) {
	go func() {
		ticker := time.NewTicker(wc.cfg.WorkerDataCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			runDataCleanup(wc)
		}
	}()
}

func runDataCleanup(wc *workerContext) {
	bg := context.Background()
	for _, target := range []struct {
		table     string
		column    string
		retention time.Duration
		extraCond string
	}{
		{"scan_jobs", "completed_at", wc.cfg.WorkerDataScanJobsRetention, "status = 'completed' AND "},
		{"scans", "created_at", wc.cfg.WorkerDataScansRetention, ""},
		{"measures", "created_at", wc.cfg.WorkerDataMeasuresRetention, ""},
	} {
		query := fmt.Sprintf(`DELETE FROM %s WHERE %s%s < now() - interval '%d seconds'`,
			target.table, target.extraCond, target.column, int64(target.retention.Seconds()))
		tag, err := wc.db.Pool.Exec(bg, query)
		if err != nil {
			slog.Warn("cleanup failed", "table", target.table, "error", err)
			continue
		}
		if tag.RowsAffected() > 0 {
			slog.Info("cleanup", "table", target.table, "deleted", tag.RowsAffected())
		}
	}
}
