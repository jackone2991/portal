-- name: GetUserByOIDCSubject :one
SELECT * FROM users WHERE oidc_subject = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpsertUserFromOIDC :one
INSERT INTO users (oidc_subject, email, display_name, avatar_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (oidc_subject) DO UPDATE
SET email = EXCLUDED.email,
    display_name = EXCLUDED.display_name,
    avatar_url = EXCLUDED.avatar_url,
    updated_at = now()
RETURNING *;
