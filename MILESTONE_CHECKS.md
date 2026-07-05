# Portal v1 — Milestone Checklist

Living scratchpad for the v1 demo loop (OIDC sign-in → upload mp4 → transcode HLS →
Vidstack playback → revocable logout). Order follows
[ADR-05](doc/en/architecture/05-phase0-wiring-order.md); scope is
[ADR-01](doc/en/architecture/01-v1-scope-cut.md). Tick as each check passes.

**Critical path:** T1.1 → T1.2 → T1.3 → T1.4 (strictly sequential), then T3 (OIDC)
and T4 (frontend) in parallel; T5 (media) needs T2 (storage).

---

## ✅ Done this session (infra / config / docs)

- [x] docker-compose: Authentik (+own Postgres, reuses Dragonfly as Redis), MinIO on
  local folder `./data/minio` + `minio-setup` bucket, healthchecks, frontend template
  env, TLS via mkcert (`docker-compose.override.yml`).
- [x] `.env.example` + `.env`, `frontend/Dockerfile` build args, `Makefile` `certs`,
  `traefik/` file-provider dir + `dynamic.dev.yml`.
- [x] Frontend versioned template scaffold (`frontend/src/templates/v1/**`, `app/(public)|(app)`).
- [x] Docs: `CLAUDE.md`, `doc/*/frontend.md`, `doc/en/architecture/04`.

---

## Phase 0 — Bring up the local stack (manual, runnable now)

- [ ] **T0.1** Fill real secrets in `.env`: `AUTHENTIK_SECRET_KEY` (`openssl rand -base64 60`),
  `AUTHENTIK_PG_PASSWORD`, `AUTHENTIK_BOOTSTRAP_PASSWORD`, `POSTGRES_PASSWORD` (keep
  `DATABASE_URL` in sync), `MINIO_ROOT_PASSWORD` (= `S3_SECRET_KEY`), `JWT_SIGNING_KEYS`.
- [ ] **T0.2** `make certs` (install mkcert first) → trusts a local CA + issues the cert.
- [ ] **T0.3** Add 5 hosts records (127.0.0.1): `portal / api / auth / minio / traefik .portal.localhost`.
- [ ] **T0.4** `make up` → `docker compose ps` all `healthy`. Check: MinIO console opens, bucket `portal-media` exists.
- [ ] **T0.5** Log into Authentik `https://auth.portal.localhost` as `akadmin`.
- [ ] **T0.6** Create Portal OIDC Provider+Application (redirect `https://portal.localhost/auth/callback`,
  scopes `openid profile email groups`), group `portal-admins`. Set `OIDC_CLIENT_ID`/`SECRET`,
  `OIDC_GROUP_ROLE_MAP=portal-admins:admin`, `BOOTSTRAP_ADMIN_OIDC_SUBJECTS=<your sub>`.

## Phase 1 — Backend wiring (ADR-05, strictly sequential)

- [~] **T1.1** Split migrations written ([D-18]): `0001_platform_init` (extensions) ·
  `0002_account_users` (+`locale`/`timezone`/`token_version`/`disabled_at`; **kept `role`**
  because `GetUserAuthSnapshot` selects it) · `0003_account_rbac` (+`user_oidc_roles`; kept
  `roles.parent_id`, not `role_parents`) · `0004_account_sessions` · `0005_platform_audit`.
  `assets` deferred to T5.1. Old `0001_account_init`/`0002_account_rbac` removed.
  Reviewed FK-order + query-consistency by hand.
  **Check (PENDING — needs Docker running):** `make migrate && make migrate-down && make migrate`
  clean. NOTE: if you already applied the OLD migrations, reset first (fresh DB / drop the
  `schema_migrations` row) — the version numbers were reused.
- [ ] **T1.2** `make sqlc` → generates `internal/modules/account/repository/`. **Check:** `.sql.go` files appear.
- [ ] **T1.3** Repository adapters (`account/repository/adapter.go`): `AuthSnapshotFetcher`,
  `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserUpserter`. **Check:** `go build ./...`.
- [ ] **T1.4** Construct `account.New(...)` in `cmd/api/main.go` (+`cmd/worker`); mount `/api/v1` +
  `MountHTTP`. **Check:** `curl -k https://api.portal.localhost/api/v1/healthz` → 200.

## Phase 2 — Storage layer (`platform/storage`, currently empty)

- [ ] **T2.1** Add `aws-sdk-go-v2` S3 client reading `S3_*` from config; `Storage` interface
  (`PresignPut`/`PresignGet`/`Put`/`Get`/`Delete`).
- [ ] **T2.2** Dynamic endpoint/path-style (MinIO dev ↔ R2 prod). **Check:** presign + PUT/GET round-trip to MinIO.

## Phase 3 — OIDC end-to-end (M0.4)

- [ ] **T3.1** Browser: `/api/v1/auth/login` → Authentik → callback → `users` + `user_oidc_roles`,
  cookies set. **Check:** `curl -b cookies.txt .../api/v1/me` returns the user.
- [ ] **T3.2** `logout-all` bumps `token_version` → next `/me` 401. **Check:** no manual SQL.

## Phase 4 — Frontend auth handoff (M0.5) + finish template

- [ ] **T4.1** `cd frontend && pnpm install && pnpm typecheck && pnpm build`. **Check:** build green; `pnpm dev` renders `/`, `/login`, `/library/comic`.
- [ ] **T4.2** `src/lib/api-server.ts` (`server-only`, forwards cookies) + `app/auth/refresh-and-return/route.ts` + `app/auth/callback` ([D-34]).
- [ ] **T4.3** RSC `/account` fetches `/api/v1/me`, 401 → refresh-and-return. **Check:** browser login shows email.
- [ ] **T4.4** *(optional)* Port popup markup (`components/popup/*` are `null`) + paste SVG sprite.

## Phase 5 — Media vertical slice (upload → transcode → playback)

- [ ] **T5.1** Migration `000N_media_assets` (the deferred `assets` table, media-owned).
- [ ] **T5.2** Upload session with presigned PUT (browser → MinIO/R2 directly).
- [ ] **T5.3** Transcode worker: ffmpeg HLS → upload variants → `assets.status=ready` → emit `media:asset_ready`.
- [ ] **T5.4** Vidstack HLS playback. **Check:** upload 1 mp4 → plays back (v1 demo loop complete).

## Phase 6 — CI drift + housekeeping

- [ ] **T6.1** `.github/workflows/ci.yml`: `sqlc-drift` + `openapi-drift`.
- [ ] **T6.2** Reserve `notify:*` prefix in `backend/MODULES.md` §5.2.
- [ ] **T6.3** *(optional)* `make certs` reads `APP_DOMAIN` instead of hard-coding `portal.localhost`.
