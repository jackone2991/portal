# Portal — Whole-project work checklist

> **Source of truth:** the product roadmap is [feature.md](feature.md) §18 (Phases 0–12) + the decision log §20 (D-1…D-40). Per-task v1 status lives in [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md) (the living tracker for the v1 demo loop). The v1 scope is the hard cut in [ADR-01](architecture/01-v1-scope-cut.md).
>
> **Note on phase numbering:** MILESTONE_CHECKS.md numbers by *v1 build order* (T5 = media, T6 = CI…). This file numbers by the *product roadmap* in feature.md §18 (Phases 0–12) — two different systems, don't conflate them.
>
> Legend: `[x]` = done & verified end-to-end · `[~]` = partially done (v1 slice) · `[ ]` = not started · ~~strikethrough~~ = superseded/retired (don't do).

Updated: 2026-07-07 · HEAD `c4e7fa2` · DB schema **v7** (`0007_media_assets`) · 8 services up.

---

## 0. Phase overview

| Phase | Subject | Status | Notes |
|---|---|---|---|
| **Phase 0** | Foundation wiring (auth + module wiring + CI) | **[x] DONE** | Exit met 2026-07-06; D-25 audit move, RFC 7807, cross-module schemas, OpenAPI drift, versioning + frontend docs, CI security/roundtrip all closed 2026-07-07 (openapi handler-drift still pending codegen) |
| **Phase 1** | Tenancy + RLS | **[ ] Deferred (v1)** | Deferred for v1 per ADR-01 |
| **Phase 2** | Media pipeline end-to-end | **[~] Partial** | VOD HLS slice done; ladder/quota/poster/event still open |
| **Phase 3** | First vertical: Movies | **[ ] Not started** | |
| **Phase 4** | Music · Stories · Comics | **[ ] Not started** | |
| **Phase 5** | Bank (Personal Finance) | **[ ] Not started** | 9 sub-phases 5a–5i |
| **Phase 6** | Notifications | **[ ] Not started** | |
| **Phase 7** | Social layer (baseline) | **[ ] Not started** | |
| **Phase 8** | Search & discovery | **[ ] Not started** | |
| **Phase 9** | Marketing microsite + extras | **[ ] Not started** | |
| **Phase 10** | Social: advanced formats & engagement | **[ ] Not started** | |
| **Phase 11** | Creator economy | **[ ] Not started** | |
| **Phase 12** | Marketplace + safety + voice call | **[ ] Not started** | |

**Done outside the roadmap (v1 foundation):** local auth (ADR-06), storage layer S3/MinIO/R2 (platform §14), Olympus UI shell (frontend §16), CI `ci.yml` (D-9). Details in [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md).

---

## Phase 0 — Foundation wiring  ✅ DONE

*Goal: turn the scaffolds into an auth flow that runs end-to-end.*

- [x] Wire `cmd/api/main.go` — load `platform/config`, open the pgx pool, construct `account.Module`, mount `MountHTTP(r)` under `/api/v1` with the standard middleware; `GET /api/v1/healthz` → 200.
- [x] Run `make sqlc` for the `account` block → `account/repository/*.sql.go`.
- [x] Repository adapters: `AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserStore`/`UserUpserter`, `APIUserFetcher`.
- [x] Split migration `0001` → the `0001_platform_init` … `0007_media_assets` tree (schema v7) [D-18].
- [x] Local auth (ADR-06): Argon2id hash/verify, `POST /auth/login|register|refresh|logout|logout-all`, `GET /auth/me`, cookies `portal_access`/`portal_refresh`/`portal_session`, brute-force rate-limit + lockout.
- [x] Reserve the Asynq `notify:*` prefix in `backend/MODULES.md` §5.2 [D-1].
- [x] Land CI `ci.yml` (build · vet · test `-race` · sqlc-drift · `next build` · openapi well-formed) [D-9].
- [x] Add `users.locale` (BCP 47, default `'en-US'`) + `users.timezone` (IANA, default `'UTC'`) in `0002_account_users` [D-7].
- [x] Move the `audit/` package `account/audit` → `platform/audit/` (`git mv`; account is now a consumer) + rename the account events to the `account.<resource>.<action>` taxonomy incl. `account.refresh.reuse_detected` [D-25]. *(build + tests green)*
- [x] Event-type taxonomy registry `<module>.<resource>.<action>` in `backend/MODULES.md` §5.3 (existing "no shared transactions" bumped to §5.4) [D-25].
- [x] Adopt the RFC 7807 `Problem` shape for 4xx/5xx in `shared/openapi.yaml` (`application/problem+json`) [D-7]. *(spec adopts it; handlers still emit legacy `{code,message}` and migrate as touched, per ADR-01)*
- [x] Eager cross-module schemas in OpenAPI: `Problem`, `Money`, `PaginatedResult`, `TenantContext`, `ContinuingItem` [D-29].
- [x] Fix OpenAPI drift: `/auth/login` → POST local-password, removed `/auth/callback`, added `/auth/register`; aligned the assets surface (presigned PUT, `/source`, `/complete`, list, public HLS) to the handlers (ADR-06).
- [x] Doc `docs/api/versioning.md` (additive-only + RFC 9745/8594 deprecation) [D-31].
- [x] Conventions doc `frontend/CLAUDE.md` (state boundary D-32 + RSC decision tree D-33).
- [~] Extend CI per [D-9]: the OpenAPI **codegen drift gate** landed in `ci.yml` ([ADR-10](../adr/10-openapi-contract-direction.md)). **Still deferred to Phase 0.5** ([ADR-01](../adr/01-v1-scope-cut.md), [ADR-05](../adr/05-phase0-wiring-order.md)): `release.yml` (GHCR image build/push on tag), migration-roundtrip (up·down·up), `govulncheck` + `pnpm audit`, and multi-arch build — **none are in `ci.yml` today**. Codegen-vs-hand-written-handler decision still open (backlog §9).
- ~~Surface `amr`/`acr`/`auth_time` claims from the IdP~~ → retired by ADR-06 (D-27.r1).
- ~~`user_oidc_roles` table + OIDC group→role sync~~ → retired by ADR-06, dropped by `0006` (D-26.r1).
- ~~Server-only API client `api-server.ts` + `/auth/refresh-and-return` route~~ → replaced by `SessionKeeper` client-side refresh + `portal_session` middleware gate (D-34.r1). *(`api-server.ts` remains future work)*

**Exit:** a developer runs `make up && make dev`, signs in via `POST /api/v1/auth/login`, hits `/auth/me`, and `RequireAuth` + `RequirePermission` reject an unauthenticated call; CI fails any PR that drifts generated code. **→ Met 2026-07-06.**

---

## Phase 1 — Tenancy + RLS  ⏸ Deferred for v1

- [ ] Schema `tenant.organizations` with a `kind` column (`'org' | 'household'`) from the start [D-24].
- [ ] Schema + queries `tenant.memberships`; role granularity per kind (org: full hierarchy; household: owner + member, soft cap 6).
- [ ] `0010_rls_enable.up.sql` — RLS on every tenant-scoped table; `USING (tenant_id = current_setting('app.tenant_id')::uuid)`.
- [ ] `platform/db.BeginTenantScope(ctx, tenantID)` sets the GUC in a per-request tx.
- [ ] Tenant-resolution middleware: slug `/t/{tenant}/...` → `tenant_id` → verify membership (or `tenant=me`) → set GUC; single-tenant maps straight through Traefik [D-23].
- [ ] `cmd/sysjobs` skeleton wiring the BYPASSRLS pool.
- [ ] `--profile observability` (Loki + Prometheus + Tempo + Grafana + GlitchTip), `/metrics` on a separate port, OTel auto-instrument chi + pgx + asynq [D-8].

**Exit:** an integration test proves tenant A's rows are invisible to a request bound to tenant B, `cmd/sysjobs` sees both; Grafana shows per-route latency by tenant.

---

## Phase 2 — Media pipeline end-to-end  🟡 Partial

**Done (v1 slice):**
- [x] Storage layer `platform/storage` — one S3 client (`aws-sdk-go-v2`, `BaseEndpoint`+`UsePathStyle`) → MinIO dev / R2 prod; round-trip test `s3_test.go`.
- [x] Upload: `POST /assets` (presigned PUT) · `PUT /assets/{id}/source` (proxied dev) · `POST /assets/{id}/complete` (enqueue transcode) · `GET /assets[/{id}]`.
- [x] Transcode worker: download → `ffprobe` → `ffmpeg` VOD HLS (h264/aac) → upload manifest+segments → `MarkAssetReady`.
- [x] Public HLS proxy `GET /assets/{id}/hls/*` + Vidstack player at `/upload`. **e2e verified.**
- [x] State machine `pending → processing → ready | failed`.

**Still open:**
- [ ] Multi-rung HLS ladder 1080p/720p/480p/360p (6s segments) + adaptive skip-rung [D-13].
- [ ] Selectable encoder `TRANSCODE_ENCODER` (`libx264`/`h264_nvenc`/`h264_vaapi`/`h264_qsv`) [D-13].
- [ ] Per-user quota + per-tenant cap + backpressure (429 + Retry-After) [D-13].
- [ ] Poster + sprite (thumbnail worker `HandleThumbnail` is still a stub).
- [ ] Emit `media:asset_ready { asset_id, hls_master_url, duration_ms, thumbnail_url }` + a consumer.
- [ ] Dead-letter `transcode:dead` after 3 retries [D-13].
- [ ] `mediaapi.SignedURL(ctx, id, ttl)` (v1 uses the public HLS proxy instead).
- [ ] Presigned direct upload for prod/R2 (`PublicEndpoint` + Traefik route to MinIO's S3 API).

**Exit:** a 30-second mp4 round-trips upload → transcode → HLS playable with Vidstack; a second user does not starve the queue. **→ Slice criterion met 2026-07-06.**

---

## Phase 3 — First vertical: Movies

- [ ] Schema + queries `movies`, `seasons`, `episodes`.
- [ ] Movie subscribes to `media:asset_ready` → flips `movies.status = ready`.
- [ ] Catalog endpoints: list (pagination + filter by genre/year/rating), detail, upsert watch-progress.
- [ ] Frontend route group `(movies)`: list, detail, player wiring the `progress` upsert.

**Exit:** happy path e2e through the frontend — add movie → transcode → browse → play → resume.

---

## Phase 4 — Music · Stories · Comics

- [ ] Each domain follows the Phase 3 template.
- [ ] Per-domain progress tables (`movie.watch_progress`, `music.listen_progress`, `story.read_progress`, `comic.read_progress`, same layout) [D-20].
- [ ] Per-domain ratings tables `<module>.ratings(...)` [D-21].
- [ ] Aggregator `GET /api/v1/continue` fans out to `<module>api.Continue(...)`, merges, sorts `updated_at DESC` [D-20].
- [ ] Music: playlists.
- [ ] Story/comic: chapter ordering + drafts (gated on the `creator` role).

**Exit:** each domain has a browse → consume → resume loop + a unified "continue" rail on home.

---

## Phase 5 — Bank (Personal Finance)

**Prerequisites (gate 5a):**
- [ ] Step-up middleware `RequireACR` (reads `acr`+`auth_time`, Problem `auth.step_up_required`) [D-27].
- [ ] MFA-enforcement login gate for users holding a `bank:*` permission [D-28].
- [ ] TOTP enrolment built in Portal (replacing Authentik per ADR-06).

**Sub-phases:**
- [ ] **5a Core ledger** — `bank.currencies`, `accounts` (`type` ASSET|LIABILITY|INCOME|EXPENSE|EQUITY), `categories`, `transactions`, `ledger_entries` with `CHECK SUM(amount)=0`; money `numeric(20,8)` + a `shopspring/decimal` `Money` type [D-14, D-15]. Destructive ops gated by `RequireACR` [D-27].
- [ ] **5b Multi-currency** — `fx_rates` daily snapshot; reporting currency on `users`; FX conversion entries.
- [ ] **5c Debts** — `debts`, `repayments`, `bank.counterparties` (optional `user_id` link) [D-16].
- [ ] **5d Loans** — mirror of debts; two-way confirmation when the counterparty is a portal user [D-16].
- [ ] **5e Investments** — `holdings`, `holding_lots` (FIFO), `price_history`; buy/sell → balanced ledger [D-15].
- [ ] **5f Budgets + goals** — `budgets`, `budget_periods`, `goals`; threshold alerts → Asynq.
- [ ] **5g Net-worth + reports** — `networth_snapshots`; per-TZ hourly scheduler `bank:snapshot_daily`; cash-flow/savings-rate/debt-to-income/performance endpoints (date range by `users.timezone`) [D-17].
- [ ] **5h Import/export** — CSV import (column mapper + dedupe) + CSV/JSON export.
- [ ] **5i Household sharing** — tenant `kind='household'`, both users `owner`, RLS unchanged, `bank:*:any` [D-24].

**Exit per sub-phase:** the op works through the frontend + writes an audit-log entry. Encryption-at-rest + audit are **non-negotiable** from 5a onward.

---

## Phase 6 — Notifications

- [ ] `notification` module owns `notifications`, `notification_preferences`, `delivery_attempts`, `push_subscriptions` [D-1].
- [ ] Asynq fan-out: emitters publish `notify:*`; a worker dispatches per channel.
- [ ] **In-app** channel — SSE `GET /api/v1/events/stream` via `platform/realtime/` [D-3].
- [ ] **Email** channel — SMTP `platform/mail/` (`wneessen/go-mail`) + templates `backend/templates/email/<category>/` [D-4].
- [ ] **Web Push** channel — VAPID (`SherClockHolmes/webpush-go`); no APNS/FCM in v1 [D-5].
- [ ] Per-category × per-channel user preferences.
- [ ] Best-effort backfill from `audit_log` at cutover.

**Exit:** ≥1 notification per emitting module delivered e2e through each enabled channel.

---

## Phase 7 — Social layer (baseline)

- [ ] Newsfeed — posts (text/image/link/poll), rich reactions (§9.19), comments, quote-share (§9.15), nested threading.
- [ ] Profiles — `social.profiles` (1:1 with `users`); identity-critical fields stay on `users` [D-19].
- [ ] Asymmetric follow graph (§9.12).
- [ ] Friend graph — requests, groups, block/mute (§9.3).
- [ ] Communities — pages, membership, page-scoped RBAC, basic moderation (§9.30 core).
- [ ] Events — calendar, RSVP, reminders.
- [ ] Messaging — 1:1 DM + group [D-3].
- [ ] Hashtags + mentions (§9.16, §9.17).
- [ ] Bookmarks (§9.18) + pinned content (§9.27) + drafts/scheduled (§9.26).
- [ ] Privacy controls (§9.21).
- [ ] Search integration `social/api.Search` [D-2].

**Exit:** a user can post/follow/join a community/react/comment/quote-share/RSVP/DM/mention/hashtag/bookmark/pin/schedule + tune privacy; a mod can run a basic community.

---

## Phase 8 — Search & discovery

- [ ] Resolve the search-engine choice (Postgres FTS first, re-evaluate Meilisearch) [D-2].
- [ ] Per-module index builders subscribing to relevant `*` events.
- [ ] Aggregator endpoint `/search?q=...&type=…`.
- [ ] Command-palette / global search bar on the frontend.

**Exit:** typeahead across people/posts/movies/music/stories/comics/events/pages.

---

## Phase 9 — Marketing microsite + extras

- [ ] Company pages + blog (lightweight CMS).
- [ ] Badges / gamification (§9.36 + §9.32).
- [ ] Optional merchandise store (defer unless explicitly required).
- [ ] 404 / 500 polish.

---

## Phase 10 — Social: advanced formats & engagement

- [ ] Stories (§9.14) — ephemeral 24h; replies → DM; highlights; close-friends.
- [ ] Reels (§9.24) — user-uploaded audio + attribution chain `social.sounds` [D-37]; duets/stitches.
- [ ] Live streaming (§9.25) — RTMP ingest + LL-HLS via `mediamtx` [D-36]; live chat; auto-VOD replay; per-tenant cap; `LIVE_LATENCY_SECONDS=4`.
- [ ] Photo carousels & albums (§9.23).
- [ ] Long-form articles (§9.28).
- [ ] Reddit-style voting + karma (§9.20, §9.32).
- [ ] Feed ranking (§9.13) — hand-tuned three-layer "For You" pipeline + `/settings/feed` UI; "Following" chronological by default [D-35].
- [ ] Lists & custom feeds (§9.22).
- [ ] Content warnings (§9.31).
- [ ] Messaging extensions (§9.37) — reactions, reply-quote, voice notes, disappearing.
- [ ] Advanced moderation (§9.30) — auto-mod, shadow-ban, appeals.
- [ ] Community wiki (§9.33), AMAs (§9.35), memories/on-this-day (§9.34).
- [ ] Verification & creator badges (§9.36).

**Exit:** a creator can record a story / post a reel (sound reused) / go live (chat) / write an article; a community runs voting + karma + auto-mod; "For You" has transparency + chronological fallback.

---

## Phase 11 — Creator economy

- [ ] Bridge module `internal/modules/creator/` ↔ `bank`.
- [ ] Tips / awards → balanced ledger entries [D-15].
- [ ] Creator subscriptions (monthly recurring billing → bank ledger).
- [ ] Paid posts / paywalls.
- [ ] Creator analytics (subscriber count, MRR, churn, top fans).
- [ ] Payouts — `bank/payout/Provider` interface; `manual` default in v1; Stripe Connect later [D-40]; each payout gated by `RequireACR` [D-27].
- [ ] Mandatory MFA for creators with active monetisation [D-28].
- [ ] DMCA take-down workflow for music reels + paid content [D-37].

**Exit:** a creator publishes a paid post + receives tips; subscribers are billed monthly; balances run through the bank ledger; the operator completes a payout (manual/Stripe).

---

## Phase 12 — Marketplace + safety + voice call

- [ ] Marketplace (§11) — `internal/modules/marketplace/`; listings + chat (§9.6) + optional escrow via bank.
- [ ] Anti-abuse / ML moderation (§12.2) — `internal/modules/safety/`, pluggable `ImageClassifier` + `TextClassifier` interfaces; default NSFWJS + pHash; CSAM block + quarantine + page [D-38].
- [ ] GDPR export + account deletion (§12.1) — long-running Asynq task; soft-delete with a 30-day grace period.
- [ ] Voice / video calls (§9.37) — LiveKit SFU (group) + P2P (1:1) [D-39]; `--profile calls`; `coturn`.
- [ ] Audio rooms / Spaces (§9.29) — LiveKit audio-only + hand-raise [D-39].
- [ ] Platform-level T&S dashboard for `superadmin` (§12.3) + Grafana metrics [D-8].

**Exit:** sell an item + chat + escrow; NSFW auto-tagged before publish; CSAM quarantined + paged; export/delete data; 1:1 P2P + group calls via LiveKit; T&S has its own dashboard.

---

## v1 — Remaining polish & known issues

*(The v1 demo loop already runs; these are optional/cleanup — from [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md).)*

- [ ] **T6.3** `make certs` reads `APP_DOMAIN` instead of hard-coding `portal.localhost`.
- [ ] Presigned **direct** upload for prod/R2 (add `PublicEndpoint` + a Traefik route to MinIO's S3 API).
- [ ] Full **openapi-drift**: wire `make openapi` output into the code, commit the stubs, have CI fail on drift.
- [ ] Emit `media:asset_ready` + a domain-module subscriber (movie/music).
- [ ] Thumbnail worker `worker.HandleThumbnail` (still a stub).
- [ ] Vidstack loads `hls.js` from a CDN at runtime — bundle it if offline playback is required.
- [ ] UI **sample data** (feed/friends/notifications/weather/calendar) are placeholders — wire to real APIs as modules land.
- [ ] UI **placeholder actions** (Profile Settings / Create Fav Page / About…) are non-functional.
- [ ] Rotate the old OIDC client secret in git history (`authentik/blueprints/portal.yaml`, since-deleted) if that value was reused elsewhere.

---

## Cross-cutting gates (recall before each dependent phase)

- **Migrations** forward-only in prod [D-12]; split/audit before sqlc freezes the schema.
- **OpenAPI spec-first** [D-29] — edit spec → `make openapi` → implement; never hand-edit generated files.
- **Module boundary** [backend/MODULES.md](../../backend/MODULES.md) — cross-module only via `api/`; no cross-table JOINs; async via `<module>:<event>` events.
- **RBAC** goes through `rbac.Engine`/middleware, never ad-hoc checks; role hierarchy is canonical for v1 [ADR-02](architecture/02-rbac-model-reconciliation.md).
- **Storage:** dev = MinIO (bind-mount) · prod = R2, same S3 client [ADR-04](architecture/04-storage-tier-budget.md) (the "R2-only" title applies to deployed environments only); the two-tier MinIO-origin + R2-edge design is long-horizon.
- **Deferred outright for v1:** bank, social, creator, marketplace, ML safety, LiveKit/mediamtx, the observability stack (ADR-01).
