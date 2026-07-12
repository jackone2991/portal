-- 0017_journal_stream_items: the life-stream projection (SPEC-06), owned by the
-- journal module (SPEC-05 §6). A polymorphic archive of journal entries + system
-- events, captured durably from day one. No FK on ref_id (cross-module,
-- polymorphic). Idempotency is structural via the (source_module, event_type,
-- ref_id) unique. This migration also BACKFILLS existing journal entries so they
-- don't vanish when the stream replaces the interim home list (P0.1).

CREATE TABLE stream_items (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_module TEXT NOT NULL,               -- 'journal' | 'media' | 'bank' | 'comic' | 'people'
    event_type    TEXT NOT NULL,               -- registry name, e.g. 'media:asset_ready'
    ref_id        UUID NOT NULL,               -- entry / asset / transaction-or-transfer / chapter / notice
    payload       JSONB NOT NULL DEFAULT '{}', -- render-minimum snapshot
    occurred_at   TIMESTAMPTZ NOT NULL,
    UNIQUE (source_module, event_type, ref_id)
);
CREATE INDEX stream_items_user_cursor_idx ON stream_items (user_id, occurred_at DESC, id DESC);

-- ── seed: stream read permission → base `user` role (P0.2 / §7) ──────────
INSERT INTO permissions (code, description) VALUES
    ('stream:read:own', 'Read own life-stream timeline')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES ('user', 'stream:read:own'))
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;

-- ── backfill: existing journal entries → stream rows (P0.1) ──────────────
INSERT INTO stream_items (user_id, source_module, event_type, ref_id, payload, occurred_at)
SELECT user_id, 'journal', 'journal:entry_created', id, '{}'::jsonb, occurred_at
FROM journal_entries
ON CONFLICT (source_module, event_type, ref_id) DO NOTHING;
