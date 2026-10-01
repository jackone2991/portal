# Authoration — Authentication, Authorization, and Multi-Tenancy

**Status:** current · **Last verified:** 2026-09-30

> Canonical security specification for Portal. Covers identity (authn),
> permission decisions (authz), and tenant isolation (data segregation).
>
> **Companion docs:**
> - [deferred/access-policies.md](deferred/access-policies.md) (was `archivetech.md`) — full functional roadmap (UI, modules, phasing)
> - [CLAUDE.md](../../CLAUDE.md) — architecture decisions + working agreement
> - [ADR-02](../adr/02-rbac-model-reconciliation.md) — role-hierarchy RBAC is canonical for v1; policy bundles layer on later
> - [ADR-06](../adr/06-local-auth-model.md) — local password auth; Authentik/OIDC removed
>
> For built (v1) surfaces, code + ADRs are canonical (ADR-02 explicitly
> disregards spec-wins clauses for v1); for the post-v1 layers specced here,
> this doc is the design of record. Update the doc in the same change-set as
> any behavior change.

> **Status (2026-07-06):** The identity layer (§2 — local password auth, tokens, two revocation
> channels, audit, login brute-force lockout) is **BUILT** and shipping in the closed v1 demo loop
> (see [ADR-06](../adr/06-local-auth-model.md); tracked then in `MILESTONE_CHECKS.md`, deleted in `f11cf3f`). The
> L2 tenant layer is **BUILT** too: one personal org per user, `RequireTenant`, and `FORCE` RLS enforced
> under `portal_app` ([ADR-07](../adr/07-tenancy-rls-model.md), §3). Still **POST-V1 DESIGN**, not current
> behaviour: §2.4 TOTP, §3.5–3.6 tenant switching and cross-tenant administration, §4 policy-bundle
> authorization, §6 step 9, and the un-numbered rows of §9. For v1, role-hierarchy RBAC is canonical per
> [ADR-02](../adr/02-rbac-model-reconciliation.md).

---

## 0. Decision log

The settled answers to the open questions raised in `access-policies.md §9` (then `archivetech.md`):

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

Every request traverses three independently-enforced layers. Each layer answers exactly one question; they do not overlap. *(All three layers are wired. L2 resolves only the caller's personal org today, and L3 is role-hierarchy RBAC that is global rather than per-tenant — so the "effective perms differ per tenant" point below is the target, not current behaviour. See §3 and the status banner.)*

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

> **Superseded by [ADR-06](../adr/06-local-auth-model.md) (2026-07-05).** Portal now owns credentials and authenticates locally; Authentik is removed from the login path. The **token, refresh, RBAC, revocation, and audit** machinery in §2.2 onward is unchanged and reused — only this login subsection changes. Any remaining "OIDC / callback / nonce / Authentik" mentions elsewhere in this doc are retired.

### 2.1 Local password login flow  *([BUILT])*

Portal is the identity provider. Credentials live in `users.password_hash` (Argon2id). Flow:

1. `POST /auth/login {email, password, remember}` — server looks up the user by email, verifies the password against `users.password_hash` (Argon2id, constant-time), then checks `disabled_at` and `approval_status`. `remember=true` → persistent refresh cookie (`Max-Age` = refresh TTL); `false` → session cookie.
2. On success, server issues access + refresh tokens, sets cookies, returns `200`. An unknown email or a wrong password returns a generic `401` (no user-enumeration) and increments the brute-force counter; a throttled caller gets `429`. A correct password on a disabled, pending or rejected account returns `403` with its own Problem type (`account/account-disabled` | `account-pending` | `account-rejected`) — the caller has proved who they are, so naming the state leaks nothing.
3. `POST /auth/register {email, password, display_name}` — creates the account (Argon2id hash) as **`pending`**, assigns the default `user` role, notifies every approver, and returns `201` **without** issuing a session. Only an `approved` account may hold a session (migration `0031`). The first account on an empty database is auto-approved and made `superadmin`; an existing install names one with `BOOTSTRAP_SUPERADMIN_EMAIL`.
   Statuses, Problem types and the open gaps (unthrottled register, the first-run race, timing of the unknown-email path) are [SPEC-01](../product/specs/SPEC-01-account-identity-admin.md) P0.1–P0.2 and §11.
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
- **`refresh_tokens.revoked_at`** — refresh-token-side revocation. Rotation chain (`parent_id`/`replaced_by_id`) is linear; presenting an already-rotated token revokes the **entire** chain (forward + backward via recursive CTE) and writes the audit action `account.refresh.reuse_detected` (an audit row, not a bus event). Theft detection — with one hole: rotation is check-then-mark, not an atomic claim, so two concurrent presentations of one token both succeed ([SPEC-01](../product/specs/SPEC-01-account-identity-admin.md) §11 row 2).

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

The middleware emits a generic `401` (`about:blank`) for every authn failure — invalid, expired, revoked token, disabled user — so the response body is no token-state oracle. One failure is named on purpose: an **unapproved** account gets `403 account/account-not-approved`, because the caller has already proved who they are and a 401 would send the frontend to a login that cannot help.

**As built, `RequireAuth` writes no audit row** (`account/middleware/auth.go`; its doc comment claims "specifics live only in the audit log", which is not true). The audit column below is the target design; of these actions only `account.refresh.reuse_detected` is written today, by the refresh handler, and `account.session.disabled_attempt` is defined in `platform/audit/logger.go` but never written ([SPEC-01](../product/specs/SPEC-01-account-identity-admin.md) P0.5, §11 row 13).

| Internal error | HTTP | Audit action (target) |
|----------------|------|--------------|
| `ErrTokenInvalid`     | 401 | `auth.token.invalid` |
| `ErrTokenExpired`     | 401 | (skipped — too noisy) |
| `ErrTokenRevoked`     | 401 | `auth.token.revoked` |
| `ErrUserDisabled`     | 401 | `auth.disabled_user_attempt` |
| `ErrUserNotApproved`  | 403 `account/account-not-approved` | — |
| `ErrTokenReused` (refresh) | 401 | `account.refresh.reuse_detected` (HIGH SEVERITY; written) |
| Step-up missing/expired | 403 | `auth.stepup.required` |
| TOTP wrong            | 401 | `auth.totp.invalid` |

---

## 3. Tenant layer (data segregation)

The decision is [ADR-07](../adr/07-tenancy-rls-model.md) (its "as built" section records what landed and what is deferred); this section describes the mechanism as it runs. Subsections marked *[TARGET]* are the post-v1 design and not current behaviour.

### 3.1 Tenant model  *([BUILT] — one personal org per user)*

A **Tenant** is a row in `organizations`. `kind ∈ {'org', 'household', 'personal'}` exists from day one [D-24], but only `personal` is in use: every user owns exactly one personal org (unique partial index `organizations_personal_owner_idx`), backfilled for pre-existing users by `0018_tenant_core` and otherwise created lazily on the user's first tenant-scoped request (`tenantapi.GetOrCreatePersonalOrg`, which runs the `CreatePersonalOrg` query; registration itself creates no org).

```text
                 ┌──────────────────┐
                 │   Organization   │  ◄── Tenant boundary. RLS enforces.
                 │ kind = personal  │      (org / household: schema-ready, unused)
                 └────────┬─────────┘
                          │ organization_memberships (org_id, user_id, role)
                          ▼
                 ┌──────────────────┐
                 │      User        │  global identity (users)
                 └──────────────────┘
```

- **Active tenant = the caller's personal org, always.** There is no `/t/{tenant}` URL prefix, no tenant claim in the JWT and no tenant switching (ADR-07 as built; `GET /api/v1/me/organizations` lists memberships, nothing else consumes them).
- `organization_memberships.role` is a free-text label (`owner` by default). It is **not** RBAC: roles and permissions stay global (`user_roles` has no `org_id`), so an RBAC grant applies in every tenant.
- *[TARGET]* Sub-organizations (`parent_org_id`), user groups (see [deferred/access-policies.md](deferred/access-policies.md) §3.1), households, and choosing the active org at login are not built.

### 3.2 Schema for tenancy  *([BUILT] — `0018_tenant_core`)*

```sql
CREATE TABLE organizations (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    kind       TEXT NOT NULL CHECK (kind IN ('org', 'household', 'personal')),
    slug       TEXT NOT NULL UNIQUE,              -- 'personal-<user id>' for personal orgs
    name       TEXT NOT NULL,
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX organizations_personal_owner_idx
    ON organizations (owner_id) WHERE kind = 'personal';

CREATE TABLE organization_memberships (
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'owner',
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);
```

Both tables are the tenancy **control plane** and are global (no RLS), like `users`.

### 3.3 `tenant_id` propagation

Every tenant-scoped table carries

```sql
tenant_id UUID NOT NULL DEFAULT current_setting('app.current_tenant')::uuid REFERENCES organizations(id)
```

plus `<table>_tenant_idx`. `0020_platform_rls_enable` added it to the 17 tables that existed then; every later migration that creates a domain table ships the column and its policy itself (the binding convention is the Tenancy bullet of the [specs README](../product/specs/README.md)). The one-argument `current_setting` in the DEFAULT is deliberate: an INSERT with no tenant context **fails** rather than landing somewhere arbitrary.

| Class | Tables (verify: `grep -ho 'ALTER TABLE [a-z_]* FORCE ROW LEVEL SECURITY' backend/db/migrations/*.up.sql \| sort -u`) | Rule |
|-------|------|------|
| Tenant-scoped, `FORCE` RLS | media (`assets`, `media_asset_variants`, `media_playback_progress`), `notifications`, journal (`journal_entries`, `stream_items`), bank (`bank_*`), comic (`comics`, `comic_*`), `movies`, music (`music_*`), story (`stories`, `story_chapters`), people (`people_*`) | `tenant_isolation`: `USING`/`WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid)` |
| Tenant-scoped + shared seed | `bank_categories` | `tenant_id` nullable; reads own tenant **or** `tenant_id IS NULL` (global seed taxonomy), writes own tenant only |
| Tenant + per-user ACL | `assets`, `media_asset_variants` (`0032_media_asset_acl`) | inside the tenant: readable if `public`, or yours, or you administer the tenant |
| Per-user RLS (cross-tenant by nature) | `social_connections` (`0037`) | a connection spans two accounts' personal orgs, so the policy keys on `app.current_user` ∈ {requester, addressee}, not on a tenant |
| Global, no RLS | `users`, `roles`, `permissions`, `role_permissions`, `user_roles`, `refresh_tokens`, `password_reset_tokens`, `audit_log`, `organizations`, `organization_memberships`, `notification_preferences`, `web_push_subscriptions`, `layout_menu_items`, `layout_widgets`, `ops_backup_runs` | identity, RBAC, control plane, per-user settings keyed by `user_id`, instance-wide shell layout, system bookkeeping |

**Why `users` is global**: a person is a person across orgs. Their email identifies them once; their presence in an org is a membership row. Denormalising a tenant onto `users` would force one account per org — the wrong shape.

### 3.4 PostgreSQL RLS — the enforcement mechanism  *([BUILT])*

```sql
ALTER TABLE movies ENABLE ROW LEVEL SECURITY;
ALTER TABLE movies FORCE ROW LEVEL SECURITY;      -- applies to the table owner too
CREATE POLICY tenant_isolation ON movies
    USING      (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
```

**Three session GUCs** carry the actor, all set transaction-locally by `platform/db` `BeginScope` (`set_config(..., true)`, discarded at COMMIT/ROLLBACK — safe under transaction pooling, though there is no PgBouncer today):

| GUC | Meaning |
|-----|---------|
| `app.current_tenant` | the active org (`tenant_isolation`, the column DEFAULT) |
| `app.current_user` | the acting user; empty for a backend job (`0032` media ACL, `0037` social) |
| `app.tenant_admin` | `on` for the tenant's owner and for trusted backend scopes (the worker processes every member's media) |

The `0032`/`0037` policies read the GUCs with the two-argument `current_setting(..., true)` wrapped in `NULLIF`, so an unset value matches nothing (public rows only) instead of raising.

**Request path.** `RequireAuth → RequireTenant → handler` ([tenant/middleware/require_tenant.go](../../backend/internal/modules/tenant/middleware/require_tenant.go)): `RequireTenant` resolves the caller's personal org, opens `BeginScope{OrgID, UserID, Admin: owner == caller}` and runs the whole handler in that transaction. A 5xx or panic rolls back. For `POST`/`PUT`/`PATCH`/`DELETE` the response is buffered until COMMIT succeeds, so a failed commit becomes a 500 instead of a lost 201. `OptionalTenant` is the same for routes that also serve anonymous callers (media variants/HLS): an anonymous request runs with no transaction and so sees public rows only.

**Worker path.** A worker handler opens the payload owner's scope (`runInUserTenant` in `cmd/worker`) before any query; periodic sweeps iterate tenants with `forEachTenant` (one committed scope per tenant). Known gap: the comic, movie, music and story `media:asset_deleted` consumers query the bare pool and are refused under `portal_app` — [backlog](../product/backlog.md) #42.

**Roles.** Enforcement depends on the connecting role ([operations/rls-cutover.md](../operations/rls-cutover.md)):

- `portal_app` (`0019`, `NOSUPERUSER NOBYPASSRLS`) — the API and worker `DATABASE_URL`; RLS applies.
- `portal` (superuser/owner) — `MIGRATE_DATABASE_URL` and `BACKUP_DATABASE_URL` only; `pg_dump` must see every tenant.
- `portal_sys` (`0019`, `BYPASSRLS`) — reserved for the planned `cmd/sysjobs` binary; nothing connects as it today, and depguard already restricts `internal/sysrepository` to `cmd/sysjobs`. **The API server never connects as a bypass role.**

**Proof.** `go test ./internal/platform/db -run TestRLS` (with `RLS_TEST_ADMIN_URL` / `RLS_TEST_APP_URL`) proves cross-tenant read/write/relocate/delete isolation on a representative table (`comics`), the no-scope write failure and the shared-seed exception, and `TestRLSEveryProtectedTableHasAPolicyAndForce` asserts that every RLS-enabled table is `FORCE`d and has a policy (`rls_media_test.go` and `rls_social_test.go` cover the per-user policies); the `backend` CI job runs it against a throwaway `postgres:18` and fails, rather than skips, when unpointed under `CI=true`.

### 3.5 Tenant switching  *([TARGET] — not built)*

With one personal org per user there is nothing to switch to. The design, kept for when orgs/households land:

```text
POST /auth/switch-tenant
  body: { "organization_id": "..." }
```

The server validates membership, requires a fresh TOTP if the user has one enrolled (§2.4, not built), mints a new access token carrying the new org, and bumps `token_version` so the previous token cannot reach the old tenant.

### 3.6 Cross-tenant administrators  *([TARGET] — not built)*

Today `superadmin` is a global RBAC role with no cross-tenant data access: a superadmin's requests are scoped to their own personal org like anyone else's, and the admin console (`/api/v1/admin/*`, [SPEC-01](../product/specs/SPEC-01-account-identity-admin.md)) reads only global tables. Bootstrapping a superadmin is the first registrant or `BOOTSTRAP_SUPERADMIN_EMAIL` (SPEC-01 P0.1/P0.12) — there is no `cmd/admin` CLI. The target design:

- A `superadmin` may enter any organization via `POST /auth/switch-tenant` without a membership, always behind TOTP step-up.
- Such a session is flagged `system_impersonation = true`; every action is audited with both the operator and the impersonated tenant.
- Tenant admins can never grant themselves `superadmin`.

---

## 4. Authorization layer

### 4.1 Access-control model recap

(Detailed in `access-policies.md §2`. Repeated here as the unit-of-decision for this document.)

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
3. On success: cascade deletes children (per `access-policies.md`), bump `token_version` for all members of all affected groups, audit `rbac.group.deleted` with a metadata field listing every cascaded child group.
4. If the actor lacks TOTP enrolment, the endpoint returns `403 totp_required` and the frontend redirects to `/account/security` to enrol.

This same pattern (`requireStepUp`) wraps every other destructive op:
`DELETE /admin/policies/{id}`, `DELETE /admin/users/{id}`, `POST /auth/logout-all`, `POST /auth/switch-tenant` (when source org has elevated perms), `cmd/admin grant-superadmin`.

### 4.6 Permission catalogue — operator console codes

Permission codes that specs add for the operator surfaces. The seeded rows in the migrations are canonical; this table records the security-relevant ones.

| Code | Granted to | Guards | Status |
|------|-----------|--------|--------|
| `ops:read` | admin | ops status / backup-run reads (SPEC-03) | seeded by SPEC-03 P0.1 |
| `queues:read` | admin | read-only asynqmon queue console (SPEC-03 P1.6) | seeded by SPEC-03 P0.1 |
| `queues:write` | admin only | asynqmon mutations — retry, delete, archive, pause (SPEC-03 P1.6) | [PLANNED] — seeded by SPEC-03 P1.6's own migration |

The queue console is mounted on the **API origin** (`/admin/queues/*`), not the frontend origin. Every non-GET/HEAD request additionally requires a same-origin check: `Sec-Fetch-Site: same-origin` or an `Origin` equal to the API origin, otherwise `403` (SameSite=Lax does not separate same-site siblings such as `minio.`, `mail.`, `traefik.`). Callers holding only `queues:read` get the read-only handler, which rejects mutations with `403`.

---

## 5. Cross-cutting concerns

### 5.1 Audit  *([BUILT] core; UI [PLANNED])*

Every security-sensitive event written to `audit_log` (append-only). See [audit/logger.go](../../backend/internal/platform/audit/logger.go). Action codes are dotted `<module>.<resource>.<action>`, e.g. `account.session.login`, `account.role.granted` — the legacy `auth.*`/`rbac.*`/`user.*` codes were renamed per [D-25]; the constants in that file are canonical. **Failures are loud but non-blocking** for the user request.

Add for multi-tenancy: every audit row carries `organization_id` (NULL for system events). Migration delta:

```sql
ALTER TABLE audit_log
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX audit_log_org_idx ON audit_log(organization_id, occurred_at DESC);
```

### 5.2 Rate limiting  *(login [BUILT]; everything else not applied)*

The `/auth/login` brute-force counter + lockout is **[BUILT], Redis-backed** ([handler/auth.go](../../backend/internal/modules/account/handler/auth.go): 5 failures per 15 min, per IP and per account, then `429`). Its per-IP key is the leftmost `X-Forwarded-For` from any peer, so a client that reaches the API without Traefik can rotate it ([SPEC-01](../product/specs/SPEC-01-account-identity-admin.md) §11 row 4).

The generic per-IP token bucket **exists but is wired nowhere**: [ratelimit.go](../../backend/internal/platform/middleware/ratelimit.go) `IPRateLimiter` is imported by no binary (`grep -rn "ratelimit\." backend --include=*.go` finds no caller), and the `rate-limit` middleware in `traefik/dynamic.yml` is attached to no router. `/auth/register` and `/auth/refresh` are therefore unthrottled (SPEC-01 §11 row 3; [backlog.md](../product/backlog.md) P0 #40). Stricter buckets per `(IP, action)` for sensitive endpoints (TOTP verify: 5/min/IP+user, lockout 15 min after 5 failures) and a Redis-backed generic bucket (`redis_rate.Limiter`) are still [PLANNED].

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
1. RequestID         ← unique per request, in logs + audit
2. RealIP            ← chi's RealIP: trusts X-Forwarded-For / X-Real-IP / True-Client-IP from any peer
3. Recoverer         ← catch panics, return 500
4. Timeout(requestWindow) ← bounded request lifetime; requestWindow is 45 min (cmd/api/main.go) so multi-GB uploads finish
5. CORS              ← origin allowlist from config
6. RateLimit         ← [NOT APPLIED] per-IP token bucket; the middleware exists, nothing mounts it (§5.2)
7. RequireAuth       ← JWT + DB snapshot; sets auth.Identity in ctx
8. RequireTenant     ← [BUILT] opens the per-request tenant-scoped transaction (ADR-07); mounted on module routes, not on the global account routes
9. RequireStepUp     ← [PLANNED] (optional, per-route) — verifies fresh TOTP for destructive ops
10. RequirePermission ← rbac.Engine.Authorize, returns 403 on deny
11. Handler
12. AuditMiddleware  ← (deferred) writes audit event for mutating ops
```

The v1 pipeline in production is steps 1–5, 7, 8 and 10; rate limiting (6) is built but unmounted, and step-up (9) lands with the TOTP phase. The order above is `cmd/api/main.go`'s `r.Use` order.

Public routes (`/auth/login`, `/auth/register`, `/auth/refresh`, health) skip 7–10; every catalogue route (`/movies`, `/tracks`, `/comics`, …) requires authentication. The only "tenant-scoped but public-readable" routes are media's variant/HLS proxies, which use `OptionalAuth` + `OptionalTenant` (§3.4): an anonymous caller runs unscoped and sees `public` assets only.

---

## 7. API surface (auth + tenant)

```text
# Authentication  [BUILT]
POST   /auth/login                     verify email+password; mint tokens
POST   /auth/register                  create account (Argon2id) as pending; returns 201, no session — user signs in via /auth/login once approved
POST   /auth/refresh                   rotate refresh; mint access
POST   /auth/logout                    revoke current refresh; bump token_version
POST   /auth/logout-all                revoke all refresh; bump token_version  [step-up]

# 2FA (TOTP)  [PLANNED]
POST   /auth/totp/enroll               start enrolment; returns secret + QR URI
POST   /auth/totp/verify               verify code; activate enrolment OR perform step-up
POST   /auth/totp/recovery-codes/regen regenerate recovery codes  [step-up]
DELETE /auth/totp                      disenrol  [step-up + recovery-code]

# Tenant
GET    /me/organizations               list orgs the user belongs to  [BUILT — today only the personal org]
POST   /auth/switch-tenant             switch active org; mints new tokens  [step-up if elevated]  [TARGET — §3.5]

# Identity  [BUILT]
GET    /auth/me                        current user + roles + org context (+ timezone, timezone_manual — PLANNED)
PATCH  /auth/me                        {timezone, timezone_manual?}; 422 account/invalid-timezone  [PLANNED — specs README Timezone]

# Sessions  [PLANNED]
GET    /me/sessions                    list active refresh tokens (devices)
DELETE /me/sessions/{id}               revoke a specific session  [step-up]

# Notifications  [PLANNED]
GET    /me/notifications               list (paginated, unread filter)
POST   /me/notifications/read          mark IDs read
GET    /me/notifications/stream        SSE channel for live updates
POST   /api/v1/me/push-subscriptions        register browser push subscription (SPEC-05 P1.1)
DELETE /api/v1/me/push-subscriptions/{id}   unsubscribe (SPEC-05 P1.1)
```

The `[step-up]` tags are the target; no step-up exists yet (§2.4). Not listed above: the password-reset pair (`/auth/forgot-password`, `/auth/reset-password` — SPEC-05 P0.3) and the **admin console** (`/admin/users*`, `/admin/roles*`, `/admin/permission-matrix` — approval queue, user CRUD, role × permission matrix). The full route table with permissions and Problem types is [SPEC-01](../product/specs/SPEC-01-account-identity-admin.md) §7; `account/module.go` `mountAdmin` is the authorization policy itself.

OpenAPI source-of-truth at [shared/openapi.yaml](../../shared/openapi.yaml). Each endpoint annotates its required permission via `x-required-permission` and step-up requirement via `x-step-up: true`. (2026-07: the earlier `/auth/register` / `/auth/callback` drift is **reconciled** — the spec now documents `/auth/register` and no longer lists the retired `/auth/callback`. The `x-required-permission` / `x-step-up` annotations remain the target convention; no account operation carries `x-required-permission` yet, and every `/admin/*` operation lacks `security` and so reads as public — SPEC-01 §11 row 20.)

---

## 8. Threat model

What we explicitly defend against, and how.

| Threat | Defence |
|--------|---------|
| Stolen access token | Short TTL (5 min) + DB snapshot check on every request (`token_version`) — instant revocation. |
| Stolen refresh token | Rotation per use + reuse detection burns the chain. Hashed at rest. (A concurrent replay is not yet detected — SPEC-01 §11 row 2.) |
| Session hijack via XSS | `HttpOnly` cookies; CSP enforced server-side. Never expose tokens to JS. |
| CSRF | `SameSite=Strict` cookies on all session cookies. Login is a same-origin `POST` (no cross-site redirect), so no `Lax` relaxation is needed. |
| Password brute force | Rate-limit + temporary lockout on `/auth/login` per IP and per account; generic `401` (no user enumeration); Argon2id (memory-hard) makes offline cracking expensive. |
| Cross-tenant data leak via app bug | `FORCE` RLS in Postgres under `portal_app`; `app.current_tenant` set transactionally per request (§3.4); `TestRLS*` in CI. The `BYPASSRLS` role `portal_sys` is reserved for the planned `cmd/sysjobs` and used by nothing today. Known gap: four `media:asset_deleted` consumers run unscoped and are refused, not leaking (backlog #42). |
| Privilege escalation by an admin | No-escalation guard in the admin console: you cannot grant, revoke or assign what you do not hold; only a `*` holder may touch the `superadmin` role; bootstrap is the first registrant or `BOOTSTRAP_SUPERADMIN_EMAIL` ([SPEC-01](../product/specs/SPEC-01-account-identity-admin.md) P0.11–P0.12). **Open holes:** re-parenting a role under `superadmin` is not guarded, and approve/reject/disable skip the target-authority check (SPEC-01 §11 rows 1, 6). |
| TOTP brute force | 6-digit code + 5 attempts/15-min lockout per user; constant-time compare. Recovery codes are single-use, Argon2id-hashed. |
| Refresh-token replay across devices | Each refresh token records issuing IP + UA. Reuse from a different fingerprint emits a higher-severity audit event (still revokes chain). |
| Password DB dump | `password_hash` is Argon2id (64 MB, t=3, p=2) with per-user salt — memory-hard, no plaintext or reversible form stored. |
| TOTP secret extraction at rest | Encrypted with separate `TOTP_KMS_KEY`; only decrypted in-memory at verify time. |
| Audit log tampering | Append-only at app layer. Long-term retention to R2 archive bucket (immutable bucket policy). |
| Permission cache poisoning | Redis cache key is `rbac:perms:<userID>:v<token_version>`; role/permission mutations bump `token_version` → forces re-fetch from DB. (No `org_id` in the key: RBAC is global, §3.1.) |
| Insider with DB write access | `audit_log` replication to a write-once R2 bucket (separate credentials). Out-of-band log forwarding to SIEM. |

What we do **NOT** defend against (out of scope for v1):

- Compromise of the Postgres host itself (holds `password_hash`; mitigated by Argon2id, not eliminated).
- A determined administrator within a tenant exfiltrating their own tenant's data (legitimate usage).
- DDoS — handled at Cloudflare, not here.

---

## 9. Migration roadmap

One numeric sequence in `backend/db/migrations/`, `000N_<owning-module>_<description>` naming; the next free number is whatever `ls backend/db/migrations | tail -2` says. The table lists only the migrations that shape **identity, authorization or tenant isolation**; domain migrations add their own `tenant_id` + policy as they create tables (§3.3).

| Migration | Layer | What it does for security |
|-----------|-------|---------------------------|
| `0002_account_users` | L1 | `users`: `token_version`, `disabled_at`, `timezone` |
| `0003_account_rbac` | L3 | `roles` (hierarchy), `permissions`, `role_permissions`, `user_roles`; the seven `is_system` roles and the seed catalogue |
| `0004_account_sessions` | L1 | `refresh_tokens` with the rotation chain used for reuse detection |
| `0005_platform_audit` | — | append-only `audit_log` (global; `actor_id ON DELETE SET NULL`) |
| `0006_account_local_auth` | L1 | `users.password_hash` (Argon2id, [ADR-06](../adr/06-local-auth-model.md)); drops `user_oidc_roles` |
| `0010_account_password_reset_tokens` | L1 | `password_reset_tokens` (the reset flow itself is SPEC-05 P0.3) |
| `0018_tenant_core` | L2 | `organizations` + `organization_memberships`; personal org backfill (§3.2) |
| `0019_platform_rls_roles` | L2 | `portal_app` (`NOBYPASSRLS`) and `portal_sys` (`BYPASSRLS`) roles and grants |
| `0020_platform_rls_enable` | L2 | `tenant_id` + `FORCE` RLS + `tenant_isolation` on the 17 tenant-scoped tables of the time |
| `0031_account_user_approval` | L1 | `users.approval_status` — only `approved` may hold a session ([SPEC-01](../product/specs/SPEC-01-account-identity-admin.md) P0.1) |
| `0032_media_asset_acl` | L2 | per-user ACL on `assets` / `media_asset_variants` (`app.current_user`, `app.tenant_admin`, `visibility`) |
| `0037_social_connections` | L2 | first per-user (cross-tenant) RLS policy set |

Every later domain migration (`0021_movie_core`, `0022_music_core`, `0023_story_core`, `0026`–`0030` comic, `0038`–`0041` music, `0043_bank_debts`, …) ships `tenant_id`, `ENABLE` + `FORCE ROW LEVEL SECURITY` and `tenant_isolation` in the same file, and `TestRLSEveryProtectedTableHasAPolicyAndForce` fails CI if one forgets the `FORCE` or the policy.

**Not yet numbered** — the post-v1 layers this document designs, each landing as the next free number when built:

| Layer | Content | Design |
|-------|---------|--------|
| L1 | TOTP: `users.totp_*`, `totp_recovery_codes` | §2.4 |
| L1 | per-user timezone: default `'Asia/Ho_Chi_Minh'`, `timezone_manual` | SPEC-01 P0.13 |
| L3 | user groups + policy bundles (`user_groups`, `policies`, attachments) on top of roles ([ADR-02](../adr/02-rbac-model-reconciliation.md)) | §4, [deferred/access-policies.md](deferred/access-policies.md) |
| L3 | file-gated permissions | §4.4 |
| L2 | membership-scoped RBAC, households/orgs, tenant switching | §3.5–3.6, ADR-07 deferred steps |

The original plan numbered these `0008`–`0015` and enabled RLS in a single late migration; that plan was overtaken — the numbers went to domain modules, and RLS landed in `0020` once the worker could open tenant scopes ([operations/rls-cutover.md](../operations/rls-cutover.md) is the cutover record).

---

## 10. Implementation pointers

### 10.1 Tenant middleware skeleton  *([BUILT] differently — see below)*

The shipped middleware is [tenant/middleware/require_tenant.go](../../backend/internal/modules/tenant/middleware/require_tenant.go) and differs from this sketch in three ways (§3.4): the org comes from `GetOrCreatePersonalOrg`, not a JWT claim, so there is no membership check or `tenant_missing`/`tenant_denied` path; it calls `BeginScope{OrgID, UserID, Admin}` (three GUCs), not `BeginTenantScope`, which is reserved for backend jobs; and mutating responses are buffered until COMMIT succeeds. Errors are RFC 7807 Problems via `platform/server`. The sketch is kept as the shape for when `/t/{tenant}` routing and memberships land.

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
3. Connects as the owner/superuser (`RLS_TEST_ADMIN_URL`) — sees both tenants. (`portal_sys`, the `BYPASSRLS` role, is unused until `cmd/sysjobs` exists.)

In practice the suite is centralised in `backend/internal/platform/db/rls_test.go` (plus `rls_media_test.go`, `rls_social_test.go`); `TestRLSEveryProtectedTableHasAPolicyAndForce` already catches a new table that forgets `FORCE` or its policy, so a PR adding a table with a non-standard policy (per-user, shared seed) adds its own test there. CI runs the suite on every push and fails rather than skips when unpointed.

---

## 11. Glossary

- **Tenant** — a unit of data isolation. In Portal, ≡ Organization.
- **Step-up auth** — a fresh authenticator confirmation required for destructive operations, on top of an already-valid session.
- **RLS** — PostgreSQL Row-Level Security. Filter clauses applied automatically to every query against a table.
- **TOTP** — Time-based One-Time Password (RFC 6238). What Google/Microsoft Authenticator emit.
- **Effective permission set** — the deduplicated, file-gated, scope-aware union of all permissions granted to a user *for a particular organization*.
- **Token version (`tv`)** — a monotonic counter on `users` that, when bumped, invalidates every outstanding access token for that user without revoking individual tokens.
