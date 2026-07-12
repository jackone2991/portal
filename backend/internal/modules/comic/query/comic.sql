-- comic module queries (SPEC-02). sqlc input only — regenerate with `make sqlc`.
-- Owner-scoped mutations; published-or-owner reads. Asset ids are validated via
-- mediaapi (no cross-module FK) and reaped via the media:asset_deleted consumer.

-- ══ Comics ══════════════════════════════════════════════════════════════

-- name: CreateComic :one
INSERT INTO comics (owner_user_id, title, description, cover_asset_id)
VALUES ($1, $2, sqlc.narg('description'), sqlc.narg('cover_asset_id'))
RETURNING *;

-- name: GetComic :one
SELECT * FROM comics WHERE id = $1;

-- name: GetComicOwner :one
SELECT owner_user_id FROM comics WHERE id = $1;

-- name: ListPublishedComics :many
SELECT c.*, (SELECT count(*) FROM comic_chapters ch WHERE ch.comic_id = c.id) AS chapter_count
FROM comics c
WHERE c.status = 'published'
  AND ( @cursor_updated_at::timestamptz IS NULL
        OR c.updated_at < @cursor_updated_at::timestamptz
        OR (c.updated_at = @cursor_updated_at::timestamptz AND c.id < @cursor_id::uuid) )
ORDER BY c.updated_at DESC, c.id DESC
LIMIT @lim::int;

-- name: ListOwnComics :many
SELECT c.*, (SELECT count(*) FROM comic_chapters ch WHERE ch.comic_id = c.id) AS chapter_count
FROM comics c
WHERE c.owner_user_id = @owner_user_id
  AND ( @cursor_updated_at::timestamptz IS NULL
        OR c.updated_at < @cursor_updated_at::timestamptz
        OR (c.updated_at = @cursor_updated_at::timestamptz AND c.id < @cursor_id::uuid) )
ORDER BY c.updated_at DESC, c.id DESC
LIMIT @lim::int;

-- name: UpdateComic :one
UPDATE comics
SET title         = COALESCE(sqlc.narg('title'), title),
    description    = COALESCE(sqlc.narg('description'), description),
    cover_asset_id = CASE WHEN @set_cover::boolean THEN sqlc.narg('cover_asset_id') ELSE cover_asset_id END,
    updated_at     = now()
WHERE id = @id
RETURNING *;

-- name: UpdateComicStatus :one
UPDATE comics SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteComic :exec
DELETE FROM comics WHERE id = $1;

-- ══ Chapters ════════════════════════════════════════════════════════════

-- name: CreateChapter :one
INSERT INTO comic_chapters (comic_id, title, sort_order)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetChapter :one
SELECT * FROM comic_chapters WHERE id = $1;

-- name: ListChaptersByComic :many
SELECT * FROM comic_chapters WHERE comic_id = $1 ORDER BY sort_order, id;

-- name: UpdateChapter :one
UPDATE comic_chapters SET title = COALESCE(sqlc.narg('title'), title) WHERE id = @id RETURNING *;

-- name: UpdateChapterOrder :exec
UPDATE comic_chapters SET sort_order = $2 WHERE id = $1 AND comic_id = $3;

-- name: DeleteChapter :exec
DELETE FROM comic_chapters WHERE id = $1;

-- name: GetComicOwnerByChapter :one
SELECT c.owner_user_id, c.id AS comic_id
FROM comic_chapters ch JOIN comics c ON c.id = ch.comic_id
WHERE ch.id = $1;

-- name: ChaptersWithoutPages :many
-- Chapters of a comic that have zero pages — the publish blocker (P0.2).
SELECT ch.id, ch.title
FROM comic_chapters ch
WHERE ch.comic_id = $1
  AND NOT EXISTS (SELECT 1 FROM comic_pages p WHERE p.chapter_id = ch.id)
ORDER BY ch.sort_order;

-- name: CountChapters :one
SELECT count(*) FROM comic_chapters WHERE comic_id = $1;

-- ══ Pages ═══════════════════════════════════════════════════════════════

-- name: CreatePage :one
INSERT INTO comic_pages (chapter_id, asset_id, sort_order)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPage :one
SELECT * FROM comic_pages WHERE id = $1;

-- name: ListPagesByChapter :many
SELECT * FROM comic_pages WHERE chapter_id = $1 ORDER BY sort_order, id;

-- name: UpdatePageOrder :exec
UPDATE comic_pages SET sort_order = $2 WHERE id = $1 AND chapter_id = $3;

-- name: DeletePage :exec
DELETE FROM comic_pages WHERE id = $1;

-- name: GetComicOwnerByPage :one
SELECT c.owner_user_id, c.id AS comic_id, ch.id AS chapter_id
FROM comic_pages p
JOIN comic_chapters ch ON ch.id = p.chapter_id
JOIN comics c ON c.id = ch.comic_id
WHERE p.id = $1;

-- ══ Reading progress (P0.4) ═════════════════════════════════════════════

-- name: UpsertProgress :exec
INSERT INTO comic_reading_progress (user_id, comic_id, chapter_id, page_id)
VALUES ($1, $2, $3, sqlc.narg('page_id'))
ON CONFLICT (user_id, comic_id)
DO UPDATE SET chapter_id = EXCLUDED.chapter_id, page_id = EXCLUDED.page_id, updated_at = now();

-- name: GetProgress :one
SELECT * FROM comic_reading_progress WHERE user_id = $1 AND comic_id = $2;

-- name: PageChapterAndComic :one
-- Membership validation for a progress write (P0.4): resolve a page's chapter +
-- comic so the service can reject a page/chapter that doesn't belong to the comic.
SELECT ch.id AS chapter_id, ch.comic_id
FROM comic_pages p JOIN comic_chapters ch ON ch.id = p.chapter_id
WHERE p.id = $1;

-- name: ChapterComic :one
SELECT comic_id FROM comic_chapters WHERE id = $1;

-- ══ media:asset_deleted consumer (P0.6) ═════════════════════════════════

-- name: DeletePagesByAsset :exec
DELETE FROM comic_pages WHERE asset_id = $1;

-- name: NullCoverByAsset :exec
UPDATE comics SET cover_asset_id = NULL, updated_at = now() WHERE cover_asset_id = $1;
