# ADR-06: Local password auth — Portal owns credentials (drop Authentik from the login path)

**Status:** Accepted
**Date:** 2026-07-05
**Deciders:** kirito
**Supersedes:** the OIDC-login decision in [ADR-05](./05-phase0-wiring-order.md) (Milestone 0.4) and the "No local password auth. OIDC via Authentik" statement in [CLAUDE.md](../../../CLAUDE.md) (Account module).

## Update (2026-07-06) — implemented

- All endpoints live: `POST /auth/login {email, password, remember}` (Redis brute-force rate-limit + lockout), `POST /auth/register` (returns 201, no session — user returns to `/login`), plus the unchanged `/auth/refresh`, `/auth/logout`, `/auth/logout-all`, `/auth/me`. Argon2id at 64 MB / t=3 / p=2, PHC format.
- Deltas from the spec below: a `remember` flag selects persistent (24h) vs session cookies; a third cookie `portal_session` (Path=/, marker read by the Next.js middleware gate) accompanies `portal_access`/`portal_refresh`; current `.env` sets `ACCESS_TOKEN_TTL=5m` and `REFRESH_TOKEN_TTL=24h` (not 30d).
- OIDC code, the Authentik compose services, blueprints, and `OIDC_*` env are fully deleted.
- Remaining drift: `shared/openapi.yaml` still lists the retired `/auth/callback` and lacks `/auth/register`.

## Context

The OIDC-via-Authentik design (built and wired in the Phase-0 sprint) authenticates the user **at the IdP**: the browser is redirected from Portal to Authentik, the user types credentials on Authentik's page, and Authentik returns an authorization code that the API exchanges for the user's identity.

That is architecturally clean, but produced a **user-experience objection**: the login always leaves the Portal domain for `auth.portal.localhost`, and the Portal login form we built was decorative (only the SSO button worked) — so users saw "two forms". Branding Authentik and a straight-to-IdP redirect ("Mức 2") reduced this to one branded form, but the login screen is still **served by, and hosted on, Authentik**, not Portal.

The product owner wants the login form to live **on Portal itself**, with Portal verifying the password directly — no redirect, no separate identity service in the login path. This ADR records that decision and its architecture.

> This reverses a prior decision. [ADR-05](./05-phase0-wiring-order.md) chose OIDC and [ADR-02](./02-rbac-model-reconciliation.md) assumed Authentik-synced roles. The **token, refresh, RBAC, revocation, and audit machinery are unaffected** — only the *front door* (how a user proves identity) changes. See §"What is reused".

## Decision

**Portal authenticates users locally against its own `users` table with a hashed password. Authentik is removed from the login path.** The login form is served by the Portal frontend; the API verifies the password and issues the same access + refresh tokens the system already uses.

Concretely:

1. `users` gains a `password_hash` column (Argon2id). No plaintext, ever.
2. `POST /api/v1/auth/login {email, password}` verifies the hash and, on success, issues the **existing** access JWT + refresh token and sets the **existing** cookies. It replaces the OIDC `/auth/login` redirect **and** `/auth/callback`.
3. The Portal `/login` page becomes a **real** form (email + password) posting to that endpoint. The frontend `middleware` gates guests to `/login` (Portal), not to Authentik.
4. Account creation is `POST /api/v1/auth/register {email, password, display_name}` (or admin-provisioned) — the upsert-from-OIDC path is retired.
5. Password reset (`forgot`/`reset` with an emailed token) is added when the notification module lands; until then, admin-set or a CLI reset.
6. **Authentik is dropped from the dev stack** (frees ~1 GB RAM + its Postgres). The OIDC provider blueprint, `auth/oidc.go`, the `/auth/callback` handler, `user_oidc_roles` sync, and `OIDC_*` config become dead and are removed.

## Architecture model

### Login flow (Luồng B)

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

### What changes

| Layer | OIDC (retired) | Local auth (this ADR) |
| --- | --- | --- |
| Credential store | Authentik | `users.password_hash` (Argon2id) in Portal Postgres |
| Login screen | Authentik flow page (`auth.portal.localhost`) | Portal `/login` form (`portal.localhost`) |
| API `/auth/login` | 302 redirect to IdP + `/auth/callback` code exchange | `POST {email,password}` → verify → issue tokens |
| User provisioning | `UpsertUserFromOIDC` on callback | `POST /auth/register` (or admin) |
| Config | `OIDC_ISSUER/CLIENT_ID/SECRET/REDIRECT_URL` | none (removed) |
| Extra infra | authentik-server + worker + its Postgres + blueprint | **none** |
| Who sees the password | only Authentik | Portal API (transiently, then discarded to a hash) |

### What is reused (unchanged)

The hard, security-sensitive parts of the account module **do not change** — this is why the switch is contained:

- **Access tokens** — HS256 JWT with rotating `kid`, `token_version`, roles (`auth.Issuer`/`Verifier`).
- **Refresh tokens** — 256-bit, SHA-256 at rest, rotation chain + reuse detection (`auth.RefreshManager`, `refresh_tokens` table).
- **Two-channel revocation** — `users.token_version` (logout-all) + `refresh_tokens.revoked_at`.
- **RBAC** — role hierarchy, recursive-CTE effective permissions, Redis cache keyed by `token_version` (unchanged; roles now assigned by Portal, not synced from Authentik groups).
- **Cookies** — `portal_access` (Path=/) + `portal_refresh` (Path=/api/v1/auth), `HttpOnly Secure SameSite=Strict`, domain `portal.localhost`.
- **Audit log**, `/auth/refresh`, `/auth/logout`, `/auth/logout-all`, `/auth/me`.

Only the **identity-proof step** at `/auth/login` and account creation change.

### New responsibilities Portal now owns

Delegating to Authentik gave these for free; local auth means implementing them:

- **Password hashing** — Argon2id (`golang.org/x/crypto/argon2`), sane params (e.g. 64 MB, t=3, p=2), per-user salt; constant-time verify.
- **Brute-force defence** — rate-limit `/auth/login` per IP + per account; exponential backoff / temporary lockout on repeated failures.
- **Password policy** — min length / breach check on register + reset.
- **Password reset** — emailed single-use token (needs the notification module, Phase 6) or CLI/admin reset until then.
- **MFA / step-up** (later, for the bank module) — TOTP enrolment + `acr`/`amr`-equivalent claims must be built in Portal (previously Authentik-managed, [D-27]/[D-28]).
- **Social login** ("Login with Google") — implement Google OAuth directly in Portal (previously a one-line Authentik source).

## Options considered

Recorded fully in the discussion that preceded this ADR; summarised:

- **A — Keep OIDC, brand Authentik (Mức 1/2).** One branded login form, but hosted on Authentik; Portal never owns credentials; Google/MFA/step-up come free. *Rejected* for the "form must live on Portal" requirement.
- **B — Local password auth *(chosen)*.** Login form on Portal, no redirect, Portal owns credentials. Portal must build the security surface Authentik provided.
- **C — Hybrid (local login, keep Authentik for MFA/social).** Most complex; two identity systems to reconcile. *Deferred* — revisit if MFA/social become required and local-only proves insufficient.

## Trade-off analysis

The decisive trade is **UX/ownership vs. security-surface-you-maintain**. Authentik existed precisely to own passwords, MFA, lockout, reset, and social federation — battle-tested. Moving in-house buys a single native login form and drops ~1 GB of infra, at the cost of re-implementing (and being responsible for) that security surface. For a single-operator v1 demo the surface is small; the risk grows when the **bank module** (which the corpus says needs step-up + MFA, [D-27]/[D-28]) and **social login** arrive — those were the original reasons OIDC was chosen. This ADR accepts that future cost in exchange for the desired UX now, and leaves Option C open as the escape hatch (add Authentik back purely as an MFA/social provider, keeping local password as the primary factor).

## Consequences

**What becomes easier**

- One login screen, served by Portal, no cross-domain redirect. The `middleware` gate simplifies to "no cookie → `/login`".
- Dev stack loses authentik-server + authentik-worker + authentik-postgres + the blueprint → ~1 GB RAM back, fewer moving parts, faster `make up`.
- No container→IdP networking hack (Traefik alias + `SSL_CERT_FILE`) and no OIDC discovery/JWKS/TLS-trust concerns.

**What becomes harder**

- Portal is now a credential custodian: a login-endpoint bug or weak hashing is a direct account-compromise vector. Rate-limiting and lockout are now must-haves, not nice-to-haves.
- Password reset needs email (notification module) — until then, recovery is manual (admin/CLI).
- The bank module's step-up/MFA ([D-27]/[D-28]) and "Login with Google" must be built in Portal, not configured in Authentik.

**What we'll need to revisit**

- [ADR-02](./02-rbac-model-reconciliation.md): `user_oidc_roles` and OIDC-group→role sync ([D-26]) no longer apply; roles are Portal-assigned only. Update the effective-permission composition note.
- If MFA/social later prove heavy to self-build, reconsider **Option C** (Authentik back as a second-factor / social IdP layered on top of local password).

## Action items (implementation plan — separate from this doc change)

1. [x] Migration: add `users.password_hash TEXT` (+ `password_updated_at`); drop `user_oidc_roles` (or leave dormant). Make `oidc_subject` nullable.
2. [x] `platform/crypto` (or `account/auth/password.go`): Argon2id hash + verify helpers.
3. [x] Queries/adapters: `GetUserByEmail`, `CreateUserLocal`, `SetPassword`.
4. [x] Handler: replace `Login`/`Callback` with `POST /auth/login {email,password}` + `POST /auth/register`; keep `refresh`/`logout`/`logout-all`/`me`.
5. [x] Rate-limit + lockout middleware on `/auth/login`.
6. [x] Frontend: real `/login` form → `POST /auth/login`; revert `middleware` to gate guests to `/login`; remove the SSO/Google buttons (or repoint Google to a future local Google-OAuth).
7. [x] Remove OIDC: `auth/oidc.go`, callback, `OIDC_*` config, Authentik services + blueprint from compose, Traefik alias + `SSL_CERT_FILE` override.
8. [ ] Docs: sync CLAUDE.md Account section, `doc/*/authoration.md`, `doc/*/feature.md §1`, and the vi mirror of this ADR. *(2026-07-06: partially done — CLAUDE.md and the vi mirror are synced; `shared/openapi.yaml` still lists the retired `/auth/callback` and is missing `/auth/register`.)*
