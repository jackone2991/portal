# Account module

Owns the identity + access-control story:

- Users (profile, lifecycle, disabled state)
- Authentication: OIDC login, JWT (access), refresh tokens with rotation + reuse detection
- 2FA / TOTP (planned)
- RBAC: roles, permissions, hierarchy, policies (planned)
- Session management
- Audit log (write-side, append-only)

## Subpackages

- `auth/` — JWT issuer/verifier, refresh manager, OIDC client, identity types
- `rbac/` — permission matcher, role catalog, Engine (decision point), cache
- `audit/` — append-only event logger (best-effort writes)
- `middleware/` — `RequireAuth`, `RequirePermission`, `RequireOwnerOrPermission`, `RequireRole`
- `handler/` — `/auth/*` HTTP handlers
- `api/` — public surface for other modules

## Owns these tables

`users`, `roles`, `permissions`, `role_permissions`, `user_roles`, `refresh_tokens`, `audit_log`.

## Talks to

- `platform/db` for the request-scoped tx
- `platform/cache` (Redis) for permission cache (key: `rbac:perms:<userID>:v<N>`)

## Emits events

- `auth.refresh.reuse_detected` — refresh-token theft alert (HIGH severity)
- `rbac.policy.updated` — when policies mutate (downstream invalidates cache + notifies users)

## Subscribes to

Nothing currently.

## Public API surface

See [api/api.go](api/api.go). Other modules MUST NOT reach into `auth`, `rbac`, `audit`, `handler`, or `middleware` directly.

## Open work

- Wire `cmd/api/main.go` to construct Issuer/Verifier/Refresh/Engine and call `MountHTTP`.
- Implement repository adapters (`UserUpserter`, `AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`) around sqlc-generated code.
- TOTP enrolment + step-up flow (see [authoration.md §2.4](../../../../authoration.md#24-totp--2fa-planned)).
- Policy + Group features (see [archivetech.md §3.1-3.3](../../../../archivetech.md)).
