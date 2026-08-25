# Account module

Owns the identity + access-control story:

- Users (profile, lifecycle, disabled state)
- Authentication: local password login (Argon2id, [ADR-06](../../../../doc/en/architecture/06-local-auth-model.md)), JWT (access), refresh tokens with rotation + reuse detection
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

- **Brute-force protection on `/auth/login`.** ADR-06 assigned this to Portal
  when Authentik was dropped. `internal/platform/middleware/ratelimit.go` is
  written and has **zero importers** — `cmd/api` supplies no limiter. For an
  internet-facing single-VPS deployment this is the highest-severity security
  gap in the tree.
- **TOTP enrolment + step-up flow** (D-27/D-28). Deliberately deferred: the
  decisions gate MFA on *real-bank* credentials, which are themselves deferred —
  the manual ledger holds none.
- **`api/` package is dead.** `account/api` has zero cross-module importers and
  its `HasPermission` is an unconditional `return false`. The real cross-module
  seam is the injected-function pattern the wiring layer builds.
