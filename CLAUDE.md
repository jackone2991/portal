# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repo state

- `README.md` does not exist — this file is the primary written description of the project. [backend/MODULES.md](backend/MODULES.md) is the authoritative spec for backend module conventions; read it before adding a new domain or crossing an existing module boundary.
- `now.png` is a legacy architecture diagram from the original spec.
- `template-main/` is **reference material, not active code** — a Laravel/PHP portal scaffold and a static HTML social template. Don't edit, don't import. The Go scaffold under `backend/` is the real implementation.
- `scraper/` is a **separate Python service** (FastAPI + SeleniumBase/`undetected-chromedriver`), not part of the Go build — it scrapes an external comic source and hands zips to the comic import pipeline (SPEC-02 P1.8). Read [scraper/README.md](scraper/README.md) before touching comic sync. **Load-bearing detail:** chapters arrive *out of order*, so a chapter's `sort_order` is parsed from its **title**, not from arrival order (`chapterSortOrder` in [backend/internal/modules/comic/import.go](backend/internal/modules/comic/import.go)) — change the naming scheme and reader order scrambles.

## Project scope & constraints (read before planning work)

Everything below describes the **full, multi-year platform** — it is NOT the current scope. An evaluation pass ([docs/adr/](docs/adr/), ADRs 00–10) cut v1 down to a hard envelope: **1 developer · single VPS · ≤ $100/mo.** That envelope still holds, but the *module* scope has since expanded well past the original video-only cut — see "Current status". Social, creator economy, and marketplace remain out.

- **The original v1 (Phase 0 wiring + one video-upload happy path) is long closed.** ([01-v1-scope-cut.md](docs/adr/01-v1-scope-cut.md) scoped it.) That loop — local password sign-in → authenticated Next.js home → upload mp4 → MinIO (dev) / R2 (prod) → worker transcodes to HLS → `assets.status = ready` → Vidstack playback → revocable logout — still works and is the regression baseline. Everything since followed [ADR-08](docs/adr/08-life-os-pivot.md)'s life-OS pivot through SPEC-01…09.
- **Phase 0 wiring is closed, not a pending blocker** ([05-phase0-wiring-order.md](docs/adr/05-phase0-wiring-order.md) has the original plan/sequencing — useful for the *shape* of the work but stale on status): migrations audited/split, `make sqlc` run, repository adapters written, `account.New(...)` + `media.New(...)` constructed and mounted in `cmd/api/main.go`, local auth end-to-end, frontend auth gate wired. See "Current status" below for what's actually left.
- **Still deferred:** social (+ advanced social), creator economy, marketplace, ML safety, LiveKit/mediamtx, the 5-service observability stack. *(Bank is no longer deferred — it shipped as the `bank` module per [SPEC-03](docs/product/specs/SPEC-03-finance-ledger.md).)* Compose profiles `--profile observability`, `--profile live`, and `--calls` stay disabled.
- **Storage:** dev runs MinIO bind-mounted to `./data/minio`; prod uses R2. Same `platform/storage` S3-compatible client code either way ([ADR-04](docs/adr/04-storage-tier-budget.md) — the title says "R2-only" but its 2026-06-06 update note keeps MinIO for local dev, since presigned-URL uploads need an S3-speaking origin).

**Docs live in [docs/](docs/)** — restructured 2026-07-07 into a Diátaxis-style tree ([ADR-09](docs/adr/09-docs-architecture.md)); the old flat `doc/en` + `doc/vi` mirror is retired. **English-only now** — the frozen Vietnamese mirror is read-only in `docs/archive/vi-2026-07/`; never update it. Map:

- **[docs/adr/](docs/adr/)** — decision records 00–10 (`NN-*.md`). [ADR-08](docs/adr/08-life-os-pivot.md) repositioned Portal from Facebook-parity to a *life OS* (finance/time/etc.); [docs/product/vision.md](docs/product/vision.md) is the current yardstick. [ADR-10](docs/adr/10-openapi-contract-direction.md) made spec-first codegen CI-enforced.
- **[docs/product/](docs/product/)** — `feature-inventory.md` (decisions `D-1`…`D-41` — cite these IDs), `backlog.md` (gap analysis), `vision.md`, `specs/` (implementation-ready SPEC-01…09), `briefs/` (per-spec framing, `00`…`09`), `analysis/` (point-in-time audits).
- **[docs/architecture/](docs/architecture/)** — `diagrams.md` (Mermaid), `security.md` (auth/RBAC design, was `authoration.md`), `frontend.md`, and `deferred/access-policies.md` (the competing RBAC vision — see the schism note in the Account section).
- **[docs/testing/](docs/testing/)** — `TEST-PLAN.md`, `TRACEABILITY-MATRIX.md`, per-spec `TEST-CASES-SPEC-0N-*.md`, dated test runs.
- **[docs/guides/](docs/guides/)** dev setup + backup/restore · **[docs/operations/](docs/operations/)** Postgres tuning · **[docs/reference/](docs/reference/)** Asynq event/task registry.

## Stack & decisions

Self-hosted media + ecosystem monorepo (movies / music / stories / comics). Resolved choices:

- **Backend: Go modular monolith.** Binaries under `backend/cmd/`: `cmd/api` (Chi HTTP server), `cmd/worker` (Asynq consumer — see the queue split below), and `cmd/opsenqueue` (throwaway dev helper that enqueues one `ops:backup_database` so the restore drill can be exercised on demand; not part of the product). `cmd/sysjobs` (cross-tenant batch, BYPASSRLS) is **planned, not written**. Domain code lives under `internal/modules/<name>/`; cross-cutting infrastructure under `internal/platform/`.
- **Reverse proxy: Traefik v3** — static config in [traefik/traefik.yml](traefik/traefik.yml), middleware in [traefik/dynamic.yml](traefik/dynamic.yml), routes via `docker-compose.yml` labels.
- **Job queue: Asynq** (not BullMQ — BullMQ is Node-only). `cmd/worker` runs **three separate Asynq servers**, each with its own pool ([backend/cmd/worker/main.go](backend/cmd/worker/main.go)): **heavy** (`heavy` queue — video transcode, low concurrency, serialized), **image** (`image` queue — `media:process_image`, `IMAGE_CONCURRENCY`, default 3), and **light** (`thumbnail` weight 3 / `default` weight 1 — posters, notify fan-out, janitors). **Do not collapse these into one server with queue weights:** weights decide *which* queue gets polled, they cannot cap parallelism, and the heavy pool's low concurrency is the OOM guard (SPEC-01 P0.1). Adding a task means choosing a *server*, not just a queue name.
- **API contract: OpenAPI** at [shared/openapi.yaml](shared/openapi.yaml) is the source of truth. Go server stubs (`oapi-codegen`) and TS client types (`openapi-typescript`) are both generated from it. Hand-editing generated files is forbidden.
- **Frontend: Next.js 15** (App Router, RSC), Tailwind v4, Zustand + TanStack Query, Vidstack for HLS playback. Two route groups — `(app)` (authenticated shell: home, `/upload`, `/library/*`) and `(public)` (`/login`, `/register`) — that are version-agnostic: actual page/component code lives in a version-switched `frontend/src/templates/v{N}/` tree (ported from the `template-main/portal` Blade reference), selected via `NEXT_PUBLIC_TEMPLATE_VERSION` through `templates/registry.ts`. Read [frontend/src/templates/README.md](frontend/src/templates/README.md) before adding a page or cutting a `v2`. **[frontend/CLAUDE.md](frontend/CLAUDE.md) is the frontend conventions contract** — state-ownership boundary (`D-32`: server state → TanStack Query, never Zustand), the RSC-first rendering decision tree (`D-33`), and the `SessionKeeper` auth handoff (`D-34`). Read it before touching frontend code.
- **Data: Postgres runs on the *host* cluster** (PG 18), reached at `host.docker.internal:5432` — the `postgres` and `pgbouncer` compose services are **commented out** in [docker-compose.yml](docker-compose.yml), which documents the move and how to roll it back; the `postgres_data` volume is deliberately retained. `make up` therefore does **not** start a database — the host cluster must already be running. Plus **DragonflyDB** (Redis-compatible cache + Asynq broker), **MinIO** (dev origin, bind-mounted) + **Cloudflare R2** (prod). *(Same S3-compatible client either way — see the scope section / [ADR-04](docs/adr/04-storage-tier-budget.md).)*

## Backend module boundaries (read before editing across modules)

The full spec is [backend/MODULES.md](backend/MODULES.md). The load-bearing rule:

> **Modules talk to each other only through their `api/` package. They never import each other's `service/`, `handler/`, `repository/`, `query/`, or subdomain packages. They never JOIN across each other's tables.**

Layout:

```
backend/internal/
├── modules/             ← one bounded context per subdir (12, all wired)
│   ├── account/         users, local password/JWT auth, RBAC, sessions, audit
│   ├── tenant/          organizations, memberships, RLS bootstrap
│   ├── media/           assets + transcode/image/thumbnail workers (shared infra)
│   ├── movie/ music/ story/ comic/   ← depend on media for assets
│   ├── bank/            finance ledger (SPEC-03)
│   ├── journal/         life stream / activity feed (SPEC-05, SPEC-06)
│   ├── notify/          notification delivery fan-out (SPEC-04)
│   ├── people/          people registry, birthdays (SPEC-08)
│   └── ops/             platform ops: backup, retention, restore drill (SPEC-09)
└── platform/            config, db, cache, storage, jobs, middleware (no business logic)
```

Inside each module: `module.go` (the `New(Deps) *Module` constructor + `MountHTTP` + `RegisterTasks`), `api/` (public surface), `handler/`, `service/`, `middleware/`, `query/` (sqlc input), `repository/` (sqlc output — do not hand-edit).

- `cmd/api/main.go` and `cmd/worker/main.go` are the wiring layer; they construct each module and call `MountHTTP` / `RegisterTasks`. Modules do not register one another's routes.
- One documented exception to "api-only": `cmd/api` may grab `account.Module.Engine()` to build module-specific `RequirePermission` middleware. Other modules MUST NOT import `account/rbac` directly.
- Cross-module async coupling is via Asynq events named `<emitting-module>:<event>` (e.g. `media:asset_ready`). No shared transactions across modules.
- Schema ownership is per-module; reading another module's tables goes through its `api/` or via events, never a raw JOIN.
- **These boundaries are CI-enforced.** [backend/.golangci.yml](backend/.golangci.yml) (depguard) + the `lint` job in [.github/workflows/ci.yml](.github/workflows/ci.yml) fail the build on: importing `internal/sysrepository` outside `cmd/sysjobs`, `platform/` importing any module, non-`account` code importing `account/rbac`, and cross-module internal imports. There's a per-module isolation rule for all 12 modules; a new module adds its own block (template comment is in the file).

Adding a new module: follow the checklist in `backend/MODULES.md` §8 (create the subtree, add an `sqlc.yaml` block, write the migration with `000N_<name>_…` prefix, wire into both `cmd/api/main.go` and `cmd/worker/main.go`).

## Account module — auth + RBAC architecture (non-obvious)

The account module ([backend/internal/modules/account/](backend/internal/modules/account/)) is intentionally strict; behavior diverges from textbook RBAC in subtle ways.

> **RBAC schism — know this before touching auth.** Two access-control specs conflict: the **role-hierarchy** model documented here (built, in code) vs the **policy-bundle / file-gated-permission** model in `docs/architecture/deferred/access-policies.md` (specced, no code). [ADR-02](docs/adr/02-rbac-model-reconciliation.md) resolves it: **role-hierarchy is canonical for v1**; policy bundles + user groups layer *on top of* roles in a later phase — they don't replace them. Disregard `access-policies.md`'s "spec wins, adjust code" clause for v1.

> **Direction change (2026-07-05) — local password auth.** [ADR-06](docs/adr/06-local-auth-model.md) supersedes the OIDC-login decision: **Portal owns credentials (`users.password_hash`, Argon2id) and Authentik is dropped from the login path.** The token / refresh / RBAC / revocation / audit machinery below is **unchanged and reused** — only the identity-proof step (`/auth/login` + account creation) changed. Anything below that still says "OIDC / Authentik / callback / nonce" is retired; the Identity flow now reads as follows.

### Identity flow
1. **Local password auth.** No IdP in the login path. `POST /api/v1/auth/login {email, password}` looks the user up by email, verifies the password against `users.password_hash` (Argon2id, constant-time), checks `disabled_at`, and on success issues the tokens below and sets the cookies. `POST /api/v1/auth/register {email, password, display_name}` creates the account (or admin-provisioned). There is **no** `/auth/callback`, `state`, or `nonce` anymore.
2. **Two tokens:** short-lived JWT access token (5min, HS256, rotating `kid` keys) + long-lived random refresh token (256-bit, SHA-256-hashed at rest, **24h** — `REFRESH_TTL`, `platform/config`; the "30d" in the original ADR-06 narrative was never the shipped default). *(Unchanged from the OIDC design.)*
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

**Verify status against the code, not against prose.** There is no status-tracker file — `MILESTONE_CHECKS.md` was deleted in commit `f11cf3f`, and `docs/README.md` still links it (stale; ignore). The reliable signals:

- **Is a module wired?** It is constructed *and* mounted in [backend/cmd/api/main.go](backend/cmd/api/main.go) (`<name>.New(...)` plus `<name>Mod.MountHTTP(r)`), and its `repository/` dir is populated. **All 12 modules currently pass both checks** — `account bank comic journal media movie music notify ops people story tenant`. An empty `repository/` is the concrete signal a module is inert, not its `README.md`.
- **What runs in the background?** [backend/cmd/worker/main.go](backend/cmd/worker/main.go) — the `publisher.Subscribe(...)` calls are the cross-module event wiring, and each `Register*Tasks` call shows which of the three servers owns a task.
- **ADRs and per-module `README.md` "open work" sections are point-in-time**, written when the decision was made, and several have gone stale (e.g. `media/README.md` still says the FFmpeg pipeline "logs and returns nil" — untrue). Prefer the code, then `git log`, over any document's status claim.

**Tests:** 14 `_test.go` files — `account` (rbac / password / reset / handler), `bank`, `comic`, `journal`, `media`, `notify`, `ops` (retention / state), `people`, `platform/events`, and `platform/storage` (integration, gated on `S3_ENDPOINT`). Run `make test-backend`.

**Known drift — OpenAPI, narrower than it used to be.** [ADR-10](docs/adr/10-openapi-contract-direction.md) landed spec-first for real: `backend/internal/handler/api.gen.go` and `frontend/src/lib/types.gen.ts` are **committed**, and CI's `openapi` job runs `make openapi` then `git diff --exit-code`, so stale codegen now fails the build. What remains: **no handler implements the generated `ServerInterface` yet** (`api.gen.go` is its only referent in the tree). Handlers are still hand-written plain-chi, retrofitted module-by-module as each is touched. So the spec is guaranteed in sync with the *generated code*, not with *handler behaviour* — verify response shapes against the handler. (The known live example: handlers emit `{code, message}` via a local `writeErr` where the spec mandates RFC 7807 `Problem`.)

## Common commands

All from repo root via the [Makefile](Makefile):

| Command | What it does |
| --- | --- |
| `make up` / `make down` | Bring the docker-compose stack (Dragonfly, MinIO, Traefik, api, worker, frontend) up/down. `up` auto-creates `.env` from `.env.example` if missing. **Postgres is not in the stack** — it runs on the host cluster (see Stack & decisions). |
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
| `make restore-drill` | Exercise the backup/restore path end-to-end (see [docs/guides/backup-restore.md](docs/guides/backup-restore.md); `cmd/opsenqueue` triggers the backup task on demand) |

Also defined: `env`, `logs`, `ps`, `restart`, `dev-api` / `dev-worker` / `dev-frontend`, `openapi-go` / `openapi-ts`, `help`.

Single Go test: `cd backend && go test ./internal/modules/account/rbac -run TestMatches -v`

## Working in this repo

- **The OpenAPI spec is contract, and CI enforces it.** When adding an endpoint: edit `shared/openapi.yaml` first, run `make openapi`, **commit the regenerated `api.gen.go` + `types.gen.ts`**, then implement. Skipping that commit fails CI's `openapi` drift gate ([ADR-10](docs/adr/10-openapi-contract-direction.md)). Note the gate proves codegen is fresh, not that handlers match — see "Known drift".
- **Don't hand-edit generated files**: any `internal/modules/*/repository/*.sql.go`, `internal/handler/api.gen.go`, `frontend/src/lib/types.gen.ts`.
- **Migration-only schema changes.** All DDL goes through `backend/db/migrations/` with `000N_<owning-module>_<description>.up.sql` naming. `query/*.sql` files contain DML/DQL only (sqlc consumes them) and live inside the owning module.
- **Never reach back to add a column to another module's table** — the owning module ships the migration after coordination.
- **System roles are protected.** Migration `0002_account_rbac` marks the seven default roles `is_system = true`; the `UpdateRole` / `DeleteRole` queries refuse to touch them. Don't override that flag without thinking about disaster recovery.
- **Cookie flags are environment-sensitive.** `COOKIE_SECURE=true` is the default; only flip to `false` for plain-`http://localhost` development. Do not commit a `.env` with `COOKIE_SECURE=false`.
- **`internal/sysrepository` (BYPASSRLS) is restricted to `cmd/sysjobs`** — enforced by depguard ([backend/.golangci.yml](backend/.golangci.yml) + CI `lint` job). Bypassing RLS in the API path would be catastrophic. (The package doesn't exist yet; the rule is a standing guardrail for when it lands.)

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **portal** (24857 symbols, 47978 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

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
