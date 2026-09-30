-- journal module queries (SPEC-05). sqlc input only — regenerate with `make sqlc`;
-- never hand-edit the *.sql.go output. Entries are owner-scoped human-authored
-- rows; the timeline orders and paginates on (occurred_at DESC, id DESC) so a
-- backdated entry sits at its date (P0.2), backed by journal_entries_user_cursor_idx.

-- name: CreateEntry :one
-- Create one journal entry. asset_ids is the Entry's Attachments in display
-- order (SPEC-12 T1) — validated as a whole by the service through the media
-- module's public lookup before this runs; an empty array is a text-only Entry.
-- The Location is three all-or-nothing columns (0045, SPEC-12 T3): all NULL for
-- none, all set for one — the service hands over a whole Location or nil, so
-- the CHECK never fires from here. Coordinates arrive as float8 (what sqlc types
-- a *float64 param as) and are assigned into numeric(7,4), which rounds to four
-- places; the RETURNING row carries what was stored. occurred_at is resolved by
-- the service (defaults to now(); backdating/future-dating unlimited).
INSERT INTO journal_entries (user_id, body_md, mood, asset_ids, location_name, location_lat, location_lon, occurred_at)
VALUES (@user_id, @body_md, sqlc.narg('mood'), @asset_ids::uuid[],
        sqlc.narg('location_name')::text, sqlc.narg('location_lat')::float8, sqlc.narg('location_lon')::float8,
        @occurred_at)
RETURNING *;

-- name: GetEntry :one
-- Owner-scoped fetch. A row owned by another user (or a missing id) returns no
-- row → the adapter maps pgx.ErrNoRows to ErrEntryNotFound so the handler answers
-- 404 (existence never leaks).
SELECT * FROM journal_entries WHERE id = $1 AND user_id = $2;

-- name: ListEntriesByUserCursor :many
-- Keyset page for the owner, newest first. A NULL @cursor_occurred_at starts at
-- the top; the (occurred_at, id) keyset is backed by journal_entries_user_cursor_idx.
SELECT * FROM journal_entries
WHERE user_id = @user_id
  AND (
        @cursor_occurred_at::timestamptz IS NULL
        OR occurred_at < @cursor_occurred_at::timestamptz
        OR (occurred_at = @cursor_occurred_at::timestamptz AND id < @cursor_id::uuid)
      )
ORDER BY occurred_at DESC, id DESC
LIMIT @lim::int;

-- name: PatchEntry :one
-- Partial update of any subset of {body_md, mood, asset_ids, location,
-- occurred_at}. A NULL arg leaves the column unchanged (COALESCE), so nil
-- pointers from the service mean "keep". asset_ids REPLACES the whole list when
-- present — an empty array (not NULL) clears it (SPEC-12 T1). The Location
-- and the mood cannot use COALESCE — NULL is how they are CLEARED — so
-- @set_location / @set_mood say whether their args apply at all: false keeps
-- the column, true writes the arg as sent (NULL = clear, a value = replace;
-- SPEC-12 T3, T5). updated_at always advances; occurred_at is only moved when
-- the caller edits it, so an entry keeps its timeline position unless
-- occurred_at itself changed. Owner-scoped; no matching row →
-- ErrEntryNotFound (404, never leaks existence).
UPDATE journal_entries
SET body_md       = COALESCE(sqlc.narg('body_md'), body_md),
    mood          = CASE WHEN @set_mood::bool THEN sqlc.narg('mood')::text ELSE mood END,
    asset_ids     = COALESCE(sqlc.narg('asset_ids')::uuid[], asset_ids),
    location_name = CASE WHEN @set_location::bool THEN sqlc.narg('location_name')::text   ELSE location_name END,
    location_lat  = CASE WHEN @set_location::bool THEN sqlc.narg('location_lat')::float8  ELSE location_lat  END,
    location_lon  = CASE WHEN @set_location::bool THEN sqlc.narg('location_lon')::float8  ELSE location_lon  END,
    occurred_at   = COALESCE(sqlc.narg('occurred_at')::timestamptz, occurred_at),
    updated_at    = now()
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: DeleteEntry :one
-- Owner-scoped delete. RETURNING id yields no row when the id is missing or owned
-- by another user → the handler answers an idempotent 404; a matched row → 204.
DELETE FROM journal_entries
WHERE id = $1 AND user_id = $2
RETURNING id;

-- name: StripAssetFromEntries :execrows
-- media:asset_deleted (SPEC-12 T4): take one Asset out of the Attachments of
-- every Entry of the owner that shows it. array_remove keeps the order of the
-- rest; the WHERE keeps the update to rows that actually carry the id, which
-- is what makes a redelivery a no-op (0 rows). An Entry may end up with '{}'
-- and an empty body — kept, by design (the text-or-Attachment rule is the
-- service's write rule, not a CHECK). Runs inside the owner's tenant scope.
UPDATE journal_entries
SET asset_ids  = array_remove(asset_ids, @asset_id::uuid),
    updated_at = now()
WHERE user_id = @user_id AND @asset_id::uuid = ANY (asset_ids);
