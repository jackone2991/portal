-- 0015_comic_core: the comic vertical (SPEC-02) — the reference media→domain
-- pattern. owner_user_id / user_id FK into users(id) (identity-anchor exception,
-- 0007 precedent). Asset references (cover_asset_id, comic_pages.asset_id) carry
-- NO cross-module FK — they're validated via mediaapi at write time and reaped
-- via the media:asset_deleted event (P0.6). Sort-order uniques are DEFERRABLE so
-- a full-list reorder can rewrite every row in one tx without tripping per-row.

CREATE TABLE comics (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title          TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description    TEXT,
    cover_asset_id UUID,                    -- media asset, validated via mediaapi
    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX comics_status_updated_idx ON comics (status, updated_at DESC, id DESC);
CREATE INDEX comics_owner_idx ON comics (owner_user_id);

CREATE TABLE comic_chapters (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    comic_id   UUID NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    sort_order INT  NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (comic_id, sort_order) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX comic_chapters_comic_idx ON comic_chapters (comic_id, sort_order);

CREATE TABLE comic_pages (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    chapter_id UUID NOT NULL REFERENCES comic_chapters(id) ON DELETE CASCADE,
    asset_id   UUID NOT NULL UNIQUE,        -- media asset (validated via mediaapi); one page max per asset
    sort_order INT  NOT NULL,
    UNIQUE (chapter_id, sort_order) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX comic_pages_chapter_idx ON comic_pages (chapter_id, sort_order);

CREATE TABLE comic_reading_progress (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comic_id   UUID NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    chapter_id UUID NOT NULL,               -- resume anchor; may dangle → fall back to first chapter
    page_id    UUID REFERENCES comic_pages(id) ON DELETE SET NULL, -- exact page; NULL after delete → chapter top
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, comic_id)
);

-- ── seed: comic permission catalog + grants (P0.2 / §7, 0003 pattern) ─────
INSERT INTO permissions (code, description) VALUES
    ('comics:read',         'Read published comics'),
    ('comics:write:own',    'Create and edit own comics'),
    ('comics:publish:own',  'Publish and unpublish own comics'),
    ('comics:write:any',    'Edit any comic (moderation)'),
    ('comics:publish:any',  'Publish any comic (moderation)'),
    ('comics:delete:any',   'Delete any comic or page (moderation)')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES
    ('user',    'comics:read'),
    ('creator', 'comics:write:own'),
    ('creator', 'comics:publish:own'),
    ('editor',  'comics:write:any'),
    ('editor',  'comics:publish:any'),
    ('admin',   'comics:delete:any')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
