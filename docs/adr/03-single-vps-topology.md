# ADR-03: Single-VPS topology and compose-profile envelope for v1

**Status:** **accepted** 2026-05-24
**Last verified:** 2026-09-11
**Deciders:** kirito
**Affects:** [docker-compose.yml](../../docker-compose.yml), [Makefile](../../Makefile), [D-8] (observability), [D-36]/[D-39] (live + calls) in [feature-inventory.md](../product/feature-inventory.md)

## Context

*As found on 2026-05-24. Two things changed after the decision and are
reflected below: [ADR-06](./06-local-auth-model.md) (2026-07-05) removed
Authentik before it ever shipped, and on 2026-08-21 Postgres moved out of
compose onto the host cluster. The sizing analysis in this section and in
Trade-offs rests on Authentik; it is kept as written because it is why CCX23
was chosen.*

`docker-compose.yml` then brought up Traefik + Postgres + PgBouncer + Dragonfly + MinIO + API + Worker + Frontend. The corpus implied additional services landing progressively: Authentik (OIDC), Mailpit (dev email), a 5-service observability stack ([D-8]: Loki, Promtail, Prometheus, Tempo, Grafana, GlitchTip), mediamtx (live ingest, [D-36]), LiveKit (group calls, [D-39]), and FFmpeg-bound workers under bursty load.

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

**v1 runs on Hetzner CCX23 (4 vCPU / 16 GB / 160 GB) or equivalent (~$30/mo). The following are explicitly *out* for v1** — and none of them exists in `docker-compose.yml`: the file has no `profiles:` key at all, so "off" means "not written", not "behind a flag":

- observability (Loki/Prometheus/Tempo/Grafana/GlitchTip) — defer until traffic justifies it. v1 runs with stdout JSON logs.
- live (mediamtx) — live streaming is Phase 10. Not v1.
- calls (LiveKit + coturn) — voice/video is Phase 12. Not v1.

**The v1 service set, as built** (`grep -E '^  [a-z][a-z0-9_-]+:$' docker-compose.yml`; as decided it also carried `authentik-server` + `authentik-worker`, which ADR-06 removed before they shipped, and `postgres` + `pgbouncer`, which moved to the host on 2026-08-21):

| Service | Role | RAM (idle) | Notes |
| --- | --- | --- | --- |
| `traefik` | TLS terminator + reverse proxy | ~50 MB | Single edge; routes by Host + path |
| *(host)* Postgres 18 | Database | — | **Not in compose.** Runs on the host cluster, reached at `host.docker.internal:5432`; `make up` does not start it. `postgres`/`pgbouncer` are commented out in the compose file with the rollback recipe; the `postgres_data` volume is retained. |
| `dragonfly` | Redis-compatible cache + Asynq broker | ~256 MB | `--default_lua_flags=allow-undeclared-keys` (Asynq's Lua needs it); **no `--maxmemory` cap** (action item 1) |
| `minio` + `minio-setup` | Dev S3 origin, bind-mounted at `./data/minio` | ~150 MB | Dev only; prod is R2 ([ADR-04](./04-storage-tier-budget.md)) |
| `mailpit` | Dev SMTP sink + web UI (`mail.${APP_DOMAIN}`) | ~30 MB | `SMTP_HOST=mailpit` in `.env.example`; prod points `SMTP_*` at a real relay and drops it. Shipped for Portal's own mail (password reset, notify), not for Authentik. |
| `api` | Go HTTP server (`cmd/api`) | ~150 MB | Single replica |
| `worker` | Asynq consumer (`cmd/worker`) | ~150 MB idle, 1–2 GB during transcode | Three servers; the heavy pool is `heavyConcurrency = 1` (a const, not an env knob) |
| `scraper` | Python comic scraper (FastAPI + headless Chrome) | Chrome-sized | Not in the original decision; see `scraper/README.md` |
| `frontend` | Next.js SSR | ~250 MB | Single replica |

Headroom on a 16 GB VPS is ample for v1 and gives room to add observability without resizing.

**Cloudflare R2** is the only off-VPS dependency (storage origin; see [ADR-04](./04-storage-tier-budget.md)). DNS via Cloudflare is assumed (free tier sufficient).

**Storage** on the VPS itself: Dragonfly snapshots and the MinIO bind-mount live on the box (Postgres data lives with the host cluster). Dev uploads land in MinIO; deployed environments upload straight to R2 (ADR-04), which is what saves the disk that would otherwise hold replicated assets.

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

Cloudflare R2 saving the VPS disk is the second-largest decision. Storing assets locally on the VPS means provisioning ≥240 GB for any meaningful library, which forces CCX33 minimum and a backup strategy (R2 replication or rsync). Sending uploads directly to R2 sidesteps both — see [ADR-04](./04-storage-tier-budget.md).

Disabling the observability profile for v1 is the cheapest call in this ADR. Loki + Prometheus + Tempo + Grafana + GlitchTip cost 5 services and ~1.1 GB for telemetry no one is reading in week 1. `docker compose logs api worker` covers the demo loop.

## Consequences

**What became easier:**

- Deploying is `make up` (`docker compose up -d`, no profile flags — there are no profiles to forget). There is no deploy script beyond the Makefile.
- Cost ceiling is predictable: $30/mo VPS + ~$5/mo R2 + Cloudflare free tier = ~$35/mo, well under budget.
- Mailpit in the stack from day one means every mail path (password reset, notification email) is testable end-to-end in dev.

**What became harder:**

- No observability — when the demo breaks at the customer's site, the only diagnostics are container logs (`docker compose logs api worker`). Still true on 2026-09-11.
- Heavy-queue concurrency of 1 means a slow source video blocks the transcode queue. Acceptable for v1 (single demo user); becomes a real bottleneck under multi-tenant usage. The per-tenant quota wiring [D-13] has not landed.
- Postgres on the host means `make up` does not give you a database; the host cluster must be running and reachable at `host.docker.internal:5432`, and tuning lives with the host, not in compose.

**What we'll need to revisit:**

- When live streaming lands, mediamtx + concurrent transcodes will push the VPS over 16 GB. Plan the CCX33 upgrade (or split to a media-dedicated VPS) ahead of that sprint.
- The backup strategy [D-10] shipped as the `ops` module (SPEC-09: `ops:backup_database`, retention, restore drill — [docs/operations/backup-restore.md](../operations/backup-restore.md)). Dragonfly snapshots are not part of it.
- Observability was to land with tenancy so per-tenant latency is measurable from day one [D-8]. Tenancy landed (ADR-07); observability did not.

## Action items

1. [ ] Cap Dragonfly memory before it competes with FFmpeg. Shipped command is `["--logtostderr", "--default_lua_flags=allow-undeclared-keys"]` (Asynq needs the Lua flag); `--maxmemory` is still absent.
2. [x] ~~Add `authentik-server`, `authentik-worker`, and `mailpit` services.~~ Obsolete per ADR-06: Authentik dropped. `mailpit` shipped on its own merits.
3. [ ] Document the out-of-scope services in `docker-compose.yml` with a one-line comment pointing at this ADR (`docs/adr/03-single-vps-topology.md`). Not done; the file has no such comment.
4. [ ] `make deploy-v1` — not done, and moot: with no `profiles:` in the file, plain `make up` cannot bring up anything it shouldn't.
5. [x] Postgres tuning values (`shared_buffers = 4GB`, `effective_cache_size = 10GB`, `max_connections = 50`) are recorded in [docs/operations/postgres-tuning.md](../operations/postgres-tuning.md). They now apply to the host cluster; the "PgBouncer pools below this" note there is stale (there is no PgBouncer).
6. [ ] `docs/operations/deployment.md` — still absent. The VPS sizing rationale lives only here.
7. [x] Transcode concurrency is 1 — as a compile-time constant (`heavyConcurrency` in `cmd/worker/main.go`), which is the OOM guard SPEC-01 P0.1 relies on. Image processing has the env knob (`IMAGE_CONCURRENCY`, default 3, in `.env.example`). `MAX_CONCURRENT_TRANSCODES_PER_USER` does not exist; per-user limits wait on [D-13].
