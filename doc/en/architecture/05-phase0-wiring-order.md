# ADR-05: Phase 0 wiring order — the critical path to a running demo

**Status:** Proposed
**Date:** 2026-05-24
**Deciders:** kirito
**Affects:** [cmd/api/main.go](../../../backend/cmd/api/main.go), [cmd/worker/main.go](../../../backend/cmd/worker/main.go), [backend/internal/modules/account/module.go](../../../backend/internal/modules/account/module.go), [backend/sqlc.yaml](../../../backend/sqlc.yaml), [backend/db/migrations/](../../../backend/db/migrations/)

## Context

CLAUDE.md states the blocker plainly:

> `cmd/api/main.go` still has a `TODO: mount OpenAPI-generated handlers` comment and does not yet call `account.New(...)` or any module's `MountHTTP`. The account module assembles its handler internally inside `backend/internal/modules/account/module.go`; the API binary just hasn't been taught to construct it. Wiring is deferred until repository adapters land.

> `internal/modules/*/repository/` directories exist but are empty. The interfaces consumed by the account module (`AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserUpserter`) need adapters around the sqlc-generated code once `make sqlc` runs.

Every v1 deliverable depends on closing this gap. The 2-week sprint cannot afford a wrong sequence — re-doing migrations after sqlc generation has run, for instance, costs the rest of a day.

[ADR-01](./01-v1-scope-cut.md)'s v1 cut keeps 8 Phase 0 items. This ADR puts them in execution order.

## Decision

**Phase 0 lands in 5 strictly-sequenced milestones over Days 1–6 of the sprint, before any feature work begins.** Each milestone ends with a concrete check the developer can run.

### Milestone 0.1 — Migration tree audit (Day 1, ~4 hours)

Before sqlc runs and freezes the schema, split `0001` per [D-18]:

```
0001_platform_init.up.sql          extensions (uuid-ossp, citext, pg_trgm), common helper functions
0002_account_users.up.sql          users (no role col; +locale +timezone +token_version +disabled_at)
0003_account_rbac.up.sql           roles, role_parents, permissions, role_permissions, user_roles,
                                   user_oidc_roles (per [D-26])
0004_account_sessions.up.sql       refresh_tokens (with parent_id, replaced_by_id, revoked_at)
0005_platform_audit.up.sql         audit_log (moved from account; per [D-25])
```

Each `up.sql` has a matching `down.sql`. The `assets` table (was in old `0001`) and any media tables are deferred — they don't ship in Milestone 0.x.

**Check:** `make migrate && make migrate-down && make migrate` runs clean. Document this as the v1 acceptance for migrations.

### Milestone 0.2 — sqlc generation + repository adapters (Day 2, ~6 hours)

1. Run `make sqlc` for the account block. Generated code lands in `backend/internal/modules/account/repository/`.
2. Write adapters that implement the interfaces account already declares:
   - `AuthSnapshotFetcher` — wraps `GetUserAuthSnapshot` (returns `id`, `token_version`, `disabled_at`).
   - `RefreshStore` — wraps `InsertRefreshToken`, `GetRefreshToken`, `RevokeRefreshTokenChain` (recursive CTE for theft detection).
   - `PermissionFetcher` — wraps `GetEffectivePermissions` (recursive role-ancestor walk).
   - `EventStore` — wraps `InsertAuditEvent`. Now in `platform/audit/`, not `account/audit/` (per Milestone 0.1 / [D-25]).
   - `UserUpserter` — wraps `UpsertOidcUser`, `SyncOidcRoles`.

Adapters are 1:1 with sqlc-generated functions; no business logic. They live in `backend/internal/modules/account/repository/adapter.go` (one file, alphabetical).

**Check:** `go build ./...` succeeds across all packages. No `// TODO: adapter` comments left in account.

### Milestone 0.3 — Construct the account module in `cmd/api/main.go` (Day 3, ~6 hours)

The wiring sequence in `cmd/api/main.go`:

```go
func main() {
    cfg := config.MustLoad()                                    // env loader
    logger := platformlog.New(cfg.Env)                          // stdout JSON in v1
    pgPool := db.MustOpen(ctx, cfg.DatabaseURL)                 // pgxpool
    cache := cache.NewDragonfly(cfg.RedisURL)                   // Dragonfly client
    asynqClient := jobs.NewClient(cfg.RedisURL)                 // Asynq producer

    auditLogger := audit.NewLogger(pgPool, logger)              // platform/audit (per [D-25])

    accountMod, err := account.New(account.Deps{
        DB:                 pgPool,
        Cache:              cache,
        Audit:              auditLogger,
        OIDCIssuerURL:      cfg.OIDCIssuerURL,
        OIDCClientID:       cfg.OIDCClientID,
        OIDCClientSecret:   cfg.OIDCClientSecret,
        OIDCRedirectURL:    cfg.OIDCRedirectURL,
        JWTSigningKeys:     cfg.JWTSigningKeys,                 // comma-separated, rotating kid
        CookieDomain:       cfg.CookieDomain,
        CookieSecure:       cfg.CookieSecure,
        BootstrapAdminSubs: cfg.BootstrapAdminOIDCSubjects,     // per [D-26]
        OIDCGroupRoleMap:   cfg.OIDCGroupRoleMap,
    })
    must(err)

    r := chi.NewRouter()
    r.Use(middleware.RealIP, middleware.RequestID, middleware.Recoverer)
    r.Use(middleware.Timeout(30 * time.Second))
    r.Use(corsMiddleware(cfg))                                  // configured per env
    r.Use(ratelimit.Middleware(cache))

    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/healthz", healthz(pgPool, cache))
        accountMod.MountHTTP(r)                                 // mounts /auth/*, /me/*
        // v1 stops here. Future modules append their MountHTTP under r.
    })

    server := &http.Server{Addr: ":8080", Handler: r}
    logger.Info("api listening", "addr", server.Addr)
    must(server.ListenAndServe())
}
```

The same shape applies to `cmd/worker/main.go` with `accountMod.RegisterTasks(asynqMux)` — account has no Asynq tasks in v1, so the call is a no-op, but the wiring scaffold is in place.

**Check:** `make up && go run ./cmd/api` (or `make dev`) starts. `curl http://localhost:8080/api/v1/healthz` returns 200 with `{"status":"ok","db":true,"cache":true}`.

### Milestone 0.4 — OIDC end-to-end (Day 4, ~6 hours)

With Authentik running in compose (per [ADR-03](./03-single-vps-topology.md)), the OIDC handshake from `diagrams.md` §5 must work:

1. Configure Authentik provider for Portal: client ID + secret, redirect URI `https://${APP_DOMAIN}/api/v1/auth/callback`, allow `openid profile email groups` scopes.
2. Set `OIDC_GROUP_ROLE_MAP=portal-admins:admin` and create a `portal-admins` group in Authentik.
3. Create your own user in Authentik, add to `portal-admins`, set `BOOTSTRAP_ADMIN_OIDC_SUBJECTS=<your-sub>`.
4. Browser flow: visit `${APP_DOMAIN}` → frontend redirects to `/api/v1/auth/login` → 302 to Authentik → log in → callback → `users` row created, `user_oidc_roles` populated, access + refresh cookies set → redirect to `/`.
5. `curl -b cookies.txt https://${APP_DOMAIN}/api/v1/me` returns the user payload.
6. `curl -b cookies.txt -X POST https://${APP_DOMAIN}/api/v1/auth/logout-all` bumps `token_version`; the next `/me` returns 401.

**Check:** above 6 steps work without manual SQL.

### Milestone 0.5 — Frontend server-only API client + RSC auth handoff (Day 5–6, ~10 hours)

Per [D-34]:

1. Create `frontend/src/lib/api-server.ts` with `import "server-only"`. Wraps `fetch` to read `cookies()` and inject `Cookie:` on outgoing API calls.
2. Create `frontend/src/lib/api-client.ts` (no server-only guard) for client-component fetches; uses `credentials: 'include'`.
3. Create `frontend/src/app/auth/refresh-and-return/route.ts` — receives `return_to=<path>` query param, calls `/api/v1/auth/refresh`, redirects to `return_to`.
4. Create a `<Sign in>` button on the index page that links to `/api/v1/auth/login`.
5. Create `/account/page.tsx` (RSC) that fetches `/api/v1/me` and renders the user. On 401, throws `redirect('/auth/refresh-and-return?return_to=/account')`.

**Check:** unauth user clicks Sign in, lands at Authentik, logs in, returns to `/account`, sees their email. Refreshing after access token expiry triggers the refresh-and-return flow once and lands back on `/account`.

### Milestone 0.6 — Reserve naming + minimal CI (parallel, ~3 hours)

These happen alongside but don't gate the milestones above:

- Add `notify:*` Asynq prefix reservation note to `backend/MODULES.md` §5.2 (per [D-1]).
- Add a minimal `.github/workflows/ci.yml` with just two jobs: `sqlc-drift` (`make sqlc && git diff --exit-code`) and `openapi-drift` (`make openapi && git diff --exit-code`). Skip lint/test/security for v1. Drift detection alone catches the most expensive class of bugs.
- Add `# v1 scope: ADR-01` comment header to `cmd/api/main.go`.

## Options considered

### Option A — sqlc first, then migrations  *(rejected)*

Generates code against an unaudited schema. Splitting migrations afterwards forces a regeneration that may rename functions/types, breaking adapters mid-sprint. Order matters — migrations are the contract sqlc reads.

### Option B — Construct modules before sqlc adapters exist  *(rejected)*

The account module's `New(Deps)` constructor requires the adapters as inputs. Stubbing them with no-ops to get the binary running is tempting but creates a "construct then re-wire" double-pass. Worse, it hides the adapter shape bugs (interface mismatches between what sqlc generates and what the account module wants) behind a green build.

### Option C — Skip Authentik in v1, hand-roll password auth  *(rejected)*

Saves ~1 GB RAM and 1 day of Authentik config. Costs 3 days of password storage + reset flow + email templates + lockout logic + recovery codes. Net loss; auth surface is exactly where security regressions cost the most. Authentik in compose is the right call for v1 even though it's heavy.

### Option D — Defer the migration split; rename inside one mega-migration  *(rejected)*

Tempting because there's no prod data yet. Costs nothing now, but introduces a "this migration is actually three migrations" cognitive tax forever. The split is cheap *only* before sqlc runs against it. After, it's expensive. Pay the cheap version. [D-18]

## Trade-off analysis

The order milestones 0.1 → 0.2 → 0.3 is non-negotiable: each depends on the previous (migrations → sqlc → adapters → module construction). Milestones 0.4 (OIDC) and 0.5 (frontend RSC auth) are parallel-ish — OIDC ends at "can curl /me with cookies", frontend starts at "browser does what curl just did". You could swap the order, but doing OIDC first gives you a working backend before touching the frontend, which is easier to debug because every problem is in one place at a time.

Milestone 0.6 (naming + CI) is small enough to drop in any gap, but doing it before Milestone 0.4 means the drift checks catch any sqlc or openapi regressions caused by 0.1–0.3 before they compound.

Total budget for Phase 0: ~35 hours, ~Days 1–6 of the sprint. That leaves Days 7–10 for the Phase 2 vertical slice (upload → transcode → playback), Days 11–12 for bugfixes + the deploy script, Days 13–14 for buffer.

## Consequences

**What becomes easier:**

- The "wire it" panic is over by Day 3. From then on every feature attaches to a working scaffold.
- The 7-step demo from [ADR-01](./01-v1-scope-cut.md) becomes architecturally trivial: 6 of 7 steps are in Milestone 0.4–0.5; the seventh (logout) is in Milestone 0.4.
- Future modules (movie, music, etc.) attach to `r.Route("/api/v1", ...)` exactly like account did — copy the pattern.

**What becomes harder:**

- The Day-1 migration audit *feels* like a tax when the goal is to ship a demo. It's the most expensive thing on the path to defer, though, so this ADR asks the developer to do the boring thing first.
- Authentik configuration is the wildcard. Budget extra time on Day 4 if you've never done it; the published Authentik OIDC provider recipe is straightforward but assumes Authentik is reachable from the browser (Traefik hostname routing must work for both Portal AND Authentik).

**What we'll need to revisit:**

- Milestone 0.5's frontend pieces are the minimum to demo. The full Zustand/TanStack/RHF boundary doc ([D-32]) and the RSC decision tree ([D-33]) are deferred per [ADR-01](./01-v1-scope-cut.md); add them in Phase 0.5.
- The skipped CI jobs (lint, test, security, multi-arch build) should land in Phase 0.5 before any external user touches the system.
- If Authentik adds >2 days of setup pain, reassess Option C (hand-rolled auth) — but only if there's clear runway loss. Don't reassess inside the sprint; finish OIDC and learn.

## Action items

1. [ ] Open 5 milestone issues in the tracker mirroring §1–§5 above; close each as its check passes.
2. [ ] Day 0 (planning): write down the env vars list (`DATABASE_URL`, `REDIS_URL`, `S3_*`, `OIDC_*`, `JWT_SIGNING_KEYS`, `COOKIE_*`, `OIDC_GROUP_ROLE_MAP`, `BOOTSTRAP_ADMIN_OIDC_SUBJECTS`) and populate `.env.example` so Day 1 doesn't stall on credentials.
3. [ ] Day 1 morning: write down the milestone-check command for each milestone in a `MILESTONE_CHECKS.md` scratchpad; tick them off as you go. Resist the urge to push to the next milestone before the previous check passes.
4. [ ] Day 4 (Authentik): block out a full afternoon. Authentik's first-time config is the highest-risk hour in the sprint.
5. [ ] End of Milestone 0.5: run the full 7-step demo from [ADR-01](./01-v1-scope-cut.md) §Decision. If it works, you're on track for v1.
