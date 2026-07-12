-- journal life-stream projection queries (SPEC-06). Idempotency is structural via
-- the (source_module, event_type, ref_id) unique. Journal rows are written in the
-- entry's own transaction (P0.1a); system rows arrive via the event consumers.

-- name: InsertStreamItem :exec
-- Idempotent insert — redelivery is a no-op (P0.1).
INSERT INTO stream_items (user_id, source_module, event_type, ref_id, payload, occurred_at)
VALUES ($1, $2, $3, $4, COALESCE(sqlc.narg('payload'), '{}'), $5)
ON CONFLICT (source_module, event_type, ref_id) DO NOTHING;

-- name: UpsertStreamItem :exec
-- Insert-or-refresh — a corrected payload/occurred_at must win (bank updated, P0.1).
INSERT INTO stream_items (user_id, source_module, event_type, ref_id, payload, occurred_at)
VALUES ($1, $2, $3, $4, COALESCE(sqlc.narg('payload'), '{}'), $5)
ON CONFLICT (source_module, event_type, ref_id)
DO UPDATE SET payload = EXCLUDED.payload, occurred_at = EXCLUDED.occurred_at;

-- name: UpdateStreamOccurredAt :exec
-- A journal edit moves its stream row to the edited position (P0.1a).
UPDATE stream_items SET occurred_at = $4
WHERE source_module = $1 AND event_type = $2 AND ref_id = $3;

-- name: DeleteStreamItem :exec
DELETE FROM stream_items WHERE source_module = $1 AND event_type = $2 AND ref_id = $3;

-- name: DeleteStreamByRef :exec
-- Delete every row for a ref regardless of event_type (media:asset_deleted, P0.1).
DELETE FROM stream_items WHERE source_module = $1 AND ref_id = $2;

-- name: ListStreamCursor :many
-- Merged timeline. Journal items carry their entry body/mood (LEFT JOIN); system
-- items leave those NULL and render compact from payload (P0.2).
SELECT s.id, s.source_module, s.event_type, s.ref_id, s.payload, s.occurred_at,
       je.body_md, je.mood
FROM stream_items s
LEFT JOIN journal_entries je ON s.source_module = 'journal' AND je.id = s.ref_id
WHERE s.user_id = @user_id
  AND ( @cursor_occurred_at::timestamptz IS NULL
        OR s.occurred_at < @cursor_occurred_at::timestamptz
        OR (s.occurred_at = @cursor_occurred_at::timestamptz AND s.id < @cursor_id::uuid) )
ORDER BY s.occurred_at DESC, s.id DESC
LIMIT @lim::int;
