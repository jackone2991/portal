-- 0028_comic_sync_sources: external-source sync for comics (SPEC-02 P1.8). A sync
-- source binds a comic to an external URL (e.g. a truyenqq comic page). Triggering
-- a sync creates a comic-level comic_imports job (0027) whose zip is produced by the
-- Python scraper service and uploaded to the import/ prefix; the existing
-- comic:import_zip worker then imports it. tenant_id + FORCE RLS mirror the sibling
-- comic tables (0020/0026).
CREATE TABLE comic_sync_sources (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    comic_id       UUID NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    owner_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_url     TEXT NOT NULL,                     -- external comic page URL
    source_site    TEXT NOT NULL DEFAULT '',          -- host, e.g. truyenqqno.com
    chapters_hint  TEXT NOT NULL DEFAULT '',          -- optional explicit chapter list/range (blank = all)
    last_status    TEXT NOT NULL DEFAULT 'idle'
                     CHECK (last_status IN ('idle', 'syncing', 'done', 'failed')),
    last_import_id UUID,                              -- comic_imports job of the most recent sync
    last_error     TEXT,
    last_synced_at TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX comic_sync_sources_comic_idx ON comic_sync_sources (comic_id);
CREATE INDEX comic_sync_sources_owner_idx ON comic_sync_sources (owner_user_id);

-- tenant_id + FORCE RLS (nullable → SET DEFAULT → SET NOT NULL; see 0026).
ALTER TABLE comic_sync_sources ADD COLUMN tenant_id UUID;
ALTER TABLE comic_sync_sources ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE comic_sync_sources ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE comic_sync_sources ADD CONSTRAINT comic_sync_sources_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE comic_sync_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE comic_sync_sources FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON comic_sync_sources
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX comic_sync_sources_tenant_idx ON comic_sync_sources (tenant_id);
