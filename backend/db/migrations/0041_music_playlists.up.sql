-- 0041_music_playlists: playlists, and the tracks in them.
--
-- The left menu has said "Music & Playlists" since the Olympus port; this is the
-- second half of that finally existing. A playlist is one owner's ordered
-- selection of their own tracks — not a shared or collaborative object, which
-- would need the social layer and a visibility model of its own.
--
-- Ownership sits on the playlist. A row in music_playlist_tracks is reachable
-- only through its playlist, so it carries no owner of its own; the tenant fence
-- and the playlist FK are what keep it honest.

CREATE TABLE music_playlists (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    description   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One name per owner: two playlists called "Chill" are a mistake every time, and
-- the error is far kinder than the duplicate.
CREATE UNIQUE INDEX music_playlists_owner_name_idx
    ON music_playlists (owner_user_id, lower(btrim(name)));

CREATE TABLE music_playlist_tracks (
    playlist_id UUID NOT NULL REFERENCES music_playlists(id) ON DELETE CASCADE,
    track_id    UUID NOT NULL REFERENCES music_tracks(id) ON DELETE CASCADE,
    -- Sparse on purpose: appending is `max(position) + 1`, so adding 200 tracks
    -- never renumbers the ones already there.
    position    INTEGER NOT NULL,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (playlist_id, track_id)
);

CREATE INDEX music_playlist_tracks_order_idx ON music_playlist_tracks (playlist_id, position);
CREATE INDEX music_playlist_tracks_track_idx ON music_playlist_tracks (track_id);

-- ── tenant_id + FORCE RLS, uniform with every other tenant-scoped table (0020) ──
ALTER TABLE music_playlists ADD COLUMN tenant_id UUID;
ALTER TABLE music_playlists ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE music_playlists ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE music_playlists ADD CONSTRAINT music_playlists_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE music_playlists ENABLE ROW LEVEL SECURITY;
ALTER TABLE music_playlists FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON music_playlists
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX music_playlists_tenant_idx ON music_playlists (tenant_id);

ALTER TABLE music_playlist_tracks ADD COLUMN tenant_id UUID;
ALTER TABLE music_playlist_tracks ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE music_playlist_tracks ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE music_playlist_tracks ADD CONSTRAINT music_playlist_tracks_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE music_playlist_tracks ENABLE ROW LEVEL SECURITY;
ALTER TABLE music_playlist_tracks FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON music_playlist_tracks
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX music_playlist_tracks_tenant_idx ON music_playlist_tracks (tenant_id);
