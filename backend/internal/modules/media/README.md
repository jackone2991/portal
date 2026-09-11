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

None listed here on purpose. Implementation status has one written owner
(`/CLAUDE.md` § Current status) and open work one list
(`docs/product/backlog.md`) — ADR-11. A status claim in a module README was
wrong within weeks every time it was tried (the 2026-08-25 audit found seven
of eight sections stale).

