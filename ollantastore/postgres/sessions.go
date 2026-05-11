package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Session holds a refresh token record used for rotating JWT sessions.
type Session struct {
	ID          int64
	UserID      int64
	RefreshHash string
	UserAgent   string
	IPAddress   string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// SessionRepository provides CRUD access to the sessions table.
type SessionRepository struct {
	db *DB
}

// NewSessionRepository creates a SessionRepository backed by db.
func NewSessionRepository(db *DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create inserts a new session.
func (r *SessionRepository) Create(ctx context.Context, s *Session) error {
	return r.db.Pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, refresh_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		s.UserID, s.RefreshHash, s.UserAgent, s.IPAddress, s.ExpiresAt,
	).Scan(&s.ID, &s.CreatedAt)
}

// GetByHash retrieves a session by its refresh token hash.
func (r *SessionRepository) GetByHash(ctx context.Context, hash string) (*Session, error) {
	s := &Session{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, refresh_hash, user_agent, ip_address, expires_at, created_at
		FROM sessions WHERE refresh_hash = $1`, hash,
	).Scan(&s.ID, &s.UserID, &s.RefreshHash, &s.UserAgent, &s.IPAddress, &s.ExpiresAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// Delete removes a specific session by ID.
func (r *SessionRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM sessions WHERE id=$1`, id)
	return err
}

// DeleteByHash removes a session by its refresh hash (used at logout).
func (r *SessionRepository) DeleteByHash(ctx context.Context, hash string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM sessions WHERE refresh_hash=$1`, hash)
	return err
}

// DeleteByUserID revokes all sessions for a user.
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID)
	return err
}
