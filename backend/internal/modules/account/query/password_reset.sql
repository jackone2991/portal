-- Password-reset tokens (SPEC-04 P0.3). Account-owned; mirrors the refresh-token
-- construction (ADR-06): ≥256-bit CSPRNG raw token, SHA-256 hash at rest, looked
-- up by hash in constant time, single-use, short TTL.

-- name: CreatePasswordResetToken :one
INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPasswordResetTokenByHash :one
SELECT * FROM password_reset_tokens WHERE token_hash = $1;

-- name: MarkPasswordResetTokenUsed :exec
-- Idempotent consume: COALESCE keeps the first used_at stamp.
UPDATE password_reset_tokens
SET used_at = COALESCE(used_at, now())
WHERE id = $1;

-- name: PurgeExpiredPasswordResetTokens :exec
-- Periodic hygiene: drop tokens well past expiry.
DELETE FROM password_reset_tokens
WHERE expires_at < now() - INTERVAL '7 days';
