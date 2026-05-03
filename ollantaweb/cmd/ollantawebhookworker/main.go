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
	slog.SetDefault(telemetry.SetupLogger(cfg.LogLevel, "service", "ollantawebhookworker", "role", "webhookworker"))
	cfg.LogStartupWarnings()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTracing, err := telemetry.SetupTracing(ctx, "ollantawebhookworker")
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

	webhookRepo := postgres.NewWebhookRepository(db)
	webhookJobRepo := postgres.NewWebhookJobRepository(db)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "ollantawebhookworker"
	}
	workerID := fmt.Sprintf("%s-%d", hostname, os.Getpid())
	metricsReg := telemetry.NewRegistry()
	appMetrics := telemetry.NewMetrics(metricsReg)
	appruntime.StartDatabaseMetricsLoop(ctx, db, metricsReg, 30*time.Second)
	telemetry.StartAdminServer(ctx, cfg.AdminAddr, metricsReg, appruntime.ReadyCheck(
		appruntime.NamedHealthCheck{Name: "postgres", Check: db},
	))

	dispatcher := webhook.NewDispatcher(webhookRepo, webhookJobRepo, workerID, appMetrics)
	ingest.StartJobRecoveryLoop(ctx, "webhook", webhookJobRepo, cfg.WebhookJobRecovery, appMetrics)
	slog.Info("started", "worker_id", workerID, "admin_addr", cfg.AdminAddr)
	dispatcher.Start(ctx)
	slog.Info("stopped")
}
