-- 0021_movie_core: the movie vertical — a single-video domain over media's video
-- assets. Mirrors the comic reference vertical (SPEC-02) but slimmer: a movie IS
-- one video (no chapters/pages/progress). owner_user_id FKs users(id) (identity
-- anchor, 0007/0015 precedent). video_asset_id / poster_asset_id carry NO
-- cross-module FK — validated via mediaapi at write time, reaped via the
-- media:asset_deleted event. Tenant-scoped from birth (ADR-07 convention now in
-- force): tenant_id + FORCE RLS in this migration; the table is new so no backfill.

CREATE TABLE movies (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL DEFAULT current_setting('app.current_tenant')::uuid
                        REFERENCES organizations(id),
    title           TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description     TEXT,
    video_asset_id  UUID,                    -- media video asset (validated via mediaapi)
    poster_asset_id UUID,                    -- optional media image asset (cover)
    release_year    INT CHECK (release_year IS NULL OR release_year BETWEEN 1880 AND 2200),
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX movies_status_updated_idx ON movies (status, updated_at DESC, id DESC);
CREATE INDEX movies_owner_idx ON movies (owner_user_id);
CREATE INDEX movies_tenant_idx ON movies (tenant_id);

ALTER TABLE movies ENABLE ROW LEVEL SECURITY;
ALTER TABLE movies FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON movies
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

-- ── seed: movie permission catalog + grants (0003 / 0015 pattern) ─────────
INSERT INTO permissions (code, description) VALUES
    ('movies:read',        'Read published movies'),
    ('movies:write:own',   'Create and edit own movies'),
    ('movies:publish:own', 'Publish and unpublish own movies'),
    ('movies:write:any',   'Edit any movie (moderation)'),
    ('movies:publish:any', 'Publish any movie (moderation)'),
    ('movies:delete:any',  'Delete any movie (moderation)')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES
    ('user',    'movies:read'),
    ('creator', 'movies:write:own'),
    ('creator', 'movies:publish:own'),
    ('editor',  'movies:write:any'),
    ('editor',  'movies:publish:any'),
    ('admin',   'movies:delete:any')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
