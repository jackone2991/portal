# ADR-07: Multi-tenancy & Row-Level Security model

**Status:** **accepted** 2026-07-07 · schema and runtime executed 2026-08-25 (plan steps 1–4, 8); steps 5–7 deferred by scope
**Last verified:** 2026-09-11
**Deciders:** kirito
**Relates:** [ADR-01](./01-v1-scope-cut.md) (v1 cut) · [ADR-02](./02-rbac-model-reconciliation.md) (RBAC) · [ADR-03](./03-single-vps-topology.md) (single-VPS) · [feature-inventory.md §2 + §18 Phase 1](../product/feature-inventory.md) · [D-23] [D-24] [D-25] · runbook: [operations/rls-cutover.md](../operations/rls-cutover.md)

## Context

*As found on 2026-07-07, when this was a plan. Everything the plan needed
from the schema has since shipped (`0018_tenant_core`, `0019_platform_rls_roles`,
`0020_platform_rls_enable`, and every tenant-scoped table since carries its own
policy). Two premises below have moved: Postgres no longer runs in compose
(host cluster since 2026-08-21) and PgBouncer is gone with it, so "the
PgBouncer constraint" no longer binds — but the `SET LOCAL`-per-transaction
design it forced is what shipped, and it is correct without a pooler too.
Where RLS is actually enforced today is under Consequences; read that before
trusting any other statement about RLS in this repo.*

`feature.md §2` and the Phase 1 roadmap wanted multi-tenancy — `organizations` + `memberships` — with **Postgres Row-Level Security (RLS) as defense-in-depth** (`architecture/security.md`'s L2): even a query that forgets `WHERE tenant_id = ?` must not leak cross-tenant data. v1 had deferred all of it — there was **no `tenant_id` column anywhere** at the time, and the app was effectively single-user.

Three forces shaped the design:

1. **Security posture.** The whole point of RLS is that the *database*, not the handler, is the last line. App-layer scoping alone is one forgotten filter away from a breach.
2. **The PgBouncer constraint (load-bearing at the time).** Prod was to pool through **PgBouncer in transaction mode** ([ADR-03](./03-single-vps-topology.md)). RLS needs a per-request session variable; transaction-mode pooling reuses one connection across many transactions, so the naive "`SET app.tenant_id` once per connection" leaks one request's tenant into the next. (Dev sidestepped this by connecting **direct to `postgres:5432`**, because pgx's prepared-statement cache also clashes with transaction-mode PgBouncer.)
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

**Where RLS stands (2026-09-11) — the one statement to trust:**

RLS is enforced **if and only if the binary's `DATABASE_URL` connects as
`portal_app`**. The policies exist on every tenant-scoped table
(`grep -ho 'ALTER TABLE [a-z_]* FORCE ROW LEVEL SECURITY' backend/db/migrations/*.up.sql | sort -u | wc -l`
— 29 tables at last check) and `portal_app` is `NOSUPERUSER NOBYPASSRLS` and
does not own them, so `FORCE` binds. But `portal` — the migration role — is a
superuser and bypasses every policy.

- **This deployment's `.env`** points `DATABASE_URL` at `portal_app`
  (`grep DATABASE_URL .env`; cutover 2026-08-25, [runbook](../operations/rls-cutover.md)).
  Here, RLS is live. `BACKUP_DATABASE_URL` stays on `portal` on purpose —
  `pg_dump` must see every tenant.
- **`.env.example` defaults `DATABASE_URL` to `portal_app`** since 2026-09-11
  (with the placeholder password `0019` seeds), so a fresh `make up` starts
  enforced. The evidence that no query relied on superuser rights: this
  deployment ran as `portal_app` from 2026-08-25 through the SPEC-10 debts
  work, and the 19 RLS tests pass against the live cluster
  (`go test ./internal/platform/db -run TestRLS`, run 2026-09-11). The
  superuser `portal` is used only by `MIGRATE_DATABASE_URL` (`make migrate`)
  and `BACKUP_DATABASE_URL` (`pg_dump`).
- Three comments in the tree still describe the pre-cutover state as current
  and should be read as history, not status: the `platform/db/db.go` package
  comment ("set but unenforced because the app connects as a superuser") and
  the headers of migrations `0019` and `0020` ("**INERT** until …"). Applied
  migrations are not edited; the `db.go` comment is corrected with this ADR.

**What shipped, against the plan:**

- **Rule kept:** every tenant-scoped migration since `0020` has `ENABLE + FORCE` RLS and a `tenant_isolation` policy in the migration that creates the table (every table-creating migration from `0021` to `0043` does). The rule is **not** written into `backend/MODULES.md` § 6 — action item.
- `media.assets` gained `tenant_id` with backfill and policy (`0020`); `users` is global; `user_roles` did **not** gain `org_id` — RBAC is not tenant-scoped (see below).
- Migration numbers: the plan's `0008`/`0009` became `0018_tenant_core` / `0020_platform_rls_enable`, with `0019_platform_rls_roles` between them.
- GUC name: `app.current_tenant` in every migration and in `platform/db`. `feature-inventory.md` still writes `app.tenant_id` in three deliverable bullets under a note that says ADR-07's name supersedes it — legible, not fixed.
- `platform/db` is real: `NewPool`, `BeginTenantScope`, `WithTx`/`TxFrom`, and `Conn` (a sqlc `DBTX` that routes each query onto the request transaction when one is bound). `QueryExecModeExec` is set — which is also why `[]byte` into a `jsonb` column needs the Go type, not a cast (`jsonb_param_test.go`).
- **Tenant resolution as built:** no `/t/{tenant}` URL prefix. Routes stay under `/api/v1`; `RequireTenant` (tenant module middleware) resolves the caller's personal org and opens the request transaction. Middleware order is `RequireAuth → RequireTenant → handler`, as designed; only the URL contract differs.
- **A tenant transaction turns a raised constraint violation into a 500 at COMMIT**, not a 409: the handler's error response is written, then the transaction fails to commit. The house pattern is `ON CONFLICT DO NOTHING` + treat no-rows as conflict, or a pre-check before an UPDATE.
- **Cross-tenant batch without BYPASSRLS:** `cmd/worker` runs periodic sweeps through `ForEachTenant` (one committed tenant scope per organisation) — `people:scan_birthdays`, `bank:scan_debts_due`. That removed the need for `portal_sys`, `cmd/sysjobs` and `internal/sysrepository`; the role exists (`0019`), the binary and package do not, and depguard keeps the guardrail for when they land.
- **Testing:** `platform/db/rls_test.go` (+ `rls_media_test.go`, `rls_social_test.go`) assert on the `portal_app` role that tenant B cannot read tenant A's rows via a raw `SELECT`, that a write with no scope fails loudly, and that NULL-tenant shared rows (`bank_categories` seeds) are visible to all. They run only when `RLS_TEST_ADMIN_URL` / `RLS_TEST_APP_URL` are set — CI does not set them, so **CI does not exercise RLS**.
- The observability profile did not land with tenancy (ADR-03).

## Implementation plan

1. [x] DB roles `portal_app` (`NOBYPASSRLS`) + `portal_sys` (`BYPASSRLS`) — `0019`. Tables owned by `portal`. Runtime cutover to `portal_app` done here 2026-08-25; **not** the `.env.example` default.
2. [x] `0018_tenant_core`: `organizations`(+`kind`) + `organization_memberships`; personal org + owner membership backfilled for every existing user; `CreatePersonalOrg` on register.
3. [x] `platform/db.BeginTenantScope` + pool config (`QueryExecModeExec`). The PgBouncer branch is moot — there is no pooler; the app connects direct to the host cluster.
4. [x] `0020_platform_rls_enable`: `ENABLE + FORCE` + `tenant_isolation` on every tenant-scoped table; `assets.tenant_id` + backfill + policy.
5. [ ] `tenant` module beyond the personal org: `GET /me/organizations` exists (`listOrganizations`); `POST /auth/switch-tenant` and `/admin/organizations` do not. Deferred at one user, one personal org.
6. [ ] Per-tenant RBAC (`user_roles(user_id, org_id, role_id)`, cache key scoped to membership) — not done; RBAC is global. Deferred with 5.
7. [ ] `cmd/sysjobs` + `internal/sysrepository` — not written; `ForEachTenant` made it unnecessary so far. The depguard rule stays.
8. [x] RLS isolation tests (8 in `rls_test.go`, plus media and social suites). MODULES.md § 6 checklist entry — **not done**.
9. [ ] Observability profile — not landed. `feature-inventory.md` GUC bullets — still `app.tenant_id` under a superseding note.
10. [x] `.env.example`'s `DATABASE_URL` defaults to `portal_app` (2026-09-11), `MIGRATE_DATABASE_URL` added for the owner DSN. The `0019`/`0020` header language ("INERT until …") is corrected in the next migration that touches those tables, not by editing applied files.

**Exit (as built):** a request under `/api/v1/…` is tenant-scoped end-to-end through `RequireTenant`; a raw query on `portal_app` cannot read another tenant's rows (tested, out of CI); there is no `/t/` prefix to make optional; cross-tenant work goes through `ForEachTenant` in the worker, not a BYPASSRLS role.
