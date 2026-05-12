# Backend Module Conventions

> How code is organized into independent modules. Read this before adding any
> new domain (movies, music, stories, comics) or before reaching across an
> existing boundary.

---

## 1. Why modular

Portal is a **modular monolith**: one Go binary family (`cmd/api`, `cmd/worker`, `cmd/sysjobs`), but the source tree is split into **independently buildable, individually testable, narrowly-imported domain modules**. The goal is the same as microservices — clear ownership, blast-radius limits, parallel work — without the operational tax.

The single rule that makes this work:

> **Modules talk to each other only through their `api/` package. They never
> import each other's `service/`, `repository/`, `domain/`, or `query/`. They
> never join across each other's tables.**

If you find yourself wanting to do either, you have a missing API boundary; add it on the side you're crossing into.

---

## 2. Top-level layout

```
backend/
├── cmd/
│   ├── api/         ← HTTP server; wires every module's HTTP routes
│   ├── worker/      ← Asynq consumer; wires every module's task handlers
│   └── sysjobs/     ← cross-tenant batch (BYPASSRLS); restricted imports
│
├── internal/
│   ├── modules/     ← domain modules — one subdirectory per bounded context
│   │   ├── account/   (users, auth, RBAC, sessions, TOTP, audit)
│   │   ├── tenant/    (organizations, memberships, RLS bootstrap)
│   │   ├── media/     (assets, transcode + thumbnail workers — shared infra)
│   │   ├── movie/     (films, episodes)  ─┐
│   │   ├── music/     (tracks, albums)    │ depend on media for assets
│   │   ├── story/     (stories, chapters) │
│   │   └── comic/     (comics, chapters)  ─┘
│   │
│   └── platform/    ← cross-cutting infrastructure with no business logic
│       ├── config/    (env loading)
│       ├── db/        (pgx pool wrapper, BeginTenantScope)
│       ├── cache/     (Redis client + tenant-aware key helpers)
│       ├── storage/   (S3 client + tenant-prefixed key helpers)
│       ├── jobs/      (Asynq client setup, NOT specific tasks)
│       ├── server/    (HTTP utilities: write JSON, error helpers)
│       └── middleware/(rate limit, request ID, logging)
│
├── db/
│   └── migrations/  ← centralized; file names prefixed by owning module
└── sqlc.yaml        ← multi-block: one block per module's query/ dir
```

### Why centralized migrations + per-module queries

`golang-migrate` works best with one numeric sequence. Splitting migrations across modules complicates ordering when a later migration depends on an earlier module's schema. We keep migrations centralized but **prefix file names by owning module**:

```
0001_platform_init.up.sql        ← extensions, common types
0002_account_init.up.sql         ← users, refresh_tokens, audit_log
0003_account_rbac.up.sql         ← roles, permissions, ...
0004_tenant_organizations.up.sql ← organizations, memberships
0005_media_assets.up.sql         ← assets table
0006_movie_init.up.sql
0007_music_init.up.sql
0008_story_init.up.sql
0009_comic_init.up.sql
0010_rls_enable.up.sql           ← enables RLS on all tenant-scoped tables
```

Queries (sqlc input) **are** per-module — generated repository code stays inside each module's tree.

---

## 3. Anatomy of a module

Every module under `internal/modules/<name>/` follows this shape:

```
modules/account/
├── module.go         ← Wiring: New(deps) *Module; MountHTTP, RegisterTasks
├── domain.go         ← Types exported beyond the module (User, Role, etc.)
├── service/          ← Business logic; no DB driver imports
│   ├── auth.go       ← split by sub-domain
│   ├── rbac.go
│   └── ...
├── handler/          ← HTTP handlers; thin — delegates to service
│   └── auth.go
├── middleware/       ← module-specific middleware (RequireAuth, RequirePerm)
│   ├── auth.go
│   └── rbac.go
├── api/              ← PUBLIC interface (other modules import this)
│   └── api.go
├── query/            ← *.sql for sqlc (this module's queries only)
│   ├── users.sql
│   ├── rbac.sql
│   └── ...
├── repository/       ← sqlc-generated; do not hand-edit
│   └── *.sql.go
├── README.md         ← short: what this module owns + which modules it talks to
└── (subdomain dirs)  ← e.g. account/auth/, account/rbac/ — internal mechanisms
```

### `module.go`

The single registration point — every module exposes a constructor and a tiny set of lifecycle methods:

```go
package account

import (
    "github.com/go-chi/chi/v5"
    "github.com/hibiken/asynq"

    "github.com/portal/backend/internal/platform/db"
)

type Deps struct {
    DB     *db.DB
    Cache  CacheClient
    Audit  AuditClient
    // ... other shared infrastructure dependencies
}

type Module struct {
    deps Deps
    svc  *Service
    // a handle to the public API for external modules
    api  *API
}

func New(deps Deps) (*Module, error) { ... }

// MountHTTP registers the module's routes on the given router. The router
// should have the right auth/tenant middleware already applied at its parent.
func (m *Module) MountHTTP(r chi.Router) { ... }

// RegisterTasks attaches the module's Asynq handlers to the worker mux.
func (m *Module) RegisterTasks(mux *asynq.ServeMux) { ... }

// API returns the public interface other modules use to reach this module.
func (m *Module) API() *API { return m.api }
```

`cmd/api/main.go` constructs each module once and calls `MountHTTP`. `cmd/worker/main.go` constructs each module and calls `RegisterTasks`. The module decides which routes/tasks belong to it; main does not.

### `api/`

This is the **only package other modules may import**. It exposes function-level operations that hide the module's internals.

```go
// internal/modules/account/api/api.go
package accountapi

import (
    "context"

    "github.com/google/uuid"
)

type UserSummary struct {
    ID          uuid.UUID
    Email       string
    DisplayName string
}

// API is the public face of the account module.
// Implementations live inside the module; consumers depend only on the
// interface so test doubles are trivial.
type API interface {
    GetUserByID(ctx context.Context, id uuid.UUID) (*UserSummary, error)
    HasPermission(ctx context.Context, code string) (bool, error)
}
```

The interface is implemented by a struct in `internal/modules/account/api_impl.go` (or wherever — the module's choice). Other modules import `accountapi` for the type and call methods through it. No deeper imports.

---

## 4. Boundary rules (enforced by linter)

| Rule | Why |
|------|-----|
| `internal/modules/<X>/...` may import `internal/platform/...` | Platform is shared infrastructure, no business logic |
| `internal/modules/<X>/...` may import `internal/modules/<Y>/api/...` | Cross-module via public API only |
| `internal/modules/<X>/...` MUST NOT import `internal/modules/<Y>/{service,handler,repository,query,domain}` | Internals are private |
| `cmd/api/...`, `cmd/worker/...` may import any module's package | They are the wiring layer |
| `cmd/sysjobs/...` may import `internal/sysrepository` (BYPASSRLS) | Cross-tenant batch only |
| Anything else may NOT import `internal/sysrepository` | Bypassing RLS in the API would be catastrophic |

Enforce with `golangci-lint` `depguard`:

```yaml
linters-settings:
  depguard:
    rules:
      cross-module-private:
        list-mode: lax
        deny:
          - pkg: "github.com/portal/backend/internal/modules/*/service"
            desc: "import only via the module's api/ package"
          - pkg: "github.com/portal/backend/internal/modules/*/repository"
            desc: "import only via the module's api/ package"
          - pkg: "github.com/portal/backend/internal/modules/*/handler"
            desc: "handlers are private; cross-module calls go through api/"
      no-bypassrls:
        files: ["!**/cmd/sysjobs/**"]
        deny:
          - pkg: "github.com/portal/backend/internal/sysrepository"
            desc: "BYPASSRLS pool is restricted to cmd/sysjobs"
```

CI fails any PR that violates these. PR description must justify any depguard waiver.

---

## 5. Cross-module communication patterns

### 5.1 Synchronous: through `api/`

Movie module needs the asset URL → calls `mediaapi.API.GetAsset(ctx, assetID)`. The media module returns a struct; movie module uses it. Movie never queries `media.assets` directly.

### 5.2 Asynchronous: events via Asynq

For loose coupling, modules publish events. Conventions:

- Task type = `<emitting-module>:<event>`, e.g. `media:asset_ready`.
- Subscribers are other modules that registered handlers in their `RegisterTasks`.
- Payloads are stable; treat them as a versioned API.

Example: when transcode finishes, media emits `media:asset_ready { asset_id, hls_master_url }`. Movie module subscribes to update `movies.status = ready`. The two modules don't need to know each other's table schema.

### 5.3 No shared transactions across modules

If module X starts a tx, it does not call into module Y's service inside that tx. Y's writes belong in its own tx. If you genuinely need atomicity across modules, that's a design smell — promote the operation to an event, or fold the entities into one module.

---

## 6. Schema ownership

A table's owning module is the only module that:

- Writes migrations for it (file prefix = module name).
- Has SQL queries against it under its `query/`.
- Generates a Go repository for it.

Other modules that need to **read** another's table must:

- Call through the owning module's `api/` (preferred), OR
- Subscribe to events from the owning module.

Joining across owners is forbidden. If a query needs data from two owners, refactor to a service-level call that orchestrates both APIs.

### What about views?

Read-only **denormalized projection views** are allowed across owners, *as long as* they are owned by exactly one module. Example: a "discovery" module might own a materialized view that joins movies + music + stories for global search. The view is then a contract — its source modules emit events, the discovery module rebuilds.

---

## 7. Migration numbering

Single sequence, file name prefixed with owning module:

```
000N_<module>_<description>.up.sql
000N_<module>_<description>.down.sql
```

Example progression:

| Seq | Owner | What |
|-----|-------|------|
| 0001 | platform | extensions, common types |
| 0002 | account | users, refresh_tokens |
| 0003 | account | roles, permissions, audit_log |
| 0004 | tenant | organizations, memberships |
| 0005 | media | assets |
| 0006 | movie | movies, episodes |
| 0007 | music | tracks, albums |
| 0008 | story | stories, chapters |
| 0009 | comic | comics, chapters |
| 0010 | platform | RLS enable on every tenant-scoped table |

When two modules' migrations interleave (rare), the later module's may reference an earlier one's table; that's fine — migrations run in numeric order and the FK is intentional. **Never** reach back to add a column to another module's table; instead, the owning module ships the migration after coordination.

---

## 8. Adding a new module

Checklist:

1. `mkdir -p internal/modules/<name>/{service,handler,middleware,api,query,repository}`
2. Write `domain.go`, `module.go`, `api/api.go`.
3. Add a block to `sqlc.yaml`:
   ```yaml
   - engine: postgresql
     queries: "internal/modules/<name>/query"
     schema: "db/migrations"
     gen:
       go:
         package: "<name>repo"
         out: "internal/modules/<name>/repository"
         sql_package: "pgx/v5"
         emit_json_tags: true
         emit_interface: true
   ```
4. Write the first migration `000N_<name>_init.up.sql` + matching `down.sql`.
5. In `cmd/api/main.go`, construct the module and call `MountHTTP` under the right middleware chain.
6. In `cmd/worker/main.go`, call `RegisterTasks`.
7. Add `internal/modules/<name>/README.md` documenting: what the module owns, which modules it talks to, which events it emits/subscribes.

---

## 9. Anti-patterns (review-blocked)

- **Reaching into another module's `service/`** — almost always wrong. Add an `api/` method instead.
- **Cross-module SQL JOIN** — break the join into two service calls or move tables under one owner.
- **Shared "common" Go package with cross-module business types** — leads to a god module. Use module-specific types in `domain.go`; convert at the boundary.
- **Module that registers others' routes** — only `cmd/api` mounts routes. A module's `MountHTTP` registers only its own.
- **Skipping `module.go`** — every module has one constructor with explicit `Deps`. No globals, no `init()` magic.
