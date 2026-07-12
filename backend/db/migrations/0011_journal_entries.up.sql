-- 0011_journal_entries: the journal module's entries table (SPEC-05 §6) + its
-- permission seeds. user_id FKs into account's users(id) — the sanctioned
-- identity-anchor exception (matches 0007/0009). The `stream_items` projection
-- table is SPEC-06's (a later migration); this migration ships only entries.

CREATE TABLE journal_entries (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body_md     TEXT NOT NULL CHECK (char_length(body_md) BETWEEN 1 AND 20000),
    mood        TEXT CHECK (mood IS NULL OR char_length(mood) BETWEEN 1 AND 80),
    asset_ids   UUID[] NOT NULL DEFAULT '{}',        -- media assets, validated via mediaapi (P1.5); no FK (cross-module)
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),  -- user-editable ("last night"); timeline sorts on this
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),  -- audit only
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX journal_entries_user_cursor_idx ON journal_entries (user_id, occurred_at DESC, id DESC);

-- ── seed: journal permission catalog + grants to base `user` (P0.2 / §7) ──
INSERT INTO permissions (code, description) VALUES
    ('journal:read:own',   'Read own journal entries'),
    ('journal:write:own',  'Create + edit own journal entries'),
    ('journal:delete:own', 'Delete own journal entries')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES
    ('user', 'journal:read:own'),
    ('user', 'journal:write:own'),
    ('user', 'journal:delete:own')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
