# SPEC-01 — Account: identity, approval gate, RBAC and the admin console

**Status:** current, rev 2 · **Drafted:** 2026-10-01 · **Last verified:** 2026-10-01
**Module:** `account` · **Depends on:** SPEC-05 (`notify:dispatch` carries the approval notification and the password-reset email; reset behaviour is SPEC-05 P0.3; the `notify:on_*` consumers of P1.3's events are SPEC-05 P1.5) · SPEC-04 P0.7 (`mediaapi.PurgeOwnerAssets`, called by the user delete, P0.10) · `platform/audit` ([D-25]) · `platform/events` (P1.3) · `platform/server` (Problem writer, `Limit`)
**Upstream:** none — an **as-built spec written retroactively** from the code on `main` @ `99b5a0b` (the docs commits on top of it change no code). The decisions it records were taken elsewhere and are cited, not re-decided: [ADR-02](../../adr/02-rbac-model-reconciliation.md) (role hierarchy is canonical for v1), [ADR-06](../../adr/06-local-auth-model.md) (local password auth; Portal owns credentials), [ADR-07](../../adr/07-tenancy-rls-model.md) (account tables are global, not tenant-scoped), migration `0031_account_user_approval` (registration requires approval), feature-inventory `D-17` (per-user timezone, owner decisions 2026-09-30 and 2026-10-01 (f)), `D-25` (audit taxonomy), `D-26.r1` (Portal-managed roles only), `D-34.r1` (SessionKeeper auth handoff)
**Refs:** [security.md](../../architecture/security.md) (design depth: token model, revocation channels, threat model — this spec owns the product requirements and the contract, not the design), [/CLAUDE.md](../../../CLAUDE.md) § Account module, [backend/MODULES.md](../../../backend/MODULES.md) §5.3 (audit taxonomy), the specs README [Timezone](README.md#conventions-binding-on-all-specs) convention and its **Per-user timezone** cross-cutting gap, which this spec now owns
**Downstream consumers:** every module's `RequirePermission` / `RequireOwnerOrPermission` (built by `cmd/api` from `account.Module.Engine()`); `tenant` (`RequireAuth` + the caller identity); `layout` (`accountapi.HasPermission`); `people` (`accountapi.ListDirectory`; SPEC-11 P0.3/P0.4 timezone); `social` (`accountapi.GetUserNames`); `notify` (recipient email, via the account repository in `cmd/worker`; the P1.3 `account:*` events and `SuperadminIDs`); the timezone readers SPEC-12 P0.6, SPEC-07 P0.4, SPEC-09 P0.1/P0.3/P1.5, SPEC-11 P0.3/P0.4, SPEC-13 §4a; the frontend's `lib/session.ts`, `lib/admin.ts`, `/login`, `/register` and `/admin/*`

---

## 1. Problem statement

Every other spec assumes a caller who is known, approved and authorised, and
none of them owns how that is established. The account module shipped in
slices — local password auth (ADR-06, `0006`), the approval gate (`0031`), the
admin console and its escalation guards — each documented only in code comments,
`/CLAUDE.md` and `shared/openapi.yaml`. The specs README already had to route one
decided change (the per-user timezone) to "no spec owns the account module".
Without an owning spec there is no single place that states the contract, the
Problem slugs, the guards an admin action must pass, or where the shipped code
diverges from the binding conventions.

## 2. Goals

1. One written contract for identity: register → approval → login → refresh →
   logout, with exact status codes and Problem slugs.
2. The admin console's authorisation policy — the route table plus the three
   invariants (no escalation, no self-edit, cache re-key on every change) —
   stated as testable requirements.
3. The account half of the per-user timezone decision (`users.timezone`,
   `timezone_manual`, `PATCH /auth/me`, `accountapi`) owned by a spec, so the
   readers in SPEC-12/07/09/11/13 have a dependency they can point at.
4. Every place the shipped module diverges from the specs README conventions
   listed in §11, with the change that closes it.

## 3. Non-goals

- **Re-deciding ADR-02 or ADR-06.** Role hierarchy stays canonical; policy
  bundles, user groups and file-gated permissions ([deferred/access-policies.md](../../architecture/deferred/access-policies.md))
  stay post-v1. No IdP in the login path.
- **Password reset behaviour.** `POST /auth/forgot-password` and
  `/auth/reset-password` are account routes whose behaviour, throttles, slugs
  (`account/invalid-reset-token`, `account/rate-limited`) and gaps are owned by
  [SPEC-05](SPEC-05-notification-module.md) P0.3 and its §11 rows 1–2 and 14.
  They appear in §7 only for completeness.
- **MFA / TOTP / step-up** ([D-27]/[D-28]) and **Login with Google** (ADR-06
  action items 10–11) — deferred; see [backlog.md § Deferred](../backlog.md).
- **Tenant-scoped roles, `switch-tenant`, `/admin/organizations`** (ADR-07
  steps 5–7) — deferred at one user with one personal org.
- **Rich profile** (bio, cover, education): [D-19] puts it in `social.profiles`,
  not on `users`.
- **An audit-log reader UI.** Writing audit rows is in scope (P0.11); reading
  them is not.

## 4. User stories

- As the person installing Portal, I register first and can sign in at once as
  superadmin; nobody else can, until I let them in. *(first-run bootstrap)*
- As a household member, I register, am told I am waiting for approval, and can
  sign in the moment the owner approves me.
- As the owner, I get a bell entry and an email when someone registers, and
  approve or reject them from `/admin/users` with an optional note the person
  sees on their next login attempt.
- As the owner, I revoke an approval or disable an account and that person's
  open browser stops working on its next request, not five minutes later.
- As an admin who is not a superadmin, I can manage ordinary users and roles but
  cannot hand myself — or anyone — a permission I do not hold, and cannot take
  over or lock out an account that outranks me.
- As a superadmin, I get a bell entry when another admin approves, disables,
  deletes or re-roles an account or changes a role, and an email when a stolen
  refresh token is replayed. *(P1.3 — decided, unbuilt)*
- As the owner, I delete an account and its uploaded files are gone from
  storage too, not just its rows. *(P0.10 — decided, unbuilt)*
- As any signed-in user, my day boundaries ("today", "this month", birthday
  countdowns) follow the timezone of where I am, unless I pinned one in
  settings. *(P0.13 — decided, unbuilt)*
- Edge: an operator upgrading an install that predates `0031` names the
  superadmin in `BOOTSTRAP_SUPERADMIN_EMAIL` and it is (re-)asserted on every
  start.

## 5. Requirements

Status codes below are the **target**; where `HEAD` differs, the requirement
says so in a *(code follow-up)* note and §11 carries the row. "about:blank"
means a bare-status Problem (specs README Errors convention).

### P0.1 — Registration (`POST /api/v1/auth/register`, public)

Body `{email, password, display_name?}` (64 KiB cap, `handler/auth.go`
`decodeJSON`). The email is trimmed and lower-cased; it is valid when it has a
non-leading `@`, a `.` after it and ≤ 320 characters (`validEmail`) — else 422
`account/invalid-email`. The password is ≥ 8 bytes (`minPasswordLen`) — else 422
`account/password-policy`. An unparseable body is 400 about:blank. The password
is hashed with Argon2id (P0.3). A taken email is 409 `account/email-taken`
(unique violation `23505` mapped in `repository/adapter.go` `CreateLocalUser`).
`display_name` defaults to the email's local part.

On success: the row is created `pending` (the `0031` column default), the `user`
role is seeded best-effort (a failure is recorded in the audit metadata as
`role_seed_error`, never fails the request), `account.user.registered` is
audited, and the response is **201 with no session**:
`{status: "registered", email, approval_status}`.

*(Code follow-up: `HEAD` answers the email and password checks with 400
`account/invalid-email` and 400 `account/weak-password` — a second slug for the
rule P0.10 calls `account/password-policy` — §11 row 17.)*

**First-run bootstrap.** When the insert leaves exactly one row in `users`
(`CountUsers` = 1), the account is also granted `superadmin` and approved by
itself (`MarkApproved`), and the response carries `approval_status: "approved"`.
The window is one account wide. The two concurrent first registrations of an
empty install must not both miss it *(code follow-up: `HEAD` counts after
insert, outside a transaction — §11 row 10)*.

**Approver fan-out** (`handler/auth.go` `notifyApprovers`). For every
registration except the founder's, the API resolves the accounts whose
*effective* permissions satisfy `users:approve`: SQL
(`ListPermissionHoldersByResource`) walks the role hierarchy for enabled,
approved accounts and prefilters to codes whose resource segment is `users` or
`*`; `rbac.Set.AllowsCode` then applies the real grammar (a superadmin's set is
the literal `*`). It enqueues one `notify:dispatch` per approver (not the
registrant) through `notifyapi`: type `account.registration_pending`, channels
`in_app` + `email`, `data {user_id, email, display_name, href}` (`href` =
`APPROVAL_QUEUE_URL`), `dedup_key = "registration:<user_id>"`. At most **50**
approvers (`maxApproverFanout`; truncation is logged). No approver is a logged
warning. Every failure is logged and swallowed — registration never fails on
notification. RBAC is not tenant-scoped, so the fan-out is global.

**Abuse control.** Registration is throttled per client IP *(code follow-up:
`HEAD` applies no limiter to `/auth/register` or `/auth/refresh` — the
`platform/middleware` `IPRateLimiter` exists but no binary wires it, and the
Traefik `rate-limit@file` middleware is attached to no router — so each
anonymous POST can create a pending row and up to 50 email dispatches; §11
row 3)*.

**Acceptance criteria.**
- Given an empty `users` table, when A registers, then 201 `approval_status:
  approved`, A holds `superadmin`, and A can log in.
- Given a non-empty table, when B registers, then 201 `approval_status:
  pending`, B holds only `user`, and every approver gets one bell row and one
  email; a redelivered dispatch adds no second bell row.
- Given a password of 7 bytes, then 422 `account/password-policy` and no row.
- Given an existing email in any case, then 409 `account/email-taken`.
- Given 50 rapid registrations from one IP, then later ones answer 429
  `platform/rate-limited` (P0.1 abuse control).

### P0.2 — Login (`POST /api/v1/auth/login`, public)

Body `{email, password, remember?}`. Missing email or password, or an
unparseable body: 400 about:blank. Order of checks (`handler/auth.go` `Login`):

1. **Throttle.** If the failure counter for the client IP **or** for the email
   has reached 5 (`loginMaxFailures`) within its 15-minute window
   (`loginFailWindow`, Redis keys `login:fail:ip:<ip>` / `login:fail:email:<email>`),
   answer 429 `account/too-many-attempts` with `Retry-After` *(code follow-up:
   no `Retry-After` on `HEAD` — §11 row 17)*. A nil Redis client disables the
   throttle.
2. **Credentials.** Unknown email, empty hash, malformed hash or wrong password
   all answer the same 401 `account/invalid-credentials` and increment both
   counters. The response time must not depend on whether the email exists
   *(code follow-up: an unknown email skips Argon2id and answers faster — §11
   row 5)*.
3. **Disabled** (`disabled_at` set): 403 `account/account-disabled`; audited as
   `account.session.disabled_attempt` *(code follow-up: the constant exists in
   `platform/audit` but nothing writes it — §11 row 13)*.
4. **Approval gate.** `pending` → 403 `account/account-pending`; `rejected` →
   403 `account/account-rejected` whose `detail` ends with the reviewer's note
   when there is one. Checked after the password, so an unapproved account
   cannot be used to probe emails. Both counters are cleared first (honest
   retries while waiting must not lock the account out) and
   `account.session.pending_attempt` is audited.
5. **Success:** counters cleared; session issued (P0.3); `account.session.login`
   audited; 200 `{access_token, expires_in, token_type: "Bearer", user: {id,
   email, display_name, roles}}`.

The client IP is the leftmost `X-Forwarded-For` entry, else the peer address
(`handler/util.go` `clientIP`). It must only honour that header from the
trusted proxy *(code follow-up — §11 row 4)*.

**Acceptance criteria.**
- Given 5 wrong passwords for one email, then the 6th attempt — even with the
  right password — is 429 until the window lapses.
- Given a pending account with the right password, then 403
  `account/account-pending` and no cookie is set; with a wrong password, 401.
- Given a rejected account with note "dup", then 403 `account/account-rejected`
  and `detail` contains "dup".
- Given valid credentials and `remember: false`, then `portal_refresh` and
  `portal_session` carry no `Max-Age` (session cookies).

### P0.3 — Credentials and session tokens

As decided in ADR-06 and designed in [security.md §2.2–2.3](../../architecture/security.md);
the contract is:

- **Password hash:** Argon2id PHC string, `m=65536,t=3,p=2`, 16-byte salt,
  32-byte key (`auth/password.go`); verification is constant-time and a
  malformed stored hash verifies false.
- **Access token:** HS256 JWT, `kid` header selects the key from the rotating
  set (`JWT_SIGNING_KEYS`; every secret ≥ 32 bytes; the first key signs).
  Claims `sub, iss, aud, exp, nbf, iat, jti, tv` (token_version), `roles`,
  `email`, `name`. TTL `ACCESS_TOKEN_TTL` (default 5 m; `NewIssuer` refuses
  > 1 h). Only HS256 verifies; an unknown `kid` is invalid.
- **Refresh token:** 32 random bytes, base64url; stored only as SHA-256
  (`refresh_tokens.token_hash`, unique). TTL `REFRESH_TOKEN_TTL` (default 24 h;
  `NewRefreshManager` refuses > 90 d).
- **Cookies** (all `HttpOnly`, `SameSite=Strict`, `Secure` = `COOKIE_SECURE`):
  `portal_access` (Path `/`, Max-Age = access TTL), `portal_refresh` (Path
  `/api/v1/auth`), `portal_session` (Path `/`, value `p` persistent / `s`
  session — the marker the Next.js middleware gates on, D-34.r1).
  `remember: true` gives the refresh and marker cookies Max-Age = refresh TTL;
  otherwise they are session cookies. Cookie `Domain`: host-only for
  `localhost` or an IP; the parent domain for an `api.` host; else
  `COOKIE_DOMAIN` (`cookieDomainFor`). Clearing uses the same Domain.

### P0.4 — Refresh (`POST /api/v1/auth/refresh`, public)

Reads `portal_refresh`, else a JSON body `{refresh_token}`. Missing → 401
about:blank. Rotation (`auth/refresh.go` `Rotate`): unknown or expired token →
401; an **already-revoked** token (rotated, logged out, or revoked) is reuse —
the whole chain is revoked forward and backward (`RevokeRefreshTokenChain`,
recursive CTE), `account.refresh.reuse_detected` is audited, cookies are
cleared, 401. Otherwise a new token is issued with `parent_id` = the presented
token and the presented one is marked `revoked_at`, `revoke_reason='rotated'`,
`replaced_by_id`. The chain is linear: **one presented token yields at most one
successor**, even under concurrent requests *(code follow-up: `HEAD` reads then
issues then marks, unconditionally — two concurrent refreshes with one token
both succeed and fork the chain, so a thief racing the owner is never detected;
§11 row 2)*.

After rotation the user snapshot is re-read: missing, disabled, or not
`approved` → cookies cleared, 401 (so a revoked approval cannot renew forever).
Then a new access token is minted with the current `token_version` and role
codes, cookies are re-set preserving the `remember` choice from
`portal_session`, `account.refresh.rotated` is audited, 200 `{access_token,
expires_in, token_type}`.

**Acceptance criteria.**
- Given a token rotated once, when the old one is presented, then 401, every
  token in the chain is revoked, and one `account.refresh.reuse_detected` row is
  written.
- Given two concurrent refreshes with the same token, then exactly one gets
  200 and the other is treated as reuse.
- Given an approval revoked after login, then the next refresh is 401 and the
  cookies are cleared.

### P0.5 — Request authentication (`RequireAuth`, `OptionalAuth`)

`middleware/auth.go`. The token comes from `Authorization: Bearer` first, else
the `portal_access` cookie. The JWT is verified (P0.3), `sub` must be a UUID,
then `users` is re-read on **every request** (`GetUserAuthSnapshot`):
disabled → 401; approval not `approved` → **403
`account/account-not-approved`** (the one named failure: the caller has proved
who they are, and a 401 would send them to a login that cannot help); stored
`token_version` ≠ claim `tv` → 401. Every other failure is the same 401
about:blank. `OptionalAuth` attaches the identity when the same checks pass and
otherwise lets the request through anonymous.

The account surface itself (`/auth/*`, `/admin/*`) runs under plain
`RequireAuth`, never `RequireTenant`: it touches only global tables (§6).

### P0.6 — Logout, logout-all, me (authenticated)

- `POST /auth/logout` → 204. Revokes the presented refresh token (reason
  `logout`), bumps the caller's `token_version` — which stops every access token
  of that user on every device until each one refreshes (see §10 Q4) — audits
  `account.session.logout`, clears cookies. The frontend refreshes first so an
  expired access token does not 401 the logout (`TopMenu.tsx` `logout`).
- `POST /auth/logout-all` → 204. Revokes every refresh token of the user
  (reason `logout_all`), bumps `token_version`, audits with `scope:
  all_sessions`, clears cookies.
- `GET /auth/me` → 200 `{id, email, display_name, roles, permissions}` —
  `roles` from the token, `permissions` the caller's **effective** codes from
  the engine (wildcards literal; best-effort: `[]` if the cache/DB read fails).
  P0.13 adds `timezone` and `timezone_manual`. *(Code follow-up: the OpenAPI
  `CurrentUser` schema declares camelCase `displayName` and `avatarUrl`, which
  the handler never sends — §11 row 15.)*

### P0.7 — RBAC engine

Canonical per ADR-02; grammar per the specs README AuthZ convention. Owned
here:

- **Grammar** (`rbac/permission.go`): `*` or 2–3 segments of `[a-z0-9_*-]`;
  `Parse` rejects anything else; `MustParse` panics, so a malformed literal in
  `RequirePermission` fails at start-up. The DB CHECK `permissions_code_format`
  (`0003`) enforces the same shape.
- **Matching:** `*` satisfies everything; `<res>:*` satisfies every action and
  every scope on `res`, `:own` included; `*:<action>` every resource for that
  action; a 2-segment or `:any` grant satisfies a bare or `:any` requirement
  but never `:own`; `:own` matches only `:own`; a literal scope matches only
  itself. `Set.AllowsCode` is fail-closed on a malformed required code, even
  for a `*` holder.
- **Hierarchy** (`0003` seed, all `is_system`): `guest ← user ← creator ←
  editor ← moderator ← admin ← superadmin` (child inherits every ancestor).
  Effective set = union over the user's non-expired `user_roles` and all their
  ancestors (`GetEffectivePermissions`, recursive CTE; `UNION` makes it
  cycle-safe). `superadmin` holds `*`. `users:approve` (`0031`) is granted to
  **no** role.
- **Cache** (`rbac/cache.go`): Redis key `rbac:perms:<userID>:v<token_version>`,
  TTL `PERMISSION_CACHE_TTL` (default 5 m), read-through, best-effort on Redis
  errors. Bumping `token_version` is the invalidation; nothing calls
  `Invalidate` in a normal flow.
- **Decision point:** `Engine.Authorize` / `AuthorizeOwnerOr`, wrapped by
  `RequirePermission` (no identity → 401 about:blank; denied → 403 about:blank;
  loader error → 500) and `RequireOwnerOrPermission` (extractor
  `ErrOwnerNotFound` → 404 about:blank; any other extractor error → logged 500).
  Only `account` and the `cmd/` wiring import `account/rbac` (depguard,
  `backend/.golangci.yml`).

**Acceptance criteria.** `TestMatches` table holds for every rule above;
`movies:*` satisfies `movies:write:own`; `movies:write` does not.

### P0.8 — Admin directory and approval queue (`GET /admin/users`, `GET /admin/users/{id}`)

`users:read:any`. Query: `status` (`pending|approved|rejected`, else 422
`account/validation`), `q` (case-insensitive substring of email or display
name, matched literally), `limit` (default **25**, max **100**, lenient and
clamped per the README Pagination rule), `offset` (non-numeric or < 0 → 0).
Order `created_at DESC, id DESC`. Response `{items: AdminUser[], total, limit,
offset, counts: {pending, approved, rejected}}` — `counts` spans the whole
table. `AdminUser` = `{id, email, display_name, avatar_url|null,
approval_status, approval_note|null, approved_at|null, approved_by|null,
disabled, has_password, created_at, roles}` (no credential material; `roles`
excludes expired grants). A malformed or unknown `{id}` is 404
`account/user-not-found`.

*(Code follow-up: `HEAD` answers `{users, …}` (Envelopes), 400
`account/validation` for the status filter, 404 about:blank for a missing user,
and does not escape `%`/`_` in `q` — §11 rows 17, 18, 23. Offset paging with a
`total` is kept pending §10 Q2.)*

### P0.9 — Approval decisions

`POST /admin/users/{id}/approve | reject | revoke-approval`, `users:approve`.
Optional body `{note}` (trimmed, ≤ 500 bytes, else 422 `account/validation`).
Guards, in order:

1. **No self-target:** acting on yourself → 403 `account/self-target`.
2. **No acting on an account that outranks you** (same rule as P0.10's
   escalation guard) → 403 `account/escalation` *(code follow-up: `decide` has
   no target-authority check — §11 row 6)*.
3. **Last approver:** `reject` and `revoke-approval` refuse when the target is
   the only enabled, approved account whose effective set satisfies
   `users:approve` → 409 `account/last-approver`.

Effect (`SetUserApproval`): `approval_status`, `approval_note`,
`approved_at` (`now()`, or NULL for `pending`), `approved_by` (the actor, NULL
for `pending`), and an unconditional `token_version` bump — a rejected or
revoked user's live session dies on its next request. Audited as
`account.user.approved | rejected | approval_revoked`. 200 `AdminUser`. A
rejected row is kept so the email cannot re-register.

### P0.10 — User create, edit, disable, delete

- **Create** `POST /admin/users` (`users:write:any`) `{email, password,
  display_name?}`: email and password rules as P0.1 (422
  `account/invalid-email`, 422 `account/password-policy`; a password is
  required). **The approval state comes from the creator's authority, never the
  body**: `approved` (with `approved_by` = actor) iff the actor's effective set
  allows `users:approve`, else `pending`. Roles are not settable; `user` is
  seeded best-effort. 409 `account/email-taken`. Audit `account.user.created`.
  201 `AdminUser`.
- **Edit** `PATCH /admin/users/{id}` (`users:write:any`) `{email?,
  display_name?, password?}` — omitted or empty fields keep their value. The
  password is validated before anything is written. Editing **yourself** is
  allowed; editing another account whose roles carry any permission you lack,
  or the `superadmin` role without your holding `*`, is 403
  `account/escalation`. A new password also bumps `token_version` and revokes
  every refresh token (reason `admin_password_change`). 409 on a taken email.
  Audit `account.user.updated` (`email_before`, `email_after`,
  `password_changed`).
- **Disable / enable** `POST /admin/users/{id}/disable | enable`
  (`users:write:any`): self → 403 `account/self-target`; the target-authority
  check of Edit → 403 `account/escalation` *(code follow-up: missing on `HEAD`,
  so an `admin` can switch off a superadmin — §11 row 6)*; disabling the last
  approver → 409 `account/last-approver`. Disable stamps `disabled_at` and bumps
  `token_version` (`DisableUser`); enable clears it. Audit
  `account.user.disabled | enabled`. 200 `AdminUser`.
- **Delete** `DELETE /admin/users/{id}` (`users:delete:any`) `{confirm_email}`:
  self → 403 `account/self-target`; `confirm_email` must equal the target's
  email case-insensitively, else 422 `account/confirmation-mismatch` (server-side,
  so a replayed request cannot reach the delete) *(code follow-up: `HEAD`
  answers 400 — §11 row 17)*; escalation → 403; last approver → 409. A hard
  delete: FKs that own content cascade (`ON DELETE CASCADE`), the audit,
  `granted_by`, `approved_by` and `people_persons.linked_user_id` references
  are `SET NULL`. Audit `account.user.deleted`. 204; a second delete is 404.

**Delete removes the account's stored files first** *(Decision 2026-10-01b
(D1); unbuilt — §11 row 8)*. After every guard above has passed:

1. **Disable** the target (`DisableUser`: stamps `disabled_at`, bumps
   `token_version`), so no new authenticated request of theirs — an upload, a
   zip import — can start. This step is part of the delete; it is not audited
   or announced on its own.
2. **Bulk pass.** `mediaapi.PurgeOwnerAssets(ctx, id)` (SPEC-04 P0.7)
   tombstones every asset the target owns, deletes their objects and rows, and
   returns how many rows are left. It is budgeted, so a large library may need
   more than one call.
3. **Locked final pass and delete**, in one account transaction:
   `SELECT 1 FROM users WHERE id = $1 FOR UPDATE` (new query
   `LockUserForDelete`); `PurgeOwnerAssets` again, with a context that does not
   carry this transaction (media opens its own scope); only when it returns
   `0`, `DeleteUser`; commit.
4. Any error, or rows left after either pass: **503
   `account/delete-incomplete`** with `Retry-After: 900` and no further
   change. The user row stays (disabled); the tombstones stay; the hourly
   `media:purge_orphans` finishes them, because the rows — and the user's
   personal organisation that `forEachTenant` iterates — still exist. A
   retried DELETE (the UI may retry at once: each pass purges every `deleting`
   row of the owner, grace or not) continues where the last stopped and
   completes once nothing is left. The account stays disabled until an admin
   enables it.

Why this order is the one that works:

- The only record of an object is its `assets` row (`purgeObjects` derives the
  prefixes from it; the janitor reads only `deleting` rows), and `assets.owner_id`
  cascades from `users` (`0007`). Only tombstoning and then deleting the user
  (the decision's first sketch) would lose the rows in the cascade before the
  janitor's 15-minute grace has passed, so the objects would still leak. The
  user row is therefore deleted only after media reports **zero** rows: the
  cascade never removes an asset row whose objects may still exist.
- **The race with a new upload.** An `assets` INSERT checks its FK by taking
  `FOR KEY SHARE` on the referenced `users` row, which step 3's `FOR UPDATE`
  conflicts with. An insert committed before the lock is seen by the final
  pass and purged; one arriving after it waits, then fails its FK check when
  the delete commits, writing no row (and, through `mediaapi.Ingest`, no
  object — the row precedes the bytes). Disabling first limits that window to
  worker tasks already running for the target (a zip import, a comic sync).
- **No deadlock.** The final pass runs on media's own connection; it updates
  `assets.status` and deletes `assets` and variant rows, none of which touches
  the FK column or locks `users`.
- **No event fan-out against a vanished tenant.** The owner purge publishes no
  `media:asset_deleted` (SPEC-04 P0.7 step 5): every row that references these
  assets is the target's own and goes in the same cascade.
- Not covered: a browser `PUT` to an already-issued presigned URL that lands
  after its asset row was purged leaves an object nothing references — the
  same exposure `DELETE /assets/{id}` has for an `uploading` asset (SPEC-04
  P0.3), not something this order can close.

*Acceptance criteria (delete).*
- Given a user with uploaded video, image and audio assets, when they are
  deleted, then 204, no object remains under any of their prefixes, and no
  `assets` row of theirs remains. *(TC-ACC-064)*
- Given storage failing during the purge, then 503 `account/delete-incomplete`,
  the user still exists and is disabled, and after the next janitor run a
  retried DELETE answers 204 with no object left. *(TC-ACC-065)*
- Given an `assets` INSERT for the target blocked behind step 3's lock, then
  after the delete commits that INSERT fails and no row or object of it
  remains. *(TC-ACC-066)*

### P0.11 — Roles, role assignment and the permission matrix

- **Assign** `PUT /admin/users/{id}/roles` (`rbac:role:assign`) `{roles:
  string[]}` — whole-set replacement in one statement (`ReplaceUserRoles`).
  Self → 403 `account/self-target`; unknown code → 422 `account/unknown-role`;
  for every role in the **symmetric difference** of current and wanted:
  `superadmin` requires the actor to hold `*`, and every permission in the
  role's effective set (own + ancestors) must be held by the actor, else 403
  `account/escalation`. Then `token_version` is bumped, audit
  `account.user.roles_changed` (`before`, `after`). 200 `AdminUser`. A wanted
  role whose previous grant had expired is granted afresh *(latent on `HEAD`:
  `ON CONFLICT DO NOTHING` keeps the expired row — §11 row 11)*.
- **Matrix** `GET /admin/permission-matrix` (`rbac:role:read`) → `{roles:
  MatrixRole[], permissions: [{code, description, group}]}`; each role carries
  `direct` (editable) and `effective` (own + ancestors, read-only) codes,
  `parent_code`, `is_system`, `user_count`. A composite read, so its two arrays
  keep their names (README Pagination).
- **Role grants** `PUT /admin/roles/{id}/permissions` (`rbac:role:write`)
  `{permissions: string[]}`: unknown role → 404 `account/role-not-found`;
  unknown code → 422 `account/unknown-permission`; every code in the symmetric
  difference must be held by the actor (you can neither grant nor strip what you
  lack) → 403 `account/escalation`. `token_version` is bumped for every holder
  of the role **and of every descendant role** (`ListUserIDsAffectedByRole`);
  their sessions survive on the refresh cookie. Audit
  `account.role.permissions_changed`. 200 = the recomputed matrix.
- **Role CRUD** (`rbac:role:write`): `POST /admin/roles` `{code, name?,
  description?, parent_code?}` — code lower-cased, 1–40 of `[a-z0-9_-]`, else
  422 `account/validation`; taken → 409 `account/role-exists`; unknown parent →
  422 `account/unknown-role`; `name` defaults to the code; created non-system;
  audit `account.role.created`; 201 `AdminRole`. `PATCH /admin/roles/{id}` —
  `code` immutable; system role → 403 `account/role-protected`; a parent that
  closes a loop → 422 `account/role-cycle`; **omitted fields keep their value**
  *(code follow-up: `HEAD` is a full replace — an omitted `name` becomes empty,
  an omitted `description` or `parent_code` is cleared; §11 row 9)*; **a parent
  change is an escalation for every holder of the role, so the actor must hold
  every permission the new parent's effective set adds** → 403
  `account/escalation` *(code follow-up: `HEAD` has no such check — §11 row 1)*;
  holders are re-keyed; audit `account.role.updated`. `DELETE /admin/roles/{id}`
  — system → 403; still assigned or still a parent → 409 `account/role-in-use`;
  audit `account.role.deleted`; 204.

**Acceptance criteria (escalation).**
- Given an `admin` (no `*`, no `users:approve`), when they grant a role carrying
  `users:approve`, or strip it from a role, then 403 `account/escalation`.
- Given an `admin` and a custom role R held by some user, when they PATCH R's
  `parent_code` to `superadmin`, then 403 `account/escalation` and R is unchanged.
- Given an `admin`, when they disable, reject or edit a superadmin, then 403
  `account/escalation`.
- Given a permission revoked from `creator`, then every `editor` holder's next
  request re-resolves (their `token_version` moved).

### P0.12 — Superadmin bootstrap for existing installs

`BOOTSTRAP_SUPERADMIN_EMAIL` (empty = off). On every `cmd/api` start
(`bootstrapSuperadmin`), the named account is made to hold `superadmin`, be
`approved` and be **enabled**, writing only what is missing (so a correct
install bumps nothing and keeps its permission cache). An unknown email is a
logged warning. A role grant bumps `token_version`. *(Code follow-up: `HEAD`
never clears `disabled_at` despite its own doc comment — §11 row 7.)*

### P0.13 — Per-user timezone *(decided 2026-09-30 and 2026-10-01 (f); unbuilt)*

The specs README **Timezone** convention is binding and is not restated in
full; the account half is:

- `users.timezone` (IANA name) defaults to **`Asia/Ho_Chi_Minh`**;
  `users.timezone_manual boolean NOT NULL DEFAULT false`.
- `PATCH /api/v1/auth/me {timezone, timezone_manual?}` — authenticated, the
  caller's own row only. `timezone` is required and validated with
  `time.LoadLocation`; empty or unknown → 422 `account/invalid-timezone`, nothing
  written. An omitted `timezone_manual` leaves the flag unchanged. 200 = the
  updated `/auth/me` body.
- `GET /auth/me` returns `timezone` and `timezone_manual`.
- `accountapi.UserSummary.Timezone`, plus a batch lookup by user ids for sweeps;
  an unparseable stored name falls back to `Asia/Ho_Chi_Minh` with a logged
  warning. Modules never query `users` themselves.
- Frontend: after sign-in, when `timezone_manual` is false and the device zone
  (`Intl.DateTimeFormat().resolvedOptions().timeZone`) differs, save
  `{timezone}`; settings offer an IANA picker (`timezone_manual: true`) and "use
  my location" (`timezone_manual: false`); `lib/time.ts` takes the display zone
  from `/auth/me`, and `GET /api/v1/time` keeps only the server clock.

*(Code follow-up: none of it exists — `0002_account_users` defaults `'UTC'`,
there is no `PATCH /auth/me`, `UserSummary` is `{ID, Email, DisplayName}`, and
`cmd/api` `handleServerTime` serves `APP_TIMEZONE`; §11 row 14.)*

**Acceptance criteria.**
- Given a fresh migration, then every existing `'UTC'` row reads
  `Asia/Ho_Chi_Minh` and `timezone_manual` is false.
- Given `PATCH /auth/me {timezone: "Mars/Olympus"}`, then 422
  `account/invalid-timezone` and the row is unchanged.
- Given `timezone_manual: true`, then the post-sign-in device save does not run.
- Given two users in different zones, then `accountapi` returns each user's own
  zone in one batch call.

### P0.14 — Cross-module API (`account/api`)

The only package other modules import (`api/api.go`):
`GetUserByID(ctx, id) (*UserSummary, error)` — `(nil, nil)` when absent,
disabled **or not approved** *(code follow-up: `HEAD` hides only disabled
accounts — §11 row 12)*; `HasPermission(ctx, code) bool` — resolves the
principal from the request context, fail-closed on every error;
`ListDirectory(ctx, exclude, limit)` — approved, enabled accounts other than
the caller, ordered `display_name, id`, `limit` default 50, max 200, clamped
*(code follow-up: `HEAD` resets an out-of-range limit to 50 — §11 row 19)*;
`GetUserNames(ctx, ids) (map[uuid]string, error)`; P0.13's timezone reads;
and P1.3's `SuperadminIDs(ctx) ([]uuid.UUID, error)`.
Not permission-gated at this layer: the caller decides who may see the result.

### P0.15 — Maintenance tasks

`account:purge_refresh_tokens` — a periodic task on the shared scheduler
(`@every 24h`, `default` queue, light server; `cmd/worker` wires the repository
directly because the worker does not construct the module) that hard-deletes
refresh tokens expired more than 30 days. The account-owned
`password_reset_tokens` table gets the same treatment (expired > 7 days,
`PurgeExpiredPasswordResetTokens`) *(code follow-up: the query and adapter
exist, nothing schedules them — §11 row 20)*. Both rows are registered in
[events.md](../../reference/events.md) *(neither is on `HEAD` — §11 row 20)*.

### P1 — nice to have

- **P1.1 Session list and revoke** — `GET /me/sessions` (active refresh tokens
  with IP and user agent; `ListActiveRefreshTokensForUser` already exists) and
  `DELETE /me/sessions/{id}` ([security.md §2.5](../../architecture/security.md)).
  *AC:* revoking one session leaves the others working.
- **P1.2 Security alert on refresh reuse** — the reuse path also dispatches
  `account.security_alert` to the user; the notify side is SPEC-05 P1.4.
  *AC:* a detected reuse produces one bell row and one email.
- **P1.3 Admin-change events** *(Decision 2026-10-01b (D3); unbuilt — §11
  row 24)*. Account is not exempt from ADR-08's "every domain module emits ≥ 1
  bus event": it announces every admin-relevant change on the bus, and notify
  turns each into a bell entry for the superadmins (SPEC-05 P1.5). The events
  are published through `platform/events` **after the write commits** — the
  account surface runs no request transaction (P0.5), so "after the store call
  returns" is after commit; a failed write publishes nothing, and a publish
  error is logged and never fails the request. Names, the payload struct and
  the publisher dependency live in `account/api` and `account.Deps` (`Events`);
  `cmd/api` (the only emitting binary) creates its publisher before
  `account.New` — today `mediaEvents` is built after it — and registers each
  consumer edge with `Subscribe`.

  | Event | Emitted by (after commit) | Extra payload fields |
  |---|---|---|
  | `account:user_registered` | `Register` (P0.1), after the founder bootstrap | `approval_status`; `actor_id` = the registrant |
  | `account:user_approval_decided` | `decide` (P0.9) | `decision`: `approved` \| `rejected` \| `approval_revoked`; `note` |
  | `account:user_access_changed` | `SetDisabled` (P0.10) | `access`: `disabled` \| `enabled` |
  | `account:user_deleted` | `DeleteUser` (P0.10), after the delete commits | — (the email and name ride in the common fields, since the row is gone when notify runs) |
  | `account:user_roles_changed` | `SetUserRoles` (P0.11) | `roles_before`, `roles_after` (role codes) |
  | `account:role_changed` | role create / PATCH / delete and `SetRolePermissions` (P0.11) | `role_id`, `role_code`, `change`: `created` \| `updated` \| `deleted` \| `permissions_changed` |
  | `account:refresh_reuse_detected` | `HandleRefresh` on `auth.ErrTokenReused` (P0.4), after `RevokeChain` | `actor_id` is null (no admin acted) |

  Every payload is one struct, `accountapi.AdminEvent`: `{event_id, occurred_at,
  actor_id|null, actor_name, user_id?, user_email?, user_name?}` plus the extra
  fields above. `event_id` is a fresh UUID per publish — the natural id notify
  uses as `dedup_key`, since these changes have no row of their own that
  identifies one occurrence; `actor_name` comes from the caller's token
  (`auth.Identity.DisplayName`); the user fields are read before a delete.
  The **superadmin set** is `accountapi.SuperadminIDs(ctx)`: enabled, approved
  accounts holding a non-expired grant of the `superadmin` role, directly or
  through a custom role parented under it (the recursive walk of
  `GetEffectivePermissions`), at most 50; `cmd/worker`, which does not construct the
  module, satisfies notify's port with the same query over the account
  repository, as it does for `ResolveRecipient`. Registration keeps its
  existing approver dispatch (P0.1); see SPEC-05 P1.5 for how the superadmin
  fan-out avoids a duplicate.
  *AC:* given an admin disabling a user, then exactly one
  `account:user_access_changed` with `access: disabled`, the admin as
  `actor_id` and the user's email and name is enqueued after the write; a
  failed write enqueues none *(TC-ACC-120)*; given each of the seven
  operations, then its event is published with the fields above
  *(TC-ACC-121…126)*; given a refused delete (P0.10 step 4), then no
  `account:user_deleted` *(TC-ACC-123)*.

### P2 — future considerations (design for, don't build)

- MFA/TOTP and step-up for destructive admin actions ([D-27]/[D-28],
  [security.md §2.4](../../architecture/security.md)); re-entry per
  [backlog.md § Deferred](../backlog.md).
- Policy bundles and user groups layered on roles (ADR-02); tenant-scoped role
  grants (ADR-07 steps 5–7).
- An audit-log reader for `audit:read` holders.

## 6. Data model

All account tables are **global** — no `tenant_id`, no RLS policy — per the
specs README Tenancy convention and ADR-07: they are read before any tenant is
resolved (`RequireAuth` runs ahead of `RequireTenant`). `refresh_tokens` and
`password_reset_tokens` are global for the same reason; the README's list of
exempt tables names only `users`, `roles`, `permissions`, `role_permissions`
and `user_roles` and should add them. `audit_log` is owned by `platform/audit`
(`0005_platform_audit`, [D-25]), not by this module.

| Table | Created by | Columns (as shipped) | Constraints and indexes |
|---|---|---|---|
| `users` | `0002_account_users`; `0006_account_local_auth`; `0031_account_user_approval` | `id uuid PK`, `oidc_subject text` (legacy, nullable since `0006`), `email text`, `display_name text`, `avatar_url text`, `role text DEFAULT 'user'` (legacy label — authorisation is `user_roles`), `locale text DEFAULT 'en-US'`, `timezone text DEFAULT 'UTC'`, `token_version int DEFAULT 1`, `disabled_at timestamptz`, `created_at`, `updated_at`; `0006`: `password_hash text` (Argon2id PHC, nullable), `password_updated_at`; `0031`: `approval_status text NOT NULL DEFAULT 'pending'`, `approval_note text`, `approved_at`, `approved_by uuid → users ON DELETE SET NULL` | `email UNIQUE` (case-sensitive; every write path lower-cases first), `oidc_subject UNIQUE`, `users_approval_status_check` (`pending\|approved\|rejected`), partial `users_approval_pending_idx (created_at DESC, id DESC) WHERE approval_status <> 'approved'`. `0031` back-filled every existing row to `approved`. |
| `roles` | `0003_account_rbac` | `id`, `code text`, `name`, `description`, `parent_id → roles ON DELETE SET NULL`, `is_system bool`, timestamps | `code UNIQUE`; `roles_no_self_parent` CHECK (longer cycles are refused in the handler, P0.11); `roles_parent_idx`. Seven seeded rows, all `is_system`. |
| `permissions` | `0003` (catalog), `0031` (`users:approve`); other modules seed their own codes | `id`, `code`, `description`, `created_at` | `code UNIQUE`; `permissions_code_format` CHECK (P0.7 grammar). |
| `role_permissions` | `0003` | `role_id → roles CASCADE`, `permission_id → permissions CASCADE`, `granted_at`, `granted_by → users SET NULL` | PK `(role_id, permission_id)`. |
| `user_roles` | `0003` | `user_id → users CASCADE`, `role_id → roles CASCADE`, `granted_at`, `granted_by → users SET NULL`, `expires_at` (nothing sets it today) | PK `(user_id, role_id)`; `user_roles_user_idx`. |
| `refresh_tokens` | `0004_account_sessions` | `id`, `user_id → users CASCADE`, `token_hash bytea`, `expires_at`, `created_at`, `revoked_at`, `revoke_reason`, `replaced_by_id`, `parent_id` (both `→ refresh_tokens SET NULL`), `issued_ip inet`, `issued_user_agent` | `refresh_tokens_hash_idx` UNIQUE; partial `refresh_tokens_user_idx`, `refresh_tokens_expiry_idx` `WHERE revoked_at IS NULL`. |
| `password_reset_tokens` | `0010_account_password_reset_tokens` (behaviour: SPEC-05 P0.3) | `id`, `user_id → users CASCADE`, `token_hash bytea UNIQUE`, `expires_at`, `used_at`, `created_at` | `password_reset_tokens_user_idx`. |

`0006` dropped `user_oidc_roles` (D-26.r1). `updated_at` is set explicitly by
every UPDATE in `query/*.sql` (README updated_at convention).

**Target migration `000N_account_user_timezone`** (P0.13; verify the next free
number with `ls backend/db/migrations | tail -2`):

```sql
ALTER TABLE users ALTER COLUMN timezone SET DEFAULT 'Asia/Ho_Chi_Minh';
-- nothing has ever written users.timezone, so every 'UTC' row is untouched
UPDATE users SET timezone = 'Asia/Ho_Chi_Minh', updated_at = now() WHERE timezone = 'UTC';
ALTER TABLE users ADD COLUMN timezone_manual boolean NOT NULL DEFAULT false;
```

The down migration drops `timezone_manual` and restores the `'UTC'` default
(it does not rewrite rows back).

**Takeout** (README convention): the user's own `users` row exports as one JSON
object `{email, display_name, avatar_url, locale, timezone, timezone_manual,
created_at, roles}`. Excluded, with reason: `password_hash`,
`refresh_tokens`, `password_reset_tokens` (credential material) and
`token_version` / approval fields (operator state, not the user's data).

## 7. API summary

Every path is under `/api/v1`. "public" operations declare `security: []`;
every other operation declares `security: [{bearerAuth: []}]` and, when behind
`RequirePermission`, `x-required-permission: <code>` (README AuthZ, OpenAPI
encoding). `module.go` `MountHTTP` / `mountAdmin` **is** the authorisation
policy; this table copies it.

| Method | Path | Permission | Request | Response | Errors (beyond 401 about:blank on authenticated routes) |
|---|---|---|---|---|---|
| POST | `/auth/register` | public | `{email, password, display_name?}` | 201 `{status, email, approval_status}` | 400 about:blank (body), 422 `account/invalid-email`, 422 `account/password-policy`, 409 `account/email-taken`, 429 `platform/rate-limited` |
| POST | `/auth/login` | public | `{email, password, remember?}` | 200 `{access_token, expires_in, token_type, user}` + cookies | 400 about:blank, 401 `account/invalid-credentials`, 403 `account/account-disabled` \| `account/account-pending` \| `account/account-rejected`, 429 `account/too-many-attempts` |
| POST | `/auth/refresh` | public | cookie or `{refresh_token}` | 200 `{access_token, expires_in, token_type}` + cookies | 401 about:blank (missing, invalid, expired, reused, user unavailable) |
| POST | `/auth/logout` | authenticated | — | 204 | — |
| POST | `/auth/logout-all` | authenticated | — | 204 | — |
| GET | `/auth/me` | authenticated | — | 200 `{id, email, display_name, roles, permissions, timezone, timezone_manual}` | — |
| PATCH | `/auth/me` | authenticated | `{timezone, timezone_manual?}` | 200 `/auth/me` body | 422 `account/invalid-timezone`, 422 `account/validation` (P0.13, unbuilt) |
| POST | `/auth/forgot-password`, `/auth/reset-password` | public | — | — | owned by SPEC-05 P0.3 |
| GET | `/admin/users?status=&q=&limit=&offset=` | `users:read:any` | — | 200 `{items: AdminUser[], total, limit, offset, counts}` | 422 `account/validation` |
| GET | `/admin/users/{id}` | `users:read:any` | — | 200 `AdminUser` | 404 `account/user-not-found` |
| POST | `/admin/users` | `users:write:any` | `{email, password, display_name?}` | 201 `AdminUser` | 422 `account/invalid-email`, 422 `account/password-policy`, 409 `account/email-taken` |
| PATCH | `/admin/users/{id}` | `users:write:any` | `{email?, display_name?, password?}` | 200 `AdminUser` | 422 (as create), 403 `account/escalation`, 404 `account/user-not-found`, 409 `account/email-taken` |
| DELETE | `/admin/users/{id}` | `users:delete:any` | `{confirm_email}` | 204 | 422 `account/confirmation-mismatch`, 403 `account/self-target` \| `account/escalation`, 404, 409 `account/last-approver`, 503 `account/delete-incomplete` (P0.10 step 4, `Retry-After`) |
| POST | `/admin/users/{id}/approve` | `users:approve` | `{note?}` | 200 `AdminUser` | 422 `account/validation`, 403 `account/self-target` \| `account/escalation`, 404 |
| POST | `/admin/users/{id}/reject` | `users:approve` | `{note?}` | 200 `AdminUser` | as approve, plus 409 `account/last-approver` |
| POST | `/admin/users/{id}/revoke-approval` | `users:approve` | `{note?}` | 200 `AdminUser` | as reject |
| POST | `/admin/users/{id}/disable` | `users:write:any` | — | 200 `AdminUser` | 403 `account/self-target` \| `account/escalation`, 404, 409 `account/last-approver` |
| POST | `/admin/users/{id}/enable` | `users:write:any` | — | 200 `AdminUser` | 403 `account/self-target` \| `account/escalation`, 404 |
| PUT | `/admin/users/{id}/roles` | `rbac:role:assign` | `{roles}` | 200 `AdminUser` | 422 `account/unknown-role`, 403 `account/self-target` \| `account/escalation`, 404 |
| GET | `/admin/permission-matrix` | `rbac:role:read` | — | 200 `{roles, permissions}` | — |
| POST | `/admin/roles` | `rbac:role:write` | `{code, name?, description?, parent_code?}` | 201 `AdminRole` | 422 `account/validation` \| `account/unknown-role`, 409 `account/role-exists` |
| PATCH | `/admin/roles/{id}` | `rbac:role:write` | `{name?, description?, parent_code?}` | 200 `AdminRole` | 422 `account/unknown-role` \| `account/role-cycle`, 403 `account/role-protected` \| `account/escalation`, 404 `account/role-not-found` |
| DELETE | `/admin/roles/{id}` | `rbac:role:write` | — | 204 | 403 `account/role-protected`, 404 `account/role-not-found`, 409 `account/role-in-use` |
| PUT | `/admin/roles/{id}/permissions` | `rbac:role:write` | `{permissions}` | 200 `{roles, permissions}` | 422 `account/unknown-permission`, 403 `account/escalation`, 404 `account/role-not-found` |

Every `/admin/*` route answers 403 about:blank when the permission is missing
(`RequirePermission`), and every authenticated route answers 403
`account/account-not-approved` for an unapproved caller (P0.5).

**Pagination.** `GET /admin/users` is the module's only list endpoint: `limit`
default 25, max 100, lenient and clamped; `offset` ≥ 0; ordered `(created_at
DESC, id DESC)`. Its envelope is `{items, …}` per the README Pagination
convention; whether it moves from offset to a keyset cursor is §10 Q2.

**Problem types** introduced by this spec (each registered in
`frontend/src/lib/problems.ts`, README Errors): `account/invalid-email`,
`account/password-policy`, `account/email-taken`, `account/invalid-credentials`,
`account/too-many-attempts`, `account/account-disabled`,
`account/account-pending`, `account/account-rejected`,
`account/account-not-approved`, `account/validation`, `account/user-not-found`,
`account/self-target`, `account/escalation`, `account/last-approver`,
`account/confirmation-mismatch`, `account/unknown-role`,
`account/unknown-permission`, `account/role-not-found`, `account/role-exists`,
`account/role-protected`, `account/role-cycle`, `account/role-in-use`,
`account/invalid-timezone`, `account/delete-incomplete`. `platform/rate-limited` belongs to
`platform/middleware`. *(Code follow-up: §11 rows 16–17.)*

## 8. Events

**Emitted bus events** — P1.3, planned (Decision 2026-10-01b (D3): no
exemption from ADR-08's "≥ 1 bus event" rule). `HEAD` emits none (§11 row 24).
All are emitted from `cmd/api`, after commit, with the `accountapi.AdminEvent`
payload of P1.3; each has one consumer, a SPEC-05 P1.5 notify task on the light
server's `default` queue.

| Event | Consumer task | Notify type |
|---|---|---|
| `account:user_registered` | `notify:on_user_registered` | `account.registration_pending` (shared with P0.1's approver dispatch, so a recipient who already has it gets no second row) |
| `account:user_approval_decided` | `notify:on_user_approval_decided` | `account.approval_decided` |
| `account:user_access_changed` | `notify:on_user_access_changed` | `account.user_access_changed` |
| `account:user_deleted` | `notify:on_user_deleted` | `account.user_deleted` |
| `account:user_roles_changed` | `notify:on_user_roles_changed` | `account.user_roles_changed` |
| `account:role_changed` | `notify:on_role_changed` | `account.role_changed` |
| `account:refresh_reuse_detected` | `notify:on_refresh_reuse_detected` | `account.refresh_reuse_detected` (in-app + email) |

They are admin notices, not life-stream moments: the stream does not project
them.

**Tasks the module causes or owns:**

| Name | Kind | Status | Note |
|---|---|---|---|
| `notify:dispatch` | task, owned by notify | live | account enqueues it through `notifyapi.Enqueue` for `account.registration_pending` (P0.1) and `account.password_reset` (SPEC-05 P0.3). Already in events.md as a notify row. |
| `account:purge_refresh_tokens` | periodic task | live (`cmd/worker`, `@every 24h`, `default`) | **missing from events.md** — drift (§11 row 20). |
| `account:purge_reset_tokens` | periodic task | planned (P0.15) | name proposed here; lands with its `scheduler.Register` and events.md row. |

**Consumed:** nothing. Audit rows (`account.*`, P0.1–P0.12) are not bus
events — P1.3's events are published beside them, not instead of them; the
audit taxonomy lives in `platform/audit/logger.go` and
[MODULES.md §5.3](../../../backend/MODULES.md).

## 9. Success metrics (n=1 honest)

- Zero sessions held by a non-`approved` or disabled account after the
  decision: auditable by joining `audit_log` `account.user.rejected |
  approval_revoked | disabled` rows to later `account.refresh.rotated` rows for
  the same actor (expected: none).
- Zero `account.refresh.reuse_detected` rows caused by the owner's own browser
  tabs over a month (a non-zero count means the P0.4 race or the SessionKeeper
  claim is misfiring, not theft).
- Every approver notification reaches the bell within one minute of a
  registration (dispatch is best-effort; the metric is whether it is observed).
- After P0.13 lands: no reader in SPEC-12/07/09/11/13 still resolves a day
  boundary from `APP_TIMEZONE` (grep `AppTimezone` in `backend/`).

## 10. Open questions

Q1 (deleting a user orphans their media objects) was decided on 2026-10-01 —
Decision 2026-10-01b (D1), now P0.10's delete order and SPEC-04 P0.7; the
remaining questions keep their numbers so citations hold.

- **Q2 (product, non-blocking) — admin list paging.** The README Pagination
  convention says lists are keyset-paged; the admin grid pages by `offset` with
  a `total` and per-status `counts`. Keep offset for this one operator list, or
  move to a cursor and drop `total`?
- **Q3 (product, non-blocking) — registration enumeration.** `POST
  /auth/register` answers 409 `account/email-taken`, which reveals whether an
  address has an account, while `forgot-password` goes to lengths not to. On a
  household instance this may be acceptable; if not, register must answer 201
  uniformly and §11 row 5 becomes meaningful.
- **Q4 (product, non-blocking) — logout scope.** `/auth/logout` bumps
  `token_version`, so every device's access token stops until it refreshes
  (silent, via `SessionKeeper`), and the RBAC cache is re-keyed. Intended, or
  should single-device logout only revoke its own refresh token?
- **Q5 (product, non-blocking) — per-account lockout.** Five wrong passwords
  lock any known email out of login for 15 minutes, which anyone can trigger.
  Accept for v1, or key the account counter on (email, IP) and keep only the
  per-IP cap global?

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the docs commits on top of it change no
code). The spec text above is the target; this section lists every place the
shipped code diverges from it. Rows are ordered by severity: **Sec** 1–5,
**AuthZ** 6–7, **Data** 8, **Integ** 9–13, **Func** (timezone) 14,
**Contract** 15–20, **UX** 21–22, **Hyg** 23, then **P1** 24 (added by
Decision 2026-10-01b and appended rather than renumbering rows other documents
cite). A row closes when the code
matches the requirement it cites and the SPEC-01 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded
on a named test. File paths are relative to
`backend/internal/modules/account/` unless they start with `backend/`,
`frontend/`, `shared/` or `docs/`. `TC-ACC-*` ids are proposed for a future
`docs/testing/TEST-CASES-SPEC-01-account.md`.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.11 role PATCH — escalation on re-parent | A `parent_code` change is refused (403 `account/escalation`) unless the actor holds every permission the new parent's effective set adds to the role. | `handler/admin.go` `UpdateRole` checks only existence, `is_system` and `wouldCycle`, then `Store.UpdateRole` + `BumpTokenVersionForRole`. An `rbac:role:write` holder (the seeded `admin`) can create a role with no permissions, get it assigned (the `SetUserRoles` guard passes — the role carries nothing), then re-parent it under `superadmin`: every holder inherits `*`. `CreateRole` with a privileged parent is harmless only because assignment is guarded afterwards. | **backend:** in `UpdateRole`, when the parent changes, compute `effectiveRolePermissions` of the role under the new parent and refuse if any code is not `AllowsCode` by the actor (and refuse a `superadmin` ancestor without `*`); apply the same check in `CreateRole` for symmetry. **test:** TC-ACC-071, TC-ACC-072. | Found while writing SPEC-01, 2026-10-01 |
| 2 | P0.4 refresh — one successor per token | Rotation is atomic: one presented token yields at most one successor; a concurrent second presentation is reuse. | `auth/refresh.go` `Rotate`: `GetByHash` → check `RevokedAt` → `issue` → `MarkReplaced`; `query/auth.sql` `MarkRefreshTokenReplaced` updates `WHERE id = $1` unconditionally. Two concurrent requests both pass the check and both get tokens — a forked chain and no theft detection. The benign variant: `SessionKeeper.tsx`'s localStorage claim is not atomic across tabs, so two tabs can still present one token back to back, and the second burns the chain (spurious logout). | **backend/query:** claim first — `UPDATE refresh_tokens SET revoked_at = now(), revoke_reason = 'rotated' WHERE id = $1 AND revoked_at IS NULL RETURNING id`; no row → reuse path; then issue the successor and set `replaced_by_id`, in one transaction. **frontend (optional):** a `navigator.locks` / BroadcastChannel single-flight in `SessionKeeper`. **test:** TC-ACC-031 (concurrent rotation), TC-ACC-032 (reuse burns chain). | Found while writing SPEC-01, 2026-10-01 |
| 3 | P0.1 abuse control | `/auth/register` (and `/auth/refresh`) throttled per client IP; over the limit → 429 `platform/rate-limited`. | `backend/internal/platform/middleware/ratelimit.go` `IPRateLimiter` is referenced by no binary (`grep -rn "ratelimit\." backend --include=*.go` finds no caller); `traefik/dynamic.yml` defines `rate-limit` but no router in `docker-compose.yml` attaches it. Every anonymous `POST /auth/register` creates a pending row and enqueues up to 50 `notify:dispatch` with an email channel (`notifyApprovers`). | **backend:** mount `IPRateLimiter` (e.g. 5/s burst 10, the preset in its doc comment) on `/auth/register` and `/auth/refresh` in `cmd/api`, keyed on the trusted client IP (row 4); optionally a per-IP daily cap on registrations in Redis. **test:** TC-ACC-012, TC-ACC-013. | Found while writing SPEC-01, 2026-10-01 |
| 4 | P0.2 client IP | The login throttle and audit IP use the client address as seen by the trusted proxy only. | `handler/util.go` `clientIP` takes the leftmost `X-Forwarded-For` from any peer, and `cmd/api` adds `chimw.RealIP` (which also trusts `True-Client-IP`/`X-Real-IP`). Behind Traefik's default forwarded-header handling the value is the real peer; on any path that reaches the API directly (`make dev` on `:8080`, the direct-IP shape `cookieDomainFor` supports) a client can rotate it and bypass the per-IP counter (the per-email counter still holds). | **backend:** honour forwarded headers only from a configured trusted-proxy CIDR (`TRUSTED_PROXIES`), else use `RemoteAddr`; drop `chimw.RealIP` or configure it the same way. **test:** TC-ACC-006. | Found while writing SPEC-01, 2026-10-01 |
| 5 | P0.2 uniform timing | An unknown email costs the same as a wrong password. | `handler/auth.go` `Login` returns 401 on `ErrUserNotFound` before `auth.VerifyPassword`, so unknown emails skip the ~64 MiB Argon2id and answer measurably faster. Moot while §10 Q3 keeps register's 409. | **backend:** on `ErrUserNotFound` (and on an empty hash) verify against a fixed dummy PHC hash before answering. **test:** TC-ACC-007 (both paths call the verifier once). | Found while writing SPEC-01, 2026-10-01; §10 Q3 |
| 6 | P0.9, P0.10 target authority on approve / reject / revoke / disable / enable | The `targetAuthorityDenial` rule of Edit and Delete also gates the approval decisions and disable/enable → 403 `account/escalation`. | `handler/admin.go` `decide` and `SetDisabled` check self-target and last-approver only. A `users:write:any` holder (the seeded `admin`) can disable any superadmin who is not the last approver, killing their session (`DisableUser` bumps `token_version`); `/CLAUDE.md` describes only Edit/Delete as takeover-guarded. | **backend:** call `targetAuthorityDenial` in `decide` and `SetDisabled` (skip for `id == actor`, already refused). **openapi:** 403 `account/escalation` on the five operations. **test:** TC-ACC-055, TC-ACC-056. | Found while writing SPEC-01, 2026-10-01 |
| 7 | P0.12 bootstrap re-enables | The named account ends up approved, **enabled** and holding `superadmin`. | `backend/cmd/api/main.go` `bootstrapSuperadmin` grants the role and approves, but never reads or clears `disabled_at`, although its doc comment promises "approved, enabled and holding `superadmin`". A disabled bootstrap account stays locked out. | **backend:** if `user.Disabled`, call `EnableUser` (and bump `token_version` once). **test:** TC-ACC-020 (fake adapter: disabled + unapproved + no role → all three fixed, one bump; already-correct → zero writes). | Found while writing SPEC-01, 2026-10-01 |
| 8 | P0.10 delete — no orphaned objects | Deleting a user leaves no unreachable stored object: disable, purge the owner's assets (`mediaapi.PurgeOwnerAssets`), then — under `FOR UPDATE` on the user row — a final pass that must report 0 rows before `DeleteUser`; otherwise 503 `account/delete-incomplete` and the user is kept. | `handler/admin_users.go` `DeleteUser` → `query/admin.sql` `DeleteUser` hard-deletes straight after the guards; `assets.owner_id` is `ON DELETE CASCADE` (`0007_media_assets`), so asset rows vanish without a `deleting` tombstone and `media/service.go` `PurgeOrphans` (which reads only `ListAssetsForPurge` rows) never visits them; the objects stay in MinIO/R2 forever. | **backend (media):** SPEC-04 §11 row 19 (`PurgeOwnerAssets`). **backend (account):** a one-method media port in `account.Deps`, bound by `cmd/api` through a closure over `mediaMod`; `DeleteUser` runs P0.10 steps 1–4; new query `LockUserForDelete` (`SELECT 1 FROM users WHERE id = $1 FOR UPDATE`) and a transaction for step 3 (the adapter receives `RunInTx` as other modules' do); 503 `account/delete-incomplete` with `Retry-After`. **openapi:** the 503 on `adminDeleteUser`. **frontend:** the slug in `problems.ts`; `/admin/users` offers a retry. **test:** TC-ACC-064…066. | Found while writing SPEC-01, 2026-10-01; Decision 2026-10-01b (D1) |
| 9 | P0.11 role PATCH semantics | PATCH keeps omitted fields. | `handler/admin.go` `decodeRoleBody` sets `Name = Code` when `name` is empty — on PATCH `code` is normally absent, so `Name` becomes `""`; `query/rbac.sql` `UpdateRole` then writes `name`, `description` and `parent_id` from the body, clearing an omitted description or parent. `shared/openapi.yaml` `RoleInput` declares `name` required, which hides it; no UI calls `updateRole` today (`frontend/src/lib/admin.ts` exports it unused). | **backend:** read the target row and merge (absent JSON key ⇒ keep; explicit `""` for `parent_code` ⇒ root role); keep `name` non-empty. **openapi:** a separate `RolePatch` schema with all-optional fields. **test:** TC-ACC-073. | Found while writing SPEC-01, 2026-10-01 |
| 10 | P0.1 first-run bootstrap race | Exactly one founder, even when the first two registrations race. | `handler/auth.go` `Register`: `CreateLocalUser`, then `CountUsers` outside any transaction — two concurrent first registrations can both count 2, so nobody is approved and nobody can approve (recoverable only with `BOOTSTRAP_SUPERADMIN_EMAIL`). | **backend:** decide the founder inside one transaction under `pg_advisory_xact_lock` (or `INSERT … SELECT … WHERE NOT EXISTS (SELECT 1 FROM users)` returning a founder flag). **test:** TC-ACC-016. | Found while writing SPEC-01, 2026-10-01 |
| 11 | P0.11 expired grants | Re-assigning a role whose grant expired grants it afresh; `user_count` counts live grants. | `query/admin.sql` `ReplaceUserRoles` inserts `ON CONFLICT (user_id, role_id) DO NOTHING`, so an expired row is kept and the role stays invisible; `ListRolesAdmin`'s `user_count` subquery ignores `expires_at`, so `DeleteRole` refuses a role only expired grants reference. Latent: nothing sets `expires_at` today. | **query:** `ON CONFLICT … DO UPDATE SET expires_at = NULL, granted_by = EXCLUDED.granted_by, granted_at = now()`; filter `user_count` by `expires_at IS NULL OR expires_at > now()`. **test:** TC-ACC-058. | Found while writing SPEC-01, 2026-10-01 |
| 12 | P0.14 `accountapi` visibility | `GetUserByID` hides disabled **and** non-approved accounts; `GetUserNames` resolves names for any id (display only). | `repository/adapter.go` `GetUserSummaryByID` filters `disabled_at` only, so a pending or rejected account resolves — including as an email recipient through `cmd/worker` `recipientResolver`. | **backend:** add `approval_status = 'approved'` to the check (or a dedicated query). **test:** TC-ACC-090. | Found while writing SPEC-01, 2026-10-01 |
| 13 | P0.2 disabled-attempt audit | A correct-password login on a disabled account writes `account.session.disabled_attempt`. | `backend/internal/platform/audit/logger.go` defines `ActionAuthDisabledAttempt`; `handler/auth.go` `Login` returns 403 without auditing. | **backend:** write the event before the 403. **test:** TC-ACC-004. | Found while writing SPEC-01, 2026-10-01 |
| 14 | P0.13 per-user timezone | Migration (`Asia/Ho_Chi_Minh` default, `'UTC'` rows rewritten, `timezone_manual`); `PATCH /auth/me` with 422 `account/invalid-timezone`; `/auth/me` returns both fields; `accountapi` single + batch lookups; frontend device save, picker, `lib/time.ts` from `/auth/me`. | `backend/db/migrations/0002_account_users.up.sql` `timezone DEFAULT 'UTC'`, no `timezone_manual`; `module.go` mounts no `PATCH /auth/me`; `handler/auth.go` `Me` returns no zone; `api/api.go` `UserSummary` is `{ID, Email, DisplayName}`; `backend/cmd/api/main.go` `handleServerTime` serves `APP_TIMEZONE`; `frontend/src/lib/problems.ts` lacks the slug. | **migration:** `000N_account_user_timezone` (§6). **backend:** `PATCH /auth/me` handler + `UpdateUserTimezone` query (`updated_at = now()`); `Me` reads the row (the token carries no zone); `UserSummary.Timezone` + `GetUserTimezones(ctx, ids)`. **openapi:** both operations, `CurrentUser.timezone`, `timezone_manual`, the 422. **frontend:** slug, post-sign-in save (skipped while manual), settings picker, `lib/time.ts`. Then its readers (README Per-user timezone list). **test:** TC-ACC-100…104. | Decision 2026-09-30 (Timezone); Decision 2026-10-01 (f); the specs README Per-user timezone cross-cutting gap, item 1 |
| 15 | P0.6 `/auth/me` contract | The OpenAPI response matches the handler: `{id, email, display_name, roles, permissions}` (+ P0.13 fields). | `shared/openapi.yaml` `CurrentUser` is `allOf: [User, …]`, and `User` requires camelCase `displayName` (plus `avatarUrl`); `handler/auth.go` `Me` sends `display_name` and no avatar. `frontend/src/lib/session.ts` declares its own `Session` type, so nothing fails — the generated `types.gen.ts` type is simply wrong. | **openapi:** give `CurrentUser` its own snake_case properties (`display_name`, `roles`, `permissions`, then `timezone`, `timezone_manual`); keep `User` only if something else uses it. **frontend:** `session.ts` may then import the generated type. **test:** TC-ACC-040 (handler body keys). | Found while writing SPEC-01, 2026-10-01 |
| 16 | §7 Problem types registered | Every slug the module emits is in `ProblemType` and `PROBLEM_MESSAGES`. | `frontend/src/lib/problems.ts` registers 17 account slugs (`grep -c '"account/' frontend/src/lib/problems.ts` counts each twice: type and message) but not `account/invalid-credentials`, `account/too-many-attempts`, `account/weak-password`, `account/validation` or `account/role-not-found`, all emitted by `handler/auth.go` / `handler/admin.go`; nor the targets `account/user-not-found` and `account/invalid-timezone`. (`account/invalid-reset-token` and `account/rate-limited` are SPEC-05 §11 row 14.) | **frontend:** add `invalid-credentials`, `too-many-attempts`, `validation`, `role-not-found`, `user-not-found`, `invalid-timezone`; `weak-password` needs no entry once row 17 renames it to `password-policy`. **test:** TC-ACC-110 (a grep test listing every `problem(`/`writeError(` code against `problems.ts`). | README Errors convention; found while writing SPEC-01, 2026-10-01 |
| 17 | §7 statuses and slugs | Body/param-shape failures are 422 (`account/validation` or a named type); one slug per rule (`account/password-policy`); a missing user is 404 `account/user-not-found`; 429 carries `Retry-After`. | `handler/auth.go` `Register` answers 400 `account/invalid-email` and 400 `account/weak-password`; `handler/admin.go` `badRequest`/`problem(…StatusBadRequest…)` answer 400 for `validation`, `unknown-role`, `unknown-permission`, `role-cycle`, and `admin_users.go` 400 for `invalid-email`, `password-policy`, `confirmation-mismatch`; `pathUUID` and `notFoundOrInternal` map a missing user to `server.ProblemType("account","not_found")` = 404 about:blank; `Login`'s 429 sets no `Retry-After`. | **backend:** switch those to 422; `weak_password` → `password-policy`; `not_found` → `user-not-found` (keep `role-not-found`); `Retry-After` = the remaining TTL of the tripped counter. **openapi:** the 422s and slugs per §7. **frontend:** none (`problemDisplayMessage` keys on `type`, not status). **test:** TC-ACC-111. | README Pagination/Errors conventions; found while writing SPEC-01, 2026-10-01 |
| 18 | P0.8 `{items}` envelope | `GET /admin/users` answers `{items, total, limit, offset, counts}`. | `handler/admin.go` `ListUsers` writes `users`; `shared/openapi.yaml` `AdminUserPage` requires `users`; `frontend/src/lib/admin.ts` `listUsers` reads `r.users`. | **backend · openapi · frontend:** rename to `items` in one PR (README Envelopes retrofit). **test:** TC-ACC-050. | Decision 2026-09-30 (Envelopes); Decision 2026-10-01 (g) |
| 19 | P0.14 `ListDirectory` limit | Out-of-range `limit` clamps (≤ 0 → 50; > 200 → 200). | `api/api.go` `ListDirectory`: `if limit <= 0 \|\| limit > 200 { limit = 50 }` — resets. | **backend:** clamp. **test:** TC-ACC-091. | Decision 2026-10-01 (e) (limit) |
| 20 | §7 OpenAPI encoding and responses; §8 events.md | Public ops `security: []`; others `security: [{bearerAuth: []}]` + `x-required-permission`; every status in §7 declared. events.md lists `account:purge_refresh_tokens` (and the planned reset-token purge, scheduled per P0.15). | `shared/openapi.yaml`: no top-level `security`; only `/auth/logout`, `/auth/logout-all`, `/auth/me` declare `bearerAuth`, so every `/admin/*` operation reads as public; no `x-required-permission`; `reject`, `revoke-approval`, `disable` lack 409 `account/last-approver`; `logout` lacks 401; `/admin/roles/{id}/permissions` and role PATCH/DELETE 404s are not typed. `docs/reference/events.md` has no account row; `backend/cmd/worker/main.go` schedules `account:purge_refresh_tokens` but nothing schedules `PurgeExpiredPasswordResetTokens`. | **openapi:** annotate all account operations per §7. **backend:** register the reset-token purge (light server, `@every 24h`). **docs:** two events.md task rows. **test:** TC-ACC-095, TC-ACC-112. | README AuthZ OpenAPI encoding; README Events DoD; found while writing SPEC-01, 2026-10-01 |
| 21 | §7 `/admin` route gate | Every `(app)` route group is in `config.matcher` (D-34 edge gate). | `frontend/src/middleware.ts` matcher lists `/`, `/login`, `/register`, `/upload`, `/library/:path*`, `/bank/:path*`, `/people/:path*` — not `/admin/:path*` (also missing `/calendar`, `/weather`; backlog #15). A signed-out visitor gets the admin shell, which then 401s. | **frontend:** add `'/admin/:path*'` (or match the whole group). **test:** TC-ACC-080. | README Frontend convention; backlog #15 |
| 22 | P0.7 client grammar | `frontend/src/lib/session.ts` `can()` decides exactly as `rbac.Permission.Matches`. | `can()` treats `res:*` like any 2-segment grant (satisfies bare/`:any`, never `:own`); the server's `Matches` lets an action wildcard satisfy every scope, `:own` included — a `movies:*` holder is hidden affordances the API would allow. | **frontend:** if `g[1] === "*"` return true after the resource match. **test:** TC-ACC-081 (vitest table mirroring `TestMatches`). | Found while writing SPEC-01, 2026-10-01 |
| 23 | P0.8 `q` is literal | `q` matches as a literal substring. | `query/admin.sql` `ListUsersAdmin` / `CountUsersAdmin` concatenate `'%' \|\| q \|\| '%'` into `ILIKE` without escaping `%`, `_` or `\`. | **query:** escape in the handler (or `ILIKE … ESCAPE '\'` with an escaped argument). **test:** TC-ACC-051. | Found while writing SPEC-01, 2026-10-01 |
| 24 | P1.3 admin-change events; P0.14 `SuperadminIDs` | Seven `account:*` events published after commit with the `accountapi.AdminEvent` payload; `accountapi.SuperadminIDs`; each event's consumer edge registered in `cmd/api`; events.md rows. | Not built. `api/api.go` declares no event constant, payload or `SuperadminIDs`; `account.Deps` has no publisher; the handlers (`Register`, `decide`, `SetDisabled`, `DeleteUser`, `SetUserRoles`, `CreateRole`, `UpdateRole`, `DeleteRole`, `SetRolePermissions`, `HandleRefresh`) write audit rows only; `backend/cmd/api/main.go` builds `mediaEvents` after `account.New` and subscribes no `account:*` name. | **backend:** constants + `AdminEvent` in `account/api`; `Deps.Events`; publish after each write per P1.3; a `ListSuperadminIDs` query (recursive role walk, enabled + approved, limit 50) behind `SuperadminIDs`, and the same query for `cmd/worker`'s notify port; in `cmd/api`, build the publisher before `account.New` and add the seven `Subscribe` edges (the notify tasks are SPEC-05 §11 row 23). **docs:** events.md rows (planned → live in the same PR). **test:** TC-ACC-120…126. | Decision 2026-10-01b (D3) |

**Already matching on HEAD.**
- Argon2id `m=65536,t=3,p=2` PHC hashing with constant-time verify; HS256-only
  verification with `kid` rotation; refresh tokens 256-bit, SHA-256 at rest,
  unique hash index; three cookies with the P0.3 flags and the `remember`
  split; cookie Domain chosen per request host and mirrored on clear.
- The approval gate on all three channels: login (after the password, counters
  cleared), every authenticated request (403 `account/account-not-approved`),
  and every refresh (401, cookies cleared); `SetUserApproval` bumps
  `token_version` unconditionally; rejected rows are kept.
- First-run founder bootstrap and `BOOTSTRAP_SUPERADMIN_EMAIL` writing only
  what is missing (apart from row 7); `0031` backfilled existing users to
  `approved`; `users:approve` granted to no role.
- Approver fan-out: SQL prefilter + `rbac.Set.AllowsCode`, enabled and approved
  holders only, capped at 50, the registrant skipped, dedup key per
  registration, best-effort.
- Refresh reuse detection revokes the whole chain forward and backward and
  audits `account.refresh.reuse_detected` (apart from row 2's race).
- RBAC grammar, matching and fail-closed parsing per P0.7; hierarchy walk is
  cycle-safe in SQL and in `effectiveRolePermissions`/`wouldCycle`; the cache
  key rolls with `token_version`; role-grant changes re-key holders of
  descendant roles.
- Admin guards: no self-target on roles, approval, disable and delete; the
  escalation check on role assignment (symmetric difference, `superadmin` only
  by `*`), role grants (both directions), user edit and delete; create cannot
  bypass approval; delete needs `confirm_email`; last-approver on delete,
  disable, reject and revoke; system roles protected in the handler and in SQL.
- `GET /admin/users` clamps `limit` (`server.Limit(r, 25, 100)`), orders by
  `(created_at DESC, id DESC)`, and returns per-status `counts`.
- Every error is an RFC 7807 Problem via `platform/server` (`writeError`,
  `writeJSONError`, `problem` all delegate to `server.Problem`).
- The account surface runs under plain `RequireAuth` (global tables, no tenant
  scope); depguard keeps `account/rbac` private.

**Test evidence to add or fix.** `find backend/internal/modules/account -name '*_test.go'`
lists the module's suites: `auth/password_test.go`, `auth/reset_test.go`,
`handler/admin_test.go`, `handler/password_reset_test.go`,
`rbac/permission_test.go`. Nothing tests:
- `Login` (throttle, generic 401, disabled, pending/rejected, counter clearing)
  — TC-ACC-001…007;
- `Register` (founder, pending, fan-out wiring, 409) — TC-ACC-010…016
  (`approverIDs` alone is covered by the `TestApproverIDs*` tests);
- `RefreshManager.Rotate` and `HandleRefresh` (rotation, reuse burns the
  chain, approval re-check, `remember` preserved) — TC-ACC-030…034;
- `RequireAuth` (token_version mismatch 401, unapproved 403, disabled 401) and
  `RequirePermission` / `RequireOwnerOrPermission` status mapping —
  TC-ACC-040…045;
- `Logout` / `LogoutAll` / `Me` — TC-ACC-046…048;
- `CachedLoader` (key rolls with `token_version`) and the
  `GetEffectivePermissions` / `ListUserIDsAffectedByRole` CTEs against a real
  database — TC-ACC-060…062;
- `SetDisabled`, `decide`'s last-approver path, `CreateRole`, `UpdateRole`,
  `DeleteRole` — TC-ACC-055…057, TC-ACC-070…075;
- `bootstrapSuperadmin` — TC-ACC-020;
- no HTTP-level test exists for the account handlers at all (the CC-1/CC-3
  pair over `httptest`, as the other modules have).

## 12. Out of scope

- Password reset behaviour (SPEC-05 P0.3) and its UI.
- MFA/TOTP, step-up, Login with Google, session devices UI beyond P1.1.
- Policy bundles, user groups, file-gated permissions (ADR-02 later phase).
- Tenant membership, organisation switching, per-tenant roles (ADR-07 steps
  5–7) — owned by the `tenant` module.
- Rich profile fields ([D-19] → social).
- The asynqmon queue console at `/admin/queues` (SPEC-03 P1.6) and the layout
  editor at `/admin/layout` (the `layout` module) — they share the `/admin`
  URL space, not this module.
