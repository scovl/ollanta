package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestPrepareDatabaseMigratesWhenEnabled(t *testing.T) {
	t.Parallel()

	manager := &fakeSchemaManager{}
	if err := PrepareDatabase(context.Background(), manager, true); err != nil {
		t.Fatalf("PrepareDatabase() error = %v", err)
	}
	if manager.migrateCalls != 1 || manager.verifyCalls != 0 {
		t.Fatalf("calls = migrate:%d verify:%d, want migrate only", manager.migrateCalls, manager.verifyCalls)
	}
}

func TestPrepareDatabaseVerifiesWhenAutoMigrateDisabled(t *testing.T) {
	t.Parallel()

	manager := &fakeSchemaManager{}
	if err := PrepareDatabase(context.Background(), manager, false); err != nil {
		t.Fatalf("PrepareDatabase() error = %v", err)
	}
	if manager.migrateCalls != 0 || manager.verifyCalls != 1 {
		t.Fatalf("calls = migrate:%d verify:%d, want verify only", manager.migrateCalls, manager.verifyCalls)
	}
}

func TestPrepareDatabasePropagatesSchemaError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("schema stale")
	manager := &fakeSchemaManager{verifyErr: wantErr}
	err := PrepareDatabase(context.Background(), manager, false)
	if !errors.Is(err, wantErr) {
		t.Fatalf("PrepareDatabase() error = %v, want %v", err, wantErr)
	}
}

type fakeSchemaManager struct {
	migrateCalls int
	verifyCalls  int
	migrateErr   error
	verifyErr    error
}

func (m *fakeSchemaManager) Migrate(context.Context) error {
	m.migrateCalls++
	return m.migrateErr
}

func (m *fakeSchemaManager) VerifySchema(context.Context) error {
	m.verifyCalls++
	return m.verifyErr
}
