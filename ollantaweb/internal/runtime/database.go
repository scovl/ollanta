package runtime

import "context"

// SchemaManager is implemented by PostgreSQL database handles that can migrate or verify schema state.
type SchemaManager interface {
	Migrate(ctx context.Context) error
	VerifySchema(ctx context.Context) error
}

// PrepareDatabase migrates automatically when enabled, otherwise verifies schema compatibility.
func PrepareDatabase(ctx context.Context, db SchemaManager, autoMigrate bool) error {
	if autoMigrate {
		return db.Migrate(ctx)
	}
	return db.VerifySchema(ctx)
}
