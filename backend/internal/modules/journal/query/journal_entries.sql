-- journal module queries (SPEC-05). sqlc input only — regenerate with `make sqlc`;
-- never hand-edit the *.sql.go output. Entries are owner-scoped human-authored
-- rows; the timeline orders and paginates on (occurred_at DESC, id DESC) so a
-- backdated entry sits at its date (P0.2), backed by journal_entries_user_cursor_idx.

-- name: CreateEntry :one
-- Create one journal entry. asset_ids is intentionally NOT set — it stays at the
-- table default '{}' until P1.5 attachments land (the service rejects any
-- asset_ids in the request before this query runs). occurred_at is resolved by
-- the service (defaults to now(); backdating/future-dating unlimited).
INSERT INTO journal_entries (user_id, body_md, mood, occurred_at)
VALUES ($1, $2, sqlc.narg('mood'), $3)
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
-- Partial update of any subset of {body_md, mood, occurred_at}. A NULL arg leaves
-- the column unchanged (COALESCE), so nil pointers from the service mean "keep".
-- updated_at always advances; occurred_at is only moved when the caller edits it,
-- so an entry keeps its timeline position unless occurred_at itself changed.
-- Owner-scoped; no matching row → ErrEntryNotFound (404, never leaks existence).
UPDATE journal_entries
SET body_md     = COALESCE(sqlc.narg('body_md'), body_md),
    mood        = COALESCE(sqlc.narg('mood'), mood),
    occurred_at = COALESCE(sqlc.narg('occurred_at')::timestamptz, occurred_at),
    updated_at  = now()
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: DeleteEntry :one
-- Owner-scoped delete. RETURNING id yields no row when the id is missing or owned
-- by another user → the handler answers an idempotent 404; a matched row → 204.
DELETE FROM journal_entries
WHERE id = $1 AND user_id = $2
RETURNING id;
