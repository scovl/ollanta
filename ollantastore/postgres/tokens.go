package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Token represents an API token stored in PostgreSQL.
type Token struct {
	ID         int64
	UserID     int64
	Name       string
	TokenHash  string
	TokenType  string // user | project_analysis | global_analysis
	ProjectID  *int64
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
}

// TokenRepository provides CRUD access to the tokens table.
type TokenRepository struct {
	db *DB
}

// NewTokenRepository creates a TokenRepository backed by db.
func NewTokenRepository(db *DB) *TokenRepository {
	return &TokenRepository{db: db}
}

// Create inserts a new token and populates its ID and CreatedAt.
func (r *TokenRepository) Create(ctx context.Context, t *Token) error {
	return r.db.Pool.QueryRow(ctx, `
		INSERT INTO tokens (user_id, name, token_hash, token_type, project_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`,
		t.UserID, t.Name, t.TokenHash, t.TokenType, t.ProjectID, t.ExpiresAt,
	).Scan(&t.ID, &t.CreatedAt)
}

// GetByHash retrieves a token by its SHA-256 hash.
func (r *TokenRepository) GetByHash(ctx context.Context, hash string) (*Token, error) {
	t := &Token{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT id, user_id, name, token_hash, token_type, project_id,
		       last_used_at, expires_at, created_at
		FROM tokens WHERE token_hash = $1`, hash,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.TokenType, &t.ProjectID,
		&t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// ListByUser returns all tokens belonging to a user, ordered by creation time.
func (r *TokenRepository) ListByUser(ctx context.Context, userID int64) ([]*Token, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, user_id, name, token_hash, token_type, project_id,
		       last_used_at, expires_at, created_at
		FROM tokens WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*Token
	for rows.Next() {
		t := &Token{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.TokenHash, &t.TokenType, &t.ProjectID,
			&t.LastUsedAt, &t.ExpiresAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// Delete removes a token by ID, scoped to the owning user.
func (r *TokenRepository) Delete(ctx context.Context, id, userID int64) error {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM tokens WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLastUsed updates last_used_at to now for a token.
func (r *TokenRepository) UpdateLastUsed(ctx context.Context, id int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE tokens SET last_used_at=now() WHERE id=$1`, id)
	return err
}
