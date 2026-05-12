-- 0001_init: foundational tables.
-- Domain tables (movies, music, stories) live in later migrations.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "unaccent";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    oidc_subject    TEXT UNIQUE NOT NULL,         -- Authentik 'sub' claim
    email           TEXT UNIQUE NOT NULL,
    display_name    TEXT NOT NULL,
    avatar_url      TEXT,
    role            TEXT NOT NULL DEFAULT 'user', -- 'user' | 'editor' | 'admin'
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Generic asset table — every uploaded media file (video, audio, image) lives here.
-- Domain tables reference assets via foreign key.
CREATE TABLE assets (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,                -- 'video' | 'audio' | 'image'
    source_key      TEXT NOT NULL,                -- S3 key of the original upload
    output_prefix   TEXT,                         -- S3 prefix where HLS / variants are written
    mime_type       TEXT NOT NULL,
    size_bytes      BIGINT NOT NULL,
    duration_ms     INTEGER,                      -- video/audio only
    width           INTEGER,
    height          INTEGER,
    status          TEXT NOT NULL DEFAULT 'uploaded',
                    -- 'uploaded' | 'processing' | 'ready' | 'failed'
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX assets_owner_idx ON assets (owner_id);
CREATE INDEX assets_status_idx ON assets (status) WHERE status <> 'ready';
