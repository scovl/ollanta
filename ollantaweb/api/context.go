package api

import (
	"context"
	"net/http"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

type contextKey int

const ctxUserKey contextKey = iota

// WithUser stores the authenticated user in the request context.
func WithUser(r *http.Request, u *postgres.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxUserKey, u))
}

// UserFromContext retrieves the authenticated user from the context.
// Returns nil if no user is present.
func UserFromContext(ctx context.Context) *postgres.User {
	u, _ := ctx.Value(ctxUserKey).(*postgres.User)
	return u
}
