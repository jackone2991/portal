# Tenant module

Owns the **organization** entity (the tenant boundary) and the middleware that pins `app.current_tenant` on the Postgres session for the lifetime of each request's transaction. This is the gatekeeper that makes RLS work.

## Subpackages

- `api/` — public surface (`Organization`, `IsMember`, `GetOrganization`)
- `query/` — sqlc input
- `repository/` — sqlc-generated
- (future) `service/`, `handler/`, `middleware/` for `RequireTenant`

## Owns these tables

`organizations`, `organization_memberships`, `organization_settings`.

## Talks to

- `platform/db` — runs `set_config('app.current_tenant', ...)` per request
- `account/api` — verifies the active user is a member of the org claimed in the JWT

## Emits events

- `tenant.organization.created`
- `tenant.organization.suspended`
- `tenant.organization.purged`

## Open work

The migration is `0018_tenant_core` (not `0004_tenant_organizations`), and
`RequireTenant` is not a skeleton — it wraps **all eleven** domain modules via
`authTenant` in `cmd/api/main.go`, and since 2026-08-25 the app connects as
`portal_app`, so its policies are actually enforced (see
[docs/guides/rls-cutover.md](../../../../docs/guides/rls-cutover.md)).

Genuinely open, and deliberately deferred at one user with one personal org:

- `POST /auth/switch-tenant` and `POST/GET /admin/organizations` (ADR-07 step 5)
- per-tenant RBAC — `user_roles(user_id, org_id, role_id)` (step 6)
- `cmd/sysjobs` + `internal/sysrepository` on the BYPASSRLS `portal_sys` role
  (step 7). `forEachTenant` in `cmd/worker` made this unnecessary for now: a
  sweep that iterates tenants needs no BYPASSRLS role.
- the `/t/{org}/…` URL contract (D-23)
