# Media module

Generic media primitives shared by all domain modules.

## Subpackages

- `worker/` — `transcode` + `thumbnail` Asynq task handlers (FFmpeg-backed)
- `service/` — upload session lifecycle (presigned multipart, complete)
- `handler/` — `/assets/*` HTTP handlers
- `api/` — public surface (`GetAsset`, `SignedURL`)
- `query/`, `repository/` — sqlc

## Owns these tables

`assets` (and future `asset_variants` for HLS/DASH metadata).

## Talks to

- `platform/storage` (S3/MinIO/R2) for presigned URLs + object PUT
- `platform/jobs` for enqueuing transcode/thumbnail
- `account/api` for owner-id validation on upload completion

## Emits events

- `media:asset_ready` — payload `{asset_id, hls_master_url, duration_ms}`. Movie / music / story / comic modules subscribe.

## Subscribes to

Nothing.

## Open work

- Real FFmpeg pipeline in `worker/transcode.go` (currently logs and returns nil).
- HLS variant ladder configurable per tier (240p/480p/720p/1080p/4K).
- S3 multipart upload session: `service/upload.go`.
- `media:asset_ready` emission wired to repository update.
