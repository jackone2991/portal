-- music module queries. sqlc input only. Owner-scoped mutations; published-or-
-- owner reads. Asset ids validated via mediaapi (no cross-module FK), reaped via
-- media:asset_deleted. tenant_id is filled by its column DEFAULT (RequireTenant
-- sets app.current_tenant) — never inserted here.

-- name: CreateTrack :one
INSERT INTO music_tracks (owner_user_id, title, artist, album, description, audio_asset_id, cover_asset_id)
VALUES ($1, $2, sqlc.narg('artist'), sqlc.narg('album'), sqlc.narg('description'), sqlc.narg('audio_asset_id'), sqlc.narg('cover_asset_id'))
RETURNING *;

-- name: GetTrack :one
SELECT * FROM music_tracks WHERE id = $1;

-- name: GetTrackOwner :one
SELECT owner_user_id FROM music_tracks WHERE id = $1;

-- name: ListPublishedTracks :many
SELECT * FROM music_tracks
WHERE status = 'published'
  AND ( @cursor_updated_at::timestamptz IS NULL
        OR updated_at < @cursor_updated_at::timestamptz
        OR (updated_at = @cursor_updated_at::timestamptz AND id < @cursor_id::uuid) )
ORDER BY updated_at DESC, id DESC
LIMIT @lim::int;

-- name: ListOwnTracks :many
SELECT * FROM music_tracks
WHERE owner_user_id = @owner_user_id
  AND ( @cursor_updated_at::timestamptz IS NULL
        OR updated_at < @cursor_updated_at::timestamptz
        OR (updated_at = @cursor_updated_at::timestamptz AND id < @cursor_id::uuid) )
ORDER BY updated_at DESC, id DESC
LIMIT @lim::int;

-- name: UpdateTrack :one
UPDATE music_tracks
SET title          = COALESCE(sqlc.narg('title'), title),
    artist         = CASE WHEN @set_artist::boolean THEN sqlc.narg('artist') ELSE artist END,
    album          = CASE WHEN @set_album::boolean  THEN sqlc.narg('album')  ELSE album  END,
    description    = COALESCE(sqlc.narg('description'), description),
    audio_asset_id = CASE WHEN @set_audio::boolean  THEN sqlc.narg('audio_asset_id') ELSE audio_asset_id END,
    cover_asset_id = CASE WHEN @set_cover::boolean  THEN sqlc.narg('cover_asset_id') ELSE cover_asset_id END,
    updated_at     = now()
WHERE id = @id
RETURNING *;

-- name: UpdateTrackStatus :one
UPDATE music_tracks SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteTrack :exec
DELETE FROM music_tracks WHERE id = $1;

-- ══ media:asset_deleted consumer ══════════════════════════════════════════

-- NullAudioByAsset also unpublishes: a published track with no audio is broken.
-- name: NullAudioByAsset :exec
UPDATE music_tracks SET audio_asset_id = NULL, status = 'draft', updated_at = now() WHERE audio_asset_id = $1;

-- name: NullTrackCoverByAsset :exec
UPDATE music_tracks SET cover_asset_id = NULL, updated_at = now() WHERE cover_asset_id = $1;
