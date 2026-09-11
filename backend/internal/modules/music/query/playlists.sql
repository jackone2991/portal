-- music playlist queries (0041). sqlc input only.
--
-- Every statement is owner-scoped in its own predicate as well as fenced by the
-- 0041 tenant policies: a playlist is one person's selection, and inside a
-- shared org the tenant fence alone would not say so.

-- name: CreatePlaylist :one
-- ON CONFLICT rather than letting the unique raise. A constraint violation
-- aborts the surrounding transaction, and every request runs inside one
-- (RequireTenant) — so the handler would map 23505 to a clean 409 and then the
-- middleware would fail to COMMIT and replace it with a 500. No rows returned
-- means the name is taken.
INSERT INTO music_playlists (owner_user_id, name, description)
VALUES ($1, $2, sqlc.narg('description'))
ON CONFLICT (owner_user_id, lower(btrim(name))) DO NOTHING
RETURNING *;

-- name: PlaylistNameTaken :one
-- Rename cannot use ON CONFLICT, so it looks first. A race would still abort the
-- transaction, but a rename colliding in the microseconds after this check is a
-- different order of rare than typing a name you already used.
SELECT EXISTS (
    SELECT 1 FROM music_playlists
    WHERE owner_user_id = $1 AND lower(btrim(name)) = lower(btrim($2::text)) AND id <> $3
);

-- name: GetPlaylist :one
SELECT * FROM music_playlists WHERE id = $1 AND owner_user_id = $2;

-- name: ListPlaylists :many
-- With the track count, because a playlist list that does not say how big each
-- one is makes you open every one to find out.
SELECT p.*, (SELECT count(*) FROM music_playlist_tracks t WHERE t.playlist_id = p.id) AS track_count
FROM music_playlists p
WHERE p.owner_user_id = $1
ORDER BY p.name, p.id;

-- name: RenamePlaylist :one
UPDATE music_playlists
SET name = COALESCE(sqlc.narg('name'), name),
    description = CASE WHEN @set_description::boolean THEN sqlc.narg('description') ELSE description END,
    updated_at = now()
WHERE id = @id AND owner_user_id = @owner_user_id
RETURNING *;

-- name: DeletePlaylist :one
DELETE FROM music_playlists WHERE id = $1 AND owner_user_id = $2 RETURNING id;

-- name: NextPlaylistPosition :one
-- Append position. Sparse numbering means adding never renumbers what is there.
SELECT COALESCE(max(position), 0) + 1 FROM music_playlist_tracks WHERE playlist_id = $1;

-- name: AddPlaylistTrack :execrows
-- Idempotent: adding a track already in the playlist leaves its position alone,
-- so re-running a bulk add does not shuffle the order. :execrows because the
-- caller reports how many landed — counting attempts instead of insertions
-- would claim "added 3" for a re-run that added nothing.
INSERT INTO music_playlist_tracks (playlist_id, track_id, position)
VALUES ($1, $2, $3)
ON CONFLICT (playlist_id, track_id) DO NOTHING;

-- name: RemovePlaylistTrack :exec
DELETE FROM music_playlist_tracks WHERE playlist_id = $1 AND track_id = $2;

-- name: ListPlaylistTracks :many
-- The tracks of one playlist, in playlist order. Joins music_tracks, which this
-- module owns — not a cross-module join.
SELECT t.*, pt.position
FROM music_playlist_tracks pt
JOIN music_tracks t ON t.id = pt.track_id
WHERE pt.playlist_id = $1
ORDER BY pt.position, t.id;

-- name: OwnedTrackIDs :many
-- Which of these track ids the caller actually owns. A bulk add filters through
-- this first, so a crafted id list cannot pull someone else's track into a
-- playlist even where RLS is inert.
SELECT id FROM music_tracks WHERE owner_user_id = $1 AND id = ANY(@ids::uuid[]);
