package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestReadyCheckPassesWhenDependenciesAreHealthy(t *testing.T) {
	t.Parallel()

	ready := ReadyCheck(NamedHealthCheck{Name: "postgres", Check: fakeHealthChecker{}})
	if err := ready(context.Background()); err != nil {
		t.Fatalf("ready() error = %v", err)
	}
}

func TestReadyCheckFailsWhenDependencyErrors(t *testing.T) {
	t.Parallel()

	ready := ReadyCheck(NamedHealthCheck{Name: "search", Check: fakeHealthChecker{err: errors.New("down")}})
	err := ready(context.Background())
	if err == nil || !strings.Contains(err.Error(), "search dependency is not ready") {
		t.Fatalf("ready() error = %v, want search readiness error", err)
	}
}

func TestReadyCheckFailsWhenDependencyMissing(t *testing.T) {
	t.Parallel()

	ready := ReadyCheck(NamedHealthCheck{Name: "postgres"})
	err := ready(context.Background())
	if err == nil || !strings.Contains(err.Error(), "postgres dependency is not configured") {
		t.Fatalf("ready() error = %v, want missing dependency error", err)
	}
}

type fakeHealthChecker struct {
	err error
}

func (f fakeHealthChecker) Health(context.Context) error {
	return f.err
}
