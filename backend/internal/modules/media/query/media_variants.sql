-- Derived, metadata-stripped variants (thumb/medium for images, poster for
-- video). The uploaded original is NEVER re-encoded — it stays as the asset's
-- own storage object; this table holds only derived artifacts (SPEC-01 §6).

-- name: InsertVariant :one
-- Upsert so a re-run of the worker (retry / re-process) replaces the row and its
-- storage key instead of colliding on the (asset_id, variant) unique constraint.
INSERT INTO media_asset_variants (asset_id, tenant_id, variant, storage_key, width, height, size_bytes)
VALUES ($1, (SELECT tenant_id FROM assets WHERE id = $1), $2, $3, $4, $5, $6)
ON CONFLICT (asset_id, variant) DO UPDATE
SET storage_key = EXCLUDED.storage_key,
    width       = EXCLUDED.width,
    height      = EXCLUDED.height,
    size_bytes  = EXCLUDED.size_bytes
RETURNING *;

-- name: ListVariantsByAsset :many
SELECT * FROM media_asset_variants
WHERE asset_id = $1
ORDER BY variant;

-- name: GetVariant :one
SELECT * FROM media_asset_variants
WHERE asset_id = $1 AND variant = $2;

-- name: DeleteVariantsByAsset :exec
-- FK ON DELETE CASCADE already drops rows when the asset row goes, but the
-- purge path deletes storage objects first (enumerated via ListVariantsByAsset)
-- and this makes the row cleanup explicit / order-independent.
DELETE FROM media_asset_variants
WHERE asset_id = $1;
