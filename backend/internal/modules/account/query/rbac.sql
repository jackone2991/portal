-- ── Roles ─────────────────────────────────────────────────────────

-- name: ListRoles :many
SELECT * FROM roles ORDER BY code;

-- name: GetRoleByCode :one
SELECT * FROM roles WHERE code = $1;

-- name: GetRoleByID :one
SELECT * FROM roles WHERE id = $1;

-- name: CreateRole :one
INSERT INTO roles (code, name, description, parent_id, is_system)
VALUES ($1, $2, $3, $4, false)
RETURNING *;

-- name: UpdateRole :one
UPDATE roles
SET name = $2,
    description = $3,
    parent_id = $4,
    updated_at = now()
WHERE id = $1 AND is_system = false
RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1 AND is_system = false;

-- ── Hierarchy resolution ──────────────────────────────────────────

-- name: GetRoleAncestors :many
-- Walks parent chain. Returns the role itself plus all ancestors, in depth order.
WITH RECURSIVE ancestry AS (
    SELECT r.*, 0 AS depth FROM roles r WHERE r.id = $1
    UNION ALL
    SELECT p.*, a.depth + 1 FROM roles p
    JOIN ancestry a ON p.id = a.parent_id
)
SELECT * FROM ancestry ORDER BY depth;

-- name: GetEffectivePermissions :many
-- Returns the union of permissions granted (directly or via ancestor roles)
-- for ALL roles assigned to a user. Skips expired role assignments.
WITH RECURSIVE
  user_role_set AS (
    SELECT ur.role_id
    FROM user_roles ur
    WHERE ur.user_id = $1
      AND (ur.expires_at IS NULL OR ur.expires_at > now())
  ),
  ancestry AS (
    SELECT r.id AS role_id FROM roles r
    JOIN user_role_set urs ON urs.role_id = r.id
    UNION
    SELECT r.parent_id FROM roles r
    JOIN ancestry a ON a.role_id = r.id
    WHERE r.parent_id IS NOT NULL
  )
SELECT DISTINCT p.code
FROM ancestry a
JOIN role_permissions rp ON rp.role_id = a.role_id
JOIN permissions p       ON p.id = rp.permission_id
ORDER BY p.code;

-- ── Permissions ───────────────────────────────────────────────────

-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY code;

-- name: CreatePermission :one
INSERT INTO permissions (code, description) VALUES ($1, $2)
RETURNING *;

-- name: GrantPermissionToRole :exec
INSERT INTO role_permissions (role_id, permission_id, granted_by)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: RevokePermissionFromRole :exec
DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2;

-- name: ListRolePermissions :many
SELECT p.* FROM permissions p
JOIN role_permissions rp ON rp.permission_id = p.id
WHERE rp.role_id = $1
ORDER BY p.code;

-- ── User ↔ Role assignment ────────────────────────────────────────

-- name: AssignRoleToUser :exec
INSERT INTO user_roles (user_id, role_id, granted_by, expires_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, role_id) DO UPDATE
SET granted_by = EXCLUDED.granted_by,
    granted_at = now(),
    expires_at = EXCLUDED.expires_at;

-- name: RevokeRoleFromUser :exec
DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2;

-- name: ListUserRoles :many
SELECT r.* FROM roles r
JOIN user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1
  AND (ur.expires_at IS NULL OR ur.expires_at > now())
ORDER BY r.code;

-- name: ListUsersByRole :many
SELECT u.* FROM users u
JOIN user_roles ur ON ur.user_id = u.id
WHERE ur.role_id = $1
  AND (ur.expires_at IS NULL OR ur.expires_at > now())
ORDER BY u.created_at DESC
LIMIT $2 OFFSET $3;
