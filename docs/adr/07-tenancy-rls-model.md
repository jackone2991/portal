# ADR-07: Multi-tenancy & Row-Level Security model (Phase 1 — deferred from v1)

**Status:** Proposed — **deferred**. Phase 1, NOT in v1 scope ([ADR-01](./01-v1-scope-cut.md)). This is a design/plan to execute when Phase 1 starts; **no code lands until then.**
**Date:** 2026-07-07
**Deciders:** kirito
**Relates:** [ADR-01](./01-v1-scope-cut.md) (v1 cut) · [ADR-02](./02-rbac-model-reconciliation.md) (RBAC) · [ADR-03](./03-single-vps-topology.md) (single-VPS / PgBouncer) · [feature.md §2 + §18 Phase 1](../feature.md) · [D-23] [D-24] [D-25]

## Context

`feature.md §2` and the Phase 1 roadmap want multi-tenancy — `organizations` + `memberships` — with **Postgres Row-Level Security (RLS) as defense-in-depth** (authoration.md's L2): even a query that forgets `WHERE tenant_id = ?` must not leak cross-tenant data. v1 deferred all of it — there is **no `tenant_id` column anywhere today**, and the app is effectively single-user.

Three forces shape the design:

1. **Security posture.** The whole point of RLS is that the *database*, not the handler, is the last line. App-layer scoping alone is one forgotten filter away from a breach.
2. **The PgBouncer constraint (load-bearing).** Prod pools through **PgBouncer in transaction mode** ([ADR-03](./03-single-vps-topology.md)). RLS needs a per-request session variable; transaction-mode pooling reuses one connection across many transactions, so the naive "`SET app.tenant_id` once per connection" leaks one request's tenant into the next. (v1 dev sidesteps this by connecting **direct to `postgres:5432`** — see `.env` — because pgx's prepared-statement cache also clashes with transaction-mode PgBouncer. Phase 1 must resolve this deliberately.)
3. **Personal vs. org data.** Most Portal data is *personal* (a user's uploads, feed, bank); only some is org-shared. A **synthetic personal tenant** per user lets every row carry a `tenant_id` and keeps a single code path for both.

## Decision

Adopt **PostgreSQL RLS keyed on a per-request GUC, set with `SET LOCAL` inside a per-request transaction, on a non-owner application role with `FORCE ROW LEVEL SECURITY`.** Model tenancy as `organizations` (with a `kind` discriminator `'org' | 'household' | 'personal'`) + `organization_memberships`; give every user a synthetic **personal** org. Run cross-tenant batch work as a separate **`BYPASSRLS`** role, isolated to `cmd/sysjobs` by depguard.

> **GUC name:** the tenant skeleton already references `app.current_tenant`; the Phase-1 roadmap wrote `app.tenant_id`. **Pick one and use it everywhere** — this ADR standardises on **`app.current_tenant`** (matches the shipped skeleton comment). Fix `feature.md §18`'s `app.tenant_id` to match when Phase 1 lands.

### 1. Data model

- `organizations(id, kind, slug, name, owner_id → users(id), created_at, updated_at)` — `kind ∈ {'org','household','personal'}` **from day one** [D-24]; adding household later must not migrate a populated table.
- `organization_memberships(org_id, user_id, role, granted_at, ...)` — user ↔ tenant with a scoped role. Role granularity differs per kind: orgs → full RBAC hierarchy; households → `owner` + `member` (soft cap ~6); personal → single `owner`.
- **Every user gets a `personal` org at signup** (`kind='personal'`, `owner_id=user`, one owner membership). Personal routes (`/t/me/...`) resolve `me` → that org's id.
- **`users` stays GLOBAL** — a person is one identity across orgs (authoration.md). No `tenant_id` on `users`.
- **Tenant-scoped tables** carry `tenant_id UUID NOT NULL REFERENCES organizations(id)`: future domain tables (movie/music/story/comic), bank, social — **and `media.assets` gains `tenant_id`** (an upload belongs to the tenant context it was made in; `me` for personal). RBAC tables (`roles`, `permissions`) stay global; `user_roles` becomes membership-scoped (see §4).

### 2. RLS enforcement (per tenant-scoped table)

- A dedicated **app role `portal_app`** (`NOSUPERUSER NOBYPASSRLS`) that the API/worker connect as. **Critical:** superusers *and the table owner* bypass RLS unless `FORCE` is set — so the app role must **not** own the tables (own them as a migration/admin role, run as `portal_app`).
- Each tenant-scoped table, in the **same migration that creates it**:
  ```sql
  ALTER TABLE movies ENABLE ROW LEVEL SECURITY;
  ALTER TABLE movies FORCE  ROW LEVEL SECURITY;      -- applies even to the owner
  CREATE POLICY tenant_isolation ON movies
    USING      (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
  ```
  `USING` filters read/update/delete; `WITH CHECK` blocks writing a row into *another* tenant.
- **Fail closed:** `current_setting('app.current_tenant')` with no GUC set raises an error — a query that forgot to open a tenant scope *errors* rather than leaking. Use the 2-arg `current_setting(..., true)` (returns NULL) only where "no tenant ⇒ deny" is handled explicitly.

### 3. Connection strategy (the crux)

RLS-per-request under PgBouncer transaction pooling:

- **`SET LOCAL app.current_tenant = $1` inside a transaction.** `SET LOCAL` is transaction-scoped and reset at `COMMIT`/`ROLLBACK`, so a pooled connection never carries one request's tenant into the next. Plain `SET` (session-scoped) is **wrong** under transaction pooling.
- **Every tenant-scoped request runs in one transaction.** `platform/db.BeginTenantScope(ctx, tenantID)` opens `BEGIN; SET LOCAL app.current_tenant = $1;`, hands the tx to the request's queries, and `COMMIT`s (auto-`ROLLBACK` on handler error).
- **pgx ⨯ PgBouncer transaction mode** is incompatible with pgx's default prepared-statement caching → set the pool's `DefaultQueryExecMode = QueryExecModeExec` (or `SimpleProtocol`) / disable the statement cache. (This clash is why dev connects direct today.) Phase 1 chooses one of:
  - **(a) Route through PgBouncer (recommended)** — transaction mode + simple/exec protocol + `SET LOCAL` per request. Keeps the connection pool.
  - **(b) Direct-to-Postgres fallback** — keep `postgres:5432`, drop PgBouncer for the app; acceptable only while connection count stays well under `max_connections`.

### 4. Tenant resolution & RBAC

- URL scheme `/api/v1/t/{tenant}/...`; `{tenant}` = an org slug or the literal `me` [D-23].
- **Middleware order:** `RequireAuth` → **`RequireTenant`** (resolve `{tenant}` → tenant_id; `me` → caller's personal org; **verify the caller has a membership**, else 403) → `BeginTenantScope(ctx, tenant_id)` for the request tx → module handler.
- **Single-tenant deployments** map `/api/v1/...` (no `/t/`) to a default tenant via Traefik/middleware — the common case isn't uglier.
- **RBAC composes per-tenant:** effective permissions are computed **within the active membership** (admin in org A, member in org B). `user_roles` becomes `(user_id, org_id, role_id)`; the RBAC cache key gains the tenant ([ADR-02](./02-rbac-model-reconciliation.md)). Roles/permissions catalogs stay global.

### 5. Cross-tenant batch (BYPASSRLS)

- `cmd/sysjobs` connects as a **separate `portal_sys` role (`BYPASSRLS`)** for cross-tenant maintenance (purges, migrations, aggregate reports) via `internal/sysrepository`. **depguard blocks every other package from importing `sysrepository`** — a BYPASSRLS path reachable from the API would defeat RLS entirely (CLAUDE.md).

## Architecture model — request path

```mermaid
sequenceDiagram
    actor U as Client
    participant MW as API middleware
    participant DB as Postgres (portal_app role, RLS FORCEd)

    U->>MW: GET /api/v1/t/acme/movies  (cookie)
    MW->>MW: RequireAuth → identity
    MW->>DB: RequireTenant: resolve slug 'acme' + verify membership
    MW->>DB: BeginTenantScope: BEGIN; SET LOCAL app.current_tenant = '<acme-id>'
    MW->>DB: SELECT * FROM movies         (no WHERE tenant_id needed)
    Note over DB: RLS policy filters to tenant_id = current_setting('app.current_tenant')
    DB-->>MW: only acme's rows
    MW->>DB: COMMIT   (SET LOCAL discarded; connection safe to reuse)
    MW-->>U: 200
    Note over MW,DB: A forgotten filter still can't leak — the DB enforces it.
```

## Options considered

- **A — RLS + per-request `SET LOCAL` GUC *(chosen)*.** DB-enforced isolation; a buggy query can't leak. Cost: every tenant-scoped request is a tx; PgBouncer/pgx config care.
- **B — App-layer scoping only (`WHERE tenant_id = ?`), no RLS.** Simplest, no GUC/tx dance. **Rejected** — one forgotten filter = cross-tenant leak, exactly the failure RLS exists to prevent; unacceptable for bank/private data.
- **C — Schema-per-tenant / DB-per-tenant.** Hard isolation, but migration/ops cost explodes with many small *personal* tenants. **Rejected** for Portal's per-user tenant shape.
- **D — Connection-per-tenant with session `SET`.** Needs session-mode pooling → kills PgBouncer transaction-mode efficiency and blows up connection count. **Rejected.**

## Trade-off analysis

RLS + `FORCE` + fail-closed GUC is the strongest containment for the least code — but it imposes two rules every dev must internalise: **(1)** tenant-scoped queries run *inside* `BeginTenantScope`, and **(2)** the app connects as a **non-owner** role. The tx-per-request + simple-protocol is a real but bounded perf/complexity tax (measure it with the observability profile, which [ADR-03] says should land the same sprint). The synthetic `personal` tenant trades one `organizations` row per user for a single, fork-free code path across personal and org data.

## Consequences

- **New rule:** every tenant-scoped migration must `ENABLE + FORCE` RLS + a `tenant_isolation` policy **in the same migration** that creates the table — add this to the MODULES.md §6 schema-ownership checklist.
- **Retrofit:** `media.assets` gains `tenant_id` (backfill = each asset's owner's personal tenant + its policy). `users` unchanged (global). `user_roles` gains `org_id`.
- **Migration numbers:** the roadmap's `0010_rls_enable` predates the media migrations; actual files continue from `0007` → `0008_tenant_core`, `0009_rls_enable`.
- **GUC-name drift** between the skeleton (`app.current_tenant`) and feature.md (`app.tenant_id`) must be reconciled (this ADR picks `app.current_tenant`).
- **Testing:** an RLS test that asserts, on the `portal_app` role, tenant B cannot read tenant A's rows even via a raw `SELECT` with no `WHERE`.

## Implementation plan (when un-deferred)

1. [ ] DB roles: `portal_app` (`NOBYPASSRLS`, app connects as this) + `portal_sys` (`BYPASSRLS`, sysjobs only). Migration + compose/env; tables owned by a separate admin/migration role.
2. [ ] `0008_tenant_core`: `organizations`(+`kind`) + `organization_memberships`; **backfill a `personal` org + owner membership for every existing user**.
3. [ ] `platform/db.BeginTenantScope(ctx, tenantID)` + pool config for PgBouncer transaction mode (simple/exec protocol), or document the direct-connect fallback. (`platform/db/` is empty today — the pool is built inline in `cmd/api/main.go`.)
4. [ ] `0009_rls_enable`: `ENABLE + FORCE` RLS + `tenant_isolation` policy on every tenant-scoped table; add `assets.tenant_id` (+ backfill + policy).
5. [ ] `tenant` module: `RequireTenant` middleware, `GET /me/organizations`, `POST /auth/switch-tenant`, `POST/GET /admin/organizations`; wire in `cmd/api` **before** any domain module.
6. [ ] Per-tenant RBAC: `user_roles(user_id, org_id, role_id)`; effective-permission query + cache key scoped to the active membership ([ADR-02]).
7. [ ] `cmd/sysjobs` + `internal/sysrepository` (BYPASSRLS) + the depguard rule that blocks other importers.
8. [ ] RLS isolation test; MODULES.md §6 checklist entry.
9. [ ] Land the observability profile in the same sprint ([D-8], [ADR-03]) so per-tenant latency is visible; update `feature.md §2/§18` status + the `app.tenant_id`→`app.current_tenant` fix.

**Exit (Phase 1):** a request to `/api/v1/t/{org}/…` is tenant-scoped end-to-end; a raw query on the app role cannot read another tenant's rows; single-tenant deployments work without the `/t/` prefix; `sysjobs` (and only `sysjobs`) can cross tenants.
