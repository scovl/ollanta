package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
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
		raw := r.Header.Get("Authorization")
		if raw == "" {
			jsonError(w, http.StatusUnauthorized, "authorization required")
			return
		}
		parts := strings.SplitN(raw, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			jsonError(w, http.StatusUnauthorized, "invalid authorization header")
			return
		}
		tokenStr := parts[1]

		// Scanner pre-shared token — allows CI/CD push without a user account.
		if m.scannerToken != "" && tokenStr == m.scannerToken {
			next.ServeHTTP(w, WithUser(r, &postgres.User{
				Login:    "scanner",
				IsActive: true,
			}))
			return
		}

		var user *postgres.User

		if auth.IsAPIToken(tokenStr) {
			// API token path
			hash := auth.HashToken(tokenStr)
			tok, err := m.tokens.GetByHash(r.Context(), hash)
			if err != nil || tok == nil {
				jsonError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			if tok.ExpiresAt != nil && time.Now().After(*tok.ExpiresAt) {
				jsonError(w, http.StatusUnauthorized, "token expired")
				return
			}
			u, err := m.users.GetByID(r.Context(), tok.UserID)
			if err != nil {
				jsonError(w, http.StatusUnauthorized, "user not found")
				return
			}
			user = u
			// Non-blocking last_used update
			go func() { _ = m.tokens.UpdateLastUsed(r.Context(), tok.ID) }()
		} else {
			// JWT path
			claims, err := auth.ParseAccessToken(m.secret, tokenStr)
			if err != nil {
				jsonError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			var userID int64
			if _, err := parseIDFromString(claims.Subject, &userID); err != nil {
				jsonError(w, http.StatusUnauthorized, "invalid token subject")
				return
			}
			u, err := m.users.GetByID(r.Context(), userID)
			if err != nil {
				jsonError(w, http.StatusUnauthorized, "user not found")
				return
			}
			user = u
		}

		if !user.IsActive {
			jsonError(w, http.StatusUnauthorized, "account deactivated")
			return
		}

		next.ServeHTTP(w, WithUser(r, user))
	})
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

// RequestLogger logs each request with method, path, status, and duration.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, ww.Status(), time.Since(start))
	})
}

// CORS adds permissive CORS headers to every response.
// In production, replace with a proper allowlist.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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
