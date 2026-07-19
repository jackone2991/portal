-- 0023_story_core: the story vertical — novels with text chapters. Mirrors the
-- comic vertical (parent + ordered children) but chapters carry a markdown body
-- instead of image pages. owner_user_id FKs users(id) (identity anchor).
-- cover_asset_id carries NO cross-module FK (validated via mediaapi, reaped via
-- media:asset_deleted). Tenant-scoped from birth (tenant_id + FORCE RLS).
-- sort_order unique is DEFERRABLE so a full-list reorder rewrites all rows in one tx.

CREATE TABLE stories (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id      UUID NOT NULL DEFAULT current_setting('app.current_tenant')::uuid
                       REFERENCES organizations(id),
    title          TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description    TEXT,
    cover_asset_id UUID,
    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX stories_status_updated_idx ON stories (status, updated_at DESC, id DESC);
CREATE INDEX stories_owner_idx ON stories (owner_user_id);
CREATE INDEX stories_tenant_idx ON stories (tenant_id);

ALTER TABLE stories ENABLE ROW LEVEL SECURITY;
ALTER TABLE stories FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON stories
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

CREATE TABLE story_chapters (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    story_id   UUID NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    tenant_id  UUID NOT NULL DEFAULT current_setting('app.current_tenant')::uuid
                   REFERENCES organizations(id),
    title      TEXT NOT NULL,
    body_md    TEXT NOT NULL DEFAULT '',
    sort_order INT  NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (story_id, sort_order) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX story_chapters_story_idx ON story_chapters (story_id, sort_order);
CREATE INDEX story_chapters_tenant_idx ON story_chapters (tenant_id);

ALTER TABLE story_chapters ENABLE ROW LEVEL SECURITY;
ALTER TABLE story_chapters FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON story_chapters
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

INSERT INTO permissions (code, description) VALUES
    ('stories:read',        'Read published stories'),
    ('stories:write:own',   'Create and edit own stories'),
    ('stories:publish:own', 'Publish and unpublish own stories'),
    ('stories:write:any',   'Edit any story (moderation)'),
    ('stories:publish:any', 'Publish any story (moderation)'),
    ('stories:delete:any',  'Delete any story (moderation)')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES
    ('user',    'stories:read'),
    ('creator', 'stories:write:own'),
    ('creator', 'stories:publish:own'),
    ('editor',  'stories:write:any'),
    ('editor',  'stories:publish:any'),
    ('admin',   'stories:delete:any')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
