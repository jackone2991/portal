# Account module

Owns the identity + access-control story:

- Users (profile, lifecycle, disabled state)
- Authentication: local password login (Argon2id, [ADR-06](../../../../docs/adr/06-local-auth-model.md)), JWT (access), refresh tokens with rotation + reuse detection
- 2FA / TOTP (planned)
- RBAC: roles, permissions, hierarchy, policies (planned)
- Session management

Audit logging is **not** owned here anymore: it moved to `internal/platform/audit` (cross-cutting; every module is a consumer — [D-25]). The account module writes `account.*` events through it (see [MODULES.md §5.3](../../../MODULES.md#53-audit-event-type-taxonomy)).

## Subpackages

- `auth/` — JWT issuer/verifier, refresh manager, password hashing (Argon2id), identity types
- `rbac/` — permission matcher, role catalog, Engine (decision point), cache
- `middleware/` — `RequireAuth`, `RequirePermission`, `RequireOwnerOrPermission`, `RequireRole`
- `handler/` — `/auth/*` HTTP handlers
- `api/` — public surface for other modules

## Owns these tables

`users`, `roles`, `permissions`, `role_permissions`, `user_roles`, `refresh_tokens`. *(`audit_log` is owned by `platform/audit` — migration `0005_platform_audit`, per [D-25].)*

## Talks to

- `platform/db` for the request-scoped tx
- `platform/cache` (Redis) for permission cache (key: `rbac:perms:<userID>:v<N>`)

## Emits events

Audit events under the `account.*` taxonomy ([MODULES.md §5.3](../../../MODULES.md#53-audit-event-type-taxonomy)), notably:

- `account.refresh.reuse_detected` — refresh-token theft alert (HIGH severity)
- `account.role.*` / `account.permission.*` — RBAC mutations (downstream invalidates cache + notifies users)

## Subscribes to

Nothing currently.

## Public API surface

See [api/api.go](api/api.go). Other modules MUST NOT reach into `auth`, `rbac`, `handler`, or `middleware` directly.

## Open work

None listed here on purpose. Implementation status has one written owner
(`/CLAUDE.md` § Current status) and open work one list
(`docs/product/backlog.md`) — ADR-11. A status claim in a module README was
wrong within weeks every time it was tried (the 2026-08-25 audit found seven
of eight sections stale).

