# Authoration — Authentication, Authorization, and Multi-Tenancy

> Canonical security specification for Portal. Covers identity (authn),
> permission decisions (authz), and tenant isolation (data segregation).
>
> **Companion docs:**
> - [archivetech.md](archivetech.md) — full functional roadmap (UI, modules, phasing)
> - [CLAUDE.md](CLAUDE.md) — architecture decisions + working agreement
>
> When this document conflicts with code, this doc wins. Update the doc as
> part of the same change-set, never afterwards.

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

Every request traverses three independently-enforced layers. Each layer answers exactly one question; they do not overlap.

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

### 2.1 OIDC login flow  *([BUILT])*

Authentik is the IdP. No local password auth. Flow:

1. `GET /auth/login` — server generates `state` + `nonce`, sets short-lived signed `portal_oidc` cookie binding both, redirects to IdP.
2. IdP authenticates the user (any factor configured upstream — including Authentik's own MFA).
3. `GET /auth/callback` — server validates state (CSRF) and nonce (ID-token replay), exchanges code, upserts the user.
4. Server issues access + refresh tokens, sets cookies, redirects to the post-login URL.

Key implementation: [oidc.go](backend/internal/auth/oidc.go).

### 2.2 Tokens

| Token | Lifetime | Storage | Algorithm | Purpose |
|-------|----------|---------|-----------|---------|
| Access | 5 min | `portal_access` cookie OR `Authorization: Bearer` | HS256 with rotating `kid` keys | Per-request authn |
| Refresh | 30 days | `portal_refresh` cookie (`Path=/auth`) OR JSON body | 256-bit random, SHA-256 hashed at rest | Mint new access token |
| Step-up (TOTP) | 5 min | session-bound; not a separate cookie | n/a — flag on the session record | Authorize destructive ops |

Cookies always: `HttpOnly; Secure; SameSite=Strict`. Implementation in [jwt.go](backend/internal/auth/jwt.go) and [refresh.go](backend/internal/auth/refresh.go).

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

**Why `users` is global**: a person is a person across orgs. Their email and OIDC subject identify them once. Their access in a given org is mediated by `organization_memberships`. Denormalising `organization_id` onto users would force unique users per org — wrong shape.

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
7. Apply wildcard / scope rules from [permission.go](backend/internal/rbac/permission.go).

Cached in Redis under key `rbac:perms:<userID>:<orgID>:v<token_version>`. TTL 5 min. **Bumping `token_version` is the only canonical invalidation channel.**

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

Every security-sensitive event written to `audit_log` (append-only). See [audit/logger.go](backend/internal/audit/logger.go). Action codes are dotted, e.g. `auth.login`, `rbac.policy.updated`, `tenant.switched`, `auth.totp.verified`. **Failures are loud but non-blocking** for the user request.

Add for multi-tenancy: every audit row carries `organization_id` (NULL for system events). Migration delta:

```sql
ALTER TABLE audit_log
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX audit_log_org_idx ON audit_log(organization_id, occurred_at DESC);
```

### 5.2 Rate limiting  *([BUILT] in-memory; Redis-backed [PLANNED])*

Token bucket per IP for `/auth/*`. Stricter buckets per `(IP, action)` for sensitive endpoints (TOTP verify: 5/min/IP+user, lockout 15 min after 5 failures). Implementation in [ratelimit.go](backend/internal/middleware/ratelimit.go); for production, swap the in-memory store for `redis_rate.Limiter`.

### 5.3 Secrets handling

- JWT signing keys: `JWT_SIGNING_KEYS` env, rotating list. Active key signs new tokens; older keys remain valid for verification during the rotation window.
- TOTP encryption key: separate `TOTP_KMS_KEY` env. Different blast radius from JWT.
- OIDC client secret: `OIDC_CLIENT_SECRET` — never logged.
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
8. RequireTenant     ← reads org_id from JWT; sets app.current_tenant on the DB conn; sets tenant.Context in ctx
9. RequireStepUp     ← (optional, per-route) — verifies fresh TOTP for destructive ops
10. RequirePermission ← rbac.Engine.Authorize, returns 403 on deny
11. Handler
12. AuditMiddleware  ← (deferred) writes audit event for mutating ops
```

Public routes (e.g. `GET /movies` for guests) skip 7–10. A handful of "tenant-scoped but public-readable" routes use `OptionalAuth` + `RequireTenant`.

---

## 7. API surface (auth + tenant)

```text
# Authentication
GET    /auth/login                     start OIDC flow
GET    /auth/callback                  finish OIDC; mint tokens
POST   /auth/refresh                   rotate refresh; mint access
POST   /auth/logout                    revoke current refresh; bump token_version
POST   /auth/logout-all                revoke all refresh; bump token_version  [step-up]

# 2FA (TOTP)
POST   /auth/totp/enroll               start enrolment; returns secret + QR URI
POST   /auth/totp/verify               verify code; activate enrolment OR perform step-up
POST   /auth/totp/recovery-codes/regen regenerate recovery codes  [step-up]
DELETE /auth/totp                      disenrol  [step-up + recovery-code]

# Tenant
GET    /me/organizations               list orgs the user belongs to
POST   /auth/switch-tenant             switch active org; mints new tokens  [step-up if elevated]

# Identity
GET    /auth/me                        current user + roles + org context

# Sessions
GET    /me/sessions                    list active refresh tokens (devices)
DELETE /me/sessions/{id}               revoke a specific session  [step-up]

# Notifications
GET    /me/notifications               list (paginated, unread filter)
POST   /me/notifications/read          mark IDs read
GET    /me/notifications/stream        SSE channel for live updates
POST   /me/web-push/subscribe          register browser push subscription
DELETE /me/web-push/{id}               unsubscribe
```

OpenAPI source-of-truth at [shared/openapi.yaml](shared/openapi.yaml). Each endpoint annotates its required permission via `x-required-permission` and step-up requirement via `x-step-up: true`.

---

## 8. Threat model

What we explicitly defend against, and how.

| Threat | Defence |
|--------|---------|
| Stolen access token | Short TTL (5 min) + DB snapshot check on every request (`token_version`) — instant revocation. |
| Stolen refresh token | Rotation per use + reuse detection burns the chain. Hashed at rest. |
| Session hijack via XSS | `HttpOnly` cookies; CSP enforced server-side. Never expose tokens to JS. |
| CSRF | `SameSite=Strict` cookies. Login uses `Lax` only for the IdP redirect; OIDC callback verifies state. |
| Cross-tenant data leak via app bug | RLS enforced in Postgres. `app.current_tenant` set transactionally per request. `BYPASSRLS` role isolated to a separate Go binary. |
| Privilege escalation by a tenant admin | `superadmin` is a system role, never grantable from tenant context. CLI bootstrap only. |
| TOTP brute force | 6-digit code + 5 attempts/15-min lockout per user; constant-time compare. Recovery codes are single-use, Argon2id-hashed. |
| Refresh-token replay across devices | Each refresh token records issuing IP + UA. Reuse from a different fingerprint emits a higher-severity audit event (still revokes chain). |
| OIDC ID-token replay | Nonce validated against bound cookie. State validates session origin (CSRF). |
| TOTP secret extraction at rest | Encrypted with separate `TOTP_KMS_KEY`; only decrypted in-memory at verify time. |
| Audit log tampering | Append-only at app layer. Long-term retention to R2 archive bucket (immutable bucket policy). |
| Permission cache poisoning | Redis cache key includes `token_version` and `org_id`; mutations bump version → forces re-fetch from DB. |
| Insider with DB write access | `audit_log` replication to a write-once R2 bucket (separate credentials). Out-of-band log forwarding to SIEM. |

What we do **NOT** defend against (out of scope for v1):

- Compromise of the Postgres host itself.
- Compromise of the Authentik IdP (we trust its issuer).
- A determined administrator within a tenant exfiltrating their own tenant's data (legitimate usage).
- DDoS — handled at Cloudflare, not here.

---

## 9. Migration roadmap

Numbered to fit the existing migration sequence in `backend/db/migrations/`.

| # | File | Purpose |
|---|------|---------|
| 0001 | `init.up.sql` | [BUILT] users + assets foundational tables |
| 0002 | `rbac.up.sql` | [BUILT] roles + permissions + refresh_tokens + audit_log |
| 0003 | `organizations.up.sql` | [PLANNED] organizations + organization_memberships; RLS scaffolding |
| 0004 | `user_groups.up.sql` | [PLANNED] user_groups + user_group_members (org-scoped) |
| 0005 | `policies.up.sql` | [PLANNED] policies + policy_permissions + group_policy_attachments + user_policy_attachments |
| 0006 | `file_gated_permissions.up.sql` | [PLANNED] user_permission_files + review workflow |
| 0007 | `totp.up.sql` | [PLANNED] users.totp_*, totp_recovery_codes |
| 0008 | `notifications.up.sql` | [PLANNED] notifications + web_push_subscriptions |
| 0009 | `rls_enable.up.sql` | [PLANNED] enable RLS + policies on every tenant-scoped table |
| 0010 | `audit_log_org.up.sql` | [PLANNED] add organization_id to audit_log |

RLS is intentionally enabled in **a separate, late migration** so that earlier development can proceed without RLS hassle. Production deployment must include 0009 before going live; CI gate verifies it.

---

## 10. Implementation pointers

### 10.1 Tenant middleware skeleton  *([PLANNED])*

```go
// internal/middleware/tenant.go
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
// internal/middleware/stepup.go
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

Place under `backend/internal/repository/rls_test.go`. Do not let CI green without these.

---

## 11. Glossary

- **Tenant** — a unit of data isolation. In Portal, ≡ Organization.
- **Step-up auth** — a fresh authenticator confirmation required for destructive operations, on top of an already-valid session.
- **RLS** — PostgreSQL Row-Level Security. Filter clauses applied automatically to every query against a table.
- **TOTP** — Time-based One-Time Password (RFC 6238). What Google/Microsoft Authenticator emit.
- **Effective permission set** — the deduplicated, file-gated, scope-aware union of all permissions granted to a user *for a particular organization*.
- **Token version (`tv`)** — a monotonic counter on `users` that, when bumped, invalidates every outstanding access token for that user without revoking individual tokens.
