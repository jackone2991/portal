-- 0039_music_metadata_lookup: fields an audio file cannot tell you, plus the
-- record of having asked. Owning module: music.
--
-- 0038's enrichment reads what is INSIDE the file. This is the other half: the
-- release year, the genre and a proper album cover live in a catalogue, not in
-- an mp3, and getting them means asking MusicBrainz.
--
-- That is a different kind of dependency from anything else in this codebase —
-- an outbound call to a third party, rate-limited to 1 req/s, that leaks what is
-- in the library to whoever answers it. So the lookup is off unless an operator
-- turns it on, and every attempt is recorded here: `lookup_status` is what stops
-- the system re-asking a question that has already been answered "no".

ALTER TABLE music_tracks
    ADD COLUMN release_year   INT,
    ADD COLUMN genre          TEXT,
    -- MusicBrainz ids. Kept so a later pass can go straight to the right entity
    -- instead of re-running a fuzzy search that might land somewhere else, and
    -- so a wrong match is traceable to the thing that was matched.
    ADD COLUMN mb_recording_id UUID,
    ADD COLUMN mb_release_id   UUID,
    ADD COLUMN lookup_status  TEXT NOT NULL DEFAULT 'none'
                   CHECK (lookup_status IN ('none', 'pending', 'matched', 'no_match', 'failed')),
    -- Why a match was refused, or why the call failed. Shown to the user: "no
    -- confident match for 'track 03'" is actionable in a way that a silent
    -- no-op is not.
    ADD COLUMN lookup_note    TEXT,
    ADD COLUMN lookup_at      TIMESTAMPTZ;

ALTER TABLE music_tracks ADD CONSTRAINT music_tracks_release_year_check
    CHECK (release_year IS NULL OR release_year BETWEEN 1860 AND 2200);

-- Partial: the only question ever asked of this column is "which tracks still
-- need looking up", and in a settled library that is none of them.
CREATE INDEX music_tracks_lookup_pending_idx ON music_tracks (owner_user_id)
    WHERE lookup_status IN ('none', 'pending');
