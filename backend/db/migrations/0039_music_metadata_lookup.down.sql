-- Down: the catalogue data goes, the tracks stay. Covers fetched from the Cover
-- Art Archive are ordinary image assets and are NOT removed — they are referenced
-- by cover_asset_id like any other, and dropping a column is no reason to delete
-- somebody's artwork.
DROP INDEX IF EXISTS music_tracks_lookup_pending_idx;
ALTER TABLE music_tracks DROP CONSTRAINT IF EXISTS music_tracks_release_year_check;
ALTER TABLE music_tracks
    DROP COLUMN IF EXISTS lookup_at,
    DROP COLUMN IF EXISTS lookup_note,
    DROP COLUMN IF EXISTS lookup_status,
    DROP COLUMN IF EXISTS mb_release_id,
    DROP COLUMN IF EXISTS mb_recording_id,
    DROP COLUMN IF EXISTS genre,
    DROP COLUMN IF EXISTS release_year;
