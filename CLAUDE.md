# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repo state

- `README.md` does not exist — this file is the primary written description of the project. [backend/MODULES.md](backend/MODULES.md) is the authoritative spec for backend module conventions; read it before adding a new domain or crossing an existing module boundary.
- `now.png` is a legacy architecture diagram from the original spec.
- `template-main/` is **reference material, not active code** — a Laravel/PHP portal scaffold and a static HTML social template. Don't edit, don't import. The Go scaffold under `backend/` is the real implementation.

## Project scope & constraints (read before planning work)

Everything below describes the **full, multi-year platform** — it is NOT the current scope. A recent evaluation pass ([docs/adr/](docs/adr/), ADRs 00–09) cut v1 down to a hard envelope: **1 developer · 2 weeks · ≤ $100/mo · single VPS.** Default to the v1 cut unless the user says otherwise — do not reach for the bank/social/marketplace modules.

- **v1 = Phase 0 wiring + one video-upload happy path — and it's done.** ([01-v1-scope-cut.md](docs/adr/01-v1-scope-cut.md) scoped it; `MILESTONE_CHECKS.md` at repo root tracks it live.) The demo loop is closed and committed: local password sign-in → authenticated Next.js home → upload mp4 → MinIO (dev) / R2 (prod) → worker transcodes to HLS → `assets.status = ready` → Vidstack playback → revocable logout. No tenants, no domain CRUD, no bank, no social, no observability stack, no mediamtx/LiveKit.
- **Phase 0 wiring is closed, not a pending blocker** ([05-phase0-wiring-order.md](docs/adr/05-phase0-wiring-order.md) has the original plan/sequencing — useful for the *shape* of the work but stale on status): migrations audited/split, `make sqlc` run, repository adapters written, `account.New(...)` + `media.New(...)` constructed and mounted in `cmd/api/main.go`, local auth end-to-end, frontend auth gate wired. See "Current status" below for what's actually left.
- **Deferred outright for v1:** bank, social (+ advanced social), creator economy, marketplace, ML safety, LiveKit/mediamtx, the 5-service observability stack. Compose profiles `--profile observability`, `--profile live`, and `--calls` stay disabled.
- **Storage:** dev runs MinIO bind-mounted to `./data/minio`; prod uses R2. Same `platform/storage` S3-compatible client code either way ([ADR-04](docs/adr/04-storage-tier-budget.md) — the title says "R2-only" but its 2026-06-06 update note keeps MinIO for local dev, since presigned-URL uploads need an S3-speaking origin).

**Docs live in [docs/](docs/)** — restructured 2026-07-07 into a Diátaxis-style tree ([ADR-09](docs/adr/09-docs-architecture.md)); the old flat `doc/en` + `doc/vi` mirror is retired. **English-only now** — the frozen Vietnamese mirror is read-only in `docs/archive/vi-2026-07/`; never update it. Map:

- **[docs/adr/](docs/adr/)** — decision records 00–09 (`NN-*.md`). [ADR-08](docs/adr/08-life-os-pivot.md) repositioned Portal from Facebook-parity to a *life OS* (finance/time/etc.); [docs/product/vision.md](docs/product/vision.md) is the current yardstick.
- **[docs/product/](docs/product/)** — `feature-inventory.md` (40 decisions `D-1`…`D-40` — cite these IDs), `backlog.md` (gap analysis), `checklist.md`, `vision.md`, `specs/` (implementation-ready SPEC-01…04), `analysis/` (point-in-time audits, e.g. the 2026-07 code-verified gap audit).
- **[docs/architecture/](docs/architecture/)** — `diagrams.md` (Mermaid), `security.md` (auth/RBAC design, was `authoration.md`), `frontend.md`, and `deferred/access-policies.md` (the competing RBAC vision — see the schism note in the Account section).
- **[docs/guides/](docs/guides/)** dev setup · **[docs/reference/](docs/reference/)** Asynq event/task registry.

## Stack & decisions

Self-hosted media + ecosystem monorepo (movies / music / stories / comics). Resolved choices:

- **Backend: Go modular monolith.** Three binaries — `cmd/api` (Chi HTTP server), `cmd/worker` (Asynq consumer for FFmpeg transcode/thumbnail), `cmd/sysjobs` (cross-tenant batch, BYPASSRLS — planned). Domain code lives under `internal/modules/<name>/`; cross-cutting infrastructure under `internal/platform/`.
- **Reverse proxy: Traefik v3** — static config in [traefik/traefik.yml](traefik/traefik.yml), middleware in [traefik/dynamic.yml](traefik/dynamic.yml), routes via `docker-compose.yml` labels.
- **Job queue: Asynq** (not BullMQ — BullMQ is Node-only). Three priority queues: `transcode` (5), `thumbnail` (3), `default` (1).
- **API contract: OpenAPI** at [shared/openapi.yaml](shared/openapi.yaml) is the source of truth. Go server stubs (`oapi-codegen`) and TS client types (`openapi-typescript`) are both generated from it. Hand-editing generated files is forbidden.
- **Frontend: Next.js 15** (App Router, RSC), Tailwind v4, Zustand + TanStack Query, Vidstack for HLS playback. Two route groups — `(app)` (authenticated shell: home, `/upload`, `/library/*`) and `(public)` (`/login`, `/register`) — that are version-agnostic: actual page/component code lives in a version-switched `frontend/src/templates/v{N}/` tree (ported from the `template-main/portal` Blade reference), selected via `NEXT_PUBLIC_TEMPLATE_VERSION` through `templates/registry.ts`. Read [frontend/src/templates/README.md](frontend/src/templates/README.md) before adding a page or cutting a `v2`.
- **Data: Postgres 17 + PgBouncer**, **DragonflyDB** (Redis-compatible cache + Asynq broker), **MinIO** (dev origin, bind-mounted) + **Cloudflare R2** (prod). *(Same S3-compatible client either way — see the scope section / [ADR-04](docs/adr/04-storage-tier-budget.md).)*

## Backend module boundaries (read before editing across modules)

The full spec is [backend/MODULES.md](backend/MODULES.md). The load-bearing rule:

> **Modules talk to each other only through their `api/` package. They never import each other's `service/`, `handler/`, `repository/`, `query/`, or subdomain packages. They never JOIN across each other's tables.**

Layout:

```
backend/internal/
├── modules/             ← one bounded context per subdir
│   ├── account/         users, local password/JWT auth, RBAC, sessions, audit
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
- **These boundaries are CI-enforced.** [backend/.golangci.yml](backend/.golangci.yml) (depguard) + the `lint` job in [.github/workflows/ci.yml](.github/workflows/ci.yml) fail the build on: importing `internal/sysrepository` outside `cmd/sysjobs`, `platform/` importing any module, non-`account` code importing `account/rbac`, and cross-module internal imports. There's a per-real-module isolation rule (account, media); a new module adds its own block (template comment is in the file).

Adding a new module: follow the checklist in `backend/MODULES.md` §8 (create the subtree, add an `sqlc.yaml` block, write the migration with `000N_<name>_…` prefix, wire into both `cmd/api/main.go` and `cmd/worker/main.go`).

## Account module — auth + RBAC architecture (non-obvious)

The account module ([backend/internal/modules/account/](backend/internal/modules/account/)) is intentionally strict; behavior diverges from textbook RBAC in subtle ways.

> **RBAC schism — know this before touching auth.** Two access-control specs conflict: the **role-hierarchy** model documented here (built, in code) vs the **policy-bundle / file-gated-permission** model in `docs/architecture/deferred/access-policies.md` (specced, no code). [ADR-02](docs/adr/02-rbac-model-reconciliation.md) resolves it: **role-hierarchy is canonical for v1**; policy bundles + user groups layer *on top of* roles in a later phase — they don't replace them. Disregard `access-policies.md`'s "spec wins, adjust code" clause for v1.

> **Direction change (2026-07-05) — local password auth.** [ADR-06](docs/adr/06-local-auth-model.md) supersedes the OIDC-login decision: **Portal owns credentials (`users.password_hash`, Argon2id) and Authentik is dropped from the login path.** The token / refresh / RBAC / revocation / audit machinery below is **unchanged and reused** — only the identity-proof step (`/auth/login` + account creation) changed. Anything below that still says "OIDC / Authentik / callback / nonce" is retired; the Identity flow now reads as follows.

### Identity flow
1. **Local password auth.** No IdP in the login path. `POST /api/v1/auth/login {email, password}` looks the user up by email, verifies the password against `users.password_hash` (Argon2id, constant-time), checks `disabled_at`, and on success issues the tokens below and sets the cookies. `POST /api/v1/auth/register {email, password, display_name}` creates the account (or admin-provisioned). There is **no** `/auth/callback`, `state`, or `nonce` anymore.
2. **Two tokens:** short-lived JWT access token (5min, HS256, rotating `kid` keys) + long-lived random refresh token (256-bit, SHA-256-hashed at rest, 30d). *(Unchanged from the OIDC design.)*
3. **Cookies:** `portal_access` (Path=/, SameSite=Strict) and `portal_refresh` (Path=/api/v1/auth, SameSite=Strict) — both `HttpOnly Secure`. API clients use `Authorization: Bearer` headers instead.
4. **New responsibilities Portal now owns** (were Authentik's): password hashing, brute-force rate-limit + lockout on `/auth/login`, password policy, password reset (needs the notification module — specced in [SPEC-04](docs/product/specs/SPEC-04-notification-module.md); admin/CLI until then), and — later — MFA/step-up and "Login with Google". See ADR-06 §"New responsibilities".

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

## Current status

The v1 demo loop is **closed and committed**. `account` and `media` are the only modules actually wired end-to-end; `tenant`/`movie`/`music`/`story`/`comic` are scaffolded (`module.go` + `api/` + `README.md`) but inert — empty `repository/`, never constructed in `cmd/api/main.go`.

- **`cmd/api/main.go`** constructs `account.New(...)` and `media.New(...)` and mounts both via `MountHTTP` under `/api/v1`. New domain modules attach the same way (see `backend/MODULES.md` §8).
- **Repository adapters exist** for `account` and `media` (`internal/modules/{account,media}/repository/`, sqlc-generated + hand-written adapter). Other modules' `repository/` dirs are still empty — that's the concrete signal a module hasn't been wired yet, not its `README.md`.
- **Tests:** `account/rbac/permission_test.go`, `account/auth/password_test.go`, `media/service_test.go` (in-memory fakes), `platform/storage/s3_test.go` (integration, gated on `S3_ENDPOINT`). `go test ./...` is green.

**`MILESTONE_CHECKS.md` (repo root) is the living status tracker — trust it over a doc's "open work"/"planned" section when they disagree.** ADRs and per-module `README.md` files describe the plan as of when they were written (e.g. `account/README.md` and `media/README.md` "Open work" sections predate the wiring landing; `media/README.md` still says the FFmpeg pipeline "logs and returns nil", which is no longer true). MILESTONE_CHECKS.md is updated as work actually lands.

**Known drift:** handlers are **hand-written**, not generated from `shared/openapi.yaml` (`make openapi` output isn't committed). The spec's auth paths were reconciled in 2026-07 (`/auth/register` added, retired `/auth/callback` removed), but CI's `openapi` job still only checks the spec **parses**, not that it matches handlers — so spec↔handler drift can still slip through. Don't assume the spec is authoritative for a module's current surface; verify against the code. (Wiring real drift-checking is on `MILESTONE_CHECKS.md`'s remaining list.)

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
| `make test` / `make test-backend` / `make test-frontend` | Test suites (`go test ./... -race -count=1` for backend, `vitest run` for frontend) |
| `make lint` | `golangci-lint run` + `pnpm lint` |
| `make certs` | Issue locally-trusted TLS certs for `*.portal.localhost` via `mkcert` (local HTTPS dev) |
| `make build` | Build production images for `api`, `worker`, `frontend` |

Single Go test: `cd backend && go test ./internal/modules/account/rbac -run TestMatches -v`

## Working in this repo

- **The OpenAPI spec is contract.** When adding an endpoint, edit `shared/openapi.yaml` first, then `make openapi`, then implement the generated handler interface. Don't write handlers that drift from the spec.
- **Don't hand-edit generated files**: any `internal/modules/*/repository/*.sql.go`, `internal/handler/api.gen.go`, `frontend/src/lib/types.gen.ts`.
- **Migration-only schema changes.** All DDL goes through `backend/db/migrations/` with `000N_<owning-module>_<description>.up.sql` naming. `query/*.sql` files contain DML/DQL only (sqlc consumes them) and live inside the owning module.
- **Never reach back to add a column to another module's table** — the owning module ships the migration after coordination.
- **System roles are protected.** Migration `0002_account_rbac` marks the seven default roles `is_system = true`; the `UpdateRole` / `DeleteRole` queries refuse to touch them. Don't override that flag without thinking about disaster recovery.
- **Cookie flags are environment-sensitive.** `COOKIE_SECURE=true` is the default; only flip to `false` for plain-`http://localhost` development. Do not commit a `.env` with `COOKIE_SECURE=false`.
- **`internal/sysrepository` (BYPASSRLS) is restricted to `cmd/sysjobs`** — enforced by depguard ([backend/.golangci.yml](backend/.golangci.yml) + CI `lint` job). Bypassing RLS in the API path would be catastrophic. (The package doesn't exist yet; the rule is a standing guardrail for when it lands.)

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **portal** (16900 symbols, 28327 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

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
