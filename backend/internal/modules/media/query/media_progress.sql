-- name: UpsertPlaybackProgress :exec
INSERT INTO media_playback_progress (
  user_id, asset_id, position_ms, completed_at
) VALUES (
  $1, $2, $3, $4
) ON CONFLICT (user_id, asset_id) DO UPDATE SET
  position_ms = EXCLUDED.position_ms,
  updated_at = now(),
  completed_at = COALESCE(media_playback_progress.completed_at, EXCLUDED.completed_at);

-- name: GetPlaybackProgress :one
SELECT position_ms, completed_at, updated_at
FROM media_playback_progress
WHERE user_id = $1 AND asset_id = $2;

-- name: GetContinueItems :many
SELECT
  'media'::text AS module,
  mpp.asset_id AS ref_id,
  COALESCE(a.title, a.original_filename, a.source_key) AS title,
  '/api/v1/assets/' || a.id::text || '/variants/poster' AS poster_url,
  (mpp.position_ms::float / NULLIF(a.duration_ms, 0)::float * 100.0)::int AS progress_pct,
  '/library/media/' || a.id::text AS href,
  mpp.updated_at
FROM media_playback_progress mpp
JOIN assets a ON mpp.asset_id = a.id
WHERE mpp.user_id = $1
  AND a.duration_ms IS NOT NULL
  AND mpp.position_ms >= 30000
  AND (mpp.position_ms::float / NULLIF(a.duration_ms, 0)::float) < 0.95
ORDER BY mpp.updated_at DESC
LIMIT $2;
