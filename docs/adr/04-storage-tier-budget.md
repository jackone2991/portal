# ADR-04: Storage tier — R2 only for v1; defer MinIO origin to multi-region phase

**Status:** **accepted** 2026-05-24 · refined 2026-06-06 (dev keeps MinIO; R2-only applies to deployed environments)
**Last verified:** 2026-09-11
**Deciders:** kirito
**Affects:** [docker-compose.yml](../../docker-compose.yml), `backend/internal/platform/storage/`, [diagrams.md §1](../architecture/diagrams.md) (system landscape)

## Context

*As found on 2026-05-24, with one refinement made on 2026-06-06 that the
Decision section now carries: the title says "R2 only", and that holds for
staging/prod; local development keeps MinIO, because the upload flow is
presigned-URL and a filesystem driver cannot sign a URL. What is built is
under Consequences.*

`diagrams.md` §1 drew a two-tier storage architecture:

- **MinIO origin** running on the VPS, holding the canonical bytes (`org/<tid>/assets/source/<id>.mp4`, `org/<tid>/assets/hls/<id>/`).
- **Cloudflare R2 edge** in front, with origin-pull on cache miss. Continuous replication via `mc admin replicate` keeps R2 hot.

This is the right architecture for a self-hosted multi-region SaaS with strict data-sovereignty requirements (some operators want to keep canonical bytes inside their jurisdiction). It is **the wrong architecture for v1**, and probably the wrong architecture for any single-VPS deployment.

### The cost of two storage tiers on a single VPS

| Cost component | MinIO + R2 (the diagram) | R2 only (proposed) |
| --- | --- | --- |
| VPS disk for assets | 100–500 GB ($5–25/mo extra disk on Hetzner) | 0 GB |
| MinIO service | ~150 MB RAM | 0 MB |
| Replication monitoring | `mc admin replicate` health + lag dashboards | None — R2 is the only copy |
| Backup strategy | Backup MinIO (rclone to second S3) AND R2 backup | R2 is the backup; weekly export to cold S3 if paranoid |
| Operational surface | Two credential sets, two URL bases, two CORS configs | One of each |
| Recovery story | Both tiers can drift; reconciliation playbook required | Single source of truth |

### Why MinIO existed in the design

Three legitimate reasons, none of which applied to v1:

1. **Data sovereignty** — some operators legally cannot send user data to Cloudflare. v1 has one operator (you) and is hosted at a Hetzner site that already isn't sovereignty-compliant for many jurisdictions.
2. **Cost ceiling under huge bandwidth** — at very high egress (>10 TB/mo), running your own origin with a cheaper CDN can beat R2. v1 demo egress is measured in MB.
3. **Air-gapped operation** — some self-hosters can't reach the public internet from the VPS. Not v1; the VPS already needs internet for image pulls and DNS (and, when this was written, for Authentik OIDC — since removed by [ADR-06](./06-local-auth-model.md)).

### What R2 alone gets you

R2 is S3-compatible: same `s3.Client` from AWS SDK, same presigned-URL flow, same lifecycle rules. It is *also* a CDN edge — fetches from a browser hit Cloudflare's PoPs directly, no origin-pull required. Egress is free inside Cloudflare's network (which includes browser fetches via Cloudflare DNS).

Pricing (May 2026, public Cloudflare rates):

- Storage: $0.015/GB-month (10 GB free tier)
- Class A operations (PUT, POST, COPY): $4.50/M (1M free)
- Class B operations (GET, HEAD): $0.36/M (10M free)
- **Egress: $0 to internet via Cloudflare**

A v1 demo with 5 GB of stored assets and 100k requests/mo costs ~$0.

## Decision

**Deployed environments use Cloudflare R2 as the single storage tier — no MinIO origin, no replication. Local development keeps MinIO as the S3-speaking origin, bind-mounted at `./data/minio`, with a one-shot `minio-setup` (`mc`) that creates the bucket.** The `platform/storage/` abstraction is one generic S3 client either way, so going live is an `.env` change — repoint `S3_ENDPOINT` + `S3_ACCESS_KEY`/`S3_SECRET_KEY` at R2 and set `S3_USE_PATH_STYLE=false` — and re-introducing a MinIO origin in a future phase is config, not code.

(As decided on 2026-05-24 the text read "MinIO is removed from `docker-compose.yml` for v1". The 2026-06-06 refinement kept it for dev because the media upload flow uses **presigned URLs** — the browser PUTs directly to the store — and a plain local-filesystem driver cannot issue one; dropping MinIO would have forced a second, dev-only upload path and made dev diverge from prod.)

Concretely:

1. The Go S3 client points at `https://<account>.r2.cloudflarestorage.com` in deployed environments and at `minio:9000` in dev. Same SDK calls.
2. Uploads go directly from the browser to the store via presigned URL — the API just signs.
3. The transcode worker reads/writes the store. FFmpeg input/output uses `s3fs`-style streaming via the SDK or via temporary local files in `/tmp` (tmpfs-backed).
4. Buckets are tenant-prefixed exactly as the spec already plans: `org/<tid>/assets/source/`, `org/<tid>/assets/hls/`. The prefix scheme is bucket-agnostic.
5. **No replication.** R2 is the source of truth in v1. A weekly export job (Asynq cron) copies to a second R2 bucket (or off-Cloudflare S3) for disaster recovery; ship in Phase 0.5 if any external data lands.
6. **CORS on R2** must allow the frontend origin (`https://${APP_DOMAIN}`) for direct browser PUT; document in the deployment guide.

## Options considered

### Option A — MinIO origin + R2 edge with replication (current diagram)

| Dimension | Assessment |
| --- | --- |
| Cost | $5–25/mo extra VPS disk + R2 storage |
| Operational complexity | High — two systems, replication, two-source recovery |
| Latency | First fetch slow (origin pull); subsequent fast |
| Failure surface | MinIO disk, replication lag, R2 outage all distinct |

**Pros:** Most architecturally complete; matches the long-run vision.
**Cons:** Two of everything for a system with one user.

### Option B — R2 only  *(chosen for v1)*

| Dimension | Assessment |
| --- | --- |
| Cost | ~$1–5/mo for v1-scale data |
| Operational complexity | Low — one credential set, one URL |
| Latency | Always edge-fast (PoP closest to viewer) |
| Failure surface | R2 outage is total — accept it for v1 |

**Pros:** Cheapest, simplest, fastest for viewers. Removes MinIO's RAM + disk from the VPS budget.
**Cons:** Single point of failure (R2). No data sovereignty story. Egress to non-Cloudflare destinations (e.g. `wget` from outside Cloudflare's network) is charged.

### Option C — MinIO only (no R2)

| Dimension | Assessment |
| --- | --- |
| Cost | VPS disk only (~$25/mo for 240 GB on Hetzner BX-class) |
| Operational complexity | Low — one system |
| Latency | Browser fetches hit the VPS directly; geographically slow |
| Failure surface | VPS = SPoF for both compute and storage |

**Pros:** Fully self-contained; no external dependency.
**Cons:** Browser HLS playback fetches HLS segments from the VPS — every viewer puts read load on the same box that's transcoding. Doesn't scale past a couple of concurrent viewers. Wrong tradeoff for media.

### Option D — Backblaze B2 instead of R2

| Dimension | Assessment |
| --- | --- |
| Cost | $0.006/GB-month storage; $0.01/GB egress |
| Operational complexity | Same as R2 |
| Latency | No native CDN — pair with Cloudflare bandwidth-alliance (free egress to Cloudflare) |
| Failure surface | Similar to R2 |

**Pros:** Cheaper per-GB storage.
**Cons:** Without bandwidth-alliance setup, egress charges add up. With it, you're back to "stored in B2, served via Cloudflare", which is just R2 with extra steps. Skip.

## Trade-off analysis

The decisive trade is "operational simplicity now vs. architectural completeness later". MinIO + R2 is the right destination architecture if and when Portal serves multiple regions or a sovereignty-sensitive operator. For v1, it's two systems doing what one does just as well.

The risk of R2-only is **R2 outage = no playback**. Cloudflare R2 had a multi-hour incident in February 2024; the next one will happen. Mitigation:

- The weekly cross-bucket export (Phase 0.5 deliverable) means data isn't *lost* even in catastrophic R2 failure, only temporarily unavailable.
- For v1's demo loop, R2 unavailability degrades to "video doesn't play"; database state is unaffected. The dependency surface is small.
- Production deployment (post-v1) can re-add MinIO as a *failover origin* — a worker that pulls from MinIO when R2 returns 5xx — without changing the storage interface. This is the future state from the diagrams, deferred.

The cost of NOT removing MinIO from v1 is concrete: ~$15/mo disk + 150 MB RAM + 2 hours of operator setup time + ongoing "is replication healthy" cognitive load. The cost of removing it is zero — the long-run architecture can come back when it earns its place.

## Consequences

What is built (2026-09-11):

- **`docker-compose.yml` gained a service rather than losing one:** `minio` + `minio-setup`, dev-only, bind-mounted at `./data/minio`. There is no `docker-compose.prod.yml` (only `docker-compose.override.yml`, the local-TLS overlay), so "remove MinIO in the prod overlay" is done by not deploying the dev file's MinIO — an `.env` pointing at R2 is the whole switch.
- `backend/internal/platform/storage/` is a single aws-sdk-go-v2 S3 client (`BaseEndpoint` + `UsePathStyle`, tested in `s3_test.go` when `S3_ENDPOINT` is set); `.env.example` carries the `S3_*` block (`S3_ACCESS_KEY`/`S3_SECRET_KEY`, MinIO-shaped defaults, R2 values in comments).
- **Upload paths:** `POST /api/v1/assets` returns a presigned PUT (the prod path); dev also has an API-proxied `PUT /api/v1/assets/{id}/source`. "The API never holds upload bytes" holds for the presigned path only.
- **Playback goes through the API, not the edge:** `GET /api/v1/assets/{id}/hls/*` proxies HLS, and `/assets/{id}/original` is a `ServeContent` range route (see `/CLAUDE.md`). Direct-edge fetch remains the deployed-prod target; "viewers always fetch from the Cloudflare edge" is not what runs today.
- **Object keys are not tenant-prefixed.** As built: `uploads/<id>/original<ext>` and `hls/<assetID>` (`media/service.go`). Decision item 4 was deferred "until tenancy lands"; tenancy landed ([ADR-07](./07-tenancy-rls-model.md), migrations 0018–0020) and the keys did not change. Isolation is by the `assets` row (RLS) and by presigned URLs, not by key prefix — a bucket listing shows every tenant's objects together.
- R2 outage = playback outage in deployed environments. Accepted for v1; still true.
- Data sovereignty story is "your bytes are on Cloudflare R2 in their default region". Not documented in any deployment guide, because there is no deployment guide (ADR-03 action item 6).
- R2 CORS: no `docs/operations/r2-setup.md`; the exact JSON is not captured anywhere in the repo.

**What we'll need to revisit:**

- When the first sovereignty-sensitive operator appears, re-introduce MinIO as a per-tenant configurable origin. The `platform/storage/` interface supports this without code changes (`Endpoint` is config).
- When R2 monthly cost exceeds the VPS line item (>~$60/mo), evaluate Backblaze B2 + Cloudflare bandwidth-alliance for storage tier and Backblaze for origin.
- When the first non-Cloudflare destination needs to fetch assets (e.g. a partner integration), the R2 egress-to-internet fees apply. Plan a signed-URL + Cloudflare Worker proxy if this becomes a hot path.
- Tenant-prefixed keys: decide whether the prefix is still wanted now that RLS does the isolation. If yes, it is a migration of every existing object; the longer it waits the larger that is.

## Action items

1. [x] ~~Remove the `minio` service block for v1.~~ Revised 2026-06-06: MinIO stays in the dev compose on a `./data/minio` bind-mount; deployed environments simply point `S3_*` at R2.
2. [x] ~~Remove `volumes.minio_data`.~~ Done differently: bind-mount; the named volume is gone.
3. [x] `S3_*` block in `.env.example` (names are `S3_ACCESS_KEY`/`S3_SECRET_KEY`; `S3_USE_PATH_STYLE=true` for dev).
4. [x] `platform/storage/` reads endpoint/region/path-style from config — aws-sdk-go-v2 wrapper.
5. [x] `POST /api/v1/assets` presigns PUT; dev also has API-proxied `PUT /assets/{id}/source`.
6. [ ] Worker `/tmp` usage cap (10 GB) — not done. `os.MkdirTemp` per job in `media/worker/{transcode,process_image,thumbnail}.go`, no cap; the guard against disk exhaustion is `heavyConcurrency = 1`.
7. [ ] `docs/operations/r2-setup.md` (bucket creation, CORS JSON, lifecycle rules, token scope) — not written. `docs/operations/` exists now; the file does not.
8. [x] [`diagrams/system-landscape.md`](diagrams/system-landscape.md) shows the v1 R2-only shape; [`architecture/diagrams.md`](../architecture/diagrams.md) keeps the destination architecture.
9. [ ] Tenant-prefixed object keys (Decision item 4) — never implemented; see "revisit" above.
