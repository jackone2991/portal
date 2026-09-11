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

None listed here on purpose. Implementation status has one written owner
(`/CLAUDE.md` § Current status) and open work one list
(`docs/product/backlog.md`) — ADR-11. A status claim in a module README was
wrong within weeks every time it was tried (the 2026-08-25 audit found seven
of eight sections stale).

