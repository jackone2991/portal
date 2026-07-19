-- 0022_music_core: the music vertical — tracks over media's audio assets. Mirrors
-- the movie vertical (a single-asset domain), + artist/album metadata.
-- owner_user_id FKs users(id) (identity anchor). audio_asset_id / cover_asset_id
-- carry NO cross-module FK — validated via mediaapi, reaped via media:asset_deleted.
-- Tenant-scoped from birth (ADR-07): tenant_id + FORCE RLS.

CREATE TABLE music_tracks (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id      UUID NOT NULL DEFAULT current_setting('app.current_tenant')::uuid
                       REFERENCES organizations(id),
    title          TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    artist         TEXT,
    album          TEXT,
    description    TEXT,
    audio_asset_id UUID,                     -- media audio asset (validated via mediaapi)
    cover_asset_id UUID,                     -- optional media image asset
    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX music_tracks_status_updated_idx ON music_tracks (status, updated_at DESC, id DESC);
CREATE INDEX music_tracks_owner_idx ON music_tracks (owner_user_id);
CREATE INDEX music_tracks_tenant_idx ON music_tracks (tenant_id);

ALTER TABLE music_tracks ENABLE ROW LEVEL SECURITY;
ALTER TABLE music_tracks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON music_tracks
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

INSERT INTO permissions (code, description) VALUES
    ('music:read',        'Read published tracks'),
    ('music:write:own',   'Create and edit own tracks'),
    ('music:publish:own', 'Publish and unpublish own tracks'),
    ('music:write:any',   'Edit any track (moderation)'),
    ('music:publish:any', 'Publish any track (moderation)'),
    ('music:delete:any',  'Delete any track (moderation)')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES
    ('user',    'music:read'),
    ('creator', 'music:write:own'),
    ('creator', 'music:publish:own'),
    ('editor',  'music:write:any'),
    ('editor',  'music:publish:any'),
    ('admin',   'music:delete:any')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
