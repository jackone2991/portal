CREATE TABLE media_playback_progress (
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                 -- identity-anchor exception (SPEC-04 §6 precedent)
  asset_id     uuid NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
                 -- the media table is named `assets` (migration 0007)
                 -- same-module FK: progress dies with the asset
  position_ms  bigint NOT NULL CHECK (position_ms >= 0),
  completed_at timestamptz,   -- P1.5 latch: set once at first >= 95% crossing
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, asset_id)
);

CREATE INDEX media_playback_progress_user_updated_idx ON media_playback_progress (user_id, updated_at DESC);
