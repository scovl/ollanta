package postgres

import (
	"strings"
	"testing"
	"time"
)

func TestPoolConfigValidateAcceptsDefaults(t *testing.T) {
	if err := DefaultPoolConfig().Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestPoolConfigValidateRejectsInconsistentConns(t *testing.T) {
	cfg := PoolConfig{
		MaxConns:        2,
		MinConns:        3,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: time.Minute,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "min connections cannot exceed max connections") {
		t.Fatalf("Validate() error = %q, want min/max guidance", err.Error())
	}
}

func TestLatestMigrationVersion(t *testing.T) {
	version, err := LatestMigrationVersion()
	if err != nil {
		t.Fatalf("LatestMigrationVersion() error = %v", err)
	}
	if version == "" || !strings.Contains(version, "_") {
		t.Fatalf("LatestMigrationVersion() = %q, want migration version name", version)
	}
}

func TestMigrateReleasesAdvisoryLock(t *testing.T) {
	db, ctx, _ := openJobRepositoryTestDB(t)

	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var locked bool
	if err := db.Pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", migrationAdvisoryLockID).Scan(&locked); err != nil {
		t.Fatalf("pg_try_advisory_lock error = %v", err)
	}
	if !locked {
		t.Fatal("migration advisory lock is still held")
	}
	var unlocked bool
	if err := db.Pool.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockID).Scan(&unlocked); err != nil {
		t.Fatalf("pg_advisory_unlock error = %v", err)
	}
	if !unlocked {
		t.Fatal("expected advisory lock to be released by test")
	}
}
