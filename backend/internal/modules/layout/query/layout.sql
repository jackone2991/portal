-- The shell's navigation and dashboard layout. Read by every authenticated
-- request that renders the app frame; written only from the admin console.

-- name: ListMenuItems :many
-- Everything, hidden rows included. The caller decides what to show: the admin
-- console wants the full list, the shell filters it. Doing the visible/permission
-- filtering here would mean two near-identical queries drifting apart.
SELECT id, key, label, icon, href, permission, position, visible, is_system
FROM layout_menu_items
ORDER BY position, id;

-- name: ListWidgets :many
SELECT id, key, label, slot, permission, position, visible
FROM layout_widgets
ORDER BY slot, position, id;

-- name: UpsertMenuItem :exec
-- One row of a whole-set save. `key` is the conflict target rather than `id`
-- because a newly added row has no id yet on the client, and a stable key is
-- what makes "add" and "edit" the same statement.
INSERT INTO layout_menu_items (key, label, icon, href, permission, position, visible)
VALUES (
    sqlc.arg('key'),
    sqlc.arg('label'),
    sqlc.arg('icon'),
    sqlc.narg('href'),
    sqlc.narg('permission'),
    sqlc.arg('position'),
    sqlc.arg('visible')
)
ON CONFLICT (key) DO UPDATE
SET label      = EXCLUDED.label,
    icon       = EXCLUDED.icon,
    href       = EXCLUDED.href,
    permission = EXCLUDED.permission,
    position   = EXCLUDED.position,
    visible    = EXCLUDED.visible,
    updated_at = now();

-- name: DeleteMenuItemsExcept :exec
-- The other half of a whole-set save: anything the client did not send is gone.
-- System rows are exempt — they are the shell's own navigation, and a client
-- that omits them (a stale tab, a bad payload) must not be able to empty the
-- menu into a state nobody can fix from inside the UI.
DELETE FROM layout_menu_items
WHERE is_system = false
  AND NOT (key = ANY(sqlc.arg('keys')::text[]));

-- name: UpdateWidget :exec
-- Widgets are never created or deleted here: `key` has to match a React
-- component, so the catalogue is fixed by the frontend registry and only
-- placement is editable.
UPDATE layout_widgets
SET label      = sqlc.arg('label'),
    slot       = sqlc.arg('slot'),
    permission = sqlc.narg('permission'),
    position   = sqlc.arg('position'),
    visible    = sqlc.arg('visible'),
    updated_at = now()
WHERE key = sqlc.arg('key');
