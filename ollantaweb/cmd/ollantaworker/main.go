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

func main() {
	cfg := config.MustLoad()
	slog.SetDefault(telemetry.SetupLogger(cfg.LogLevel, "service", "ollantaworker", "role", "worker"))
	cfg.LogStartupWarnings()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTracing, err := telemetry.SetupTracing(ctx, "ollantaworker")
	if err != nil {
		slog.Error("setup tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			slog.Warn("shutdown tracing", "error", err)
		}
	}()

	db, err := postgres.New(ctx, cfg.DatabaseURL, cfg.PostgresPool)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := appruntime.PrepareDatabase(ctx, db, cfg.AutoMigrate); err != nil {
		slog.Error("prepare database", "auto_migrate", cfg.AutoMigrate, "error", err)
		os.Exit(1)
	}

	projectRepo := postgres.NewProjectRepository(db)
	scanRepo := postgres.NewScanRepository(db)
	scanJobRepo := postgres.NewScanJobRepository(db)
	indexJobRepo := postgres.NewIndexJobRepository(db)
	issueRepo := postgres.NewIssueRepository(db)
	measureRepo := postgres.NewMeasureRepository(db)
	snapshotRepo := postgres.NewCodeSnapshotRepository(db)
	profileSnapshotRepo := postgres.NewProfileSnapshotRepository(db)
	tagRepo := postgres.NewTagRepository(db)
	gateRepo := postgres.NewGateRepository(db)
	webhookRepo := postgres.NewWebhookRepository(db)
	webhookJobRepo := postgres.NewWebhookJobRepository(db)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ollantaworker"
	}
	workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())
	metricsReg := telemetry.NewRegistry()
	appMetrics := telemetry.NewMetrics(metricsReg)
	appruntime.StartDatabaseMetricsLoop(ctx, db, metricsReg, 30*time.Second)
	telemetry.StartAdminServer(ctx, cfg.AdminAddr, metricsReg, appruntime.ReadyCheck(
		appruntime.NamedHealthCheck{Name: "postgres", Check: db},
	))

	indexEnqueuer := ingest.NewIndexJobEnqueuer(indexJobRepo)
	webhookDispatcher := webhook.NewDispatcher(webhookRepo, webhookJobRepo, workerID, appMetrics)
	ingest.StartJobRecoveryLoop(ctx, "scan", scanJobRepo, cfg.ScanJobRecovery, appMetrics)

	processor := ingest.NewScanJobProcessor(
		workerID,
		scanJobRepo,
		ingest.IngestRepositories{
			Projects:  projectRepo,
			Scans:     scanRepo,
			Issues:    issueRepo,
			Measures:  measureRepo,
			Snapshots: snapshotRepo,
			Profiles:  profileSnapshotRepo,
			Tags:      tagRepo,
			Gates:     gateRepo,
		},
		indexEnqueuer,
		webhookDispatcher,
	)
	wp := int(cfg.WorkerPool)
	if wp < 1 {
		wp = 4
	}
	slog.Info("started", "worker_id", workerID, "worker_pool", wp, "admin_addr", cfg.AdminAddr)

	for i := 0; i < wp; i++ {
		go func(id int) {
			wrk := ingest.NewScanJobWorker(processor, 100*time.Millisecond, appMetrics)
			slog.Debug("worker goroutine started", "id", id)
			wrk.Start(ctx)
		}(i)
	}

	// Heartbeat goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			_ = scanJobRepo.UpdateHeartbeat(context.Background(), workerID)
		}
	}()

	// Stale job recovery
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			n, _ := scanJobRepo.ReclaimStale(context.Background(), 30*time.Second)
			if n > 0 {
				slog.Warn("reclaimed stale jobs", "count", n)
			}
		}
	}()

	// Data lifecycle cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			ctx := context.Background()
			pool := db.Pool
			if tag, err := pool.Exec(ctx, `DELETE FROM scan_jobs WHERE status = 'completed' AND completed_at < now() - interval '7 days'`); err != nil {
				slog.Warn("cleanup scan_jobs failed", "error", err)
			} else if tag.RowsAffected() > 0 {
				slog.Info("cleanup scan_jobs", "deleted", tag.RowsAffected())
			}
			if tag, err := pool.Exec(ctx, `DELETE FROM scans WHERE created_at < now() - interval '365 days'`); err != nil {
				slog.Warn("cleanup scans failed", "error", err)
			} else if tag.RowsAffected() > 0 {
				slog.Info("cleanup scans", "deleted", tag.RowsAffected())
			}
			if tag, err := pool.Exec(ctx, `DELETE FROM measures WHERE created_at < now() - interval '90 days'`); err != nil {
				slog.Warn("cleanup measures failed", "error", err)
			} else if tag.RowsAffected() > 0 {
				slog.Info("cleanup measures", "deleted", tag.RowsAffected())
			}
		}
	}()

	<-ctx.Done()

	webhookDispatcher.Stop()
	slog.Info("stopped")
}
