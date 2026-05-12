-- name: CreateAsset :one
INSERT INTO assets (owner_id, kind, source_key, mime_type, size_bytes)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAsset :one
SELECT * FROM assets WHERE id = $1;

-- name: ListAssetsByOwner :many
SELECT * FROM assets
WHERE owner_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: MarkAssetReady :exec
UPDATE assets
SET status = 'ready',
    output_prefix = $2,
    duration_ms = $3,
    width = $4,
    height = $5,
    updated_at = now()
WHERE id = $1;

-- name: MarkAssetFailed :exec
UPDATE assets
SET status = 'failed',
    error_message = $2,
    updated_at = now()
WHERE id = $1;
