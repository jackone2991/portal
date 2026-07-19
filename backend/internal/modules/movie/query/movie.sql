-- movie module queries (first domain vertical). sqlc input only — regenerate with
-- `make sqlc`. Owner-scoped mutations; published-or-owner reads. Asset ids are
-- validated via mediaapi (no cross-module FK) and reaped via the
-- media:asset_deleted consumer. tenant_id is filled by its column DEFAULT
-- (RequireTenant sets app.current_tenant per request) — never inserted here.

-- name: CreateMovie :one
INSERT INTO movies (owner_user_id, title, description, video_asset_id, poster_asset_id, release_year)
VALUES ($1, $2, sqlc.narg('description'), sqlc.narg('video_asset_id'), sqlc.narg('poster_asset_id'), sqlc.narg('release_year'))
RETURNING *;

-- name: GetMovie :one
SELECT * FROM movies WHERE id = $1;

-- name: GetMovieOwner :one
SELECT owner_user_id FROM movies WHERE id = $1;

-- name: ListPublishedMovies :many
SELECT * FROM movies
WHERE status = 'published'
  AND ( @cursor_updated_at::timestamptz IS NULL
        OR updated_at < @cursor_updated_at::timestamptz
        OR (updated_at = @cursor_updated_at::timestamptz AND id < @cursor_id::uuid) )
ORDER BY updated_at DESC, id DESC
LIMIT @lim::int;

-- name: ListOwnMovies :many
SELECT * FROM movies
WHERE owner_user_id = @owner_user_id
  AND ( @cursor_updated_at::timestamptz IS NULL
        OR updated_at < @cursor_updated_at::timestamptz
        OR (updated_at = @cursor_updated_at::timestamptz AND id < @cursor_id::uuid) )
ORDER BY updated_at DESC, id DESC
LIMIT @lim::int;

-- name: UpdateMovie :one
UPDATE movies
SET title           = COALESCE(sqlc.narg('title'), title),
    description      = COALESCE(sqlc.narg('description'), description),
    video_asset_id   = CASE WHEN @set_video::boolean  THEN sqlc.narg('video_asset_id')  ELSE video_asset_id  END,
    poster_asset_id  = CASE WHEN @set_poster::boolean THEN sqlc.narg('poster_asset_id') ELSE poster_asset_id END,
    release_year     = COALESCE(sqlc.narg('release_year'), release_year),
    updated_at       = now()
WHERE id = @id
RETURNING *;

-- name: UpdateMovieStatus :one
UPDATE movies SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteMovie :exec
DELETE FROM movies WHERE id = $1;

-- ══ media:asset_deleted consumer ══════════════════════════════════════════

-- NullVideoByAsset also unpublishes: a published movie with no video is broken.
-- name: NullVideoByAsset :exec
UPDATE movies SET video_asset_id = NULL, status = 'draft', updated_at = now() WHERE video_asset_id = $1;

-- name: NullPosterByAsset :exec
UPDATE movies SET poster_asset_id = NULL, updated_at = now() WHERE poster_asset_id = $1;
