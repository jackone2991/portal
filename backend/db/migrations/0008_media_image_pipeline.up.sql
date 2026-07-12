-- 0008_media_image_pipeline: image-pipeline schema for SPEC-01 (§6).
--
-- Adds the derived-variant table plus the `assets` columns, status value, and
-- indexes the image pipeline (P0.1), delete flow (P0.3), library cursor (P0.4),
-- download (P0.5) and the media:asset_ready event (P1.2) need. The uploaded
-- ORIGINAL is never re-encoded — it stays as the asset's own storage object;
-- media_asset_variants holds only derived, metadata-stripped artifacts.

-- P0.3 soft-delete tombstone: `deleting` rows are excluded from all listings
-- until the purge (API path or hourly janitor) removes the objects + row.
ALTER TABLE assets DROP CONSTRAINT assets_status_chk;
ALTER TABLE assets ADD CONSTRAINT assets_status_chk
    CHECK (status IN ('uploading', 'processing', 'ready', 'failed', 'deleting'));

-- title       : library grid label + rename (P1.1) + media:asset_ready payload;
--               defaults to original_filename when unset.
-- original_filename : Content-Disposition filename on download (P0.5).
-- origin      : media:asset_ready flood-guard discriminator [F009] — 'upload'
--               for a normal upload, 'import' for the SPEC-02 zip batch-create,
--               so consumers can suppress the 300-asset burst of one import.
ALTER TABLE assets
    ADD COLUMN title             TEXT,
    ADD COLUMN original_filename TEXT,
    ADD COLUMN origin            TEXT NOT NULL DEFAULT 'upload'
        CHECK (origin IN ('upload', 'import'));

-- Janitor indexed scan over the (near-empty) deleting set; library keyset cursor
-- (owner_id, created_at DESC, id DESC) — the convention all newer specs use [F040].
CREATE INDEX assets_deleting_idx ON assets (updated_at) WHERE status = 'deleting';
CREATE INDEX assets_owner_cursor_idx ON assets (owner_id, created_at DESC, id DESC);

-- Derived served variants (thumb/medium for images, poster for video). WebP,
-- metadata stripped, EXIF orientation baked in — safe to serve semi-public.
CREATE TABLE media_asset_variants (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    asset_id    UUID NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    variant     TEXT NOT NULL CHECK (variant IN ('thumb', 'medium', 'poster')),
    storage_key TEXT NOT NULL,
    width       INT  NOT NULL,
    height      INT  NOT NULL,
    size_bytes  BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (asset_id, variant)
);
