-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
-- Primary login lookup (ADR-06 local auth). Email is UNIQUE.
SELECT * FROM users WHERE email = $1;

-- name: CreateLocalUser :one
-- Registration. password_hash is an Argon2id PHC string; oidc_subject stays NULL.
INSERT INTO users (email, display_name, password_hash, password_updated_at)
VALUES ($1, $2, $3, now())
RETURNING *;

-- name: SetUserPassword :exec
-- Change-password / admin reset. Pair with BumpUserTokenVersion to force a
-- re-login everywhere after a credential change.
UPDATE users
SET password_hash = $2,
    password_updated_at = now(),
    updated_at = now()
WHERE id = $1;

-- name: ListUserDirectory :many
-- Other accounts on this instance: approved, not disabled, not the caller. The
-- roster behind "people you may know". Ids and names only — a directory, not a
-- profile dump.
--
-- approval_status comes from migration 0031: an account nobody has approved yet
-- must not be suggested as someone you might know.
SELECT id, email, display_name
FROM users
WHERE id <> @exclude::uuid
  AND disabled_at IS NULL
  AND approval_status = 'approved'
ORDER BY display_name, id
LIMIT @lim::int;

-- name: GetUsersByIDs :many
-- Batch name lookup for other modules (social resolves the other party in a
-- connection this way, because it may not join `users` itself).
SELECT id, email, display_name FROM users WHERE id = ANY(@ids::uuid[]);
