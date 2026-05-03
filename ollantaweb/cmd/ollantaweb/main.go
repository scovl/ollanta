// Command ollantaweb is the centralized scan-collection server.
// It exposes a REST API for receiving scan reports from ollantascanner,
// persisting them to PostgreSQL, and indexing them in ZincSearch.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
	"github.com/scovl/ollanta/ollantaweb/api"
	"github.com/scovl/ollanta/ollantaweb/config"
	appruntime "github.com/scovl/ollanta/ollantaweb/internal/runtime"
	"github.com/scovl/ollanta/ollantaweb/webhook"
)

func main() {
	cfg := config.MustLoad()
	slog.SetDefault(telemetry.SetupLogger(cfg.LogLevel, "service", "ollantaweb", "role", "api"))
	cfg.LogStartupWarnings()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTracing, err := telemetry.SetupTracing(ctx, "ollantaweb")
	if err != nil {
		slog.Error("setup tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			slog.Warn("shutdown tracing", "error", err)
		}
	}()

	// ── PostgreSQL ─────────────────────────────────────────────────────────
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
	slog.Info("database ready", "auto_migrate", cfg.AutoMigrate)

	// ── Repositories ───────────────────────────────────────────────────────
	projectRepo := postgres.NewProjectRepository(db)
	scanRepo := postgres.NewScanRepository(db)
	scanJobRepo := postgres.NewScanJobRepository(db)
	indexJobRepo := postgres.NewIndexJobRepository(db)
	issueRepo := postgres.NewIssueRepository(db)
	measureRepo := postgres.NewMeasureRepository(db)
	snapshotRepo := postgres.NewCodeSnapshotRepository(db)
	userRepo := postgres.NewUserRepository(db)
	groupRepo := postgres.NewGroupRepository(db)
	tokenRepo := postgres.NewTokenRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	permRepo := postgres.NewPermissionRepository(db)
	profileRepo := postgres.NewProfileRepository(db)
	gateRepo := postgres.NewGateRepository(db)
	periodRepo := postgres.NewNewCodePeriodRepository(db)
	webhookRepo := postgres.NewWebhookRepository(db)
	webhookJobRepo := postgres.NewWebhookJobRepository(db)
	changelogRepo := postgres.NewChangelogRepository(db)

	// ── Search backend ─────────────────────────────────────────────────────
	zincCfg := search.ZincConfig{
		Host:     cfg.ZincSearchURL,
		User:     cfg.ZincSearchUser,
		Password: cfg.ZincSearchPassword,
	}
	searcher, indexer, err := search.NewBackend(cfg.SearchBackend, zincCfg, db.Pool)
	if err != nil {
		slog.Error("create search backend", "error", err)
		os.Exit(1)
	}
	if err := indexer.ConfigureIndexes(ctx); err != nil {
		slog.Warn("search configure failed; continuing", "error", err)
	}

	// ── Health deps ────────────────────────────────────────────────────────
	api.SetHealthDeps(db, indexer, nil)

	// ── Telemetry ──────────────────────────────────────────────────────────
	metricsReg := telemetry.NewRegistry()
	appMetrics := telemetry.NewMetrics(metricsReg)
	webhookDispatcher := webhook.NewDispatcher(webhookRepo, webhookJobRepo, "ollantaweb", appMetrics)
	appruntime.StartDatabaseMetricsLoop(ctx, db, metricsReg, 30*time.Second)
	api.StartBackgroundTaskMetricsLoop(ctx, api.NewBackgroundTasksHandler(scanJobRepo, indexJobRepo, webhookJobRepo, metricsReg), 30*time.Second)

	// ── HTTP server ────────────────────────────────────────────────────────
	router := api.NewRouter(&api.RouterDeps{
		Config:      cfg,
		Projects:    projectRepo,
		Scans:       scanRepo,
		ScanJobs:    scanJobRepo,
		IndexJobs:   indexJobRepo,
		Issues:      issueRepo,
		Measures:    measureRepo,
		Snapshots:   snapshotRepo,
		Users:       userRepo,
		Groups:      groupRepo,
		Tokens:      tokenRepo,
		Sessions:    sessionRepo,
		Perms:       permRepo,
		Searcher:    searcher,
		Indexer:     indexer,
		Profiles:    profileRepo,
		Gates:       gateRepo,
		Periods:     periodRepo,
		Webhooks:    webhookRepo,
		WebhookJobs: webhookJobRepo,
		Dispatcher:  webhookDispatcher,
		MetricsReg:  metricsReg,
		AppMetrics:  appMetrics,
		Changelog:   changelogRepo,
	})

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      telemetry.WrapHTTPHandler("ollantaweb", router),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Warn("graceful shutdown failed", "error", err)
	}
	slog.Info("stopped")
}
