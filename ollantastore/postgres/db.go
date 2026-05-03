// Package postgres provides PostgreSQL-backed repositories for the Ollanta platform.
// It uses pgx/v5 (binary protocol, connection pooling) and a built-in migration runner
// that applies versioned SQL files embedded in the binary.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps a pgxpool.Pool with health check and migration support.
type DB struct {
	Pool *pgxpool.Pool
}

const migrationAdvisoryLockID int64 = 0x6f6c6c616e7461

// PoolConfig controls PostgreSQL connection pool sizing and lifetimes.
type PoolConfig struct {
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// DefaultPoolConfig returns Ollanta's local-development pool defaults.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConns:        25,
		MinConns:        5,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	}
}

// Validate checks that pool settings are usable before a pool is opened.
func (c PoolConfig) Validate() error {
	if c.MaxConns <= 0 {
		return errors.New("max connections must be greater than zero")
	}
	if c.MinConns < 0 {
		return errors.New("min connections cannot be negative")
	}
	if c.MinConns > c.MaxConns {
		return errors.New("min connections cannot exceed max connections")
	}
	if c.MaxConnLifetime <= 0 {
		return errors.New("max connection lifetime must be greater than zero")
	}
	if c.MaxConnIdleTime <= 0 {
		return errors.New("max connection idle time must be greater than zero")
	}
	return nil
}

// New creates a connection pool using the given PostgreSQL URL.
// The URL format is: postgres://user:pass@host:5432/dbname?sslmode=disable
func New(ctx context.Context, databaseURL string, poolConfigs ...PoolConfig) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	poolConfig := DefaultPoolConfig()
	if len(poolConfigs) > 0 {
		poolConfig = poolConfigs[0]
	}
	if err := poolConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid pool config: %w", err)
	}
	cfg.MaxConns = poolConfig.MaxConns
	cfg.MinConns = poolConfig.MinConns
	cfg.MaxConnLifetime = poolConfig.MaxConnLifetime
	cfg.MaxConnIdleTime = poolConfig.MaxConnIdleTime
	cfg.ConnConfig.Tracer = queryTracer{}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Health pings the database with a SELECT 1.
func (db *DB) Health(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, "SELECT 1")
	return err
}

// Close closes all connections in the pool.
func (db *DB) Close() {
	db.Pool.Close()
}

// Migrate applies all pending up-migrations found in the embedded migrations directory.
// It creates a schema_migrations table to track which migrations have been applied.
func (db *DB) Migrate(ctx context.Context) error {
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn for migration: %w", err)
	}
	defer conn.Release()

	if err := acquireMigrationLock(ctx, conn); err != nil {
		return err
	}
	defer releaseMigrationLock(conn)

	if err := ensureSchemaMigrations(ctx, conn); err != nil {
		return err
	}
	files, err := migrationFiles()
	if err != nil {
		return err
	}
	return applyPendingMigrations(ctx, conn, files)
}

func acquireMigrationLock(ctx context.Context, conn *pgxpool.Conn) error {
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockID); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	return nil
}

func releaseMigrationLock(conn *pgxpool.Conn) {
	var unlocked bool
	if err := conn.QueryRow(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockID).Scan(&unlocked); err != nil {
		slog.Error("release migration advisory lock", "error", err)
		return
	}
	if !unlocked {
		slog.Error("release migration advisory lock", "error", "lock was not held")
	}
}

func ensureSchemaMigrations(ctx context.Context, conn *pgxpool.Conn) error {
	_, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

func applyPendingMigrations(ctx context.Context, conn *pgxpool.Conn, files []string) error {
	for _, f := range files {
		version := migrationVersion(f)
		exists, err := migrationApplied(ctx, conn, version)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := applyMigration(ctx, conn, f, version); err != nil {
			return err
		}
	}
	return nil
}

func migrationApplied(ctx context.Context, conn *pgxpool.Conn, version string) (bool, error) {
	var exists bool
	err := conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}

func applyMigration(ctx context.Context, conn *pgxpool.Conn, path, version string) error {
	sql, err := migrationsFS.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", path, err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx for migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("run migration %s: %w", version, err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}

// VerifySchema checks that the database has the latest embedded migration applied.
func (db *DB) VerifySchema(ctx context.Context) error {
	latest, err := LatestMigrationVersion()
	if err != nil {
		return err
	}

	var exists bool
	if err := db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_name = 'schema_migrations'
		)`).Scan(&exists); err != nil {
		return fmt.Errorf("check schema_migrations table: %w", err)
	}
	if !exists {
		return errors.New("database schema is not initialized; run ollantamigrate or enable OLLANTA_AUTO_MIGRATE")
	}

	if err := db.Pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", latest,
	).Scan(&exists); err != nil {
		return fmt.Errorf("check latest migration %s: %w", latest, err)
	}
	if !exists {
		return fmt.Errorf("database schema is not current; missing migration %s", latest)
	}
	return nil
}

// LatestMigrationVersion returns the newest embedded up-migration version.
func LatestMigrationVersion() (string, error) {
	files, err := migrationFiles()
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("no embedded migrations found")
	}
	return migrationVersion(files[len(files)-1]), nil
}

func migrationFiles() ([]string, error) {
	files, err := fs.Glob(migrationsFS, "migrations/*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func migrationVersion(path string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, "migrations/"), ".up.sql")
}
