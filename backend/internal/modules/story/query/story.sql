-- story module queries. sqlc input only. Owner-scoped mutations; published-or-
-- owner reads. cover_asset_id validated via mediaapi (no FK), reaped via
-- media:asset_deleted. tenant_id filled by column DEFAULT (RequireTenant sets
-- app.current_tenant) — never inserted here.

-- ══ Stories ══════════════════════════════════════════════════════════════

-- name: CreateStory :one
INSERT INTO stories (owner_user_id, title, description, cover_asset_id)
VALUES ($1, $2, sqlc.narg('description'), sqlc.narg('cover_asset_id'))
RETURNING *;

-- name: GetStory :one
SELECT * FROM stories WHERE id = $1;

-- name: GetStoryOwner :one
SELECT owner_user_id FROM stories WHERE id = $1;

-- name: ListPublishedStories :many
SELECT s.*, (SELECT count(*) FROM story_chapters ch WHERE ch.story_id = s.id) AS chapter_count
FROM stories s
WHERE s.status = 'published'
  AND ( @cursor_updated_at::timestamptz IS NULL
        OR s.updated_at < @cursor_updated_at::timestamptz
        OR (s.updated_at = @cursor_updated_at::timestamptz AND s.id < @cursor_id::uuid) )
ORDER BY s.updated_at DESC, s.id DESC
LIMIT @lim::int;

-- name: ListOwnStories :many
SELECT s.*, (SELECT count(*) FROM story_chapters ch WHERE ch.story_id = s.id) AS chapter_count
FROM stories s
WHERE s.owner_user_id = @owner_user_id
  AND ( @cursor_updated_at::timestamptz IS NULL
        OR s.updated_at < @cursor_updated_at::timestamptz
        OR (s.updated_at = @cursor_updated_at::timestamptz AND s.id < @cursor_id::uuid) )
ORDER BY s.updated_at DESC, s.id DESC
LIMIT @lim::int;

-- name: UpdateStory :one
UPDATE stories
SET title          = COALESCE(sqlc.narg('title'), title),
    description     = COALESCE(sqlc.narg('description'), description),
    cover_asset_id  = CASE WHEN @set_cover::boolean THEN sqlc.narg('cover_asset_id') ELSE cover_asset_id END,
    updated_at      = now()
WHERE id = @id
RETURNING *;

-- name: UpdateStoryStatus :one
UPDATE stories SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteStory :exec
DELETE FROM stories WHERE id = $1;

-- name: CountStoryChapters :one
SELECT count(*) FROM story_chapters WHERE story_id = $1;

-- EmptyChapters lists chapters with a blank body — the publish blocker.
-- name: EmptyChapters :many
SELECT id, title FROM story_chapters
WHERE story_id = $1 AND char_length(btrim(body_md)) = 0
ORDER BY sort_order;

-- name: NullStoryCoverByAsset :exec
UPDATE stories SET cover_asset_id = NULL, updated_at = now() WHERE cover_asset_id = $1;

-- ══ Chapters ═══════════════════════════════════════════════════════════════

-- name: CreateStoryChapter :one
INSERT INTO story_chapters (story_id, title, body_md, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetStoryChapter :one
SELECT * FROM story_chapters WHERE id = $1;

-- name: ListStoryChapters :many
SELECT * FROM story_chapters WHERE story_id = $1 ORDER BY sort_order, id;

-- name: UpdateStoryChapter :one
UPDATE story_chapters
SET title   = COALESCE(sqlc.narg('title'), title),
    body_md = COALESCE(sqlc.narg('body_md'), body_md),
    updated_at = now()
WHERE id = @id
RETURNING *;

-- name: UpdateStoryChapterOrder :exec
UPDATE story_chapters SET sort_order = $2 WHERE id = $1 AND story_id = $3;

-- name: DeleteStoryChapter :exec
DELETE FROM story_chapters WHERE id = $1;

-- name: GetStoryOwnerByChapter :one
SELECT s.owner_user_id, s.id AS story_id
FROM story_chapters ch JOIN stories s ON s.id = ch.story_id
WHERE ch.id = $1;
