# RLS cutover — running the app as `portal_app`

**Status:** executed 2026-08-25 · **Relates:** [ADR-07](../adr/07-tenancy-rls-model.md) · `backend/db/migrations/0020_platform_rls_enable.up.sql`

Row-Level Security has been in the schema since migration `0020`, but it was
**inert**: the app connected as `portal`, a superuser, and a superuser bypasses
even `FORCE ROW LEVEL SECURITY`. Every tenant policy was decoration. This guide
records the cutover that made it real, and what has to stay true for it to
remain real.

## What "cut over" means

| | Before | After |
|---|---|---|
| `DATABASE_URL` role | `portal` (superuser, BYPASSRLS) | **`portal_app`** (NOSUPERUSER, NOBYPASSRLS) |
| A query with no tenant scope | silently returns **every tenant's rows** | returns **zero rows**, or errors on write |
| A write with no tenant scope | succeeds, `tenant_id` from a subquery or default | **errors** — `unrecognized configuration parameter "app.current_tenant"` |
| Tenant isolation | asserted by application code only | enforced by **PostgreSQL** |

`BACKUP_DATABASE_URL` deliberately stays on `portal`. `pg_dump` must see every
row in every tenant; a backup taken through RLS would be a silently partial one.

## Prerequisites — all four now met

The `⚠️ DO NOT APPLY` banner on `0020` named a gate ("worker tenant-scoping,
increment 1b"). That gate was **partly** met and partly mis-stated, so the real
list is:

1. **Every request path opens a scope.** `RequireTenant`
   (`internal/modules/tenant/middleware/require_tenant.go`) wraps all eleven
   domain modules via `authTenant` in `cmd/api/main.go`. ✔ already true
2. **Every per-user worker write opens a scope.** `runInUserTenant` in
   `cmd/worker/main.go` covers journal (`stream_items`), notify
   (`notifications`) and people (`people_birthday_notices`). ✔ already true
3. **The media worker opens a scope.** ✔ **added by this cutover.**
   `media.Deps.RunInUserTenant` now reaches the transcode / image / poster
   workers through `worker.inTenant`, and the task payloads carry
   `owner_user_id` so the worker knows whose tenant to open.
4. **The periodic sweeps iterate tenants.** ✔ **added by this cutover.**
   `forEachTenant` in `cmd/worker/main.go` runs `media:purge_orphans` and
   `people:scan_birthdays` once per tenant, each in its own committed scope.

### The landmine the banner did not name

`InsertVariant` used to read `tenant_id` from a subquery:

```sql
VALUES ($1, (SELECT tenant_id FROM assets WHERE id = $1), $2, ...)
```

That works **only** under a role that bypasses RLS. Under `portal_app` the
`assets` policy filters the subquery to zero rows, the subquery yields `NULL`,
and `media_asset_variants.tenant_id NOT NULL` rejects every insert — so every
image variant and every video poster would have failed, silently, from the first
upload after the flip. The query now omits `tenant_id` and lets the column
DEFAULT resolve it from the enclosing scope.

## Steps

```sh
# 1. Confirm the schema is fully migrated (expects 30 or higher).
psql -U portal -d portal -c 'select version from schema_migrations;'

# 2. Give portal_app a login. Migration 0019 creates the role but not a password.
psql -U portal -d portal -c "ALTER ROLE portal_app WITH LOGIN PASSWORD '<password>';"

# 3. Prove isolation actually holds BEFORE flipping anything.
cd backend
RLS_TEST_ADMIN_URL='postgres://portal:<pw>@127.0.0.1:5432/portal?sslmode=disable' \
RLS_TEST_APP_URL='postgres://portal_app:<pw>@127.0.0.1:5432/portal?sslmode=disable' \
go test ./internal/platform/db -run TestRLS -v

# 4. Flip .env — DATABASE_URL only. Leave BACKUP_DATABASE_URL on portal.
#    DATABASE_URL=postgres://portal_app:<pw>@host.docker.internal:5432/portal?...

# 5. Restart the app containers (config change, no rebuild needed).
docker compose up -d --force-recreate api worker

# 6. Verify.
curl -k --resolve api.portal.localhost:443:127.0.0.1 https://api.portal.localhost/api/v1/healthz
docker compose logs --tail=50 api worker
```

## Verification performed on 2026-08-25

Steps 1–4 were executed. Step 3 passed **8 of 8** against the live host cluster
as `portal_app`:

| Assertion | Result |
|---|---|
| Tenant B cannot read tenant A's rows | pass |
| `WITH CHECK` blocks writing into another tenant | pass |
| `UPDATE` cannot relocate a row across tenants | pass |
| Tenant B's `DELETE` does not reach tenant A's row | pass |
| A write with no tenant scope fails loudly | pass — `unrecognized configuration parameter "app.current_tenant"` |
| `media_asset_variants` resolves `tenant_id` from the scope | pass — the landmine above |
| Shared NULL-tenant `bank_categories` visible to every tenant | pass |
| All 23 RLS tables carry a policy **and** `FORCE` | pass |

Steps 5–6 were **not** run: Docker Desktop was not running on the machine at the
time. The configuration is in place; the stack picks it up on the next
`docker compose up -d --force-recreate api worker`. Watch the first worker
run of `media:process_image` and `media:purge_orphans` in the logs.

## Rollback

One line, no schema change:

```sh
# .env
DATABASE_URL=postgres://portal:<pw>@host.docker.internal:5432/portal?...
docker compose up -d --force-recreate api worker
```

RLS goes inert again immediately, because the superuser bypasses it. Nothing
written under `portal_app` becomes unreadable — `tenant_id` is a real column
either way.

## What is still deferred, deliberately

The cutover closes ADR-07 steps 1–4 and 8. Steps 5–7 stay open **on purpose** at
n=1 users with one personal org each:

- `POST /auth/switch-tenant`, `POST/GET /admin/organizations` — multi-org UX
- per-tenant RBAC (`user_roles(user_id, org_id, role_id)`)
- the `/t/{org}/…` URL contract (D-23)
- `cmd/sysjobs` + `internal/sysrepository` on the BYPASSRLS `portal_sys` role —
  **`forEachTenant` is what made this unnecessary for now.** A sweep that
  iterates tenants needs no BYPASSRLS role at all. Revisit when the tenant count
  makes per-tenant iteration too slow, not before.

`portal_sys` exists in the database (created by `0019`, `rolbypassrls = true`)
and nothing connects as it. That is the correct state: the role is provisioned,
the depguard rule fencing `internal/sysrepository` to `cmd/sysjobs` is armed,
and neither is load-bearing yet.

## The rule that keeps this true

**Any new tenant-scoped table must ship its policy in the same migration that
creates it**, and any new worker write must go through `runInUserTenant` or
`forEachTenant`. `TestRLSEveryProtectedTableHasAPolicyAndForce` fails the moment
a table has RLS without a policy or without `FORCE`, so the schema half is
self-checking. The worker half is not — it is a review rule.
