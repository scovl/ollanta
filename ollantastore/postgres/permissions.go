package postgres

import (
	"context"
	"time"
)

// GlobalPermission represents a global permission grant.
type GlobalPermission struct {
	ID         int64
	Target     string
	TargetID   int64
	Permission string
	CreatedAt  time.Time
}

// ProjectPermission represents a per-project permission grant.
type ProjectPermission struct {
	ID         int64
	ProjectID  int64
	Target     string
	TargetID   int64
	Permission string
	CreatedAt  time.Time
}

// PermissionRepository provides grant/revoke/check operations on permissions.
type PermissionRepository struct {
	db *DB
}

// NewPermissionRepository creates a PermissionRepository backed by db.
func NewPermissionRepository(db *DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// GrantGlobal grants a global permission to a user or group (idempotent).
func (r *PermissionRepository) GrantGlobal(ctx context.Context, target string, targetID int64, permission string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO global_permissions (target, target_id, permission)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		target, targetID, permission)
	return err
}

// RevokeGlobal removes a global permission from a user or group.
func (r *PermissionRepository) RevokeGlobal(ctx context.Context, target string, targetID int64, permission string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM global_permissions WHERE target=$1 AND target_id=$2 AND permission=$3`,
		target, targetID, permission)
	return err
}

// ListGlobal returns all global permission grants.
func (r *PermissionRepository) ListGlobal(ctx context.Context) ([]GlobalPermission, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, target, target_id, permission, created_at
		 FROM global_permissions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []GlobalPermission
	for rows.Next() {
		p := GlobalPermission{}
		if err := rows.Scan(&p.ID, &p.Target, &p.TargetID, &p.Permission, &p.CreatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// HasGlobal returns true if the user has the given global permission,
// either directly or through any group membership.
func (r *PermissionRepository) HasGlobal(ctx context.Context, userID int64, permission string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM global_permissions
			WHERE permission = $2
			  AND (
			    (target = 'user'  AND target_id = $1)
			    OR
			    (target = 'group' AND target_id IN (
			        SELECT group_id FROM group_members WHERE user_id = $1
			    ))
			  )
		)`, userID, permission,
	).Scan(&exists)
	return exists, err
}

// GrantProject grants a permission on a specific project.
func (r *PermissionRepository) GrantProject(ctx context.Context, projectID int64, target string, targetID int64, permission string) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO project_permissions (project_id, target, target_id, permission)
		 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		projectID, target, targetID, permission)
	return err
}

// RevokeProject removes a permission from a specific project.
func (r *PermissionRepository) RevokeProject(ctx context.Context, projectID int64, target string, targetID int64, permission string) error {
	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM project_permissions WHERE project_id=$1 AND target=$2 AND target_id=$3 AND permission=$4`,
		projectID, target, targetID, permission)
	return err
}

// ListProject returns all permission grants for a project.
func (r *PermissionRepository) ListProject(ctx context.Context, projectID int64) ([]ProjectPermission, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, project_id, target, target_id, permission, created_at
		 FROM project_permissions WHERE project_id=$1 ORDER BY id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []ProjectPermission
	for rows.Next() {
		p := ProjectPermission{}
		if err := rows.Scan(&p.ID, &p.ProjectID, &p.Target, &p.TargetID, &p.Permission, &p.CreatedAt); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// HasProject returns true if the user has the given permission on the project,
// either directly, through global 'admin', or through group membership.
func (r *PermissionRepository) HasProject(ctx context.Context, userID, projectID int64, permission string) (bool, error) {
	// Global admin always has project access
	if ok, err := r.HasGlobal(ctx, userID, "admin"); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	var exists bool
	err := r.db.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM project_permissions
			WHERE project_id = $3
			  AND permission = $2
			  AND (
			    (target = 'user'  AND target_id = $1)
			    OR
			    (target = 'group' AND target_id IN (
			        SELECT group_id FROM group_members WHERE user_id = $1
			    ))
			  )
		)`, userID, permission, projectID,
	).Scan(&exists)
	return exists, err
}
