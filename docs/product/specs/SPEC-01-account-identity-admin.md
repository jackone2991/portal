# SPEC-01 — Account: identity, approval gate, RBAC and the admin console

**Status:** current, rev 4 · **Drafted:** 2026-10-01 · **Last verified:** 2026-10-02
**Module:** `account` · **Depends on:** SPEC-05 (`notify:dispatch` carries the approval notification, the password-reset email and the account's other transactional mails — registration receipts, P0.1, and email-change links, P0.10; reset behaviour is SPEC-05 P0.3; the `notify:on_*` consumers of P1.3's events are SPEC-05 P1.5) · SPEC-04 P0.7 (`mediaapi.PurgeOwnerAssets`, media's purge in the user delete, P0.10) · every module that owns per-User data (`PurgeOwnerData`, P0.10) · `platform/audit` ([D-25]; identity retention, P0.16) · `platform/events` (P1.3) · `platform/server` (Problem writer, `Limit`)
**Upstream:** none — an **as-built spec written retroactively** from the code on `main` @ `99b5a0b` (the docs commits on top of it change no code). The decisions it records were taken earlier and are cited, not re-decided; the three architecture decisions are kept in full under [Decision records](#decision-records): [ADR-02](#adr-02) (role hierarchy is canonical for v1), [ADR-06](#adr-06) (local password auth; Portal owns credentials), [ADR-07](#adr-07) (account tables are global, not tenant-scoped), migration `0031_account_user_approval` (registration requires approval), feature-inventory `D-17` (per-user timezone, owner decisions 2026-09-30 and 2026-10-02 (A8)), `D-25` (audit taxonomy), `D-26.r1` (Portal-managed roles only), `D-34.r1` (SessionKeeper auth handoff). Rev 4 writes in the twelve owner decisions of 2026-10-02 (A1–A12, listed in the specs README "Decisions recorded 2026-10-02"); where they change shipped behaviour the text below is the target and §11 rows 25–31 carry the change
**Vocabulary:** [CONTEXT.md](../../../CONTEXT.md) — a person's identity is a **User** (never "account": that word is reserved for a ledger Account; the module and code names stay `account`), one signed-in browser is a **Session**, and **Approval**, **Rejected**, **Disabled**, **Superadmin** and **Approver** mean exactly what the glossary says
**Tenancy:** the `tenant` module has no spec of its own; its decision record, [ADR-07](#adr-07) (multi-tenancy and RLS), lives in this file, and the mechanism as built is [security.md §3](../../architecture/security.md#3-tenant-layer-data-segregation).
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
   `PATCH /auth/me`, `accountapi`) owned by a spec, so the readers in
   SPEC-12/07/09/11/13 have a dependency they can point at.
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
- **MFA / TOTP / step-up** ([D-27]/[D-28]) and **Login with Google** ([ADR-06](#adr-06)
  action items 10–11) — deferred; see [backlog.md § Deferred](../backlog.md).
- **Tenant-scoped roles, `switch-tenant`, `/admin/organizations`** (ADR-07
  steps 5–7) — deferred at one user with one personal org.
- **Rich profile** (bio, cover, education): [D-19] puts it in `social.profiles`,
  not on `users`.
- **An audit-log reader UI.** Writing audit rows is in scope (P0.11); reading
  them is not.

## 4. User stories

- As the person installing Portal, I put my email and a first password in
  `BOOTSTRAP_SUPERADMIN_EMAIL` / `BOOTSTRAP_SUPERADMIN_PASSWORD` before the
  first start; the API creates my User as an approved Superadmin, and on my
  first sign-in I must choose a new password before anything else works.
  Nobody who registers through the form is ever let in without an Approver.
  *(first-run bootstrap, P0.12 — Decision 2026-10-02 (A2); unbuilt)*
- As a second person on the instance, I register, am told — on screen and by
  email — that I am waiting for approval, and can sign in the moment an
  Approver approves me. My data is my own: a second Approved User shares
  nothing with the first (shared household data is not designed).
  *(Decision 2026-10-02 (A1, A12))*
- As someone who already has a User and forgot it, registering again tells me
  nothing on screen that a stranger could not also see, and my inbox explains
  what state my User is in. *(P0.1 — Decision 2026-10-02 (A1); unbuilt)*
- As the owner, I get a bell entry and an email when someone registers, and
  approve or reject them from `/admin/users` with an optional note the person
  sees on their next login attempt.
- As the owner, I revoke an approval or disable a User and that person's open
  browser stops working on its next request, not five minutes later.
- As an admin who is not a Superadmin, I can manage ordinary Users and roles
  but cannot hand myself — or anyone — a permission I do not hold, and cannot
  take over or lock out a User that outranks me.
- As a Superadmin, I get a bell entry when another admin approves, disables,
  deletes or re-roles a User or changes a role, and an email when a stolen
  refresh token is replayed. *(P1.3 — decided, unbuilt)*
- As the owner, I delete a User and everything they owned — rows, uploaded
  files, cache entries — is gone, irreversibly; only a short identity snapshot
  survives for 90 days. *(P0.10 — Decisions 2026-10-01b (D1) and 2026-10-02
  (A6); unbuilt)*
- As a signed-in User, logging out of this browser leaves my other Sessions
  working; "log out everywhere" ends them all. *(P0.6 — Decision 2026-10-02
  (A3); unbuilt)*
- As any signed-in User, my day boundaries ("today", "this month", birthday
  countdowns) follow the timezone saved on my User — set from my device the
  first time I sign in, and changed only when I confirm it.
  *(P0.13 — Decision 2026-10-02 (A8); unbuilt)*
- As an admin correcting someone's email, the change waits until the new
  address proves it can receive mail, and the old address can cancel it.
  *(P0.10 — Decision 2026-10-02 (A9); unbuilt)*
- Edge: an operator upgrading an install that predates `0031` names an
  existing User in `BOOTSTRAP_SUPERADMIN_EMAIL` and it is (re-)asserted as an
  approved, enabled Superadmin on every start; its password is never touched.

## 5. Requirements

Status codes below are the **target**; where `HEAD` differs, the requirement
says so in a *(code follow-up)* note and §11 carries the row. "about:blank"
means a bare-status Problem (specs README Errors convention).

### P0.1 — Registration (`POST /api/v1/auth/register`, public)

Body `{email, password, display_name?}` (64 KiB cap, `handler/auth.go`
`decodeJSON`). The email is trimmed and lower-cased; it is valid when it has a
non-leading `@`, a `.` after it and ≤ 320 characters (`validEmail`) — else 422
`account/invalid-email`. The password is ≥ 8 bytes (`minPasswordLen`) — else 422
`account/password-policy`. An unparseable body is 400 about:blank. These shape
checks run before anything else and are the only answers that differ from the
success response. `display_name` defaults to the email's local part.

*(Code follow-up: `HEAD` answers the email and password checks with 400
`account/invalid-email` and 400 `account/weak-password` — a second slug for the
rule P0.10 calls `account/password-policy` — §11 row 17.)*

**One answer for every valid request** *(Decision 2026-10-02 (A1); closes the
former §10 Q3; unbuilt — §11 row 25)*. A well-formed registration always
answers **201 `{status: "registered"}`** and issues no Session — no
`approval_status`, no echo of the email, and never 409 `account/email-taken`
(that slug survives only on admin create and edit, P0.10). The password is
hashed with Argon2id (P0.3) **on both paths**, before the email is looked up,
so the response time does not reveal which path ran; every other difference
between the paths (the insert, the audit row, the notification enqueues) is
best-effort work whose cost must stay well under the hash's, and the handler
must not branch on it before answering.

- **New email.** The User is created **Pending** (the `0031` column default),
  the `user` role is seeded best-effort (a failure is recorded in the audit
  metadata as `role_seed_error`, never fails the request), and
  `account.user.registered` is audited. Two notifications follow, both
  best-effort: the registrant gets one email, type
  `account.registration_received` ("registered, awaiting approval"), and every
  Approver gets the fan-out below. There is no first-registrant exception:
  every registration is Pending, and the instance's first Superadmin comes from
  P0.12 (Decision 2026-10-02 (A2)).
- **Existing email** (any Approval state). Nothing is written —
  no row, no role, no audit entry — and no Approver is notified. The existing
  User gets **one** email, type `account.registration_repeat`, whose text
  depends on their state: **Pending** → "your registration is waiting for
  approval"; **Approved** → "you already have a User here — sign in, or reset
  your password"; **Rejected** → a generic message that offers no way to
  reapply (a Rejected User cannot register again; only an admin moves them back
  to Pending or Approved — P0.9). At most one such email per address per 24 h
  (Redis `SET NX EX 86400` on `register:notice:<sha256(email)>`; without Redis
  the email is skipped, never sent unthrottled), so the form cannot be used to
  flood someone's inbox. A Disabled User gets no email (notify resolves a
  Disabled User to no address — SPEC-05 P0.3); the answer is the same 201.

Both registrant emails go through notify's email channel (SPEC-05 P0.3):
`notify:dispatch` with `channels: ["email"]`, not persisted in-app, the
recipient named by `user_id` and resolved at send time. Because they are
addressed to a User who is not (yet, or ever) Approved, the worker's recipient
resolution for these two types reads the row whatever its Approval state —
P0.14's approved-only rule governs `accountapi.GetUserByID`, not this resolver
(§11 row 12). With no SMTP configured (`SMTP_HOST` empty, `platform/config`)
the worker's log-sink sender swallows them and registration still succeeds:
these emails are a courtesy, never a gate.

**Approver fan-out** (`handler/auth.go` `notifyApprovers`; new-email path only).
The API resolves the Users whose *effective* permissions satisfy
`users:approve` — the Approvers: SQL (`ListPermissionHoldersByResource`) walks
the role hierarchy for enabled, approved Users and prefilters to codes whose
resource segment is `users` or `*`; `rbac.Set.AllowsCode` then applies the real
grammar (a Superadmin's set is the literal `*`). It enqueues one
`notify:dispatch` per Approver (not the registrant) through `notifyapi`: type
`account.registration_pending`, channels `in_app` + `email`, `data {user_id,
email, display_name, href}` (`href` = `APPROVAL_QUEUE_URL`), `dedup_key =
"registration:<user_id>"`. At most **50** Approvers (`maxApproverFanout`;
truncation is logged). No Approver is a logged warning. Every failure is logged
and swallowed — registration never fails on notification. RBAC is not
tenant-scoped, so the fan-out is global.

**Abuse control.** Registration is throttled per client IP *(code follow-up:
`HEAD` applies no limiter to `/auth/register` or `/auth/refresh` — the
`platform/middleware` `IPRateLimiter` exists but no binary wires it, and the
Traefik `rate-limit@file` middleware is attached to no router — so each
anonymous POST can create a pending row and up to 50 email dispatches; §11
row 3)*.

*(Code follow-up for the whole of A1 — `HEAD` answers 409 on a taken email,
returns `{status, email, approval_status}`, sends the registrant nothing, and
the frontend's `AuthForm.tsx` reads `approval_status` to choose its message —
§11 row 25; the first-registrant rule `HEAD` still runs is §11 row 26.)*

**Acceptance criteria.**
- Given any `users` table, empty or not, when A registers a new email, then 201
  `{status: "registered"}` with no other field, A is Pending and holds only
  `user`, A gets one `account.registration_received` email, and every Approver
  gets one bell row and one email; a redelivered dispatch adds no second bell
  row. *(TC-ACC-130)*
- Given an existing Approved User B, when someone registers B's email (in any
  case), then 201 `{status: "registered"}`, no row or audit entry is written,
  no Approver is notified, and B gets one `account.registration_repeat` email
  in its Approved wording; a second attempt within 24 h sends nothing more.
  *(TC-ACC-131, TC-ACC-132)*
- Given a Pending and a Rejected User, when their emails are registered again,
  then each gets the email for its own state, and the Rejected one offers no
  way to reapply. *(TC-ACC-133)*
- Given the new-email and existing-email paths timed over many runs, then their
  response-time distributions overlap (both run one Argon2id hash).
  *(TC-ACC-134)*
- Given no SMTP configured, then registration still answers 201 and the User is
  created. *(TC-ACC-130)*
- Given a password of 7 bytes, then 422 `account/password-policy` and no row.
- Given 50 rapid registrations from one IP, then later ones answer 429
  `platform/rate-limited` (P0.1 abuse control).

### P0.2 — Login (`POST /api/v1/auth/login`, public)

Body `{email, password, remember?}`. Missing email or password, or an
unparseable body: 400 about:blank. Order of checks (`handler/auth.go` `Login`):

1. **Throttle** *(Decision 2026-10-02 (A4); closes the former §10 Q5)*. Two
   counters, each capped at 5 failures (`loginMaxFailures`) within a 15-minute
   window (`loginFailWindow`): the **global per-IP** counter
   (`login:fail:ip:<ip>`, every email from that address) and a **per-(email,
   client IP)** counter (`login:fail:email_ip:<email>:<ip>`). When either has
   reached the cap, answer 429 `account/too-many-attempts` with `Retry-After`
   *(code follow-up: no `Retry-After` on `HEAD` — §11 row 17)*. There is no
   per-email counter shared across addresses, so a stranger guessing from
   another IP can no longer lock a User out of login; the price is that a
   guesser with many addresses gets 5 tries per address, which the per-IP cap
   and Argon2id's cost bound. This depends on a trustworthy client IP (§11
   row 4): while any peer can choose its `X-Forwarded-For`, both keys are
   attacker-chosen. A nil Redis client disables the throttle. *(Code
   follow-up: `HEAD` keys the second counter on the email alone,
   `login:fail:email:<email>` in `loginFailKeys`, so anyone can lock any known
   email out for 15 minutes — §11 row 28.)*
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
   when there is one. Checked after the password, so an unapproved User
   cannot be used to probe emails. Both counters are cleared first (honest
   retries while waiting must not lock the User out) and
   `account.session.pending_attempt` is audited.
5. **Success:** counters cleared; session issued (P0.3); `account.session.login`
   audited; 200 `{access_token, expires_in, token_type: "Bearer", user: {id,
   email, display_name, roles}}`.

The client IP is the leftmost `X-Forwarded-For` entry, else the peer address
(`handler/util.go` `clientIP`). It must only honour that header from the
trusted proxy *(code follow-up — §11 row 4)*.

**Acceptance criteria.**
- Given 5 wrong passwords for one email from one IP, then the 6th attempt from
  that IP — even with the right password — is 429 until the window lapses.
  *(TC-ACC-144)*
- Given that lockout, when the User signs in with the right password from
  another IP, then 200; given 5 failures from one IP spread over five
  different emails, then that IP's next attempt on any email is 429.
  *(TC-ACC-145)*
- Given a Pending User with the right password, then 403
  `account/account-pending` and no cookie is set; with a wrong password, 401.
- Given a Rejected User with note "dup", then 403 `account/account-rejected`
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

**Password change required** *(Decision 2026-10-02 (A2); unbuilt — §11
row 26)*. The snapshot also carries `users.password_must_change` (§6). While it
is true, every authenticated route answers **403
`account/password-change-required`** except `GET /auth/me`, `POST
/auth/logout`, `POST /auth/logout-all` and `POST /auth/password` (P0.12) — the
four a User needs to see why, leave, or comply. `/auth/refresh` is public and
unaffected, so the Session survives while the User chooses a password. The
check sits in `RequireAuth` beside the approval check, so it cannot be
forgotten by a route; the four exempt routes are named in one allow-list there.

The account surface itself (`/auth/*`, `/admin/*`) runs under plain
`RequireAuth`, never `RequireTenant`: it touches only global tables (§6).

### P0.6 — Logout, logout-all, me (authenticated)

- `POST /auth/logout` → 204 — **ends one Session** *(Decision 2026-10-02 (A3);
  closes the former §10 Q4; unbuilt — §11 row 27)*. It requires the refresh
  token, from the `portal_refresh` cookie or a JSON body `{refresh_token}`;
  without one it answers **422 `account/validation`** whose `detail` says to
  use `POST /auth/logout-all` to end every Session. With one, it revokes that
  token's whole chain (reason `logout`, `RevokeRefreshTokenChain`) when the
  token belongs to the caller — an unknown, expired, already-revoked or
  foreign token revokes nothing and still answers 204 — audits
  `account.session.logout` (`scope: session`) and clears the cookies. It does
  **not** bump `token_version`: the User's other Sessions keep working and the
  RBAC cache is not re-keyed. The browser that logged out loses its cookies;
  an access token copied out of it before logout stays valid until it
  expires, at most `ACCESS_TOKEN_TTL` (5 minutes by default) — the accepted
  cost of not ending every other Session. The frontend refreshes first so an
  expired access token does not 401 the logout (`TopMenu.tsx` `logout`).
  *(Code follow-up: `HEAD` revokes only the presented token, ignores a missing
  one, and bumps `token_version`, so logging out of one browser stops every
  device's access token until it refreshes — §11 row 27.)*
- `POST /auth/logout-all` → 204 — ends **every** Session. Revokes every
  refresh token of the User (reason `logout_all`), bumps `token_version` (every
  access token everywhere stops on its next request), audits with `scope:
  all_sessions`, clears cookies.
- `GET /auth/me` → 200 `{id, email, display_name, roles, permissions}` —
  `roles` from the token, `permissions` the caller's **effective** codes from
  the engine (wildcards literal; best-effort: `[]` if the cache/DB read fails).
  P0.13 adds `timezone` (`string | null`) and P0.12 adds
  `password_must_change` (boolean); both are read from the `users` row, since
  the token carries neither. *(Code follow-up: the OpenAPI `CurrentUser`
  schema declares camelCase `displayName` and `avatarUrl`, which the handler
  never sends — §11 row 15.)*

**Acceptance criteria (logout).**
- Given Sessions S1 and S2 of one User, when S1 logs out, then S1's refresh
  chain is revoked and its cookies cleared, S2's next request and next refresh
  both succeed, and `token_version` is unchanged. *(TC-ACC-141)*
- Given a logout with no refresh token, then 422 `account/validation` naming
  logout-all, and nothing is revoked. *(TC-ACC-142)*
- Given logout-all from S1, then S2's next request is 401 and its next refresh
  is 401. *(TC-ACC-143)*

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
and does not escape `%`/`_` in `q` — §11 rows 17, 18, 23.)*

**Offset paging is a named exception** *(Decision 2026-10-02 (A5); closes the
former §10 Q2)*. The README Pagination convention keyset-pages lists; this
admin-only operator grid keeps `limit` + `offset` with a `total` and the
per-status `counts`, because an approval queue is read as "how many are
waiting, and page 3 of them", not as an endless feed, and it holds at most a
household's worth of rows. The exception is recorded in the convention itself
and covers this endpoint only; it is not a template for new lists.

### P0.9 — Approval decisions

`POST /admin/users/{id}/approve | reject | revoke-approval`, `users:approve`.
Optional body `{note}` (trimmed, ≤ 500 bytes, else 422 `account/validation`).

What each decision means *(Decision 2026-10-02 (A10); the glossary terms of
[CONTEXT.md](../../../CONTEXT.md))*. Approval is the one-time admission
decision: **approve** admits a Pending (or Rejected) User; **reject** refuses
admission — a Rejected User cannot register again with the same email (P0.1),
and only an admin moves them back to Pending (`revoke-approval`) or to
Approved (`approve`); **revoke approval** sends an Approved User back to
Pending for a fresh review and is meant to be rare. A temporary, reversible
suspension of an Approved User is not an Approval decision at all — it is
**Disable** (P0.10), and a Disabled User is still Approved. All of this is
shipped; A10 fixes the words, not the behaviour.

Guards, in order:

1. **No self-target:** acting on yourself → 403 `account/self-target`.
2. **No acting on a User who outranks you** (same rule as P0.10's escalation
   guard) → 403 `account/escalation` *(code follow-up: `decide` has no
   target-authority check — §11 row 6)*.
3. **Last Approver:** `reject` and `revoke-approval` refuse when the target is
   the only enabled, Approved User whose effective set satisfies
   `users:approve` → 409 `account/last-approver`.

Effect (`SetUserApproval`): `approval_status`, `approval_note`,
`approved_at` (`now()`, or NULL for `pending`), `approved_by` (the actor, NULL
for `pending`), and an unconditional `token_version` bump — a Rejected or
revoked User's live Session dies on its next request. Audited as
`account.user.approved | rejected | approval_revoked`. 200 `AdminUser`. A
Rejected row is kept so the email cannot re-register; to refuse someone
permanently, keep them Rejected rather than deleting them (P0.10 — a delete
frees the email).

### P0.10 — User create, edit, disable, delete

- **Create** `POST /admin/users` (`users:write:any`) `{email, password,
  display_name?}`: email and password rules as P0.1 (422
  `account/invalid-email`, 422 `account/password-policy`; a password is
  required). **The approval state comes from the creator's authority, never the
  body**: `approved` (with `approved_by` = actor) iff the actor's effective set
  allows `users:approve`, else `pending`. Roles are not settable; `user` is
  seeded best-effort. 409 `account/email-taken` (admin create and edit are the
  only places that slug survives — P0.1). Audit `account.user.created`.
  201 `AdminUser`.
- **Edit** `PATCH /admin/users/{id}` (`users:write:any`) `{email?,
  display_name?, password?}` — omitted or empty fields keep their value. The
  password is validated before anything is written. Editing **yourself** is
  allowed; editing another User whose roles carry any permission you lack,
  or the `superadmin` role without your holding `*`, is 403
  `account/escalation`. A new password also bumps `token_version` and revokes
  every refresh token (reason `admin_password_change`). Audit
  `account.user.updated` (`email_before`, `email_after`, `password_changed`).
  A **changed email is not written** — it starts a verified email change
  (below), and the response's `AdminUser` carries the request as
  `pending_email` (`string | null`).
- **Disable / enable** `POST /admin/users/{id}/disable | enable`
  (`users:write:any`): self → 403 `account/self-target`; the target-authority
  check of Edit → 403 `account/escalation` *(code follow-up: missing on `HEAD`,
  so an `admin` can switch off a Superadmin — §11 row 6)*; disabling the last
  Approver → 409 `account/last-approver`. Disable is the temporary, reversible
  suspension of an Approved User (Decision 2026-10-02 (A10)): it stamps
  `disabled_at` and bumps `token_version` (`DisableUser`); enable clears it.
  Audit `account.user.disabled | enabled`. 200 `AdminUser`.
- **Delete** `DELETE /admin/users/{id}` (`users:delete:any`) `{confirm_email}`:
  self → 403 `account/self-target`; `confirm_email` must equal the target's
  email case-insensitively, else 422 `account/confirmation-mismatch` (server-side,
  so a replayed request cannot reach the delete) *(code follow-up: `HEAD`
  answers 400 — §11 row 17)*; escalation → 403; last Approver → 409. Audit
  `account.user.deleted`. 204; a second delete is 404. What runs after the
  guards is the purge order below; the database cascade (`ON DELETE CASCADE`
  on every ownership FK; `SET NULL` on the audit, `granted_by`, `approved_by`
  and `people_persons.linked_user_id` references) is only the safety net
  under it.

**Email change requires verification** *(Decision 2026-10-02 (A9); unbuilt —
§11 row 30)*. The email is the login identifier, so it changes only once the
new address has proved it can receive mail, and the old address can stop it.
An Edit carrying a different email (another User's, or your own through this
route):

1. Re-validates the address (422 `account/invalid-email`) and refuses one
   already held by another User (409 `account/email-taken`).
2. Refuses with **503 `account/mail-unavailable`** when no SMTP transport is
   configured (`SMTP_HOST` empty, `platform/config`): without mail the change
   could never complete, so nothing at all is written — not the other fields
   of the same PATCH either.
3. Otherwise keeps the **old email active** and stores one pending change in
   `email_change_requests` (§6) — at most one open request per User; a new one
   replaces the old — then sends two `notify:dispatch` emails (`channels:
   ["email"]`, not persisted in-app): `account.email_change_confirm` to the
   **new** address, with a one-time confirm link valid **1 hour**, and
   `account.email_change_alert` to the **old** address, saying a change was
   requested and carrying a one-time "this wasn't me" cancel link. Both links
   and the new address are minted at send time from the request id
   (`accountapi.MintEmailChangeLinks`), never carried in a task payload — the
   rule SPEC-05 §11 row 1 sets for the reset token. Audit
   `account.user.email_change_requested`.
4. **Resend** `POST /admin/users/{id}/email-change/resend` (`users:write:any`,
   same guards as Edit) re-sends both emails with fresh links, which
   **revokes the previous links**; at most once a minute and 5 times per
   request, else 429 `account/rate-limited` with `Retry-After`. **Cancel**
   `DELETE /admin/users/{id}/email-change` (`users:write:any`) → 204.
5. **Confirm** `POST /auth/email-change/confirm {token}` (public): an unknown,
   used, cancelled or **expired** token → 422
   `account/invalid-email-change-token`, and an expired one also cancels its
   pending change. A valid token re-checks uniqueness: if another User took
   the address meanwhile, the change is cancelled and the answer is 409
   `account/email-taken`. Otherwise `users.email` becomes the new address, the
   request is marked confirmed, `token_version` is bumped (the access token
   carries the email claim; every Session picks up the new one on its next
   refresh, and refresh tokens survive), audit `account.user.email_changed`
   (`email_before`, `email_after`) → 204.
6. **Cancel by the old address** `POST /auth/email-change/cancel {token}`
   (public) → 204 whether or not the token matched anything (no oracle); a
   match cancels the pending change and audits
   `account.user.email_change_cancelled`.

A User's own self-service change from settings is P1.4 — the same flow.

*Acceptance criteria (email change).*
- Given an admin changing B's email to `n@x`, then B's login email is still
  the old one, `n@x` gets one confirm email and the old address one alert, and
  `AdminUser.pending_email` is `n@x`. *(TC-ACC-155)*
- Given the confirm link used within the hour, then B signs in with `n@x` only;
  used again, then 422 `account/invalid-email-change-token`; after 1 hour,
  then 422 and the pending change is gone. *(TC-ACC-156)*
- Given another User registering `n@x` before B confirms, then the confirm
  answers 409 `account/email-taken` and the pending change is cancelled.
  *(TC-ACC-157)*
- Given a resend, then the first link is dead and the new one works; a sixth
  resend is 429. Given the old address's cancel link, then the confirm link is
  dead. *(TC-ACC-158)*
- Given no SMTP configured, then 503 `account/mail-unavailable` and the row —
  display name included — is unchanged. *(TC-ACC-159)*

**Deleting a User purges everything they own, module by module, then the
row** *(Decisions 2026-10-01b (D1) and 2026-10-02 (A6); unbuilt — §11 rows 8
and 29)*. Each module that owns per-User data implements one idempotent
method behind its `api/` package,

```go
PurgeOwnerData(ctx context.Context, userID uuid.UUID) (remaining int, err error)
```

which deletes that module's rows for the User **and its non-database side
effects** — stored objects, cache keys, staged uploads — and returns how many
of its rows are still left (a budgeted purge may need several calls). The
database cascade stays only as the safety net under it: it would delete rows,
but never an object in storage or a Redis key. Account calls a **registry** of
these, in order, wired by `cmd/api`:

1. **Content modules**, in a fixed order: `comic`, `music`, `movie`, `story`,
   `journal`, `bank`, `people`, `social`, `notify`. (Checked against
   `ls -d backend/internal/modules/*/`: `layout` owns only the instance-wide
   shell and `ops` only system rows — neither holds per-User data; `account`
   deletes its own rows in the final transaction; `tenant` is item 3.)
2. **`media` last among them** — its implementation is SPEC-04 P0.7's
   `mediaapi.PurgeOwnerAssets`, because every content module refers to Assets
   and lets go of them first.
3. **`tenant`** — the User's personal organisation and memberships, only in the
   locked final pass and only once everything above reported zero: every
   tenant-scoped row references `organizations(id)` with no `ON DELETE`
   action, and the media janitor reaches a half-purged User only through
   `forEachTenant`, which needs that organisation to exist.

After every guard above has passed:

1. **Disable** the target (`DisableUser`: stamps `disabled_at`, bumps
   `token_version`), so no new authenticated request of theirs — an upload, a
   zip import — can start. This step is part of the delete; it is not audited
   or announced on its own.
2. **Bulk pass.** Every registry entry in order (content modules, then media),
   each on its own connection and tenant scope.
3. **Locked final pass and delete**, in one account transaction:
   `SELECT 1 FROM users WHERE id = $1 FOR UPDATE` (new query
   `LockUserForDelete`); the whole registry again — tenant included — with a
   context that does not carry this transaction (each module opens its own
   scope); only when **every** entry returns `0`: write the identity snapshot
   to `deleted_users` (§6: user id, email, display name, role codes, the
   acting admin, `now()`), then `DeleteUser` (the account's own
   `refresh_tokens`, `password_reset_tokens`, `email_change_requests` and
   `user_roles` go with it by cascade); commit.
4. Any error, or rows left in any entry after either pass: **503
   `account/delete-incomplete`** with `Retry-After: 900` and no further
   change. The User row stays (Disabled); whatever is half-purged stays
   purgeable — media's tombstones are finished by the hourly
   `media:purge_orphans`, because the rows and the personal organisation still
   exist, and every other module's purge is idempotent. A retried DELETE (the
   UI may retry at once) continues where the last stopped and completes once
   nothing is left. The User stays Disabled until an admin enables it.

**No grace period.** A completed delete is irreversible: there is no
soft-delete and no restore. What survives is the `deleted_users` snapshot,
kept for **90 days** and then removed by the identity-retention sweep
(P0.16) — enough to answer "who was this, and who deleted them" during an
incident, nothing that would let the User be rebuilt.

**Deleting frees the email.** The deleted person may register again and
becomes a new Pending User with none of the old data. To refuse someone
permanently, keep them **Rejected** instead (P0.9); the admin UI says so in
the confirmation next to Delete ("Deleting frees this email — to keep this
person out, reject them instead"). **Restoring a backup** taken before the
delete brings the User and their data back; that is an operations exception,
not a feature ([backup-restore.md](../../operations/backup-restore.md) § 4).

Why this order is the one that works (Decision 2026-10-01b (D1)'s reasoning,
generalised from media to every module):

- **Rows are the only record of side effects.** Media's objects are found only
  through their `assets` rows (`purgeObjects` derives the prefixes from them;
  the janitor reads only `deleting` rows), and `assets.owner_id` cascades from
  `users` (`0007`); the same holds for any module whose rows point at objects
  or keys. Deleting the User first would lose those rows in the cascade and
  leak whatever they pointed at. The `users` row is therefore deleted only
  after every module reports **zero** rows: the cascade never removes a row
  whose side effects may still exist.
- **The race with a new write.** A per-User INSERT into a table with an FK to
  `users(id)` (the identity-anchor FK of the README Module boundaries
  convention) checks it by taking `FOR KEY SHARE` on the `users` row, which
  step 3's `FOR UPDATE` conflicts with. An insert committed before the lock is
  seen by the final pass and purged; one arriving after it waits, then fails
  its FK check when the delete commits, writing no row (and, through
  `mediaapi.Ingest`, no object — the row precedes the bytes). A row that
  reaches the User only through `tenant_id` is caught the other way: the
  organisation delete fails on its `NO ACTION` FK, so the delete answers 503
  and the retry purges it — the worst case is a retry, never an orphan.
  Disabling first limits the window to worker tasks already running for the
  target (a zip import, a comic sync).
- **No deadlock.** Each final-pass purge runs on its module's own connection
  and deletes or updates only that module's rows, none of which updates the FK
  column or locks `users`.
- **No event fan-out against a vanished tenant.** Owner purges publish no
  delete events (SPEC-04 P0.7 step 5 for media): every row that references
  the purged data is the target's own and goes in the same pass.
- Not covered: a browser `PUT` to an already-issued presigned URL that lands
  after its asset row was purged leaves an object nothing references — the
  same exposure `DELETE /assets/{id}` has for an `uploading` asset (SPEC-04
  P0.3), not something this order can close.

*Acceptance criteria (delete).*
- Given a User with uploaded video, image and audio assets, when they are
  deleted, then 204, no object remains under any of their prefixes, and no
  `assets` row of theirs remains. *(TC-ACC-064)*
- Given storage failing during the purge, then 503 `account/delete-incomplete`,
  the User still exists and is Disabled, and after the next janitor run a
  retried DELETE answers 204 with no object left. *(TC-ACC-065)*
- Given an `assets` INSERT for the target blocked behind step 3's lock, then
  after the delete commits that INSERT fails and no row or object of it
  remains. *(TC-ACC-066)*
- Given a User with rows in every content module, when they are deleted, then
  every registry entry was called in order (content, media, tenant), no row of
  theirs remains in any module, and no Redis key of theirs remains.
  *(TC-ACC-146)*
- Given one content module returning `remaining > 0`, then 503
  `account/delete-incomplete`, the User is Disabled and still exists, and a
  retry after that module drains answers 204. *(TC-ACC-147)*
- Given a completed delete, then one `deleted_users` row holds the User's id,
  email, display name, roles and the acting admin, and no `users` row exists;
  after 90 days the sweep has removed the snapshot. *(TC-ACC-148)*
- Given a deleted User's email, when it is registered again, then a new Pending
  User is created with none of the old data. *(TC-ACC-149)*

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

### P0.12 — Superadmin bootstrap

There is **no first-registrant rule**: every registration is Pending (P0.1),
and the instance's first Superadmin is created by the operator before the
first start *(Decision 2026-10-02 (A2); replaces the former founder rule of
P0.1; unbuilt — §11 row 26)*. On every `cmd/api` start (`bootstrapSuperadmin`)
the API reads two variables:

- `BOOTSTRAP_SUPERADMIN_EMAIL` (empty = off) — the Superadmin's email,
  trimmed and lower-cased.
- `BOOTSTRAP_SUPERADMIN_PASSWORD` *(new)* — that User's first password, read
  **only** when the User has to be created. It lives in the environment
  (`.env`), never in a migration or a seed file, and the operator may remove
  it once the User exists; it must satisfy the password policy (≥ 8 bytes).

Then:

1. **No User has that email** → create one: Approved (`approved_by` NULL —
   nobody approved it but the operator), enabled, holding `superadmin`,
   `display_name` = the email's local part, password = the variable's value
   (Argon2id, P0.3) and **`password_must_change = true`**. Audited as
   `account.user.created` with `actor_kind = 'system'`. A unique violation
   (another process created it first) is treated as "exists". If the password
   variable is empty or fails the policy, nothing is created and the API logs
   an error naming the variable — and still starts.
2. **The User exists** → the shipped re-assert: the User is made to hold
   `superadmin`, be `approved` and be **enabled**, writing only what is missing
   (so a correct install bumps nothing and keeps its permission cache); a role
   grant bumps `token_version`. The password is **never** touched on this path,
   whatever the variable says. *(Code follow-up: `HEAD` never clears
   `disabled_at` despite its own doc comment — §11 row 7.)*
3. **`BOOTSTRAP_SUPERADMIN_EMAIL` unset** → the API starts and logs a warning,
   "no superadmin configured", on every start. On a fresh install that means
   nobody can approve anybody until the operator sets the variables and
   restarts: registrations wait as Pending, and nothing deadlocks permanently.

**First sign-in must change the password.** A User with
`password_must_change` signs in normally (P0.2) and gets a Session, but every
authenticated route except `GET /auth/me`, `POST /auth/logout`,
`POST /auth/logout-all` and `POST /auth/password` answers 403
`account/password-change-required` (P0.5); `/auth/me` returns
`password_must_change: true`, and the frontend routes the User to a
change-password form before anything else. The flag exists so that a password
that sat in an `.env` file is never the one a Superadmin keeps.

**Self-service password change** — `POST /api/v1/auth/password
{current_password, new_password}` (authenticated; the caller's own User only;
usable whether or not the flag is set):

- `current_password` is verified with Argon2id (P0.3). A wrong one is 422
  `account/wrong-current-password` and counts as a failure on the caller's
  per-(email, client IP) login counter (P0.2); at the cap the answer is 429
  `account/too-many-attempts` — a stolen access token cannot be used to guess
  the password at leisure.
- `new_password` must satisfy the policy (422 `account/password-policy`) and
  differ from the current one (the same 422, with a `detail` saying so).
- On success, in one transaction: the new hash and `password_updated_at`,
  `password_must_change = false`, a `token_version` bump, and every refresh
  token of the User revoked (reason `password_change`) **except the current
  Session's chain** — the chain of the refresh token the request presents
  (`portal_refresh` is sent, since its Path is `/api/v1/auth`; an API client
  may put `refresh_token` in the body). The bump stops every other device's
  access token at once; the current Session survives because the response
  mints it a fresh access token at the new `token_version` and re-sets the
  cookies (200 `{access_token, expires_in, token_type}`). A request that
  presents no refresh token keeps no Session but its own new access token:
  every chain is revoked. Audited as `account.password.changed`.

*Acceptance criteria (bootstrap).*
- Given an empty database and both variables set, when the API starts, then
  one Approved, enabled User with that email holds `superadmin` and has
  `password_must_change = true`; a second start writes nothing.
  *(TC-ACC-135)*
- Given the User exists with another password, when the API starts with a
  different `BOOTSTRAP_SUPERADMIN_PASSWORD`, then the stored hash is unchanged.
  *(TC-ACC-136)*
- Given neither variable, then the API starts and logs "no superadmin
  configured"; given only the email on an empty database, then nothing is
  created and an error names the password variable. *(TC-ACC-137)*
- Given an empty `users` table, when someone registers, then they are Pending
  and hold no `superadmin`. *(TC-ACC-130)*
- Given a User with `password_must_change`, then `GET /auth/me` answers 200
  with the flag, `GET /admin/users` answers 403
  `account/password-change-required`, and after `POST /auth/password` with
  the right current password both answer normally. *(TC-ACC-138)*
- Given Sessions S1 and S2, when S1 changes the password, then S1 keeps
  working with the returned access token and its refresh cookie, and S2's next
  request and refresh are 401. *(TC-ACC-139)*
- Given 5 wrong `current_password` attempts from one IP, then the 6th is 429.
  *(TC-ACC-140)*

### P0.13 — Per-user timezone *(decided 2026-09-30; revised by Decision 2026-10-02 (A8); unbuilt)*

The specs README **Timezone** convention is binding and is not restated in
full; the account half is:

- `users.timezone` (IANA name) is **NULLable: NULL means "not set"**. Nothing
  has ever written the column, so every existing `'UTC'` row was never chosen
  by anyone and becomes NULL; the column has no default. There is no manual
  flag — Decision 2026-10-02 (A8) dropped `timezone_manual`, which superseded
  Decision 2026-10-01 (f).
- `PATCH /api/v1/auth/me {timezone}` — authenticated, the caller's own row
  only. `timezone` is required and validated with `time.LoadLocation`; empty
  or unknown → 422 `account/invalid-timezone`, nothing written. 200 = the
  updated `/auth/me` body.
- `GET /auth/me` returns `timezone` as the stored name, or `null` when not set.
- `accountapi.UserSummary.Timezone` (plus a batch lookup by user ids for
  sweeps) always returns a usable IANA name: the stored one, or
  **`Asia/Ho_Chi_Minh`** when the column is NULL or the stored name does not
  parse (the latter also logs a warning). Backend sweeps and readers therefore
  never see NULL. Modules never query `users` themselves.
- **Frontend.** After sign-in: when `/auth/me` says `timezone: null`, the app
  saves the device zone (`Intl.DateTimeFormat().resolvedOptions().timeZone`)
  with `PATCH /auth/me {timezone}` automatically, without asking. When a zone
  is set, it is applied everywhere and the device **never overwrites** it;
  when the device zone differs from it, the UI offers to switch — one prompt
  per differing device zone, remembered per browser — and saves only if the
  User confirms. Settings offer an IANA picker that saves `{timezone}`. Until a
  zone is stored (before the first save completes), the UI uses the device
  zone. `lib/time.ts` takes the display zone from `/auth/me`, and
  `GET /api/v1/time` keeps only the server clock.

*(Code follow-up: none of it exists — `0002_account_users` declares `timezone
TEXT NOT NULL DEFAULT 'UTC'`, there is no `PATCH /auth/me`, `UserSummary` is
`{ID, Email, DisplayName}`, and `cmd/api` `handleServerTime` serves
`APP_TIMEZONE`; §11 row 14.)*

**Acceptance criteria.**
- Given a fresh migration, then every existing `'UTC'` row reads NULL, a new
  User's `timezone` is NULL, and no `timezone_manual` column exists.
  *(TC-ACC-100)*
- Given `PATCH /auth/me {timezone: "Mars/Olympus"}`, then 422
  `account/invalid-timezone` and the row is unchanged. *(TC-ACC-101)*
- Given a User with `timezone: null` signing in on a device in `Europe/Paris`,
  then the app saves `Europe/Paris` without a prompt; given a stored
  `Asia/Ho_Chi_Minh` and a device in `Europe/Paris`, then the app shows one
  prompt and writes nothing unless the User confirms. *(TC-ACC-102)*
- Given a User whose `timezone` is NULL, then `accountapi` returns
  `Asia/Ho_Chi_Minh` for them. *(TC-ACC-103)*
- Given two Users in different zones, then `accountapi` returns each User's
  own zone in one batch call. *(TC-ACC-104)*

### P0.14 — Cross-module API (`account/api`)

The only package other modules import (`api/api.go`):
`GetUserByID(ctx, id) (*UserSummary, error)` — `(nil, nil)` when absent,
disabled **or not approved** *(code follow-up: `HEAD` hides only disabled
Users — §11 row 12)*; `HasPermission(ctx, code) bool` — resolves the
principal from the request context, fail-closed on every error;
`ListDirectory(ctx, exclude, limit)` — approved, enabled Users other than
the caller, ordered `display_name, id`, `limit` default 50, max 200, clamped
*(code follow-up: `HEAD` resets an out-of-range limit to 50 — §11 row 19)*;
`GetUserNames(ctx, ids) (map[uuid]string, error)`; P0.13's timezone reads;
P0.10's `MintEmailChangeLinks(ctx, requestID)` (for notify's send-time
render); and P1.3's `SuperadminIDs(ctx) ([]uuid.UUID, error)`.
Not permission-gated at this layer: the caller decides who may see the result.

The other direction — what account **calls** — is P0.10's purge registry:
every module that owns per-User data exposes `PurgeOwnerData` from its own
`api/` package, and `cmd/api` hands account the ordered list, as it already
binds other cross-module ports.

### P0.15 — Maintenance tasks

`account:purge_refresh_tokens` — a periodic task on the shared scheduler
(`@every 24h`, `default` queue, light server; `cmd/worker` wires the repository
directly because the worker does not construct the module) that hard-deletes
refresh tokens expired more than 30 days. The account-owned
`password_reset_tokens` table gets the same treatment (expired > 7 days,
`PurgeExpiredPasswordResetTokens`) *(code follow-up: the query and adapter
exist, nothing schedules them — §11 row 20)*. `account:expire_identity_data`
(P0.16) is the third. All three are registered in
[events.md](../../reference/events.md) (the first live, the other two
planned).

### P0.16 — Identity data in the audit log lives 90 days *(Decision 2026-10-02 (A7); owned jointly with `platform/audit`; unbuilt — §11 row 31)*

`audit_log` is owned by `platform/audit` (`0005_platform_audit`, [D-25]; the
action taxonomy is [backend/MODULES.md](../../../backend/MODULES.md) §5.3), and
this requirement binds **every** row in it, from every module — not only rows
about deleted Users. Account owns the requirement because it writes most of
the identifying data and owns the sweep that enforces it.

- **What counts as identifying data:** the target User's id (`target_id` when
  `target_kind = 'user'`), email and display name wherever they appear
  (`metadata` keys such as `email_before`, `email_after`, `email`,
  `display_name`), the client IP (`ip`), the user agent (`user_agent`), and any
  other `metadata` key that names a person. `action`, `occurred_at`,
  `actor_kind` and `target_kind` are not identifying.
- **Stored encrypted.** Each new row keeps its identifying fields in one new
  column, `pii bytea` — AES-256-GCM over a JSON object, encrypted in
  `platform/audit` before the insert with a key from a new environment
  variable, **`AUDIT_PII_KEY`** (32 bytes, base64; separate from the JWT keys,
  like `TOTP_KMS_KEY` in [security.md §2.4](../../architecture/security.md)).
  The plaintext columns (`ip`, `user_agent`, a user `target_id`) are written
  NULL and `metadata` carries no identifying key. With `AUDIT_PII_KEY` unset
  the identifying fields are **dropped, never stored in clear**, and the
  binaries log a warning at start; the audit write stays best-effort as
  before.
- **Readable only by Superadmins.** Decrypting `pii` is allowed only to a
  User whose effective permissions contain `*`. No reader exists yet — the P2
  audit-log reader is where this applies.
- **90 days, then anonymised.** The daily sweep `account:expire_identity_data`
  (light server, `default` queue, `@every 24h`, wired directly in `cmd/worker`
  like `account:purge_refresh_tokens`) does three things in batches: (1) for
  every `audit_log` row older than 90 days, sets `pii`, `ip`, `user_agent` and
  a user `target_id` to NULL and strips identifying `metadata` keys, keeping
  `action`, `occurred_at` and the acting admin's `actor_id`; a row whose actor
  is its own subject (login, logout, refresh, registration — the User acting
  on themselves) loses `actor_id` too, since there it identifies the subject;
  (2) encrypts in place any younger row that still holds plaintext identifying
  fields (the rows written before this ships) — or, without a key, drops them;
  (3) deletes `deleted_users` rows older than 90 days and
  `email_change_requests` rows closed more than 90 days ago (P0.10). The `audit_log`
  half calls a `platform/audit` function, so the rule lives beside the table
  it rewrites.

`audit_log.actor_id` keeps its `ON DELETE SET NULL`, so deleting a User
already blanks them as an actor; this requirement covers everything else.

*Acceptance criteria.*
- Given `AUDIT_PII_KEY` set, when an admin edits a User's email, then the new
  `audit_log` row has NULL `ip` and `user_agent`, no email in `metadata`, and a
  `pii` value that decrypts to the IP, user agent and both emails.
  *(TC-ACC-150)*
- Given a row 91 days old, when the sweep runs, then `pii`, `ip`,
  `user_agent` and the user `target_id` are NULL, `action` and `occurred_at`
  are unchanged, and `actor_id` is kept for an admin action on another User
  but cleared for a login. *(TC-ACC-151)*
- Given a plaintext row 10 days old from before the change, when the sweep
  runs, then its identifying fields move into `pii`. *(TC-ACC-152)*
- Given `AUDIT_PII_KEY` unset, then new rows carry no identifying field in any
  column and the start-up log warns. *(TC-ACC-153)*

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
  turns each into a bell entry for the Superadmins (SPEC-05 P1.5). The events
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
  | `account:user_registered` | `Register` (P0.1), only when a new Pending User was created — never for an existing email (Decision 2026-10-02 (A1)) | `approval_status` (always `pending` since A2 removed the founder rule); `actor_id` = the registrant |
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
  The **Superadmin set** is `accountapi.SuperadminIDs(ctx)` *(definition per
  Decision 2026-10-02 (A11))*: the enabled, Approved Users whose **effective
  permissions contain `*`** — defined by permission, not by role name, so a
  custom role granted `*` counts and a renamed role changes nothing. The query
  walks the role hierarchy exactly as `GetEffectivePermissions` does
  (non-expired grants plus every ancestor) and keeps the Users whose set holds
  the literal code `*`; at most 50. (The Approvers are the different set whose
  effective permissions satisfy `users:approve`, P0.1; out of the box every
  Superadmin is one.) `cmd/worker`, which does not construct the module,
  satisfies notify's port with the same query over the account repository, as
  it does for `ResolveRecipient`. Registration keeps its existing Approver
  dispatch (P0.1); see SPEC-05 P1.5 for how the Superadmin fan-out avoids a
  duplicate.
  *AC:* given an admin disabling a User, then exactly one
  `account:user_access_changed` with `access: disabled`, the admin as
  `actor_id` and the User's email and name is enqueued after the write; a
  failed write enqueues none *(TC-ACC-120)*; given each of the seven
  operations, then its event is published with the fields above
  *(TC-ACC-121…126)*; given a refused delete (P0.10 step 4), then no
  `account:user_deleted` *(TC-ACC-123)*; given a registration of an existing
  email, then no `account:user_registered` *(TC-ACC-121)*; given a custom role
  granted `*` and a User holding only it, then `SuperadminIDs` includes that
  User, and a User holding `superadmin` while Disabled is excluded
  *(TC-ACC-127)*.
- **P1.4 Self-service email change** *(Decision 2026-10-02 (A9); unbuilt)* —
  `POST /api/v1/auth/email-change {new_email, current_password}`
  (authenticated, the caller's own User) runs P0.10's verified flow: the
  current password is checked as in `POST /auth/password` (P0.12, counted on
  the login throttle), then the same `email_change_requests` row, the same two
  emails, the same public confirm and cancel routes, 503
  `account/mail-unavailable` without SMTP; resend and cancel are
  `POST /auth/email-change/resend` and `DELETE /auth/email-change`. The
  settings page shows the pending address until it is confirmed. *AC:* a User
  changing their own email keeps signing in with the old one until the link
  is used *(TC-ACC-160)*.

### P2 — future considerations (design for, don't build)

- MFA/TOTP and step-up for destructive admin actions ([D-27]/[D-28],
  [security.md §2.4](../../architecture/security.md)); re-entry per
  [backlog.md § Deferred](../backlog.md).
- Policy bundles and user groups layered on roles (ADR-02); tenant-scoped role
  grants (ADR-07 steps 5–7).
- An audit-log reader. Reading rows is for `audit:read` holders; decrypting
  their identifying data (P0.16) is for Superadmins only — a User whose
  effective permissions contain `*`.

## 6. Data model

All account tables are **global** — no `tenant_id`, no RLS policy — per the
specs README Tenancy convention and ADR-07: they are read before any tenant is
resolved (`RequireAuth` runs ahead of `RequireTenant`), and the README's list
of exempt tables names every one of them, the two target tables below
included. `audit_log` is owned by `platform/audit` (`0005_platform_audit`,
[D-25]), not by this module; P0.16 adds one column to it (below).

| Table | Created by | Columns (as shipped) | Constraints and indexes |
|---|---|---|---|
| `users` | `0002_account_users`; `0006_account_local_auth`; `0031_account_user_approval` | `id uuid PK`, `oidc_subject text` (legacy, nullable since `0006`), `email text`, `display_name text`, `avatar_url text`, `role text DEFAULT 'user'` (legacy label — authorisation is `user_roles`), `locale text DEFAULT 'en-US'`, `timezone text NOT NULL DEFAULT 'UTC'`, `token_version int DEFAULT 1`, `disabled_at timestamptz`, `created_at`, `updated_at`; `0006`: `password_hash text` (Argon2id PHC, nullable), `password_updated_at`; `0031`: `approval_status text NOT NULL DEFAULT 'pending'`, `approval_note text`, `approved_at`, `approved_by uuid → users ON DELETE SET NULL` | `email UNIQUE` (case-sensitive; every write path lower-cases first), `oidc_subject UNIQUE`, `users_approval_status_check` (`pending\|approved\|rejected`), partial `users_approval_pending_idx (created_at DESC, id DESC) WHERE approval_status <> 'approved'`. `0031` back-filled every existing row to `approved`. |
| `roles` | `0003_account_rbac` | `id`, `code text`, `name`, `description`, `parent_id → roles ON DELETE SET NULL`, `is_system bool`, timestamps | `code UNIQUE`; `roles_no_self_parent` CHECK (longer cycles are refused in the handler, P0.11); `roles_parent_idx`. Seven seeded rows, all `is_system`. |
| `permissions` | `0003` (catalog), `0031` (`users:approve`); other modules seed their own codes | `id`, `code`, `description`, `created_at` | `code UNIQUE`; `permissions_code_format` CHECK (P0.7 grammar). |
| `role_permissions` | `0003` | `role_id → roles CASCADE`, `permission_id → permissions CASCADE`, `granted_at`, `granted_by → users SET NULL` | PK `(role_id, permission_id)`. |
| `user_roles` | `0003` | `user_id → users CASCADE`, `role_id → roles CASCADE`, `granted_at`, `granted_by → users SET NULL`, `expires_at` (nothing sets it today) | PK `(user_id, role_id)`; `user_roles_user_idx`. |
| `refresh_tokens` | `0004_account_sessions` | `id`, `user_id → users CASCADE`, `token_hash bytea`, `expires_at`, `created_at`, `revoked_at`, `revoke_reason`, `replaced_by_id`, `parent_id` (both `→ refresh_tokens SET NULL`), `issued_ip inet`, `issued_user_agent` | `refresh_tokens_hash_idx` UNIQUE; partial `refresh_tokens_user_idx`, `refresh_tokens_expiry_idx` `WHERE revoked_at IS NULL`. |
| `password_reset_tokens` | `0010_account_password_reset_tokens` (behaviour: SPEC-05 P0.3) | `id`, `user_id → users CASCADE`, `token_hash bytea UNIQUE`, `expires_at`, `used_at`, `created_at` | `password_reset_tokens_user_idx`. |

`0006` dropped `user_oidc_roles` (D-26.r1). `updated_at` is set explicitly by
every UPDATE in `query/*.sql` (README updated_at convention).

**Target migrations** (none exists on `HEAD`; verify the next free numbers
with `ls backend/db/migrations | tail -2` — several specs can claim numbers
concurrently):

`000N_account_user_timezone` (P0.13, Decision 2026-10-02 (A8)):

```sql
ALTER TABLE users ALTER COLUMN timezone DROP NOT NULL;
ALTER TABLE users ALTER COLUMN timezone DROP DEFAULT;
-- nothing has ever written users.timezone, so every 'UTC' row was never chosen by anyone
UPDATE users SET timezone = NULL, updated_at = now() WHERE timezone = 'UTC';
```

The down migration sets NULL rows back to `'UTC'` and restores `NOT NULL
DEFAULT 'UTC'`. There is no `timezone_manual` column.

`000N_account_password_must_change` (P0.12, A2):

```sql
ALTER TABLE users ADD COLUMN password_must_change boolean NOT NULL DEFAULT false;
```

`GetUserAuthSnapshot` returns it beside `disabled_at`, `approval_status` and
`token_version` (P0.5).

`000N_account_email_change_requests` (P0.10, A9):

```sql
CREATE TABLE email_change_requests (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    new_email         text NOT NULL,                 -- lower-cased
    requested_by      uuid REFERENCES users(id) ON DELETE SET NULL,
    confirm_hash      bytea UNIQUE,                  -- SHA-256 of the confirm token, minted at send time
    cancel_hash       bytea UNIQUE,                  -- SHA-256 of the old address's cancel token
    expires_at        timestamptz,                   -- send time + 1 hour
    sends             int NOT NULL DEFAULT 0,        -- resend cap (5)
    last_sent_at      timestamptz,                   -- resend spacing (1 minute)
    confirmed_at      timestamptz,
    cancelled_at      timestamptz,
    cancel_reason     text,                          -- 'replaced' | 'expired' | 'taken' | 'admin' | 'owner'
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX email_change_requests_open_idx
    ON email_change_requests (user_id) WHERE confirmed_at IS NULL AND cancelled_at IS NULL;
```

Tokens follow the reset-token construction (≥ 256-bit CSPRNG, SHA-256 at rest,
single use); a resend overwrites both hashes, which is what revokes the
previous links. A closed row (confirmed or cancelled) still holds two
addresses, so `account:expire_identity_data` (P0.16) deletes it 90 days after
it closed.

`000N_account_deleted_users` (P0.10, A6):

```sql
CREATE TABLE deleted_users (
    user_id       uuid PRIMARY KEY,                  -- no FK: the users row is gone
    email         text NOT NULL,
    display_name  text NOT NULL,
    roles         text[] NOT NULL,                   -- role codes held at deletion
    deleted_by    uuid,                              -- the acting admin; no FK, it may be deleted later
    deleted_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX deleted_users_deleted_at_idx ON deleted_users (deleted_at);
```

No unique index on `email`: a deleted email may register again (P0.10), and
may be deleted again. Rows older than 90 days are removed by
`account:expire_identity_data` (P0.16). There is no reader: the table is read
with SQL by the operator during an incident.

`000N_platform_audit_pii` (P0.16, A7; owned by `platform/audit`, not this
module): `ALTER TABLE audit_log ADD COLUMN pii bytea;` plus an index on
`occurred_at` for the sweep (`audit_log_occurred_idx` already exists).

**Takeout** (README convention): the User's own `users` row exports as one JSON
object `{email, display_name, avatar_url, locale, timezone, created_at,
roles}`. Excluded, with reason: `password_hash`, `password_must_change`,
`refresh_tokens`, `password_reset_tokens`, `email_change_requests`
(credential material) and `token_version` / approval fields (operator state,
not the User's data). `deleted_users` is operator state about a User who no
longer exists, and is never exported.

## 7. API summary

Every path is under `/api/v1`. "public" operations declare `security: []`;
every other operation declares `security: [{bearerAuth: []}]` and, when behind
`RequirePermission`, `x-required-permission: <code>` (README AuthZ, OpenAPI
encoding). `module.go` `MountHTTP` / `mountAdmin` **is** the authorisation
policy; this table copies it.

| Method | Path | Permission | Request | Response | Errors (beyond 401 about:blank on authenticated routes) |
|---|---|---|---|---|---|
| POST | `/auth/register` | public | `{email, password, display_name?}` | 201 `{status: "registered"}` — the same for a new and an existing email (P0.1) | 400 about:blank (body), 422 `account/invalid-email`, 422 `account/password-policy`, 429 `platform/rate-limited` |
| POST | `/auth/login` | public | `{email, password, remember?}` | 200 `{access_token, expires_in, token_type, user}` + cookies | 400 about:blank, 401 `account/invalid-credentials`, 403 `account/account-disabled` \| `account/account-pending` \| `account/account-rejected`, 429 `account/too-many-attempts` |
| POST | `/auth/refresh` | public | cookie or `{refresh_token}` | 200 `{access_token, expires_in, token_type}` + cookies | 401 about:blank (missing, invalid, expired, reused, user unavailable) |
| POST | `/auth/logout` | authenticated | refresh token: cookie or `{refresh_token}` | 204 | 422 `account/validation` (no refresh token — use logout-all; P0.6) |
| POST | `/auth/logout-all` | authenticated | — | 204 | — |
| GET | `/auth/me` | authenticated | — | 200 `{id, email, display_name, roles, permissions, timezone: string\|null, password_must_change}` | — |
| PATCH | `/auth/me` | authenticated | `{timezone}` | 200 `/auth/me` body | 422 `account/invalid-timezone`, 422 `account/validation` (P0.13, unbuilt) |
| POST | `/auth/password` | authenticated (allowed while `password_must_change`) | `{current_password, new_password}` | 200 `{access_token, expires_in, token_type}` + cookies | 422 `account/wrong-current-password` \| `account/password-policy`, 429 `account/too-many-attempts` (P0.12, unbuilt) |
| POST | `/auth/email-change/confirm` | public | `{token}` | 204 | 422 `account/invalid-email-change-token`, 409 `account/email-taken` (P0.10, unbuilt) |
| POST | `/auth/email-change/cancel` | public | `{token}` | 204 (always) | 400 about:blank (body) (P0.10, unbuilt) |
| POST | `/auth/email-change` | authenticated | `{new_email, current_password}` | 202 `{pending_email}` | 422 `account/invalid-email` \| `account/wrong-current-password`, 409 `account/email-taken`, 429, 503 `account/mail-unavailable` (P1.4, unbuilt) |
| POST, DELETE | `/auth/email-change/resend`, `/auth/email-change` | authenticated | — | 202 / 204 | 404 (no open request), 429 `account/rate-limited` (P1.4, unbuilt) |
| POST | `/auth/forgot-password`, `/auth/reset-password` | public | — | — | owned by SPEC-05 P0.3 |
| GET | `/admin/users?status=&q=&limit=&offset=` | `users:read:any` | — | 200 `{items: AdminUser[], total, limit, offset, counts}` | 422 `account/validation` |
| GET | `/admin/users/{id}` | `users:read:any` | — | 200 `AdminUser` | 404 `account/user-not-found` |
| POST | `/admin/users` | `users:write:any` | `{email, password, display_name?}` | 201 `AdminUser` | 422 `account/invalid-email`, 422 `account/password-policy`, 409 `account/email-taken` |
| PATCH | `/admin/users/{id}` | `users:write:any` | `{email?, display_name?, password?}` | 200 `AdminUser` (a changed email appears as `pending_email`, P0.10) | 422 (as create), 403 `account/escalation`, 404 `account/user-not-found`, 409 `account/email-taken`, 503 `account/mail-unavailable` (email change without SMTP) |
| POST | `/admin/users/{id}/email-change/resend` | `users:write:any` | — | 200 `AdminUser` | 403 `account/escalation`, 404 (no open request), 429 `account/rate-limited` (P0.10, unbuilt) |
| DELETE | `/admin/users/{id}/email-change` | `users:write:any` | — | 204 | 403 `account/escalation`, 404 (P0.10, unbuilt) |
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
`account/account-not-approved` for an unapproved caller and — outside the four
exempt routes — 403 `account/password-change-required` while the caller must
change their password (P0.5).

**Pagination.** `GET /admin/users` is the module's only list endpoint: `limit`
default 25, max 100, lenient and clamped; `offset` ≥ 0; ordered `(created_at
DESC, id DESC)`. Its envelope is `{items, …}` per the README Pagination
convention, and it is that convention's one named exception to keyset paging:
it keeps `offset`, `total` and `counts` (Decision 2026-10-02 (A5), P0.8).

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
`account/invalid-timezone`, `account/delete-incomplete`, and — from the
2026-10-02 decisions — `account/password-change-required`,
`account/wrong-current-password`, `account/mail-unavailable`,
`account/invalid-email-change-token`. `account/rate-limited` is SPEC-05
P0.3's slug (reset throttle), reused here for the email-change resend.
`platform/rate-limited` belongs to `platform/middleware`. *(Code follow-up:
§11 rows 16–17, 25–31.)*

## 8. Events

**Emitted bus events** — P1.3, planned (Decision 2026-10-01b (D3): no
exemption from ADR-08's "≥ 1 bus event" rule). `HEAD` emits none (§11 row 24).
All are emitted from `cmd/api`, after commit, with the `accountapi.AdminEvent`
payload of P1.3; each has one consumer, a SPEC-05 P1.5 notify task on the light
server's `default` queue.

| Event | Consumer task | Notify type |
|---|---|---|
| `account:user_registered` | `notify:on_user_registered` | `account.registration_pending` (shared with P0.1's Approver dispatch, so a recipient who already has it gets no second row); emitted only for a new Pending User (A1) |
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
| `notify:dispatch` | task, owned by notify | live | account enqueues it through `notifyapi.Enqueue` for `account.registration_pending` (P0.1) and `account.password_reset` (SPEC-05 P0.3); planned, the transactional email-only types `account.registration_received` and `account.registration_repeat` (P0.1, A1) and `account.email_change_confirm` / `account.email_change_alert` (P0.10, A9). New notify **types**, not new tasks — already in events.md as a notify row. |
| `account:purge_refresh_tokens` | periodic task | live (`cmd/worker`, `@every 24h`, `default`) | in events.md. |
| `account:purge_reset_tokens` | periodic task | planned (P0.15) | in events.md as planned; lands with its `scheduler.Register` (§11 row 20). |
| `account:expire_identity_data` | periodic task | planned (P0.16, A7) | anonymises `audit_log` identity data and deletes `deleted_users` and closed `email_change_requests` rows after 90 days; in events.md as planned (§11 row 31). |

**Consumed:** nothing. Audit rows (`account.*`, P0.1–P0.12) are not bus
events — P1.3's events are published beside them, not instead of them; the
audit taxonomy lives in `platform/audit/logger.go` and
[MODULES.md §5.3](../../../backend/MODULES.md).

## 9. Success metrics (n=1 honest)

- Zero Sessions held by a non-`approved` or Disabled User after the
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
- After P0.16 lands: no `audit_log` row older than 90 days holds an IP, a user
  agent or an email in clear or in `pii`
  (`SELECT count(*) FROM audit_log WHERE occurred_at < now() - interval '90 days' AND (pii IS NOT NULL OR ip IS NOT NULL)` is 0).

## 10. Open questions

- **Q6 (owner, non-blocking) — encrypt the `deleted_users` snapshot?** A7
  (P0.16) encrypts identity data in `audit_log` with `AUDIT_PII_KEY`; the
  `deleted_users` snapshot written by a delete (P0.10, A6) holds the same kind
  of data — email, display name, roles — in plaintext and only expires after
  90 days. Options: (a) encrypt its identifying columns with the same key and
  the same superadmin-only read rule; (b) keep it plaintext, since it is a
  short-lived tombstone that no API reads. Recommended: (a), so one rule
  covers every piece of identity data that outlives its User. Does not block
  §11 row 29; whichever answer lands is applied to it.

Q1 (deleting a user orphans their media objects) was decided on
2026-10-01 — Decision 2026-10-01b (D1), now P0.10's delete order and SPEC-04
P0.7. Q2–Q5 were decided on 2026-10-02 and removed the same way; their
numbers are not reused:

- Q2 (admin list paging) — decided 2026-10-02 (A5): offset paging stays, as a
  named exception to the README Pagination convention (P0.8).
- Q3 (registration enumeration) — decided 2026-10-02 (A1): register answers
  201 uniformly (P0.1; §11 rows 5 and 25).
- Q4 (logout scope) — decided 2026-10-02 (A3): logout ends one Session and
  bumps nothing (P0.6; §11 row 27).
- Q5 (per-account lockout) — decided 2026-10-02 (A4): the per-email counter
  becomes per-(email, client IP) (P0.2; §11 row 28).

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the docs commits on top of it change no
code). The spec text above is the target; this section lists every place the
shipped code diverges from it. Rows are ordered by severity: **Sec** 1–5,
**AuthZ** 6–7, **Data** 8, **Integ** 9–13, **Func** (timezone) 14,
**Contract** 15–20, **UX** 21–22, **Hyg** 23, then **P1** 24 (added by
Decision 2026-10-01b), then the rows the owner decisions of 2026-10-02 turned
into targets — **Sec** 25–28 (uniform registration, pre-created Superadmin,
single-Session logout, per-(email, IP) lockout), **Data** 29 (delete purges
every module), **Sec** 30 (verified email change), **Data** 31 (audit identity
retention) — all appended rather than renumbering rows other documents cite.
Row 10 is superseded by row 26 and closes with it; rows 14 and 24 were
rewritten in place for A8 and A11, since neither describes shipped
behaviour. A row closes when the code
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
| 5 | P0.2 uniform timing | An unknown email costs the same as a wrong password. | `handler/auth.go` `Login` returns 401 on `ErrUserNotFound` before `auth.VerifyPassword`, so unknown emails skip the ~64 MiB Argon2id and answer measurably faster. Load-bearing since Decision 2026-10-02 (A1): once register answers uniformly (row 25), this is the remaining email-enumeration oracle. | **backend:** on `ErrUserNotFound` (and on an empty hash) verify against a fixed dummy PHC hash before answering. **test:** TC-ACC-007 (both paths call the verifier once). | Found while writing SPEC-01, 2026-10-01; Decision 2026-10-02 (A1) |
| 6 | P0.9, P0.10 target authority on approve / reject / revoke / disable / enable | The `targetAuthorityDenial` rule of Edit and Delete also gates the approval decisions and disable/enable → 403 `account/escalation`. | `handler/admin.go` `decide` and `SetDisabled` check self-target and last-approver only. A `users:write:any` holder (the seeded `admin`) can disable any superadmin who is not the last approver, killing their session (`DisableUser` bumps `token_version`); `/CLAUDE.md` describes only Edit/Delete as takeover-guarded. | **backend:** call `targetAuthorityDenial` in `decide` and `SetDisabled` (skip for `id == actor`, already refused). **openapi:** 403 `account/escalation` on the five operations. **test:** TC-ACC-055, TC-ACC-056. | Found while writing SPEC-01, 2026-10-01 |
| 7 | P0.12 bootstrap re-enables | The named User ends up approved, **enabled** and holding `superadmin`. | `backend/cmd/api/main.go` `bootstrapSuperadmin` grants the role and approves, but never reads or clears `disabled_at`, although its doc comment promises "approved, enabled and holding `superadmin`". A disabled bootstrap account stays locked out. | **backend:** if `user.Disabled`, call `EnableUser` (and bump `token_version` once). **test:** TC-ACC-020 (fake adapter: disabled + unapproved + no role → all three fixed, one bump; already-correct → zero writes). | Found while writing SPEC-01, 2026-10-01 |
| 8 | P0.10 delete — no orphaned objects | Deleting a user leaves no unreachable stored object: disable, purge the owner's assets (`mediaapi.PurgeOwnerAssets`), then — under `FOR UPDATE` on the user row — a final pass that must report 0 rows before `DeleteUser`; otherwise 503 `account/delete-incomplete` and the user is kept. | `handler/admin_users.go` `DeleteUser` → `query/admin.sql` `DeleteUser` hard-deletes straight after the guards; `assets.owner_id` is `ON DELETE CASCADE` (`0007_media_assets`), so asset rows vanish without a `deleting` tombstone and `media/service.go` `PurgeOrphans` (which reads only `ListAssetsForPurge` rows) never visits them; the objects stay in MinIO/R2 forever. | **backend (media):** SPEC-04 §11 row 19 (`PurgeOwnerAssets`). **backend (account):** a one-method media port in `account.Deps`, bound by `cmd/api` through a closure over `mediaMod`; `DeleteUser` runs P0.10 steps 1–4; new query `LockUserForDelete` (`SELECT 1 FROM users WHERE id = $1 FOR UPDATE`) and a transaction for step 3 (the adapter receives `RunInTx` as other modules' do); 503 `account/delete-incomplete` with `Retry-After`. **openapi:** the 503 on `adminDeleteUser`. **frontend:** the slug in `problems.ts`; `/admin/users` offers a retry. **test:** TC-ACC-064…066. | Found while writing SPEC-01, 2026-10-01; Decision 2026-10-01b (D1) |
| 9 | P0.11 role PATCH semantics | PATCH keeps omitted fields. | `handler/admin.go` `decodeRoleBody` sets `Name = Code` when `name` is empty — on PATCH `code` is normally absent, so `Name` becomes `""`; `query/rbac.sql` `UpdateRole` then writes `name`, `description` and `parent_id` from the body, clearing an omitted description or parent. `shared/openapi.yaml` `RoleInput` declares `name` required, which hides it; no UI calls `updateRole` today (`frontend/src/lib/admin.ts` exports it unused). | **backend:** read the target row and merge (absent JSON key ⇒ keep; explicit `""` for `parent_code` ⇒ root role); keep `name` non-empty. **openapi:** a separate `RolePatch` schema with all-optional fields. **test:** TC-ACC-073. | Found while writing SPEC-01, 2026-10-01 |
| 10 | P0.1 first-run bootstrap race — **superseded by row 26** | *Superseded by Decision 2026-10-02 (A2).* There is no first-registrant rule any more, so there is no race left to close; the row is kept so its number holds. | `handler/auth.go` `Register`: `CreateLocalUser`, then `CountUsers` outside any transaction — two concurrent first registrations can both count 2, so nobody is approved and nobody can approve. | **None of its own:** row 26 deletes the `CountUsers` branch this row would have fixed; it closes with row 26 (TC-ACC-016 is retired in favour of TC-ACC-130). | Found while writing SPEC-01, 2026-10-01; superseded by Decision 2026-10-02 (A2) |
| 11 | P0.11 expired grants | Re-assigning a role whose grant expired grants it afresh; `user_count` counts live grants. | `query/admin.sql` `ReplaceUserRoles` inserts `ON CONFLICT (user_id, role_id) DO NOTHING`, so an expired row is kept and the role stays invisible; `ListRolesAdmin`'s `user_count` subquery ignores `expires_at`, so `DeleteRole` refuses a role only expired grants reference. Latent: nothing sets `expires_at` today. | **query:** `ON CONFLICT … DO UPDATE SET expires_at = NULL, granted_by = EXCLUDED.granted_by, granted_at = now()`; filter `user_count` by `expires_at IS NULL OR expires_at > now()`. **test:** TC-ACC-058. | Found while writing SPEC-01, 2026-10-01 |
| 12 | P0.14 `accountapi` visibility | `GetUserByID` hides disabled **and** non-approved accounts; `GetUserNames` resolves names for any id (display only). | `repository/adapter.go` `GetUserSummaryByID` filters `disabled_at` only, so a pending or rejected account resolves — including as an email recipient through `cmd/worker` `recipientResolver`. | **backend:** a dedicated query behind `GetUserByID` that adds `approval_status = 'approved'`. The worker's `recipientResolver` must **not** inherit the filter for the account's transactional email types — `account.registration_received`, `account.registration_repeat` (P0.1) and `account.email_change_alert` (P0.10) are addressed to Users who are Pending, Rejected or about to change address — so it either keeps its own query or takes the type into account. **test:** TC-ACC-090. | Found while writing SPEC-01, 2026-10-01 |
| 13 | P0.2 disabled-attempt audit | A correct-password login on a disabled account writes `account.session.disabled_attempt`. | `backend/internal/platform/audit/logger.go` defines `ActionAuthDisabledAttempt`; `handler/auth.go` `Login` returns 403 without auditing. | **backend:** write the event before the 403. **test:** TC-ACC-004. | Found while writing SPEC-01, 2026-10-01 |
| 14 | P0.13 per-user timezone | Migration (`users.timezone` NULLable with no default, `'UTC'` rows set to NULL; no `timezone_manual`); `PATCH /auth/me {timezone}` with 422 `account/invalid-timezone`; `/auth/me` returns `timezone` as a name or `null`; `accountapi` single + batch lookups returning the stored zone, else `Asia/Ho_Chi_Minh`; frontend saves the device zone automatically while the stored one is NULL, otherwise offers one confirm prompt when the device differs, a settings picker, and `lib/time.ts` from `/auth/me`. | `backend/db/migrations/0002_account_users.up.sql` `timezone TEXT NOT NULL DEFAULT 'UTC'`; `module.go` mounts no `PATCH /auth/me`; `handler/auth.go` `Me` returns no zone; `api/api.go` `UserSummary` is `{ID, Email, DisplayName}`; `backend/cmd/api/main.go` `handleServerTime` serves `APP_TIMEZONE`; `frontend/src/lib/problems.ts` lacks the slug. | **migration:** `000N_account_user_timezone` (§6). **backend:** `PATCH /auth/me` handler + `UpdateUserTimezone` query (`updated_at = now()`); `Me` reads the row (the token carries no zone); `UserSummary.Timezone` + `GetUserTimezones(ctx, ids)`, both mapping NULL and unparseable names to `Asia/Ho_Chi_Minh`. **openapi:** both operations, `CurrentUser.timezone` (nullable), the 422. **frontend:** slug; the post-sign-in save when `timezone` is null; the one-time switch prompt when the device zone differs from a set zone (dismissal remembered per browser and zone); settings picker; `lib/time.ts`. Then its readers (README Per-user timezone list). **test:** TC-ACC-100…104. | Decision 2026-09-30 (Timezone); Decision 2026-10-02 (A8), superseding Decision 2026-10-01 (f); the specs README Per-user timezone cross-cutting gap, item 1 |
| 15 | P0.6 `/auth/me` contract | The OpenAPI response matches the handler: `{id, email, display_name, roles, permissions}` (+ the P0.13 and P0.12 fields). | `shared/openapi.yaml` `CurrentUser` is `allOf: [User, …]`, and `User` requires camelCase `displayName` (plus `avatarUrl`); `handler/auth.go` `Me` sends `display_name` and no avatar. `frontend/src/lib/session.ts` declares its own `Session` type, so nothing fails — the generated `types.gen.ts` type is simply wrong. | **openapi:** give `CurrentUser` its own snake_case properties (`display_name`, `roles`, `permissions`, then `timezone` (nullable) and `password_must_change`); keep `User` only if something else uses it. **frontend:** `session.ts` may then import the generated type. **test:** TC-ACC-040 (handler body keys). | Found while writing SPEC-01, 2026-10-01 |
| 16 | §7 Problem types registered | Every slug the module emits is in `ProblemType` and `PROBLEM_MESSAGES`. | `frontend/src/lib/problems.ts` registers 17 account slugs (`grep -c '"account/' frontend/src/lib/problems.ts` counts each twice: type and message) but not `account/invalid-credentials`, `account/too-many-attempts`, `account/weak-password`, `account/validation` or `account/role-not-found`, all emitted by `handler/auth.go` / `handler/admin.go`; nor the targets `account/user-not-found`, `account/invalid-timezone`, and — from the 2026-10-02 decisions — `account/password-change-required`, `account/wrong-current-password`, `account/mail-unavailable`, `account/invalid-email-change-token`. (`account/invalid-reset-token` and `account/rate-limited` are SPEC-05 §11 row 14.) | **frontend:** add `invalid-credentials`, `too-many-attempts`, `validation`, `role-not-found`, `user-not-found`, `invalid-timezone` (and the four 2026-10-02 slugs with rows 26, 30); `weak-password` needs no entry once row 17 renames it to `password-policy`. **test:** TC-ACC-110 (a grep test listing every `problem(`/`writeError(` code against `problems.ts`). | README Errors convention; found while writing SPEC-01, 2026-10-01 |
| 17 | §7 statuses and slugs | Body/param-shape failures are 422 (`account/validation` or a named type); one slug per rule (`account/password-policy`); a missing user is 404 `account/user-not-found`; 429 carries `Retry-After`. | `handler/auth.go` `Register` answers 400 `account/invalid-email` and 400 `account/weak-password`; `handler/admin.go` `badRequest`/`problem(…StatusBadRequest…)` answer 400 for `validation`, `unknown-role`, `unknown-permission`, `role-cycle`, and `admin_users.go` 400 for `invalid-email`, `password-policy`, `confirmation-mismatch`; `pathUUID` and `notFoundOrInternal` map a missing user to `server.ProblemType("account","not_found")` = 404 about:blank; `Login`'s 429 sets no `Retry-After`. | **backend:** switch those to 422; `weak_password` → `password-policy`; `not_found` → `user-not-found` (keep `role-not-found`); `Retry-After` = the remaining TTL of the tripped counter. **openapi:** the 422s and slugs per §7. **frontend:** none (`problemDisplayMessage` keys on `type`, not status). **test:** TC-ACC-111. | README Pagination/Errors conventions; found while writing SPEC-01, 2026-10-01 |
| 18 | P0.8 `{items}` envelope | `GET /admin/users` answers `{items, total, limit, offset, counts}`. | `handler/admin.go` `ListUsers` writes `users`; `shared/openapi.yaml` `AdminUserPage` requires `users`; `frontend/src/lib/admin.ts` `listUsers` reads `r.users`. | **backend · openapi · frontend:** rename to `items` in one PR (README Envelopes retrofit). **test:** TC-ACC-050. | Decision 2026-09-30 (Envelopes); Decision 2026-10-01 (g) |
| 19 | P0.14 `ListDirectory` limit | Out-of-range `limit` clamps (≤ 0 → 50; > 200 → 200). | `api/api.go` `ListDirectory`: `if limit <= 0 \|\| limit > 200 { limit = 50 }` — resets. | **backend:** clamp. **test:** TC-ACC-091. | Decision 2026-10-01 (e) (limit) |
| 20 | §7 OpenAPI encoding and responses; P0.15 reset-token purge | Public ops `security: []`; others `security: [{bearerAuth: []}]` + `x-required-permission`; every status in §7 declared. The reset-token purge is scheduled (P0.15). | `shared/openapi.yaml`: no top-level `security`; only `/auth/logout`, `/auth/logout-all`, `/auth/me` declare `bearerAuth`, so every `/admin/*` operation reads as public; no `x-required-permission`; `reject`, `revoke-approval`, `disable` lack 409 `account/last-approver`; `logout` lacks 401; `/admin/roles/{id}/permissions` and role PATCH/DELETE 404s are not typed. `backend/cmd/worker/main.go` schedules `account:purge_refresh_tokens` but nothing schedules `PurgeExpiredPasswordResetTokens`. | **openapi:** annotate all account operations per §7. **backend:** register the reset-token purge (light server, `@every 24h`); events.md already carries both task rows (the second as planned) and flips it to live in the same PR. **test:** TC-ACC-095, TC-ACC-112. | README AuthZ OpenAPI encoding; README Events DoD; found while writing SPEC-01, 2026-10-01 |
| 21 | §7 `/admin` route gate | Every `(app)` route group is in `config.matcher` (D-34 edge gate). | `frontend/src/middleware.ts` matcher lists `/`, `/login`, `/register`, `/upload`, `/library/:path*`, `/bank/:path*`, `/people/:path*` — not `/admin/:path*` (also missing `/calendar`, `/weather`; backlog #15). A signed-out visitor gets the admin shell, which then 401s. | **frontend:** add `'/admin/:path*'` (or match the whole group). **test:** TC-ACC-080. | README Frontend convention; backlog #15 |
| 22 | P0.7 client grammar | `frontend/src/lib/session.ts` `can()` decides exactly as `rbac.Permission.Matches`. | `can()` treats `res:*` like any 2-segment grant (satisfies bare/`:any`, never `:own`); the server's `Matches` lets an action wildcard satisfy every scope, `:own` included — a `movies:*` holder is hidden affordances the API would allow. | **frontend:** if `g[1] === "*"` return true after the resource match. **test:** TC-ACC-081 (vitest table mirroring `TestMatches`). | Found while writing SPEC-01, 2026-10-01 |
| 23 | P0.8 `q` is literal | `q` matches as a literal substring. | `query/admin.sql` `ListUsersAdmin` / `CountUsersAdmin` concatenate `'%' \|\| q \|\| '%'` into `ILIKE` without escaping `%`, `_` or `\`. | **query:** escape in the handler (or `ILIKE … ESCAPE '\'` with an escaped argument). **test:** TC-ACC-051. | Found while writing SPEC-01, 2026-10-01 |
| 24 | P1.3 admin-change events; P0.14 `SuperadminIDs` | Seven `account:*` events published after commit with the `accountapi.AdminEvent` payload; `accountapi.SuperadminIDs` — the enabled, Approved Users whose effective permissions contain `*` (Decision 2026-10-02 (A11): by permission, not by role name); each event's consumer edge registered in `cmd/api`; events.md rows. | Not built. `api/api.go` declares no event constant, payload or `SuperadminIDs`; `account.Deps` has no publisher; the handlers (`Register`, `decide`, `SetDisabled`, `DeleteUser`, `SetUserRoles`, `CreateRole`, `UpdateRole`, `DeleteRole`, `SetRolePermissions`, `HandleRefresh`) write audit rows only; `backend/cmd/api/main.go` builds `mediaEvents` after `account.New` and subscribes no `account:*` name. | **backend:** constants + `AdminEvent` in `account/api`; `Deps.Events`; publish after each write per P1.3; a `ListSuperadminIDs` query (the `GetEffectivePermissions` recursive walk, keeping Users whose effective set holds the literal `*`; enabled + approved; limit 50) behind `SuperadminIDs`, and the same query for `cmd/worker`'s notify port; in `cmd/api`, build the publisher before `account.New` and add the seven `Subscribe` edges (the notify tasks are SPEC-05 §11 row 23). **docs:** events.md rows (planned → live in the same PR). **test:** TC-ACC-120…127. | Decision 2026-10-01b (D3); Decision 2026-10-02 (A11) |
| 25 | P0.1 uniform registration | Every well-formed `POST /auth/register` answers 201 `{status: "registered"}`; Argon2id runs on both paths. New email → Pending User, registrant email `account.registration_received`, Approver fan-out. Existing email → nothing written, no Approver notified, one `account.registration_repeat` email by state (Pending / Approved / Rejected), at most one per address per 24 h. `account/email-taken` only on admin create/edit. Registration succeeds without SMTP. | `handler/auth.go` `Register` hashes, then `CreateLocalUser`; on `ErrEmailTaken` (unique violation `23505`, `repository/adapter.go`) it answers 409 `email_taken`; the 201 body is `{status, email, approval_status}`; the registrant is sent nothing. `frontend/src/templates/v1/views/auth/AuthForm.tsx` chooses its message from `approval_status` (`onRegistered(email, data?.approval_status !== "approved")`). | **backend:** after hashing, look the email up; existing → `SET NX EX 86400` on `register:notice:<sha256(email)>`, enqueue `account.registration_repeat` (`data.state`) when the key was set, answer 201; new → create as today (minus the founder branch, row 26), then `account.registration_received` to the registrant and the Approver fan-out; both answer `{status: "registered"}`. **notify:** register the two types (email-only, not persisted) in `notify/api` and the `notify/README.md` registry, with `renderEmail` templates; recipient resolution per row 12. **openapi:** the 201 schema becomes `{status}`; drop the 409. **frontend:** `AuthForm.tsx` shows one message ("check your email — an administrator will review your registration"). **test:** TC-ACC-130…134. | Decision 2026-10-02 (A1); former §10 Q3 |
| 26 | P0.12, P0.5, P0.1 pre-created Superadmin; password change | No first-registrant rule. `cmd/api` creates the `BOOTSTRAP_SUPERADMIN_EMAIL` User from `BOOTSTRAP_SUPERADMIN_PASSWORD` (Approved, enabled, `superadmin`, `password_must_change`) when it does not exist, never touches the password when it does, and warns "no superadmin configured" when the email is unset. While `password_must_change`, `RequireAuth` answers 403 `account/password-change-required` outside `/auth/me`, `/auth/logout`, `/auth/logout-all`, `/auth/password`. `POST /auth/password` changes the password, clears the flag, bumps `token_version` and keeps only the current Session. | `handler/auth.go` `Register` grants `superadmin` and calls `MarkApproved` when `CountUsers` = 1 (`founder`), and skips the Approver fan-out for that User. `backend/cmd/api/main.go` `bootstrapSuperadmin` only re-asserts an existing User; for an unknown email it logs "names no account yet; register it, then restart the api". `backend/internal/platform/config/config.go` has `BootstrapSuperadminEmail` only. `users` has no `password_must_change`; `middleware/auth.go` has no such check; `module.go` mounts no `/auth/password`. | **migration:** `000N_account_password_must_change` (§6). **backend:** delete the founder branch (this closes row 10); `bootstrapSuperadmin` creates the User (hash, approve, grant `superadmin`, set the flag, audit `account.user.created` with `actor_kind = 'system'`) when absent, refuses an empty or short password with a logged error, warns when the email is unset; `BOOTSTRAP_SUPERADMIN_PASSWORD` in `platform/config` and `.env.example`; `GetUserAuthSnapshot` returns the flag and `RequireAuth` checks it against a four-route allow-list; `POST /auth/password` (verify on the login throttle, one transaction, revoke every chain but the presented one with reason `password_change`, bump, re-mint) and the audit action `account.password.changed` in `platform/audit`. **openapi:** the operation, `CurrentUser.password_must_change`, the 403 on authenticated operations. **frontend:** the two slugs; a forced change-password screen while `/auth/me` reports the flag; `AuthForm` loses its "sign in now" branch. **docs:** `/CLAUDE.md` § Account module ("Bootstrapping an approver") in the same PR. **test:** TC-ACC-135…140. | Decision 2026-10-02 (A2) |
| 27 | P0.6 logout ends one Session | `POST /auth/logout` requires the refresh token (cookie or body; none → 422 `account/validation` pointing to logout-all), revokes that token's chain if it is the caller's, does not bump `token_version`, clears cookies. | `handler/auth.go` `Logout`: `Refresh.Revoke` (`auth/refresh.go`) revokes the single presented token, a missing token is ignored, then `BumpUserTokenVersion` — every device's access token stops and the RBAC cache is re-keyed on each logout. | **backend:** require the token; `RevokeRefreshTokenChain` scoped to the caller's `user_id`; drop the bump; audit `scope: session`. **openapi:** the request body and the 422 on `/auth/logout`. **frontend:** none (`TopMenu.tsx` already sends the cookie). **test:** TC-ACC-141…143. | Decision 2026-10-02 (A3); former §10 Q4 |
| 28 | P0.2 lockout per (email, client IP) | Two failure counters: the global per-IP one and one per (email, client IP); no per-email counter shared across addresses. | `handler/auth.go` `loginFailKeys` returns `login:fail:ip:<ip>` and `login:fail:email:<email>`; `loginThrottled` refuses when either reaches 5, so anyone can lock any known email out for 15 minutes. | **backend:** the second key becomes `login:fail:email_ip:<email>:<ip>` (the pending-path clear and `POST /auth/password` use the same pair). Meaningful only with row 4's trusted client IP. **test:** TC-ACC-144, TC-ACC-145. | Decision 2026-10-02 (A4); former §10 Q5 |
| 29 | P0.10 delete purges every module, then the row | Every module owning per-User data exposes an idempotent `PurgeOwnerData(ctx, userID) (remaining int, err error)` that removes its rows and side effects; account runs the registry in order (content modules, media, then tenant in the locked final pass), writes a `deleted_users` snapshot, then deletes the row; any error or remainder → 503 `account/delete-incomplete`, User stays Disabled. Snapshot rows expire after 90 days. | No module exposes `PurgeOwnerData` (`grep -rn PurgeOwner backend` finds nothing); media's `PurgeOwnerAssets` is itself unbuilt (row 8). `handler/admin_users.go` `DeleteUser` hard-deletes and relies on the cascade, which removes rows but never objects or Redis keys. No `deleted_users` table exists. | **backend (each module):** `PurgeOwnerData` in `comic`, `music`, `movie`, `story`, `journal`, `bank`, `people`, `social`, `notify` and `tenant` `api/` packages (media's is row 8 / SPEC-04 §11 row 19), each opening its own tenant scope. **backend (account):** a registry port in `account.Deps`, bound in `cmd/api`; `DeleteUser` runs P0.10's passes over it; the snapshot insert in the final transaction. **migration:** `000N_account_deleted_users` (§6). **frontend:** the Delete confirmation says that deleting frees the email and suggests Reject instead. **docs:** [backup-restore.md](../../operations/backup-restore.md) warns that a restore resurrects deleted Users. **test:** TC-ACC-146…149. | Decision 2026-10-02 (A6), extending Decision 2026-10-01b (D1) |
| 30 | P0.10 email change requires verification; P1.4 | An email change keeps the old address active, stores one pending request, mails a 1-hour confirm link to the new address and a cancel link to the old one, re-checks uniqueness at confirm, rate-limits resend (which revokes old links), cancels on expiry; no SMTP → 503 `account/mail-unavailable`, nothing changed. | `handler/admin_users.go` `UpdateUser` writes the new email at once (`email_before`/`email_after` in the audit) — a typo or a hostile admin moves the login identifier to an address nobody verified. No `email_change_requests` table, no confirm/cancel routes, no notify types. | **migration:** `000N_account_email_change_requests` (§6). **backend:** `UpdateUser` diverts a changed email into a request (503 without `SMTP_HOST`); resend/cancel admin routes; public confirm/cancel routes; `accountapi.MintEmailChangeLinks` for notify's send-time render; audit actions `account.user.email_change_requested`, `account.user.email_changed`, `account.user.email_change_cancelled`. **notify:** `account.email_change_confirm` (to the address the mint returns) and `account.email_change_alert` (to the current address), email-only, not persisted. **openapi:** the four operations, `AdminUser.pending_email`, the 503. **frontend:** the pending address and resend/cancel in `/admin/users`; public confirm and cancel pages (outside the middleware matcher); the slugs. **test:** TC-ACC-155…160. | Decision 2026-10-02 (A9) |
| 31 | P0.16 audit identity data lives 90 days | Every `audit_log` row keeps identifying data (target User id, email, display name, IP, user agent, identifying metadata) only encrypted in `pii` with `AUDIT_PII_KEY`, decryptable only by Superadmins; after 90 days `account:expire_identity_data` anonymises the row (keeping action, time and the acting admin's id) and deletes expired `deleted_users` and closed `email_change_requests` rows. | `backend/db/migrations/0005_platform_audit.up.sql` stores `ip` and `user_agent` in clear and `metadata` as plain `jsonb`; `backend/internal/platform/audit/logger.go` `Write` copies them as given (e.g. `account.user.updated` carries `email_before`/`email_after`); nothing ever rewrites or deletes an `audit_log` row; no `AUDIT_PII_KEY` exists. | **migration:** `000N_platform_audit_pii` (`pii bytea`; owned by `platform/audit`). **backend:** encryption in `audit.Logger.Write` (identifying fields → `pii`, plaintext columns NULL; drop when the key is unset, with a start-up warning); `AUDIT_PII_KEY` in `platform/config` and `.env.example`; an `audit` anonymise/encrypt-in-place function; the `account:expire_identity_data` periodic task in `cmd/worker` (light server, `@every 24h`). **docs:** events.md row live in the same PR. **test:** TC-ACC-150…153. | Decision 2026-10-02 (A7) |

**Already matching on HEAD.**
- Argon2id `m=65536,t=3,p=2` PHC hashing with constant-time verify; HS256-only
  verification with `kid` rotation; refresh tokens 256-bit, SHA-256 at rest,
  unique hash index; three cookies with the P0.3 flags and the `remember`
  split; cookie Domain chosen per request host and mirrored on clear.
- The approval gate on all three channels: login (after the password, counters
  cleared), every authenticated request (403 `account/account-not-approved`),
  and every refresh (401, cookies cleared); `SetUserApproval` bumps
  `token_version` unconditionally; rejected rows are kept.
- The `BOOTSTRAP_SUPERADMIN_EMAIL` re-assert writing only what is missing
  (apart from row 7); `0031` backfilled existing users to `approved`;
  `users:approve` granted to no role. (The first-registrant founder rule `HEAD`
  also runs is no longer the target — row 26.)
- Approver fan-out: SQL prefilter + `rbac.Set.AllowsCode`, enabled and approved
  holders only, capped at 50, the registrant skipped, dedup key per
  registration, best-effort.
- The Approval semantics of P0.9 as worded by Decision 2026-10-02 (A10):
  approve, reject and revoke-approval behave as described, a Rejected email
  cannot register again, and only an admin moves a Rejected User back.
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
- `Register` (pending, fan-out wiring) — TC-ACC-010…015 (`approverIDs` alone
  is covered by the `TestApproverIDs*` tests); the 2026-10-02 targets add
  TC-ACC-130…134 (uniform answer, registrant emails, timing);
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
- `bootstrapSuperadmin` — TC-ACC-020, and for the 2026-10-02 targets:
  creation from the env pair, the must-change gate and `POST /auth/password`
  — TC-ACC-135…140; single-Session logout — TC-ACC-141…143; the per-(email,
  IP) lockout — TC-ACC-144…145; the delete registry and `deleted_users` —
  TC-ACC-146…149; audit identity retention — TC-ACC-150…153; the verified
  email change — TC-ACC-155…160; `SuperadminIDs` by permission — TC-ACC-127;
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

## Decision records

The architecture decisions this spec stands on, folded in from the retired
`docs/adr/` folder on 2026-10-01. The `ADR-NN` ids stay the stable citation;
the anchors below are fixed. Each record keeps the binding shape — Context →
Decision → Options considered → Trade-offs → Consequences → Action items. The
Decision, Options and Trade-offs are the narrative layer, kept verbatim (it
records what was known at the time and may name things since retired); Context,
Consequences and Action items are the fact layer, corrected in place and true as
of this file's `Last verified`. Where the spec above already states a fact, the
record points to it instead of repeating it.

<a id="adr-02"></a>
### ADR-02 — Reconcile RBAC: role hierarchy (built) vs policy bundles (specced)

**Decided:** 2026-05-24 · **Status:** accepted; amended by [ADR-06](#adr-06) (Authentik-synced roles removed)

Deciders: kirito. Affects [D-26] in [feature-inventory.md](../feature-inventory.md)
and [access-policies.md §2, §3.3](../../architecture/deferred/access-policies.md)
(then `archivetech.md`).

#### Context

*As found on 2026-05-24. v1 shipped on the role-hierarchy model exactly as
decided here; the policy-bundle/user-group layer and file-gating remain
deferred and have no code. One input has since gone: [ADR-06](#adr-06)
removed Authentik, and migration `0006` dropped `user_oidc_roles`, so effective
permissions come from `user_roles` + role ancestors only (P0.7). The composition
rule in Trade-offs still names that table; it is kept as written.*

The project had **two specs for access control that contradicted each other**,
and code existed for one of them.

##### Spec A — Role hierarchy (CLAUDE.md + feature.md + actual code)

- Permission grammar: `<resource>:<action>[:<scope>]` with `*` / `:any` / `:own` wildcards.
- Grants flow through **roles**. `roles.parent_id` forms an adjacency-list hierarchy: `guest → user → creator → editor → moderator → admin → superadmin`.
- Effective permission set = walk role ancestors via recursive CTE, union with directly-assigned `user_roles`, union with `user_oidc_roles` (synced from Authentik groups — retired with ADR-06). [D-26]
- Implemented in `backend/internal/modules/account/rbac/`.
- Two-channel revocation: `users.token_version` (instant logout-all) + `refresh_tokens.revoked_at` (chain revoke).
- Cache key `rbac:perms:<userID>:v<N>` namespaced by `token_version`.

##### Spec B — Policy bundles (`archivetech.md`, now `docs/architecture/deferred/access-policies.md`)

- Same permission grammar at the leaf.
- Grants flow through **policies** (reusable named bundles like "Radiologist", "Read-Only Auditor"). Policies attach to **user groups** or directly to **users**.
- User groups form their own hierarchy (`user_groups.parent_id`); a user inherits every active policy attached to any ancestor group plus their own per-user policies.
- **File-gated permissions**: certain permissions inside a policy require an uploaded file (license, certificate) to be effective. Admin review queue. File expiry → permission silently disappears.
- Conflict resolution: **deny-wins** (AWS IAM / OPA semantics).
- Its §1 declared "the spec wins, adjust code, not the other way around" — meaning if it were accepted, the existing role-hierarchy code was wrong.

##### Why this was a real conflict

These are not two views of one model. They're two different models with
different primary entities (roles vs policies), different group concepts (the
system roles in spec A are not the same thing as user groups in spec B),
different cache invalidation flows (file-gating in B has no analogue in A), and
different audit semantics (spec B logs "permission became ineffective" events
that spec A has no concept of).

You cannot ship both unmodified. You can ship one, ship both layered, or ship
one now and migrate later. The cost differs.

<!-- adr-narrative -->
#### Decision

**For v1, keep role hierarchy (Spec A) as the grant primitive. Reframe policy bundles (Spec B) as a *layer on top of* roles, deferred to a future phase. File-gated permissions stay as a Phase-3+ feature (per archivetech.md's own phasing), not v1.**

Concretely:

1. **Spec A is canonical for v1.** The existing role-hierarchy code stays. `users.token_version` remains the revocation channel. The recursive-CTE effective-permission walk is unchanged.
2. **Spec B's User Groups become a future module**, not a renaming of `roles`. Call it `usergroup` (or fold it into a future `organization`/`tenant` module — see [D-24]) so the two hierarchies don't collide in vocabulary.
3. **Spec B's Policies layer on top.** When the policy module ships, a policy expands into a set of `(role | permission)` grants; the effective-permission walk gains a "policies attached to this user/group" step *before* the role union.
4. **File-gating becomes a permission-effectivity filter** at the end of the resolution chain. The existing matcher stays grant-only; effectivity filtering is a separate stage that prunes permissions whose required-file row is missing/expired/rejected.
5. **Deny-wins precedence is reserved.** Spec A is grant-only and doesn't currently support deny rules. Reserve the contract — when explicit deny lands, the order is "any deny path wins". archivetech.md §2.3 already commits to this contract; record it now even though no code implements deny.

For v1 itself, none of the policy/group machinery exists. The 2-week sprint ships only the role-hierarchy auth that already works. This ADR's purpose is to **prevent the two specs from being implemented simultaneously and incompatibly**, and to keep the path open for adding policies on top later.

#### Options considered

##### Option A — Migrate to Spec B; rewrite the role module

| Dimension | Assessment |
| --- | --- |
| Complexity | High — rewrites every auth-touching test, migrations 0002+, RBAC engine, middleware |
| Cost | 4–7 days of solo-dev time before v1 ships |
| Scalability | Spec B is arguably the more flexible long-run model |
| Team familiarity | New ground; spec B's deny-wins is unfamiliar |

**Pros:** archivetech.md §1's "spec wins" clause is honoured. File-gated permissions are first-class.
**Cons:** Burns 30–50% of the v1 sprint on a rewrite. Throws away working code with test coverage. The first thing the v1 demo proves is the auth flow — destabilising it in week 1 destabilises everything.

##### Option B — Keep Spec A; layer Spec B on top in a future phase  *(chosen)*

| Dimension | Assessment |
| --- | --- |
| Complexity | Low for v1; medium for the layering work later |
| Cost | 0 days now; ~1 week when policies are added |
| Scalability | Best-of-both — roles for coarse access, policies for organisational fine-grain |
| Team familiarity | Existing code stays; no auth churn |

**Pros:** v1 ships with auth that already works. Layering policies on top of roles is a well-trodden pattern (AWS IAM has both); the effective-permission walk just gains an extra union step. File-gating fits cleanly as a final-stage filter.
**Cons:** Two concept hierarchies for grant management (roles + policies + user groups). Operators have to learn both. The "spec wins" promise in archivetech.md is *softened*, not honoured — explicit ADR needed to record the change of intent.

##### Option C — Hybrid now: keep roles, add policies in v1

| Dimension | Assessment |
| --- | --- |
| Complexity | Medium-high — two new tables, new resolution code, new admin UI |
| Cost | 3–5 days of v1 sprint |
| Scalability | Same as Option B long-run |
| Team familiarity | Mixed |

**Pros:** Avoids the future "we said we'd add policies" debt.
**Cons:** Crowds v1 with non-demo features. Policy admin UI isn't demoable in the 7-step happy path. Pure scope creep against [ADR-01](README.md#adr-01).

#### Trade-off analysis

Option A's strongest argument is the "spec wins" clause; its weakest is that the spec it's honouring (archivetech.md) is itself a 6-screen sketch from `template-main/portal/document/anh{1,2,3}.png` with no code behind it. The spec hasn't earned the right to override working code.

Option C's strongest argument is "do it right the first time"; its weakest is that "right" here means "policies + roles + groups + file-gating + review queue" — five concepts piled into a sprint that already has 8 deliverables. The auth surface gets brittle exactly when the demo needs it stable.

Option B's strongest argument is sequencing — get v1 demonstrable, then add the organisational features when there's a real operator asking for them. Its weakest is the conceptual cost: anyone reading both specs has to mentally compose roles + policies + groups + file-gating into one model. This ADR's job is to make that composition explicit so future-you doesn't reverse-engineer it.

The composition rule, written down once for clarity:

```
effective_permissions(user, tenant):
    1. roles_user_holds = recursive_walk(user_roles ∪ user_oidc_roles)
    2. policies_user_holds = ∪{
           policies_attached(user),
           policies_attached(group) for group in walk(user_groups(user))
       }
    3. grants = ∪{
           permissions(role) for role in roles_user_holds,
           permissions(policy) for policy in policies_user_holds
       }
    4. effective = filter(grants, where file_gate_satisfied(grant, user))
    5. if any deny grant in policies_user_holds matches the required code:
           return DENY
    6. return effective
```

Steps 1, 3, and 5 (without deny) are what currently exists. Steps 2 and 4 are the additions when Spec B layers on. Step 5's deny path is reserved.
<!-- /adr-narrative -->

#### Consequences

**What became easier:**

- v1 shipped on time; the 7-step demo was unaffected.
- The tests in `backend/internal/modules/account/rbac/` stayed green, and
  `rbac.Matches` was later fixed so a wildcard-action grant like `movies:*`
  covers every scope including `:own` — consistent with, not contradicting,
  this decision (P0.7).
- When policies are added, the existing code is unchanged — the new code is
  purely additive (new tables, new resolution stage). Nothing has been added yet.

**What became harder:**

- `access-policies.md` carries a header note saying its RBAC section is
  **layered on top of**, not a **replacement for**, the role hierarchy, and
  that its "spec wins" clause is suspended for v1 (done 2026-07-06).
- Future contributors see two grant concepts and need this record to know how
  they compose. The composition rule lives in this record's Trade-offs (and
  the layering is P2 above); it is **not** in
  `backend/internal/modules/account/README.md`
  (`grep -c effective_permissions backend/internal/modules/account/README.md`
  is 0 — action item 2).
- The "deny-wins" promise commits us to a particular semantics. No
  explicit-deny implementation exists; the promise stands.

**What we said we'd revisit — and what happened instead:**

- The admin UI was built for **roles**, not for policies + groups:
  `/api/v1/admin/*` and the `AdminRolesView` / `AdminUsersView` screens
  (`frontend/src/templates/v1/views/admin/`; P0.8–P0.11). Roles became
  editable, which is why `/auth/me` returns effective permission codes (P0.6).
  The `anh1/2/3.png` mocks did not pull Spec B's tables forward; if a
  Policy/Group sprint happens it starts from a shipped roles UI, not a blank one.
- File-gated permissions (object storage for licenses + review queue + expiry
  checks) remain their own future phase, gated on the policy layer existing.
- Cycle prevention: `roles` still has the self-only DB CHECK
  (`roles_no_self_parent`) with deeper cycles caught at the app layer (§6,
  P0.11). The policy/group migration, when written, must include proper cycle
  prevention.

#### Action items

1. [x] Header note on `access-policies.md` referencing this decision (2026-07-06).
2. [ ] Add the composition rule to `backend/internal/modules/account/README.md` —
   not done; tracked as [backlog.md](../backlog.md) #17.
3. [ ] Reserve depguard rules for `internal/modules/policy/` and
   `internal/modules/usergroup/` — not done (`grep -n 'policy\|usergroup'
   backend/.golangci.yml` finds nothing); tracked with item 2 as backlog #17.
4. [ ] Tracking issue for "RBAC Phase 1.5: policy bundles + user groups" — no
   GitHub issue is filed (`gh issue list --state all --search policy`); the
   deferral is recorded here, in P2 above and in `/CLAUDE.md` § RBAC schism.
5. [ ] Policy/Group sprint "before any admin UI work begins" — overtaken: the
   roles admin UI shipped first (`0031`/`0036` era). The sprint, if scheduled,
   layers onto it.

<a id="adr-06"></a>
### ADR-06 — Local password auth: Portal owns credentials (drop Authentik from the login path)

**Decided:** 2026-07-05 · **Status:** accepted, executed 2026-07-06

Deciders: kirito. Supersedes the OIDC-login decision in
[ADR-05](SPEC-03-platform-ops.md#adr-05) (Milestone 0.4) and the "No local
password auth. OIDC via Authentik" statement that [/CLAUDE.md](../../../CLAUDE.md)
carried at the time (Account module).

#### Context

*As found on 2026-07-05. Everything decided below shipped the next day; what
differs from the text, and what has been added since, is under Consequences.*

The OIDC-via-Authentik design (built and wired in the Phase-0 sprint)
authenticated the user **at the IdP**: the browser was redirected from Portal to
Authentik, the user typed credentials on Authentik's page, and Authentik
returned an authorization code that the API exchanged for the user's identity.

That was architecturally clean, but produced a **user-experience objection**:
the login always left the Portal domain for `auth.portal.localhost`, and the
Portal login form we built was decorative (only the SSO button worked) — so
users saw "two forms". Branding Authentik and a straight-to-IdP redirect ("Mức
2") reduced this to one branded form, but the login screen was still **served
by, and hosted on, Authentik**, not Portal.

The product owner wanted the login form to live **on Portal itself**, with
Portal verifying the password directly — no redirect, no separate identity
service in the login path. This record holds that decision and its
architecture.

> This reverses a prior decision. [ADR-05](SPEC-03-platform-ops.md#adr-05)
> chose OIDC and [ADR-02](#adr-02) assumed Authentik-synced roles. The **token,
> refresh, RBAC, revocation, and audit machinery are unaffected** — only the
> *front door* (how a user proves identity) changes. See "What is reused".

<!-- adr-narrative -->
#### Decision

**Portal authenticates users locally against its own `users` table with a hashed password. Authentik is removed from the login path.** The login form is served by the Portal frontend; the API verifies the password and issues the same access + refresh tokens the system already uses.

Concretely:

1. `users` gains a `password_hash` column (Argon2id). No plaintext, ever.
2. `POST /api/v1/auth/login {email, password}` verifies the hash and, on success, issues the **existing** access JWT + refresh token and sets the **existing** cookies. It replaces the OIDC `/auth/login` redirect **and** `/auth/callback`.
3. The Portal `/login` page becomes a **real** form (email + password) posting to that endpoint. The frontend `middleware` gates guests to `/login` (Portal), not to Authentik.
4. Account creation is `POST /api/v1/auth/register {email, password, display_name}` (or admin-provisioned) — the upsert-from-OIDC path is retired.
5. Password reset (`forgot`/`reset` with an emailed token) is added when the notification module lands; until then, admin-set or a CLI reset.
6. **Authentik is dropped from the dev stack** (frees ~1 GB RAM + its Postgres). The OIDC provider blueprint, `auth/oidc.go`, the `/auth/callback` handler, `user_oidc_roles` sync, and `OIDC_*` config become dead and are removed.

#### Architecture model

##### Login flow (Luồng B)

```mermaid
sequenceDiagram
    actor U as User (Browser)
    participant F as Frontend<br/>portal.localhost
    participant A as API<br/>api.portal.localhost
    participant DB as Postgres

    U->>F: GET / (no session cookie)
    F-->>U: 307 → /login
    U->>F: GET /login (real Portal form)
    Note over U: ★ types email + password ON PORTAL ★
    U->>A: POST /api/v1/auth/login {email, password}
    A->>DB: SELECT user by email
    A->>A: argon2 verify password; check disabled_at
    A->>DB: INSERT refresh_token (sha256 hash)
    A-->>U: Set-Cookie portal_access (JWT) + portal_refresh; 200
    U->>F: GET / (with cookie)
    F-->>U: home (authenticated)
    Note over A,DB: No Authentik anywhere in the flow.
```

Compare with the OIDC flow (retired): the browser detoured through `auth.portal.localhost`, the API never saw the password, and identity came back as an ID token. Here the password is posted straight to the API and checked against `users.password_hash`.

##### What changes

| Layer | OIDC (retired) | Local auth (this ADR) |
| --- | --- | --- |
| Credential store | Authentik | `users.password_hash` (Argon2id) in Portal Postgres |
| Login screen | Authentik flow page (`auth.portal.localhost`) | Portal `/login` form (`portal.localhost`) |
| API `/auth/login` | 302 redirect to IdP + `/auth/callback` code exchange | `POST {email,password}` → verify → issue tokens |
| User provisioning | `UpsertUserFromOIDC` on callback | `POST /auth/register` (or admin) |
| Config | `OIDC_ISSUER/CLIENT_ID/SECRET/REDIRECT_URL` | none (removed) |
| Extra infra | authentik-server + worker + its Postgres + blueprint | **none** |
| Who sees the password | only Authentik | Portal API (transiently, then discarded to a hash) |

##### What is reused (unchanged)

The hard, security-sensitive parts of the account module **do not change** — this is why the switch is contained:

- **Access tokens** — HS256 JWT with rotating `kid`, `token_version`, roles (`auth.Issuer`/`Verifier`).
- **Refresh tokens** — 256-bit, SHA-256 at rest, rotation chain + reuse detection (`auth.RefreshManager`, `refresh_tokens` table).
- **Two-channel revocation** — `users.token_version` (logout-all) + `refresh_tokens.revoked_at`.
- **RBAC** — role hierarchy, recursive-CTE effective permissions, Redis cache keyed by `token_version` (unchanged; roles now assigned by Portal, not synced from Authentik groups).
- **Cookies** — `portal_access` (Path=/) + `portal_refresh` (Path=/api/v1/auth), `HttpOnly Secure SameSite=Strict`, domain `portal.localhost`.
- **Audit log**, `/auth/refresh`, `/auth/logout`, `/auth/logout-all`, `/auth/me`.

Only the **identity-proof step** at `/auth/login` and account creation change.

##### New responsibilities Portal now owns

Delegating to Authentik gave these for free; local auth means implementing them:

- **Password hashing** — Argon2id (`golang.org/x/crypto/argon2`), sane params (e.g. 64 MB, t=3, p=2), per-user salt; constant-time verify.
- **Brute-force defence** — rate-limit `/auth/login` per IP + per account; exponential backoff / temporary lockout on repeated failures.
- **Password policy** — min length / breach check on register + reset.
- **Password reset** — emailed single-use token (needs the notification module, Phase 6) or CLI/admin reset until then.
- **MFA / step-up** (later, for the bank module) — TOTP enrolment + `acr`/`amr`-equivalent claims must be built in Portal (previously Authentik-managed, [D-27]/[D-28]).
- **Social login** ("Login with Google") — implement Google OAuth directly in Portal (previously a one-line Authentik source).

#### Options considered

Recorded fully in the discussion that preceded this ADR; summarised:

- **A — Keep OIDC, brand Authentik (Mức 1/2).** One branded login form, but hosted on Authentik; Portal never owns credentials; Google/MFA/step-up come free. *Rejected* for the "form must live on Portal" requirement.
- **B — Local password auth *(chosen)*.** Login form on Portal, no redirect, Portal owns credentials. Portal must build the security surface Authentik provided.
- **C — Hybrid (local login, keep Authentik for MFA/social).** Most complex; two identity systems to reconcile. *Deferred* — revisit if MFA/social become required and local-only proves insufficient.

#### Trade-off analysis

The decisive trade is **UX/ownership vs. security-surface-you-maintain**. Authentik existed precisely to own passwords, MFA, lockout, reset, and social federation — battle-tested. Moving in-house buys a single native login form and drops ~1 GB of infra, at the cost of re-implementing (and being responsible for) that security surface. For a single-operator v1 demo the surface is small; the risk grows when the **bank module** (which the corpus says needs step-up + MFA, [D-27]/[D-28]) and **social login** arrive — those were the original reasons OIDC was chosen. This ADR accepts that future cost in exchange for the desired UX now, and leaves Option C open as the escape hatch (add Authentik back purely as an MFA/social provider, keeping local password as the primary factor).
<!-- /adr-narrative -->

#### Consequences

As built. The account module's full contract is this spec — route table §7,
login and throttle P0.2, hashing / tokens / cookies / TTLs P0.3 — and its open
gaps are §11; what follows is only what the decision changed and what has been
learned since.

- **Route table:** `/auth/login {email, password, remember}`, `/auth/register`
  (201, no session — the user returns to `/login`), `/auth/forgot-password`,
  `/auth/reset-password`, plus the unchanged `/auth/refresh`, `/auth/logout`,
  `/auth/logout-all`, `/auth/me` (`account/module.go` `MountHTTP`).
  `/auth/callback` is gone from both code and `shared/openapi.yaml`.
- **Departures from the Decision text:** three cookies, not two —
  `portal_session` is the marker the Next.js middleware gates on — and a
  `remember` flag selects persistent vs session cookies (P0.3); the refresh
  TTL is 24 h (`REFRESH_TOKEN_TTL`, `platform/config`) and nothing in the
  shipped code ever defaulted to 30 days.
- **Password reset shipped** with migration `0010_account_password_reset_tokens`
  and the notify module ([SPEC-05](SPEC-05-notification-module.md) P0.3) — the
  "admin/CLI until then" interim is over.
- **Registration requires approval** (migration `0031`, not part of this
  decision): only `approved` may hold a session, the first account on an empty
  database is the founder, `BOOTSTRAP_SUPERADMIN_EMAIL` covers an existing
  install (P0.1, P0.2, P0.5, P0.12). That is the shipped state; Decision
  2026-10-02 (A2) removes the founder rule in favour of a pre-created
  Superadmin who must change the password on first sign-in — a target,
  §11 row 26.
- **`/auth/me` returns effective permission codes** so the frontend can hide
  what the API would refuse (P0.6; added when roles became editable).
- One login screen, served by Portal, no cross-domain redirect. The dev stack
  lost authentik-server + authentik-worker + authentik-postgres + the
  blueprint and the container→IdP networking hack (Traefik alias +
  `SSL_CERT_FILE`).

**What became harder, as predicted:**

- Portal is a credential custodian. The brute-force guard on `/auth/login` is
  live (P0.2: 5 failures per 15 minutes per IP and per email, Redis-backed,
  `handler/auth.go` `loginThrottled`; the per-email key becomes per-(email,
  IP) by Decision 2026-10-02 (A4), §11 row 28). It is the **only** throttle —
  `/auth/register` and `/auth/refresh` are unthrottled and the per-IP key
  trusts any `X-Forwarded-For` (§11 rows 3–4).
- **MFA / step-up** ([D-27]/[D-28]) and **"Login with Google"** are still not
  built. `account/module.go`'s package comment mentions "2FA/TOTP" and
  `api/api.go` reserves `totp_*` as a sensitive field, but no migration adds
  such columns and no route implements enrolment or verification. The ledger
  shipped ([SPEC-12](SPEC-12-finance-ledger.md)) without step-up; the re-entry
  condition is [backlog.md § Deferred](../backlog.md).

**Revisited:**

- [ADR-02](#adr-02) — `user_oidc_roles` dropped by migration `0006`; roles are
  Portal-assigned only (D-26.r1). ADR-02's Context says so; its composition
  rule keeps the historical term.
- Option C (Authentik back as a second-factor / social IdP) remains the escape
  hatch if MFA/social prove heavy to self-build. Not exercised.

#### Action items

Items 1–9 are done: migration `0006_account_local_auth` (`password_hash`,
`password_updated_at`, `user_oidc_roles` dropped, `oidc_subject` nullable);
`auth/password.go` Argon2id; the `GetUserByEmail` / `CreateLocalUser` /
`SetUserPassword` queries; `POST /auth/login` + `/auth/register` with
refresh/logout/logout-all/me kept; the login rate-limit + lockout (only there —
see Consequences); the real `/login` form with the middleware gating guests to
it and the SSO/Google buttons removed; OIDC removed (`auth/oidc.go`, callback,
`OIDC_*` config, the Authentik services and blueprint, the Traefik alias and
`SSL_CERT_FILE` override); docs synced (`/CLAUDE.md` Account section,
[security.md](../../architecture/security.md) (then `authoration.md`),
[feature-inventory.md](../feature-inventory.md) §1, `shared/openapi.yaml` with
`/auth/register` and no `/auth/callback` — the Vietnamese mirror the item also
named was deleted with `docs/archive/` in `f11cf3f`); and password reset (`0010`,
SPEC-05), which made the interim admin/CLI reset unnecessary.

10. [ ] MFA/TOTP enrolment + verification, and step-up for bank
    ([D-27]/[D-28]) — not built; deferred (P2, backlog § Deferred).
11. [ ] "Login with Google" as a local OAuth flow — not built.

<a id="adr-07"></a>
### ADR-07 — Multi-tenancy & Row-Level Security model

**Decided:** 2026-07-07 · **Status:** accepted, executed 2026-08-25 (plan steps 1–4, 8, 10); steps 5–7 deferred by scope

Deciders: kirito. Relates to [ADR-01](README.md#adr-01) (v1 cut), [ADR-02](#adr-02)
(RBAC), [ADR-03](SPEC-03-platform-ops.md#adr-03) (single VPS),
[feature-inventory.md](../feature-inventory.md) §2 + §18 Phase 1, [D-23] [D-24]
[D-25]; runbook [operations/rls-cutover.md](../../operations/rls-cutover.md).
The `tenant` module has no spec of its own, so this record is its decision
record; the mechanism as it runs today — tenant model, schema, `tenant_id`
propagation, the three GUCs, request and worker paths, roles and the RLS test
suite — is [security.md §3](../../architecture/security.md#3-tenant-layer-data-segregation),
and this record does not repeat it.

#### Context

*As found on 2026-07-07, when this was a plan. Everything the plan needed from
the schema has since shipped (`0018_tenant_core`, `0019_platform_rls_roles`,
`0020_platform_rls_enable`, and every tenant-scoped table since carries its own
policy). Two premises below have moved: Postgres no longer runs in compose
(host cluster since 2026-08-21) and PgBouncer is gone with it, so "the
PgBouncer constraint" no longer binds — but the transaction-local GUC design it
forced is what shipped, and it is correct without a pooler too. Where RLS is
actually enforced today is under Consequences; read that before trusting any
other statement about RLS in this repo.*

`feature.md §2` and the Phase 1 roadmap wanted multi-tenancy — `organizations`
+ `memberships` — with **Postgres Row-Level Security (RLS) as
defense-in-depth** (`architecture/security.md`'s L2): even a query that forgets
`WHERE tenant_id = ?` must not leak cross-tenant data. v1 had deferred all of
it — there was **no `tenant_id` column anywhere** at the time, and the app was
effectively single-user.

Three forces shaped the design:

1. **Security posture.** The whole point of RLS is that the *database*, not the handler, is the last line. App-layer scoping alone is one forgotten filter away from a breach.
2. **The PgBouncer constraint (load-bearing at the time).** Prod was to pool through **PgBouncer in transaction mode** ([ADR-03](SPEC-03-platform-ops.md#adr-03)). RLS needs a per-request session variable; transaction-mode pooling reuses one connection across many transactions, so the naive "`SET app.tenant_id` once per connection" leaks one request's tenant into the next. (Dev sidestepped this by connecting **direct to `postgres:5432`**, because pgx's prepared-statement cache also clashes with transaction-mode PgBouncer.)
3. **Personal vs. org data.** Most Portal data is *personal* (a user's uploads, feed, bank); only some is org-shared. A **synthetic personal tenant** per user lets every row carry a `tenant_id` and keeps a single code path for both.

<!-- adr-narrative -->
#### Decision

Adopt **PostgreSQL RLS keyed on a per-request GUC, set with `SET LOCAL` inside a per-request transaction, on a non-owner application role with `FORCE ROW LEVEL SECURITY`.** Model tenancy as `organizations` (with a `kind` discriminator `'org' | 'household' | 'personal'`) + `organization_memberships`; give every user a synthetic **personal** org. Run cross-tenant batch work as a separate **`BYPASSRLS`** role, isolated to `cmd/sysjobs` by depguard.

> **GUC name:** the tenant skeleton already references `app.current_tenant`; the Phase-1 roadmap wrote `app.tenant_id`. **Pick one and use it everywhere** — this ADR standardises on **`app.current_tenant`** (matches the shipped skeleton comment). Fix `feature.md §18`'s `app.tenant_id` to match when Phase 1 lands.

##### 1. Data model

- `organizations(id, kind, slug, name, owner_id → users(id), created_at, updated_at)` — `kind ∈ {'org','household','personal'}` **from day one** [D-24]; adding household later must not migrate a populated table.
- `organization_memberships(org_id, user_id, role, granted_at, ...)` — user ↔ tenant with a scoped role. Role granularity differs per kind: orgs → full RBAC hierarchy; households → `owner` + `member` (soft cap ~6); personal → single `owner`.
- **Every user gets a `personal` org at signup** (`kind='personal'`, `owner_id=user`, one owner membership). Personal routes (`/t/me/...`) resolve `me` → that org's id.
- **`users` stays GLOBAL** — a person is one identity across orgs (authoration.md). No `tenant_id` on `users`.
- **Tenant-scoped tables** carry `tenant_id UUID NOT NULL REFERENCES organizations(id)`: future domain tables (movie/music/story/comic), bank, social — **and `media.assets` gains `tenant_id`** (an upload belongs to the tenant context it was made in; `me` for personal). RBAC tables (`roles`, `permissions`) stay global; `user_roles` becomes membership-scoped (see §4).

##### 2. RLS enforcement (per tenant-scoped table)

- A dedicated **app role `portal_app`** (`NOSUPERUSER NOBYPASSRLS`) that the API/worker connect as. **Critical:** superusers *and the table owner* bypass RLS unless `FORCE` is set — so the app role must **not** own the tables (own them as a migration/admin role, run as `portal_app`).
- Each tenant-scoped table, in the **same migration that creates it**:
  ```sql
  ALTER TABLE movies ENABLE ROW LEVEL SECURITY;
  ALTER TABLE movies FORCE  ROW LEVEL SECURITY;      -- applies even to the owner
  CREATE POLICY tenant_isolation ON movies
    USING      (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
  ```
  `USING` filters read/update/delete; `WITH CHECK` blocks writing a row into *another* tenant.
- **Fail closed:** `current_setting('app.current_tenant')` with no GUC set raises an error — a query that forgot to open a tenant scope *errors* rather than leaking. Use the 2-arg `current_setting(..., true)` (returns NULL) only where "no tenant ⇒ deny" is handled explicitly.

##### 3. Connection strategy (the crux)

RLS-per-request under PgBouncer transaction pooling:

- **`SET LOCAL app.current_tenant = $1` inside a transaction.** `SET LOCAL` is transaction-scoped and reset at `COMMIT`/`ROLLBACK`, so a pooled connection never carries one request's tenant into the next. Plain `SET` (session-scoped) is **wrong** under transaction pooling.
- **Every tenant-scoped request runs in one transaction.** `platform/db.BeginTenantScope(ctx, tenantID)` opens `BEGIN; SET LOCAL app.current_tenant = $1;`, hands the tx to the request's queries, and `COMMIT`s (auto-`ROLLBACK` on handler error).
- **pgx ⨯ PgBouncer transaction mode** is incompatible with pgx's default prepared-statement caching → set the pool's `DefaultQueryExecMode = QueryExecModeExec` (or `SimpleProtocol`) / disable the statement cache. (This clash is why dev connects direct today.) Phase 1 chooses one of:
  - **(a) Route through PgBouncer (recommended)** — transaction mode + simple/exec protocol + `SET LOCAL` per request. Keeps the connection pool.
  - **(b) Direct-to-Postgres fallback** — keep `postgres:5432`, drop PgBouncer for the app; acceptable only while connection count stays well under `max_connections`.

##### 4. Tenant resolution & RBAC

- URL scheme `/api/v1/t/{tenant}/...`; `{tenant}` = an org slug or the literal `me` [D-23].
- **Middleware order:** `RequireAuth` → **`RequireTenant`** (resolve `{tenant}` → tenant_id; `me` → caller's personal org; **verify the caller has a membership**, else 403) → `BeginTenantScope(ctx, tenant_id)` for the request tx → module handler.
- **Single-tenant deployments** map `/api/v1/...` (no `/t/`) to a default tenant via Traefik/middleware — the common case isn't uglier.
- **RBAC composes per-tenant:** effective permissions are computed **within the active membership** (admin in org A, member in org B). `user_roles` becomes `(user_id, org_id, role_id)`; the RBAC cache key gains the tenant ([ADR-02](#adr-02)). Roles/permissions catalogs stay global.

##### 5. Cross-tenant batch (BYPASSRLS)

- `cmd/sysjobs` connects as a **separate `portal_sys` role (`BYPASSRLS`)** for cross-tenant maintenance (purges, migrations, aggregate reports) via `internal/sysrepository`. **depguard blocks every other package from importing `sysrepository`** — a BYPASSRLS path reachable from the API would defeat RLS entirely (CLAUDE.md).

##### Architecture model — request path

```mermaid
sequenceDiagram
    actor U as Client
    participant MW as API middleware
    participant DB as Postgres (portal_app role, RLS FORCEd)

    U->>MW: GET /api/v1/t/acme/movies  (cookie)
    MW->>MW: RequireAuth → identity
    MW->>DB: RequireTenant: resolve slug 'acme' + verify membership
    MW->>DB: BeginTenantScope: BEGIN; SET LOCAL app.current_tenant = '<acme-id>'
    MW->>DB: SELECT * FROM movies         (no WHERE tenant_id needed)
    Note over DB: RLS policy filters to tenant_id = current_setting('app.current_tenant')
    DB-->>MW: only acme's rows
    MW->>DB: COMMIT   (SET LOCAL discarded; connection safe to reuse)
    MW-->>U: 200
    Note over MW,DB: A forgotten filter still can't leak — the DB enforces it.
```

#### Options considered

- **A — RLS + per-request `SET LOCAL` GUC *(chosen)*.** DB-enforced isolation; a buggy query can't leak. Cost: every tenant-scoped request is a tx; PgBouncer/pgx config care.
- **B — App-layer scoping only (`WHERE tenant_id = ?`), no RLS.** Simplest, no GUC/tx dance. **Rejected** — one forgotten filter = cross-tenant leak, exactly the failure RLS exists to prevent; unacceptable for bank/private data.
- **C — Schema-per-tenant / DB-per-tenant.** Hard isolation, but migration/ops cost explodes with many small *personal* tenants. **Rejected** for Portal's per-user tenant shape.
- **D — Connection-per-tenant with session `SET`.** Needs session-mode pooling → kills PgBouncer transaction-mode efficiency and blows up connection count. **Rejected.**

#### Trade-off analysis

RLS + `FORCE` + fail-closed GUC is the strongest containment for the least code — but it imposes two rules every dev must internalise: **(1)** tenant-scoped queries run *inside* `BeginTenantScope`, and **(2)** the app connects as a **non-owner** role. The tx-per-request + simple-protocol is a real but bounded perf/complexity tax (measure it with the observability profile, which [ADR-03] says should land the same sprint). The synthetic `personal` tenant trades one `organizations` row per user for a single, fork-free code path across personal and org data.
<!-- /adr-narrative -->

#### Consequences

**Where RLS stands — the one statement to trust.** RLS is enforced **if and
only if the binary's `DATABASE_URL` connects as `portal_app`**. The policies
exist on every tenant-scoped table
(`grep -ho 'ALTER TABLE [a-z_]* FORCE ROW LEVEL SECURITY' backend/db/migrations/*.up.sql | sort -u | wc -l`)
and `portal_app` is `NOSUPERUSER NOBYPASSRLS` and does not own them, so
`FORCE` binds; `portal`, the migration role, is a superuser and bypasses every
policy. This deployment's `.env` has run as `portal_app` since the cutover of
2026-08-25 (`grep DATABASE_URL .env`; [runbook](../../operations/rls-cutover.md)),
and since 2026-09-11 `.env.example` defaults to it too, so a fresh `make up`
starts enforced; `portal` is used only by `MIGRATE_DATABASE_URL` and
`BACKUP_DATABASE_URL`. Which role does what, the GUCs, and the test suite that
proves isolation (run on every push by the `backend` CI job) are
[security.md §3.4](../../architecture/security.md#34-postgresql-rls--the-enforcement-mechanism--built).

**What shipped, against the plan:**

- **Rule kept, and written down:** every migration that creates a tenant-scoped
  table carries `ENABLE + FORCE` RLS and its policy in the same file (the
  migrations creating global tables, such as `0036_layout_core`, carry none,
  by design). The rule is binding in [backend/MODULES.md](../../../backend/MODULES.md)
  §6 ("Tenant-scoped tables carry RLS from birth") and §8 step 4, and in the
  specs README Tenancy convention.
- `media.assets` gained `tenant_id` with backfill and policy (`0020`); `users`
  is global; `user_roles` did **not** gain `org_id` — RBAC is not
  tenant-scoped (P0.7, §6).
- Migration numbers: the plan's `0008`/`0009` became `0018_tenant_core` /
  `0020_platform_rls_enable`, with `0019_platform_rls_roles` between them.
- GUC name: `app.current_tenant` in every migration and in `platform/db`.
  `feature-inventory.md` still writes `app.tenant_id` in three deliverable
  bullets under a note that says this decision's name supersedes it — legible,
  not fixed (`grep -n 'app.tenant_id' docs/product/feature-inventory.md`).
- `platform/db` provides `NewPool` (with `QueryExecModeExec` — which is also
  why `[]byte` into a `jsonb` column needs the Go type, not a cast,
  `jsonb_param_test.go`), `BeginScope` / `BeginTenantScope`, `WithTx` /
  `TxFrom`, `RunInTx` and `Conn` (a sqlc `DBTX` that routes each query onto
  the request transaction when one is bound). The GUCs are set
  transaction-locally with `set_config(..., true)` rather than a literal
  `SET LOCAL`; the effect is the same.
- **Tenant resolution as built:** no `/t/{tenant}` URL prefix. Routes stay
  under `/api/v1`; `RequireTenant` (tenant module middleware) resolves the
  caller's personal org and opens the request transaction. Middleware order is
  `RequireAuth → RequireTenant → handler`, as designed; only the URL contract
  differs. The account surface itself runs ahead of `RequireTenant` (P0.5).
- **A constraint violation raised inside a tenant transaction surfaces at
  COMMIT.** For a write, `RequireTenant` buffers the response until COMMIT
  succeeds, so a failed commit becomes a 500, not a 409. The house pattern is
  `ON CONFLICT DO NOTHING` + treat no-rows as conflict, or a pre-check before
  an UPDATE.
- **Cross-tenant batch without BYPASSRLS:** `cmd/worker` runs periodic sweeps
  through `forEachTenant` (one committed tenant scope per organisation) —
  `people:scan_birthdays`, `bank:scan_debts_due`, `media:purge_orphans`. That
  removed the need for `portal_sys`, `cmd/sysjobs` and
  `internal/sysrepository` so far; the role exists (`0019`), the binary and
  package do not, and depguard keeps the guardrail for when they land.
- Two comments in the tree still describe the pre-cutover state as current and
  should be read as history: the headers of migrations `0019` and `0020`
  ("**INERT** until …"). Applied migrations are not edited; they are corrected
  in the next migration that touches those tables. The `platform/db/db.go`
  package comment has been corrected.
- The observability profile did not land with tenancy
  ([ADR-03](SPEC-03-platform-ops.md#adr-03)).

#### Action items (implementation plan)

Done: (1) DB roles `portal_app` (`NOBYPASSRLS`) + `portal_sys` (`BYPASSRLS`) in
`0019`, tables owned by `portal`, runtime cutover to `portal_app` on
2026-08-25; (2) `0018_tenant_core` — `organizations` (+`kind`) and
`organization_memberships`, a personal org and owner membership backfilled for
every existing user, and for new users created lazily on their first
tenant-scoped request (`RequireTenant` → `GetOrCreatePersonalOrg` →
`CreatePersonalOrg`), never at registration; (3) `platform/db.BeginTenantScope`
+ pool config (`QueryExecModeExec`) — the PgBouncer branch is moot, the app
connects direct to the host cluster; (4) `0020_platform_rls_enable` — `ENABLE +
FORCE` + `tenant_isolation` on every tenant-scoped table, `assets.tenant_id` +
backfill + policy; (8) the RLS isolation tests (`rls_test.go`,
`rls_media_test.go`, `rls_social_test.go`) and the MODULES.md §6/§8 entry;
(10) `.env.example`'s `DATABASE_URL` defaults to `portal_app` (2026-09-11),
with `MIGRATE_DATABASE_URL` for the owner DSN.

5. [ ] `tenant` module beyond the personal org: `GET /me/organizations` exists;
   `POST /auth/switch-tenant` and `/admin/organizations` do not
   ([security.md §3.5–3.6](../../architecture/security.md#35-tenant-switching--target--not-built)).
   Deferred at one user, one personal org.
6. [ ] Per-tenant RBAC (`user_roles(user_id, org_id, role_id)`, cache key
   scoped to membership) — not done; RBAC is global. Deferred with 5.
7. [ ] `cmd/sysjobs` + `internal/sysrepository` — not written; `forEachTenant`
   has made it unnecessary so far. The depguard rule stays.
9. [ ] Observability profile — not landed. `feature-inventory.md` GUC bullets —
   still `app.tenant_id` under a superseding note.

**Exit (as built):** a request under `/api/v1/…` is tenant-scoped end-to-end
through `RequireTenant`; a raw query on `portal_app` cannot read another
tenant's rows (tested, in CI since 2026-09-11); there is no `/t/` prefix to make
optional; cross-tenant work goes through `forEachTenant` in the worker, not a
BYPASSRLS role.
