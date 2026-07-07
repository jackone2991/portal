# Authoration — Authentication, Authorization, and Multi-Tenancy

> Canonical security specification for Portal. Covers identity (authn),
> permission decisions (authz), and tenant isolation (data segregation).
>
> **Companion docs:**
> - [archivetech.md](archivetech.md) — full functional roadmap (UI, modules, phasing)
> - [CLAUDE.md](../../CLAUDE.md) — architecture decisions + working agreement
> - [ADR-02](architecture/02-rbac-model-reconciliation.md) — role-hierarchy RBAC is canonical for v1; policy bundles layer on later
> - [ADR-06](architecture/06-local-auth-model.md) — local password auth; Authentik/OIDC removed
>
> For built (v1) surfaces, code + ADRs are canonical (ADR-02 explicitly
> disregards spec-wins clauses for v1); for the post-v1 layers specced here,
> this doc is the design of record. Update the doc in the same change-set as
> any behavior change.

> **Status (2026-07-06):** The identity layer (§2 — local password auth, tokens, two revocation
> channels, audit, login brute-force lockout) is **BUILT** and shipping in the closed v1 demo loop
> (see `MILESTONE_CHECKS.md`, [ADR-06](architecture/06-local-auth-model.md)). Everything
> tenant/policy-shaped — §1 L2 tenant layer, §2.4 TOTP, §3 tenancy+RLS, §4 policy-bundle
> authorization, §5.4 notifications, §6 steps 8–9, §9 migrations beyond 0007 — is **POST-V1 DESIGN**,
> not current behavior. For v1, role-hierarchy RBAC is canonical per [ADR-02](architecture/02-rbac-model-reconciliation.md).

---

## 0. Decision log

The settled answers to the open questions raised in `archivetech.md §9`:

| # | Question | Decision |
|---|----------|----------|
| 1 | Group-deletion UX | **TOTP step-up confirm**: user must enter a current code from Google/Microsoft Authenticator (or our enrolled equivalent) before the destructive op proceeds. Applies to all hard-delete operations. |
| 2 | "User Role" labels | **Cosmetic only.** Permissions come from policies attached to the group + user. The label is metadata. |
| 3 | Per-user policies | **Additive.** No deny rules in v1. |
| 4 | File-gated permission expiry | **Auto cut-off** at expiry + audit event. Re-opening is a manual admin action (re-review the file). |
| 5 | Policy mid-flight changes | **Instant invalidation** via `token_version` bump + cache key roll-forward. Affected users receive an in-app + push notification. |
| — | Multi-tenancy | **Shared DB + Row-Level Security (RLS)** keyed on `tenant_id`, with TOTP-required tenant switching. Schema-per-tenant deferred to "enterprise tier" later. |

---

## 1. Three-layer security architecture

Every request traverses three independently-enforced layers. Each layer answers exactly one question; they do not overlap. *(Target architecture — the shipped v1 wires L1 + role-hierarchy L3 only; the L2 tenant layer and RLS are post-v1. See the status banner above.)*

```text
                ┌──────────────────────────────────────┐
   Request ──►  │  L1 — IDENTITY                       │  Who is this principal?
                │  (auth middleware: JWT + DB snapshot)│  → auth.Identity in ctx
                └──────────────────────────────────────┘
                                │
                                ▼
                ┌──────────────────────────────────────┐
                │  L2 — TENANT                         │  In which organization are they
                │  (tenant middleware: org binding)    │  acting? Sets DB session var.
                │                                      │  → tenant.Context in ctx
                └──────────────────────────────────────┘
                                │
                                ▼
                ┌──────────────────────────────────────┐
                │  L3 — AUTHORIZATION                  │  Within that tenant, may they
                │  (rbac.Engine: policy resolution)    │  perform this action on this
                │                                      │  resource? → allow / deny
                └──────────────────────────────────────┘
                                │
                                ▼
                          Handler runs.
                          Postgres enforces RLS using app.current_tenant.
```

### Why three layers, not one

- L1 alone leaks data: authenticated ≠ authorized for *this* tenant's data.
- L1 + L3 alone is fragile: a missed permission check in a handler leaks tenant data. RLS at L2 is **defense in depth at the database** — even if a query forgets `WHERE tenant_id = $1`, Postgres refuses.
- The order matters: L2 depends on L1 (need a verified user to know which tenants they belong to); L3 depends on L2 (effective perms differ per tenant for users who belong to many).

---

## 2. Identity layer (authentication)

> **Superseded by [ADR-06](architecture/06-local-auth-model.md) (2026-07-05).** Portal now owns credentials and authenticates locally; Authentik is removed from the login path. The **token, refresh, RBAC, revocation, and audit** machinery in §2.2 onward is unchanged and reused — only this login subsection changes. Any remaining "OIDC / callback / nonce / Authentik" mentions elsewhere in this doc are retired.

### 2.1 Local password login flow  *([BUILT])*

Portal is the identity provider. Credentials live in `users.password_hash` (Argon2id). Flow:

1. `POST /auth/login {email, password, remember}` — server looks up the user by email, verifies the password against `users.password_hash` (Argon2id, constant-time), and checks `disabled_at`. `remember=true` → persistent refresh cookie (`Max-Age` = refresh TTL); `false` → session cookie.
2. On success, server issues access + refresh tokens, sets cookies, returns `200`. On failure it returns a generic `401` (no user-enumeration) and increments the brute-force counter.
3. `POST /auth/register {email, password, display_name}` — creates the account (Argon2id hash), assigns the default `user` role, and returns `201` **without** issuing a session; the user is redirected back to `/login` to sign in (product decision). Registration may be admin-gated depending on deployment.
4. There is **no** `/auth/callback`, `state`, or `nonce` — the browser never leaves the Portal domain.

New security responsibilities Portal now owns (were Authentik's): password hashing, brute-force rate-limit + lockout on `/auth/login`, password policy, and password reset (emailed token once the notification module lands; admin/CLI until then). MFA/step-up (§2.4) and "Login with Google" are now built in Portal, not configured in an IdP.

Key implementation: [password.go](../../backend/internal/modules/account/auth/password.go) + [handler/auth.go](../../backend/internal/modules/account/handler/auth.go).

### 2.2 Tokens

| Token | Lifetime | Storage | Algorithm | Purpose |
|-------|----------|---------|-----------|---------|
| Access | 5 min | `portal_access` cookie OR `Authorization: Bearer` | HS256 with rotating `kid` keys | Per-request authn |
| Refresh | 24 h in the current deployment (`REFRESH_TOKEN_TTL`; design allows up to 30 d) | `portal_refresh` cookie (`Path=/api/v1/auth`) OR JSON body | 256-bit random, SHA-256 hashed at rest | Mint new access token |
| Step-up (TOTP) | 5 min | session-bound; not a separate cookie | n/a — flag on the session record | Authorize destructive ops |

Cookies always: `HttpOnly; Secure; SameSite=Strict`. A third cookie exists: `portal_session` — a non-sensitive marker cookie (`Path=/`), read by the Next.js middleware for the route-level auth gate; its value encodes persistent (`p`) vs session (`s`). Implementation in [jwt.go](../../backend/internal/modules/account/auth/jwt.go) and [refresh.go](../../backend/internal/modules/account/auth/refresh.go).

### 2.3 Two revocation channels  *([BUILT])*

Both are needed; either alone is insufficient.

- **`users.token_version`** — bump it and every existing access token fails the DB snapshot check inside `RequireAuth`. Instant logout-all.
- **`refresh_tokens.revoked_at`** — refresh-token-side revocation. Rotation chain (`parent_id`/`replaced_by_id`) is linear; presenting an already-rotated token revokes the **entire** chain (forward + backward via recursive CTE) and emits an `auth.refresh.reuse_detected` event. Theft detection.

### 2.4 TOTP / 2FA  *([PLANNED])*

Per decision-log #1, every destructive admin action requires a fresh TOTP confirmation. Implementation:

- **Enrolment**: `POST /auth/totp/enroll` returns a base32 secret + provisioning URI (`otpauth://`). User scans with Google/Microsoft/Authy/etc. Confirm with one valid code → `users.totp_enrolled_at = now()`.
- **Verify**: `POST /auth/totp/verify` accepts a 6-digit code. Time window: ±1 step (30 s) to absorb clock drift. **Constant-time** comparison.
- **Recovery codes**: 10 single-use codes, hashed (Argon2id). Generated at enrolment; regenerated on demand. Each consumed code is immediately marked used.
- **Step-up flow**: a destructive endpoint requires `X-Step-Up-Token: <code>` header OR a session marked `stepped_up_at < 5 min ago`. The middleware rejects with `403 step_up_required` otherwise. Frontend prompts for a code, calls `POST /auth/totp/verify?intent=step-up`, then retries the original request.
- **Re-enrolment**: requires the current code OR a recovery code. Admins cannot reset TOTP for users (would defeat the purpose); users without their device must use a recovery code.

#### Schema delta for TOTP

```sql
ALTER TABLE users
    ADD COLUMN totp_secret_enc       BYTEA,         -- AES-GCM ciphertext
    ADD COLUMN totp_enrolled_at      TIMESTAMPTZ,
    ADD COLUMN totp_last_verified_at TIMESTAMPTZ;

CREATE TABLE totp_recovery_codes (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash     BYTEA NOT NULL,                   -- Argon2id of plaintext
    used_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX totp_recovery_codes_user_idx ON totp_recovery_codes(user_id);
```

The TOTP secret is encrypted using a process-level key derived from `TOTP_KMS_KEY` env (separate from JWT keys — different blast radius). Decrypted only when verifying.

### 2.5 Session management

- `GET /me/sessions` — list active refresh tokens with their IP + user-agent (already supported by `ListActiveRefreshTokensForUser`).
- `DELETE /me/sessions/{id}` — revoke a specific session. Useful when "I left my laptop at the office".
- `POST /auth/logout-all` — revoke every refresh + bump `token_version`. Use after suspected compromise.

### 2.6 Failure-mode mapping

The middleware emits a generic `401 unauthorized` for every authn failure. The actual reason is in audit, not the response body. This avoids token-state oracles.

| Internal error | HTTP | Audit action |
|----------------|------|--------------|
| `ErrTokenInvalid`     | 401 | `auth.token.invalid` |
| `ErrTokenExpired`     | 401 | (skipped — too noisy) |
| `ErrTokenRevoked`     | 401 | `auth.token.revoked` |
| `ErrUserDisabled`     | 401 | `auth.disabled_user_attempt` |
| `ErrTokenReused`      | 401 | `auth.refresh.reuse_detected` (HIGH SEVERITY) |
| Step-up missing/expired | 403 | `auth.stepup.required` |
| TOTP wrong            | 401 | `auth.totp.invalid` |

---

## 3. Tenant layer (data segregation)

### 3.1 Tenant model

A **Tenant** is the top-level data isolation boundary. In Portal, a Tenant ≡ an `organization`. Companies, hospitals, studios, archives — each gets one organization, with hard data segregation.

```text
                 ┌──────────────────┐
                 │   Organization   │  ◄── Tenant boundary. RLS enforces.
                 └────────┬─────────┘
        sub-orgs (opt.)   │
                          ▼
                 ┌──────────────────┐
                 │ Sub-organization │  hierarchical (parent_org_id), same tenant
                 └────────┬─────────┘
                          ▼
                 ┌──────────────────┐
                 │   User Group     │  see archivetech.md §3.1
                 └────────┬─────────┘
                          ▼
                 ┌──────────────────┐
                 │      User        │
                 └──────────────────┘
```

A user can be a member of multiple organizations (e.g., a freelance auditor working with several clinics). Each membership has its own role/policy assignments. The active organization for a session is part of the JWT and chosen at login or via explicit switch.

### 3.2 Schema for tenancy

```sql
CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            TEXT UNIQUE NOT NULL,           -- short slug e.g. 'acme-clinic'
    name            TEXT NOT NULL,
    parent_org_id   UUID REFERENCES organizations(id) ON DELETE RESTRICT,
    tier            TEXT NOT NULL DEFAULT 'standard', -- 'standard' | 'enterprise'
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE organization_memberships (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id           UUID NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    is_default        BOOLEAN NOT NULL DEFAULT false, -- chosen at login if no explicit
    invited_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    joined_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, user_id)
);
CREATE INDEX organization_memberships_user_idx ON organization_memberships(user_id);
```

### 3.3 `tenant_id` propagation

Every tenant-scoped table carries `organization_id` and a corresponding RLS policy.

| Table | `organization_id` column | RLS enforced |
|-------|-------------------------|--------------|
| `user_groups` | YES | YES |
| `user_group_members` | inherited via group | YES |
| `policies` | YES (system-wide policies use NULL — special case) | YES |
| `group_policy_attachments` | inherited via group | YES |
| `user_policy_attachments` | YES (denormalized for RLS) | YES |
| `assets` | YES | YES |
| `movies`, `music`, `stories` | YES | YES |
| `audit_log` | YES (NULL for system events) | YES |
| `users` | NO — global identity | n/a (read scoped via membership join) |
| `refresh_tokens` | NO — bound to user, not tenant | n/a |
| `organizations`, `organization_memberships` | n/a | special policies |

**Why `users` is global**: a person is a person across orgs. Their email identifies them once. Their access in a given org is mediated by `organization_memberships`. Denormalising `organization_id` onto users would force unique users per org — wrong shape.

### 3.4 PostgreSQL RLS — the enforcement mechanism

For every tenant-scoped table:

```sql
ALTER TABLE assets ENABLE ROW LEVEL SECURITY;

CREATE POLICY assets_tenant_isolation ON assets
    USING (organization_id = current_setting('app.current_tenant')::uuid);

CREATE POLICY assets_tenant_insert ON assets
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_tenant')::uuid);
```

Application middleware sets `app.current_tenant` per request, **before** any query runs:

```go
// pseudocode in tenant middleware
conn.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", tenantID)
```

`set_config(..., true)` makes the setting transaction-local — it does not leak across pooled connections. Combined with **PgBouncer in transaction pooling mode**, this is safe.

#### Bypass for system operations

A small set of background jobs (cross-tenant audit aggregation, billing rollups) need to read across tenants. They use a dedicated role `portal_system` for which `BYPASSRLS` is set, AND those operations live in a separate Go binary (`cmd/sysjobs/`) that never serves user traffic. **The API server never connects as this role.**

### 3.5 Tenant switching

A user with multiple memberships chooses one at login (default-selected if `is_default=true`). To switch:

```text
POST /auth/switch-tenant
  body: { "organization_id": "..." }
```

Server validates membership, **requires fresh TOTP if the user has it enrolled**, mints a new access token with the new `org_id` claim, and bumps `token_version` for the *previous* token (so it cannot be used to access the old tenant after switching).

### 3.6 Cross-tenant administrators

The `superadmin` role is *system-level*, not tenant-level. It exists in a virtual "system organization" (`organization_id = NULL` for matching system policies). Concretely:

- A user with `superadmin` can switch into any organization via `POST /auth/switch-tenant` without being a member, **with TOTP step-up always required**.
- Their session is flagged `system_impersonation = true`; every action is audited with both their identity and the impersonated tenant.
- Tenant admins can never grant themselves `superadmin`. Bootstrapping requires `cmd/admin grant-superadmin` (CLI), which itself requires a runtime secret unavailable to the API process.

---

## 4. Authorization layer

### 4.1 Access-control model recap

(Detailed in `archivetech.md §2`. Repeated here as the unit-of-decision for this document.)

```text
        Group hierarchy             Policies (reusable bundles)
        within an Org                attached to Group OR User
              │                                  │
              └──────────────┬───────────────────┘
                             ▼
                    Effective permission set
                     for (user, organization)
                             │
                             ▼
              rbac.Engine.Authorize(...)  ← single decision point
```

### 4.2 Effective permission resolution

For each `(user_id, organization_id)`, compute the set in this order, **per request, cached**:

1. Find the user's `organization_membership` for this tenant. If none → deny everything.
2. Find every User Group the user belongs to (via `user_group_members`).
3. For each group, walk the parent chain (group → parent group → root). Each ancestor's policies apply too.
4. Collect every **active** policy attached to any group on the path (`group_policy_attachments` JOIN `policies` on `is_active = true`).
5. Add every **active** policy attached directly to the user (`user_policy_attachments` scoped to the same org).
6. Expand each policy → permissions (`policy_permissions`). For permissions with `requires_file = true`, drop them unless the user has a corresponding `user_permission_files` row with `status = 'approved'` and `expires_at > now()`.
7. Apply wildcard / scope rules from [permission.go](../../backend/internal/modules/account/rbac/permission.go).

Cached in Redis under key `rbac:perms:<userID>:<orgID>:v<token_version>`. TTL 5 min. **Bumping `token_version` is the only canonical invalidation channel.** Note: the built v1 cache key is `rbac:perms:<userID>:v<token_version>` ([cache.go](../../backend/internal/modules/account/rbac/cache.go)) — the `<orgID>` segment is added when multi-tenancy lands.

### 4.3 Policy versioning + user notification

Per decision-log #5, when a policy mutates:

1. The mutation handler updates `policies` / `policy_permissions`.
2. It computes the **set of users affected** by joining `policies → group_policy_attachments → user_group_members → users` (transitively up the group hierarchy) and `user_policy_attachments → users`.
3. For each affected user, bump `users.token_version`. This invalidates their access tokens at the next request and rolls the cache key forward.
4. Enqueue an Asynq task `notify:policy_changed` per affected user. The notification worker:
    - Writes an in-app notification row (`notifications` table — to be defined).
    - Fires a Web Push if the user has subscribed (VAPID keys per `web_push_subscriptions`).
    - For high-impact changes (new permission grant/revoke that the user actively uses), also writes an audit event referencing the actor.

```text
Policy P changes
   │
   ├─► RLS-isolated find affected users (within owning org)
   ├─► Bump token_version for each
   ├─► For each: enqueue notify:policy_changed
   │        └─► in-app notification row
   │        └─► web push if subscribed
   └─► audit event: rbac.policy.updated
```

### 4.4 File-gated permissions — auto cut-off + audit

Per decision-log #4. Run as a periodic Asynq task (every 5 min):

```text
  cron: rbac:expire_files
   │
   ├─► SELECT user_permission_files
   │     WHERE status='approved' AND expires_at < now()
   │
   ├─► For each row:
   │     UPDATE ... SET status='expired'
   │     bump users.token_version
   │     audit: rbac.file.expired
   │     enqueue notify:perm_lost
```

Re-opening = an admin re-reviews (or the user re-uploads), file goes through `pending → approved` again. The old expired row stays in history; never deleted.

### 4.5 Group deletion with TOTP step-up

Per decision-log #1. The `DELETE /admin/groups/{id}` endpoint:

1. Requires permission `rbac:role:write` (or equivalent group-management perm) — standard authz.
2. Additionally requires step-up: either header `X-Step-Up-Token: <6 digits>` OR a session flag set within the last 5 min.
3. On success: cascade deletes children (per `archivetech.md`), bump `token_version` for all members of all affected groups, audit `rbac.group.deleted` with a metadata field listing every cascaded child group.
4. If the actor lacks TOTP enrolment, the endpoint returns `403 totp_required` and the frontend redirects to `/account/security` to enrol.

This same pattern (`requireStepUp`) wraps every other destructive op:
`DELETE /admin/policies/{id}`, `DELETE /admin/users/{id}`, `POST /auth/logout-all`, `POST /auth/switch-tenant` (when source org has elevated perms), `cmd/admin grant-superadmin`.

---

## 5. Cross-cutting concerns

### 5.1 Audit  *([BUILT] core; UI [PLANNED])*

Every security-sensitive event written to `audit_log` (append-only). See [audit/logger.go](../../backend/internal/modules/account/audit/logger.go). Action codes are dotted, e.g. `auth.login`, `rbac.policy.updated`, `tenant.switched`, `auth.totp.verified`. **Failures are loud but non-blocking** for the user request.

Add for multi-tenancy: every audit row carries `organization_id` (NULL for system events). Migration delta:

```sql
ALTER TABLE audit_log
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX audit_log_org_idx ON audit_log(organization_id, occurred_at DESC);
```

### 5.2 Rate limiting  *([BUILT] — see breakdown)*

The `/auth/login` brute-force counter + lockout is **[BUILT], Redis-backed** ([handler/auth.go](../../backend/internal/modules/account/handler/auth.go)). The generic per-IP token bucket is **[BUILT], in-memory** at [ratelimit.go](../../backend/internal/platform/middleware/ratelimit.go). Stricter buckets per `(IP, action)` for sensitive endpoints (TOTP verify: 5/min/IP+user, lockout 15 min after 5 failures) and a Redis-backed generic bucket (`redis_rate.Limiter`) are still [PLANNED].

### 5.3 Secrets handling

- JWT signing keys: `JWT_SIGNING_KEYS` env, rotating list. Active key signs new tokens; older keys remain valid for verification during the rotation window.
- TOTP encryption key: separate `TOTP_KMS_KEY` env. Different blast radius from JWT.
- ~~OIDC client secret: `OIDC_CLIENT_SECRET` — never logged.~~ *(retired — Authentik/OIDC removed per ADR-06)*
- In production: Doppler manages all of the above; deployment never reads `.env` from disk.

### 5.4 Notifications channel

Used by §4.3 policy-change notifications and §4.4 file-expiry notifications.

```sql
CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,                -- 'policy_changed' | 'perm_lost' | ...
    title           TEXT NOT NULL,
    body            TEXT NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_unread_idx
    ON notifications(user_id, created_at DESC)
    WHERE read_at IS NULL;

CREATE TABLE web_push_subscriptions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint        TEXT NOT NULL,
    p256dh_key      TEXT NOT NULL,
    auth_key        TEXT NOT NULL,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Frontend uses TanStack Query + an SSE channel `/me/notifications/stream` for real-time delivery, falling back to Web Push when the browser is closed (VAPID).

---

## 6. Middleware pipeline

In order — applied to every authenticated route:

```text
1. RealIP            ← preserve X-Forwarded-For (Traefik trusted)
2. RequestID         ← unique per request, in logs + audit
3. Recoverer         ← catch panics, return 500
4. Timeout(30s)      ← bounded request lifetime
5. CORS              ← origin allowlist from config
6. RateLimit         ← per-IP token bucket; stricter on /auth/*
7. RequireAuth       ← JWT + DB snapshot; sets auth.Identity in ctx
8. RequireTenant     ← [PLANNED] reads org_id from JWT; sets app.current_tenant on the DB conn; sets tenant.Context in ctx
9. RequireStepUp     ← [PLANNED] (optional, per-route) — verifies fresh TOTP for destructive ops
10. RequirePermission ← rbac.Engine.Authorize, returns 403 on deny
11. Handler
12. AuditMiddleware  ← (deferred) writes audit event for mutating ops
```

The v1 pipeline in production is steps 1–7 + 10 (`RequirePermission`); the tenant and step-up middleware (8–9) land with the tenancy/TOTP phases.

Public routes (e.g. `GET /movies` for guests) skip 7–10. A handful of "tenant-scoped but public-readable" routes use `OptionalAuth` + `RequireTenant`.

---

## 7. API surface (auth + tenant)

```text
# Authentication  [BUILT]
POST   /auth/login                     verify email+password; mint tokens
POST   /auth/register                  create account (Argon2id); returns 201, no session — user signs in via /auth/login
POST   /auth/refresh                   rotate refresh; mint access
POST   /auth/logout                    revoke current refresh; bump token_version
POST   /auth/logout-all                revoke all refresh; bump token_version  [step-up]

# 2FA (TOTP)  [PLANNED]
POST   /auth/totp/enroll               start enrolment; returns secret + QR URI
POST   /auth/totp/verify               verify code; activate enrolment OR perform step-up
POST   /auth/totp/recovery-codes/regen regenerate recovery codes  [step-up]
DELETE /auth/totp                      disenrol  [step-up + recovery-code]

# Tenant  [PLANNED]
GET    /me/organizations               list orgs the user belongs to
POST   /auth/switch-tenant             switch active org; mints new tokens  [step-up if elevated]

# Identity  [BUILT]
GET    /auth/me                        current user + roles + org context

# Sessions  [PLANNED]
GET    /me/sessions                    list active refresh tokens (devices)
DELETE /me/sessions/{id}               revoke a specific session  [step-up]

# Notifications  [PLANNED]
GET    /me/notifications               list (paginated, unread filter)
POST   /me/notifications/read          mark IDs read
GET    /me/notifications/stream        SSE channel for live updates
POST   /me/web-push/subscribe          register browser push subscription
DELETE /me/web-push/{id}               unsubscribe
```

OpenAPI source-of-truth at [shared/openapi.yaml](../../shared/openapi.yaml). Each endpoint annotates its required permission via `x-required-permission` and step-up requirement via `x-step-up: true`. Known drift (2026-07-06): `shared/openapi.yaml` is missing `/auth/register` and still lists the retired `/auth/callback`; the `x-required-permission` / `x-step-up` annotations are the target convention, not yet uniformly present.

---

## 8. Threat model

What we explicitly defend against, and how.

| Threat | Defence |
|--------|---------|
| Stolen access token | Short TTL (5 min) + DB snapshot check on every request (`token_version`) — instant revocation. |
| Stolen refresh token | Rotation per use + reuse detection burns the chain. Hashed at rest. |
| Session hijack via XSS | `HttpOnly` cookies; CSP enforced server-side. Never expose tokens to JS. |
| CSRF | `SameSite=Strict` cookies on all session cookies. Login is a same-origin `POST` (no cross-site redirect), so no `Lax` relaxation is needed. |
| Password brute force | Rate-limit + temporary lockout on `/auth/login` per IP and per account; generic `401` (no user enumeration); Argon2id (memory-hard) makes offline cracking expensive. |
| Cross-tenant data leak via app bug | RLS enforced in Postgres. `app.current_tenant` set transactionally per request. `BYPASSRLS` role isolated to a separate Go binary. |
| Privilege escalation by a tenant admin | `superadmin` is a system role, never grantable from tenant context. CLI bootstrap only. |
| TOTP brute force | 6-digit code + 5 attempts/15-min lockout per user; constant-time compare. Recovery codes are single-use, Argon2id-hashed. |
| Refresh-token replay across devices | Each refresh token records issuing IP + UA. Reuse from a different fingerprint emits a higher-severity audit event (still revokes chain). |
| Password DB dump | `password_hash` is Argon2id (64 MB, t=3, p=2) with per-user salt — memory-hard, no plaintext or reversible form stored. |
| TOTP secret extraction at rest | Encrypted with separate `TOTP_KMS_KEY`; only decrypted in-memory at verify time. |
| Audit log tampering | Append-only at app layer. Long-term retention to R2 archive bucket (immutable bucket policy). |
| Permission cache poisoning | Redis cache key includes `token_version` and `org_id`; mutations bump version → forces re-fetch from DB. |
| Insider with DB write access | `audit_log` replication to a write-once R2 bucket (separate credentials). Out-of-band log forwarding to SIEM. |

What we do **NOT** defend against (out of scope for v1):

- Compromise of the Postgres host itself (holds `password_hash`; mitigated by Argon2id, not eliminated).
- A determined administrator within a tenant exfiltrating their own tenant's data (legitimate usage).
- DDoS — handled at Cloudflare, not here.

---

## 9. Migration roadmap

Numbered to fit the existing migration sequence in `backend/db/migrations/` (single numeric sequence, `000N_<owning-module>_<description>` naming). Migrations 0001–0007 are applied (schema v7, 2026-07-06); tenancy/policy migrations start at 0008.

| # | File | Purpose |
|---|------|---------|
| 0001 | `0001_platform_init` | [BUILT/APPLIED] database extensions + shared helpers |
| 0002 | `0002_account_users` | [BUILT/APPLIED] users table (identity core, token_version, disabled_at) |
| 0003 | `0003_account_rbac` | [BUILT/APPLIED] roles (hierarchy) + permissions + role_permissions + user_roles |
| 0004 | `0004_account_sessions` | [BUILT/APPLIED] refresh_tokens with rotation-chain theft detection |
| 0005 | `0005_platform_audit` | [BUILT/APPLIED] append-only audit_log |
| 0006 | `0006_account_local_auth` | [BUILT/APPLIED] users.password_hash (ADR-06); drops user_oidc_roles |
| 0007 | `0007_media_assets` | [BUILT/APPLIED] media assets table (upload/transcode lifecycle) |
| 0008 | `0008_tenant_organizations` | [PLANNED] organizations + organization_memberships; RLS scaffolding |
| 0009 | `0009_account_user_groups` | [PLANNED] user_groups + user_group_members (org-scoped) |
| 0010 | `0010_account_policies` | [PLANNED] policies + policy_permissions + group_policy_attachments + user_policy_attachments |
| 0011 | `0011_account_file_gated_permissions` | [PLANNED] user_permission_files + review workflow |
| 0012 | `0012_account_totp` | [PLANNED] users.totp_*, totp_recovery_codes |
| 0013 | `0013_notification_core` | [PLANNED] notifications + web_push_subscriptions |
| 0014 | `0014_tenant_rls_enable` | [PLANNED] enable RLS + policies on every tenant-scoped table |
| 0015 | `0015_platform_audit_org` | [PLANNED] add organization_id to audit_log |

RLS is intentionally enabled in **a separate, late migration** so that earlier development can proceed without RLS hassle. Production deployment must include `0014_tenant_rls_enable` before the tenancy phase goes live; CI gate verifies it.

---

## 10. Implementation pointers

### 10.1 Tenant middleware skeleton  *([PLANNED])*

```go
// internal/modules/tenant/middleware/tenant.go  (module layout per backend/MODULES.md)
//
// RequireTenant resolves the active organization for the request, validates
// the user's membership, and binds app.current_tenant on the DB connection
// for the lifetime of the request's transaction.
func RequireTenant(memberships MembershipFetcher, db DB) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id, _ := auth.FromContext(r.Context())
            orgID := orgFromJWT(id) // claim 'org_id'
            if orgID == uuid.Nil {
                writeJSONError(w, 400, "tenant_missing", "no active organization")
                return
            }
            ok, err := memberships.IsMember(r.Context(), id.UserID, orgID)
            if err != nil || !ok {
                writeJSONError(w, 403, "tenant_denied", "not a member of this organization")
                return
            }
            // Bind RLS guard for the rest of this request.
            ctx, cleanup, err := db.BeginTenantScope(r.Context(), orgID)
            if err != nil {
                writeJSONError(w, 500, "internal", "tenant scope failed")
                return
            }
            defer cleanup()
            r = r.WithContext(tenant.WithOrg(ctx, orgID))
            next.ServeHTTP(w, r)
        })
    }
}
```

### 10.2 Step-up middleware skeleton  *([PLANNED])*

```go
// internal/modules/account/middleware/stepup.go  (module layout per backend/MODULES.md)
func RequireStepUp(verifier *auth.TOTPVerifier, store StepUpStore, ttl time.Duration) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id, _ := auth.FromContext(r.Context())
            // Path 1: header with a current TOTP code
            if code := r.Header.Get("X-Step-Up-Token"); code != "" {
                if err := verifier.Verify(r.Context(), id.UserID, code); err == nil {
                    store.MarkStepUp(r.Context(), id.TokenID, time.Now())
                    next.ServeHTTP(w, r)
                    return
                }
                writeJSONError(w, 401, "totp_invalid", "")
                return
            }
            // Path 2: session was stepped up recently
            if t, ok := store.LastStepUp(r.Context(), id.TokenID); ok && time.Since(t) < ttl {
                next.ServeHTTP(w, r)
                return
            }
            writeJSONError(w, 403, "step_up_required", "this action requires a fresh TOTP code")
        })
    }
}
```

### 10.3 RLS test discipline

Every PR that adds a tenant-scoped table must include an integration test that:

1. Inserts rows with `organization_id = A` while `app.current_tenant = A` — succeeds.
2. Switches to `app.current_tenant = B` — `SELECT *` returns 0 rows; `INSERT ... organization_id = A` is rejected.
3. Connects as `portal_system` (`BYPASSRLS`) — sees both tenants.

Place under the owning module's repository package (e.g. `backend/internal/modules/tenant/repository/rls_test.go` — repositories are per-module; there is no shared `internal/repository/`). Do not let CI green without these.

---

## 11. Glossary

- **Tenant** — a unit of data isolation. In Portal, ≡ Organization.
- **Step-up auth** — a fresh authenticator confirmation required for destructive operations, on top of an already-valid session.
- **RLS** — PostgreSQL Row-Level Security. Filter clauses applied automatically to every query against a table.
- **TOTP** — Time-based One-Time Password (RFC 6238). What Google/Microsoft Authenticator emit.
- **Effective permission set** — the deduplicated, file-gated, scope-aware union of all permissions granted to a user *for a particular organization*.
- **Token version (`tv`)** — a monotonic counter on `users` that, when bumped, invalidates every outstanding access token for that user without revoking individual tokens.
