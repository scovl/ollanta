// Command ollantamigrate applies PostgreSQL schema migrations and exits.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/config"
)

func main() {
	cfg := config.MustLoad()
	slog.SetDefault(telemetry.SetupLogger(cfg.LogLevel, "service", "ollantamigrate", "role", "migrator"))
	cfg.LogStartupWarnings()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.New(ctx, cfg.DatabaseURL, cfg.PostgresPool)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	startedAt := time.Now()
	if err := db.Migrate(ctx); err != nil {
		slog.Error("migrate database", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied", "duration", time.Since(startedAt).String())
}
