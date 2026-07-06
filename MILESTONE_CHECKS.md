# Portal v1 — Milestone Checklist

Living scratchpad for the v1 demo loop. **Auth direction changed** ([ADR-06](doc/en/architecture/06-local-auth-model.md), 2026-07-05): the loop is

> **local password sign-in → authenticated Next.js home → upload mp4 → MinIO/R2 → worker transcodes to HLS → `assets.status = ready` → Vidstack playback → revocable logout.**

Authentik / OIDC is **removed** — Portal owns credentials (Argon2id). Scope is
[ADR-01](doc/en/architecture/01-v1-scope-cut.md). Tick as each check passes.

**Where we are:** the **v1 demo loop is closed and committed** — local sign-in →
home → upload mp4 → worker transcodes to HLS → Vidstack playback → revocable
logout, all end-to-end. Auth, the Olympus UI shell, storage (Phase 2), the media
slice (Phase 5), and CI + tests (Phase 6) are **done**. Only optional polish
(T6.3) and the known-issues cleanup below remain.

Stack status: **8 services** up (postgres · pgbouncer · dragonfly · minio(+setup) ·
traefik · api · worker · frontend). DB migrations at **v7** (through
`0007_media_assets`). HEAD **`c4e7fa2`** — phase commits: `578b994` (storage/P2) ·
`95fe07e` (media/P5) · `c4e7fa2` (CI+tests/P6).

---

## ✅ Done

### Infra / stack
- [x] docker-compose: Postgres · PgBouncer · Dragonfly · MinIO (`./data/minio` + `minio-setup` bucket) · Traefik, healthchecks, frontend build args, local TLS (self-signed) via `docker-compose.override.yml`.
- [x] **Authentik fully removed** — services, own Postgres, blueprints (`authentik/`), Traefik `auth.` alias, `SSL_CERT_FILE` override, all `OIDC_*`/`AUTHENTIK_*` env. `docker compose config` clean (8 services).
- [x] **Dragonfly** runs with `--default_lua_flags=allow-undeclared-keys` — Asynq's Lua scripts access undeclared keys, which Dragonfly rejects by default (was leaving transcode jobs stuck at `processing`).
- [x] CORS: credentialed cross-subdomain (`CORS_ALLOWED_ORIGINS=https://portal.localhost`, `AllowCredentials`) so the login form POST can set cookies.
- [x] `.env` / `.env.example` reconciled to local auth; `ACCESS_TOKEN_TTL=5m`, `REFRESH_TOKEN_TTL=24h` (1-day remember-me window).
- [x] Module + Dockerfile builder on **Go 1.24** (aws-sdk-go-v2 min-Go).

### Backend — local auth (ADR-06), wired end-to-end
- [x] **Migrations 0001–0006 applied.** `0006_account_local_auth`: `+users.password_hash`/`password_updated_at`, `oidc_subject` → nullable, drop `user_oidc_roles` (up + down).
- [x] **`make sqlc`** → `account/repository/*.sql.go` generated.
- [x] **Repository adapter** implements `AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserStore` (`GetUserByEmail`/`CreateLocalUser`/`AssignRoleByCode`), `APIUserFetcher`.
- [x] **`account.New(...)` constructed in `cmd/api/main.go`**, `/api/v1` mounted. `GET /api/v1/healthz` → 200.
- [x] **Argon2id** password hash/verify (`account/auth/password.go`, 64 MB / t=3 / p=2, PHC format).
- [x] **Endpoints:** `POST /auth/login {email,password,remember}` (verify + brute-force rate-limit/lockout via Redis), `POST /auth/register` (201, **no session** → back to /login), `POST /auth/refresh` (rotation + reuse detection), `POST /auth/logout` / `/auth/logout-all` (bump token_version), `GET /auth/me`.
- [x] **Session cookies:** `portal_access` (5m) + `portal_refresh` (Path=/api/v1/auth) + durable `portal_session` marker (Path=/, read by middleware). `remember` controls persistence (24h vs session cookie). **Clear-cookies domain bug fixed** (was causing a /login↔/ redirect loop on logout).
- [x] **OIDC code removed** (`auth/oidc.go`, config `OIDC_*`, Login/Callback handlers); `go mod tidy`.
- [x] Fixed latent `writeJSONError` undefined in `platform/middleware/ratelimit.go`.
- [x] **Fixed `rbac.Matches`**: a wildcard-action grant (`movies:*`) covers every scope incl. `:own`, matching the package doc.

### Backend — storage layer (Phase 2)
- [x] **`platform/storage`**: `Storage` interface (`PresignPut`/`PresignGet`/`Put`/`Get`/`Delete`/`Exists`) + `Config`/`PresignedRequest`/`ErrNotFound`.
- [x] **aws-sdk-go-v2 S3 client** (`s3.go`) with `BaseEndpoint` + `UsePathStyle` → same code hits **MinIO (dev, path-style)** and **R2/S3 (prod, virtual-host)**; static creds from `S3_*`.
- [x] **Integration round-trip test** (`s3_test.go`, gated on `S3_ENDPOINT`) — passes against MinIO: Put → Exists → Get → PresignGet(HTTP) → PresignPut(HTTP) → Delete → ErrNotFound.

### Backend — media vertical slice (Phase 5)
- [x] **T5.1** Migration `0007_media_assets` (owner/kind/status/source_key/output_prefix/dims); `media` block re-enabled in `sqlc.yaml` → `mediarepo` generated.
- [x] **T5.2** Upload: `POST /assets` (asset + presigned PUT) · `PUT /assets/{id}/source` (API-proxied upload, dev) · `POST /assets/{id}/complete` (enqueue transcode) · `GET /assets[/{id}]`. `mediarepo` adapter + `media.Service`/`Handler`.
- [x] **T5.3** Transcode worker `worker.Transcoder` — download original → `ffprobe` dims/duration → `ffmpeg` VOD HLS (h264/aac) → upload manifest+segments → `MarkAssetReady`. Wired into `cmd/worker` (consume) + `cmd/api` (enqueue).
- [x] **T5.4** Public HLS proxy `GET /assets/{id}/hls/*` + **Vidstack** player at `/upload` (progress, poll, library). **e2e verified**: mp4 → transcode → `ready` (320×240, 3000 ms) → manifest `#EXTM3U` + segment served.

### Frontend — Olympus light-theme port (Next 15 / Tailwind v4)
- [x] **Light theme tokens** + **full Olympus SVG sprite (85 icons)** auto-generated into `SvgSprite`, referenced via `<Icon name=.. />`.
- [x] **App shell:** dark header (logo · search · Find Friends · **3 notification dropdowns** · **profile dropdown**), **collapsible left menu** (+ Profile Completion + "Upload Video"), **right friends panel** (Close Friends / My Family / Uncategorized, status dots, live search, collapse, Olympus Chat bar), scroll-to-top FAB.
- [x] **Home newsfeed:** weather · calendar · Pages widgets · composer (tabs + optimistic post) · feed posts (video card, likes, quick-actions) · **Birthday card** · Friend Suggestions · Activity Feed.
- [x] **Auth pages:** real `/login` + `/register` forms → `POST /auth/login|register`, **Remember me** wired, register success → login tab (prefilled email).
- [x] **Auth gate:** middleware on `portal_session` (`/`, `/upload`, `/library/*`); **`SessionKeeper`** silently refreshes the access token (interval + focus, multi-tab throttled); **Log Out** works (refresh-then-logout → clears cookies → /login).
- [x] Notification dropdowns **functional** (live badges, accept/decline, mark-as-read; one open at a time, click-outside/Esc).
- [x] **`/upload` studio** (Vidstack): proxied upload w/ progress, poll until ready, HLS player, library list. `next build` green.

### CI + tests (Phase 6)
- [x] **T6.1** `.github/workflows/ci.yml`: backend (`go build` · `go vet` · `go test -race` · **sqlc-drift**), frontend (`next build` = typecheck + lint), openapi (spec well-formed). *(Full openapi-drift activates once `make openapi` stubs are committed/used.)*
- [x] **T6.2** Reserved `notify:*` task prefix in `backend/MODULES.md` §5.2.
- [x] Tests: `account/auth/password_test.go` (Argon2id) · `media/service_test.go` (upload/complete/HLS path-safety/owner-scope, in-memory fakes) · `platform/storage/s3_test.go` (integration). **`go test ./...` green.**

### Docs
- [x] **ADR-06** (en + vi) + architecture README index row.
- [x] Synced to local auth: `CLAUDE.md` "Identity flow", `doc/*/authoration.md` (§2.1, threat model, endpoints), `doc/*/feature.md` §1.

---

## ⛔ Superseded (do NOT do)
- ~~Phase 0 — Authentik secrets / OIDC provider setup~~ → removed (ADR-06).
- ~~Phase 3 — OIDC login/callback end-to-end~~ → replaced by local password auth (done).
- ~~T4.2/T4.3 — RSC `refresh-and-return` + `/auth/callback` handoff~~ → replaced by the client-side `SessionKeeper` refresh (done).

---

## ▶️ Remaining

The v1 demo loop already works; everything here is optional polish or future work.

- [ ] **T6.3** *(optional)* `make certs` reads `APP_DOMAIN` instead of hard-coding `portal.localhost`.
- [ ] Presigned **direct** upload for prod/R2: `POST /assets` already returns a presigned PUT, but it points at the internal `S3_ENDPOINT`. For browser-direct upload add a `PublicEndpoint` (+ Traefik route for MinIO's S3 API) so the URL is browser-reachable. Dev uses the API-proxied `PUT /assets/{id}/source` (no extra host).
- [ ] Full **openapi-drift**: wire `make openapi` output into the code, commit the generated stubs, then have CI fail on drift (today's job only validates the spec is well-formed).
- [ ] `media:asset_ready` event emit + a domain-module subscriber (movie/music) — the async pattern is speced but no consumer yet.
- [ ] Thumbnail worker (`worker.HandleThumbnail`) is still a stub.

---

## ⚠️ Known issues / cleanup
- [ ] Vidstack loads hls.js from a CDN at runtime — playback needs internet. Bundle hls.js if offline playback is required.
- [ ] UI **sample data**: feed, friends panel, notification dropdowns, weather/calendar are static placeholders (no social backend in v1). Wire to real APIs as modules land.
- [ ] UI **placeholder actions**: Profile Settings / Create Fav Page / About links + per-item "Settings"/"⋯" are non-functional (visual, as in the template).
- [ ] Old dev OIDC client secret is in git history (`authentik/blueprints/portal.yaml`, since-deleted). Inert now (no Authentik), but rotate if that value was reused elsewhere.
