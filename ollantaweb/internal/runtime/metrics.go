package runtime

import (
	"context"
	"log/slog"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

// StartDatabaseMetricsLoop refreshes PostgreSQL pool and health metrics periodically.
func StartDatabaseMetricsLoop(ctx context.Context, db *postgres.DB, reg *telemetry.Registry, interval time.Duration) {
	if db == nil || reg == nil || interval <= 0 {
		return
	}
	go func() {
		RefreshDatabaseMetrics(ctx, db, reg)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				RefreshDatabaseMetrics(ctx, db, reg)
			}
		}
	}()
}

// RefreshDatabaseMetrics records current PostgreSQL pool stats and health.
func RefreshDatabaseMetrics(ctx context.Context, db *postgres.DB, reg *telemetry.Registry) {
	if db == nil || reg == nil || db.Pool == nil {
		return
	}
	stat := db.Pool.Stat()
	reg.Gauge("ollanta_db_pool_acquired_conns", "Current acquired PostgreSQL pool connections").Set(int64(stat.AcquiredConns()))
	reg.Gauge("ollanta_db_pool_idle_conns", "Current idle PostgreSQL pool connections").Set(int64(stat.IdleConns()))
	reg.Gauge("ollanta_db_pool_total_conns", "Current total PostgreSQL pool connections").Set(int64(stat.TotalConns()))
	if err := db.Health(ctx); err != nil {
		reg.Gauge("ollanta_db_health", "PostgreSQL health status, 1 healthy and 0 unhealthy").Set(0)
		slog.WarnContext(ctx, "refresh database health metric", "error", err)
		return
	}
	reg.Gauge("ollanta_db_health", "PostgreSQL health status, 1 healthy and 0 unhealthy").Set(1)
}
