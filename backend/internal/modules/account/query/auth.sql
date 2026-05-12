-- ── User lifecycle (auth-related) ─────────────────────────────────

-- name: BumpUserTokenVersion :one
-- Invalidates all outstanding access tokens for this user (forced logout-all).
-- Refresh tokens are NOT touched here; revoke them separately when needed.
UPDATE users
SET token_version = token_version + 1,
    updated_at = now()
WHERE id = $1
RETURNING token_version;

-- name: DisableUser :exec
UPDATE users
SET disabled_at = now(),
    token_version = token_version + 1,
    updated_at = now()
WHERE id = $1;

-- name: EnableUser :exec
UPDATE users
SET disabled_at = NULL,
    updated_at = now()
WHERE id = $1;

-- name: GetUserAuthSnapshot :one
-- Minimal projection used by JWT middleware on each request to validate
-- token_version + disabled state. Indexed PK lookup.
SELECT id, email, display_name, role, token_version, disabled_at
FROM users
WHERE id = $1;

-- ── Refresh tokens ────────────────────────────────────────────────

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
    user_id, token_hash, expires_at, parent_id, issued_ip, issued_user_agent
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1;

-- name: MarkRefreshTokenReplaced :exec
UPDATE refresh_tokens
SET revoked_at = now(),
    revoke_reason = 'rotated',
    replaced_by_id = $2
WHERE id = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = COALESCE(revoked_at, now()),
    revoke_reason = COALESCE(revoke_reason, $2)
WHERE id = $1;

-- name: RevokeRefreshTokenChain :exec
-- Walks the rotation chain forward and backward from the given token,
-- revoking every link. Used on suspected token theft (reuse detection).
WITH RECURSIVE
  forward AS (
    SELECT id FROM refresh_tokens WHERE id = $1
    UNION
    SELECT rt.id FROM refresh_tokens rt
    JOIN forward f ON rt.parent_id = f.id
  ),
  backward AS (
    SELECT id, parent_id FROM refresh_tokens WHERE id = $1
    UNION
    SELECT rt.id, rt.parent_id FROM refresh_tokens rt
    JOIN backward b ON b.parent_id = rt.id
  ),
  chain AS (
    SELECT id FROM forward
    UNION
    SELECT id FROM backward
  )
UPDATE refresh_tokens
SET revoked_at = COALESCE(revoked_at, now()),
    revoke_reason = COALESCE(revoke_reason, $2)
WHERE id IN (SELECT id FROM chain);

-- name: RevokeAllRefreshTokensForUser :exec
UPDATE refresh_tokens
SET revoked_at = now(),
    revoke_reason = $2
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: ListActiveRefreshTokensForUser :many
SELECT id, expires_at, created_at, issued_ip, issued_user_agent
FROM refresh_tokens
WHERE user_id = $1
  AND revoked_at IS NULL
  AND expires_at > now()
ORDER BY created_at DESC;

-- name: PurgeExpiredRefreshTokens :exec
-- Run from a periodic job. Anything past expiry + grace can be hard-deleted.
DELETE FROM refresh_tokens
WHERE expires_at < now() - INTERVAL '30 days';

-- ── Audit log ─────────────────────────────────────────────────────

-- name: WriteAuditEvent :exec
INSERT INTO audit_log (
    actor_id, actor_kind, action, target_kind, target_id, metadata, ip, user_agent
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListAuditEvents :many
SELECT * FROM audit_log
WHERE ($1::uuid IS NULL OR actor_id = $1)
  AND ($2::text IS NULL OR action = $2)
ORDER BY occurred_at DESC
LIMIT $3 OFFSET $4;
