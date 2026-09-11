-- 0038_music_imports: bulk track import jobs.
-- Owning module: music.
--
-- Mirrors comic_imports (0026) — same job shape, same poll-a-report contract —
-- with two differences that come from the medium, not from taste:
--
--   * No parent row. A comic zip belongs to a chapter; a music zip belongs to
--     nobody, it CREATES the tracks. So the job hangs off the owner alone.
--   * No poll-to-ready loop in the worker. Audio assets are marked ready inside
--     `/complete` (there is no transcode for audio — see media.completeAudio), so
--     an imported track is playable the moment it is created. The comic import
--     spends most of its code waiting for images to transcode; this one does not
--     need any of that.
CREATE TABLE music_imports (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status        TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'uploaded', 'processing', 'done', 'failed')),
    upload_ref    TEXT,                               -- storage key of the uploaded zip
    total         INT  NOT NULL DEFAULT 0,            -- audio entries found in the zip
    succeeded     INT  NOT NULL DEFAULT 0,            -- tracks created
    failed        INT  NOT NULL DEFAULT 0,            -- entries that failed
    report        JSONB NOT NULL DEFAULT '[]'::jsonb, -- [{name, ok, track_id?, title?, error?}]
    error         TEXT,                               -- job-level failure
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX music_imports_owner_idx ON music_imports (owner_user_id, created_at DESC);

-- tenant_id + FORCE RLS, uniform with every other tenant-scoped table (0020).
-- Added in three steps on purpose: a single `ADD COLUMN NOT NULL DEFAULT
-- current_setting(...)` evaluates the GUC at ALTER time and fails when it is
-- unset, which is exactly the case during a migration.
ALTER TABLE music_imports ADD COLUMN tenant_id UUID;
ALTER TABLE music_imports ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE music_imports ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE music_imports ADD CONSTRAINT music_imports_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE music_imports ENABLE ROW LEVEL SECURITY;
ALTER TABLE music_imports FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON music_imports
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX music_imports_tenant_idx ON music_imports (tenant_id);
