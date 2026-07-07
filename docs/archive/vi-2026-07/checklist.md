# Portal — Checklist công việc toàn dự án

> **Nguồn sự thật:** roadmap sản phẩm là [feature.md](feature.md) §18 (Phase 0–12) + decision log §20 (D-1…D-40). Trạng thái v1 chi tiết theo từng task nằm ở [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md) (living tracker cho vòng demo v1). Scope v1 là hard cut trong [ADR-01](architecture/01-v1-scope-cut.md).
>
> **Lưu ý về đánh số phase:** MILESTONE_CHECKS.md dùng đánh số theo *thứ tự build v1* (T5 = media, T6 = CI…). File này dùng đánh số *roadmap sản phẩm* của feature.md §18 (Phase 0–12) — hai hệ khác nhau, đừng lẫn.
>
> Chú thích: `[x]` = xong & verify end-to-end · `[~]` = xong một phần (lát cắt v1) · `[ ]` = chưa làm · ~~gạch ngang~~ = đã bị thay thế/retired (đừng làm).

Cập nhật: 2026-07-07 · HEAD `c4e7fa2` · DB schema **v7** (`0007_media_assets`) · 8 service up.

---

## 0. Bảng tổng quan phase

| Phase | Nội dung | Trạng thái | Ghi chú |
|---|---|---|---|
| **Phase 0** | Foundation wiring (auth + module wiring + CI) | **[x] HOÀN THÀNH** | Đạt exit 2026-07-06; D-25 move audit, RFC 7807, schema cross-module, drift OpenAPI, doc versioning + frontend, CI security/roundtrip đã đóng 2026-07-07 (openapi handler-drift còn chờ codegen) |
| **Phase 1** | Tenancy + RLS | **[ ] Deferred (v1)** | Hoãn cho v1 theo ADR-01 |
| **Phase 2** | Media pipeline end-to-end | **[~] Một phần** | Lát cắt VOD HLS xong; ladder/quota/poster/event còn mở |
| **Phase 3** | Vertical đầu: Movies | **[ ] Chưa** | |
| **Phase 4** | Music · Stories · Comics | **[ ] Chưa** | |
| **Phase 5** | Bank (Personal Finance) | **[ ] Chưa** | 9 sub-phase 5a–5i |
| **Phase 6** | Notifications | **[ ] Chưa** | |
| **Phase 7** | Social layer (baseline) | **[ ] Chưa** | |
| **Phase 8** | Search & discovery | **[ ] Chưa** | |
| **Phase 9** | Marketing microsite + extras | **[ ] Chưa** | |
| **Phase 10** | Social: format nâng cao & engagement | **[ ] Chưa** | |
| **Phase 11** | Creator economy | **[ ] Chưa** | |
| **Phase 12** | Marketplace + safety + voice call | **[ ] Chưa** | |

**Đã làm ngoài roadmap (nền v1):** local auth (ADR-06), storage layer S3/MinIO/R2 (platform §14), Olympus UI shell (frontend §16), CI `ci.yml` (D-9). Chi tiết ở [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md).

---

## Phase 0 — Foundation wiring  ✅ HOÀN THÀNH

*Mục tiêu: biến scaffold thành flow auth chạy end-to-end.*

- [x] Wire `cmd/api/main.go` — load `platform/config`, mở pgx pool, construct `account.Module`, mount `MountHTTP(r)` dưới `/api/v1` với middleware chuẩn; `GET /api/v1/healthz` → 200.
- [x] Chạy `make sqlc` cho block `account` → `account/repository/*.sql.go`.
- [x] Repository adapter: `AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserStore`/`UserUpserter`, `APIUserFetcher`.
- [x] Split migration `0001` → tree `0001_platform_init` … `0007_media_assets` (schema v7) [D-18].
- [x] Local auth (ADR-06): Argon2id hash/verify, `POST /auth/login|register|refresh|logout|logout-all`, `GET /auth/me`, cookie `portal_access`/`portal_refresh`/`portal_session`, brute-force rate-limit + lockout.
- [x] Reserve prefix Asynq `notify:*` trong `backend/MODULES.md` §5.2 [D-1].
- [x] Land CI `ci.yml` (build · vet · test `-race` · sqlc-drift · `next build` · openapi well-formed) [D-9].
- [x] Thêm `users.locale` (BCP 47, default `'en-US'`) + `users.timezone` (IANA, default `'UTC'`) trong `0002_account_users` [D-7].
- [x] Move package `audit/` `account/audit` → `platform/audit/` (`git mv`; account giờ là consumer) + đổi tên event của account sang taxonomy `account.<resource>.<action>` gồm `account.refresh.reuse_detected` [D-25]. *(build + test xanh)*
- [x] Registry event-type taxonomy `<module>.<resource>.<action>` trong `backend/MODULES.md` §5.3 (mục "no shared transactions" cũ dời thành §5.4) [D-25].
- [x] Adopt shape RFC 7807 `Problem` cho 4xx/5xx trong `shared/openapi.yaml` (`application/problem+json`) [D-7]. *(spec đã dùng; handler vẫn phát `{code,message}` cũ và migrate dần khi đụng tới, theo ADR-01)*
- [x] Schema cross-module eager trong OpenAPI: `Problem`, `Money`, `PaginatedResult`, `TenantContext`, `ContinuingItem` [D-29].
- [x] Fix drift OpenAPI: `/auth/login` → POST mật khẩu local, gỡ `/auth/callback`, thêm `/auth/register`; đồng bộ bề mặt assets (presigned PUT, `/source`, `/complete`, list, HLS public) theo handler (ADR-06).
- [x] Doc `docs/api/versioning.md` (additive-only + deprecation RFC 9745/8594) [D-31].
- [x] Doc convention `frontend/CLAUDE.md` (boundary state D-32 + decision tree RSC D-33).
- [~] Mở rộng CI theo [D-9]: `release.yml` (build/push image GHCR khi tag), migration-roundtrip (up·down·up), security (`govulncheck` bắt buộc + `pnpm audit` advisory) — **xong**; openapi handler-drift vẫn chờ wire codegen (`internal/handler/api.gen.go` chưa generate).
- ~~Surface claim `amr`/`acr`/`auth_time` từ IdP~~ → retired ADR-06 (D-27.r1).
- ~~Table `user_oidc_roles` + OIDC group→role sync~~ → retired ADR-06, drop bởi `0006` (D-26.r1).
- ~~API client server-only `api-server.ts` + route `/auth/refresh-and-return`~~ → thay bằng `SessionKeeper` client-side refresh + middleware gate `portal_session` (D-34.r1). *(`api-server.ts` vẫn là future work)*

**Exit:** developer `make up && make dev`, sign in qua `POST /api/v1/auth/login`, hit `/auth/me`, `RequireAuth` + `RequirePermission` reject call unauthenticated; CI fail PR làm generated code drift. **→ Đạt 2026-07-06.**

---

## Phase 1 — Tenancy + RLS  ⏸ Deferred cho v1

- [ ] Schema `tenant.organizations` gồm cột `kind` (`'org' | 'household'`) từ đầu [D-24].
- [ ] Schema + query `tenant.memberships`; role granularity per kind (org: hierarchy đầy đủ; household: owner + member, soft cap 6).
- [ ] `0010_rls_enable.up.sql` — RLS mọi table tenant-scoped; `USING (tenant_id = current_setting('app.tenant_id')::uuid)`.
- [ ] `platform/db.BeginTenantScope(ctx, tenantID)` set GUC trong tx per-request.
- [ ] Middleware tenant-resolution: slug `/t/{tenant}/...` → `tenant_id` → verify membership (hoặc `tenant=me`) → set GUC; single-tenant map thẳng qua Traefik [D-23].
- [ ] Skeleton `cmd/sysjobs` wire pool BYPASSRLS.
- [ ] Profile `--profile observability` (Loki + Prometheus + Tempo + Grafana + GlitchTip), `/metrics` port riêng, OTel auto-instrument chi + pgx + asynq [D-8].

**Exit:** integration test chứng minh row tenant A invisible với request bound tenant B, `cmd/sysjobs` thấy cả hai; Grafana show latency per-route theo tenant.

---

## Phase 2 — Media pipeline end-to-end  🟡 Một phần

**Đã có (lát cắt v1):**
- [x] Storage layer `platform/storage` — 1 S3 client (`aws-sdk-go-v2`, `BaseEndpoint`+`UsePathStyle`) → MinIO dev / R2 prod; test round-trip `s3_test.go`.
- [x] Upload: `POST /assets` (presigned PUT) · `PUT /assets/{id}/source` (proxied dev) · `POST /assets/{id}/complete` (enqueue transcode) · `GET /assets[/{id}]`.
- [x] Worker transcode: download → `ffprobe` → `ffmpeg` VOD HLS (h264/aac) → upload manifest+segment → `MarkAssetReady`.
- [x] Public HLS proxy `GET /assets/{id}/hls/*` + Vidstack player `/upload`. **e2e verified.**
- [x] State machine `pending → processing → ready | failed`.

**Còn mở:**
- [ ] HLS ladder multi-rung 1080p/720p/480p/360p (6s segment) + adaptive skip-rung [D-13].
- [ ] Encoder selectable `TRANSCODE_ENCODER` (`libx264`/`h264_nvenc`/`h264_vaapi`/`h264_qsv`) [D-13].
- [ ] Quota per-user + cap per-tenant + backpressure (429 + Retry-After) [D-13].
- [ ] Poster + sprite (thumbnail worker `HandleThumbnail` còn stub).
- [ ] Emit `media:asset_ready { asset_id, hls_master_url, duration_ms, thumbnail_url }` + có consumer.
- [ ] Dead-letter `transcode:dead` sau 3 retry [D-13].
- [ ] `mediaapi.SignedURL(ctx, id, ttl)` (v1 dùng public HLS proxy thay thế).
- [ ] Presigned direct upload cho prod/R2 (`PublicEndpoint` + Traefik route MinIO S3 API).

**Exit:** mp4 30-giây round-trip upload → transcode → HLS playable với Vidstack; user thứ hai không starve queue. **→ Tiêu chí lát cắt đạt 2026-07-06.**

---

## Phase 3 — Vertical đầu: Movies

- [ ] Schema + query `movies`, `seasons`, `episodes`.
- [ ] Movie subscribe `media:asset_ready` → flip `movies.status = ready`.
- [ ] Endpoint catalog: list (pagination + filter genre/year/rating), detail, upsert watch-progress.
- [ ] Route group frontend `(movies)`: list, detail, player wire upsert `progress`.

**Exit:** happy path e2e qua frontend — thêm movie → transcode → browse → play → resume.

---

## Phase 4 — Music · Stories · Comics

- [ ] Mỗi domain theo template Phase 3.
- [ ] Table progress per-domain (`movie.watch_progress`, `music.listen_progress`, `story.read_progress`, `comic.read_progress`, cùng layout) [D-20].
- [ ] Table ratings per-domain `<module>.ratings(...)` [D-21].
- [ ] Aggregator `GET /api/v1/continue` fan-out `<module>api.Continue(...)`, merge, sort `updated_at DESC` [D-20].
- [ ] Music: playlists.
- [ ] Story/comic: thứ tự chapter + draft (gate role `creator`).

**Exit:** mỗi domain có loop browse → consume → resume + rail "continue" thống nhất ở home.

---

## Phase 5 — Bank (Personal Finance)

**Prerequisites (gate 5a):**
- [ ] Middleware step-up `RequireACR` (đọc `acr`+`auth_time`, Problem `auth.step_up_required`) [D-27].
- [ ] Login gate MFA-enforcement cho user có permission `bank:*` [D-28].
- [ ] TOTP enrolment tự xây trong Portal (thay Authentik theo ADR-06).

**Sub-phase:**
- [ ] **5a Core ledger** — `bank.currencies`, `accounts` (`type` ASSET|LIABILITY|INCOME|EXPENSE|EQUITY), `categories`, `transactions`, `ledger_entries` với `CHECK SUM(amount)=0`; money `numeric(20,8)` + `shopspring/decimal` type `Money` [D-14, D-15]. Op huỷ diệt gate `RequireACR` [D-27].
- [ ] **5b Multi-currency** — `fx_rates` snapshot hàng ngày; reporting currency trên `users`; FX conversion entry.
- [ ] **5c Debts** — `debts`, `repayments`, `bank.counterparties` (link `user_id` optional) [D-16].
- [ ] **5d Loans** — mirror debts; confirmation hai-chiều khi counterparty là portal user [D-16].
- [ ] **5e Investments** — `holdings`, `holding_lots` (FIFO), `price_history`; buy/sell → ledger cân [D-15].
- [ ] **5f Budgets + goals** — `budgets`, `budget_periods`, `goals`; threshold alert → Asynq.
- [ ] **5g Net-worth + reports** — `networth_snapshots`; scheduler per-TZ hourly `bank:snapshot_daily`; endpoint cash-flow/savings-rate/debt-to-income/performance (date range theo `users.timezone`) [D-17].
- [ ] **5h Import/export** — CSV import (mapper column + dedupe) + CSV/JSON export.
- [ ] **5i Household sharing** — tenant `kind='household'`, cả hai user `owner`, RLS không đổi, `bank:*:any` [D-24].

**Exit per sub-phase:** op làm được qua frontend + có entry audit log. Encryption-at-rest + audit **không thương lượng** từ 5a.

---

## Phase 6 — Notifications

- [ ] Module `notification` sở hữu `notifications`, `notification_preferences`, `delivery_attempts`, `push_subscriptions` [D-1].
- [ ] Fan-out Asynq: emitter publish `notify:*`; worker dispatch per-channel.
- [ ] Channel **in-app** — SSE `GET /api/v1/events/stream` qua `platform/realtime/` [D-3].
- [ ] Channel **email** — SMTP `platform/mail/` (`wneessen/go-mail`) + template `backend/templates/email/<category>/` [D-4].
- [ ] Channel **Web Push** — VAPID (`SherClockHolmes/webpush-go`); không APNS/FCM v1 [D-5].
- [ ] Preference user per category × per channel.
- [ ] Backfill best-effort từ `audit_log` khi cutover.

**Exit:** ≥1 notification per emitting module deliver e2e qua mỗi channel đã enable.

---

## Phase 7 — Social layer (baseline)

- [ ] Newsfeed — post (text/image/link/poll), reaction phong phú (§9.19), comment, quote-share (§9.15), nested threading.
- [ ] Profile — `social.profiles` (1:1 `users`); field identity-critical ở lại `users` [D-19].
- [ ] Follow graph bất đối xứng (§9.12).
- [ ] Friend graph — request, group, block/mute (§9.3).
- [ ] Communities — page, membership, RBAC page-scoped, moderation cơ bản (§9.30 core).
- [ ] Events — calendar, RSVP, reminder.
- [ ] Messaging — DM 1:1 + group [D-3].
- [ ] Hashtags + mentions (§9.16, §9.17).
- [ ] Bookmarks (§9.18) + pinned content (§9.27) + drafts/scheduled (§9.26).
- [ ] Controls privacy (§9.21).
- [ ] Search integration `social/api.Search` [D-2].

**Exit:** user post/follow/join community/react/comment/quote-share/RSVP/DM/mention/hashtag/bookmark/pin/schedule + tune privacy; mod chạy được community cơ bản.

---

## Phase 8 — Search & discovery

- [ ] Resolve lựa chọn search engine (Postgres FTS trước, re-evaluate Meilisearch) [D-2].
- [ ] Index builder per-module subscribe event `*` liên quan.
- [ ] Endpoint aggregator `/search?q=...&type=…`.
- [ ] Command-palette / global search bar frontend.

**Exit:** typeahead across people/post/movie/music/story/comic/event/page.

---

## Phase 9 — Marketing microsite + extras

- [ ] Page company + blog (CMS nhẹ).
- [ ] Badge / gamification (§9.36 + §9.32).
- [ ] Optional merchandise store (defer trừ khi yêu cầu rõ).
- [ ] 404 / 500 polish.

---

## Phase 10 — Social: format nâng cao & engagement

- [ ] Stories (§9.14) — ephemeral 24h; replies → DM; highlight; close-friends.
- [ ] Reels (§9.24) — audio user-uploaded + chain attribution `social.sounds` [D-37]; duets/stitches.
- [ ] Live streaming (§9.25) — RTMP ingest + LL-HLS qua `mediamtx` [D-36]; chat live; auto-VOD replay; cap per-tenant; `LIVE_LATENCY_SECONDS=4`.
- [ ] Photo carousels & albums (§9.23).
- [ ] Long-form articles (§9.28).
- [ ] Voting + karma kiểu Reddit (§9.20, §9.32).
- [ ] Feed ranking (§9.13) — pipeline ba-lớp "For You" hand-tuned + UI `/settings/feed`; "Following" chronological default [D-35].
- [ ] Lists & custom feeds (§9.22).
- [ ] Content warnings (§9.31).
- [ ] Messaging extensions (§9.37) — reaction, reply-quote, voice note, disappearing.
- [ ] Moderation nâng cao (§9.30) — auto-mod, shadow-ban, appeal.
- [ ] Community wiki (§9.33), AMAs (§9.35), memories/on-this-day (§9.34).
- [ ] Verification & creator badge (§9.36).

**Exit:** creator record story / post reel (sound reused) / go live (chat) / viết article; community chạy voting + karma + auto-mod; "For You" có transparency + fallback chronological.

---

## Phase 11 — Creator economy

- [ ] Module bridge `internal/modules/creator/` ↔ `bank`.
- [ ] Tips / awards → ledger entry cân [D-15].
- [ ] Creator subscriptions (billing monthly recurring → bank ledger).
- [ ] Paid posts / paywalls.
- [ ] Creator analytics (subscriber count, MRR, churn, top fan).
- [ ] Payouts — interface `bank/payout/Provider`; `manual` default v1; Stripe Connect sau [D-40]; mỗi payout gate `RequireACR` [D-27].
- [ ] MFA bắt buộc cho creator có monetisation active [D-28].
- [ ] Workflow DMCA take-down cho music reels + paid content [D-37].

**Exit:** creator publish paid post + nhận tip; subscriber bill monthly; balance qua bank ledger; operator complete payout (manual/Stripe).

---

## Phase 12 — Marketplace + safety + voice call

- [ ] Marketplace (§11) — `internal/modules/marketplace/`; listing + chat (§9.6) + escrow optional qua bank.
- [ ] Anti-abuse / ML moderation (§12.2) — `internal/modules/safety/`, interface `ImageClassifier` + `TextClassifier` pluggable; default NSFWJS + pHash; CSAM block + quarantine + page [D-38].
- [ ] GDPR export + xoá account (§12.1) — task Asynq long-running; soft-delete grace 30 ngày.
- [ ] Voice / video calls (§9.37) — LiveKit SFU (group) + P2P (1:1) [D-39]; `--profile calls`; `coturn`.
- [ ] Audio rooms / Spaces (§9.29) — LiveKit audio-only + hand-raise [D-39].
- [ ] T&S dashboard cấp platform cho `superadmin` (§12.3) + metric Grafana [D-8].

**Exit:** bán item + chat + escrow; NSFW auto-tag trước publish; CSAM quarantine + page; export/xoá data; call 1:1 P2P + group qua LiveKit; T&S có dashboard riêng.

---

## v1 — Polish còn lại & known issues

*(Vòng demo v1 đã chạy; đây là optional/cleanup — từ [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md).)*

- [ ] **T6.3** `make certs` đọc `APP_DOMAIN` thay vì hard-code `portal.localhost`.
- [ ] Presigned **direct** upload cho prod/R2 (thêm `PublicEndpoint` + Traefik route MinIO S3 API).
- [ ] Full **openapi-drift**: wire output `make openapi` vào code, commit stub, CI fail on drift.
- [ ] Emit `media:asset_ready` + subscriber domain-module (movie/music).
- [ ] Thumbnail worker `worker.HandleThumbnail` (còn stub).
- [ ] Vidstack load `hls.js` từ CDN lúc runtime — bundle nếu cần offline playback.
- [ ] UI **sample data** (feed/friends/notification/weather/calendar) là placeholder — wire API thật khi module land.
- [ ] UI **placeholder actions** (Profile Settings / Create Fav Page / About…) non-functional.
- [ ] Rotate OIDC client secret cũ trong git history (`authentik/blueprints/portal.yaml` đã xoá) nếu value từng reuse nơi khác.

---

## Gate xuyên suốt (nhắc trước mỗi phase phụ thuộc)

- **Migration** forward-only ở prod [D-12]; split/audit trước khi sqlc freeze schema.
- **OpenAPI spec-first** [D-29] — edit spec → `make openapi` → implement; đừng hand-edit generated file.
- **Module boundary** [backend/MODULES.md](../../backend/MODULES.md) — cross-module chỉ qua `api/`; không JOIN cross-table; async qua event `<module>:<event>`.
- **RBAC** đi qua `rbac.Engine`/middleware, không check ad-hoc; role hierarchy canonical cho v1 [ADR-02](architecture/02-rbac-model-reconciliation.md).
- **Storage:** dev = MinIO (bind-mount) · prod = R2, cùng một S3 client [ADR-04](architecture/04-storage-tier-budget.md) (title "R2-only" chỉ áp cho môi trường đã triển khai); hai-tầng MinIO-origin + R2-edge là long-horizon.
- **Deferred outright cho v1:** bank, social, creator, marketplace, ML safety, LiveKit/mediamtx, observability stack (ADR-01).
