# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repo state

- `README.md` does not exist — this file is the primary written description of the project. [backend/MODULES.md](backend/MODULES.md) is the authoritative spec for backend module conventions; read it before adding a new domain or crossing an existing module boundary.
- `now.png` is a legacy architecture diagram from the original spec.
- `template-main/` is **reference material, not active code** — a Laravel/PHP portal scaffold and a static HTML social template. Don't edit, don't import. The Go scaffold under `backend/` is the real implementation.

## Stack & decisions

Self-hosted media + ecosystem monorepo (movies / music / stories / comics). Resolved choices:

- **Backend: Go modular monolith.** Three binaries — `cmd/api` (Chi HTTP server), `cmd/worker` (Asynq consumer for FFmpeg transcode/thumbnail), `cmd/sysjobs` (cross-tenant batch, BYPASSRLS — planned). Domain code lives under `internal/modules/<name>/`; cross-cutting infrastructure under `internal/platform/`.
- **Reverse proxy: Traefik v3** — static config in [traefik/traefik.yml](traefik/traefik.yml), middleware in [traefik/dynamic.yml](traefik/dynamic.yml), routes via `docker-compose.yml` labels.
- **Job queue: Asynq** (not BullMQ — BullMQ is Node-only). Three priority queues: `transcode` (5), `thumbnail` (3), `default` (1).
- **API contract: OpenAPI** at [shared/openapi.yaml](shared/openapi.yaml) is the source of truth. Go server stubs (`oapi-codegen`) and TS client types (`openapi-typescript`) are both generated from it. Hand-editing generated files is forbidden.
- **Frontend: Next.js 15** (App Router, RSC), Tailwind v4, Zustand + TanStack Query, Vidstack for HLS playback. Route groups planned: `(movies)`, `(music)`, `(stories)`.
- **Data: Postgres 17 + PgBouncer**, **DragonflyDB** (Redis-compatible cache + Asynq broker), **MinIO** (origin) + **Cloudflare R2** (CDN edge).

## Backend module boundaries (read before editing across modules)

The full spec is [backend/MODULES.md](backend/MODULES.md). The load-bearing rule:

> **Modules talk to each other only through their `api/` package. They never import each other's `service/`, `handler/`, `repository/`, `query/`, or subdomain packages. They never JOIN across each other's tables.**

Layout:

```
backend/internal/
├── modules/             ← one bounded context per subdir
│   ├── account/         users, OIDC/JWT auth, RBAC, sessions, audit
│   ├── tenant/          organizations, memberships, RLS bootstrap
│   ├── media/           assets + transcode/thumbnail workers (shared infra)
│   ├── movie/ music/ story/ comic/   ← depend on media for assets
└── platform/            config, db, cache, storage, jobs, middleware (no business logic)
```

Inside each module: `module.go` (the `New(Deps) *Module` constructor + `MountHTTP` + `RegisterTasks`), `api/` (public surface), `handler/`, `service/`, `middleware/`, `query/` (sqlc input), `repository/` (sqlc output — do not hand-edit).

- `cmd/api/main.go` and `cmd/worker/main.go` are the wiring layer; they construct each module and call `MountHTTP` / `RegisterTasks`. Modules do not register one another's routes.
- One documented exception to "api-only": `cmd/api` may grab `account.Module.Engine()` to build module-specific `RequirePermission` middleware. Other modules MUST NOT import `account/rbac` directly.
- Cross-module async coupling is via Asynq events named `<emitting-module>:<event>` (e.g. `media:asset_ready`). No shared transactions across modules.
- Schema ownership is per-module; reading another module's tables goes through its `api/` or via events, never a raw JOIN.

Adding a new module: follow the checklist in `backend/MODULES.md` §8 (create the subtree, add an `sqlc.yaml` block, write the migration with `000N_<name>_…` prefix, wire into both `cmd/api/main.go` and `cmd/worker/main.go`).

## Account module — auth + RBAC architecture (non-obvious)

The account module ([backend/internal/modules/account/](backend/internal/modules/account/)) is intentionally strict; behavior diverges from textbook RBAC in subtle ways.

### Identity flow
1. **OIDC via Authentik.** No local password auth. `/auth/login` sets a 5-min `portal_oidc` cookie binding `state` (CSRF) + `nonce` (ID-token replay). Callback validates both before exchange.
2. **Two tokens:** short-lived JWT access token (5min, HS256, rotating `kid` keys) + long-lived random refresh token (256-bit, SHA-256-hashed at rest, 30d).
3. **Cookies:** `portal_access` (Path=/, SameSite=Strict) and `portal_refresh` (Path=/auth, SameSite=Strict) — both `HttpOnly Secure`. API clients use `Authorization: Bearer` headers instead.

### Two revocation channels — both are needed
- **`users.token_version`** — bump it and every existing access token fails its DB snapshot check inside `RequireAuth` middleware. The "instant logout-all" channel. Middleware verifies the JWT *and then* re-reads `users.token_version` + `disabled_at` on every request — a still-valid signature is not sufficient.
- **`refresh_tokens.revoked_at`** — refresh-token-side revocation. Rotation chain (`parent_id` / `replaced_by_id`) is linear; **presenting an already-rotated token revokes the entire chain** (forward + backward via recursive CTE) and emits an `auth.refresh.reuse_detected` audit event. Theft detection, not just bookkeeping.

### Permission grammar
Codes are `<resource>:<action>[:<scope>]`. Wildcards: `*`, `<resource>:*`, `*:<action>`. Scope rules in [backend/internal/modules/account/rbac/permission.go](backend/internal/modules/account/rbac/permission.go):

- A 2-segment grant (`movies:write`) satisfies a bare or `:any` requirement, **but not** `:own`.
- A 3-segment `:any` grant satisfies bare or `:any` requirements.
- `:own` grants only match `:own` requirements. Ownership comparison is the caller's responsibility — `RequireOwnerOrPermission` middleware composes "owner OR :any-perm" for the canonical pattern.
- `Set.AllowsCode` is **fail-closed** on malformed input — even a `*` superadmin grant returns false against an invalid required code.

### Role hierarchy
Adjacency list (`roles.parent_id`). Cycles prevented at app layer (DB CHECK is self-only). Hierarchy walk is a recursive CTE in `GetEffectivePermissions`:

```text
guest → user → creator → editor → moderator → admin → superadmin
```

A child inherits **every** ancestor's permissions. Effective permission set is the union across all assigned (non-expired) roles + their ancestors. `superadmin` holds the literal `*` wildcard.

### Permission cache invalidation
[backend/internal/modules/account/rbac/cache.go](backend/internal/modules/account/rbac/cache.go) namespaces Redis keys by `token_version`: `rbac:perms:<userID>:v<N>`. Bumping `token_version` is therefore both token-revocation AND cache-invalidation in one step — never call `Invalidate` manually for normal flows.

### Engine is the single decision point
Never check permissions ad-hoc. Always go through `rbac.Engine.Authorize` / `rbac.Engine.AuthorizeOwnerOr`, or the middleware wrappers `RequirePermission` / `RequireOwnerOrPermission` / `RequireRole` from [backend/internal/modules/account/middleware/](backend/internal/modules/account/middleware/). Direct slice scans in handlers are the wrong layer.

### Audit log is best-effort, never blocking
[backend/internal/modules/account/audit/logger.go](backend/internal/modules/account/audit/logger.go) logs and swallows errors — a DB hiccup must not abort the user request. If audit reliability becomes load-bearing, route through Asynq with a dedicated queue. Don't make handlers depend on the return value.

## What's NOT wired up yet

- **`cmd/api/main.go`** still has a `TODO: mount OpenAPI-generated handlers` comment and does not yet call `account.New(...)` or any module's `MountHTTP`. The account module assembles its handler internally inside [backend/internal/modules/account/module.go](backend/internal/modules/account/module.go); the API binary just hasn't been taught to construct it. Wiring is deferred until repository adapters land.
- **`internal/modules/*/repository/`** directories exist but are empty. The interfaces consumed by the account module (`AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserUpserter`) need adapters around the sqlc-generated code once `make sqlc` runs.
- **No unit/integration test runner beyond** [backend/internal/modules/account/rbac/permission_test.go](backend/internal/modules/account/rbac/permission_test.go).

## Common commands

All from repo root via the [Makefile](Makefile):

| Command | What it does |
| --- | --- |
| `make up` / `make down` | Bring the docker-compose stack (Postgres, PgBouncer, Dragonfly, MinIO, Traefik) up/down. `up` auto-creates `.env` from `.env.example` if missing. |
| `make migrate` | Apply pending migrations from `backend/db/migrations/` (single numeric sequence; file names prefixed by owning module) |
| `make migrate-new name=<snake_case>` | Scaffold a new migration pair |
| `make migrate-down` | Roll back the last migration |
| `make sqlc` | Generate per-module `repository/*.sql.go` from `internal/modules/*/query/*.sql` (multi-block `sqlc.yaml`) |
| `make openapi` | Regen Go server stubs (`oapi-codegen`) + TS client types (`openapi-typescript`) from `shared/openapi.yaml` |
| `make dev` | Run api + worker + frontend in parallel with hot reload (needs `air`, `pnpm`) |
| `make test` / `make test-backend` / `make test-frontend` | Test suites (`go test ./... -race -count=1` for backend) |
| `make lint` | `golangci-lint run` + `pnpm lint` |

Single Go test: `cd backend && go test ./internal/modules/account/rbac -run TestMatches -v`

## Working in this repo

- **The OpenAPI spec is contract.** When adding an endpoint, edit `shared/openapi.yaml` first, then `make openapi`, then implement the generated handler interface. Don't write handlers that drift from the spec.
- **Don't hand-edit generated files**: any `internal/modules/*/repository/*.sql.go`, `internal/handler/api.gen.go`, `frontend/src/lib/types.gen.ts`.
- **Migration-only schema changes.** All DDL goes through `backend/db/migrations/` with `000N_<owning-module>_<description>.up.sql` naming. `query/*.sql` files contain DML/DQL only (sqlc consumes them) and live inside the owning module.
- **Never reach back to add a column to another module's table** — the owning module ships the migration after coordination.
- **System roles are protected.** Migration `0002_account_rbac` marks the seven default roles `is_system = true`; the `UpdateRole` / `DeleteRole` queries refuse to touch them. Don't override that flag without thinking about disaster recovery.
- **Cookie flags are environment-sensitive.** `COOKIE_SECURE=true` is the default; only flip to `false` for plain-`http://localhost` development. Do not commit a `.env` with `COOKIE_SECURE=false`.
- **`internal/sysrepository` (BYPASSRLS) is restricted to `cmd/sysjobs`.** Anything else importing it is a depguard violation — bypassing RLS in the API path would be catastrophic.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **portal** (17499 symbols, 34457 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/portal/context` | Codebase overview, check index freshness |
| `gitnexus://repo/portal/clusters` | All functional areas |
| `gitnexus://repo/portal/processes` | All execution flows |
| `gitnexus://repo/portal/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
