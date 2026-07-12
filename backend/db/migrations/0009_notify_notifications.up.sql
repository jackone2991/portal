-- 0009_notify_notifications: the notify module's tables (SPEC-04 §6) plus its
-- permission-catalog seeds. `user_id` FKs into account's `users(id)` — the one
-- sanctioned cross-module identity-anchor FK (matches 0007_media_assets).

CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT NOT NULL,                 -- open registry, e.g. 'media.asset_ready'
    title      TEXT NOT NULL,
    body       TEXT,
    data       JSONB NOT NULL DEFAULT '{}',   -- target ids, links, aggregation payload
    dedup_key  TEXT,                          -- optional event-derived natural id; idempotency under at-least-once delivery
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_cursor_idx ON notifications (user_id, created_at DESC, id DESC);
CREATE INDEX notifications_unread_idx ON notifications (user_id) WHERE read_at IS NULL;
-- A redelivered dispatch / outbox re-publish is a no-op, not a duplicate row.
CREATE UNIQUE INDEX notifications_dedup_idx ON notifications (user_id, type, dedup_key) WHERE dedup_key IS NOT NULL;

CREATE TABLE notification_preferences (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type    TEXT NOT NULL,
    in_app  BOOLEAN NOT NULL DEFAULT true,
    email   BOOLEAN NOT NULL DEFAULT false,
    push    BOOLEAN NOT NULL DEFAULT false,
    muted   BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, type)               -- absent row = module default
);

CREATE TABLE web_push_subscriptions (        -- P1.1
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint     TEXT NOT NULL UNIQUE,
    p256dh       TEXT NOT NULL,
    auth         TEXT NOT NULL,
    user_agent   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ
);

-- ── seed: notify permission catalog + grants to base `user` (P0.1) ──
-- An unseeded code 403s everyone below superadmin (fail-closed engine) — i.e. a
-- dead bell — so the module ships its own grants, 0003 `WITH grants(...)` style.
INSERT INTO permissions (code, description) VALUES
    ('notifications:read:own',        'Read own notifications'),
    ('notifications:write:own',       'Mark own notifications read'),
    ('notification-prefs:read:own',   'Read own notification preferences'),
    ('notification-prefs:write:own',  'Update own notification preferences'),
    ('push-subscriptions:write:own',  'Register own web-push subscriptions'),
    ('push-subscriptions:delete:own', 'Remove own web-push subscriptions')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES
    ('user', 'notifications:read:own'),
    ('user', 'notifications:write:own'),
    ('user', 'notification-prefs:read:own'),
    ('user', 'notification-prefs:write:own'),
    ('user', 'push-subscriptions:write:own'),
    ('user', 'push-subscriptions:delete:own')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
