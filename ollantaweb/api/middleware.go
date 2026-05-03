package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"github.com/scovl/ollanta/ollantaweb/auth"
)

// AuthMiddleware holds the dependencies needed to authenticate requests.
type AuthMiddleware struct {
	users        *postgres.UserRepository
	tokens       *postgres.TokenRepository
	sessions     *postgres.SessionRepository
	secret       []byte
	scannerToken string
}

// NewAuthMiddleware creates a new AuthMiddleware.
// scannerToken is an optional pre-shared key accepted for scanner push (POST /api/v1/scans).
// Pass an empty string to disable scanner token auth.
func NewAuthMiddleware(
	users *postgres.UserRepository,
	tokens *postgres.TokenRepository,
	sessions *postgres.SessionRepository,
	secret []byte,
	scannerToken string,
) *AuthMiddleware {
	return &AuthMiddleware{
		users:        users,
		tokens:       tokens,
		sessions:     sessions,
		secret:       secret,
		scannerToken: scannerToken,
	}
}

// Authenticate is a chi middleware that validates Bearer tokens (JWT or API token).
// On success it stores the user in the request context; on failure it returns 401.
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, message := m.userFromRequest(r)
		if message != "" {
			jsonError(w, http.StatusUnauthorized, message)
			return
		}
		if !user.IsActive {
			jsonError(w, http.StatusUnauthorized, "account deactivated")
			return
		}

		next.ServeHTTP(w, WithUser(r, user))
	})
}

func (m *AuthMiddleware) userFromRequest(r *http.Request) (*postgres.User, string) {
	tokenStr, message := bearerToken(r.Header.Get("Authorization"))
	if message != "" {
		return nil, message
	}
	if m.scannerToken != "" && tokenStr == m.scannerToken {
		return &postgres.User{Login: "scanner", IsActive: true}, ""
	}
	if auth.IsAPIToken(tokenStr) {
		return m.userFromAPIToken(r.Context(), tokenStr)
	}
	return m.userFromJWT(r.Context(), tokenStr)
}

func bearerToken(raw string) (string, string) {
	if raw == "" {
		return "", "authorization required"
	}
	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", "invalid authorization header"
	}
	return parts[1], ""
}

func (m *AuthMiddleware) userFromAPIToken(ctx context.Context, tokenStr string) (*postgres.User, string) {
	hash := auth.HashToken(tokenStr)
	tok, err := m.tokens.GetByHash(ctx, hash)
	if err != nil || tok == nil {
		return nil, "invalid token"
	}
	if tok.ExpiresAt != nil && time.Now().After(*tok.ExpiresAt) {
		return nil, "token expired"
	}
	user, err := m.users.GetByID(ctx, tok.UserID)
	if err != nil {
		return nil, "user not found"
	}
	go func() { _ = m.tokens.UpdateLastUsed(ctx, tok.ID) }()
	return user, ""
}

func (m *AuthMiddleware) userFromJWT(ctx context.Context, tokenStr string) (*postgres.User, string) {
	claims, err := auth.ParseAccessToken(m.secret, tokenStr)
	if err != nil {
		return nil, "invalid or expired token"
	}
	var userID int64
	if _, err := parseIDFromString(claims.Subject, &userID); err != nil {
		return nil, "invalid token subject"
	}
	user, err := m.users.GetByID(ctx, userID)
	if err != nil {
		return nil, "user not found"
	}
	return user, ""
}

// RequirePermission returns a middleware that checks a global permission.
// The permission repository is needed; pass it via closure.
func RequirePermission(perms *postgres.PermissionRepository, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				jsonError(w, http.StatusUnauthorized, "authorization required")
				return
			}
			ok, err := perms.HasGlobal(r.Context(), user.ID, permission)
			if err != nil || !ok {
				jsonError(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// parseIDFromString parses a string into an int64 pointer.
func parseIDFromString(s string, out *int64) (int64, error) {
	var id int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errBadID
		}
		id = id*10 + int64(c-'0')
	}
	if out != nil {
		*out = id
	}
	return id, nil
}

var errBadID = &idParseError{}

type idParseError struct{}

func (e *idParseError) Error() string { return "invalid id" }

// RequestLogger logs each request with method, route, status, and duration.
func RequestLogger(metrics *telemetry.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			metrics.ObserveHTTPRequest(duration)

			route := r.URL.Path
			if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
				if pattern := routeContext.RoutePattern(); pattern != "" {
					route = pattern
				}
			}

			attrs := telemetry.WithTraceAttrs(
				r.Context(),
				"method", r.Method,
				"path", r.URL.Path,
				"route", route,
				"status", ww.Status(),
				"duration_ms", duration.Milliseconds(),
			)

			if ww.Status() >= http.StatusInternalServerError || duration >= 5*time.Second {
				slog.WarnContext(r.Context(), "request completed", attrs...)
				return
			}
			slog.InfoContext(r.Context(), "request completed", attrs...)
		})
	}
}

// CORS adds CORS headers for configured allowed origins.
func CORS(allowedOrigins []string, allowUnsafeWildcard bool) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowWildcard := false
	for _, origin := range allowedOrigins {
		if origin == "*" && allowUnsafeWildcard {
			allowWildcard = true
			continue
		}
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if allowWildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBody limits the request body to limit bytes (default 10 MB).
func MaxBody(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
