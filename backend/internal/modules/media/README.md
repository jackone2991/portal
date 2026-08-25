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

The FFmpeg pipeline is real (`worker/transcode.go`), `media:asset_ready` is
emitted and has two consumers, and all three workers open a tenant scope before
writing (ADR-07 increment 1b). Genuinely open:

- **HLS variant ladder** configurable per tier (240p/480p/720p/1080p/4K) —
  transcode currently produces one rendition.
- **S3 multipart upload session** for large originals; today a source is a
  single presigned PUT.
- **`PATCH /assets/{id}` metadata edit** (SPEC-01 P1.1) — not mounted.
- **Audio transcode profile** — audio is stored and served as-is.
