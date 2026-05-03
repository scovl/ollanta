package runtime

import (
	"context"
	"fmt"
)

// HealthChecker is implemented by dependencies that can report readiness.
type HealthChecker interface {
	Health(ctx context.Context) error
}

// NamedHealthCheck describes one dependency required by a role.
type NamedHealthCheck struct {
	Name  string
	Check HealthChecker
}

// ReadyCheck returns an admin-server readiness function for role dependencies.
func ReadyCheck(checks ...NamedHealthCheck) func(context.Context) error {
	return func(ctx context.Context) error {
		for _, check := range checks {
			if check.Check == nil {
				return fmt.Errorf("%s dependency is not configured", check.Name)
			}
			if err := check.Check.Health(ctx); err != nil {
				return fmt.Errorf("%s dependency is not ready: %w", check.Name, err)
			}
		}
		return nil
	}
}
