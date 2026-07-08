---
name: Portal project guide
description: Orientation + conventions for the Portal monorepo (auto-loaded by Continue)
alwaysApply: true
---

# Portal — Continue Project Guide

> Quick orientation for AI-assisted work. **Authoritative sources** (read these for depth):
> `CLAUDE.md` (root, full conventions), `MILESTONE_CHECKS.md` (root, live status),
> `backend/MODULES.md` (module-boundary contract), `docs/` (ADRs, product, architecture).
> When this guide and those disagree, **they win** — flag the drift.

## 1. Project Overview
Self-hosted media + social ecosystem monorepo (movies / music / stories / comics + a
Facebook-like social layer). **Go modular-monolith backend + Next.js 15 frontend.**

- **v1 scope (shipped):** local password auth + one video **upload → transcode → HLS playback**
  happy path. Most social UI exists but is **sample data** with no backend. Don't assume a
  feature is wired just because a screen or a module folder exists.
- **Stack:** Go 1.24 · Chi · Asynq · sqlc · Postgres 17 + PgBouncer · DragonflyDB (Redis-compat)
  · MinIO (dev) + Cloudflare R2 (prod, one S3 client) · Traefik v3 · Next.js 15 (App Router/RSC)
  · Tailwind v4 · Vidstack · Docker Compose.

## 2. Getting Started
Go/Node are **not** installed locally on the dev machine — the stack runs via Docker.
All commands go through the root `Makefile`:

| Command | Does |
|---|---|
| `make up` / `make down` | Bring the compose stack up/down (auto-creates `.env`). **Use `make up`, not `-f docker-compose.yml`**, so the override mounts TLS certs. |
| `make migrate` / `make migrate-new name=<snake>` | Apply / scaffold migrations (`backend/db/migrations/`) |
| `make sqlc` | Regenerate `repository/*.sql.go` from `query/*.sql` |
| `make openapi` | Regenerate Go stubs + TS types from `shared/openapi.yaml` |
| `make dev` | api + worker + frontend with hot reload |
| `make test` / `make lint` | `go test ./... -race` + vitest / golangci-lint + pnpm lint |

Dev login: `demo@portal.localhost` / `Passw0rd123`. App at `https://portal.localhost/`.

## 3. Project Structure
```
backend/
├── cmd/            api (Chi), worker (Asynq+FFmpeg), sysjobs (planned, BYPASSRLS)
├── internal/
│   ├── modules/    account, media (REAL) · tenant/movie/music/story/comic (SKELETON)
│   └── platform/   config, db, cache, storage, jobs, middleware, audit (no business logic)
└── db/migrations/  single numeric sequence, files prefixed by owning module
frontend/src/
├── app/(app)|(public)   route groups (version-agnostic shells)
├── templates/v{N}/      actual page/component code (v1 = Olympus light)
└── lib/                 api-client, generated types
shared/openapi.yaml      API contract (source of truth)
docs/                    adr/ product/ architecture/ guides/ reference/
```
An empty `repository/` dir = that module is scaffolded but **not wired** (not constructed in `cmd/api/main.go`).

## 4. Development Workflow (hard rules)
- **Module boundaries:** modules talk **only** through their `api/` package — never import another
  module's `service/handler/repository/query`, never JOIN across their tables. Cross-module coupling
  is async via Asynq events named `<module>:<event>` (e.g. `media:asset_ready`).
- **OpenAPI first:** to add an endpoint, edit `shared/openapi.yaml` → `make openapi` → implement the
  generated interface. (Note: spec currently drifts from handlers — see `MILESTONE_CHECKS.md`.)
- **Never hand-edit generated files:** `*/repository/*.sql.go`, `internal/handler/api.gen.go`,
  `frontend/src/lib/types.gen.ts`.
- **Schema changes are migration-only** (`000N_<owning-module>_<desc>.up.sql`). Never add a column to
  another module's table.
- **System roles are protected** (`is_system=true`); `COOKIE_SECURE=true` default (never commit `false`);
  `internal/sysrepository` (BYPASSRLS) is restricted to `cmd/sysjobs`.
- **Commit only when asked.** Run impact analysis before editing shared symbols.

## 5. Key Concepts
- **Auth (ADR-06):** Portal owns credentials — Argon2id password hash, short JWT access token
  (5min, rotating `kid`) + long refresh token (rotation + reuse detection). Two revocation channels:
  `users.token_version` (instant logout-all) and `refresh_tokens.revoked_at`. No OIDC/Authentik anymore.
- **RBAC:** role hierarchy (recursive CTE), permission grammar `<resource>:<action>[:<scope>]`.
  Always decide through `rbac.Engine.Authorize` / middleware — never ad-hoc slice scans.
- **Media pipeline:** upload → `assets` row → Asynq `transcode` job → `ffmpeg` VOD HLS → `status=ready`
  → Vidstack playback. Worker in `cmd/worker`.
- **Templates:** frontend pages live in `src/templates/v{N}/`, selected via
  `NEXT_PUBLIC_TEMPLATE_VERSION` — read `frontend/src/templates/README.md` before adding a page.

## 6. Common Tasks
- **Add an endpoint:** openapi.yaml → `make openapi` → handler in the owning module → mount via its `MountHTTP`.
- **Add a DB query:** write SQL in the module's `query/*.sql` → `make sqlc` → use the generated method via the repo adapter.
- **Add a module:** follow `backend/MODULES.md` §8 (subtree + `sqlc.yaml` block + migration + wire into `cmd/api` & `cmd/worker`).
- **Run one Go test:** `cd backend && go test ./internal/modules/account/rbac -run TestMatches -v`.

## 7. Troubleshooting
- **`portal.localhost` down / TLS error:** Traefik lost its cert mounts — `docker compose up -d --force-recreate traefik` (started via `make up`, not raw `-f`).
- **Transcode stuck at `processing`:** Dragonfly must run with `--default_lua_flags=allow-undeclared-keys` (Asynq Lua).
- **pgx prepared-statement clash in dev:** dev connects **direct** to `postgres:5432`, bypassing PgBouncer transaction mode.
- **Browser can't reach `minio:9000`:** dev uploads proxy through the API (`PUT /assets/{id}/source`), not presigned direct-to-bucket.

## 8. References
- `CLAUDE.md` — full conventions (authoritative). `MILESTONE_CHECKS.md` — what's actually done.
- `backend/MODULES.md` — module contract. `docs/adr/` — decisions (00–09). `docs/product/` — scope/backlog.
- `shared/openapi.yaml` — API contract. `docs/reference/events.md` — Asynq event registry.
