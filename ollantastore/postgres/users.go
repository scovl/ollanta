package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// User is the canonical user record stored in PostgreSQL.
type User struct {
	ID           int64
	Login        string
	Email        string
	Name         string
	PasswordHash string
	AvatarURL    string
	Provider     string
	ProviderID   string
	IsActive     bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserRepository provides CRUD access to the users table.
type UserRepository struct {
	db *DB
}

// NewUserRepository creates a UserRepository backed by db.
func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

const userColumns = `id, login, email, name, password_hash, avatar_url,
	provider, provider_id, is_active, last_login_at, created_at, updated_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	u := &User{}
	err := row.Scan(
		&u.ID, &u.Login, &u.Email, &u.Name, &u.PasswordHash, &u.AvatarURL,
		&u.Provider, &u.ProviderID, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// Create inserts a new user and populates its ID and timestamps.
func (r *UserRepository) Create(ctx context.Context, u *User) error {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO users (login, email, name, password_hash, avatar_url, provider, provider_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+userColumns,
		u.Login, u.Email, u.Name, u.PasswordHash, u.AvatarURL, u.Provider, u.ProviderID,
	)
	got, err := scanUser(row)
	if err != nil {
		return err
	}
	*u = *got
	return nil
}

// GetByID retrieves a user by primary key.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(r.db.Pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// GetByLogin retrieves a user by login name.
func (r *UserRepository) GetByLogin(ctx context.Context, login string) (*User, error) {
	return scanUser(r.db.Pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE login = $1`, login))
}

// GetByEmail retrieves a user by email address.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(r.db.Pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1`, email))
}

// GetByProvider retrieves a user by external provider + provider-specific ID.
func (r *UserRepository) GetByProvider(ctx context.Context, provider, providerID string) (*User, error) {
	return scanUser(r.db.Pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE provider = $1 AND provider_id = $2`,
		provider, providerID))
}

// UpsertOAuth creates or updates a user from OAuth provider data.
// The user is matched by (provider, provider_id). If not found, a new user is created.
func (r *UserRepository) UpsertOAuth(ctx context.Context, u *User) error {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO users (login, email, name, avatar_url, provider, provider_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider_id) DO NOTHING
		RETURNING `+userColumns,
		u.Login, u.Email, u.Name, u.AvatarURL, u.Provider, u.ProviderID,
	)
	got, err := scanUser(row)
	if errors.Is(err, ErrNotFound) {
		// Row already existed; fetch it
		return r.db.Pool.QueryRow(ctx,
			`UPDATE users SET name=$1, avatar_url=$2, updated_at=now()
			 WHERE provider=$3 AND provider_id=$4
			 RETURNING `+userColumns,
			u.Name, u.AvatarURL, u.Provider, u.ProviderID,
		).Scan(
			&u.ID, &u.Login, &u.Email, &u.Name, &u.PasswordHash, &u.AvatarURL,
			&u.Provider, &u.ProviderID, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
		)
	}
	if err != nil {
		return err
	}
	*u = *got
	return nil
}

// List returns a paginated list of users and the total count.
func (r *UserRepository) List(ctx context.Context, page, pageSize int) ([]*User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize

	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Pool.Query(ctx,
		`SELECT `+userColumns+` FROM users ORDER BY id LIMIT $1 OFFSET $2`,
		pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(
			&u.ID, &u.Login, &u.Email, &u.Name, &u.PasswordHash, &u.AvatarURL,
			&u.Provider, &u.ProviderID, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// Update saves name, email, avatar_url changes for an existing user.
func (r *UserRepository) Update(ctx context.Context, u *User) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET name=$1, email=$2, avatar_url=$3, updated_at=now() WHERE id=$4`,
		u.Name, u.Email, u.AvatarURL, u.ID)
	return err
}

// SetPassword updates the password hash for a user.
func (r *UserRepository) SetPassword(ctx context.Context, id int64, hash string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET password_hash=$1, updated_at=now() WHERE id=$2`, hash, id)
	return err
}

// Deactivate sets is_active=false for a user (soft delete).
func (r *UserRepository) Deactivate(ctx context.Context, id int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET is_active=FALSE, updated_at=now() WHERE id=$1`, id)
	return err
}

// Reactivate sets is_active=true for a previously deactivated user.
func (r *UserRepository) Reactivate(ctx context.Context, id int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET is_active=TRUE, updated_at=now() WHERE id=$1`, id)
	return err
}

// SetLastLogin updates last_login_at to now for a user.
func (r *UserRepository) SetLastLogin(ctx context.Context, id int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET last_login_at=now() WHERE id=$1`, id)
	return err
}

// Count returns the total number of users.
func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var n int
	return n, r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
}
