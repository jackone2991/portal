-- notify module queries (SPEC-04). sqlc input only — regenerate with `make sqlc`;
-- never hand-edit the *.sql.go output.

-- name: InsertNotification :one
-- Insert an in-app notification. When @dedup_key is present the partial unique
-- index (user_id, type, dedup_key) WHERE dedup_key IS NOT NULL makes a redelivered
-- dispatch a no-op: DO NOTHING returns zero rows, which the adapter reads as
-- "conflict" (inserted=false) so channel fan-out is skipped too. A NULL dedup_key
-- never conflicts (the index excludes it) and always inserts.
INSERT INTO notifications (user_id, type, title, body, data, dedup_key)
VALUES ($1, $2, $3, sqlc.narg('body'), $4, sqlc.narg('dedup_key'))
ON CONFLICT (user_id, type, dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: ListNotifications :many
-- Keyset page, newest first. @unread_only restricts to unread rows (the bell's
-- ?status=unread). A NULL @cursor_created_at starts at the top; the (created_at,
-- id) keyset is backed by notifications_user_cursor_idx.
SELECT * FROM notifications
WHERE user_id = @user_id
  AND (NOT @unread_only::boolean OR read_at IS NULL)
  AND (
        @cursor_created_at::timestamptz IS NULL
        OR created_at < @cursor_created_at::timestamptz
        OR (created_at = @cursor_created_at::timestamptz AND id < @cursor_id::uuid)
      )
ORDER BY created_at DESC, id DESC
LIMIT @lim::int;

-- name: CountUnreadNotifications :one
-- Unread badge; backed by the partial notifications_unread_idx.
SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL;

-- name: MarkNotificationRead :one
-- Idempotent, owner-scoped. COALESCE keeps the first read_at stamp so a second
-- call is a no-op. RETURNING id yields no row when the id is missing or belongs
-- to another user → the handler answers 404 (never leaks existence).
UPDATE notifications
SET read_at = COALESCE(read_at, now())
WHERE id = $1 AND user_id = $2
RETURNING id;

-- name: MarkAllNotificationsRead :exec
-- Read-all with an optional 'before' watermark: only rows at/older than the
-- newest rendered item (@before_created_at, @before_id) are marked, so rows that
-- arrived after the dropdown rendered stay unread. A NULL watermark marks all.
UPDATE notifications
SET read_at = now()
WHERE user_id = @user_id
  AND read_at IS NULL
  AND (
        @before_created_at::timestamptz IS NULL
        OR created_at < @before_created_at::timestamptz
        OR (created_at = @before_created_at::timestamptz AND id <= @before_id::uuid)
      );

-- name: GetNotificationPreference :one
-- Per-type preference; an absent row means the module default (§6).
SELECT in_app, email, push, muted
FROM notification_preferences
WHERE user_id = $1 AND type = $2;

-- name: ListNotificationPreferences :many
SELECT type, in_app, email, push, muted
FROM notification_preferences
WHERE user_id = $1
ORDER BY type;

-- name: UpsertNotificationPreference :exec
-- P1.3 preferences write.
INSERT INTO notification_preferences (user_id, type, in_app, email, push, muted)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id, type) DO UPDATE
SET in_app = EXCLUDED.in_app,
    email  = EXCLUDED.email,
    push   = EXCLUDED.push,
    muted  = EXCLUDED.muted;

-- name: PurgeOldNotifications :exec
-- P2 retention janitor: read > 90d and unread > 180d, exempting the non-mutable
-- security type. Batched deletes are a future refinement (§P2).
DELETE FROM notifications
WHERE (
        (read_at IS NOT NULL AND created_at < now() - INTERVAL '90 days')
     OR (read_at IS NULL     AND created_at < now() - INTERVAL '180 days')
      )
  AND type <> 'account.security_alert';
