# Portal — Architecture Overview

**Status:** current · **Last verified:** 2026-07-07
**Companions:** [diagrams.md](diagrams.md) (visual views) · [security.md](security.md) (authn/authz spec) · [frontend.md](frontend.md) · decisions in [../adr/](../adr/README.md)
**Live implementation status:** [`/MILESTONE_CHECKS.md`](../../MILESTONE_CHECKS.md) — trust it over any prose here.

This is the narrative architecture of record. It describes the system in three
tiers and keeps them separate on purpose:

- **SHIPPED** — running today, verifiable on the stack.
- **NEXT** — committed and specified ([product/specs/](../product/specs/README.md), ADR-08 order).
- **DEFERRED** — designed but explicitly out of scope ([ADR-01](../adr/01-v1-scope-cut.md), re-entry conditions in [briefs/04](../product/briefs/04-deferred.md)).

## 1. What Portal is, architecturally

A **self-hosted life OS** ([vision](../product/vision.md), [ADR-08](../adr/08-life-os-pivot.md)):
one identity, many life facets (money, time, learning, social, entertainment),
one VPS. The architecture that serves this is:

> **A Go modular monolith + a Next.js 15 frontend, integrated through exactly two
> seams: synchronous `api/` package calls and asynchronous Asynq events.**

The operating envelope shapes everything: **1 developer · 2-week build bursts ·
≤ $100/month · a single VPS**. Every choice below is downstream of that constraint
— which is why the answer to "why not microservices / Kafka / Kubernetes" is
always the same: the envelope.

## 2. Principles (binding)

1. **Modular monolith, hard boundaries.** Domain modules under
   `internal/modules/<name>/` are bounded contexts. They interact **only** through
   each other's `api/` package (sync) or Asynq events `<module>:<event>` (async).
   Never import another module's `service/handler/repository/query`; never JOIN
   across another module's tables. This is the single load-bearing rule
   ([MODULES.md](../../backend/MODULES.md) is authoritative).
2. **The event bus is the product, not plumbing.** Under the life-OS positioning,
   domain events (`bank:transaction_created`, `comic:chapter_published`,
   `media:asset_ready`) are the raw material of the user's **life stream**. Every
   new domain module emits at least one event from its first release (ADR-08).
   Registry: [reference/events.md](../reference/events.md).
3. **One identity, one authorization engine.** Local password auth (Argon2id,
   short-lived JWT + rotating refresh with reuse detection — [ADR-06](../adr/06-local-auth-model.md));
   role-hierarchy RBAC with grammar `<resource>:<action>[:<scope>]`, enforced only
   via `RequirePermission` ([ADR-02], [security.md](security.md)). No ad-hoc checks.
4. **Contracts and generated code are sacred.** Schema changes via numbered
   migrations only; `query/*.sql` → sqlc; `shared/openapi.yaml` is the API
   contract. Generated files are never hand-edited.
5. **Derived over stored.** Where correctness matters (account balances, RBAC
   effective permissions), values are computed from source-of-truth rows (+ cache),
   not maintained as mutable columns.
6. **Status truth is centralized.** Documents don't restate implementation state;
   `MILESTONE_CHECKS.md` does. Diagrams tag tiers instead of pretending.

## 3. The system, by tier

### SHIPPED (the running v1)

Eight compose services behind Traefik v3 on `*.portal.localhost`:
`postgres` (17) + `pgbouncer`, `dragonfly`, `minio` (+ `minio-setup`), `traefik`,
`api`, `worker`, `frontend`.

- **`cmd/api`** — Chi HTTP. Middleware chain: RealIP → RequestID → Recoverer →
  Timeout → RequireAuth → RequirePermission.
- **`cmd/worker`** — Asynq consumer; FFmpeg transcode video → single-rendition HLS.
- **Modules built + wired:** `account` (auth + RBAC + audit + lockout), `media`
  (upload → transcode → `ready` → playback via the API HLS proxy
  `GET /api/v1/assets/{id}/hls/*`).
- **Frontend:** Next.js 15 App Router, RSC-first; `portal_session` middleware
  gate; `SessionKeeper` silent refresh; Vidstack playback (hls.js from CDN —
  internet required, known backlog item).
- **Dev/prod storage:** MinIO / Cloudflare R2 through one S3 client (ADR-04);
  single-tier, no CDN edge.
- Skeleton-only modules exist in tree (`tenant`, `movie`, `music`, `story`,
  `comic`) but are not constructed in `main.go`.

Known shipped-tier gaps (tracked in [backlog](../product/backlog.md)): thumbnail
worker is a stub, no asset delete, HLS effectively public, dead-letter on
transcode failure is design-intent only, rate-limiter built but not mounted on
`/auth/*`.

### NEXT (committed, specified — ADR-08 order)

1. **[SPEC-01](../product/specs/SPEC-01-media-image-pipeline.md)** — image asset
   kind (`media:process_image`, WebP variants, EXIF strip), real video posters,
   `DELETE /assets` + purge janitor, media library page, `media:asset_ready` emit.
2. **[SPEC-02](../product/specs/SPEC-02-comic-vertical.md)** — comic vertical
   end-to-end; the reference implementation of the *media → domain vertical*
   pattern that movie/music/story will copy.
3. **[SPEC-03](../product/specs/SPEC-03-finance-ledger.md)** — finance ledger in
   module `bank` (ledger scope per ADR-08: manual multi-account bookkeeping;
   derived balances; paired-leg transfers; import-ready schema). **Not** gated on
   MFA — it holds no bank credentials; TOTP gates *real bank integration* only.
4. **Notification module** (brief pending) — consumes the three event streams into
   an in-app life stream; email/push trail behind. `notify:*` namespace reserved.

### DEFERRED (designed, not scheduled)

Multi-tenancy + RLS and `cmd/sysjobs`/BYPASSRLS ([ADR-07], [deferred/multi-tenant-backend.md](deferred/multi-tenant-backend.md));
policy-bundle/file-gated authorization ([deferred/access-policies.md](deferred/access-policies.md));
TOTP/step-up (unlock condition: credential-holding or money-moving features);
social baseline at scale, search, creator economy, marketplace, safety workers,
observability stack, LiveKit/mediamtx, CDN edge tier. Each with an explicit
re-entry condition in [briefs/04-deferred.md](../product/briefs/04-deferred.md).

## 4. Cross-cutting views

**Data.** Postgres is the system of record; single numeric migration sequence
across all modules (each migration named for its owning module). Dragonfly serves
three roles: cache (RBAC snapshots, TTL 5 min, invalidated by `token_version`
bump), Asynq broker (requires `--default_lua_flags=allow-undeclared-keys`), and
future pub/sub for realtime. Object storage is per-asset-prefixed S3.
Dev connects **directly to `postgres:5432`** (PgBouncer transaction mode conflicts
with pgx prepared statements); prod goes through PgBouncer.

**AuthZ decision path** (every protected request): JWT verify + user snapshot
(`token_version`, `disabled_at`) → effective permissions via recursive-CTE role
hierarchy, cached per user+version → wildcard-aware permission match. Full spec
and threat model: [security.md](security.md).

**Events.** Naming `<module>:<event>` (facts, past tense) vs tasks (imperatives);
payloads carry IDs + minimum context, consumers fetch detail via `api/` packages.
Emitting with zero consumers is encouraged — it is life-stream groundwork.
Inventory: [reference/events.md](../reference/events.md).

**Frontend.** RSC-first shells, client islands (D-33); TanStack owns server state,
Zustand UI state (D-32); versioned template layer `src/templates/v{N}` (v1 =
Olympus light). Budgets (LCP < 2.5 s, initial JS < 200 KB) in
[frontend.md](frontend.md) §8 bind all new pages, including SPEC-02's reader and
SPEC-03's `(bank)` group.

**Error contract.** RFC 7807 `Problem` on every non-2xx; `type` URIs double as
i18n keys (D-7).

## 5. Risks & watch items

- **Boundary enforcement is convention, not CI.** depguard/golangci-lint is
  planned but absent; until then the module rules hold only by discipline.
  Mitigation candidate: add depguard when the third module lands (comic).
- **OpenAPI drift.** The contract file already lags the code (stale
  `/auth/callback`, missing `/auth/register`); every spec requires fixing drift in
  the same PR, but the codegen-vs-handwritten decision (backlog §9) is still open.
- **Public-ish HLS + finance data on one box.** Acceptable at n=1; both flip with
  the first real second user (playback ACL; revisit admin wildcard reach into
  `bank:*` — flagged in SPEC-03 and ADR-08).
- **Single-VPS blast radius.** Backups/retention are P3 backlog; the ledger raises
  the stakes — schedule Postgres dumps before SPEC-03 dogfooding ends.

## 6. How to change this architecture

Expensive-to-reverse or cross-module choices → new ADR ([adr/README.md](../adr/README.md));
feature-level decisions → `D-N` entries in [feature-inventory](../product/feature-inventory.md);
diagrams updated **in the same PR** as the change they depict; this overview's
tier lists updated when a spec ships (move item SHIPPED-ward, never edit history).
