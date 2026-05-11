package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Group is the canonical group record.
type Group struct {
	ID          int64
	Name        string
	Description string
	IsBuiltin   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GroupRepository provides CRUD access to groups and group_members.
type GroupRepository struct {
	db *DB
}

// NewGroupRepository creates a GroupRepository backed by db.
func NewGroupRepository(db *DB) *GroupRepository {
	return &GroupRepository{db: db}
}

func scanGroup(row interface{ Scan(...any) error }) (*Group, error) {
	g := &Group{}
	err := row.Scan(&g.ID, &g.Name, &g.Description, &g.IsBuiltin, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return g, err
}

// Create inserts a new group.
func (r *GroupRepository) Create(ctx context.Context, g *Group) error {
	row := r.db.Pool.QueryRow(ctx,
		`INSERT INTO groups (name, description) VALUES ($1, $2)
		 RETURNING id, name, description, is_builtin, created_at, updated_at`,
		g.Name, g.Description)
	got, err := scanGroup(row)
	if err != nil {
		return err
	}
	*g = *got
	return nil
}

// GetByID retrieves a group by primary key.
func (r *GroupRepository) GetByID(ctx context.Context, id int64) (*Group, error) {
	return scanGroup(r.db.Pool.QueryRow(ctx,
		`SELECT id, name, description, is_builtin, created_at, updated_at FROM groups WHERE id=$1`, id))
}

// GetByName retrieves a group by name.
func (r *GroupRepository) GetByName(ctx context.Context, name string) (*Group, error) {
	return scanGroup(r.db.Pool.QueryRow(ctx,
		`SELECT id, name, description, is_builtin, created_at, updated_at FROM groups WHERE name=$1`, name))
}

// List returns all groups ordered by name.
func (r *GroupRepository) List(ctx context.Context) ([]*Group, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, description, is_builtin, created_at, updated_at FROM groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		g := &Group{}
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.IsBuiltin, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// Update saves name and description changes (only for non-builtin groups).
func (r *GroupRepository) Update(ctx context.Context, g *Group) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE groups SET name=$1, description=$2, updated_at=now() WHERE id=$3 AND is_builtin=FALSE`,
		g.Name, g.Description, g.ID)
	return err
}

// Delete removes a non-builtin group.
func (r *GroupRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM groups WHERE id=$1 AND is_builtin=FALSE`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddMember adds a user to a group (idempotent).
func (r *GroupRepository) AddMember(ctx context.Context, groupID, userID int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		groupID, userID)
	return err
}

// RemoveMember removes a user from a group.
func (r *GroupRepository) RemoveMember(ctx context.Context, groupID, userID int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM group_members WHERE group_id=$1 AND user_id=$2`, groupID, userID)
	return err
}

// ListMembers returns all users belonging to a group.
func (r *GroupRepository) ListMembers(ctx context.Context, groupID int64) ([]*User, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT u.`+userColumns+`
		 FROM users u
		 JOIN group_members gm ON gm.user_id = u.id
		 WHERE gm.group_id = $1
		 ORDER BY u.login`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(
			&u.ID, &u.Login, &u.Email, &u.Name, &u.PasswordHash, &u.AvatarURL,
			&u.Provider, &u.ProviderID, &u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ListUserGroups returns all groups a user belongs to.
func (r *GroupRepository) ListUserGroups(ctx context.Context, userID int64) ([]*Group, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT g.id, g.name, g.description, g.is_builtin, g.created_at, g.updated_at
		 FROM groups g
		 JOIN group_members gm ON gm.group_id = g.id
		 WHERE gm.user_id = $1
		 ORDER BY g.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*Group
	for rows.Next() {
		g := &Group{}
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.IsBuiltin, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// AddUserToDefaultGroup adds a user to the 'ollanta-users' built-in group.
func (r *GroupRepository) AddUserToDefaultGroup(ctx context.Context, userID int64) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO group_members (group_id, user_id)
		 SELECT id, $1 FROM groups WHERE name = 'ollanta-users'
		 ON CONFLICT DO NOTHING`, userID)
	return err
}
