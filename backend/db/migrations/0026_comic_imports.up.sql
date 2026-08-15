-- 0026_comic_imports: zip-chapter import jobs (SPEC-02 P1.7). Persisted so the
-- client can poll status + the per-file report. The zip lands in object storage
-- (import/ prefix) and a `comic:import_zip` worker task unpacks it, ingests each
-- image via mediaapi (origin='import'), polls the assets to `ready`, then creates
-- the pages. tenant_id + FORCE RLS mirror the sibling comic tables (0020).
CREATE TABLE comic_imports (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    comic_id      UUID NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
    chapter_id    UUID NOT NULL REFERENCES comic_chapters(id) ON DELETE CASCADE,
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status        TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'uploaded', 'processing', 'done', 'failed')),
    upload_ref    TEXT,                              -- storage key of the uploaded zip
    total         INT  NOT NULL DEFAULT 0,           -- image entries found in the zip
    succeeded     INT  NOT NULL DEFAULT 0,           -- pages created
    failed        INT  NOT NULL DEFAULT 0,           -- entries that failed
    report        JSONB NOT NULL DEFAULT '[]'::jsonb, -- [{name, ok, error?}]
    error         TEXT,                              -- job-level failure
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX comic_imports_chapter_idx ON comic_imports (chapter_id);
CREATE INDEX comic_imports_owner_idx   ON comic_imports (owner_user_id);

-- tenant_id + FORCE RLS (uniform policy, mirrors 0020). Add nullable → SET DEFAULT
-- → SET NOT NULL in separate steps: a single `ADD COLUMN NOT NULL DEFAULT
-- current_setting(...)` evaluates the GUC at ALTER time (fast-default) and errors
-- when it is unset (as during a migration).
ALTER TABLE comic_imports ADD COLUMN tenant_id UUID;
ALTER TABLE comic_imports ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE comic_imports ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE comic_imports ADD CONSTRAINT comic_imports_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE comic_imports ENABLE ROW LEVEL SECURITY;
ALTER TABLE comic_imports FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON comic_imports
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX comic_imports_tenant_idx ON comic_imports (tenant_id);
