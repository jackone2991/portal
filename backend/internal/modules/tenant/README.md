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

- Migration `0004_tenant_organizations.up.sql` + RLS scaffolding
- `RequireTenant` middleware (skeleton in [authoration.md §10.1](../../../../authoration.md))
- `POST /auth/switch-tenant` flow with TOTP step-up
- Tenant lifecycle endpoints (onboarding, suspension, hard-delete cron in `cmd/sysjobs/`)
