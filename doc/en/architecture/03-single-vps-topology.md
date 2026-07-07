# ADR-03: Single-VPS topology and compose-profile envelope for v1

**Status:** Accepted (see Update 2026-07-06)
**Date:** 2026-05-24
**Deciders:** kirito
**Affects:** [docker-compose.yml](../../../docker-compose.yml), [Makefile](../../../Makefile), [feature.md D-8 (observability)], [feature.md D-36/D-39 (live + calls)]

## Update (2026-07-06) — Authentik removed (ADR-06); service set as-built

The core decision stands: single VPS (CCX23), profiles `observability`/`live`/`calls` remain off. But [ADR-06](./06-local-auth-model.md) (2026-07-05) replaced OIDC with local password auth, so `authentik-server`/`authentik-worker`/`mailpit` **never shipped** — every Authentik/Mailpit/OIDC mention below is historical. As-built compose set: `traefik`, `postgres`, `pgbouncer`, `dragonfly` (run with `--default_lua_flags=allow-undeclared-keys` for Asynq), `minio` + `minio-setup` (dev-only, per [ADR-04 update 2026-06-06](./04-storage-tier-budget.md)), `api`, `worker`, `frontend`. With Authentik gone the RAM floor is ~1 GB lower than analysed below; the CCX23 sizing is unchanged (extra headroom). Storage split as-built: no Authentik volume; dev uploads land in MinIO on the `./data/minio` bind-mount — direct-to-R2 applies to deployed environments only (ADR-04).

## Context

`docker-compose.yml` currently brings up Traefik + Postgres + PgBouncer + Dragonfly + MinIO + API + Worker + Frontend. The corpus implies additional services land progressively: Authentik (OIDC), Mailpit (dev email), a 5-service observability stack ([D-8]: Loki, Promtail, Prometheus, Tempo, Grafana, GlitchTip), mediamtx (live ingest, [D-36]), LiveKit (group calls, [D-39]), and FFmpeg-bound workers under bursty load.

The constraint envelope is **single VPS, ≤ $100/mo**. At reasonable VPS prices, this translates to:

| Tier | Example (Hetzner) | vCPU | RAM | Disk | ~$/mo |
| --- | --- | --- | --- | --- | --- |
| Bare minimum | CCX13 | 2 dedicated | 8 GB | 80 GB SSD | ~$13 |
| Recommended v1 | CCX23 | 4 dedicated | 16 GB | 160 GB SSD | ~$30 |
| With headroom | CCX33 | 8 dedicated | 32 GB | 240 GB SSD | ~$60 |
| Ceiling | CCX43 | 16 dedicated | 64 GB | 360 GB SSD | ~$120 |

Cloudflare R2 storage is ~$0.015/GB-month with no egress fees inside Cloudflare's network. 100 GB of stored HLS = $1.50/mo; bandwidth to viewers is free at the edge. So infra budget is dominated by the VPS itself.

Once Authentik (~1 GB resident), Postgres (~500 MB shared_buffers + work), Dragonfly (~256 MB allocated, scales with cache), MinIO (~150 MB), Traefik (~50 MB), API + Worker (~300 MB combined idle), Frontend Next.js SSR (~250 MB), and a transcode burst (FFmpeg can spike 1–2 GB for 1080p+ content) are stacked, the **floor is ~3.5 GB RAM idle, ~6 GB under transcode**. CCX13 (8 GB) is too tight; CCX23 (16 GB) is the right v1 tier.

If the observability stack ([D-8]) is brought up, add ~1.1 GB. If LiveKit + mediamtx + coturn are brought up, add ~500 MB idle plus bursty CPU/bandwidth. CCX23 cannot host both observability AND live streaming AND a transcode burst simultaneously.

## Decision

**v1 runs on Hetzner CCX23 (4 vCPU / 16 GB / 160 GB) or equivalent (~$30/mo). The following compose profiles are explicitly *off* for v1:**

- `--profile observability` (Loki/Prometheus/Tempo/Grafana/GlitchTip) — defer until traffic justifies it. v1 runs with stdout JSON logs.
- `--profile live` (mediamtx) — live streaming is Phase 10. Not v1.
- `--profile calls` (LiveKit + coturn) — voice/video is Phase 12. Not v1.

**The v1 service set is:**

| Service | Role | RAM (idle) | Notes |
| --- | --- | --- | --- |
| `traefik` | TLS terminator + reverse proxy | ~50 MB | Single edge; routes by Host + path |
| `postgres` | Database | ~500 MB | shared_buffers tuned to 25% of RAM (4 GB) |
| `pgbouncer` | Connection pool | ~30 MB | Transaction-pool mode |
| `dragonfly` | Redis-compatible cache + Asynq broker | ~256 MB | Bounded with `--maxmemory` (see action items) |
| `api` | Go HTTP server (`cmd/api`) | ~150 MB | Single replica |
| `worker` | Asynq consumer (`cmd/worker`) | ~150 MB idle, 1–2 GB during transcode | TRANSCODE_CONCURRENCY=1 in v1 |
| `frontend` | Next.js SSR | ~250 MB | Single replica |
| `authentik-server` | OIDC IdP | ~700 MB | New addition for v1 |
| `authentik-worker` | Authentik background tasks | ~300 MB | Required by Authentik |
| `mailpit` | Dev SMTP (Authentik password-reset emails) | ~30 MB | Replace with real SMTP in prod |

Floor ~2.4 GB idle, ~4 GB under load. Headroom on a 16 GB VPS is ample for v1 and gives room to add the observability profile in Phase 0.5 without resizing.

**Cloudflare R2** is the only off-VPS dependency (storage origin; see [ADR-04](./04-storage-tier-budget.md)). DNS via Cloudflare is assumed (free tier sufficient).

**Storage** on the VPS itself is split: Postgres data + Authentik data + Dragonfly snapshots on a volume; uploads bypass MinIO and go directly to R2 — saves the disk that would otherwise hold replicated assets.

## Options considered

### Option A — CCX13 (2 vCPU / 8 GB) at ~$13/mo

| Dimension | Assessment |
| --- | --- |
| Cost | Best — under $20/mo |
| Headroom | None — Authentik + Postgres + transcode burst will OOM |
| Future-proofing | Forces a migration to a bigger VPS within months |

**Pros:** Cheapest possible. Fits a hobbyist who never transcodes >720p.
**Cons:** Authentik alone is 1 GB resident; one 1080p transcode and the kernel kills something. Not viable for the 7-step demo.

### Option B — CCX23 (4 vCPU / 16 GB) at ~$30/mo  *(chosen for v1)*

| Dimension | Assessment |
| --- | --- |
| Cost | $30/mo leaves $70 budget for R2, DNS, future paid tiers |
| Headroom | Comfortable idle; one concurrent transcode survives |
| Future-proofing | Can add observability profile without resize; live streaming would force a resize |

**Pros:** Right-sized for v1 + Phase 0.5 expansion. Cheap to upgrade in-place to CCX33 if needed.
**Cons:** Cannot run multiple concurrent transcodes; `TRANSCODE_CONCURRENCY=1` is a hard floor.

### Option C — CCX33 (8 vCPU / 32 GB) at ~$60/mo

| Dimension | Assessment |
| --- | --- |
| Cost | $60/mo + ~$20 R2/Cloudflare = ~$80; still under budget |
| Headroom | Comfortable with observability + 2-3 concurrent transcodes |
| Future-proofing | Runway through Phase 5 (bank) before resize |

**Pros:** Plenty of room; no resize until Phase 7 (social).
**Cons:** Pays for capacity v1 doesn't use. Start smaller; upgrade in-place when needed.

### Option D — Split across two cheap VPSes (one app, one DB/storage)

| Dimension | Assessment |
| --- | --- |
| Cost | ~$26 (2 × CCX13) |
| Headroom | DB on dedicated box; app on the other |
| Operational complexity | Higher — private network, certs, monitoring across two hosts |

**Pros:** Cheaper than CCX23 by ~$4.
**Cons:** Violates the "single VPS" constraint stated upfront. Adds ops complexity for marginal savings. Skip.

## Trade-off analysis

The pivotal question is the **memory pressure from Authentik plus a transcode burst**. Without Authentik, an 8 GB VPS would do. With Authentik, 16 GB is the floor. The alternative (skipping Authentik in favour of a hand-rolled local password store) trades ~1 GB of RAM for 3 days of solo-dev time writing password storage + reset flow + email templates + lockout logic; the time is more valuable than the RAM.

> **Update (2026-07-06):** [ADR-06](./06-local-auth-model.md) reversed this trade — Portal now owns credentials (Argon2id local password auth) and Authentik was removed from the stack. The memory-pressure analysis above no longer binds the VPS sizing.

Cloudflare R2 saving the VPS disk is the second-largest decision. Storing assets locally on the VPS means provisioning ≥240 GB for any meaningful library, which forces CCX33 minimum and a backup strategy (R2 replication or rsync). Sending uploads directly to R2 sidesteps both — see [ADR-04](./04-storage-tier-budget.md).

Disabling the observability profile for v1 is the cheapest call in this ADR. Loki + Prometheus + Tempo + Grafana + GlitchTip cost 5 services and ~1.1 GB for telemetry no one is reading in week 1. `docker compose logs api worker` covers the demo loop.

## Consequences

**What becomes easier:**

- The deploy script is *one* `docker compose up -d` invocation; no profile flags to remember.
- Cost ceiling is predictable: $30/mo VPS + ~$5/mo R2 + Cloudflare free tier = ~$35/mo well under budget.
- Authentik + Mailpit being in the stack from day one means the OIDC flow is testable end-to-end during development (no "wire OIDC later" debt).

**What becomes harder:**

- No observability — when the demo breaks at the customer's site, the only diagnostics are container logs. Schedule the observability profile for the Phase 0.5 sprint.
- TRANSCODE_CONCURRENCY=1 means a slow source video can block the queue. Acceptable for v1 (single demo user); becomes a real bottleneck under multi-tenant usage. Phase 1 must add the per-tenant quota wiring [D-13].
- Authentik adds an entire database schema and admin surface to learn. The Authentik deployment recipe lives in `docs/operations/authentik.md` (referenced in [D-28]) — write at least a stub during the v1 sprint so the next deploy isn't a treasure hunt. *(Moot — Authentik removed, ADR-06.)*

**What we'll need to revisit:**

- When Phase 1 lands tenancy + RLS, the observability profile should land in the same sprint so per-tenant request latency is measurable from day one [D-8].
- When Phase 10 lands live streaming, mediamtx + concurrent transcodes will push the VPS over 16 GB. Plan the CCX33 upgrade (or split to a media-dedicated VPS) ahead of that sprint.
- The backup strategy [D-10] (pgbackrest + R2 replication + Dragonfly BGSAVE) is not v1, but should land before any external user touches the system. Add to Phase 0.5.

## Action items

1. [ ] Set `dragonfly` `command: ["--logtostderr", "--cluster_mode=emulated", "--maxmemory=2GB"]` in docker-compose to cap memory before it competes with FFmpeg. **Partially superseded (2026-07-06):** shipped command is `["--logtostderr", "--default_lua_flags=allow-undeclared-keys"]` (required by Asynq); the `--maxmemory` cap is still open.
2. [x] ~~Add `authentik-server`, `authentik-worker`, and `mailpit` services to `docker-compose.yml`. Use Authentik's published recipe; gate them behind no profile (they're always-on for v1).~~ **Obsolete per ADR-06 (2026-07-05):** Authentik dropped; Portal owns credentials.
3. [ ] Document the disabled profiles in `docker-compose.yml` with a one-line comment: `# v1 disables --profile observability, --profile live, --profile calls — see doc/en/architecture/03-single-vps-topology.md`.
4. [ ] Add a `Makefile` target `make deploy-v1` that runs `docker compose up -d` with NO profile flags — prevents accidental observability/live in v1.
5. [ ] Set Postgres `shared_buffers = 4GB`, `effective_cache_size = 10GB`, `max_connections = 50` (PgBouncer pools below it). Document in `docs/operations/postgres-tuning.md` stub.
6. [ ] In the v1 deployment doc (`docs/operations/deployment.md` — currently absent), record the VPS sizing decision and the rationale for disabled profiles. Cross-reference this ADR.
7. [ ] Set `TRANSCODE_CONCURRENCY=1` and `MAX_CONCURRENT_TRANSCODES_PER_USER=1` in `.env.example` for v1; bump later when quotas land [D-13]. **Still open (2026-07-06):** `cmd/worker/main.go` currently hardcodes Asynq `Concurrency: 4`; no env knob yet.
