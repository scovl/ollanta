package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantastore/search"
	"github.com/scovl/ollanta/ollantaweb/config"
	"github.com/scovl/ollanta/ollantaweb/ingest"
)

func main() {
	cfg := config.MustLoad()
	slog.SetDefault(telemetry.SetupLogger(cfg.LogLevel, "service", "ollantaindexer", "role", "indexer"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTracing, err := telemetry.SetupTracing(ctx, "ollantaindexer")
	if err != nil {
		slog.Error("setup tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			slog.Warn("shutdown tracing", "error", err)
		}
	}()

	db, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		slog.Error("migrate database", "error", err)
		os.Exit(1)
	}

	indexJobRepo := postgres.NewIndexJobRepository(db)
	issueRepo := postgres.NewIssueRepository(db)

	zincCfg := search.ZincConfig{
		Host:     cfg.ZincSearchURL,
		User:     cfg.ZincSearchUser,
		Password: cfg.ZincSearchPassword,
	}
	_, indexer, err := search.NewBackend(cfg.SearchBackend, zincCfg, db.Pool)
	if err != nil {
		slog.Error("create search backend", "error", err)
		os.Exit(1)
	}
	if err := indexer.ConfigureIndexes(ctx); err != nil {
		slog.Warn("search configure failed; continuing", "error", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ollantaindexer"
	}
	workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())
	metricsReg := telemetry.NewRegistry()
	appMetrics := telemetry.NewMetrics(metricsReg)
	telemetry.StartAdminServer(ctx, cfg.AdminAddr, metricsReg, nil)

	worker := ingest.NewWorker(indexer, issueRepo, indexJobRepo, workerID, appMetrics)
	slog.Info("started", "worker_id", workerID, "admin_addr", cfg.AdminAddr)
	worker.Start(ctx)
	slog.Info("stopped")
}
