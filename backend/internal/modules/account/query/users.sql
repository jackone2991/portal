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
