-- 0007_media_assets: the media `assets` table (deferred from the Phase-0 split).
-- Owning module: media. Domain modules (movie/music/story/comic) reference an
-- asset by id; they never manage its upload/transcode lifecycle.
--
-- Lifecycle: uploading → processing → ready | failed
--   uploading  : row created, browser is PUTting the original to source_key
--   processing : /complete called, transcode job enqueued
--   ready       : worker produced the HLS output under output_prefix
--   failed      : transcode errored (error_message set)

CREATE TABLE assets (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL DEFAULT 'video',      -- video | audio | image
    status        TEXT NOT NULL DEFAULT 'uploading',
    source_key    TEXT NOT NULL,                      -- S3 key of the uploaded original
    output_prefix TEXT,                               -- S3 prefix of the HLS output
    mime_type     TEXT NOT NULL DEFAULT '',
    size_bytes    BIGINT NOT NULL DEFAULT 0,
    duration_ms   INTEGER,
    width         INTEGER,
    height        INTEGER,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT assets_status_chk CHECK (status IN ('uploading','processing','ready','failed')),
    CONSTRAINT assets_kind_chk   CHECK (kind IN ('video','audio','image'))
);

CREATE INDEX assets_owner_idx ON assets(owner_id, created_at DESC);
