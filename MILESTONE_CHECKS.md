# Portal v1 — Milestone Checklist

Living scratchpad for the v1 demo loop. **Auth direction changed** ([ADR-06](doc/en/architecture/06-local-auth-model.md), 2026-07-05): the loop is now

> **local password sign-in → authenticated Next.js home → upload mp4 → R2/MinIO → worker transcodes to HLS → `assets.status = ready` → Vidstack playback → revocable logout.**

Authentik / OIDC is **removed** — Portal owns credentials (Argon2id). Scope is
[ADR-01](doc/en/architecture/01-v1-scope-cut.md). Tick as each check passes.

**Where we are:** the **v1 demo loop is closed** — local sign-in → home → upload
mp4 → worker transcodes to HLS → Vidstack playback → logout all work end-to-end.
Auth, the Olympus UI shell, storage (Phase 2), and the media slice (Phase 5) are
done and running. What's left is hardening: CI drift, tests, and the known-issues
cleanup below.

Stack status: 8 services up (postgres · pgbouncer · dragonfly · minio(+setup) ·
traefik · api · worker · frontend). DB migrations at **v6**. Committed at
`05b6cf7` ("newsfeed").

---

## ✅ Done

### Infra / stack
- [x] docker-compose: Postgres · PgBouncer · Dragonfly · MinIO (`./data/minio` + `minio-setup` bucket) · Traefik, healthchecks, frontend build args, local TLS (self-signed) via `docker-compose.override.yml`.
- [x] **Authentik fully removed** — services, own Postgres, blueprints (`authentik/`), Traefik `auth.` alias, `SSL_CERT_FILE` override, all `OIDC_*`/`AUTHENTIK_*` env. `docker compose config` clean (8 services).
- [x] CORS: credentialed cross-subdomain (`CORS_ALLOWED_ORIGINS=https://portal.localhost`, `AllowCredentials`) so the login form POST can set cookies.
- [x] `.env` / `.env.example` reconciled to local auth; `ACCESS_TOKEN_TTL=5m`, `REFRESH_TOKEN_TTL=24h` (1-day remember-me window).

### Backend — local auth (ADR-06), wired end-to-end
- [x] **Migrations 0001–0006 applied (DB v6).** `0006_account_local_auth`: `+users.password_hash`/`password_updated_at`, `oidc_subject` → nullable, drop `user_oidc_roles` (up + down).
- [x] **`make sqlc`** → `account/repository/*.sql.go` generated (via `sqlc/sqlc` Docker).
- [x] **Repository adapter** implements `AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserStore` (`GetUserByEmail`/`CreateLocalUser`/`AssignRoleByCode`), `APIUserFetcher`.
- [x] **`account.New(...)` constructed in `cmd/api/main.go`**, `/api/v1` mounted, `MountHTTP`. `GET /api/v1/healthz` → 200; api logs "api listening" (no OIDC warning).
- [x] **Argon2id** password hash/verify (`account/auth/password.go`, 64 MB / t=3 / p=2, PHC format).
- [x] **Endpoints:** `POST /auth/login {email,password,remember}` (verify + brute-force rate-limit/lockout via Redis), `POST /auth/register` (201, **no session** → back to /login), `POST /auth/refresh` (rotation + reuse detection), `POST /auth/logout` / `/auth/logout-all` (bump token_version), `GET /auth/me`.
- [x] **Session cookies:** `portal_access` (5m) + `portal_refresh` (Path=/api/v1/auth) + durable `portal_session` marker (Path=/, read by middleware). `remember` controls persistence (24h vs session cookie). **Clear-cookies domain bug fixed** (was causing a /login↔/ redirect loop on logout).
- [x] **OIDC code removed** (`auth/oidc.go`, config `OIDC_*`, Login/Callback handlers); `go mod tidy` (dropped `go-oidc`/`oauth2`).
- [x] Fixed latent `writeJSONError` undefined in `platform/middleware/ratelimit.go` (unblocks `go build ./...`).

### Backend — storage layer (Phase 2)
- [x] **`platform/storage`**: `Storage` interface (`PresignPut`/`PresignGet`/`Put`/`Get`/`Delete`/`Exists`) + `Config`/`PresignedRequest`/`ErrNotFound`.
- [x] **aws-sdk-go-v2 S3 client** (`s3.go`) with `BaseEndpoint` + `UsePathStyle` → same code hits **MinIO (dev, path-style)** and **R2/S3 (prod, virtual-host)**; static creds from `S3_*`.
- [x] **Integration round-trip test** (`s3_test.go`, gated on `S3_ENDPOINT`) — **PASSES** against MinIO: Put → Exists → Get → PresignGet(HTTP) → PresignPut(HTTP) → Delete → ErrNotFound.
- [x] Bumped module + Dockerfile builder **Go 1.23 → 1.24** (aws-sdk-go-v2 min-Go); api/worker images rebuild green, `healthz` 200.

### Frontend — Olympus light-theme port (Next 15 / Tailwind v4)
- [x] **Light theme tokens** + **full Olympus SVG sprite (85 icons)** auto-generated into `SvgSprite`, referenced via `<Icon name=.. />`.
- [x] **App shell:** dark header (logo · search · Find Friends · **3 notification dropdowns** · **profile dropdown**), **collapsible left menu** (+ Profile Completion), **right friends panel** (Close Friends / My Family / Uncategorized, status dots, live search, collapse, Olympus Chat bar), scroll-to-top FAB.
- [x] **Home newsfeed:** weather · calendar · Pages widgets · composer (tabs + optimistic post) · feed posts (video card, likes, quick-actions) · **Birthday card** · Friend Suggestions · Activity Feed.
- [x] **Auth pages:** real `/login` + `/register` forms → `POST /auth/login|register`, **Remember me** wired, register success → login tab (prefilled email).
- [x] **Auth gate:** middleware on `portal_session`; **`SessionKeeper`** silently refreshes the access token (interval + focus, multi-tab throttled) so sessions don't drop; **Log Out** works (refresh-then-logout → clears cookies → /login).
- [x] Notification dropdowns are **functional**: live badges, accept/decline friend requests, mark-as-read (one menu open at a time, click-outside/Esc to close).
- [x] `next build` green (typecheck + lint pass).

### Docs
- [x] **ADR-06** (en + vi) + architecture README index row.
- [x] Synced to local auth: `CLAUDE.md` "Identity flow", `doc/*/authoration.md` (§2.1, threat model, endpoints), `doc/*/feature.md` §1.

---

## ⛔ Superseded (do NOT do)
- ~~Phase 0 — Authentik secrets / OIDC provider setup~~ → removed (ADR-06).
- ~~Phase 3 — OIDC login/callback end-to-end~~ → replaced by local password auth (done).
- ~~T4.2/T4.3 — RSC `refresh-and-return` + `/auth/callback` handoff~~ → replaced by the client-side `SessionKeeper` refresh (done).

---

### Backend — media vertical slice (Phase 5)
- [x] **T5.1** Migration `0007_media_assets` (owner/kind/status/source_key/output_prefix/dims); `media` block re-enabled in `sqlc.yaml` → `mediarepo` generated.
- [x] **T5.2** Upload: `POST /assets` (asset + presigned PUT) · `PUT /assets/{id}/source` (API-proxied upload, dev) · `POST /assets/{id}/complete` (enqueue transcode). `mediarepo` adapter + `media.Service`/`Handler`.
- [x] **T5.3** Transcode worker: `worker.Transcoder` — download original → `ffprobe` dims/duration → `ffmpeg` VOD HLS (h264/aac) → upload manifest+segments → `MarkAssetReady`. Wired into `cmd/worker` (+ `cmd/api` enqueue).
- [x] **T5.4** Public HLS proxy `GET /assets/{id}/hls/*` + **Vidstack** player at `/upload` (progress, poll, library). **e2e verified**: mp4 → transcode → `ready` (dims + duration) → manifest `#EXTM3U` + segment served.
- [x] **Dragonfly fix**: `--default_lua_flags=allow-undeclared-keys` (Asynq's Lua scripts use undeclared keys; Dragonfly rejects them by default).

## ▶️ Remaining (hardening — demo loop already works)

### Phase 6 — CI drift + housekeeping
- [ ] **T6.1** `.github/workflows/ci.yml`: `sqlc-drift` + `openapi-drift` + `go build ./...` + `next build`.
- [ ] **T6.2** Reserve `notify:*` task prefix in `backend/MODULES.md` §5.2.
- [ ] **T6.3** *(optional)* `make certs` reads `APP_DOMAIN` instead of hard-coding `portal.localhost`.

---

## ⚠️ Known issues / cleanup
- [ ] `rbac.TestMatches` **fails** (pre-existing): `movies:*` vs `movies:write:own` wildcard-scope semantics disagree between test and impl — decide the intended rule, then fix one side.
- [ ] UI **sample data**: feed, friends panel, notification dropdowns, weather/calendar are static placeholders (no social/media backend in v1). Wire to real APIs as modules land.
- [ ] UI **placeholder actions**: Profile Settings / Create Fav Page / About links + per-item "Settings"/"⋯" are non-functional (visual, as in the template).
- [ ] Old dev OIDC client secret is in git history (`authentik/blueprints/portal.yaml`, since-deleted). Inert now (no Authentik), but rotate if that value was reused anywhere.
- [ ] No test runner beyond `rbac/permission_test.go`; add auth (password/login/refresh) + storage tests with the slice.
