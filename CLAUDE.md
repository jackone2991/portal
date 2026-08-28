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
- **Bulk music import (0038).** `/library/music` takes either many files or one zip. The two paths differ in more than convenience: multi-file uploads from the browser (per-file progress, names from the filename), while the zip goes to `music:import_zip` on the worker, which reads embedded tags with **ffprobe** — already a hard dependency for the video pipeline — so it recovers artist and album too. `frontend/src/lib/music.ts`'s `metaFromFilename` is a deliberate port of `titleFromFilename` in `music/import.go`; keep them in step or the same file gets two different names depending on the path. Unlike the comic importer it mirrors, there is **no poll-to-ready loop**: audio assets are marked ready inside `/complete` (no transcode), so an imported track is playable immediately. The worker opens the owner's tenant scope from the task payload before any query — `portal_app` cannot read an RLS table unscoped, it errors outright.
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
├── modules/             ← one bounded context per subdir (13, all wired)
│   ├── account/         users, local password/JWT auth, RBAC, sessions, audit
│   ├── tenant/          organizations, memberships, RLS bootstrap
│   ├── media/           assets + transcode/image/thumbnail workers (shared infra)
│   ├── movie/ music/ story/ comic/   ← depend on media for assets
│   ├── bank/            finance ledger (SPEC-03)
│   ├── journal/         life stream / activity feed (SPEC-05, SPEC-06)
│   ├── notify/          notification delivery fan-out (SPEC-04)
│   ├── people/          people registry, birthdays (SPEC-08)
│   ├── social/          connections between accounts (0037) — the social layer's first slice
│   ├── ops/             platform ops: backup, retention, restore drill (SPEC-09)
│   └── layout/          shell navigation menu + dashboard widget placement (0036)
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
1. **Local password auth.** No IdP in the login path. `POST /api/v1/auth/login {email, password}` looks the user up by email, verifies the password against `users.password_hash` (Argon2id, constant-time), checks `disabled_at` **and `approval_status`**, and on success issues the tokens below and sets the cookies. `POST /api/v1/auth/register {email, password, display_name}` creates the account (or admin-provisioned). There is **no** `/auth/callback`, `state`, or `nonce` anymore.
2. **Two tokens:** short-lived JWT access token (5min, HS256, rotating `kid` keys) + long-lived random refresh token (256-bit, SHA-256-hashed at rest, **24h** — `REFRESH_TTL`, `platform/config`; the "30d" in the original ADR-06 narrative was never the shipped default). *(Unchanged from the OIDC design.)*
3. **Cookies:** `portal_access` (Path=/, SameSite=Strict) and `portal_refresh` (Path=/api/v1/auth, SameSite=Strict) — both `HttpOnly Secure`. API clients use `Authorization: Bearer` headers instead.
4. **New responsibilities Portal now owns** (were Authentik's): password hashing, brute-force rate-limit + lockout on `/auth/login`, password policy, password reset (needs the notification module — specced in [SPEC-04](docs/product/specs/SPEC-04-notification-module.md); admin/CLI until then), and — later — MFA/step-up and "Login with Google". See ADR-06 §"New responsibilities".

### Registration requires approval (migration 0031)
`users.approval_status` is `pending` | `approved` | `rejected`, and **only `approved` may hold a session**. Register creates a `pending` row: the password works, but login answers 403 `account/account-pending`. The check is re-read on *every* request (it rides along in `GetUserAuthSnapshot`, next to `disabled_at`) and again on `/auth/refresh` — so revoking an approval kills a live session on its next call rather than at token expiry.

- **`users:approve` is granted to no role.** `superadmin` reaches it through the `*` wildcard, which is what makes approval superadmin-only out of the box. Grant it elsewhere from the matrix screen if a deployment wants that.
- **Bootstrapping an approver.** On an empty database the first account to register is auto-approved and given `superadmin` (`handler.Register`) — without it a fresh install deadlocks. An *existing* database has no superadmin, so `BOOTSTRAP_SUPERADMIN_EMAIL` names one; `cmd/api` re-asserts it on every start and writes nothing once it is already true (an unconditional write would bump `token_version` on every restart).
- A rejected account is **kept, not deleted**, so the same email cannot re-register past the refusal.
- **Registering notifies the approvers.** `handler.notifyApprovers` resolves everyone whose *effective* permissions satisfy `users:approve` and enqueues a `notify:dispatch` per person (`account.registration_pending`, in-app + an email override, capped and best-effort). Recipient resolution deliberately splits: SQL walks the role hierarchy and prefilters to codes whose resource segment is `users` or `*`, then `rbac.Set.AllowsCode` applies the real grammar — a superadmin's set is the literal `*`, so a `contains` test would have found nobody. **RBAC is not tenant-scoped** (no `org_id` on `roles`/`user_roles`; `organization_memberships.role` is an unrelated label), so this fan-out is global, not per-organization.

### Admin console (`/api/v1/admin/*`, mounted by the account module)
User directory + approval queue + the role × permission matrix. `account/module.go`'s `mountAdmin` route table **is** the authorization policy — read it there, not in the handlers. Three invariants live in `handler/admin.go` and are covered by `admin_test.go`:

1. **No escalation** — you cannot grant *or revoke* a permission you do not hold, and you cannot assign a role whose effective permissions exceed yours. Only a `*` holder may touch the `superadmin` role. Without this, any `rbac:role:write` holder could grant themselves `users:approve` and the approval gate evaporates.
2. **No self-edit** — your own roles / approval / enabled state are off limits (`account/self-target`), so the last superadmin cannot demote themselves.
3. **Cache re-key on every change** — role and permission edits bump `token_version` for everyone affected *including holders of descendant roles*; that is the RBAC cache key, so a revoke takes effect at once instead of after the TTL. It is not a logout: the refresh cookie is untouched.

### Layout module — the shell is data now (migration 0036)
`internal/modules/layout` owns the navigation menu and the dashboard widget rails, which used to be hardcoded arrays in the React bundle (`SidebarLeft`'s `ITEMS`, `HomeView`'s two columns). The seed reproduces that shell row for row, so applying the migration changes nothing visible.

The two halves are **not** symmetric, and the asymmetry is the design:

- **Menu items are free-form.** A link is data, so an admin can add, rename, reorder and delete them — including most of the seeded ones, half of which are inherited template decoration (Friend Groups, Community Badges, Account Stats) with no link behind them. `href` must be an in-app path starting with a single `/` — an absolute URL would turn the app's own navigation into an open-redirect surface shown to every user. Exactly **one** row is `is_system` and undeletable: `admin-layout`, the link to the editor itself, because "delete the door you are standing in" is worth refusing rather than explaining afterwards. The guard against wiping the menu entirely is the service's "cannot be empty" check, not a flag on every row.
- **Widgets are registry-backed.** `key` must match a component in `frontend/src/templates/v1/components/widget/registry.ts` — the database cannot conjure a component. The API refuses an unknown key rather than storing a hole, and there is no "add widget" button. Adding one means adding it to the registry *and* the table.

`GET /layout` is authenticated but not permission-gated (the shell cannot render without it) and is **filtered server-side** per caller, so an admin-only entry never reaches a bundle anyone can read. Both saves are whole-set and transactional; `position` is renumbered from array order, so two rows can never claim the same slot. The frontend keeps a static copy of the unrestricted seed as a fallback, so an API blip degrades the sidebar to "not editable", never to empty.

This is also the module that forced `accountapi.HasPermission` to be implemented — it had been a stub returning `false` since the interface was written.

**User CRUD carries three guards that are not visible in the route table** (`handler/admin_users.go`, covered by `admin_test.go`):

- **Create cannot bypass the approval gate.** The new account's `approval_status` comes from the CREATOR's authority, never the request body — a creator without `users:approve` produces a `pending` row. Roles are not settable at creation; they go through `SetUserRoles` so the no-escalation rule lives in one place.
- **Edit and delete cannot be a takeover.** Email is the login identifier, so acting on an account that holds permissions you lack is refused (403 `account/escalation`). Editing *yourself* is allowed — renaming locks nobody out — unlike roles and approval.
- **Delete cascades and says so.** Every FK to `users` is `ON DELETE CASCADE`, so it also destroys the account's assets, comics, movies, tracks, stories, ledger, journal entries, people and organizations; Postgres will not object and there is no undo. The API therefore requires `confirm_email` to match the target — enforced server-side, so a misclick or a replayed request cannot reach the DELETE — and the UI offers "disable" first. Delete, disable and revoke-approval additionally refuse to remove the **last** holder of `users:approve` (409 `account/last-approver`), which would leave signups nobody can ever let in.

`/auth/me` now also returns the caller's **effective permission codes**, so the frontend can hide affordances the API would refuse (`frontend/src/lib/session.ts` mirrors the wildcard grammar — role names stopped being a usable proxy once roles became editable).

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
[backend/internal/platform/audit/logger.go](backend/internal/platform/audit/logger.go) logs and swallows errors — a DB hiccup must not abort the user request. If audit reliability becomes load-bearing, route through Asynq with a dedicated queue. Don't make handlers depend on the return value.

## Current status

**Verify status against the code, not against prose.** There is no status-tracker file — `MILESTONE_CHECKS.md` was deleted in commit `f11cf3f`, and `docs/README.md` still links it (stale; ignore). The reliable signals:

- **Is a module wired?** It is constructed *and* mounted in [backend/cmd/api/main.go](backend/cmd/api/main.go) (`<name>.New(...)` plus `<name>Mod.MountHTTP(r)`), and its `repository/` dir is populated. **All 12 modules currently pass both checks** — `account bank comic journal layout media movie music notify ops people story tenant`. An empty `repository/` is the concrete signal a module is inert, not its `README.md`.
- **What runs in the background?** [backend/cmd/worker/main.go](backend/cmd/worker/main.go) — the `publisher.Subscribe(...)` calls are the cross-module event wiring, and each `Register*Tasks` call shows which of the three servers owns a task.
- **ADRs and per-module `README.md` "open work" sections are point-in-time**, written when the decision was made, and several have gone stale (e.g. `media/README.md` still says the FFmpeg pipeline "logs and returns nil" — untrue). Prefer the code, then `git log`, over any document's status claim.

**Tests:** 14 `_test.go` files — `account` (rbac / password / reset / handler), `bank`, `comic`, `journal`, `media`, `notify`, `ops` (retention / state), `people`, `platform/events`, and `platform/storage` (integration, gated on `S3_ENDPOINT`). Run `make test-backend`.

**Known drift — OpenAPI, narrower than it used to be.** [ADR-10](docs/adr/10-openapi-contract-direction.md) landed spec-first for real: `backend/internal/handler/api.gen.go` and `frontend/src/lib/types.gen.ts` are **committed**, and CI's `openapi` job runs `make openapi` then `git diff --exit-code`, so stale codegen now fails the build. What remains: **no handler implements the generated `ServerInterface` yet** (`api.gen.go` is its only referent in the tree). Handlers are still hand-written plain-chi, retrofitted module-by-module as each is touched. So the spec is guaranteed in sync with the *generated code*, not with *handler behaviour* — verify response shapes against the handler. (The long-standing example — handlers emitting `{code, message}` via a local `writeErr` — is **closed as of 2026-08-27**: `internal/platform/server` is now the single error writer, the four surviving `writeErr`/`writeError` shims delegate to `server.Problem`, and the last raw emitters in `account/middleware`, `platform/middleware/ratelimit.go` and `cmd/api`'s `/continue` route were retrofitted. `schemas/Error` is deprecated and referenced by no operation.)

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
