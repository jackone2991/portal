# ADR-04: Storage tier — R2 only for v1; defer MinIO origin to multi-region phase

**Status:** Proposed
**Date:** 2026-05-24
**Deciders:** kirito
**Affects:** [docker-compose.yml](../../../docker-compose.yml), `backend/internal/platform/storage/`, [diagrams.md §1] (system landscape)

## Update (2026-06-06) — local dev runs MinIO on a local folder; R2-only still applies to deployed environments

The R2-only decision below stands **for deployed environments** (staging/prod): no MinIO origin tier, no replication. For **local development**, the dev `docker-compose.yml` keeps MinIO as the S3-compatible origin, **bound to the local folder `./data/minio`** (a bind-mount, not a named volume), with a one-shot `minio-setup` (`mc`) that creates the media bucket.

Rationale: the media upload flow uses **presigned URLs** (the browser PUTs directly to the store). A plain local-filesystem driver cannot issue presigned URLs, which would force a second, dev-only upload path (client → API → disk) and make dev diverge from prod. MinIO speaks S3, so dev keeps the exact presigned flow and **going live is an `.env` change only** — repoint `S3_ENDPOINT` + `S3_ACCESS_KEY`/`S3_SECRET_KEY` at R2 and set `S3_USE_PATH_STYLE=false`. No code difference; `platform/storage/` stays a single S3 client.

Net: **dev = MinIO (local folder) · prod = R2.** This *refines*, not reverses, the decision below. Action items 1–2 are superseded accordingly: MinIO is removed only from the prod overlay (`docker-compose.prod.yml`); the dev base keeps it on a bind-mount.

## Context

`diagrams.md` §1 draws a two-tier storage architecture:

- **MinIO origin** running on the VPS, holding the canonical bytes (`org/<tid>/assets/source/<id>.mp4`, `org/<tid>/assets/hls/<id>/`).
- **Cloudflare R2 edge** in front, with origin-pull on cache miss. Continuous replication via `mc admin replicate` keeps R2 hot.

This is the right architecture for a self-hosted multi-region SaaS with strict data-sovereignty requirements (some operators want to keep canonical bytes inside their jurisdiction). It is **the wrong architecture for v1**, and probably the wrong architecture for any single-VPS deployment.

### The cost of two storage tiers on a single VPS

| Cost component | MinIO + R2 (current diagram) | R2 only (proposed) |
| --- | --- | --- |
| VPS disk for assets | 100–500 GB ($5–25/mo extra disk on Hetzner) | 0 GB |
| MinIO service | ~150 MB RAM | 0 MB |
| Replication monitoring | `mc admin replicate` health + lag dashboards | None — R2 is the only copy |
| Backup strategy | Backup MinIO (rclone to second S3) AND R2 backup | R2 is the backup; weekly export to cold S3 if paranoid |
| Operational surface | Two credential sets, two URL bases, two CORS configs | One of each |
| Recovery story | Both tiers can drift; reconciliation playbook required | Single source of truth |

### Why MinIO existed in the design

Three legitimate reasons, none of which apply to v1:

1. **Data sovereignty** — some operators legally cannot send user data to Cloudflare. v1 has one operator (you) and is hosted at a Hetzner site that already isn't sovereignty-compliant for many jurisdictions.
2. **Cost ceiling under huge bandwidth** — at very high egress (>10 TB/mo), running your own origin with a cheaper CDN can beat R2. v1 demo egress is measured in MB.
3. **Air-gapped operation** — some self-hosters can't reach the public internet from the VPS. Not v1; the VPS already needs internet for Authentik OIDC flows, image pulls, and DNS.

### What R2 alone gets you

R2 is S3-compatible: same `s3.Client` from AWS SDK, same presigned-URL flow, same lifecycle rules. It is *also* a CDN edge — fetches from a browser hit Cloudflare's PoPs directly, no origin-pull required. Egress is free inside Cloudflare's network (which includes browser fetches via Cloudflare DNS).

Pricing (May 2026, public Cloudflare rates):

- Storage: $0.015/GB-month (10 GB free tier)
- Class A operations (PUT, POST, COPY): $4.50/M (1M free)
- Class B operations (GET, HEAD): $0.36/M (10M free)
- **Egress: $0 to internet via Cloudflare**

A v1 demo with 5 GB of stored assets and 100k requests/mo costs ~$0.

## Decision

**v1 uses Cloudflare R2 as the single storage tier. MinIO is removed from `docker-compose.yml` for v1.** The `platform/storage/` abstraction stays a generic S3 interface (it's already that), so re-introducing MinIO origin in a future phase is a config change, not a code change.

Concretely:

1. The Go S3 client points at `https://<account>.r2.cloudflarestorage.com` instead of `minio:9000`. Same SDK calls.
2. Uploads go directly from the API to R2 via presigned URL — the browser PUTs to R2, the API just signs.
3. The transcode worker reads/writes R2. FFmpeg input/output uses `s3fs`-style streaming via the SDK or via temporary local files in `/tmp` (tmpfs-backed).
4. R2 buckets are tenant-prefixed exactly as the spec already plans: `org/<tid>/assets/source/`, `org/<tid>/assets/hls/`. The prefix scheme is bucket-agnostic.
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

**What becomes easier:**

- `docker-compose.yml` loses one service. The VPS budget loosens.
- The storage interface in `platform/storage/` is simpler — no replication monitoring, no failover logic.
- Frontend uploads to R2 directly (presigned PUT) bypass the API for the data plane — API only signs URLs, never holds upload bytes in memory.
- Viewers always fetch from the Cloudflare edge — global latency floor without operator effort.

**What becomes harder:**

- R2 outage = playback outage. Accept it for v1.
- Data sovereignty story is "your bytes are on Cloudflare R2 in their default region" — if any operator needs different, they have to add MinIO themselves. Document this honestly in the v1 deployment guide.
- The frontend's CORS configuration on R2 buckets is one more thing to get right (a misconfig produces opaque browser errors). Capture the exact JSON in `docs/operations/r2-setup.md`.

**What we'll need to revisit:**

- When the first sovereignty-sensitive operator appears, re-introduce MinIO as a per-tenant configurable origin. The `platform/storage/` interface should support this without code changes (it already does — `Endpoint` is config).
- When R2 monthly cost exceeds the VPS line item (>~$60/mo), evaluate Backblaze B2 + Cloudflare bandwidth-alliance for storage tier and Backblaze for origin.
- When the first non-Cloudflare destination needs to fetch assets (e.g. a partner integration), the R2 egress-to-internet fees apply. Plan a signed-URL + Cloudflare Worker proxy if this becomes a hot path.

## Action items

1. [x] ~~Remove the `minio` service block from `docker-compose.yml` for v1.~~ **Revised (Update 2026-06-06):** keep MinIO in the dev compose bound to `./data/minio`; remove it only in `docker-compose.prod.yml`.
2. [x] ~~Remove `volumes.minio_data` from `docker-compose.yml`.~~ Done a different way: switched MinIO to a `./data/minio` bind-mount (the named volume is gone).
3. [ ] Add `S3_ENDPOINT`, `S3_REGION`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_BUCKET`, `S3_USE_PATH_STYLE=false` to `.env.example` with R2-shaped placeholders.
4. [ ] In `backend/internal/platform/storage/`, ensure the S3 client constructor reads endpoint + region from config (not hard-coded to MinIO). If the package is still empty, scaffold it as a thin wrapper over `aws-sdk-go-v2/service/s3`.
5. [ ] In `cmd/api`, the upload handler signs PUTs (`s3:PutObject`, 5-minute expiry) and returns the URL + key to the frontend; the frontend uploads directly to R2.
6. [ ] In the transcode worker, source download uses presigned GET; HLS segments are uploaded via the SDK directly. Cap the worker's `/tmp` usage at 10 GB.
7. [ ] Write a one-page `docs/operations/r2-setup.md` with: bucket creation, CORS config (allow `${APP_DOMAIN}`), lifecycle rules (none for v1), how to mint the R2 token with `Object Read & Write` scope.
8. [ ] In `diagrams/system-landscape.md` (this ADR set), the v1-scoped diagram already shows R2-only; keep `diagrams.md` (the full vision) as-is — it shows the destination architecture.
