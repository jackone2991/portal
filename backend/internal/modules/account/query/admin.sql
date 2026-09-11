-- Admin console reads/writes: the user directory, the approval queue, and the
-- role/permission matrix. Everything here is behind users:read:any /
-- users:approve / rbac:role:* — see handler/admin.go for the route→permission map.

-- ── User directory ────────────────────────────────────────────────

-- name: ListUsersAdmin :many
-- One row per user with their role codes folded in, so the directory renders
-- without an N+1 walk of user_roles. Both filters are optional: pass NULL for
-- `status` to span every approval state, and an empty `q` to skip the search.
-- Expired role grants are excluded here exactly as they are in ListUserRoles —
-- a lapsed grant must not show as a live badge.
SELECT u.id,
       u.email,
       u.display_name,
       u.avatar_url,
       u.approval_status,
       u.approval_note,
       u.approved_at,
       u.approved_by,
       u.disabled_at,
       u.created_at,
       (u.password_hash IS NOT NULL)::bool AS has_password,
       COALESCE(
           ARRAY(
               SELECT r.code FROM user_roles ur
               JOIN roles r ON r.id = ur.role_id
               WHERE ur.user_id = u.id
                 AND (ur.expires_at IS NULL OR ur.expires_at > now())
               ORDER BY r.code
           ),
           ARRAY[]::text[]
       )::text[] AS role_codes
FROM users u
WHERE (sqlc.narg('status')::text IS NULL OR u.approval_status = sqlc.narg('status')::text)
  AND (sqlc.arg('q')::text = '' OR u.email ILIKE '%' || sqlc.arg('q')::text || '%'
                                OR u.display_name ILIKE '%' || sqlc.arg('q')::text || '%')
ORDER BY u.created_at DESC, u.id DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountUsersAdmin :one
-- Total for the same filter as ListUsersAdmin, so the grid can page.
SELECT count(*) FROM users u
WHERE (sqlc.narg('status')::text IS NULL OR u.approval_status = sqlc.narg('status')::text)
  AND (sqlc.arg('q')::text = '' OR u.email ILIKE '%' || sqlc.arg('q')::text || '%'
                                OR u.display_name ILIKE '%' || sqlc.arg('q')::text || '%');

-- name: CountUsersByApprovalStatus :many
-- Drives the queue badge ("3 chờ duyệt") without fetching the rows.
SELECT approval_status, count(*) AS total
FROM users
GROUP BY approval_status;

-- name: GetUserAdmin :one
SELECT u.id,
       u.email,
       u.display_name,
       u.avatar_url,
       u.approval_status,
       u.approval_note,
       u.approved_at,
       u.approved_by,
       u.disabled_at,
       u.created_at,
       (u.password_hash IS NOT NULL)::bool AS has_password,
       COALESCE(
           ARRAY(
               SELECT r.code FROM user_roles ur
               JOIN roles r ON r.id = ur.role_id
               WHERE ur.user_id = u.id
                 AND (ur.expires_at IS NULL OR ur.expires_at > now())
               ORDER BY r.code
           ),
           ARRAY[]::text[]
       )::text[] AS role_codes
FROM users u
WHERE u.id = $1;

-- name: CountUsers :one
-- Used once, at registration, to detect a first-run install. See
-- handler/auth.go: the founding account approves itself and takes superadmin,
-- because otherwise a fresh deployment has nobody who can ever approve anyone.
SELECT count(*) FROM users;

-- ── Approval decisions ────────────────────────────────────────────

-- name: SetUserApproval :one
-- Records the decision and bumps token_version unconditionally.
--
-- The bump is the whole point on a rejection or a revocation: RequireAuth
-- re-reads this row on every request, so any session the user already holds
-- dies on their next call rather than lingering for the rest of the access-token
-- TTL. On an approval it is harmless — a pending user has no session to lose.
UPDATE users
SET approval_status = sqlc.arg('status')::text,
    approval_note   = sqlc.narg('note'),
    approved_at     = CASE WHEN sqlc.arg('status')::text = 'pending' THEN NULL ELSE now() END,
    approved_by     = sqlc.narg('approved_by'),
    token_version   = token_version + 1,
    updated_at      = now()
WHERE id = sqlc.arg('id')
RETURNING id, email, display_name, approval_status, approval_note, approved_at, approved_by, disabled_at, created_at;

-- ── Role assignment (bulk set) ────────────────────────────────────

-- name: ReplaceUserRoles :exec
-- Sets a user's roles to exactly `codes`, in one statement, so the grid's "save
-- roles" is atomic instead of a diff of grants and revokes that can half-apply.
-- Unknown codes are silently dropped by the join — the handler validates first
-- so that cannot happen from the UI.
WITH wanted AS (
    SELECT r.id FROM roles r WHERE r.code = ANY(sqlc.arg('codes')::text[])
),
removed AS (
    DELETE FROM user_roles
    WHERE user_id = sqlc.arg('user_id')
      AND role_id NOT IN (SELECT id FROM wanted)
)
INSERT INTO user_roles (user_id, role_id, granted_by)
SELECT sqlc.arg('user_id'), w.id, sqlc.narg('granted_by')
FROM wanted w
ON CONFLICT (user_id, role_id) DO NOTHING;

-- ── Role / permission matrix ──────────────────────────────────────

-- name: ListRolesAdmin :many
-- Roles plus their parent's CODE (not just the id) and how many users hold each,
-- which is what the matrix header and the delete guard both need.
SELECT r.id,
       r.code,
       r.name,
       r.description,
       r.parent_id,
       p.code AS parent_code,
       r.is_system,
       (SELECT count(*) FROM user_roles ur WHERE ur.role_id = r.id) AS user_count
FROM roles r
LEFT JOIN roles p ON p.id = r.parent_id
ORDER BY r.code;

-- name: ListAllRolePermissions :many
-- Every (role, permission) grant in one shot. The matrix is a dense grid, so
-- fetching it per-role would be one round trip per row for no reason.
SELECT r.code AS role_code, p.code AS permission_code
FROM role_permissions rp
JOIN roles r       ON r.id = rp.role_id
JOIN permissions p ON p.id = rp.permission_id
ORDER BY r.code, p.code;

-- name: ReplaceRolePermissions :exec
-- Same atomic-set shape as ReplaceUserRoles: the matrix saves a whole row of
-- checkboxes, and a partially applied row is a security hole, not a glitch.
WITH wanted AS (
    SELECT p.id FROM permissions p WHERE p.code = ANY(sqlc.arg('codes')::text[])
),
removed AS (
    DELETE FROM role_permissions
    WHERE role_id = sqlc.arg('role_id')
      AND permission_id NOT IN (SELECT id FROM wanted)
)
INSERT INTO role_permissions (role_id, permission_id, granted_by)
SELECT sqlc.arg('role_id'), w.id, sqlc.narg('granted_by')
FROM wanted w
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- name: ListUserIDsAffectedByRole :many
-- Everyone whose effective permissions change when this role's grants change:
-- holders of the role itself AND holders of any role that inherits from it. A
-- plain `WHERE role_id = $1` would miss the descendants, so revoking something
-- from `creator` would leave every `editor` still holding it until the cache
-- expired.
WITH RECURSIVE descendants (role_id) AS (
    SELECT r.id FROM roles r WHERE r.id = $1
    UNION
    SELECT c.id FROM roles c JOIN descendants d ON c.parent_id = d.role_id
)
SELECT DISTINCT ur.user_id
FROM user_roles ur
JOIN descendants d ON d.role_id = ur.role_id;

-- name: BumpTokenVersionForUsers :exec
-- Invalidates the RBAC permission cache for these users, which is namespaced by
-- token_version (rbac:perms:<userID>:v<N>) — so the bump IS the invalidation and
-- `Invalidate` never has to be called by hand.
--
-- It is not a logout. Only the ACCESS token stops verifying; the refresh cookie
-- is untouched, so the browser's next silent refresh mints a token at the new
-- version and the user keeps their session with the new permissions applied.
UPDATE users SET token_version = token_version + 1, updated_at = now()
WHERE id = ANY(sqlc.arg('ids')::uuid[]);

-- ── Who can approve a registration ────────────────────────────────

-- name: ListPermissionHoldersByResource :many
-- (user_id, permission code) for every account that could act on `resource`,
-- with the role hierarchy already walked — the same union GetEffectivePermissions
-- computes for one user, done for everybody at once.
--
-- The WHERE is a PREFILTER, not the decision. Matching `users:approve` means
-- honouring the scope and wildcard rules in rbac/permission.go, which is not
-- something to re-implement in SQL; this narrows the rows to codes whose first
-- segment could possibly match, and the caller applies the real matcher.
--
-- Disabled and non-approved accounts are excluded here rather than by the
-- caller: someone who cannot sign in cannot act on a queue, and mailing them
-- would be noise.
WITH RECURSIVE ancestry AS (
    SELECT ur.user_id, ur.role_id
    FROM user_roles ur
    WHERE ur.expires_at IS NULL OR ur.expires_at > now()
    UNION
    SELECT a.user_id, r.parent_id
    FROM roles r
    JOIN ancestry a ON a.role_id = r.id
    WHERE r.parent_id IS NOT NULL
)
SELECT DISTINCT a.user_id, p.code
FROM ancestry a
JOIN role_permissions rp ON rp.role_id = a.role_id
JOIN permissions p       ON p.id = rp.permission_id
JOIN users u             ON u.id = a.user_id
WHERE u.disabled_at IS NULL
  AND u.approval_status = 'approved'
  AND (p.code = '*' OR split_part(p.code, ':', 1) IN (sqlc.arg('resource')::text, '*'));

-- ── Admin-provisioned accounts ────────────────────────────────────

-- name: CreateAdminUser :one
-- Admin-provisioned account. Unlike CreateLocalUser (self-registration) the
-- approval state is decided by the caller: a creator who can approve gets a
-- usable account immediately, one who cannot gets a pending row that still has
-- to go through the queue. Without that the create form would be a way around
-- the approval gate for anyone holding `users:write:any`.
INSERT INTO users (email, display_name, password_hash, password_updated_at, approval_status, approved_at, approved_by)
VALUES (
    sqlc.arg('email'),
    sqlc.arg('display_name'),
    sqlc.narg('password_hash'),
    CASE WHEN sqlc.narg('password_hash')::text IS NULL THEN NULL ELSE now() END,
    sqlc.arg('approval_status')::text,
    CASE WHEN sqlc.arg('approval_status')::text = 'approved' THEN now() ELSE NULL END,
    sqlc.narg('approved_by')
)
RETURNING id;

-- name: UpdateAdminUser :one
-- Profile edit. Email is included because it is the login identifier and the
-- only way to fix a typo that locks somebody out; the UNIQUE index is what
-- rejects a collision, surfaced as 409.
UPDATE users
SET email        = sqlc.arg('email'),
    display_name = sqlc.arg('display_name'),
    updated_at   = now()
WHERE id = sqlc.arg('id')
RETURNING id;

-- name: DeleteUser :execrows
-- HARD delete. Every content table's FK to users is ON DELETE CASCADE, so this
-- also removes the account's assets, comics, movies, tracks, stories, ledger,
-- journal entries, people and organizations. There is no undo and the database
-- will not object — the confirmation lives in the handler, not here.
DELETE FROM users WHERE id = $1;
