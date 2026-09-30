# Spec-gap fix worklog — 2026-09-30

**Status:** current · **Last verified:** 2026-09-30 (generated from the run; ticks record fix progress)

**Source:** `spec-gap-review` workflow run `wf_eb011b57-514` (43 agents: 18 finders, merge, adversarial refute + fix audit). 267 raw → 189 canonical → **184 confirmed** (8 critical · 85 major · 91 minor), 5 refuted (listed at the end, not to be applied).

## How to resume (read this first if continuing after a token-out)

- This file is the single source of truth for fix progress. Each finding has a stable ID `Fnnn` and a checkbox.
- `- [ ]` = not started · `- [x]` = fixed · `- [~]` = partial/deferred (see the **Applied** note) · `- [c]` = the spec is now correct but the **shipped code** still diverges — the **Applied** note names the code follow-up; all of them are collected in § Code follow-ups at the end (docs-only pass: no code was changed).
- Every finding carries its own **Fix** text. Where the fix audit revised the proposal, only the revised text is given — apply that, not the original.
- **Line numbers in Evidence are as of 2026-09-30** (`main` @ `99b5a0b`); re-locate by quoted text if a file has moved.
- **Code is the tie-breaker**: where a spec and shipped code disagree and the finding says the code is right, correct the spec; never change code to match a stale spec inside this worklog.
- **Work top-to-bottom**: 🔴 critical → 🟠 systemic (MULTIPLE / README) → 🟠 per-spec major → 🟡 minor. Fix **one spec file at a time**; tick boxes and fill **Applied** in the same edit.
- After ticking, update the **Progress** counter just below.

## Progress

`[ updated 2026-09-30 ]`  **Fixed in the specs: 182 / 184** (`[x]` 103 · `[c]` 79 — spec corrected, shipped code still diverges) · `[~]` 2 partial  ·  critical 7/8 · major 84/85 · minor 91/91

---

## 🔴 Critical

### MULTIPLE

#### - [c] F001 · 🔴 CRITICAL · `MULTIPLE` · SPEC-07 P0.2 client + §7; SPEC-02 P0.4 + Reader redesign hooks + §7
*Category:* unimplementable / frontend-beacon  
**Problem:** Both progress specs tell the client to save with `navigator.sendBeacon`, which can only send POST. Both progress routes are PUT-only, so every beacon save gets a 405. The save is fire-and-forget, so the failure is never seen. The API is also cross-origin, so a JSON beacon requires a CORS preflight.  
**Evidence:** SPEC-07:88-95 'plus on pause and `pagehide` (via `navigator.sendBeacon`/keepalive fetch) ... The `sendBeacon` call sends a `Blob([JSON.stringify(body)], {type: 'application/json'})`'; SPEC-07:68 and §7:207 define only PUT. SPEC-02:349 '`useReaderProgress` (extract ... `sendBeacon`)'; SPEC-02:468 PUT only. Shipped code follows the spec: frontend/src/lib/comic.ts:118-119 and MediaDetailView.tsx:31-34 call sendBeacon; backend comic/module.go:86 and media/module.go:128 register `r.Put` only. sendBeacon returns true once the request is queued, so the keepalive fallback never runs. The API is on another origin (api-client.ts:15-23). TC-CONT-026 tests the Blob path.  
**Fix:** SPEC-07 P0.2 client: 'All saves, including pagehide, use `fetch(url, {method:'PUT', keepalive:true, credentials:'include', headers:{'Content-Type':'application/json'}, body})`. Never use `navigator.sendBeacon`: it can only POST, and this route is PUT-only. The pagehide save is a best-effort preflighted CORS request; the 10 s and on-pause saves are the durable path. Do not relax the handler to accept text/plain.' Add AC: 'Given a pagehide save against the cross-origin API, then the API logs a PUT 2xx, never POST 405.' SPEC-02:349 → '... throttle + keepalive `PUT` (SPEC-07 P0.2 transport rule; never sendBeacon)'; update SPEC-02 P0.4:182 and TC-CONT-026 to match.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — SPEC-07 P0.2 keepalive-PUT transport rule (never sendBeacon) + AC (pagehide save logs PUT 2xx, never POST 405); SPEC-02 P0.4 + useReaderProgress cite it; TC-CONT-026 rewritten. Code follow-up: MediaDetailView.tsx and lib/comic.ts still call navigator.sendBeacon first.

#### - [~] F002 · 🔴 CRITICAL · `MULTIPLE` · README 'Conventions binding on all specs'; §6 of SPEC-02, 05, 06, 07, 08, 09; SPEC-08 P0.4; SPEC-06 P1.6; SPEC-09 P0.2
*Category:* missing-convention / tenancy (ADR-07)  
**Problem:** The binding conventions never mention ADR-07 tenancy or RLS, and they allow exactly one cross-module FK (to users). As a result, every spec DDL omits `tenant_id`, RLS and the policy, cross-user periodic scans are written per-user, and pg_dump's role requirement is missing. Built as written, the tables are unscoped, or unreachable under FORCE RLS.  
**Evidence:** README.md:43-45 'no cross-module JOINs or FKs — with one sanctioned exception: the identity-anchor FK to `users(id)`'; the README has no tenant/RLS/ADR-07 text (lines 36-86). ADR-07 was accepted 2026-07-07 (07-tenancy-rls-model.md:3), before SPEC-05..09. ADR-07:40 'Tenant-scoped tables carry `tenant_id UUID NOT NULL REFERENCES organizations(id)`'; ADR-07:139 'every tenant-scoped migration since 0020 has ENABLE + FORCE RLS and a tenant_isolation policy'. No tenant_id or policy in: SPEC-05:186-197, SPEC-06:241-253, SPEC-07:187-198, SPEC-08:225-269, SPEC-09:242-255, SPEC-02 §6 :385-422 (while SPEC-02 rev notes rely on `comic_imports.tenant_id`, `runInUserTenant`). SPEC-08:136 'For every user's people' vs events.md:59 'runs once per tenant via forEachTenant'. ADR-07:122/130 requires pg_dump to see every tenant via BACKUP_DATABASE_URL.  
**Fix (revised by verify):** README: change the FK exception to 'two sanctioned identity-anchor FKs: `users(id)` and `organizations(id)` via `tenant_id` (ADR-07)'. Add the bullet: '**Tenancy (ADR-07)**: every user/org-data table carries `tenant_id uuid NOT NULL DEFAULT current_setting('app.current_tenant')::uuid REFERENCES organizations(id)` plus `CREATE INDEX <t>_tenant_idx ON <t>(tenant_id)`. The migration that creates it runs `ENABLE` + `FORCE ROW LEVEL SECURITY` and `CREATE POLICY tenant_isolation … USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (same)`, per 0020/0043. Shared seed rows visible to every tenant use a nullable tenant_id and `USING (… OR tenant_id IS NULL)` (the bank_categories precedent), and migrations insert them with explicit `tenant_id = NULL`. Request paths run inside `platform/db.BeginTenantScope` (RequireTenant). Worker handlers open the payload user's tenant scope. Periodic sweeps iterate `ForEachTenant` in cmd/worker. `pg_dump` connects as the owner role via `BACKUP_DATABASE_URL`. Global tables (`users`, `roles`, `permissions`, `role_permissions`, `user_roles`) and system tables (`ops_backup_runs`) are exempt, and each §6 states per table whether it is tenant-scoped.' Then add tenant_id + DEFAULT + index + policy to §6 of SPEC-02 (comics, comic_chapters, comic_pages, comic_reading_progress, comic_imports, comic_sync_sources), SPEC-05 (journal_entries), SPEC-06 (stream_items), SPEC-07 (media_playback_progress), SPEC-08 (people_persons, people_birthday_notices, circles) and SPEC-09 `ops_exports`. SPEC-08 P0.4 and SPEC-06 P1.6: 'For each tenant (ForEachTenant), …'. SPEC-09 P0.2 step 2: 'connects as the table-owner role via BACKUP_DATABASE_URL (as portal_app under FORCE RLS it would dump only rows visible to no tenant)'. Also add the rule to backend/MODULES.md §6 (ADR-07:139 action item).  
**Applied:** ◐ partial — README Tenancy (ADR-07) convention + organizations(id) FK; Tenancy paragraph under §6 of SPEC-01..09 (SPEC-04 flags notification_preferences/web_push_subscriptions lacking tenant_id; SPEC-09 ops_backup_runs exempt, ops_exports scoped); per-tenant sweep wording SPEC-08 P0.4, SPEC-06 P1.6; SPEC-09 P0.2 BACKUP_DATABASE_URL owner role. Deferred: backend/MODULES.md §6 rule (outside docs/).

### README.md

#### - [x] F003 · 🔴 CRITICAL · `README.md` · Conventions — Events
*Category:* wrong-mechanism / silent event loss  
**Problem:** The README says the event→consumer subscription table is registered in `cmd/worker` only. The table is per-binary, and `Publish` on a name the binary never registered returns nil. An event published from the API binary with its subscription registered only in cmd/worker is therefore silently dropped.  
**Evidence:** README.md:49-51 'the event-name→consumer-task subscription table registered in `cmd/worker`'. events.md:113-114 '`Publish` on an unregistered name returns nil by design'; events.md:120-121 'The subscription table is per-binary and hand-duplicated — `cmd/api/main.go` and `cmd/worker/main.go` each restate it.'  
**Fix (revised by verify):** README.md:48-51 → 'The `platform/events` fan-out helper (`Publish(ctx, name, payload)` + the event-name→consumer-task subscription table) was built by SPEC-01 P0.6. The table is per-binary: every binary that **emits** an event registers that event's consumer edges on its own publisher (`cmd/api/main.go` for HTTP-emitted events, `cmd/worker/main.go` for worker-emitted ones; also wiring the other binary is harmless because `Subscribe` is idempotent). `Publish` on a name the emitting binary has not registered is a silent no-op (events.md "What is NOT enforced"). DoD: the events.md row and the `Subscribe(` call in each emitting binary land in the same PR.' Apply the same correction to SPEC-01:428-429: replace 'registry in `cmd/worker`' with 'registry in each emitting binary (`cmd/api` and/or `cmd/worker`)'.  
**Applied:** ✅ README Events: subscription table per-binary + silent no-op + DoD; SPEC-01 P0.6/§9 "each emitting binary", "panics" corrected to no-op; events.md Delivery mechanics aligned.

### SPEC-01-media-image-pipeline.md

#### - [x] F004 · 🔴 CRITICAL · `SPEC-01-media-image-pipeline.md` · §5 P0.3 rationale, P0.5, P1.1, §7
*Category:* security / authz  
**Problem:** `GET /assets/{id}/original` and `PATCH /assets/{id}` are gated only by `RequirePermission` on an `:own` code, and `RequirePermission` never checks ownership. Any `user` can download anyone's original with GPS intact, and any `creator` can rename anyone's asset. The P0.5 AC leaves wildcard/`:any` holders undefined, and the rev-3 reason given for DELETE's `RequireOwnerOrPermission` is factually wrong.  
**Evidence:** SPEC-01:266-267 'GET /api/v1/assets/{id}/original — permission `assets:read:own` (rev 3: seeded to `user` by 0003)'; SPEC-01:308-309 and :403 'PATCH ... `assets:write:own`'; SPEC-01:275-276 says the original 'retains full EXIF including GPS'; AC SPEC-01:282 'Given user B (no wildcard perms) requests user A's original, then 403/404'. SPEC-02:113-115 and SPEC-02:508 r3: ':own does not check ownership by itself'. Worklog spec-gap-fix-worklog-2026-07-11.md:31: RequirePermission(x:own) 'passes any grant-holder regardless of ownership'. SPEC-01:186-188 claims a plain RequirePermission would '403 admin's 0003-seeded assets:delete:any', but admin inherits `assets:delete:own` via the parent chain (0003:133-138,156). Editor holds `assets:read:any` (0003:162).  
**Fix (revised by verify):** (1) P0.5 and §7 GET original: 'Authenticated. The handler loads the row owner-scoped (`owner_id = caller`). There is no `assets:read:any` or `*` bypass, because originals carry GPS. `assets:read:own` (seeded to `user` by 0003) documents the capability only; RequirePermission alone checks no ownership and must never be the sole gate.' AC: 'Given user B (any role, including `*` and editor's `assets:read:any`) requests user A's original, then 403 (or 404 `media/asset-not-found` when RLS hides the row), and no bytes are streamed.' (2) P1.1 and §7 PATCH: '`RequireOwnerOrPermission(engine, "assets:write:any", extractAssetOwner)`. `assets:write:any` is already seeded to `editor` by 0003 (admin inherits it), so no new seed row is needed.' (3) Replace the P0.3 parenthetical at :186-189 with the finding's text as proposed. (4) AC :229: 'without `assets:delete:any` (and not `*`)'.  
**Applied:** ✅ P0.5 original owner-scoped with no :any/* bypass, AC covers editor and *, Content-Disposition inline; P1.1 PATCH {title} via RequireOwnerOrPermission(assets:write:any) (shipped {visibility} stays owner-only); P0.3 gate rationale corrected; DELETE AC "without assets:delete:any (and not *)"; §7 rows.

#### - [x] F005 · 🔴 CRITICAL · `SPEC-01-media-image-pipeline.md` · §5 P0.1 worker step 1 + AC
*Category:* wrong-requirement  
**Problem:** The 8,000 px per-side cap rejects the tall webtoon strips that SPEC-02, the primary consumer, routinely ingests. The spec's own memory rationale is about area, not side length. There is also no height clamp for WebP's 16,383 px limit.  
**Evidence:** SPEC-01:101-105 'reject ... if either dimension > 8,000 px ... Memory rationale: decoded RGBA at 8,000² ≈ 256 MB'. An 800×12,000 strip is about 38 MB, yet it is rejected. AC SPEC-01:146 '9,000 px image → failed'. SPEC-02:8,357,365 (webtoon mode). TRACEABILITY-MATRIX.md:52: the shipped worker uses 'the area cap that admits a 704×18000 strip and refuses 8001×8000 ... libwebp height clamp'.  
**Fix (revised by verify):** Step 1: 'reject if width × height > 64,000,000 px (8,000² ≈ 256 MB decoded RGBA, the real memory guard), if either side > 30,000 px (sanity ceiling for malformed input), if animated, or on decode failure'. Step 3: 'each variant fits within max-width × 16,000 px (libwebp hard limit 16,383), aspect-preserving and never upscaled (`scale=w='min(W,iw)':h='min(16000,ih)':force_original_aspect_ratio=decrease`)'. ACs: replace '9,000 px image' with 'an 8,001×8,000 image (area over budget)', and add 'Given a 704×18,000 webtoon strip, then it reaches `ready` and `medium` is ≤ 16,000 px tall'. Record the change in the next rev's history.  
**Applied:** ✅ P0.1 step 1 area cap 64 MP + 30,000 px side ceiling; step 3 16,000 px height clamp; ACs 8,001×8,000 and 704×18,000 strip. Matches process_image.go.

### SPEC-04-notification-module.md

#### - [c] F006 · 🔴 CRITICAL · `SPEC-04-notification-module.md` · §5 P0.2 steps 3-4
*Category:* correctness / data loss  
**Problem:** The in-app row is the only idempotency gate. If a dispatch fails partway, channel sends are silently lost on retry. Types that write no row get no deduplication at all.  
**Evidence:** SPEC-04:87 'unless step 3's insert hit the dedup conflict, in which case channel fan-out is skipped too (the store row is the single idempotency gate for a redelivered intent)'. If the row insert succeeds and the enqueue then fails, the Asynq retry hits ON CONFLICT, skips fan-out, and the email is lost. `account.password_reset` is never persisted (:106), and the same is true for in_app=false types, so a redelivered intent sends the email again.  
**Fix (revised by verify):** Replace the step-4 clause 'unless step 3's insert hit the dedup conflict … redelivered intent' with: 'Fan-out runs on every attempt, including when step 3 hits ON CONFLICT. The store and each channel are deduplicated independently. When the intent carries `dedup_key`, each channel task is enqueued with a deterministic `asynq.TaskID("notify:email:<user_id>:<type>:<dedup_key>")` (likewise `notify:web_push:…`) and `asynq.Retention(24*time.Hour)`, so the ID stays reserved after completion. `asynq.ErrTaskIDConflict` is treated as success. Intents without `dedup_key` (e.g. `account.password_reset`) get no channel dedup and are bounded by the producer throttle instead.' Add AC: 'Given dispatch fails after the row insert but before the email enqueue, when Asynq retries, then exactly one row and exactly one `notify:email` task exist. Given the same `dedup_key` intent redelivered within 24 h after its email was sent, then no second email is sent.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 step 4 fan-out on every attempt, per-channel deterministic asynq.TaskID + Retention(24h), ErrTaskIDConflict = success; no-dedup intents bounded by producer throttle; 2 ACs. Code follow-up: dispatchIntent returns early on dedup conflict and sets no TaskID.

### SPEC-09-platform-ops.md

#### - [x] F007 · 🔴 CRITICAL · `SPEC-09-platform-ops.md` · §10 OQ4 (streaming vs temp-file); P0.2 steps 2 and 4
*Category:* factual-error / repo-reality  
**Problem:** The 'resolved' plan to pipe pg_dump stdout straight into storage Put cannot work. Put needs a seekable body because aws-sdk-go-v2 PutObject hashes the payload, and the storage layer documents this. The interface name `Store` is also wrong (it is `Storage`).  
**Evidence:** SPEC-09:299-302 '**stream** — `platform/storage.Store.Put(ctx, key, body io.Reader, contentType)` already takes an `io.Reader` (verified in `storage.go`/`s3.go`); pipe `pg_dump` stdout straight into it.' storage.go:43,53-55 `type Storage interface` ... 'Put ... body should be seekable (e.g. a *bytes.Reader or *os.File) so the SDK can sign the payload.' s3.go:74-79 is a single PutObject (5 GiB cap). media/service.go:65 already spools uploads to a temp file for this reason. Shipped ops/backup.go:80-90: 'a straight stdout→Put pipe fails with "request stream is not seekable"' → os.CreateTemp.  
**Fix (revised by verify):** §10 OQ4: '(resolved, corrected) Spool to a temp file. `platform/storage.Storage.Put` needs a seekable body so the SDK can sign the payload. Run `pg_dump -Fc` with `cmd.Stdout = io.MultiWriter(tmp, sha256)`, check exit 0, `Seek(0)`, then `Put(tmp)`, and remove the file on exit. Worker scratch disk must hold one dump. Above ~5 GiB, add a multipart Put (P2).' P0.2 step 2: 'spooled to a worker temp file (sha256 teed in the same pass), then uploaded via `platform/storage.Storage.Put`'. Step 4: 'sha256 computed while spooling'. §9 item 2: replace 'streaming' with 'dump + upload'.  
**Applied:** ✅ §10 OQ4 → spool to a temp file (seekable body for Storage.Put; multipart > ~5 GiB is P2); P0.2 steps 2/4 + §9 item 2; runbook §2; TC-OPS-006. Shipped backup.go already spools.

#### - [x] F008 · 🔴 CRITICAL · `SPEC-09-platform-ops.md` · §5 P0.2 worker image; §10 resolved questions
*Category:* factual-error  
**Problem:** The spec pins the pg_dump/pg_restore client to Postgres 17, but the server is PG 18. A 17 client refuses to dump an 18 server, and a 17 pg_restore cannot read an 18 archive. The runbook repeats the wrong pin, so the drill fails.  
**Evidence:** SPEC-09:100-101 'pin the client's **major version to Postgres 17**'; SPEC-09:294-295 'major-version-pinned to PG 17'. backend/Dockerfile: 'Bumped 17 → 18 on 2026-08-21 ... postgresql18-client', and it notes 'a pg_dump OLDER than the server refuses outright ("server version mismatch")'; ci.yml:35 'image: postgres:18'. docs/operations/backup-restore.md:70 and :135 say `postgresql17-client`.  
**Fix (revised by verify):** P0.2: 'The worker image carries the Postgres client whose major version equals the server's (currently `postgresql18-client`). An older pg_dump refuses to dump a newer server. Bump the client in the same change that moves the server major.' §10 row 1 to match. Fix backup-restore.md:70-71 and :135 to postgresql18-client / Postgres 18.  
**Applied:** ✅ P0.2 client major = server major (postgresql18-client), bumped with the server; §10 row 1; runbook + TESTDOC precondition. Shipped Dockerfile matches.

## 🟠 Major

### MULTIPLE

#### - [c] F009 · 🟠 MAJOR · `MULTIPLE` · README AuthZ; SPEC-01 §7/P0.3/P1.1; SPEC-02 P0.2 rev 7; SPEC-05 P1.5; SPEC-08 P1.7; SPEC-12
*Category:* rbac-role-floor  
**Problem:** Several features are granted to the base `user` role but depend on uploads, and SPEC-01 gates uploads on `assets:write:own`, which is seeded only to `creator`. A `user` can create a comic but never add pages, so it can never publish. The same user cannot attach journal photos or avatars. The README's 'owner is creator+' premise no longer holds now that registration is multi-user.  
**Evidence:** README.md:76-77 'The single v1 owner is provisioned `creator` or higher; grants to `user` are the floor.' SPEC-01:399 `/complete | assets:write:own`; :309 'seeded to creator by 0003'; 0003:155 `('creator','assets:write:own')`. SPEC-02:103-107 (0025 widened comics write/publish to `user`); :136-137 publishing requires ≥1 page per chapter. SPEC-05:169-171, SPEC-08:210-212, SPEC-12:183-184 rely on the creator-tier upload. 0031_account_user_approval approves multiple `user` accounts.  
**Fix (revised by verify):** SPEC-01 §7/P0.3/P1.1: 'A new migration `000N_media_user_asset_grants` seeds `('user','assets:write:own')` and `('user','assets:delete:own')` via the 0003 `WITH grants(...)` pattern (ON CONFLICT DO NOTHING). 0008 is applied and is not edited. Only after that migration is applied do upload-sessions/complete/PATCH enforce `RequirePermission("assets:write:own")`; today they are authenticated-only.' README:76-77 → 'Grants to `user` are the floor: any capability a `user`-granted feature depends on (incl. `assets:write:own`) must itself be granted to `user`. `creator`/`editor` hold only catalogue/moderation tiers.' SPEC-05:169-171, SPEC-08:210-212, SPEC-12:183-184 → '`assets:write:own`, granted to `user` by the SPEC-01 grant migration'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — README AuthZ owner-floor wording; SPEC-01 §7 names the 000N_media_user_asset_grants migration (upload authenticated-only until applied), P1.1 updated; SPEC-05 P1.5, SPEC-08 P1.7, SPEC-12 cite it. Code follow-up: the grant migration + RequirePermission enforcement.

#### - [x] F010 · 🟠 MAJOR · `MULTIPLE` · SPEC-06 header, §4, P0.1(b), P0.2, P1.6, §8; SPEC-02 P1.9 and non-goal line 43
*Category:* event-consumer-drift / stale contract  
**Problem:** SPEC-06 still projects `media:asset_ready` and `comic:chapter_published` into the stream, and its P1.6 backfill reseeds asset_ready rows. SPEC-02 P1.9 still calls itself a stream producer and requires a `comic:chapter_deleted` stream row that SPEC-06 lacks. The events registry, which calls itself the only contract, records that these projections were removed in 0033/0034 and now go to the bell only. The three documents disagree about who consumes what.  
**Evidence:** SPEC-06:4 'system events attach as their producers land (SPEC-01 P1.2/P0.3, SPEC-02 P1.9, …)'; :87 '| `media:asset_ready` | SPEC-01 P1.2 | `asset_id` | insert; skip origin='import' |'; :93 '| `comic:chapter_published` | SPEC-02 P1.9 | `chapter_id` | insert |'; :52 'what finished transcoding'; :128, :150, :165-167 asset_ready examples/ACs; :214-225 P1.6 seeds `event_type='media:asset_ready'`; :275-277 metric keyed on asset_ready. events.md:20 '**Not projected into the stream** since `0033`: an upload finishing is a library event, not a moment in the day'; :23 'since `0034`'; :24 chapter_deleted '**none** ... emit-only'; :58 dropped in 0033/0034/0040; :115-121 'this table is the only contract'. SPEC-02:306-314 '— life-stream producer #2 ... Register it in events.md with consumer "stream — delete the ... item (SPEC-06 P0.1)", and add the matching handler row to SPEC-06's P0.1 table'. SPEC-06 has no chapter_deleted row.  
**Fix (revised by verify):** As proposed for SPEC-06: delete the asset_ready and chapter_published rows from P0.1(b), delete AC :128, fix user story :52, change the P0.2 examples/§8 metric to `media:playback_completed → ("Watched <title>", /library/media/{asset_id})` and a `bank:transaction_created` recovery metric, update header :4, retire P1.6 (and its events.md row :57), and drop 'comic' from the §6 source_module comment. Add under the table: 'The stream projects moments, not library events: `media:asset_ready`, `comic:published` and the catalogue publishes go to the bell (SPEC-04) only (0033/0034/0040).' SPEC-02 P1.9 → 'On publish, emit ONE `comic:published {comic_id, owner_user_id, title, chapter_count}` (consumer: notify:on_comic_published, bell, dedup_key = comic_id:chapter_count). Not projected into the stream (0034). `comic:chapter_deleted {comic_id, chapter_id, owner_user_id}` is emitted per chapter on delete and is emit-only.' Drop 'life-stream producer #2' at :43. events.md: replace the `comic:chapter_published` row with `comic:published` as above.  
**Applied:** ✅ SPEC-06 header/story/P0.1(b) asset_ready + chapter_published rows and AC removed, "stream projects moments" note; P0.2 example + §8 metric moved; P1.6 retired; §6/§9 updated. SPEC-02 non-goal + P1.9 rewritten (comic:published → bell; chapter_deleted emit-only). events.md comic:published row; SPEC-01 P1.2/§6 backfill cite dropped; TC-STREAM-004/012/033/092, overview.md, diagrams.md §7 updated.

#### - [x] F011 · 🟠 MAJOR · `MULTIPLE` · SPEC-02 P1.9; events.md comic:chapter_published; SPEC-04 §7
*Category:* event-flood  
**Problem:** `comic:chapter_published` is emitted per chapter on publish. After whole-comic import (about 100 chapters), one publish sends about 100 bell notifications. events.md claims 'one bell entry', but nothing defines how that is achieved.  
**Evidence:** SPEC-02:306-307 payload `{comic_id, chapter_id, owner_user_id, title}`; events.md:23 'emitted per chapter on comic publish | notify — `notify:on_comic_published`, one bell entry'. SPEC-02 rev 10 describes the ~100-chapter archive. SPEC-04:231 names no `notify:on_comic_published` and no aggregation rule.  
**Fix (revised by verify):** SPEC-02 P1.9: replace the per-chapter publish emit with 'emit ONE `comic:published {comic_id, owner_user_id, title, chapter_count}` per publish action; the notify consumer `notify:on_comic_published` dispatches with `dedup_key = comic_id + ":" + chapter_count`, so a repeat publish of an unchanged comic is silent and a publish after new chapters notifies once.' events.md:23 → rename the row to `comic:published` with that payload and consumer. Add `notify:on_comic_published` to SPEC-04 §7 owned tasks. AC: 'Given a comic with 100 chapters published at once, then exactly one bell notification; given it re-published unchanged, then none.'  
**Applied:** ✅ SPEC-02 P1.9 one comic:published {comic_id, owner_user_id, title, chapter_count} per publish, dedup_key comic_id:chapter_count, 100-chapter AC; events.md row renamed; SPEC-04 §7 notify:on_comic_published; TC-COMIC-147. Matches HEAD.

#### - [c] F012 · 🟠 MAJOR · `MULTIPLE` · SPEC-04 P0.4; SPEC-01 P1.2 origin; SPEC-12; events.md:20
*Category:* event-flood  
**Problem:** Zip imports are suppressed via origin='import', but the same flood still happens for non-zip bulk uploads. Every journal photo (up to 10 per entry), manually uploaded comic page or cover, and avatar produces a 'ready' bell notification.  
**Evidence:** SPEC-01:373-374 `origin IN ('upload','import')`, and :316 upload-session sets 'upload'. SPEC-04:129 '**Events with `origin='import'` are skipped**' (the only filter). SPEC-12:209-211 composer uploads several files at once. SPEC-02 rev 8 comic pages go through `lib/media-upload.ts`. SPEC-04:43 user story is scoped to 'when my upload finishes **transcoding**'.  
**Fix:** SPEC-04 P0.4 → '**Only `kind='video'` with `origin='upload'` produces a notification.** Images process in seconds while the uploader watches, and imports are bulk; both are skipped.' ACs: 'Given 10 image uploads for one journal entry, then zero notifications'; 'Given a video upload reaching ready, then exactly one.' Mirror this in the events.md:20 consumer text.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — SPEC-04 P0.4 only kind='video' origin='upload' notifies + two ACs; events.md asset_ready consumer text mirrors. Code follow-up: notify OnAssetReady skips only origin='import'.

#### - [x] F013 · 🟠 MAJOR · `MULTIPLE` · README Money bullet; SPEC-03 §7 Ratification vehicle, §10 step 1; frontend.md §5.3
*Category:* dangling obligation  
**Problem:** D-41 has been recorded, but the README and SPEC-03 still call it a pending proposal and schedule it as build work. D-41's paired obligation, reconciling frontend.md §5.3 and the Phase-5 Money notes, was never done: frontend.md still describes decimal-string amounts.  
**Evidence:** README.md:81-84 '...the obligation to record it as a new decision entry'; SPEC-03:508-513 '(propose `D-41`) ... and reconcile frontend.md §5.3'; SPEC-03:556 step '1. D-41 decision entry'. feature-inventory.md:1771 '### D-41 — Bank v1 ledger money model: integer minor units'. frontend.md:618 'API returns `{ amount: "12345.67", currency: "USD" }`'; :931 '(string-amount-aware)'.  
**Fix (revised by verify):** README Money parenthetical → '(Ratified divergence: D-41 — integer minor units for the v1 ledger only; D-14/D-15 still govern multi-currency/creator-economy money. Rationale in SPEC-03 §7.)' SPEC-03 §7 → 'Ratified as D-41 (feature-inventory.md). Remaining DoD: reconcile frontend.md §5.3 + Phase-5 notes'; §10 step 1 → 'Migration + seeds + sqlc (1 day)'. frontend.md:631 → '`bank` money is `{ amount: <int64 minor units>, currency }` (D-41); `formatMoney` divides by `10**exponent(currency)` (VND = 0) before `Intl.NumberFormat`. D-14 string amounts remain the rule outside the v1 ledger'; :931 → 'minor-unit-aware (D-41)'.  
**Applied:** ✅ README Money bullet "ratified by D-41"; SPEC-03 §7 "Ratified as D-41", §10 step 1 "Migration + seeds + sqlc"; frontend.md §5.3 integer minor units, Phase 5 line minor-unit-aware.

#### - [x] F014 · 🟠 MAJOR · `MULTIPLE` · SPEC-04 header :4, P0.4 note :131; README SPEC-04 row :21
*Category:* dependency  
**Problem:** The header (and README) say SPEC-04 depends on SPEC-01 P1.2 'otherwise nothing hard', while P0.4 says it is not gated on P1.2. The real hard prerequisite is SPEC-01 P0.6 (the `platform/events` fan-out), which neither lists. P0.4 also quotes a 'nothing hard' header text that no longer matches.  
**Evidence:** SPEC-04:4 'Depends on: SPEC-01 P1.2 for P0.4 ...'; :131 '**Dependency (the header's "nothing hard" does not cover this):** ... **Decided: P0.4 is not gated.**'; :129 subscribes via 'the `platform/events` fan-out'. SPEC-01:293-305 P0.6 'every later multi-consumer stream/notify feature gates on this (SPEC-04 bell ...)'. README.md:21 'SPEC-01 P1.2 for its P0.4 consumer (rest is dependency-free...)'.  
**Fix (revised by verify):** SPEC-04:4 → '**Depends on:** SPEC-01 P0.6 (`platform/events` fan-out; hard, for P0.4); SPEC-01 P1.2 is not a gate (P0.4 ships the emit itself if needed).' README:21 cell → 'SPEC-01 P0.6 (fan-out); P1.2 not a gate'. SPEC-04:131 → '**Dependency (beyond the header's P0.6 prerequisite):**'.  
**Applied:** ✅ SPEC-04 header Depends-on = SPEC-01 P0.6 (P1.2 not a gate); P0.4 note reworded; README SPEC-04 row aligned.

#### - [x] F015 · 🟠 MAJOR · `MULTIPLE` · SPEC-05 P0.4 ACs; SPEC-06 P0.3/P0.2
*Category:* optimistic-update  
**Problem:** Both specs require an optimistic insert 'at its occurred_at position' into a cursor-paginated infinite query. A backdated entry older than the loaded pages has no loaded position, and inserting it anyway duplicates it when that page loads. The POST response is not a stream item, and the tie-break id is unstated. The rollback AC also restores only the text, not mood or date.  
**Evidence:** SPEC-05:150-154 'the entry appears at its occurred_at position ... on server error ... the composer restores its text'; SPEC-06:174-175 'optimistically inserted at its `occurred_at` position in the stream query'; SPEC-06:140 order `occurred_at DESC, id DESC`; SPEC-06:81-84 journal rows use `ref_id` = entry id.  
**Fix (revised by verify):** Add to SPEC-05 P0.4 and SPEC-06 P0.3: 'Optimistic placement: insert into the loaded page whose range contains `occurred_at`. If it is older than the last loaded item and `hasNextPage`, do not insert; show a "Saved to <date>" toast. Deduplicate by `ref_id` (= entry id) against fetched pages. The same rule applies to edits that change occurred_at. On error, restore body, mood and occurred_at.' SPEC-06 P0.2: 'The optimistic journal item is built from the POST response in the StreamItem shape `{id: <temp client id>, source_module:"journal", event_type:"journal:entry", ref_id: entry.id, occurred_at, body_md, mood}`. On refetch the server item (real `id`) replaces it by ref_id. Ties at equal occurred_at are placed first and are corrected by the refetch.'  
**Applied:** ✅ SPEC-05 P0.4 + SPEC-06 P0.3 optimistic-placement rule (in-range page only, "Saved to <date>" toast, dedupe by ref_id, edits, error restore) + backdated AC; SPEC-06 P0.2 optimistic StreamItem shape + tie rule.

#### - [c] F016 · 🟠 MAJOR · `MULTIPLE` · SPEC-06 P0.2, §10 OQ4; SPEC-03 P0.7; events.md:25
*Category:* payload-contract / unimplementable  
**Problem:** The bank stream card must render '<amount> <source>→<dest>', but the bank:transaction_* payload carries only account IDs and no currency. The spec's 'payload-only' decision cannot produce names, and minor units cannot be formatted in a multi-currency ledger.  
**Evidence:** SPEC-06:156-159 'renders the identical "moved <amount> <source>→<dest>" card'; :151; :299-301 'Start payload-only'. SPEC-03:280-283 payload `{transaction_id, user_id, account_id, amount, direction, category_id, occurred_at, is_transfer, transfer_id, counterparty_account_id}`; SPEC-03:399 per-account currency; :258 dashboard grouped by currency. category_id goes stale after a reassign-delete that emits no events (SPEC-03:292-295).  
**Fix:** SPEC-03 P0.7 and events.md:25: add `currency` to the payload. SPEC-06 P0.2: 'Bank cards are the first per-type fetcher (§10). At read time the stream service batch-resolves the page's account and category ids via one `bankapi.Names(ctx, userID, accountIDs, categoryIDs)` call per page. Unresolvable ids render "(deleted account)" / "Uncategorized". Amounts use the payload `currency` exponent (VND = 0).' §10 bullet 4 → 'Resolved for bank:*; other types stay payload-only.' AC: 'Given a USD account transfer, the card shows USD, never ₫.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — SPEC-03 P0.7 payload gains currency + Downstream line; events.md bank row; SPEC-06 P0.2 bank cards first per-type fetcher (bankapi.Names batch resolve, fallbacks, currency exponent) + USD AC; §10 bullet resolved. Code follow-up: no currency in payload, no bankapi.Names, stream.go formats all as VND.

#### - [c] F017 · 🟠 MAJOR · `MULTIPLE` · SPEC-08 P0.2/P0.4, §7 DELETE; SPEC-06 P0.1(b)
*Category:* event-lifecycle-gap  
**Problem:** Deleting a person, editing a birthday, or renaming a person leaves wrong or dangling birthday stream items. People emits no delete or revoke event, so SPEC-06 cannot correct its projection. A deleted person's cards link to a 404. This is the bug class SPEC-02 P1.9 fixed for comics.  
**Evidence:** SPEC-08:107-109 (an edit clears notice rows and re-emits); :90-93 'Already-emitted ... items are NOT retracted at v1 — accepted staleness' (edits only); :180-181 'person deleted between scans, then no event (notice rows gone via cascade)'; §7:283 DELETE emits nothing. SPEC-06:94 inserts on notice_id, and :152 href `/people/id`.  
**Fix (revised by verify):** As proposed: emit `people:person_deleted {person_id, user_id}` after commit and `people:birthday_notice_revoked {notice_id, person_id, user_id}` per cleared emitted notice, register both in events.md, add the two SPEC-06 P0.1(b) handler rows and the AC. Also add to SPEC-08 P0.2: 'A rename does not re-emit. The stream renders the stored `display_name` snapshot; accepted v1 staleness until the card ages out (state explicitly).' Alternatively, have SPEC-06 resolve names through peopleapi at read time, as the bank fix (finding 9) does.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — SPEC-08 P0.2 emits people:birthday_notice_revoked + people:person_deleted after commit; rename staleness accepted; AC; §7 PATCH/DELETE rows. SPEC-06 P0.1(b) two handler rows + AC; events.md two planned rows; TC-STREAM-005. Code follow-up: neither event emitted on HEAD.

#### - [c] F018 · 🟠 MAJOR · `MULTIPLE` · SPEC-06 P0.2 mapping + AC; SPEC-07 P0.3/P0.4; SPEC-04 P0.4
*Category:* route-contract  
**Problem:** Three specs give three media deep links. SPEC-06 uses a fragment on the grid, SPEC-07 the video-only per-asset player, and SPEC-04 leaves it undefined. Image assets cannot use the SPEC-07 player, which 404s non-video assets. The implementations have already diverged three ways.  
**Evidence:** SPEC-06:150,167 `/library/media#id`; SPEC-07:149-150 'builds each item's `href` to this exact path, `/library/media/{id}`'; SPEC-07:75-79 (progress 404s non-video); SPEC-04:133 'the asset's library entry'. HEAD: journal/stream.go:110 '/library/media'; media_progress.sql:23 '/library/media/' || id; notify/service.go:283 '/library/media'.  
**Fix (revised by verify):** Add to SPEC-07 P0.4, and reference from SPEC-06 P0.2 and SPEC-04 P0.4: '**Media deep-link rule:** video/audio asset → `/library/media/{id}` (detail player); image asset → `/library/media?open={id}`. SPEC-01 P0.4's grid reads `open` and opens its lightbox for that id, or shows a not-found toast. Every producer or consumer that builds a media href uses this kind-aware rule (notify `media.asset_ready` data.href, stream render mapping, continue items).' In SPEC-06:150/:167, replace `/library/media#id` accordingly, and add `media:playback_completed → ("Watched <title>", /library/media/{asset_id})`.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — SPEC-07 P0.4 Media deep-link rule (video/audio → /library/media/{id}; image → /library/media?open={id}); SPEC-06 P0.2 + SPEC-04 P0.4 cite it. Code follow-up: HEAD builds three different hrefs and the grid does not read ?open.

#### - [x] F019 · 🟠 MAJOR · `MULTIPLE` · specs README implementation order; briefs README build order; SPEC-06 §1/§9; SPEC-03 P0.7
*Category:* sequencing  
**Problem:** SPEC-06 says its projection must exist before the producers land, but both build orders put SPEC-06 last, after SPEC-02/03/07/08. SPEC-03 assumes SPEC-06 is already live. Every bank, playback or birthday event emitted before SPEC-06 is permanently lost from the stream, and no backfill recovers them.  
**Evidence:** SPEC-06:19-21 'the projection must exist **before** the producer modules land, not after, or the stream starts with holes.' vs specs/README.md:101-102 '6. SPEC-08, then SPEC-06 last' and briefs/README.md:37-38 'then 08 → 06'. SPEC-03:295-296 'these events already have a registered first consumer (SPEC-06's stream)'. SPEC-06:214-216 backfill covers media assets only.  
**Fix:** specs/README step 3 → '**SPEC-05** + **SPEC-06 P0.1** (journal module + `stream_items` projection and its `journal:stream_*` consumers; must precede every producer below, SPEC-06 §1)'. Step 6 → 'SPEC-08, then **SPEC-06 P0.2–P0.4 + P1** (read API, home, rail)'. Mirror this in briefs/README.md:34-38. Add to SPEC-06 §9 phase 1: '(lands with SPEC-05, before SPEC-02/03/07/08 emit)'.  
**Applied:** ✅ README build order: SPEC-05 + SPEC-06 P0.1 at step 3, SPEC-06 P0.2–P1 at step 6; briefs README mirrors; SPEC-06 §9 phase 1 lands with SPEC-05 before producers.

#### - [x] F020 · 🟠 MAJOR · `MULTIPLE` · SPEC-09 §1; SPEC-03 header/§10; specs README step 5
*Category:* sequencing  
**Problem:** The rule that SPEC-09 P0 (backups) lands before SPEC-03 data accrues is stated inconsistently. SPEC-09 calls itself the burst-filler, which is SPEC-07's role. SPEC-03's header and timeline never mention the backup gate. The README calls SPEC-03 'parallelizable' without saying with what.  
**Evidence:** SPEC-09:21-24 'lists it as a dependency-free burst-filler' vs briefs/README.md:36-37 '09 P0 (backups — must land **before** SPEC-03 ledger data accrues) → 03 (parallelizable) with 07 as burst-filler'; README.md:26, :97-98. SPEC-03:4 'Depends on: ADR-08; SPEC-01 only for P1 receipts'; SPEC-03:565-567; specs/README.md:99 '5. **SPEC-03** (parallelizable)'.  
**Fix:** SPEC-09:23-24 → 'which is why the build order places 09 P0 as a hard gate immediately before SPEC-03 (SPEC-07 is the burst-filler).' SPEC-03:4 → '**Depends on:** ADR-08; **SPEC-09 P0 (nightly backup + exercised restore drill) live before the first real ledger entry**; SPEC-01 only for P1 receipts'. SPEC-03 §10: 'Do not begin Goal 1's month of real logging until SPEC-09 P0.2–P0.4 is green.' README:99 → '5. SPEC-03 (after SPEC-09 P0 is live; in parallel with SPEC-07)'.  
**Applied:** ✅ SPEC-09 §1: P0 is a hard gate before SPEC-03, SPEC-07 is the burst-filler; SPEC-03 header Depends-on + §10 backup-gate paragraph; README step 5 + briefs sequencing.

#### - [x] F021 · 🟠 MAJOR · `MULTIPLE` · SPEC-07 P1.5; SPEC-08 P0.4; events.md rows 22/34
*Category:* registry-closure  
**Problem:** SPEC-07 and SPEC-08 each say 'two consumers are registered' for their events. The registry lists one live consumer (stream), and no spec defines a notify consumer task for either event. Wiring a subscription without a handler would make Asynq fail every such task.  
**Evidence:** SPEC-07:172-175 'Published via `platform/events` (... two consumers are registered). Consumers: stream (SPEC-06), notify (SPEC-04 open type registry)'; SPEC-08:152-153 '(via `platform/events` — two consumers are registered)' vs :160-162 'attach as they land'. events.md:22 'live — **1 consumer** (stream) | stream; notify' (self-contradictory); :34 '1 consumer (stream)'. SPEC-04:231 and events.md:39-62 define no `notify:on_playback_completed` / `notify:on_birthday_upcoming`.  
**Fix (revised by verify):** SPEC-07 P1.5 and SPEC-08:152-153 → '(via `platform/events`; v1 consumer: stream only; notify attaches when a `notify:on_*` task is specced in SPEC-04 §7)'. events.md:22/:34: move notify to 'planned'.  
**Applied:** ✅ SPEC-07 P1.5 + SPEC-08 P0.4 "v1 consumer: stream only; notify attaches when specced in SPEC-04 §7"; events.md rows move notify to Planned; Downstream lines mark SPEC-04 future.

#### - [x] F022 · 🟠 MAJOR · `MULTIPLE` · SPEC-07 Goal 2 / P2 'Comic leg'; SPEC-02 header
*Category:* dependency-ownership  
**Problem:** No spec owns `comicapi.Continue`. SPEC-07 Goal 2 promises comics in /continue 'automatically when SPEC-02 lands', but its P2 says 'design for, don't build'. SPEC-02 never specs the export, although it lands before SPEC-07.  
**Evidence:** SPEC-07:24-26 'comics automatically when SPEC-02 lands' vs :177-180 '### P2 ... (design for, don't build) - **Comic leg** ... only `comicapi.Continue`'. SPEC-02 has no `comicapi.Continue`. specs/README.md:96-99 orders SPEC-02 before SPEC-07.  
**Fix (revised by verify):** SPEC-07: move the comic leg to '**P1.6 Comic leg** — if SPEC-02 P0.4 has shipped, implement `comicapi.Continue(ctx, userID, limit)` returning the P0.3 item (module='comic', href=`/library/comic/{comic_id}/read/{chapter_id}?page={page_position}`) and add it to the aggregator fan-out in cmd/api handleContinue (½ day).' Goal 2 → 'comics join via P1.6 once SPEC-02 P0.4 exists'. SPEC-02 header: '**Downstream consumers:** SPEC-07 P1.6 (`comicapi.Continue`)'. SPEC-07 §9: 'P1 adds ~½ day'.  
**Applied:** ✅ SPEC-07 comic leg → P1.6 (comicapi.Continue + handleContinue fan-out; unbuilt on HEAD), removed from P2; Goal 2, P0.3, §9 updated; SPEC-02 header Downstream line.

#### - [~] F023 · 🟠 MAJOR · `MULTIPLE` · briefs README Status column; SPEC-01..09 header Status/Module lines; SPEC-02 header/§9/§11; SPEC-05..09 revision history
*Category:* status-coherence  
**Problem:** SPEC-01..09 are implemented according to events.md, the traceability matrix, SPEC-10 and SPEC-12. Yet the briefs README marks every one 'Specced, unbuilt', and the headers still say 'ready to build', 'not scaffolded', 'no code' or 'Last verified: never'. SPEC-02's header has two conflicting Last-verified fields, a stale 'skeleton' Module line, §11 stopping at r3, and a §9 that omits R1–R5 and sync. SPEC-05..09 have no revision-history section, and no lifecycle rule exists.  
**Evidence:** briefs/README.md:23-32 'Specced, unbuilt'. SPEC-04:3-4 'ready to build, rev 3 ... not yet scaffolded'; SPEC-05:3-4; SPEC-06:3; SPEC-07:3; SPEC-08:3-4; SPEC-09:3-4; SPEC-03:4 '(no code)'. SPEC-02:3 '**Last-verified:** 2026-08-15 · **Last verified:** never'; :14 'skeleton'; §11 ends :508 r3; :490 '5. P1: zip import (1 day), reader modes + bookmarks'. Contradicted by SPEC-12:4 'SPEC-05 (entries — shipped)', SPEC-10:11, events.md:21-36 'live', TRACEABILITY-MATRIX.md:204-214.  
**Fix (revised by verify):** Add a README convention that points to docs/STYLE.md: '**Spec header**: `Status:` uses STYLE.md's vocabulary (`draft` before build; `current` once the spec describes shipped behaviour; `historical` for executed one-shot specs such as SPEC-11). There is exactly one `Last verified:` field. Implementation state is not restated in headers: link /CLAUDE.md § Current status and TRACEABILITY-MATRIX.md. The PR that ships a phase corrects the text and bumps Last verified.' Then: SPEC-02:3 → '**Status:** current · **Last verified:** 2026-08-15' (drop the duplicate field and the 'skeleton' Module wording). SPEC-03..09 headers → 'Status: current', with 'not yet scaffolded' / 'no code' removed. Keep Last verified as `never` until each is checked whole. SPEC-02 §9 item 5 → 'zip import (per-chapter + whole-comic), reader R1–R5, external sync (P1.10); bookmarks unscheduled'. Briefs README Status cells → 'Specced; built (see TRACEABILITY-MATRIX)'. Do not add new revision-history sections to SPEC-05..09; fold their inline 2026-07-10 notes into the body text.  
**Applied:** ◐ partial — README spec-header convention; SPEC-02 header single Last verified, no "skeleton", §9 item 5, §11 r4–r14 pointer row; SPEC-03..09 Status current, "not scaffolded/no code" removed; briefs README Status → TRACEABILITY-MATRIX. Deferred: folding inline 2026-07-10 notes in SPEC-05..09 (owned by per-spec findings).

#### - [c] F024 · 🟠 MAJOR · `MULTIPLE` · README conventions; §7 of SPEC-01..08
*Category:* pagination-convention  
**Problem:** Specs cite a shared cursor convention that no document defines. Response field naming, limit defaults and maxima, the invalid-cursor Problem type and the list wrapper vary per spec. Implementers invented undeclared types such as `people/invalid-cursor`, `people/validation` and `journal/invalid-cursor`.  
**Evidence:** SPEC-03:482-483 'next_cursor ... matching SPEC-01's cursor convention', but SPEC-01:248-253/:404 never define next_cursor or limit. SPEC-02:453 'matches the SPEC-01/04/05/06/08 cursor convention'. Only SPEC-06:266 declares `stream/invalid-cursor`; SPEC-01/03/04/05/08 declare none. SPEC-05:215 and SPEC-08:280 have no limit. SPEC-07:120 returns a bare array, SPEC-04:216 wraps. people/handler.go:272,276 emits `people/invalid-cursor`, `people/validation`. README.md:36-86 has no pagination bullet.  
**Fix (revised by verify):** Add a README bullet: '**Pagination**: list endpoints use an opaque base64 keyset `?cursor=&limit=`. Each spec §7 states the default and max. For a new endpoint that states none: default 30, max 50. Existing endpoints keep their OpenAPI-declared limits. Responses are `{items: [...], next_cursor: string|null}` (extra top-level fields allowed, e.g. `unread_count`), and non-paginated lists still return `{items}`. The ordering key ends in `id`. A malformed cursor is 400 `<module>/invalid-cursor`, and body/param-shape failures without a named type are 422 `<module>/validation`. Each §7 lists both for every list endpoint.' Then add the invalid-cursor types to each §7 (incl. the missing media and notify), add `people/validation` to SPEC-08, and wrap SPEC-08 upcoming-birthdays as `{items}` (SPEC-07 /continue already is).  
**Applied:** ✅ spec fixed · ⚙ code follow-up — README pagination convention (next_cursor absent-or-null per handlers); §7 invalid-cursor/validation types in SPEC-01/02/03/04/05/06/08; SPEC-06 journal/invalid-cursor; SPEC-08 upcoming-birthdays documented as shipped {upcoming}. Code follow-up: media/notify answer bad cursor with about:blank; problems.ts lacks journal/invalid-cursor.

#### - [x] F025 · 🟠 MAJOR · `MULTIPLE` · README AuthZ convention; every §7 table
*Category:* authz-contract  
**Problem:** The README says `x-required-permission` applies 'per security.md's convention', but neither document says how to encode non-code rows: authenticated-only, public, owner-or-elevated, state-dependent or combined-method. No spec states its annotation, so each implementer will diverge.  
**Evidence:** README.md:74-76 'The OpenAPI `x-required-permission` extension applies per security.md's target convention.'; security.md:496 '...not yet uniformly present'. Rows that are not a single code: SPEC-01:400 'owner, or `assets:delete:any`', :402 'public-ish', SPEC-02:456 'published: `comics:read`; own draft: owner', :458, SPEC-07:207-209 'authenticated', SPEC-04:222 '*(public)*', SPEC-03:467 'GET/POST ... read/write'.  
**Fix (revised by verify):** Extend the README AuthZ bullet: 'OpenAPI encoding: public operations declare `security: []`; authenticated operations declare `security: [{bearerAuth: []}]`. An operation behind RequirePermission additionally carries `x-required-permission: <code>`. One behind RequireOwnerOrPermission carries `x-required-permission: {owner_or: <elevated code>, also: <code>?}`. Combined-method §7 rows are split per operation. The ADR-10 drift gate checks the annotation against the route middleware once the retrofit PR has annotated every existing operation; until then it warns.' Add to each spec's §7: 'Annotate per the README encoding.'  
**Applied:** ✅ README AuthZ OpenAPI encoding rules (honest: no operation annotated, no drift check → review item); "annotate per README encoding" line in §7 of SPEC-01..09 and SPEC-10 §6.

#### - [x] F026 · 🟠 MAJOR · `MULTIPLE` · README conventions; SPEC-01 P0.3 step (2); SPEC-07 P0.2; SPEC-05 P0.2; SPEC-02 PATCH
*Category:* data-model  
**Problem:** Several specs rely on `updated_at` changing on UPDATE or upsert, but the schema has no updated_at trigger and no spec says to set it. The sharpest case: SPEC-01 sets `deleting` without bumping updated_at, so its janitor picks up an old asset immediately and races the API purge (double purge, double `media:asset_deleted`). The same gap freezes SPEC-07 rail ordering at first watch.  
**Evidence:** SPEC-01:191 '(2) set `status='deleting'`' + :202-204 'WHERE status='deleting' AND updated_at < now() - interval '15 minutes' (grace window so it never races an in-flight API delete)'; 0007_media_assets.up.sql:25 `updated_at ... DEFAULT now()` with no trigger (0001 creates extensions only). SPEC-07:74 'Upsert, last-write-wins' + :123 'sorted `updated_at DESC`' + :195; SPEC-05:99 'Given an edit, then `updated_at` changes'; SPEC-02:393.  
**Fix:** README convention: '**updated_at**: there is no trigger; every UPDATE and every `ON CONFLICT … DO UPDATE` in `query/*.sql` sets `updated_at = now()` explicitly.' SPEC-01 P0.3 step (2) → 'set `status='deleting', updated_at=now()` (the janitor grace keys on this)'. SPEC-07 P0.2 → '`INSERT … ON CONFLICT (user_id, asset_id) DO UPDATE SET position_ms = EXCLUDED.position_ms, updated_at = now()`'. SPEC-05 and SPEC-02 PATCH → '… SET …, updated_at = now()'.  
**Applied:** ✅ README updated_at convention; SPEC-01 P0.3 step (2), SPEC-07 P0.2 upsert, SPEC-05 §7 PATCH, SPEC-02 §7 PATCH set updated_at.

#### - [x] F027 · 🟠 MAJOR · `MULTIPLE` · README Frontend; SPEC-01 P0.4; SPEC-02 P0.3; SPEC-03 §8; SPEC-05 P0.4; SPEC-06 P0.3; SPEC-08 P0.5
*Category:* rsc-coherence  
**Problem:** Six specs call their pages 'RSC shell' or 'RSC-first', which implies server-side authenticated fetches. There is no server API client, and the access cookie lives 5 minutes and is refreshed only in the browser, so RSC fetches would 401 after a short idle. No spec owns building the server client, and every shipped view is 'use client'.  
**Evidence:** frontend/CLAUDE.md:99-102 'The server-only API client (`src/lib/api-server.ts`) is future work'; account/handler/auth.go:691 access cookie MaxAge = AccessTTL (5 min); refresh only via the client SessionKeeper. Labels: SPEC-01:237, SPEC-02:154, SPEC-03:532, SPEC-05:147, SPEC-06:173, SPEC-08:186. HomeView, ComicIndexView, PeopleIndexView etc. begin with 'use client'.  
**Fix:** Add to the README Frontend bullet: '**"RSC shell" (binding until `src/lib/api-server.ts` exists):** the route `page.tsx` is a server component that exports `metadata` and renders the template view. All authenticated data is fetched in client islands via TanStack (D-32). A spec that wants server-side fetching must own building `api-server.ts` (`server-only` + `cookies()` forwarding) and the 401 path when `portal_access` has lapsed.' Point each spec's label at this definition.  
**Applied:** ✅ README Frontend defines "RSC shell"; SPEC-01 P0.4, SPEC-02 P0.3, SPEC-03 §8, SPEC-05 P0.4, SPEC-06 P0.3, SPEC-08 P0.5 point at it.

#### - [c] F028 · 🟠 MAJOR · `MULTIPLE` · SPEC-08 P0.3, P0.4; SPEC-06 P1.5; events.md:59
*Category:* repo-reality / ambiguity  
**Problem:** The TZ fallback relies on an undefined 'instance default', and `users.timezone` is NOT NULL DEFAULT 'UTC', so it is never unset. It also has no write path and is not exposed through accountapi, so people cannot read it without breaking the module boundary. The scan hour is never stated, and the registry says 'instance-TZ' at 06:00 UTC. The single daily run diverges from D-17's hourly per-TZ scheduler without saying so, and near local midnight it can skip a local date across a DST change, losing a day-of notice.  
**Evidence:** SPEC-08:115-117 'the stored user timezone; fall back to the instance default when unset'; :136-137; :164-167. 0002_account_users.up.sql:18 `timezone TEXT NOT NULL DEFAULT 'UTC'`; nothing writes it; account/api/api.go:22-26 `UserSummary{ID, Email, DisplayName}`; no APP_TIMEZONE in platform/config. D-17 (feature-inventory.md:1169) specifies an hourly scheduler. events.md:59 'daily scan, instance-TZ ... daily 06:00 UTC'. Shipped people/module.go:20-22 uses an instance-default TZ.  
**Fix (revised by verify):** P0.3: '(D-17). **TZ source at v1:** the existing platform setting `APP_TIMEZONE` (platform/config, IANA, default `UTC`), injected into people as `Deps.Timezone`. `users.timezone` has no write path and is not in `accountapi.UserSummary`; per-user TZ needs both (out of scope). An unparseable name falls back to UTC with a warning.' P0.4: 'Scheduled daily at 06:00 UTC on the shared scheduler, once per tenant via ForEachTenant; evaluated in APP_TIMEZONE. This deliberately deviates from D-17's hourly per-TZ pattern: a fixed-UTC run near local midnight can skip one local date across DST. The re-entry fix is D-17's hourly pattern.' Point SPEC-06 P1.5 at the same source, and change events.md:59 'instance-TZ' → 'APP_TIMEZONE'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — SPEC-08 P0.3 TZ = APP_TIMEZONE via Deps.Timezone (declared v1 deviation from D-17); P0.4 daily 06:00 UTC via forEachTenant, DST risk, D-17 re-entry; Goal 2/AC/§6; SPEC-06 P1.5, TC-STREAM-090, events.md aligned. Code follow-up: cmd/api + cmd/worker build people.Deps without Timezone (scan runs in UTC).

#### - [c] F029 · 🟠 MAJOR · `MULTIPLE` · SPEC-08 P0.5, §1/§2; SPEC-06 P0.4
*Category:* repo-reality  
**Problem:** The specs treat `BirthdayCard` and `PersonalInfoWidget` as kits that only need data wired in. Both hard-code sample data as prop defaults, and BirthdayCard renders only 'Today is X's Birthday!' for one person, with no 'in N days', list, empty or loading states. This is an interface change, and the zero-fixture grep test fails while those defaults exist.  
**Evidence:** SPEC-08:185-188 'reusing the `FriendCard` / `PersonalInfoWidget` kits ... `BirthdayCard` on home wired to P0.3'; :197 'Grep test: ... render zero fixture rows'; SPEC-06:195,199. BirthdayCard.tsx:10-16 `name = "Marina Valentine"` and :44-45 'Today is ... Birthday!'; PersonalInfoWidget.tsx:18-27 `DEFAULT_ITEMS` ('james@example.com'); HomeView.tsx:476 has its own inline BirthdayCard.  
**Fix (revised by verify):** SPEC-08 P0.5: '`BirthdayCard` is reworked, not wired. It is a self-fetching client island (TanStack query `["people","upcoming"]` → `GET /people/upcoming-birthdays?days=14`) with no fixture defaults. The headline is "Today is" at days_until=0, "Tomorrow is" at 1, else "In N days". It shows a skeleton while loading and renders nothing when the list is empty. `age_turning` is shown when present.' SPEC-06 P0.4: '`PersonalInfoWidget`'s `DEFAULT_ITEMS` is deleted and `items` made required, built from `GET /auth/me`; otherwise the §2 goal-4 grep fails.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — SPEC-08 P0.5 BirthdayCard reworked (self-fetching, Today/Tomorrow/In N days, skeleton, empty → nothing, age_turning) + ACs; SPEC-06 P0.4 PersonalInfoWidget DEFAULT_ITEMS deleted, items required. Code follow-up: both components still ship fixture prop defaults.

### README.md

#### - [x] F030 · 🟠 MAJOR · `README.md` · Documents table; Suggested implementation order
*Category:* stale sequencing  
**Problem:** The implementation order lists only SPEC-01..09, omits SPEC-10/11/12, and still opens with 'ADR-10 codegen cutover first'. ADR-10 records that this cutover never landed and was replaced by per-module retrofit. The Documents table has no Status column, so readers cannot tell built from next.  
**Evidence:** README.md:90-103 '**ADR-10 codegen cutover first** ... 1. SPEC-01 ... 6. SPEC-08, then SPEC-06 last'; :29 'SPEC-01, SPEC-05, SPEC-06 (all shipped)'. SPEC-10:3 'phase 1 building'; SPEC-11:3 'executed 2026-09-11'; SPEC-12:3 'executed 2026-09-19'. ADR-10:170 'Retrofit per module as each is touched'; :181-182 'The "cutover PR before SPEC-01" as scoped ... never landed; only the gate did.'  
**Fix (revised by verify):** Add a `Status` column to the Documents table, filled from code reality (the modules present under `backend/internal/modules/` and the TEST-RUN records), not from spec headers. Correct each stale spec header (SPEC-02/04/05/06/07/08/09 'ready to build' → 'built', or 'P0 built, P1.x open' as verified) and the briefs README Status column in the same PR. Replace 'Suggested implementation order' with 'Build state and what remains'. That section drops the ADR-10 cutover gate ('ADR-10's spec-first CI gate is in force; the ServerInterface retrofit is per-module-as-touched, ADR-10 checklist'). It lists what is open per spec, e.g. SPEC-09 P1.7 takeout (unbuilt: no ExportProvider in code), and SPEC-10 phases after SPEC-03 + SPEC-04. Finally, it states whether SPEC-09 P0 backups landed before SPEC-03 ledger data, citing the commit or test run.  
**Applied:** ✅ README Documents table code-derived Status column (creating migration per spec); "Suggested implementation order" → "Build state and what remains" (actual landing order, open items); stale headers + briefs Status corrected.

#### - [c] F031 · 🟠 MAJOR · `README.md` · Conventions — Errors (D-7)
*Category:* i18n / DoD  
**Problem:** The i18n DoD points at a next-intl catalog that does not exist and leaves the type-URI format undefined. The live catalog is `frontend/src/lib/problems.ts` with relative slugs, and shipped types such as `journal/invalid-cursor` and `media/asset-not-playable` are already missing from it.  
**Evidence:** README.md:52-57 'registered as an i18n message key in the frontend i18n catalog (per `frontend.md` §5)'; frontend.md:588-620 describes `src/i18n/messages/<locale>/errors.json` with full URIs, which does not exist (next-intl is not a dependency). problems.ts is keyed by slugs like 'journal/invalid-body'. journal/handler.go:230 and media/handler.go:23 emit types absent from the ProblemType union. Generic errors use `about:blank`.  
**Fix (revised by verify):** README.md:52-57 → 'Errors: RFC 7807 `Problem` on every non-2xx (D-7). Every Problem `type` a spec introduces is a relative slug `<module>/<kebab-reason>` and doubles as the i18n key. The same PR adds it to `ProblemType` and `PROBLEM_MESSAGES` in `frontend/src/lib/problems.ts`, the live catalog (frontend.md §5.2 next-intl `errors.json` is the future target). Generic transport failures use `about:blank` and need no key. A backend-emitted type absent from problems.ts is a DoD failure.' In shared/openapi.yaml `Problem.type`, change `format: uri` → `format: uri-reference` and the description to 'Stable problem-type reference: a relative `<module>/<reason>` slug, or `about:blank` (i18n key).' Add `journal/invalid-cursor` and `media/asset-not-playable` to problems.ts, and add an 'interim catalog: src/lib/problems.ts' note to frontend.md §5.2.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — README Errors: relative slugs, problems.ts is the live catalog; frontend.md §5.2 interim note. Code follow-up: openapi.yaml Problem.type → uri-reference; add journal/invalid-cursor + media/asset-not-playable to problems.ts.

#### - [c] F032 · 🟠 MAJOR · `README.md` · Conventions — Frontend
*Category:* frontend-auth  
**Problem:** Adding each new app route to the D-34 middleware matcher is required spec by spec (SPEC-03, SPEC-08) rather than by convention. SPEC-05/06/07 omit it (SPEC-07 creates a new route), and the shipped matcher already misses `admin`, `calendar` and `weather`.  
**Evidence:** SPEC-03:535-536 and SPEC-08:188-190 add matchers; SPEC-07:145-148 creates `app/(app)/library/media/[id]/page.tsx` with no matcher note. README.md:85-86 is silent. middleware.ts matcher is ['/', '/login', '/register', '/upload', '/library/:path*', '/bank/:path*', '/people/:path*'], while `app/(app)/` also contains admin, calendar and weather.  
**Fix (revised by verify):** Append to the README Frontend bullet: 'Every new route group under `app/(app)/` extends `config.matcher` in `frontend/src/middleware.ts` (D-34 edge gate) in the same PR; a route group absent from the matcher is a DoD failure.' File a fix adding '/admin/:path*', '/calendar/:path*' and '/weather/:path*' to the matcher. No SPEC-07 change is needed, because /library/:path* covers it.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — README Frontend: matcher extension is part of DoD. Code follow-up: add /admin, /calendar, /weather to the middleware matcher.

#### - [x] F033 · 🟠 MAJOR · `README.md` · Conventions (new bullet); SPEC-09 P1.7
*Category:* completeness / takeout  
**Problem:** Nothing obliges a module to ship a takeout `ExportProvider`. SPEC-09 lists five providers, so comic progress, playback progress, notification prefs and people interactions have no export owner, although Goal 5 promises a per-module archive.  
**Evidence:** SPEC-09:196-200 lists accountapi, mediaapi, journal, bank, people 'as they land'; SPEC-09 Goal 5. SPEC-02, SPEC-04, SPEC-07 (:187-198) and SPEC-08 P1.6 never mention ExportProvider.  
**Fix (revised by verify):** Add a README bullet: '**Takeout**: every module that stores user-authored or user-history data implements the `opsapi.ExportProvider` interface (SPEC-09 P1.7) from its own `api/` package, in the PR that creates the table. Otherwise its spec states the exclusion and the reason. The spec's §6 names the export format per table.' SPEC-09 P1.7 provider list → 'account (profile + audit trail), media (metadata + originals + playback progress), journal (markdown + attachments), bank (CSV per account + categories/budgets), people (persons + interactions), comic (metadata + reading progress), movie/music/story (catalogue metadata the owner authored), notify (prefs only; notification history excluded as derived), social (if user-authored rows exist, else stated exclusion). The list is non-exhaustive: the README Takeout rule governs new modules.'  
**Applied:** ✅ README Takeout convention (ExportProvider unbuilt until SPEC-09 P1.7); SPEC-09 P1.7 provider list completed, non-exhaustive.

### SPEC-01-media-image-pipeline.md

#### - [x] F034 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.1 Resource guardrails, AC, §10
*Category:* contradiction / repo-reality  
**Problem:** The spec puts `media:process_image` on the shared `heavy` queue (concurrency 1–2) with transcode. SPEC-02 rev 12, the registry and the code put it on its own `image` queue sized by IMAGE_CONCURRENCY (default 3). The AC and the open question are therefore wrong.  
**Evidence:** SPEC-01:119-121 '`media:process_image` and `media:transcode` run on a **`heavy` queue** ... `Concurrency: 1–2`'; AC SPEC-01:155-157 'at most the heavy server's configured concurrency (1–2)'; §10 SPEC-01:446-447. events.md:44 'its own `image` queue on its own server, concurrency `IMAGE_CONCURRENCY` — NOT the heavy pool'; SPEC-02:6 rev 12. Code: process_image.go:71 `asynq.Queue("image")`; cmd/worker/main.go:374-388 heavySrv `{"heavy":1}` and imageSrv `Concurrency: imageConcurrency`. SPEC-02:283-288 still sizes its timeout off 'heavy-queue concurrency'.  
**Fix (revised by verify):** Bump to rev 4. Replace the Resource guardrails paragraph: '`media:transcode` runs on the `heavy` queue with its own server (Concurrency 1). `media:process_image` runs on the `image` queue with its own server (Concurrency = `IMAGE_CONCURRENCY`, default 3; set 1 on a < 4 GB VPS). The step-1 area cap bounds each decode, so peak ≈ IMAGE_CONCURRENCY × 256 MB + one transcode. Light tasks stay on the weighted default server.' AC: 'Given 5 image tasks + 2 transcodes enqueued, at most IMAGE_CONCURRENCY image tasks and 1 transcode run at once (Asynq inspector).' Rewrite §10 as the IMAGE_CONCURRENCY sizing question.  
**Applied:** ✅ Rev 4: transcode on heavy (Concurrency 1), process_image on image (IMAGE_CONCURRENCY, default 3); AC; §10/§9 reworded. Matches cmd/worker.

#### - [c] F035 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.3 abandoned-upload sweep vs P0.5 / Goal 3
*Category:* contradiction / undefined behavior  
**Problem:** The abandoned-upload sweep only flips `uploading` to `failed`. Its objects are never deleted, sniffed or size-checked, which contradicts its stated goal. P0.5's AC then serves a possibly partial, unsniffed object as a downloadable original.  
**Evidence:** SPEC-01:205-209 'marking them `failed` ... so partial objects and stranded rows don't accumulate forever against Goal 3'; Goal 3 SPEC-01:27-28 'all storage objects are gone'. P0.5 AC SPEC-01:283-285 'Given an asset in processing/failed whose source object still exists ... still downloadable', yet SPEC-01:289-291 says the object 'may be partial'. P0.5 streams 'the sniffed content type' (:270-271), and an abandoned asset was never sniffed.  
**Fix (revised by verify):** In the P0.5 ACs, add: 'Given an asset `failed` with `error_message='upload abandoned'` (never passed `/complete`), then 404 `media/asset-not-found`; its object is deleted when the owner deletes the asset or by the sweep.' Optionally have the sweep delete any object at `source_key` before setting `failed`.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.5 AC: abandoned asset's original → 404 media/asset-not-found. Code follow-up: OriginalContent blocks only uploading/deleting.

#### - [c] F036 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.3 Delete asset
*Category:* undefined behavior  
**Problem:** The spec does not define the DELETE response when the storage purge fails mid-request, or when DELETE hits an asset already in `deleting`. The frontend's optimistic removal cannot interpret an error.  
**Evidence:** SPEC-01:191-195 steps (2) set deleting, (3) delete objects, (4) delete rows, with no response for a step-3 failure. §7 SPEC-01:400 '204; idempotent 404'. ACs :227-233 cover only an already-deleted id and janitor recovery. SPEC-01:252 describes 'optimistic removal'.  
**Fix (revised by verify):** Add to P0.3: 'Once step (2) commits, the delete is irrevocable. If step (3) or (4) fails, the API still returns 204 and logs a warning, and the janitor completes the purge after the 15-min grace. A DELETE on an asset already in `deleting` returns 204. Only a missing row returns 404 `media/asset-not-found`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.3 Response paragraph (204 once deleting commits, janitor finishes failed purge, repeat 204, 404 only for missing row); AC + §7. Code follow-up: handler returns 500 about:blank on purge failure.

#### - [c] F037 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.3 vs P0.1/P0.2 workers
*Category:* undefined behavior / storage leak  
**Problem:** Deleting an asset while its worker task is in flight is unspecified. The worker can upload variants after the purge and then set `ready` or insert variant rows on a deleted asset. That orphans storage nothing ever sweeps, violating Goal 3.  
**Evidence:** P0.3 SPEC-01:191-195 purges without checking `processing`. The janitor (:202-204) scans only `status='deleting'`, and the row is gone after step 4. P0.1 step 4 (:116) 'Insert media_asset_variants rows; set status='ready'' has no guard. The library offers delete on `processing` cards (:245-252).  
**Fix (revised by verify):** Add to P0.3: 'Workers finish under a status guard inside their existing transaction. process_image and transcode use `UPDATE assets SET status='ready' … WHERE id=$1 AND status='processing'`. The thumbnail worker inserts its `poster` row only if the asset exists and `status <> 'deleting'`. When the guard affects 0 rows, or a variant insert hits the FK or a missing row, the transaction is rolled back (no variant rows, no `media:asset_ready`). The worker then deletes every object it uploaded for the asset and returns nil, so the task is not retried.' AC: 'Given an asset deleted while `processing`, or while its poster task is in flight, when the worker finishes, then no objects remain under its prefix, no variant rows exist, and no `media:asset_ready` is emitted.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.3 delete-while-in-flight status guard + P0.1 step 4 cross-ref + AC. Code follow-up: MarkAssetReady/MarkAssetImageReady lack status='processing' predicate; workers don't clean up.

#### - [c] F038 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P1.2, P2, §6
*Category:* missing contract  
**Problem:** The spec relies on a 'mediaapi batch-create SPEC-02 P1.7 uses' to set origin='import' but never defines it, and P2 even says 'design for, don't build' bulk upload. Nothing says which gates (sniff, HEIC rejection, 50 MB cap) apply on that non-/complete path.  
**Evidence:** SPEC-01:315-317 ''import' by the mediaapi batch-create SPEC-02 P1.7 uses'; :386-388; P2 :328-329 'API shape should not preclude a batch-create'. SPEC-02:276-281 'batch-created assets start `processing`', which bypasses /complete, where the only sniff and size checks live (SPEC-01:84-98).  
**Fix (revised by verify):** Add P1.3: '**mediaapi ingest for server-side producers**: `mediaapi.Ingest(ctx, ownerID, filename, contentType string, data []byte, origin string) (uuid.UUID, error)` runs the same create → store → `/complete` path as a browser upload, so the magic-byte sniff, HEIC rejection and 50 MB cap apply unchanged, with the same errors as the HTTP Problem types. The row is created with the given `origin` (`import` for SPEC-02 P1.7 and other bulk producers; the HTTP upload-session endpoint always passes `upload`). Media owns it; SPEC-02 P1.7 is the first caller.' Replace the 'mediaapi batch-create' wording in P1.2, P2 and §6 with 'P1.3 `mediaapi.Ingest`'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — New P1.3 mediaapi.Ingest(..., origin); "batch-create" wording replaced in P1.2/P2/§6. Code follow-up: Ingest has no origin param (always 'upload').

#### - [c] F039 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.1 /complete sniffing, P0.5, §6
*Category:* ambiguity  
**Problem:** 'Magic bytes decide', but the spec never says how the sniffed type is stored, or what happens when it disagrees with the kind or content type declared at session start. P0.5 depends on 'the sniffed content type'.  
**Evidence:** SPEC-01:81-87 'asset row is created with kind='image' ... at session start ... magic bytes decide — never file extension or client-declared Content-Type'. SPEC-01:270-273 streams 'the sniffed content type' and derives `ext` from it. The §6 ALTER (:370-374) adds no content-type column.  
**Fix (revised by verify):** Add to P0.1: 'For `kind='image'`, `/complete` overwrites `assets.mime_type` with the sniffed type (image/jpeg|png|webp). A body whose magic bytes match no accepted image type, including a video or other non-image file sent with an image content type, follows the existing unsupported-format rule (422, object deleted, `failed`). Video and audio kinds are not sniffed at v1; their `mime_type` stays client-declared.' P0.5: 'served with `assets.mime_type` (sniffed for images, declared otherwise)'. No §6 change.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.1 "Sniffed type is stored"; P0.5 serves assets.mime_type. Code follow-up: completeImage discards the sniffed type.

#### - [x] F040 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.2 Video poster
*Category:* ambiguity  
**Problem:** The poster is generated 'after (or as the final step of) transcode'. That either/or decides whether `ready` and `media:asset_ready` fire before the poster exists, and it conflicts with the AC. The registry lists `media:thumbnail` as a separate task.  
**Evidence:** SPEC-01:165-166 'after (or as the final step of) transcode'; AC :173-174 'when transcode completes, then a `poster` variant exists'; :125 lists thumbnail as a separate light task; events.md:45 '`media:thumbnail` ... "thumbnail" queue'.  
**Fix (revised by verify):** Replace 'after (or as the final step of) transcode' with 'On transcode success the worker sets `ready` and enqueues `media:thumbnail {asset_id, source_key, owner_user_id}` on the `thumbnail` queue. The poster never gates `ready`.' Change the AC to 'within one thumbnail-task cycle after `ready`, a `poster` variant exists'.  
**Applied:** ✅ P0.2: transcode success → ready + media:thumbnail on the thumbnail queue; poster never gates ready; AC. Matches code.

#### - [x] F041 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.6, P0.3
*Category:* acceptance-criteria gap  
**Problem:** Several P0 behaviours have no AC: the P0.6 `platform/events` fan-out, the `media:asset_deleted` emission (including the janitor path), the abandoned-upload sweep, and the shared-scheduler registration.  
**Evidence:** P0.6 (SPEC-01:293-304) has no AC block. The P0.3 ACs (:224-233) do not cover `media:asset_deleted` (:195-199), the `uploading` sweep (:205-209), or the single `asynq.Scheduler` (:215-222).  
**Fix (revised by verify):** P0.6 AC: 'Given two subscriptions for an event, Publish enqueues exactly one task per subscriber; given none, Publish returns nil and enqueues nothing; an enqueue error propagates.' P0.3 ACs: 'Given a successful delete (API or janitor path), exactly one `media:asset_deleted {asset_id, owner_user_id}` is published after the row is gone'; 'Given an `uploading` asset older than 24 h, the janitor marks it `failed` with `upload abandoned`'; 'Given cmd/worker starts, `media:purge_orphans` is registered hourly on the single shared asynq.Scheduler'.  
**Applied:** ✅ P0.6 AC block (fan-out per subscriber, no-subscription nil, enqueue error propagates); P0.3 ACs for asset_deleted emission, 24 h sweep, hourly scheduler registration.

### SPEC-02-comic-vertical.md

#### - [c] F042 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · §3 Non-goals vs header rev 13/14 (P1.8 sync)
*Category:* brief-coverage / internal-contradiction  
**Problem:** External import/scraping is still a non-goal, yet revs 13–14 add a shipped external-source sync feature. That feature has no requirement section, ACs, §6 schema or config, and it is labelled 'P1.8', which is the Bookmarks id.  
**Evidence:** SPEC-02:44 '- Import/scraping from external comic sources.' vs :5 '**rev 13** ... **P1.8 external-source sync (full UI feature).** ... `comic_sync_sources`, migration `0028`' and :4 rev 14 (0029/0030, status `cancelled`). :301 '- **P1.8 Bookmarks**'. §6 (:382-423) has no `comic_sync_sources`.  
**Fix (revised by verify):** 1) Replace :44 with '- Import/scraping from external sources was a v1 non-goal; reversed by rev 13 (2026-08-15), see P1.10. Still out of scope: scheduled/automatic re-sync, and passing anti-bot challenges from the containerised scraper (host-mode only).' 2) In the rev 13 and rev 14 header lines, rename 'P1.8 external-source sync' / 'P1.8 sync' to 'P1.10 external-source sync'. 3) Add '### P1.10 - External-source sync (revs 13-14)' after P1.9, covering: the flow (TriggerSync -> scraper discovery -> one comic_imports job per batch via sync-batch -> comic:import_zip {import_id}); config COMIC_SCRAPER_URL (unset => feature off; sync-source endpoints return 404 Problem comic/sync-disabled), COMIC_SYNC_SECRET, SCRAPER_BATCH_SIZE (default 1), SCRAPER_BATCH_RETRIES (default 3); and cancel semantics. ACs: (a) with batch size 1, each chapter appears in the chapter list as soon as it is scraped, and scraped_chapters counts imported chapters 1:1; (b) cancel mid-sync -> status 'cancelled', the in-flight chapter finishes, and chapters already imported are kept; (c) a batch still yielding 0 images after SCRAPER_BATCH_RETRIES attempts is listed in last_error and the sync continues. 4) Add the comic_sync_sources DDL to section 6, transcribed verbatim from the shipped 0028/0029/0030 migrations rather than invented. It must include tenant_id plus RLS, comic_id FK ON DELETE CASCADE, the full status CHECK including 'cancelled' (0030), total_chapters/scraped_chapters (0029), last_error, and the owning-user column (created_by, or whatever 0028 names it) that runInUserTenant reads (see finding #1).  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §3 non-goal replaced; revs 13/14 relabelled P1.10; new §5 P1.10 (flow, config, cancel, ACs incl. re-sync + owner-404); §6 comic_sync_sources DDL from 0028–0030. Code follow-up: COMIC_SCRAPER_URL unset should 404 comic/sync-disabled (shipped: CRUD served, trigger 500).

#### - [c] F043 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · §7 — internal sync endpoints
*Category:* security / missing-authz  
**Problem:** The internal endpoints write data outside `authTenant` and impersonate a user tenant, but the spec gives none of them a secret-verification rule, network restriction, or rule for where the tenant is resolved from.  
**Evidence:** SPEC-02:4 '`sync-batch` runs outside `authTenant`, so `RequestSyncBatch` creates the batch import via an API-side `runInUserTenant`'; :5 'hits the shared-secret `POST /internal/comic/sync-callback`'. README.md:58-59 'every endpoint lists its required permission'. §7 has no `/internal/comic/*` rows. Code: comic/module.go:100-103,195 check only X-Internal-Secret.  
**Fix (revised by verify):** Add a section 7 sub-table 'Internal (scraper -> API) endpoints': POST /internal/comic/sync-callback, /internal/comic/sync-batch, /internal/comic/sync-progress, /internal/comic/sync-finalize. Permission column: 'internal: header X-Internal-Secret must equal COMIC_SYNC_SECRET (constant-time compare); missing or mismatched -> 401 Problem; when COMIC_SYNC_SECRET is empty the routes are not mounted (404), so an empty secret never authenticates.' Add a note: 'Served on the API but blocked at the public reverse proxy (/internal/* is not routed). Exempt from the shared/openapi.yaml rule (README Conventions > API contract) because these are private service-to-service calls, not a public contract. Documented here instead.' Tenant resolution: 'runInUserTenant derives tenant_id and user_id from the stored source row (comic_sync_sources.tenant_id, plus the owning user: comic_sync_sources.created_by, or comics.owner_user_id via comic_id) looked up by source_id. Tenant or user ids in the request body are ignored.' Also: 'API -> scraper calls (scrape, /cancel) send the same X-Internal-Secret; the scraper service is internal-only (no published port).'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §7 "Internal (scraper → API) endpoints" table (4 routes) + Secret/Exposure/Tenant resolution/API→scraper rules; shipped owner re-verify inside runInUserTenant kept. Code follow-up: constant-time compare; stop routing /api/v1/internal/* publicly; cmd/api/scraper.go sends no X-Internal-Secret; openapi 401 vs handler 404.

#### - [x] F044 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · P1.7, §7 vs revs 9/10/13/14
*Category:* wrong API / rbac-coverage  
**Problem:** P1.7 and §7 still describe a presigned PUT plus `POST /chapters/{id}/pages:import-zip {upload_ref}`, which was never built. The shipped import-job and sync endpoints (about a dozen) are absent from §7, with no permission and no Problem types, which breaks the README rule that every endpoint lists its permission.  
**Evidence:** SPEC-02:270-275 'dedicated presigned PUT ... `pages:import-zip {upload_ref}` ... `comic:import_zip {chapter_id, upload_ref}`'; §7:469 same. vs :13 rev 9 '`POST /chapters/{id}/imports`, `PUT /imports/{id}/zip` (API-proxied ...), `GET /imports/{id}`'; :12 rev 10 '`POST /comics/{id}/imports`'; :5 and :4 sync-source endpoints and cancel; events.md:47 '`comic:import_zip` | `{import_id}`'. §7 Problem types (:471-473) stop at `comic/zip-rejected`. Code: comic/module.go:92-94,126-127 mount sync and imports routes with no RBAC middleware.  
**Fix (revised by verify):** As proposed. Rewrite P1.7 around `POST /chapters/{id}/imports` | `POST /comics/{id}/imports` → `PUT /imports/{id}/zip` → `comic:import_zip {import_id}` → poll `GET /imports/{id}`. In §7, delete the `pages:import-zip` row and add rows, each with an explicit permission, for the two import-create endpoints, `PUT /imports/{id}/zip`, `GET /imports/{id}`, `GET/POST /comics/{id}/sync-sources`, `POST /sync-sources/{id}/sync`, `POST /sync-sources/{id}/cancel` and `DELETE /sync-sources/{id}`. State whether these are owner-only (the shipped handler check) or owner-or-`comics:write:any`. Add Problem types for import-not-found and sync-source-not-found, and add `comic_imports` to §6.  
**Applied:** ✅ P1.7 rewritten around the shipped imports flow (POST imports → PUT zip → comic:import_zip → poll); §7 pages:import-zip row deleted, 9 import/sync-source rows added (owner-only); comic/not-found reused; comic_imports DDL in §6.

#### - [x] F045 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · P1.7 guards vs rev 10
*Category:* internal-contradiction  
**Problem:** The P1.7 guard list (500 MB, 300 entries, nested directories rejected) contradicts whole-comic import (3 GB, 20,000 entries, grouping by top-level folder, which needs nesting). The spec never says which limits apply to which endpoint.  
**Evidence:** SPEC-02:290-292 'max 500 MB zip, max 300 entries ... reject nested directories' vs :12 'groups images by top-level folder → one chapter per folder ... limits raised to 3 GB / 20000 entries'. The timeout text (:283-286) is still sized to 'a full 300-entry import'.  
**Fix (revised by verify):** Replace the guard sentence at :289-293 with a table. Per-chapter (POST /chapters/{id}/imports): zip <= 500 MB, <= 300 image entries, flat (any directory entry -> 422 comic/zip-rejected). Whole-comic (POST /comics/{id}/imports): zip <= 3 GB, <= 20,000 image entries, exactly one level of chapter folders (a single top-level wrapper folder is stripped first), each folder becomes one chapter in natural-sort order; deeper nesting or loose root-level images (after stripping the wrapper) -> 422 comic/zip-rejected. Both modes: images only by magic bytes, '../' and absolute paths rejected, compression ratio > 100:1 rejected, pages in filename natural-sort order, per-file failure report. Delete the parenthetical at :283-287 ('bounded timeout = max(10 min, ...) ... reported as failures') and state once: 'Poll timeout = max(2 min, entry_count x 5 s), capped at 11 h (under the 12 h task lease, rev 11), for both modes.' Add ACs: 'per-chapter zip with a subdirectory -> 422 comic/zip-rejected, no pages'; 'whole-comic zip wrapper/ch1/*, wrapper/ch2/* -> two chapters ch1, ch2 in natural order'.  
**Applied:** ✅ Guard sentence → per-mode table (shared importMaxZipBytes 16 GiB / importMaxEntries 100,000; per-chapter flattens, whole-comic one chapter per folder; bad entries skipped; oversize 422 comic/validation); ACs.

#### - [x] F046 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · §6, §7 PATCH row, R3
*Category:* internal-contradiction / data-model  
**Problem:** R3 adds `comics.reading_direction` to the payloads and `ComicPatch`, but §6 omits the column, §6's scope note still calls it future P2, and §7 says PATCH updates title/description/cover only.  
**Evidence:** SPEC-02:337-339 '`comics.reading_direction text NOT NULL DEFAULT 'vertical' CHECK (...)` — migration `0024_comic_reading_direction`'; :362-363 'on `Comic`/`ComicPatch`' vs §6 `CREATE TABLE comics` (:385-394) without it; :446 '(tags, RTL direction — P2)'; :457 'update title/description/cover only'.  
**Fix (revised by verify):** As proposed. Add `reading_direction text NOT NULL DEFAULT 'vertical' CHECK (reading_direction IN ('ltr','rtl','vertical')), -- R3, 0024_comic_reading_direction` to the §6 comics DDL. Change :446 to '(e.g. tags)'. Change §7:457 to 'update title/description/cover/reading_direction (`ComicPatch`) — status is NOT changed here'.  
**Applied:** ✅ reading_direction in §6 comics DDL; scope note "(e.g. tags)"; §7 PATCH covers title/description/cover/reading_direction.

#### - [x] F047 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · Reader redesign — Image quality; §7 reader payload
*Category:* unimplementable / api-contract  
**Problem:** 'High' quality maps to a `poster`/large variant that images never get: SPEC-01 makes only thumb and medium for images, and poster only for video. The reader payload has no `asset_id`, so the client cannot call `variantURL(assetId, variant)` to switch quality.  
**Evidence:** SPEC-02:352-353 'Data-saver = `thumb`, Standard = `medium`, High = `poster`/large — via the existing `variantURL(assetId, variant)`'; :380 'quality selects the image variant'; :467 'reader payload: `{pages: [{page_id, url(medium), width, height}]}`'. SPEC-01:114-115 (images: thumb 320, medium 1280); :167 (poster is video-only).  
**Fix (revised by verify):** As proposed. Change :352-353 to 'Data-saver = `thumb`, Standard = `medium` (default); High = `medium` until SPEC-01 adds a larger image variant (images have no `poster`; originals are never served to the reader).' Change the §7:467 payload to `{pages: [{page_id, asset_id, sort_order, url (medium), width, height}]}`, with width and height taken from the medium variant.  
**Applied:** ✅ Image quality High falls back to medium until SPEC-01 adds a larger variant; §7 reader payload aligned to shipped {pages:[{page_id, asset_id, width, height}]}.

#### - [c] F048 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · §6 indexes; §7 GET /comics, /comics/mine; P0.5 AC
*Category:* pagination / data-model  
**Problem:** The public list pages by keyset on `updated_at`, which changes on every edit and has no `id` tiebreaker. That breaks the spec's own 'no duplicates' AC and the cursor convention it cites. `/comics/mine` has no order and no cursor index.  
**Evidence:** SPEC-02:453 'cursor paging, order `status, updated_at DESC` (the §6 index) — matches the SPEC-01/04/05/06/08 cursor convention'; SPEC-01:248 defines the convention as `created_at DESC, id DESC`. §6 :395 `CREATE INDEX ON comics (status, updated_at DESC);`, :396 `(owner_user_id)`. :227 'paginates without duplicates'. :454 `/comics/mine?cursor=` gives no order. Shipped 0015 added `id DESC` ad hoc.  
**Fix (revised by verify):** Decide: the library is ordered 'recently updated' (as shipped in 0015). Section 6: replace the two indexes with 'CREATE INDEX comics_published_cursor_idx ON comics (updated_at DESC, id DESC) WHERE status = ''published'';' and 'CREATE INDEX comics_owner_cursor_idx ON comics (owner_user_id, updated_at DESC, id DESC);', and add the note 'shipped as 0015; supersedes the earlier (status, updated_at DESC)'. Section 7 GET /comics: 'drafts excluded; keyset paging ordered updated_at DESC, id DESC; opaque cursor encodes (updated_at, id). The id tiebreaker is mandatory. This deliberately differs from the created_at DESC, id DESC convention (SPEC-01:248) because the library sorts by recent activity.' Section 7 GET /comics/mine: 'incl. drafts; same keyset (updated_at DESC, id DESC) over owner_user_id'. P0.5 AC :227 -> 'A result set of >1 page paginates with no duplicates or skips, provided no comic in the set is modified between page fetches (an edited comic legitimately moves to the head).'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §6 keyset (updated_at DESC, id DESC); §7 order/cursor/tiebreaker/limits 30/50; P0.5 AC. Code follow-up: comics_owner_cursor_idx target (0015 shipped only owner_user_id).

#### - [c] F049 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · P0.2 publish validation; P0.6; P1.9
*Category:* missing edge case  
**Problem:** The publishable invariant (≥1 chapter, each chapter ≥1 page) is checked only at publish. The spec never says what happens afterwards when a chapter is added, emptied (including by the P0.6 reaper), or deleted, or whether any of those emit `comic:chapter_published`.  
**Evidence:** SPEC-02:136-138 'may be published only if it has ≥1 chapter and every chapter has ≥1 page'; :246-248 P0.6 deletes pages regardless of status; :306 P1.9 'emit `comic:chapter_published`' with no trigger defined. Chapters have no status (§6 :398-405). events.md:23 'emitted per chapter on comic publish'.  
**Fix (revised by verify):** Add to P0.2 after :138: 'The invariant is enforced at publish only. On a published comic: (a) a chapter with 0 pages is omitted from the chapter list, the chapter count, the reader chapter picker and prev/next navigation, and GET /chapters/{id}/pages for it returns 404 comic/not-found to non-owners, until it has >= 1 page; (b) comic:chapter_published is emitted once per non-empty chapter when the comic is published, and once for any chapter of an already-published comic when it gains its first page (manual add, zip import or sync); (c) deleting pages or chapters, including via the P0.6 reaper, never auto-unpublishes; a published comic left with no non-empty chapters simply shows no chapters.' Add one AC per clause. Change P1.9 to 'emit ... on the triggers defined in P0.2 (b)', and update events.md:23 'Emitted when' to 'per non-empty chapter on comic publish, and when a chapter of a published comic gains its first page' in the same change.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 post-publish invariant (a)/(b)/(c) with ACs; P1.9 cross-ref (one comic:published per publish, deduped by chapter_count). Code follow-up: GetComic/ReaderPages/chapter_count include empty chapters.

#### - [c] F050 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · P0.1 asset ownership vs P0.2 moderation / P1.7 import
*Category:* ambiguity  
**Problem:** Page and cover assets must be 'owned by the creator', but `comics:write:any` editors can edit other people's comics, and imports create assets on someone's behalf. The spec never says whose ownership is checked or who owns imported assets.  
**Evidence:** SPEC-02:70-73 'must reference a **ready image** asset owned by the creator'; :123 'update / reorder / add pages ... owner, **or** `comics:write:any`'; :276-277 import 'creates media assets via `mediaapi`' with no owner.  
**Fix (revised by verify):** Add to P0.1: 'Owned by the creator means the asset's owner, as returned by mediaapi (assets.owner_id), equals comics.owner_user_id of the target comic, whoever the caller is (owner or a comics:write:any editor). Assets created by zip import (P1.7) or sync (P1.10) are ingested via mediaapi.IngestImage with owner = comics.owner_user_id, never the requesting editor.' AC: 'Given editor E (comics:write:any) attaching an asset E owns to creator C's comic, then 422 comic/invalid-page-asset (or comic/invalid-cover-asset for a cover) and no change.' AC: 'Given an editor runs a zip import on C's comic, then every created asset's owner is C.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.1 asset ownership = comics.owner_user_id; imports/syncs ingest as comic owner; 2 ACs. Code follow-up: RunImport ingests as the caller, so an editor's import is rejected by CreatePages.

#### - [c] F051 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · P0.2 publish RBAC; §7 publish/unpublish
*Category:* rbac-composition  
**Problem:** The spec says publish is 'owner + comics:publish:own, or comics:publish:any'. `RequireOwnerOrPermission` cannot express that, because its owner branch admits the owner with no permission check. As written, `comics:publish:own` is never checked, and revoking it has no effect. That is what shipped.  
**Evidence:** SPEC-02:101-102 '(owner still needs comics:publish:own)'; :124 and :458-459. rbac/engine.go:80-83 `if ownerID == p.UserID { return nil }`. cmd/api/main.go:514 wires only `RequireOwnerOrPermission(engine, "comics:publish:any", byComic)`.  
**Fix (revised by verify):** P0.2 table row :124 and section 7 rows :458-459 -> 'RequirePermission("comics:publish:own") chained before RequireOwnerOrPermission(engine, "comics:publish:any", extractComicOwner). An owner passes only while holding comics:publish:own. Editors pass the first check because the matcher lets an :any grant satisfy the same code's :own check (and user already holds :own via 0025).' Replace ':101-102 (owner still needs comics:publish:own)' with 'enforced by the chained check above; unlike SPEC-01 assets:delete:own, this :own code is not merely documentary'. AC: 'Given an owner whose effective permissions lack comics:publish:own, when POST /comics/{id}/publish, then 403 and status unchanged.' Record the movies/music/stories PublishMW alignment as a separate backlog item, not in SPEC-02.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 + §7 publish/unpublish chain RequirePermission("comics:publish:own") before RequireOwnerOrPermission(comics:publish:any); owner-lacks-:own AC. Code follow-up: main.go wires only RequireOwnerOrPermission.

#### - [x] F052 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · P0.4 / P0.5 / §7
*Category:* api-contract  
**Problem:** The detail page must show 'Continue reading → ch. N, p. M' only when progress exists, but §7 defines no read path for progress (only the PUT). The frontend cannot learn N or M, and the improvised schema carries neither.  
**Evidence:** SPEC-02:183-187 'Detail page shows **Continue reading → ch. N, p. M** (M = the page's current 1-based position, computed when the detail page is fetched)'; :229; §7:456 `GET /comics/{id}` has empty notes; :468 has only PUT progress. openapi.yaml:4080-4086 `ComicDetail.progress {chapter_id, page_id, updated_at}`.  
**Fix (revised by verify):** Fill the §7:456 note with: 'returns `ComicDetail` = Comic + `chapters[]` (ordered) + `progress: {chapter_id, page_id|null, updated_at} | null` for the caller; the client derives ch. N from the chapter's index and p. M from the page's index in `GET /chapters/{id}/pages`'. Alternatively, add server-computed `chapter_number`/`page_number` fields, and change P0.4's wording to whichever option is chosen.  
**Applied:** ✅ §7 GET /comics/{id} describes shipped ComicDetail (chapters[] + progress); P0.4 client derives N/M.

### SPEC-03-finance-ledger.md

#### - [x] F053 · 🟠 MAJOR · `SPEC-03-finance-ledger.md` · P0.2 / P0.3 / P0.8 / §6
*Category:* security / ambiguity  
**Problem:** Category ownership is spelled out, but account ownership never is. `account_id` on transactions and `from_account`/`to_account` on transfers have no ownership rule, and the DB does not guarantee that `bank_transactions.user_id` matches the account owner. A foreign account_id would change the victim's derived balance.  
**Evidence:** SPEC-03:102-104 (category rules only). §6 :426-427 has no composite constraint. :79 derives balance per account. P0.8 ACs (:329-330) test only list and fetch.  
**Fix (revised by verify):** Add to P0.2, after the category attachment rules: 'Account attachment rules: `account_id` on POST/PATCH /bank/transactions, and `from_account`/`to_account` on POST/PATCH /bank/transfers, must resolve to an account the caller owns (owner-scoped lookup, `user_id = caller`). A foreign or nonexistent id is 404 `bank/not-found`, as if it didn't exist, never a 422 that confirms existence. Derived-balance, dashboard and report queries filter on `user_id = caller` as well as `account_id`.' Add a P0.8 AC: 'Given user B's account id, when user A POSTs a transaction or a transfer naming it (or PATCHes a transaction onto it), then 404 `bank/not-found`, no row is written, and B's derived balance is unchanged.'  
**Applied:** ✅ P0.2 "Account attachment rules": owner-scoped accounts, foreign/unknown id → 404 bank/not-found, derived queries filter user_id; P0.8 AC.

#### - [x] F054 · 🟠 MAJOR · `SPEC-03-finance-ledger.md` · P0.3 Transfers / §7
*Category:* ambiguity  
**Problem:** PATCH /bank/transfers/{transfer_id} has no defined body or validation: which fields are mutable, whether accounts can change, and whether the POST checks re-run are all unstated.  
**Evidence:** SPEC-03:128-134 gives only the POST body, then 'PATCH/DELETE ... mutates/removes both legs'. §7 :472 lists the permission only. P0.3 ACs (:151-160) do not cover edits, although user story :68-69 requires corrections.  
**Fix (revised by verify):** Add to P0.3: 'PATCH /bank/transfers/{transfer_id} accepts any subset of `{from_account, to_account, amount, occurred_at, note}` and rewrites both legs atomically. The POST validations re-run on the resulting pair: own accounts (404), `bank/same-account-transfer`, `bank/currency-mismatch`, `bank/invalid-amount`. Each leg emits one `bank:transaction_updated`.' AC: 'Given a TCB→Momo 5,000,000 transfer PATCHed to 4,000,000, then TCB and Momo each move back by 1,000,000 and month income/expense move by 0.'  
**Applied:** ✅ P0.3 transfer PATCH body {from_account,to_account,amount,occurred_at,note} (any subset), POST validations re-run, one transaction_updated per leg; 5M→4M AC. Matches UpdateTransfer.

#### - [x] F055 · 🟠 MAJOR · `SPEC-03-finance-ledger.md` · P0.2 / §6 CHECK
*Category:* data integrity  
**Problem:** The spec never says that `transfer_id` is server-assigned only, and gives no error for a missing category. A client-supplied `transfer_id` with a NULL category passes the CHECK and creates a one-sided 'leg' that is invisible to income/expense. A plain missing category hits the DB CHECK and returns 500.  
**Evidence:** SPEC-03:98 'category (required unless transfer leg)'; :137 leg predicate; §6 :439 `CHECK (category_id IS NOT NULL OR transfer_id IS NOT NULL)`. The §7 problems (:485-490) have no missing-category code.  
**Fix (revised by verify):** Add to P0.2: '`transfer_id` is server-assigned only. It is not part of the POST/PATCH /bank/transactions request schema and is never read from a request body. Only /bank/transfers mints it, so a client cannot create a one-sided leg. On the manual path `category_id` is required: omitted or null → 422 `bank/validation` (checked in the service before the insert, never left to the §6 CHECK, which would surface as a 500).' Append `bank/validation` (422, generic request-shape violations) to the §7 Problem types. Add a P0.2 AC: 'Given POST /bank/transactions without `category_id`, then 422 `bank/validation` and no row is written.'  
**Applied:** ✅ P0.2 transfer_id server-assigned only; missing category_id → 422 bank/validation; AC; §7 note.

#### - [c] F056 · 🟠 MAJOR · `SPEC-03-finance-ledger.md` · P0.2/P0.4/P0.8 / §7 Problem types
*Category:* problem-type closure  
**Problem:** There are many mandated 404 paths but no not-found Problem type. `bank/category-immutable` is declared, but the spec never says what triggers it or what status it returns, so implementers must guess between 409 and 422.  
**Evidence:** SPEC-03:104, :177-178, :194, :330 each mandate a 404 'as if it didn't exist'; §7 :485-490 has no not-found type. :178-180 '`kind` is immutable after creation (mirror of P0.1's currency immutability)', with no status; :490 declares `bank/category-immutable`. P0.1's mirror uses 409 (:90-91).  
**Fix (revised by verify):** Append to the §7 Problem types: '`bank/not-found` (404) — a single type for every missing or foreign account, transaction, transfer or category id, including ids outside the caller's visible set (existence never leaks).' Change P0.4 :179-180 to: '`kind` is immutable after creation — a PATCH that changes it is 422 `bank/category-immutable` (the request is well-formed but semantically invalid; unlike P0.1's currency, `kind` is unconditionally immutable, so it is a validation failure, not a state conflict).' Add a P0.4 AC: 'Given a PATCH changing an own category's `kind`, then 422 `bank/category-immutable` and the row is unchanged.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §7 bank/not-found (404); P0.4 kind change → 422 bank/category-immutable + AC. Code follow-up: category PATCH ignores kind instead of rejecting it.

#### - [c] F057 · 🟠 MAJOR · `SPEC-03-finance-ledger.md` · P0.6 Dashboard / §7
*Category:* api-contract  
**Problem:** SPEC-06's finance widget depends on `/bank/dashboard`, but the response has no field-level shape and `?month=` has no default. The widget calls it without a month.  
**Evidence:** SPEC-03:258-268 is prose only; :476 `/bank/dashboard?month=` with no default; :272 example `?month=2026-06`. SPEC-06:197 'Finance month card | `GET /bank/dashboard`'; SPEC-06:53-54 'month-to-date spend'.  
**Fix (revised by verify):** P0.6: '`month` is `YYYY-MM`. When omitted, it defaults to the current month in `users.timezone` (D-17); a malformed value is 400 `bank/invalid-month`. Response: `{month, accounts: [{id, name, type, currency, opening_balance, balance, archived}], income, expense, budgets: [{category_id, parent_id|null, name, parent_name|null, amount|null, spent}], recent: [Transaction]}`. All money is integer minor units, and `amount: null` marks a synthesized header entry (P0.5). `income`/`expense` are single-currency sums; per-currency grouping is realised client-side by `accounts[].currency` while every account shares one currency, and the server must split the totals by currency before a second currency is allowed (tracked with the §11 budgets-vs-currency question).' Add `bank/invalid-month` to §7. Code follow-up: monthParam must use the caller's TZ, not UTC.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.6 Contract: month YYYY-MM, default current month in users.timezone (D-17), full response shape, client-side currency grouping; bank/invalid-month. Code follow-up: monthParam uses UTC month, malformed → about:blank.

#### - [x] F058 · 🟠 MAJOR · `SPEC-03-finance-ledger.md` · §8 Frontend
*Category:* missing-route  
**Problem:** P0.4 makes categories user-creatable, editable and deletable, with a reassign flow on delete that a user story depends on. §8 has no category management page, so those endpoints have no UI.  
**Evidence:** SPEC-03:169-170 'users may add their own'; :70-71 reassign story; §7:474 `PATCH/DELETE /bank/categories/{id}` (`?reassign_to=`). §8 :516-530 lists only dashboard, transactions, accounts and budgets. types.ts:65-66 later added `bankCategories`, citing 'SPEC-03 P0.4'.  
**Fix (revised by verify):** Add to §8: '- `/bank/categories`: two-level tree per kind; seeds read-only; own categories can be created, renamed and re-parented. Delete opens a reassign picker that sends `?reassign_to=`, and 409 `bank/category-in-use` renders inline.' Extend :518 to `{transactions,accounts,budgets,categories}` and list the `TemplateManifest.views` keys, including `bankCategories`.  
**Applied:** ✅ §8 /bank/categories page (tree, read-only seeds, reassign picker, inline 409); path list + TemplateManifest keys.

### SPEC-04-notification-module.md

#### - [c] F059 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.3 token storage
*Category:* security  
**Problem:** A successful reset does not invalidate the user's other outstanding tokens (up to 3 can be live), and 'short TTL' has no value. There is also no cleanup of expired or used rows.  
**Evidence:** SPEC-04:104 'single-use, short TTL'; :110 '≤3 sends per email per hour'; AC :118 says only 'the token is consumed'.  
**Fix (revised by verify):** P0.3 token storage: 'TTL = `PASSWORD_RESET_TTL` (default 1h, must be in (0, 24h]). On a successful reset, in the same transaction as the password update and `token_version` bump, set `used_at = now()` on every unused `password_reset_tokens` row of that user. Minting does not revoke older tokens. A periodic task on the shared scheduler (the SPEC-01 P0.3 runner) deletes rows that are used or expired and older than 7 d.' Extend AC :118: '…and the token is consumed, and every other outstanding reset token of that user is then rejected with 400 `account/invalid-reset-token`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.3 PASSWORD_RESET_TTL (1h, (0,24h]), revoke all unused tokens in the reset tx, 7 d purge on the shared scheduler; AC. Code follow-up: reset marks only the presented token, non-transactional; purge never scheduled.

#### - [c] F060 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.3 password_reset payload
*Category:* security  
**Problem:** The plaintext reset URL travels in Asynq payloads. Redis retains them, and SPEC-09's queue console displays them, so hashing the token at rest is defeated.  
**Evidence:** SPEC-04:106 'The channel-only payload (the reset URL) rides the `notify:email` task'; :102 dispatch intent. The Asynq archive keeps failed payloads; SPEC-09:187 mounts asynqmon, which shows payloads to `queues:read` holders.  
**Fix:** Add to P0.3: 'No Asynq payload may contain a plaintext reset token. The intent carries only `{user_id, type:"account.password_reset"}`. At send time the `notify:email` template calls `accountapi.MintPasswordResetURL(ctx, user_id)`, which mints a token, stores its hash and returns the URL. A retried send mints a fresh token. Throttling stays in forgot-password.' If that is rejected, require AES-GCM encryption of the URL with `asynq.Retention(0)`.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.3 "No plaintext reset token in any Asynq payload" (intent {user_id,type,channels}; accountapi.MintPasswordResetURL at send time; retry mints fresh); AC. Code follow-up: password_reset.go puts data.reset_url in dispatch/email payloads.

#### - [x] F061 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.2 / P0.3 email recipient
*Category:* missing decision  
**Problem:** The spec never says how `notify:email` gets the recipient address. Payloads carry only `user_id`, and the cross-module lookup is not declared.  
**Evidence:** SPEC-04:83 intent `{user_id, type, title, body, data, channels?, dedup_key?}`; events.md:53 payload `{user_id, type, title, data}`; :102 'renders a template ... and sends' with no address. Goal 6 (:28) forbids importing account internals, and the header (:4) lists no account/api dependency.  
**Fix (revised by verify):** P0.3: '`notify:email` resolves the recipient at send time through the account/api read port `ResolveRecipient(ctx, user_id) → (email, display_name, error)`. Account returns an empty email for a disabled or deleted user. On an empty address the handler logs and returns nil, dropping the send without retry. A lookup error returns a plain error, so Asynq retries. The address is never placed in any task payload.' Header Depends-on: add 'account/api (`ResolveRecipient`, plus the P0.3 reset-URL mint)'.  
**Applied:** ✅ P0.3 "Recipient resolution" (ResolveRecipient contract, empty → drop, error → retry, never in payload) + AC; header Depends-on account/api. Matches shipped UserResolver.

#### - [c] F062 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.3 / §9 phase 3
*Category:* coverage / unowned work  
**Problem:** The AC requires a working reset link, but no requirement owns the forgot-password and reset-password pages or the base URL the link uses.  
**Evidence:** SPEC-04:117 'an email appears in Mailpit with a working reset link'. P0.5 (:139-150) covers only the bell. §9 (:243) is backend-only. There is no frontend reset route and no link-origin config key.  
**Fix (revised by verify):** Add P0.3 'Frontend': '`/forgot-password` (an email form that always shows the same "check your inbox" message after 202, and maps 429 `account/rate-limited`) and `/reset-password?token=` (new-password form that maps `account/invalid-reset-token` and `account/password-policy`; the page sets `Referrer-Policy: no-referrer`). Both are public routes and must not be added to the `src/middleware.ts` auth matcher. Add a "Forgot password?" link on `/login`. The emailed link is `${PASSWORD_RESET_URL}?token=<token>`, where `PASSWORD_RESET_URL` is the existing platform/config key (.env.example), which must point at this page.' Register the three Problem types as i18n keys. Add ½ day to §9 phase 3.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.3 "Frontend" (/forgot-password, /reset-password?token=, no-referrer, public, /login link, PASSWORD_RESET_URL, problems.ts); AC; §9 phase 3 2½ days. Code follow-up: no forgot/reset pages.

#### - [c] F063 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.1, P0.5, §7 list + read-all
*Category:* api-contract  
**Problem:** Pagination is underspecified: no limit, no cursor format, and the P0.1 notes omit `next_cursor`. The read-all `before` watermark is either inclusive or exclusive, and exclusive leaves the newest item unread. It is supposed to be 'the cursor of the newest rendered item', yet items carry no cursor. The frontend invented a codec the backend rejects, so 'mark all read' fails.  
**Evidence:** SPEC-04:56 '`?status=unread|all&cursor=`' and 'returns items + `unread_count`' vs §7:216 `{items, unread_count, next_cursor}`; :66 AC pages 500 items with no page size; :58 '`?before=` watermark cursor'; :145 'the `(created_at, id)` cursor of the newest rendered item'. frontend/src/lib/notifications.ts:86-91 builds `${created_at}_${id}`; notify/service.go:531-538 decodes base64url 'created_at|id' → ErrBadCursor.  
**Fix (revised by verify):** P0.1 GET notes: '`?status=unread|all&cursor=&limit=` (limit default 50, max 100). Returns `{items, unread_count, next_cursor}`. `next_cursor` and each item's `cursor` field are opaque (server-encoded base64url of `(created_at, id)`), and `next_cursor` is null on the last page. Clients never construct cursors.' read-all row: '`?before=<an item's cursor>`, inclusive: marks read every unread row at or older than that item (absent = all). A malformed cursor returns 400.' P0.5: 'read-all sends `before` = the `cursor` field of the newest rendered item.' Add AC: 'Given `before` = the newest rendered item's cursor, then that item is read and rows created after it stay unread.' Add the item `cursor` field to §7 and shared/openapi.yaml.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.1 limit 50/100, {items, unread_count, next_cursor}, inclusive before, opaque cursor + AC; P0.5 read-all uses item cursor; §7. Code follow-up: items carry no cursor; watermarkCursor format rejected by backend; openapi item cursor.

#### - [c] F064 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.3 global send ceiling
*Category:* unsafe mechanism  
**Problem:** 'Pause the channel' is undefined. The obvious implementation, pausing the shared `default` queue, stalls every light task. The spec names no config key, default or counter store.  
**Evidence:** SPEC-04:112 'crossing it pauses the channel'; AC :122 'tasks re-queue/park'; :87 puts `notify:email` on the shared `default` queue with janitor work.  
**Fix (revised by verify):** Replace ':112 crossing it pauses the channel' with: '`NOTIFY_EMAIL_HOURLY_CAP` (default 200; 0 = uncapped), counted in Dragonfly as `notify:email:sent:<UTC YYYYMMDDHH>` (INCR after each successful send, EXPIRE ~65 min). Before sending, a `notify:email` task over the cap returns a plain error so Asynq retries it later. A `RetryDelayFunc` should delay these tasks to the next hour boundary so a breach cannot burn the retry budget. The handler error-logs the breach. The `default` queue is never paused, and a Dragonfly outage fails open.' AC :122: '…then `notify:email` tasks are retried after the hour rolls over (no queue pause; other `default`-queue tasks keep running), an error is logged, and `/auth/forgot-password` still answers 202.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.3 NOTIFY_EMAIL_HOURLY_CAP (200, 0 = uncapped), Dragonfly hour key, next-hour RetryDelayFunc, default queue never paused, fail-open; AC. Code follow-up: no RetryDelayFunc.

#### - [c] F065 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.3 per-IP throttle; P1.1; P1.3; §7
*Category:* untestable AC / problem types  
**Problem:** The per-IP throttle has no rate or window and no Problem type for its 429, so the AC cannot be tested and the README Problem/i18n DoD cannot be met. The P1.3 preferences and P1.1 push endpoints also have no validation Problem types.  
**Evidence:** SPEC-04:111 'Per-IP throttle ... 429' (no rate); AC :121 'Given an IP-level flood, then 429'; §7:225 lists only `notify/notification-not-found`, `account/invalid-reset-token`, `account/password-policy`. README.md:52-57 i18n DoD.  
**Fix (revised by verify):** :111 → 'Per-IP throttle on `/auth/forgot-password` and `/auth/reset-password`: 10 requests per IP per minute per endpoint (Dragonfly INCR+EXPIRE, keyed on the client IP taken from Traefik's forwarded header, fail-open). The response is 429 Problem `account/rate-limited` with `Retry-After` and a generic detail that never mentions an email.' AC :121 → 'Given 11 POSTs from one IP within one minute, then the 11th is 429 `account/rate-limited` and its body reveals nothing about any email.' Append to §7 Problem types: '`account/rate-limited` (429), `notify/invalid-preference` (422), `notify/type-not-mutable` (422), `notify/invalid-push-subscription` (422), `notify/push-subscription-not-found` (404)'. Each must be registered as an i18n key per the README DoD.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.3 per-IP throttle 10/IP/min/endpoint, fail-open, 429 account/rate-limited + Retry-After; AC; §7 Problem list + problems.ts DoD. Code follow-up: one shared counter, no Retry-After, slugs missing from problems.ts.

#### - [c] F066 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.2 intent `channels`
*Category:* contract gap  
**Problem:** The allowed values of `channels` are never defined. The spec uses 'email' and 'in_app', while the prefs column is `push`, the task is `notify:web_push`, and MODULES.md cites a `notify:in_app` task that does not exist.  
**Evidence:** SPEC-04:83 `channels?`; :102 `["email"]`; :157 `["email","in_app"]`; §6 :191 column `push`; :87 `notify:web_push`; MODULES.md §5.2 `notify:in_app`.  
**Fix (revised by verify):** Append to P0.2 step 2: '`channels` ⊆ {`in_app`, `email`, `push`}, the §6 column names; `push` maps to `notify:web_push`. An unknown value makes the intent malformed (step 5, `asynq.SkipRetry`). In-app is written inline by dispatch, and no `notify:in_app` task exists. Update the MODULES.md §5.2 example list in the same PR.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 step 2 channels ⊆ {in_app,email,push}, push → notify:web_push, unknown = malformed (SkipRetry), no notify:in_app task; step 5 + AC. Code follow-up: resolveChannels ignores unknown values.

#### - [x] F067 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.4
*Category:* idempotency  
**Problem:** P0.4, the only P0 in-app producer, never sets `dedup_key`. A redelivered event therefore creates duplicate bell rows, although P0.2 cites `asset_id` as the example key. The spec also does not say whether the consumer dispatches inline or enqueues a task.  
**Evidence:** SPEC-04:129 intent `{user_id: owner_user_id, type:"media.asset_ready", title, data:{asset_id, kind, href}}` has no dedup_key; :83 '`dedup_key` is optional and event-derived (e.g. `notice_id`, `asset_id`)'; :86 ON CONFLICT applies only with a dedup_key.  
**Fix (revised by verify):** In P0.4 change the intent to `{user_id: owner_user_id, type:"media.asset_ready", title, data:{asset_id, kind, href}, dedup_key: asset_id}`, and add: 'Every event-driven consumer MUST set `dedup_key` from the event's natural id.' AC: 'Given `notify:on_asset_ready` delivered twice for one asset, then exactly one `notifications` row exists.'  
**Applied:** ✅ P0.4 intent dedup_key: asset_id + "every event-driven consumer MUST set dedup_key" + AC. Shipped OnAssetReady already does.

### SPEC-05-journal.md

#### - [x] F068 · 🟠 MAJOR · `SPEC-05-journal.md` · §6 Module-scope decision vs §5 P0.3
*Category:* internal-contradiction  
**Problem:** The locked module-scope rationale says the stream projection 'only consumes bus events'. P0.3 and SPEC-06 P0.1(a) project journal rows inside the entry's own transaction, with no bus, so a later split is not mechanical.  
**Evidence:** SPEC-05:206-208 'splitting later is a mechanical move because the projection already only consumes bus events.' vs SPEC-05:111, :113-118 'all inside the entry's own transaction'; SPEC-06:67-72 '(a) Journal rows — transactional, no bus.'  
**Fix (revised by verify):** Replace SPEC-05:206-208 with: 'if the stream ever grows its own roadmap, a split is NOT mechanical: system rows arrive via bus events, but journal rows are projected inside the entry transaction (P0.3, SPEC-06 P0.1(a)); a split must first replace that write with journal:entry_created/updated/deleted events plus an idempotent consumer, and accept eventual consistency on the composer's post-create refetch.'  
**Applied:** ✅ §6 Module-scope decision: a split is NOT mechanical (journal rows projected in the entry tx); needs entry events, idempotent consumer, eventual consistency.

#### - [c] F069 · 🟠 MAJOR · `SPEC-05-journal.md` · §5 P0.2 / §7 list endpoint
*Category:* missing-decision  
**Problem:** The list endpoint has no page size, response envelope, `next_cursor` or cursor encoding, and no Problem type for a malformed cursor, which the code emits anyway. The 'scroll back months' story has no bound.  
**Evidence:** SPEC-05:215 '| GET | `/api/v1/journal/entries?cursor=` | ... ordered `occurred_at DESC, id DESC` |'; :96-97 AC '500 entries ... paging by cursor'; :55-56 story. Compare SPEC-06:136-140 (default 30, max 50). journal/handler.go:230 emits `journal/invalid-cursor`, which is not in §7 (:224-227). Worklog F064 fixed this only in SPEC-06.  
**Fix (revised by verify):** §7 row: 'GET /api/v1/journal/entries?cursor=&limit= — limit default 50; a value <= 0 or > 100 falls back to 50; response {items, next_cursor}, where next_cursor is an opaque base64url keyset of (occurred_at, id), omitted on the last page; malformed cursor → 400 journal/invalid-cursor.' Add journal/invalid-cursor to the §7 Problem types. AC: 'Given 500 entries and limit=50, then exactly 10 pages, the last without next_cursor.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §7 list: cursor/limit (default 50, ≤0 or >100 → 50), {items,next_cursor}, opaque base64url (occurred_at,id), omitted on last page, 400 journal/invalid-cursor; P0.2 AC 500 entries → 10 pages. Code follow-up: journal/invalid-cursor missing from problems.ts.

#### - [x] F070 · 🟠 MAJOR · `SPEC-05-journal.md` · §5 P0.2 / §7 PATCH
*Category:* ambiguity  
**Problem:** PATCH semantics for explicit null versus an absent field are undefined. With empty or whitespace mood rejected, there is no way to clear a mood. Null occurred_at and null body_md are undefined, and so is the mechanism that maintains `updated_at`.  
**Evidence:** SPEC-05:217 'any subset of create fields'; :77-80 an empty mood → 422; :99-100 AC 'updated_at changes' (no trigger or rule). SPEC-12:164 later had to decide mood handling.  
**Fix (revised by verify):** SPEC-05 §7 PATCH notes: 'Absent field = unchanged; mood: null clears it (per SPEC-12); body_md: null or occurred_at: null → 422 (journal/invalid-body / journal/invalid-occurred-at); the UPDATE sets updated_at = now().' Add journal/invalid-occurred-at to the §7 Problem types if adopted.  
**Applied:** ✅ §7 PATCH: absent = unchanged, mood:null clears, present fields validated as on create, updated_at = now(); P0.2 AC. Follows HEAD: body_md:null / occurred_at:null treated as absent (COALESCE), not 422.

#### - [x] F071 · 🟠 MAJOR · `SPEC-05-journal.md` · §5 P0.2 mood validation vs §6 CHECK
*Category:* internal-contradiction / data-model  
**Problem:** Mood is validated after trimming and 'matches the §6 CHECK', but the CHECK counts the untrimmed value, and the spec never says the trimmed value is stored. Padded input passes the service and then fails the CHECK with a 500 at COMMIT. A whitespace-only mood also satisfies the CHECK.  
**Evidence:** SPEC-05:77-80 '1–80 chars after trimming ... matching the §6 CHECK — otherwise it would surface as a 500 at the DB' vs :191 `CHECK (mood IS NULL OR char_length(mood) BETWEEN 1 AND 80)`. ADR-07:143: under the tenant transaction a constraint violation surfaces as a 500 at COMMIT.  
**Fix (revised by verify):** P0.2: 'The service validates and stores `strings.TrimSpace(mood)`. The trimmed value must be 1–80 code points, otherwise 422 `journal/invalid-mood`. A whitespace-only mood trims to empty and is 422.' Keep the §6 CHECK as written, since it is satisfied by any stored trimmed value. Optional hardening, if wanted, goes in the next free journal migration and not in §6's original DDL: `UPDATE journal_entries SET mood = NULLIF(btrim(mood, E' \t\r\n'), '') WHERE mood IS NOT NULL;` then `ALTER TABLE journal_entries ADD CONSTRAINT journal_entries_mood_trimmed_chk CHECK (mood IS NULL OR mood !~ '^\s|\s$')`. AC: 'mood " vui " is stored as "vui"; 80 non-space code points padded with spaces → 201 (not 500); 81 non-space code points → 422 `journal/invalid-mood`.'  
**Applied:** ✅ P0.2: service stores TrimSpace(mood), 1–80 after trim, whitespace-only 422; mood ACs. Matches normalizeMood on HEAD.

#### - [x] F072 · 🟠 MAJOR · `SPEC-05-journal.md` · §5 P0.2 ACs / §7 DELETE
*Category:* ac-quality  
**Problem:** Write isolation has no AC: PATCH and DELETE of another user's entry are untested. The DELETE note '204; idempotent 404' is ambiguous about what a repeat delete returns.  
**Evidence:** SPEC-05:94-95 'when user A lists or fetches ... direct fetch is 404' (no PATCH/DELETE); :218 '204; idempotent 404'.  
**Fix (revised by verify):** §7 DELETE: '204 on delete; 404 journal/entry-not-found when the id is unknown, already deleted, or foreign (a repeat DELETE is 404).' Extend the AC at :94-95: 'when user A lists, fetches, PATCHes or DELETEs one of B's entries, then list omits it and fetch/PATCH/DELETE return 404 journal/entry-not-found; B's row is unchanged.'  
**Applied:** ✅ §7 DELETE: 204, or 404 journal/entry-not-found for unknown/deleted/foreign (repeat is 404); P0.2 isolation AC covers list/fetch/PATCH/DELETE.

#### - [c] F073 · 🟠 MAJOR · `SPEC-05-journal.md` · §5 P0.4 date control; P2 on-this-day
*Category:* ambiguity  
**Problem:** The composer's 'date control' does not say whether it picks a date or a date-time, what time a date-only pick gets, or which timezone applies. The 'last night' backdating story needs a time, and the on-this-day feature matches in the user's timezone.  
**Evidence:** SPEC-05:134-136 'optional date control (occurred_at, defaults to now)'; :52-54 'backdate an entry to last night'; :179-181 on-this-day; SPEC-06:209-212 'in the user's timezone (D-17)'.  
**Fix:** 'An optional date-and-time control (`occurred_at`, defaults to now). The picker works in the browser's local TZ; the client sends RFC 3339 with an offset, and the server stores the timestamptz unchanged. A date picked without a time defaults to 12:00 local.' AC: 'Given occurred_at = yesterday 21:00 local, it is stored at that instant and listed between its neighbours.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.4 date-and-time control, RFC 3339 with offset, date-only → 12:00 in the user's timezone (D-17); "yesterday 21:00" AC. Code follow-up: home Composer has no occurred_at control.

### SPEC-06-life-stream-home.md

#### - [c] F074 · 🟠 MAJOR · `SPEC-06-life-stream-home.md` · P0.1 occurred_at rule; §6
*Category:* event-ordering / data-model  
**Problem:** Stream occurred_at is taken from the bank payload's occurred_at, which is a DATE, while `stream_items.occurred_at` is timestamptz. The conversion is unspecified, so the implicit cast uses the session TZ (UTC, 07:00 ICT). Bank items then sort wrongly against timed journal entries and across day boundaries.  
**Evidence:** SPEC-06:98-100 '`occurred_at` = the payload's own timestamp field where one exists (`occurred_at` on bank rows)'; :250 `occurred_at timestamptz NOT NULL`. SPEC-03:98 '`occurred_at` **date**'; SPEC-03:432 `occurred_at date NOT NULL`.  
**Fix (revised by verify):** Replace SPEC-06:98-100 with: '`occurred_at`: for date-only payloads (bank `occurred_at` is a `date`, SPEC-03 §6), convert explicitly in the owner's TZ (D-17, looked up via accountapi). Use the ingest instant if the date equals the owner's local today at ingest, else 12:00 local on that date. Never rely on the implicit date→timestamptz cast (session TZ is UTC). On `bank:transaction_updated`, keep the stored `occurred_at` when the payload date is unchanged (`DO UPDATE SET occurred_at = CASE WHEN (stream_items.occurred_at AT TIME ZONE <tz>)::date = EXCLUDED_date THEN stream_items.occurred_at ELSE EXCLUDED.occurred_at END`), so metadata edits never move the item. Re-derive it only when the date itself changed. Other events: the payload timestamp if present, else ingest time.' Add ACs: 'Given a transaction dated today, created at 21:00 local, then it sorts above a journal entry written at 08:00 local that day'; 'Given that transaction later re-categorized, then its stream position does not change'; 'Given a transaction dated 2026-07-01 for a UTC+7 owner, then its item falls on 2026-07-01 local, not 2026-06-30.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.1 per-event occurred_at list; bank dates converted in the owner's TZ (APP_TIMEZONE v1; ingest instant if today, else 12:00 local), update keeps occurred_at unless the date changed; 3 ACs; TC-STREAM-017. Code follow-up: bankRef parses as UTC midnight; upsert overwrites occurred_at.

#### - [c] F075 · 🟠 MAJOR · `SPEC-06-life-stream-home.md` · P0.2 / P1.5 / §7
*Category:* api-contract  
**Problem:** `GET /stream` has no response envelope: no `items`, no `next_cursor`, and no per-item `id`/`source_module` in the compact shape, even though items are discriminated by source_module. The §7 row drops `limit`, an invalid limit has no Problem type, and `/stream/memories` has no shape.  
**Evidence:** SPEC-06:136-140 '`GET /api/v1/stream?cursor=&limit=` ... defaults to 30, hard max 50 (clamped above)' vs §7:263 '`/api/v1/stream?cursor=`'; :141-144 compact `{event_type, title, href, occurred_at}`; :266 only `stream/invalid-cursor`; :209-212 memories 'grouped by years-ago' with no shape.  
**Fix (revised by verify):** P0.2, append: 'Response `200 {items: StreamItem[], next_cursor: string|null}` (null on the last page). `StreamItem = {id (stream_items.id), source_module, event_type, ref_id, occurred_at} & ({kind:"journal", entry: JournalEntry} | {kind:"system", title, href: string|null, data: object})`. `JournalEntry` is SPEC-05\'s OpenAPI schema reused by `$ref`, including `asset_ids` and thumbs, never a parallel shape. `data` = the render-selected payload fields.' §7 row: 'GET | `/api/v1/stream?cursor=&limit=` | `stream:read:own` | merged timeline; `limit` default 30, max 50 (values above are clamped); non-integer or < 1 → 400 `journal/invalid-limit`; returns `{items, next_cursor}`'. P1.5: '`GET /stream/memories` → `200 {groups: [{years_ago: int, entries: JournalEntry[]}]}`, groups ordered years_ago ASC; an empty `groups` means none.' §7 Problem types: '`journal/invalid-cursor`, `journal/invalid-limit`', each registered as an i18n key (README DoD).  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 envelope {items, next_cursor?} + flat StreamItem (follows HEAD, not the nested proposal); lenient limit (default 30, clamp 50); §7 &limit=; P1.5 {groups:[{years_ago, entries}]}; TC-STREAM-032/036. Code follow-up: limit > 50 resets to 30; /stream/memories unshipped.

### SPEC-07-continue-rail.md

#### - [c] F076 · 🟠 MAJOR · `SPEC-07-continue-rail.md` · §5 P0.2 GET progress / P0.4
*Category:* ambiguity  
**Problem:** The spec does not say what `GET /assets/{id}/progress` returns when no row exists, which is every first open. A 404 would be indistinguishable from 'asset gone'.  
**Evidence:** SPEC-07:81-84 'returns `{position_ms, progress_pct, completed_at, updated_at}` for the caller's own row, 404ing under the identical conditions as the PUT'; :152-154 the player fetches it before initialising. The PUT's 404 conditions (:74-79) do not cover 'owned, no row'.  
**Fix (revised by verify):** P0.2 GET paragraph, append: 'When the asset passes every check but the caller has no row (first open), return 200 `{position_ms: 0, progress_pct: 0 (null when duration_ms IS NULL or ≤ 0), completed_at: null, updated_at: null}`. 404 is reserved for asset-level failures (see the status table), so the player never has to treat a 404 as "start from 0".' §7 GET row: '...; 200 with position_ms 0 / updated_at null when no row exists'. OpenAPI (same PR): in `PlaybackProgress`, `required: [position_ms]`, `updated_at` nullable: true, `progress_pct` nullable: true. Add AC to P0.2: 'Given a playable owned video never played, when GET progress is called, then 200 with position_ms 0 and updated_at null, and playback starts at 0.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 GET: first open with no row → 200 {position_ms 0, progress_pct 0/null, completed_at null, updated_at null}; 404 kept for asset-level failures; AC; §7 GET row; TC-CONT-030. Code follow-up: HEAD returns 404 on no row; openapi PlaybackProgress must relax required/nullable.

#### - [c] F077 · 🟠 MAJOR · `SPEC-07-continue-rail.md` · §5 P0.2 / §7 Problem types
*Category:* problem-type closure  
**Problem:** `media/asset-not-playable` is declared but never raised, because P0.2 sends non-video assets to 404. Videos in `uploading`, `processing` or `failed` status (no HLS output to resume) are unspecified. The 'first spec to ship defines it' hedge is stale: SPEC-01 already declares asset-not-found.  
**Evidence:** SPEC-07:74-79 '404s assets that are unknown, non-video, `deleting` ... or not owned'; :84; :211 'Problem types: `media/asset-not-found` (shared with SPEC-01 — first spec to ship defines it), `media/asset-not-playable`.' SPEC-01:413 declares asset-not-found and asset-not-ready. media/handler.go:23 defines asset-not-playable.  
**Fix (revised by verify):** Replace the 404 clause in P0.2 with a table that applies to both PUT and GET. Unknown id, malformed id, `deleting`, or owned by another user (no role or permission bypass) → 404 `media/asset-not-found`. Owned, not a video → 404 `media/asset-not-playable`, keeping the shipped status so existence is not leaked beyond ownership. Owned video in `uploading`/`processing`/`failed` → 409 `media/asset-not-ready`. Add a note: 'The current handler's ErrForbidden→403 path and GetProgress's deleting→not-playable mapping must be changed to match this table.' §7 → 'Problem types: `media/asset-not-found`, `media/asset-not-ready` (both declared in SPEC-01 §7), `media/asset-not-playable` (new here; register its i18n key per the Errors DoD).' ACs: 'Given an owned image asset, then PUT/GET progress → 404 `media/asset-not-playable`'; 'Given an owned video still `processing`, then PUT/GET → 409 `media/asset-not-ready` and no row'; 'Given another user's asset, then 404 `media/asset-not-found` (never 403).'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 status table for PUT/GET (not-found / not-playable 404 / not-ready 409) + 3 ACs; §7 Problem types rewritten; TC-CONT-021/024. Code follow-up: ErrForbidden→403, deleting→not-playable, no 409 status check, media/asset-not-playable missing from problems.ts.

#### - [c] F078 · 🟠 MAJOR · `SPEC-07-continue-rail.md` · §5 P0.3 title / P1.5 payload / header
*Category:* contradiction / repo-reality  
**Problem:** There are two different title fallbacks. The `/continue` item derives a basename from `source_key`, which is always `uploads/{id}/original{ext}`, so every item would be 'original.mp4'. The P1.5 event uses `original_filename`, which can itself be NULL and only exists after SPEC-01's migration. The header does not list SPEC-01 even though P0.3 says 'see the header'.  
**Evidence:** SPEC-07:125-127 'falls back to a filename derived from `assets.source_key` ... (a soft dependency — see the header)'; :167-169 'falls back to `original_filename` per SPEC-01 P1.2'; header :4 does not mention SPEC-01. media/service.go:41 `uploads/%s/original%s`. SPEC-01:371-372 adds title/original_filename; :271 notes original_filename is NULL for older assets.  
**Fix:** Define once and use in P0.3 and P1.5: 'Display title = COALESCE(assets.title, assets.original_filename, 'Untitled video'); never derive a title from source_key.' Header → 'Depends on: nothing hard; soft on SPEC-01 P0 (`assets.title`/`original_filename`, `deleting` status, `poster` variant); the comic leg plugs in with SPEC-02 `comic_reading_progress`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — Header Depends-on (soft on SPEC-01 P0 fields); P0.1 one display-title rule (title → original_filename → 'Untitled video', never source_key), cited by P0.3/P1.5; AC; TC-CONT-046. Code follow-up: GetContinueItems still falls back to a.source_key.

#### - [c] F079 · 🟠 MAJOR · `SPEC-07-continue-rail.md` · §1, §5 P0.3 response shape, §7
*Category:* api-contract / repo-reality  
**Problem:** The module-agnostic item's fields are undefined: the `module` values, what `ref_id` means, the type and rounding of `progress_pct`, where `poster_url` comes from and whether it can be null, and whether `title` can be null. The spec also claims D-20's shape while silently diverging from it, and it ignores the existing `ContinuingItem` schema in openapi.yaml, which uses different names, units and enum.  
**Evidence:** SPEC-07:120 `[{module, ref_id, title, poster_url, progress_pct, href, updated_at}]`; :15-16 '`D-20` already decided the aggregator shape'; :111 'Per D-20'; :137-138 AC 'contract test on the item schema'. feature-inventory D-20 (~1221-1236) `ContinuingItem{Kind, ID, Title, Position, Duration, Thumbnail, UpdatedAt}`; openapi.yaml:700-725 `ContinuingItem` with `module enum [movie,music,story,comic]`, `progress` 0..1, `resourceUrl`/`thumbnailUrl`, `updatedAt`. SPEC-01:179-180 'poster absent' is possible.  
**Fix (revised by verify):** Add a field table under P0.3 that matches `shared/openapi.yaml` `components.schemas.ContinueItem`. `module` string ∈ {media, movie, music, story, comic}: 'media' today, and the comic leg uses 'comic'. `ref_id` uuid: the asset id for media, the comic id for comic. `title` string, non-null, per the single fallback chain (finding 2). `poster_url` string|null: `/api/v1/assets/{ref_id}/variants/poster` only when a poster variant exists, else null; the comic leg uses its cover `thumb`. `progress_pct` integer 0–100, derived per P0.1's ratio rule. `href` relative app path (`/library/media/{ref_id}`). `updated_at` RFC 3339. The response body is `{items: [...]}`. State: 'The shared Go type moves to a platform package (e.g. `platform/continueitem.Item`), and every `<module>api.Continue` returns it, so comicapi does not import mediaapi.' §1/P0.3: 'D-20 decided the fan-out; the item schema is revised here (position/duration → progress_pct; the exact seek comes from GET progress). Record an update note under D-20 in feature-inventory.' Do not reference an openapi `ContinuingItem`.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §1 + P0.3: {items:[...]} with a field table matching openapi ContinueItem; shared Go type in a platform package. Code follow-up: HEAD type is mediaapi.ContinueItem and poster_url is never null.

### SPEC-08-people-registry.md

#### - [c] F080 · 🟠 MAJOR · `SPEC-08-people-registry.md` · P0.4 emit rule
*Category:* ambiguity  
**Problem:** If the first scan to see an occurrence has days_until = 0 (person created on the birthday, birthday edited to today, or scanner down days −3..−1), both the 3-day and the day-of thresholds fire in the same scan. The owner gets two birthday items for one day.  
**Evidence:** SPEC-08:141-144 'for each threshold `T`, emit when `0 ≤ days_until ≤ T` and no notice row exists'. At 0, both T=3 and T=0 match. AC :170-171 'exactly one 3-day event fires; day-of fires once' does not cover this case; :172-175 covers only a scanner down on day −3.  
**Fix (revised by verify):** Replace the SPEC-08:141-143 condition with: 'for each threshold `T`, emit when `lower(T) ≤ days_until ≤ T` and no notice row exists for `(person, occurrence_year, T)`, where lower(3) = 1 and lower(0) = 0. A 3-day notice is never emitted on the day itself; the day-of notice supersedes it.' Add AC: 'Given a person created (or a birthday edited) on the birthday itself, or the scanner down days −3..−1, then only the day-of event fires.' No new column is needed.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.4 emit rule lower(T) ≤ days_until ≤ T (day-of replaces 3-day on the day); lunar-skip sentence; AC. Code follow-up: ScanBirthdays fires both thresholds at days_until=0.

#### - [c] F081 · 🟠 MAJOR · `SPEC-08-people-registry.md` · P0.2 birthday wire shape / notice reset
*Category:* correctness  
**Problem:** Any PATCH that carries a birthday deletes the notice rows, even when the date is unchanged. PATCH must resend the whole birthday object, so an ordinary save (rename, or correcting only birth_year) resets the notices, and the next scan re-emits under a new notice_id. Consumers dedupe by notice_id, so the stream and bell get duplicates.  
**Evidence:** SPEC-08:84-85 'send the whole object'; :86-88 '**Editing or clearing a birthday also deletes the person's `people_birthday_notices` rows for the current and future occurrence years**'; :92; :155-156 'consumers idempotent by `notice_id`'.  
**Fix (revised by verify):** Replace SPEC-08:86-88 with: 'When a PATCH changes the effective occurrence (birth_month or birth_day differ from the stored values, birth_calendar changes, or the birthday is cleared), the same transaction deletes the person's `people_birthday_notices` rows with `year >= the current year in the owner's TZ`. Resending an identical birthday object, or changing only `birth_year`, leaves the notice rows untouched.' Add AC: 'Given a PATCH that changes only display_name and resends the unchanged birthday after the 3-day notice fired, then no second 3-day event is emitted.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 notices reset only when the occurrence changes, in the same tx; §7 PATCH row; AC. Code follow-up: UpdatePerson resets on every PATCH carrying a birthday, outside the tx.

#### - [c] F082 · 🟠 MAJOR · `SPEC-08-people-registry.md` · P0.4 outbox re-publish; §6 people_birthday_notices
*Category:* ambiguity / data-model  
**Problem:** A retry of an `emitted_at NULL` row has no defined payload. The notices table stores neither the occurrence date nor days_until, so the retry recomputes a stale value. If the next scan runs after the date, the retry publishes a late 'today' or a negative days_until, which breaks the 'never emitted late' rule and the 'self-sufficient payload' promise.  
**Evidence:** SPEC-08:153-156 'Rows left with `emitted_at NULL` ... are re-published by the next scan'; :145-147 'a missed day-of notice is never emitted after the day passes'; :92-93 'payload is self-sufficient'; §6 :251-258 columns `id, person_id, year, threshold, emitted_at`; payload :137-138 includes `days_until`.  
**Fix (revised by verify):** §6 people_birthday_notices: add `occurrence date NOT NULL -- celebrated date (Feb-28-adjusted) in the owner's TZ at insert time`. P0.4, after the outbox paragraph: 'A re-publish recomputes `days_until = occurrence − today(owner TZ)` and rebuilds the payload from the current person row. If days_until < 0, do not publish: set `emitted_at = now()` and log it as expired. If a smaller threshold also emits for the same (person, year) in this scan, mark the pending larger-threshold row as suppressed (`emitted_at = now()`, `suppressed = true`, no publish), the same rule as the day-0 collapse.' Extend AC :176-178: '...and if the next scan runs after the occurrence date, the row expires unpublished; if it runs on the day itself, only the day-of event is published and the pending 3-day row is suppressed.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §6 notices gain occurrence date NOT NULL + suppressed; P0.4 "Re-publish payload" (days_until from occurrence, past rows expire, larger threshold suppressed); crash-retry AC. Code follow-up: retry publishes days_until = threshold; columns absent.

#### - [c] F083 · 🟠 MAJOR · `SPEC-08-people-registry.md` · P0.2 birthday validation; §6 people_persons; P0.4
*Category:* data-model  
**Problem:** Gregorian validity checks apply to every birthday, including lunar ones. Legal lunar dates such as 30 tháng 2 are rejected, the Feb-29 leap-year rule is meaningless for lunar, and there is no leap-month flag. Lunar data that ships 'now for later math' is therefore rejected or stored lossily, and the P0.4 scan never says to skip lunar rows.  
**Evidence:** SPEC-08:95-99 'the (month, day) pair must be a real calendar date — Feb-29 is allowed with no year or a leap year'; :106; :37-40 'the `birth_calendar` column ships now ... the recurrence math ships when ... requested'; :217-218; §6 :231-234 has no leap-month column; :312 '(resolved) column now, math later'. P0.3 omits lunar (:131), but P0.4 (:135-139) does not.  
**Fix (revised by verify):** P0.2 validation: 'The real-calendar-date and Feb-29/leap-year checks apply only when `calendar='solar'`. For `calendar='lunar'`: month 1–12, day 1–30, optional `leap_month` (default false). `birth_year` plausibility (1900..current year) applies to both.' Wire shape: `birthday: { month, day, year?, calendar?: 'solar'|'lunar', leap_month?: boolean /* lunar only */ } | null`. :82-84: '`PATCH { birthday: null }` NULLs birth_month/day/year, resets `birth_calendar` to 'solar' and `birth_leap_month` to false.' §6: `birth_leap_month boolean NOT NULL DEFAULT false`, `CHECK (NOT birth_leap_month OR birth_calendar = 'lunar')`, `CHECK (birth_calendar = 'solar' OR birth_day IS NULL OR birth_day <= 30)`. P0.4 first paragraph: 'Rows with `birth_calendar='lunar'` are skipped at v1 (§3).' ACs: P0.2 'Given lunar 30/2, then 201 (accepted)'; P0.4 'Given a lunar row, the scan inserts no notice and emits nothing.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 wire shape leap_month?; Gregorian checks solar-only, lunar month 1–12 day 1–30; §6 birth_leap_month + CHECKs; P0.4 lunar rows skipped; ACs. Code follow-up: validateBirthday applies Gregorian checks to lunar; no leap_month.

### SPEC-09-platform-ops.md

#### - [c] F084 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · §4 primary story; P0.2 step 2; P0.3; P0.4
*Category:* unimplementable  
**Problem:** The restore drill never says which object store it reads from. The stack has one set of `S3_*` settings, which on dev points at the disposable dev MinIO, so the drill cannot reach last night's prod dump.  
**Evidence:** SPEC-09:50-51 'I run `make restore-drill` on a fresh dev stack and my prod ledger from last night is queryable'; :85-87 'bucket differs'; :116-117 'The dev MinIO bind-mount is explicitly disposable'; :124; :133 'The drill reads the manifest' (no source). .env.example:71-72 'The app reads S3_* for ALL object storage'; backup-restore.md:141 'auto-reads `S3_*` from `.env`'.  
**Fix (revised by verify):** P0.4 step 0: 'Source: the drill reads from an explicit backup source (`RESTORE_S3_ENDPOINT`, `RESTORE_S3_BUCKET`, `RESTORE_S3_ACCESS_KEY`, `RESTORE_S3_SECRET_KEY`), each falling back to the stack's `S3_*` only when unset. The prod drill uses a read-only R2 token scoped to `backups/pg/*`.' Add AC: 'Given `RESTORE_S3_*` pointing at prod R2, the drill restores last night's prod dump without changing the stack's `S3_*`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.4 step 0 RESTORE_S3_* backup source (falls back to S3_*, read-only token on backups/pg/*); AC; §4 story; TC-OPS-021a; runbook knob + prerequisites rewritten. Code follow-up: restore-drill.sh hard-codes http://minio:9000.

#### - [c] F085 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · §5 P0.2 step 5 retention + AC
*Category:* ambiguity  
**Problem:** Retention does not say whether its input is a key listing or the ok rows, what counts as 'Sunday' when Sunday's run failed, or which timezone defines days and weeks. The property-test AC cannot be written without these.  
**Evidence:** SPEC-09:94-95 'keep the last **7 daily** dumps and **4 weekly** (the most recent Sunday dump per week)'; :108-109 property test. P0.4 :133-134 already rejects key listing ('a failed partial upload could poison'). The key date (:85) has no TZ. Shipped backup.go:142 `ListSuccessfulRuns`.  
**Fix (revised by verify):** P0.2 step 5 → '**Retention** after a successful run. Input = `ops_backup_runs` rows with `status='ok'` and non-null `storage_key`, de-duplicated by key (never a storage listing). All dates are UTC: a dump's date is the `<yyyy-mm-dd>` in its key, which is `started_at` in UTC. Keep = the 7 most recent dump dates ∪, for each of the 4 most recent ISO weeks (Mon–Sun, UTC) that contain an ok dump, the latest ok dump in that week ∪ the current LATEST.json target. Delete every other key from storage. `Storage.Delete` of a missing key is a no-op, so rows are never modified and re-running prune is idempotent. `backups/pg/LATEST.json` is never a candidate. Objects with no ok row are left alone: after a disaster restore the ledger is older than the bucket, and sweeping would delete the newest dumps.' Extend the AC: '...property test covering weeks whose Sunday run failed, inputs spanning a year boundary, and duplicate same-day keys.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 step 5 retention (ok rows by key, UTC, 7 recent dates ∪ latest per ISO week ×4 ∪ LATEST, idempotent Delete); AC, runbook, TC-OPS-004. Code follow-up: retention.go anchors weeks on Sunday dumps only.

#### - [c] F086 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · §5 P0.2 task lifecycle; P0.5
*Category:* missing-decision / NFR  
**Problem:** The backup task has no retry, timeout, uniqueness or crash policy. Asynq defaults (25 retries, 30-min timeout) turn one outage into many rows and many `ops:backup_failed` events, can overlap dumps, can overwrite the same-day key, and kill a growing DB dump at 30 minutes. A worker crash leaves a row stuck in `running` forever with nothing to reap it.  
**Evidence:** SPEC-09:77-98 steps 1-6 set no Asynq options; :83 'Insert a `backup_runs` row (`running`)'; :96-97 emits failed; AC :106-107 'no wedged state'; :168 'A `running` run doesn't change state'. Key `<yyyy-mm-dd>.dump` (:85). §8 has no duration or size metric.  
**Fix (revised by verify):** Add to P0.2: 'Task options: `asynq.MaxRetry(0)` (a failed night is recorded once and retried by the next night), `asynq.Timeout(2h)`, `asynq.Unique(23h)`. At start, mark any `running` row older than the timeout as `failed` (`error='abandoned'`, `finished_at=now()`).' ACs: 'a `running` row older than 2 h becomes `failed` ("abandoned") on the next run'; 'a DB outage yields exactly one `ops:backup_failed`'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 task options MaxRetry(0), Timeout(2h), Unique(23h) + abandoned-row reap; 2 ACs; TC-OPS-003/003a. Code follow-up: scheduler sets only Queue("default"); no reaper.

#### - [c] F087 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · §2 Goal 4; P0.1 seeding; P1.6; §7
*Category:* authz / security  
**Problem:** The asynqmon console can retry, delete, archive and pause, yet it is gated only by the read code `queues:read` and is not set ReadOnly, which breaks the README read/write grammar. It also has no CSRF guard beyond SameSite, and SameSite does not separate the same-site sibling hosts (minio., mail., traefik.).  
**Evidence:** SPEC-09:33 'inspects/retries/purges Asynq jobs'; :187-193 'gated `RequireAuth` + `RequirePermission("queues:read")` ... retryable from the browser'; §7:265 '`queues:read` (admin)'. README.md:68 'action ∈ `read | write | delete`'. cmd/api/main.go:733 `asynqmon.New(asynqmon.Options{RootPath: "/admin/queues", ...})` with no ReadOnly, under `queues:read`. docker-compose traefik rules put siblings on the same site.  
**Fix (revised by verify):** P1.6: 'Two asynqmon handlers are built: `asynqmon.New(asynqmon.Options{RootPath: "/admin/queues", RedisConnOpt: ..., ReadOnly: true})` for callers holding only `queues:read`, and one with `ReadOnly: false` for `queues:write` holders. The mount picks the handler per request after RequireAuth. GET/HEAD under `/admin/queues` require `queues:read`. Every non-GET/HEAD request requires `queues:write`, a new admin-only code seeded (permissions + role_permissions → `admin`) by P1.6's own migration `000N_ops_queues_write`. 0012 is consumed and stays at four codes. Non-GET requests are rejected with 403 unless `Sec-Fetch-Site: same-origin` (or `Origin` equals the API origin), because SameSite=Lax does not separate same-site siblings such as `minio.`, `mail.` and `traefik.`.' P0.1 stays unchanged. Add a note: '`queues:write` is added by P1.6's migration.' §7 row: '`queues:read` (GET) / `queues:write` (mutations), admin'. ACs: 'An admin retries a dead-lettered task and it re-enters `pending`'; 'A caller with only `queues:read` POSTing a retry → 403'; 'A cross-origin POST with the admin cookie → 403'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P1.6 two asynqmon handlers (ReadOnly for queues:read, writable for new queues:write seeded by P1.6), Sec-Fetch-Site/Origin check on non-GET; §7 rows, ACs, TC-OPS-082/083/120. Code follow-up: single writable handler under queues:read, no CSRF check.

#### - [x] F088 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · §5 P1.7 download URL, creation limits; §7
*Category:* unimplementable / security  
**Problem:** The takeout download is left as an either/or, and its presign branch asks for a 'single-use' presigned R2 URL, which is impossible because SigV4 presigns are replayable until they expire. The proxy branch has no route. Export creation is unbounded, with no in-flight limit and no rule on whether the tar is streamed or staged on disk.  
**Evidence:** SPEC-09:202-205 'served through an authenticated owner-scoped proxy (matching SPEC-01 P0.5), or — if presigning R2 directly — a single-use, opaque key with an explicit TTL of ≤5 min'; §7 :263-264 has no download row; :206-207 the archive bundles EXIF/GPS originals; :194-201 POST has no concurrency or size limit.  
**Fix (revised by verify):** P1.7 → '**Download: proxy only (decided).** The archive carries EXIF/GPS originals, so the SPEC-01 P0.5 rule applies. S3/R2 presigns are replayable until expiry and cannot be single-use. `download_url` = `/api/v1/me/export/{id}/download`.' Add a §7 row: 'GET | `/api/v1/me/export/{id}/download` | `takeout:read:own` | P1.7; streams via `Storage.Get` with `Content-Disposition: attachment; filename="portal-export-<yyyy-mm>.tar.gz"`; 404 `ops/export-not-found` if absent or not the caller's; 409 `ops/export-not-ready` while pending/running; 410 `ops/export-expired`'. Add: 'At most one export per user in `pending|running`. Another POST → 409 `ops/export-in-progress`, whose body carries the existing `export_id`. The tar.gz is written to a worker temp file (the same seekable-body constraint as P0.2), then uploaded with `Storage.Put`, and the file is removed on exit. Worker scratch disk must hold one archive. Past ~5 GiB, a `Storage.PutMultipart` (feature/s3/manager.Uploader) is the P2 path. `ops:takeout` options: `asynq.Timeout(2h)`, `asynq.MaxRetry(3)`.' Problem types in §7: `ops/export-not-found`, `ops/export-not-ready`, `ops/export-expired`, `ops/export-in-progress`.  
**Applied:** ✅ P1.7 download proxy only (/api/v1/me/export/{id}/download), one in flight (409 ops/export-in-progress), temp-file spool, ops:takeout Timeout(2h)/MaxRetry(3); §7 row, four Problem types; TC-OPS-102/103/103a.

#### - [x] F089 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · §5 P1.6 / P1.7
*Category:* acceptance-criteria / contract  
**Problem:** P1.6 and P1.7 have no ACs at all. P1.7 also leaves the ExportProvider signature undefined (including how GB-scale originals are streamed), what happens when one provider fails, and the `ops:export_ready` payload.  
**Evidence:** SPEC-09:187-215 has no 'Acceptance criteria' block (every P0 has one: :103, :146, :176). :197-198 'each wired module's `api/` ExportProvider' gives no signature; :205 'emits `ops:export_ready`' gives no payload (events.md:37 `{export_id, user_id}`).  
**Fix (revised by verify):** P1.7: 'Contract: `type ExportProvider interface { Name() string; Export(ctx, userID uuid.UUID, w ExportWriter) error }` with `ExportWriter.Add(path string, size int64, r io.Reader) error` streaming one entry under `<Name()>/`. Any provider error → export `failed` with `error='<provider>: <msg>'` and the partial archive deleted.'  
**Applied:** ✅ P1.7 ExportProvider/ExportWriter contract, provider-failure rule, ops:export_ready {export_id, user_id}, AC block; P1.6 AC block; TC-OPS-103b.

#### - [x] F090 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · P0.2 step 2 (direct connection)
*Category:* repo-reality  
**Problem:** The dump must bypass PgBouncer, but the spec names no connection setting. The worker's only DSN points at PgBouncer in transaction mode, so reusing it causes exactly the failure the spec warns about.  
**Evidence:** SPEC-09:87-88 '**Dump connects directly to Postgres, not through PgBouncer**'. .env.example:17 `DATABASE_URL=...@pgbouncer:6432/...`; :20 `PGBOUNCER_POOL_MODE=transaction`; cmd/worker/main.go:42 uses cfg.DatabaseURL. Shipped backup.go:85-86 introduced `BACKUP_DATABASE_URL`.  
**Fix (revised by verify):** Append to P0.2 step 2: 'The DSN is `BACKUP_DATABASE_URL` (`platform/config` → `cfg.BackupDatabaseURL`; documented in `.env.example`). It connects directly to Postgres, never through a pooler, as the schema-owner role `portal`, which bypasses RLS so every tenant's rows are dumped. `DATABASE_URL` (`portal_app`, NOBYPASSRLS) must never be reused: pg_dump under RLS either errors or silently omits tenants. If the setting is unset, the run fails with `error="BACKUP_DATABASE_URL is not configured"`, so P0.5 reads `failed`.' Fix backup-restore.md:60-64 so the example host matches the deployment (`host.docker.internal:5432` under the host-PG topology) and drop the PgBouncer wording. List BACKUP_DATABASE_URL in §9 item 2.  
**Applied:** ✅ P0.2 step 2 BACKUP_DATABASE_URL (owner role, never DATABASE_URL/PgBouncer, unset → failed); §9; runbook example; TC-OPS-002. Shipped code matches.

#### - [c] F091 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · §2 Goal 3; P0.5; header
*Category:* completeness  
**Problem:** Goal 3 says failure or staleness is 'impossible to miss', but there is no push path. Notify is live, yet no spec owns a consumer for `ops:backup_failed`. Staleness from a dead scheduler emits no event at all, only a pull endpoint that nobody polls.  
**Evidence:** SPEC-09:32 'Backup failure or staleness is impossible to miss (event + queryable status).'; :4 'failure alerts get delivery when SPEC-04 lands'; events.md:35-36 '`ops:backup_failed` ... emitter only; no consumer yet' although notify is live (:52-55); story :56-58 depends on someone polling.  
**Fix (revised by verify):** Replace the header clause with: 'failure alerts: notify consumer `notify:on_backup_failed` (owned here, P0.6) now that SPEC-04 is live; staleness remains pull-only via P0.5 until a freshness-check task is added (P2).'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — Header: notify:on_backup_failed owned here as P0.6; staleness pull-only until P2; P0.6 section + AC; §7 tasks; TC-OPS-050. Code follow-up: no notify:on_backup_failed consumer/type on HEAD.

### SPEC-10-ledger-expansion.md

#### - [x] F092 · 🟠 MAJOR · `SPEC-10-ledger-expansion.md` · §6 API summary (phase 1)
*Category:* rbac-seeding  
**Problem:** SPEC-10 introduces `bank-debts:read/write:own`, but no migration seeds it and no role receives it. It covers only one of the four debt routes (DELETE, movements and accrue have no permission), and there is no delete code. Wired as written, every non-superadmin gets 403. The shipped code gates debts on the bank-transactions codes instead.  
**Evidence:** SPEC-10:173 '| GET/POST | /api/v1/bank/debts | bank-debts:read/write:own |'; :174-176 list the other routes without a permission. README.md:71-74 'ships the permissions + role_permissions seed rows in its own migration ... an unseeded code 403s everyone below superadmin'. 0043_bank_debts seeds nothing; bank/module.go:110-116 uses `bank-transactions:*:own`.  
**Fix (revised by verify):** Replace the §6 Notes column: GET list / GET {id} → `bank-transactions:read:own`; POST, PATCH {id}, POST {id}/movements, POST {id}/accrue → `bank-transactions:write:own`; DELETE {id} → `bank-transactions:delete:own` (refuses while the balance is non-zero). Add below the table: 'Debts reuse the SPEC-03 bank-transactions codes, because a debt is an account whose money moves by transfer. Migration 0043 therefore seeds no new permission. A later phase that adds a non-transaction surface seeds `bank-<noun>:read|write|delete:own` → `user` in its own migration, per README seeding rule.'  
**Applied:** ✅ Already resolved by cross-spec pass (SPEC-10 §6 per-method bank-transactions:* codes); rationale added (0043 seeds nothing; later phases seed own bank-<noun> codes).

#### - [x] F093 · 🟠 MAJOR · `SPEC-10-ledger-expansion.md` · §4 Phases 1, 4, 6, 7
*Category:* event-policy-gap  
**Problem:** SPEC-10 adds four new write paths to bank_transactions (debt movements and accruals, recurring drafts, splits, CSV/PDF import) and a due-reminder task, without saying which bank:transaction_* events they emit or naming the task. Taken literally, SPEC-03 P0.7 means one event per row, which floods the stream on import, projects unconfirmed drafts, and double-counts splits.  
**Evidence:** SPEC-10:115-117 'materialises into **draft** transactions'; :131-133 'parent plus N category legs'; :139-142 CSV/PDF import; :90-91 'Due reminders through `notify:dispatch`, driven by the existing Asynq scheduler' (no name). No 'event'/'emit'/'stream' in the file. SPEC-03:301-302 'exactly one matching event'; carve-out only for bulk reassign (:292-295).  
**Fix (revised by verify):** Add '§4a Event policy' to SPEC-10: '(1) Debt movements (borrow/lend/repay/collect) are ordinary transfer pairs and emit `bank:transaction_created|updated|deleted` per SPEC-03 P0.7 (`is_transfer=true`, shared `transfer_id`, so SPEC-06 collapses them into one item). An accrual is a normal categorised expense and emits per P0.7. (2) Recurring (phase 4): a draft emits nothing. Confirming a draft emits one `bank:transaction_created`. Discarding one emits nothing. (3) Splits (phase 6): only the parent emits. Creating, editing or deleting any leg emits one `bank:transaction_created|updated|deleted` for the parent. Legs never emit. (4) Import (phase 7): import-batch rows emit no per-row `bank:transaction_*` event (the same carve-out rationale as SPEC-03 P0.4 bulk reassign: one user action, not N mutations). The batch emits one `bank:import_completed {import_batch_id, user_id, account_id, row_count}` after commit. It has no stream consumer (the stream projects moments, events.md). The bell may consume it later. (5) Due reminders (phases 1 and 3): the daily periodic task `bank:scan_due_reminders` runs on the shared scheduler in `cmd/worker`, on the `default` queue, once per tenant via `forEachTenant`. For thresholds T ∈ {3, 0} it enqueues `notify:dispatch` when 0 ≤ days_until ≤ T, with `dedup_key = <debt_id|card_account_id>:<due_on>:<T>` (catch-up semantics as in SPEC-08 P0.4).' Register `bank:import_completed` (status planned, SPEC-10 phase 7, no consumer) and `bank:scan_due_reminders` (task, SPEC-10 phase 1) in docs/reference/events.md. Do NOT add a SPEC-06 stream row.  
**Applied:** ✅ SPEC-10 §4a Event policy (movements/accruals, recurring drafts, splits parent-only, import → bank:import_completed, due reminders = shipped bank:scan_debts_due daily 07:00 UTC, leads 7/1/0, dedup <debt_id>|<due_on>|<lead>); events.md rows added.

## 🟡 Minor

### MULTIPLE

#### - [x] F094 · 🟡 MINOR · `MULTIPLE` · SPEC-06 P1.6 vs SPEC-01 §6/P1.2
*Category:* incorrect cross-spec claim  
**Problem:** SPEC-06 P1.6 says SPEC-01 does not persist `origin` and restricts the backfill to instances without comic imports. SPEC-01 rev 3 persists `assets.origin` and exposes it on mediaapi listings specifically for this backfill. (This applies only if P1.6 survives the stream de-projection above.)  
**Evidence:** SPEC-06:222-225 'Since SPEC-01 does not persist `origin` on assets ... until it does, the backfill is restricted to instances with no comic imports.' vs SPEC-01:315-320 'origin is read from `assets.origin` (§6) ... mediaapi asset listings also expose `origin` (SPEC-06 P1.6 backfill needs it)'; SPEC-01:373-374, :384-388.  
**Fix:** If P1.6 is kept, replace :222-225 with 'Skip `origin='import'` assets (SPEC-01 §6 persists `assets.origin` and mediaapi listings expose it); no instance restriction', and name the trigger ('enqueued by `portal-cli stream backfill`, admin only'). If P1.6 is retired, delete the paragraph.  
**Applied:** ✅ SPEC-06 P1.6 retired (per F010) → paragraph deleted; SPEC-01 P1.2/§6 no backfill-origin claim; events.md journal:backfill_stream retired.

#### - [c] F095 · 🟡 MINOR · `MULTIPLE` · SPEC-04 P0.2 step 4; SPEC-09 P1.7; SPEC-02 P1.7
*Category:* cross-spec consistency  
**Problem:** SPEC-04 cites SPEC-01 for a 'default queue (weight 1)' assignment that SPEC-01 never makes, and requires every default-queue handler to be light. SPEC-09 puts the multi-GB takeout on `default`, and the comic zip import runs there with a 12 h timeout.  
**Evidence:** SPEC-04:87 'on the **`default` queue** (weight 1 — SPEC-01 P0.1's resource guardrails already assign light notify/janitor work there) ... Handlers must stay lightweight' vs SPEC-01:121-125 (no weight, no queue named). SPEC-09:196-197 'enqueues `ops:takeout` (default queue)'. comic/import.go:141 `asynq.Queue("default"), asynq.Timeout(12*time.Hour)`; cmd/worker/main.go:395-399 (default is served by the Concurrency-4 light server).  
**Fix:** SPEC-04 step 4 → 'on the `default` queue of the light weighted server (SPEC-01 P0.1 keeps notify/janitor off `heavy`). Long-running jobs (takeout, zip import) must use their own queue and server so they cannot starve notify.' SPEC-09 P1.7 → '(a dedicated `bulk` queue served by its own `asynq.Server`, Concurrency 1)'. SPEC-02 P1.7 likewise names the `bulk` queue for `comic:import_zip`.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — SPEC-04 P0.2 step 4 light default queue (wrong SPEC-01 cite dropped), long jobs own queue; SPEC-09 P1.7 ops:takeout on dedicated bulk queue (Concurrency 1); SPEC-02 P1.7 + events.md comic:import_zip → bulk. Code follow-up: comic/import.go enqueues on default with 12 h timeout.

#### - [x] F096 · 🟡 MINOR · `MULTIPLE` · SPEC-04 header/§1/§3; SPEC-05 header; SPEC-06 §3; SPEC-08 header; SPEC-09 header
*Category:* stale reference  
**Problem:** Several specs cite `backlog.md §N` (§1, §3, §5, §7). The numbered backlog was replaced on 2026-09-11 with an unnumbered one, so every such citation resolves to nothing, including SPEC-04's upstream link. SPEC-08's 'Events/birthdays' item exists only in facebook-comparison.md.  
**Evidence:** SPEC-04:5 'folded into backlog.md §5', :14, :16, :25, :32; SPEC-05:5 'backlog §3 P1'; SPEC-06:45; SPEC-08:5 'backlog §3 P2 ("Events/birthdays")'; SPEC-09:5 'backlog §7'. backlog.md:17-19 'replaced wholesale on 2026-09-11'; current headings are 'P0 — ...', 'P1 — ...' etc. The numbered version is at `8d382d2^:docs/product/backlog.md`. gap-audit-2026-07.md was never committed.  
**Fix:** Rewrite each `backlog §N` as '2026-07 backlog §N (archived; `git show 8d382d2^:docs/product/backlog.md`)', citing the live backlog line where the item is still open. SPEC-08:5 → '[analysis/facebook-comparison.md] row "Events / birthdays" (the former backlog §3 P2 item)'.  
**Applied:** ✅ Backlog §N citations → "2026-07 backlog §N (archived; git show 8d382d2^:docs/product/backlog.md)" in SPEC-01/04/05/06/09; SPEC-08 header cites facebook-comparison.md Events/birthdays row.

#### - [x] F097 · 🟡 MINOR · `MULTIPLE` · specs README intro; briefs README build order
*Category:* brief↔spec mapping  
**Problem:** The specs README says every spec is a promoted brief and that the briefs README tracks the mapping. SPEC-10, 11 and 12 have no brief and no row there, not even a 'no brief' row like SPEC-04's.  
**Evidence:** specs/README.md:11-12; briefs/README.md:20-32 has no SPEC-10/11/12 rows (compare :24 SPEC-04 'no brief'); SPEC-10:4, SPEC-11:5, SPEC-12:5 name their upstream sessions.  
**Fix:** specs/README:11-12 → 'Most specs are promoted from a brief (briefs README tracks the mapping); SPEC-04, 10, 11 and 12 have none, and each header names its upstream.' Add briefs README rows: '| — | Ledger expansion | `bank` | SPEC-10 | Phase 1 building (no brief — design session 2026-09-11) |', '| — | Docs canonicalisation | — | SPEC-11 | Executed 2026-09-11 (no brief) |', '| — | Journal attachments | `journal` | SPEC-12 | Executed 2026-09-19 (no brief — issue #8) |'.  
**Applied:** ✅ README intro: most specs come from a brief, SPEC-04/10/11/12 have none; briefs README rows for SPEC-10/11/12.

#### - [x] F098 · 🟡 MINOR · `MULTIPLE` · §7 permission columns of SPEC-03, SPEC-04, SPEC-10; README AuthZ
*Category:* rbac-naming  
**Problem:** Several §7 tables write permissions as slash or pipe shorthand, which is not a parseable code. Copying one into `RequirePermission` or `x-required-permission` panics at startup.  
**Evidence:** SPEC-03:467-475 `bank-accounts:read/write:own`, :319-320 `bank-accounts:read|write|delete:own`; SPEC-04:219-220 `notification-prefs:read/write:own`, `push-subscriptions:write/delete:own`; SPEC-10:173. rbac.Parse rejects '/' and '|' (permission.go:88-101).  
**Fix:** Add to the README AuthZ bullet: '§7 tables give exactly one literal, parseable code per method (split `GET/POST` rows); shorthand like `read/write` is not a code.' Then split the SPEC-03, SPEC-04 and SPEC-10 rows accordingly.  
**Applied:** ✅ README AuthZ: one literal code per method; SPEC-03 §7 + P0.8 split, SPEC-04 §7 prefs/push split, SPEC-10 §6 split with Permission column using shipped bank-transactions:* codes.

#### - [x] F099 · 🟡 MINOR · `MULTIPLE` · events.md Tasks table + Delivery mechanics; SPEC-04 §7
*Category:* registry-closure  
**Problem:** The events.md Events table names six notify consumer tasks (on_comic_published, on_movie/track/story_published, on_connection_requested/accepted) that are absent from the Tasks table and from SPEC-04's owned-task list. The Delivery-mechanics example cites a nonexistent `journal:stream_ingest` for an event the stream no longer consumes.  
**Evidence:** events.md:23, :28-31 name the tasks; Tasks table :39-62 lists only dispatch/email/web_push/on_asset_ready/purge_old; SPEC-04:231. events.md:86-87 '`media:asset_ready → notify:on_asset_ready` + `journal:stream_ingest`' vs :20, :58.  
**Fix (revised by verify):** Add Tasks-table rows: `notify:on_comic_published` (subscribes to `comic:published {comic_id, owner_user_id, title, chapter_count}`; dedup comic_id:chapter_count), `notify:on_movie_published` (movie:published), `notify:on_track_published` (music:track_published), `notify:on_story_published` (story:published), `notify:on_connection_requested` / `notify:on_connection_accepted` (social:connection_*; dedup connection_id:phase). Append them to SPEC-04 §7's owned tasks. Replace the Events-table `comic:chapter_published` row with `comic:published`. Change events.md:86-87 to '(e.g. `media:asset_deleted → comic:on_asset_deleted` + `journal:stream_asset_deleted`)'.  
**Applied:** ✅ events.md Tasks table: notify:on_comic/movie/track/story_published, on_connection_requested/accepted (dedup keys); SPEC-04 §7 owned/subscribes lists; Delivery-mechanics example → media:asset_deleted fan-out.

#### - [x] F100 · 🟡 MINOR · `MULTIPLE` · events.md:20/:118; SPEC-01 P1.2; SPEC-04 P0.4
*Category:* stale-number  
**Problem:** The flood-guard rationale cites '≤300 assets' per zip import, but rev 10 raised the limit to 20,000 entries (a 9,129-image import was verified).  
**Evidence:** events.md:20, :118; SPEC-01:318; SPEC-04:129 'up to 300 assets'; SPEC-02 rev 10 '20000 entries', rev 12 '9129-image archive'.  
**Fix (revised by verify):** Replace each '≤300 assets' / 'up to 300 assets' with 'thousands of assets (the whole-comic zip import cap, `importMaxEntries` in comic/import.go, 100,000 as of 2026-09; a 9,129-image archive is verified)'. Also update SPEC-02 rev 10's '20000 entries' to point at the same constant.  
**Applied:** ✅ Every "≤300 assets"/"300-page" in events.md, SPEC-01 P1.2, SPEC-04 P0.4 → importMaxEntries (100,000); SPEC-02 rev 10 "20000 entries" → the constant.

#### - [x] F101 · 🟡 MINOR · `MULTIPLE` · specs README Depends-on; SPEC-05/07/08 headers
*Category:* dependency-graph  
**Problem:** SPEC-05, 07 and 08 are listed with no dependencies, yet each publishes through `platform/events` (SPEC-01 P0.6), and SPEC-08 also rides the SPEC-01 P0.3 scheduler. SPEC-09 handles the same situation explicitly.  
**Evidence:** specs/README.md:22, :24, :25 '—'; SPEC-05:20-21 'zero hard dependencies' vs :106-108; SPEC-07:173; SPEC-08:135-136, :152; README.md:49-51 'SPEC-01 P0 owns building it'.  
**Fix:** README cells: SPEC-05 'SPEC-01 P0.6 (fan-out) only; photos via SPEC-12'; SPEC-07 'SPEC-01 P0.6 (P1.5 event); comic leg after SPEC-02'; SPEC-08 'SPEC-01 P0.3 (scheduler) + P0.6 (fan-out); avatars need SPEC-01'. SPEC-08 header: 'rides (or, if first, introduces) SPEC-01 P0.3 and P0.6'. README:49-51 → '...a prerequisite of the first spec to land (SPEC-01 P0.6 in the suggested order; otherwise whichever lands first builds it)'.  
**Applied:** ✅ README Depends-on SPEC-05/07/08; Events bullet "prerequisite of the first spec to land"; SPEC-05/07/08 headers name SPEC-01 P0.6 (SPEC-08 also P0.3).

#### - [x] F102 · 🟡 MINOR · `MULTIPLE` · SPEC-04 header Consumes; SPEC-06/07/08/09 Downstream consumers
*Category:* reciprocity  
**Problem:** Four specs list SPEC-04 as a downstream consumer, but SPEC-04 declares that it consumes only `media:asset_ready`. SPEC-06 credits its own P2 daily digest to 'SPEC-04 P2'.  
**Evidence:** SPEC-04:6 'Consumes: `media:asset_ready`' vs SPEC-07:6, SPEC-08:6, SPEC-09:6. SPEC-06:6 'SPEC-04 P2 daily digest' vs SPEC-06:229 and SPEC-04:162.  
**Fix (revised by verify):** SPEC-04:6 → '**Consumes (live):** `media:asset_ready`, `comic:published`, `movie:published`, `music:track_published`, `story:published`, `social:connection_requested`/`social:connection_accepted` · **Future consumers (each needs a `notify:on_*` task + type row):** `media:playback_completed`, `people:birthday_upcoming`, `ops:backup_failed`/`ops:export_ready`'. SPEC-06:6 → 'this spec's own P2 daily digest (delivered via SPEC-04 channels)'.  
**Applied:** ✅ SPEC-04 header Consumes (live) vs Future consumers; SPEC-06 Downstream credits P2 digest; SPEC-07/08/09 Downstream mark SPEC-04 future.

#### - [x] F103 · 🟡 MINOR · `MULTIPLE` · Downstream-consumers lines of SPEC-01, SPEC-02, SPEC-03
*Category:* reciprocity  
**Problem:** Downstream lists are not reciprocal. SPEC-01 omits SPEC-05/12, 06, 08 and 09; SPEC-02 and SPEC-03 have no Downstream line at all, although SPEC-06/07/09/10 depend on them.  
**Evidence:** SPEC-01:6; SPEC-02:14-18 and SPEC-03:4-5 have none; dependents SPEC-06:4, SPEC-07:4, SPEC-09:4,198-200, SPEC-10:4, SPEC-12:4.  
**Fix (revised by verify):** SPEC-01:6 → 'SPEC-02, SPEC-03 P1, SPEC-04 P0.4, SPEC-05/SPEC-12 (entry photos), SPEC-06 (`media:asset_deleted`; playback via SPEC-07), SPEC-08 P1.7, SPEC-09 (P0.3 scheduler; P1.7 mediaapi ExportProvider)'. SPEC-02: '**Downstream consumers:** SPEC-07 P1.6 (comic leg), SPEC-04 (`comic:published` bell)'. SPEC-03: '**Downstream consumers:** SPEC-06 (stream + dashboard widget), SPEC-09 P1.7 (bank ExportProvider), SPEC-10.'  
**Applied:** ✅ SPEC-01 Downstream reciprocal (SPEC-05/12, 06, 07, 08 P1.7, 09); SPEC-02 Downstream SPEC-07 P1.6, SPEC-04; SPEC-03 Downstream SPEC-06, SPEC-09 P1.7, SPEC-10.

### README.md

#### - [x] F104 · 🟡 MINOR · `README.md` · Conventions — API contract
*Category:* stale convention  
**Problem:** The README says ADR-10 'should land before new endpoint families'. ADR-10 was accepted 2026-07-11, and the concrete rule now lives in MODULES.md §8.  
**Evidence:** README.md:78-80; ADR-10:3 'accepted 2026-07-11'; ADR-10:183-184.  
**Fix (revised by verify):** '**API contract**: declare every endpoint in `shared/openapi.yaml` first, run `make openapi`, and commit the regenerated `api.gen.go` + `types.gen.ts` in the same PR (ADR-10, accepted 2026-07-11; CI `openapi` job diffs them; `backend/MODULES.md` §8). The gate proves presence only; handler conformance stays a review item until handlers implement ServerInterface.'  
**Applied:** ✅ README API contract bullet: spec-first, make openapi, committed codegen, CI openapi job, MODULES.md §8, handler conformance as review item.

#### - [x] F105 · 🟡 MINOR · `README.md` · Header — Language policy
*Category:* stale reference  
**Problem:** The README says the old vi mirror 'is a frozen archive'. It was deleted in f11cf3f, and ADR-11 replaced part of ADR-09.  
**Evidence:** README.md:5-6; ADR-09:3, :11-13.  
**Fix (revised by verify):** '**Language policy:** English only, per owner decision 2026-07-07 (ADR-09; the former vi mirror was deleted in `f11cf3f` — see ADR-11).'  
**Applied:** ✅ README + briefs README: vi mirror deleted in f11cf3f (ADR-11).

#### - [x] F106 · 🟡 MINOR · `README.md` · Documents table — SPEC-10 row
*Category:* incomplete dependency  
**Problem:** The SPEC-10 row omits SPEC-01, which SPEC-10's header lists for P1.10 receipts.  
**Evidence:** README.md:27 'SPEC-03; SPEC-04 for reminders'; SPEC-10:4.  
**Fix (revised by verify):** 'SPEC-03; SPEC-04 for reminders; SPEC-01 (P1.10 receipts only)'  
**Applied:** ✅ README SPEC-10 row: SPEC-01 (P1.10 receipts only).

#### - [x] F107 · 🟡 MINOR · `README.md` · Conventions (new Definition of done block)
*Category:* DoD-consistency  
**Problem:** The DoD is scattered and uneven. SPEC-07 omits the events.md registration its brief promised, and nothing requires the per-spec test-case doc or traceability matrix.  
**Evidence:** SPEC-07:167-175 has no 'register in events.md', unlike briefs/07 P1.5, SPEC-05:109-110, SPEC-08:160 and SPEC-09:270-271. docs/testing/TEST-CASES-SPEC-*.md and TRACEABILITY-MATRIX.md are never named in README.md:36-86. events.md:124-127 'the only contract'.  
**Fix (revised by verify):** Add one line under Conventions: '**Definition of done (every spec PR)** also includes a `docs/testing/TEST-CASES-SPEC-NN-<module>.md` with rows citing the real `_test.go` evidence.' Drop the TRACEABILITY-MATRIX reference (no such file exists) and the SPEC-07 edit (the README events rule already binds it).  
**Applied:** ✅ README DoD: TEST-CASES-SPEC-NN doc citing _test.go evidence; TRACEABILITY-MATRIX link kept (file exists — verify note was wrong).

### SPEC-01-media-image-pipeline.md

#### - [x] F108 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §5 P0.6 Event fan-out
*Category:* factually wrong cross-ref  
**Problem:** P0.6 claims events.md warns that publishing an unregistered event name panics cmd/worker. events.md says the opposite: Publish on an unregistered name returns nil, and emitting with zero consumers is encouraged. The panic comes from duplicate handler registration. P0.6 also lists `media:asset_ready` as a P0 producer, but it is P1.2.  
**Evidence:** SPEC-01:299-304 'SPEC-01 is the first producer (`media:asset_ready`, `media:asset_deleted`) ... events.md warns that publishing an event name with no registered subscription panics `cmd/worker`'. events.md:113-114 '`Publish` on an unregistered name returns nil by design'; :100 'Emitting with zero consumers is normal and encouraged'; :75-81 ServeMux panics on duplicate registration. SPEC-01:311 P1.2 emits asset_ready.  
**Fix (revised by verify):** Replace :299-304 with: 'SPEC-01 is the first producer (`media:asset_deleted` at P0.3; `media:asset_ready` at P1.2) and owns building the fan-out. Asynq's ServeMux panics on duplicate handler registration, so two consumers can never both handle the raw event task type; the registry must exist before any event gains a second consumer. Publishing with zero subscriptions is a silent no-op by design.'  
**Applied:** ✅ Silent-no-op half already fixed by cross-spec pass; asset_deleted at P0.3, asset_ready at P1.2, ServeMux duplicate-registration rationale.

#### - [x] F109 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §5 P0.3 Shared periodic scheduler
*Category:* wrong cross-ref  
**Problem:** The shared-scheduler list attributes `people:scan_birthdays` to SPEC-05; it belongs to SPEC-08. The list also leaves out `ops:purge_exports` (SPEC-09 P1.7).  
**Evidence:** SPEC-01:218-219 '`ops:backup_database` (SPEC-09), `people:scan_birthdays` (SPEC-05), `notify:purge_old` (SPEC-04)'; SPEC-08:135 'Daily periodic task **`people:scan_birthdays`**'; events.md:59 '(SPEC-08 P0.4 ...)'; SPEC-09:210 '`ops:purge_exports`'.  
**Fix (revised by verify):** Change :218-219 to '`ops:backup_database` and `ops:purge_exports` (SPEC-09), `people:scan_birthdays` (SPEC-08 P0.4), `notify:purge_old` (SPEC-04)'.  
**Applied:** ✅ Scheduler list: ops:backup_database + ops:purge_exports (SPEC-09), people:scan_birthdays (SPEC-08 P0.4).

#### - [x] F110 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §5 P0.1, P1.2
*Category:* contradiction with registry  
**Problem:** The `media:process_image` payload is given as `{asset_id}`, but the registry defines `{asset_id, source_key, owner_user_id}`.  
**Evidence:** SPEC-01:98 'enqueues `media:process_image` `{asset_id}`'; :317 'the worker task payload (`{asset_id}`)'; events.md:44 '`{asset_id, source_key, owner_user_id}`'.  
**Fix (revised by verify):** Change both occurrences to '`{asset_id, source_key, owner_user_id}`', keeping the P1.2 point that `origin` is read from `assets.origin`, not the payload.  
**Applied:** ✅ process_image payload {asset_id, source_key, owner_user_id} in P0.1/P1.2; origin read from assets.origin.

#### - [x] F111 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §6 vs §5 P1.2, P1.1
*Category:* contradiction  
**Problem:** The `title` semantics conflict: §6 says it 'defaults to' original_filename (populated at creation), while P1.2 says it 'falls back ... when unset' (nullable). PATCH rules for empty or null titles are undefined.  
**Evidence:** SPEC-01:384-385 '`title` defaults to it' vs :314 '`title` falls back to `original_filename` when unset'; §6 :371 `ADD COLUMN title text`; P1.1 :308-310 gives no validation.  
**Fix (revised by verify):** §6 and P1.2: '`title` is set to `original_filename` when the row is created (upload session or mediaapi ingest). Rows predating this migration may have NULL `title`/`original_filename`, so every read (grid, `media:asset_ready`, Content-Disposition) uses `COALESCE(title, original_filename, id::text)`.' P1.1: 'PATCH `{title}` accepts a trimmed string of 1–200 chars; an empty string gets 422; `{title: null}` resets it to `original_filename`.'  
**Applied:** ✅ §6/P1.2 title = original_filename at creation, reads COALESCE(title, original_filename, id::text); P1.1 1–200 chars, "" → 422, null resets.

#### - [c] F112 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §5 P0.4, §7 GET /assets
*Category:* ambiguity  
**Problem:** The cursor list contract is incomplete. There is no `limit`, no `next_cursor` response shape, and nothing says whether `status=processing` expands to include `uploading` on the server. The LCP AC cannot be tested without a page size.  
**Evidence:** SPEC-01:248-253; §7:404 '`?kind=&status=&cursor=`'; :245-247 'the `processing` filter includes still-`uploading` sessions'; AC :257-258.  
**Fix (revised by verify):** §7: '`GET /api/v1/assets?kind=&status=&cursor=&limit=` (default 50, max 100; out-of-range values fall back to 50) → `{assets: [...], next_cursor?}` (`next_cursor` present only when another page exists); opaque cursor over `(created_at, id)` DESC; `status=processing` expands server-side to `processing,uploading`; `deleting` is never returned; a malformed cursor gets 400 Problem `media/bad_request` ("invalid cursor").' LCP AC: 'the first page (≤ 50 thumbs) renders with LCP < 2.5 s'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §7 Pagination: limit 50/100, next_cursor only when more, processing expands to processing,uploading, deleting never returned; P0.4 + LCP AC ≤ 50 thumbs; README {items}/media/invalid-cursor kept. Code follow-up: handler/OpenAPI return {assets} and media/bad_request.

#### - [x] F113 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §5 P0.1 Serving variants vs P0.3 AC
*Category:* ambiguity  
**Problem:** Variants are served with 'long-lived cache headers' but no value is given. Public or immutable caching would keep serving a deleted asset's variants, so the P0.3 '404' AC would hold only at the origin.  
**Evidence:** SPEC-01:130-131 'long-lived cache headers'; AC :225-226 'variant URLs ... return 404/403'.  
**Fix (revised by verify):** '`Cache-Control: private, max-age=600`, with no `immutable` and no year-long max-age, so a delete or un-share takes effect within minutes. (Assets later made public via visibility may use `public, max-age=86400`.)' Qualify the P0.3 AC: 'variant/HLS/original URLs return 404 from the API origin (cache bypassed); browser caches may serve a variant for ≤ 10 min'.  
**Applied:** ✅ Variant Cache-Control private max-age=600 (public 86400, no immutable); P0.3 AC qualified. Matches code.

#### - [x] F114 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §11
*Category:* revision history  
**Problem:** The rev 3 history entry omits substantive rev-3 changes, including a new P0 requirement (P0.6), the shared scheduler, the abandoned-upload sweep, the origin column, the indexes and more.  
**Evidence:** SPEC-01:459 r3 lists (1)–(6) only; the body carries rev-3 tags F002 (:215), F006 (:242), F007 (:295), F009 (:319), F010 (:94), F038 (:209), F039 (:273), F040 (:376).  
**Fix (revised by verify):** Append to r3: '(7) P0.6 `platform/events` fan-out (F007); (8) single shared `asynq.Scheduler` (F002); (9) abandoned `uploading` sweep (F038); (10) `assets.origin` (F009) and `assets_deleting_idx`/`assets_owner_cursor_idx` (F040); (11) unsupported-format → object deleted + failed (F010); `original_filename` fallback (F039); template-registry note (F006).'  
**Applied:** ✅ r3 history items (7)–(11); r4 row.

#### - [c] F115 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §5 P0.3 janitor / §6 indexes
*Category:* data-model  
**Problem:** The abandoned-upload sweep has no supporting index. `assets_deleting_idx` is partial on `deleting`, yet the spec calls the whole janitor an 'indexed scan'.  
**Evidence:** SPEC-01:205-207 'WHERE status='uploading' AND updated_at < ...'; :211-213 'indexed scan (`assets_deleting_idx`)'; §6 :377.  
**Fix (revised by verify):** Add `CREATE INDEX assets_uploading_idx ON assets (updated_at) WHERE status = 'uploading';` to §6, and change :211-212 to 'indexed scans (`assets_deleting_idx`, `assets_uploading_idx`)'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §6 assets_uploading_idx; janitor "indexed scans". Code follow-up: needs a 000N_media_uploading_idx migration.

#### - [x] F116 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §7; P0.3; P0.5
*Category:* api-contract  
**Problem:** §7 omits the upload-session endpoint (`POST /api/v1/assets`) that §6 changes to record `original_filename` and `origin`. P0.3's 403 for a cross-owner DELETE is out of step with the owner-only 404 used elsewhere (SPEC-07).  
**Evidence:** SPEC-01:384-388 'The upload-session endpoint starts recording `original_filename` ... `'upload'` by the upload-session endpoint'; §7 (:397-404) has no POST /assets row. SPEC-01:229-230 'then 403'; SPEC-07:77-79 '404s ... not owned by the caller'.  
**Fix (revised by verify):** Add a §7 row: '| POST | `/api/v1/assets` | `assets:write:own` | modified: records `original_filename`, sets `title = original_filename` and `origin='upload'`; response unchanged |'. In P0.3 after the 403 AC: '403 (not 404) is deliberate: RequireOwnerOrPermission answers 404 only for a missing row and 403 for another owner's asset. A row hidden by RLS surfaces as 404 `media/asset-not-found`.'  
**Applied:** ✅ §7 POST /api/v1/assets row (original_filename, title, origin='upload'); P0.3 AC explains cross-owner delete 403 vs RLS-hidden 404.

### SPEC-02-comic-vertical.md

#### - [x] F117 · 🟡 MINOR · `SPEC-02-comic-vertical.md` · P0.2 Seeding paragraph
*Category:* internal-contradiction / rbac-seeding  
**Problem:** The Seeding paragraph still seeds `comics:write:own` and `comics:publish:own` to `creator` and says ':any moderation + delete stay creator+/admin'. Rev 7 (0025) moved them to `user`, the `:any` codes are editor-tier, and the `movies:publish` precedent is stale.  
**Evidence:** SPEC-02:103-106 '0025 ... widened ... to the base user role ... :any moderation + delete stay creator+/admin' vs :129-132 '`comics:write:own` + `comics:publish:own` → `creator`; `comics:write:any` + `comics:publish:any` → `editor` ... mirroring movies:write:any + movies:publish'. 0021 re-seeded `movies:publish:own/:any`.  
**Fix (revised by verify):** Change :106 to '`:any` write/publish moderation stays editor, and `comics:delete:any` stays admin.' In :129-132, append '(widened to `user` by `0025_comic_user_write_grant`, rev 7)' after '→ `creator`', and change the precedent to '`movies:write:any` + `movies:publish:any` (0021)'.  
**Applied:** ✅ Seeding paragraph: 0025 widened :own to user; precedent movies:write:any + movies:publish:any (0021); rev 7 prose corrected.

#### - [x] F118 · 🟡 MINOR · `SPEC-02-comic-vertical.md` · P0.2 permission prose/table vs §7
*Category:* internal-contradiction  
**Problem:** P0.2 says every mutation goes through `comics:write:any` and that publish is a `{status}` change. §7 uses `comics:delete:any` for page delete and dedicated publish/unpublish endpoints, and forbids status changes via PATCH.  
**Evidence:** SPEC-02:95-97 '**Every mutation on an existing comic/chapter/page** goes through `RequireOwnerOrPermission(engine, "comics:write:any", ...)`'; :101-102 'a `{status}` publish/unpublish change'; :124 vs :457-459 '**dedicated endpoint** (not a `PATCH {status}`)' and :466 'owner, or `comics:delete:any`'.  
**Fix (revised by verify):** At :101, replace 'a `{status}` publish/unpublish change' with '`POST /comics/{id}/publish` / `/unpublish`'. At :124, replace '(`{status}` change)' with '(`POST /comics/{id}/publish`, `/unpublish`)'.  
**Applied:** ✅ P0.2 + table cite POST /comics/{id}/publish and /unpublish.

#### - [c] F119 · 🟡 MINOR · `SPEC-02-comic-vertical.md` · P0.2 acceptance criteria
*Category:* AC-quality  
**Problem:** The cross-owner write AC accepts '404/403', so it is not deterministic. Publish/unpublish authorisation has no AC.  
**Evidence:** SPEC-02:146-147 'then 404/403 and no change'; the ACs at :141-149 do not cover the :124 publish rows.  
**Fix (revised by verify):** Replace :146-147 with: 'Given creator D (holds comics:write:own only) mutating creator C's comic, then 403 if C's comic is published, or 404 comic/not-found if it is a draft, and no change. extractComicOwner returns 404 comic/not-found for a draft when the caller is neither the owner nor a holder of the endpoint's :any code; holders of :any (editors) resolve drafts normally.' Add: 'Given an editor with comics:publish:any, when POST /comics/{C}/unpublish on C's published comic, then 200 and status draft.' Add (with the finding #9 chaining): 'Given an owner whose role lacks comics:publish:own, when POST /comics/{id}/publish on their draft, then 403 and status unchanged.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — Deterministic AC (403 published, 404 comic/not-found draft); editor-unpublish + owner-lacks-publish:own ACs. Code follow-up: owner extractor resolves drafts for everyone (draft → 403). TC-COMIC-036/102/140 now stale.

#### - [x] F120 · 🟡 MINOR · `SPEC-02-comic-vertical.md` · §3 Non-goals
*Category:* stale  
**Problem:** RTL and page-spread layout are still listed as a P2 non-goal, but they shipped as R3.  
**Evidence:** SPEC-02:48 '- Reading-direction RTL (manga) and page-spread layout intelligence — P2.' vs :362-365 'R3 ... *Status: implemented 2026-08-07.*'  
**Fix (revised by verify):** Replace :48 with '- Automatic page-spread detection (double-page art); RTL and manual double-page pairing shipped as R3.' and mark the P2 bullet at :319-320 as '— shipped as R3 (2026-08-07)'.  
**Applied:** ✅ §3 non-goal: auto spread detection only; P2 bullet marked shipped as R3 (2026-08-07).

#### - [c] F121 · 🟡 MINOR · `SPEC-02-comic-vertical.md` · §5 P1.8 bookmarks sketch / P0.6 handler
*Category:* data-model  
**Problem:** The `comic_bookmarks` sketch has no identity-anchor FK on user_id, so deleting a user orphans rows. The P0.6 cover-nulling UPDATE has no index.  
**Evidence:** SPEC-02:301-303 FK on page_id only; :387/:416 use `REFERENCES users(id) ON DELETE CASCADE`. :248 'sets any `comics.cover_asset_id = NULL`'; :390 has no index.  
**Fix (revised by verify):** Change the sketch to `user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, page_id uuid NOT NULL REFERENCES comic_pages(id) ON DELETE CASCADE, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (user_id, page_id)`. Add `CREATE INDEX comics_cover_asset_idx ON comics (cover_asset_id) WHERE cover_asset_id IS NOT NULL;` to §6.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — Bookmarks sketch users(id) anchor, full types, tenancy note; comics_cover_asset_idx in §6. Code follow-up: no cover index shipped.

### SPEC-03-finance-ledger.md

#### - [c] F122 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.1 Accounts / P0.3
*Category:* ambiguity  
**Problem:** Archiving is defined only as UI picker behaviour. The API behaviour for new transactions or transfers on an archived account is undefined, and GET /bank/accounts has no archived filter.  
**Evidence:** SPEC-03:88-89 'absent from transaction-entry pickers but present in historical reports'; §7 :467; P0.3 :133-134.  
**Fix (revised by verify):** P0.1: 'Archived accounts reject new transactions and transfer legs (409 `bank/account-archived`). Edits and deletes of existing rows are allowed. Unarchive via PATCH `{archived:false}`. GET /bank/accounts returns all accounts with the `archived` flag.' Add `bank/account-archived` to §7.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.1 archived accounts reject new transactions/transfer legs with 409 bank/account-archived; edits/deletes allowed; unarchive via PATCH; AC + §7 type. Code follow-up: service never checks archived on write.

#### - [c] F123 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.2 / P0.1
*Category:* data integrity  
**Problem:** A transaction PATCH that moves the row to an account with a different currency would silently reinterpret the integer minor-unit amount in the new currency.  
**Evidence:** SPEC-03:68-69 'I correct a mistyped transaction (wrong ... account)'; :133-134 rejects only cross-currency transfers.  
**Fix (revised by verify):** P0.2: 'A PATCH that changes `account_id` to an account with a different `currency` is 422 `bank/currency-mismatch`. Same-currency moves re-derive both balances.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 PATCH onto another-currency account → 422 bank/currency-mismatch; same-currency move re-derives balances; AC. Code follow-up: UpdateTransaction does not check currency.

#### - [x] F124 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.2, P0.6, §8
*Category:* stale naming / wrong path  
**Problem:** P0.2 and P0.6 still refer to a `(bank)` route group, which §8 says does not exist. §8's brace-expanded path yields `bank/page/page.tsx`. §8 names Recharts, which is not a dependency.  
**Evidence:** SPEC-03:110 'anywhere in the `(bank)` group'; :268 'the `(bank)` group landing page' vs :519-520 'there is **no** separate `(bank)` route group'; :518 '`app/(app)/bank/{page,transactions,accounts,budgets}/page.tsx`'; :525 'Recharts'. package.json has no recharts; charts are hand-rolled in Charts.tsx.  
**Fix (revised by verify):** :110 → 'reachable from every `/bank/*` page'. :268 → 'Frontend `/bank` (`app/(app)/bank/page.tsx`) renders it as the bank landing page.' :518 → '`app/(app)/bank/page.tsx` and `app/(app)/bank/{transactions,accounts,budgets}/page.tsx`'. :525: drop 'Recharts' unless it is an existing dependency, and state 'budget bars as inline SVG/CSS components (any chart dependency needs a bundle-budget check)'.  
**Applied:** ✅ P0.2 "reachable from every /bank/* page"; P0.6 landing page named; §8 paths split; Recharts → inline SVG/CSS + bundle note.

#### - [x] F125 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.7 Events
*Category:* acceptance criteria  
**Problem:** The P0.7 AC says 'exactly one matching event' per write, but a transfer write touches two rows and emits two events, which SPEC-06 relies on.  
**Evidence:** SPEC-03:301-302 'exactly one matching event is emitted'; :287 'Emitted for transfer legs too'; SPEC-06:124 'one transfer (two `bank:transaction_created` legs)'.  
**Fix (revised by verify):** Add a P0.7 AC: 'Given POST/PATCH/DELETE /bank/transfers, exactly two events are emitted after commit: one per leg, `is_transfer=true`, the same `transfer_id`, and each with `counterparty_account_id` set to the other leg's account.'  
**Applied:** ✅ P0.7 AC: transfer POST/PATCH/DELETE emits exactly two events (one per leg). Matches emitTransferLegs.

#### - [x] F126 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · §11 Open questions
*Category:* wrong cross-reference  
**Problem:** The life-stream privacy question names the notification module as the deciding consumer. The consumer is SPEC-06's stream, which already owns this question.  
**Evidence:** SPEC-03:572-573 'the *consumer* (notification module) decides display'; :295-296 (SPEC-06 is the first consumer); SPEC-06:296-298.  
**Fix (revised by verify):** '...the *consumer* (SPEC-06's life stream) decides display. SPEC-06 §11 defaults to show for n=1; revisit at household tenancy.'  
**Applied:** ✅ §11 privacy question names SPEC-06 stream as consumer, show-for-n=1 default, revisit at household tenancy.

#### - [x] F127 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.3 Transfers
*Category:* brief coverage  
**Problem:** The brief says editing a leg edits the pair. The spec instead rejects leg edits with 409 and does not note the departure.  
**Evidence:** briefs/03-finance-ledger.md:66 'Editing/deleting one leg edits/deletes the pair atomically.' vs SPEC-03:131-132 '409 `bank/is-transfer-leg`'.  
**Fix (revised by verify):** Append to P0.3: '(Refines the brief: pair edits go only through `/bank/transfers/{transfer_id}` so a leg is never half-edited; the leg endpoint returns 409 with the transfer URL in the Problem `detail`.)'  
**Applied:** ✅ P0.3 refines the brief: pair edits only via /bank/transfers/{transfer_id}; 409 detail names it.

#### - [c] F128 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.5 / §7 budgets
*Category:* ambiguity  
**Problem:** Month format and location are undefined for PUT /bank/budgets (query or body). `?month=` appears only by example, and an omitted month has no default or timezone rule.  
**Evidence:** SPEC-03:224-226 'PUT /bank/budgets upserts one (category_id, month, amount)'; §7 :475 'GET/PUT `/api/v1/bank/budgets?month=`'; §6 :453.  
**Fix (revised by verify):** §7: '`month` is `YYYY-MM` on the wire everywhere, stored as the first-of-month date. The PUT body is `{category_id, month, amount}`. An omitted `?month=` on GET means the current month in the caller's timezone (D-17). Malformed → 400 `bank/invalid-month`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §7 Month rule (YYYY-MM wire, first-of-month storage, PUT month in body, default caller-TZ month, 400 bank/invalid-month); PUT row /api/v1/bank/budgets {category_id, month, amount}. Code follow-up: UTC default, about:blank on malformed.

#### - [x] F129 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.5 ACs
*Category:* acceptance criteria  
**Problem:** The budget AC percentages depend on an unstated rounding rule: 106.67% is shown as 107%, and 1/3 as 33%.  
**Evidence:** SPEC-03:246-247 'the bar shows 107%'; :249-250 '33%'.  
**Fix (revised by verify):** P0.5: 'Percent shown = `round(spent*100/amount)`, rounded half-up. The >100% highlight uses the exact comparison `spent > amount`.'  
**Applied:** ✅ P0.5 percent round(spent×100/amount) half-up; highlight spent > amount.

#### - [x] F130 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.8 RBAC
*Category:* contradiction  
**Problem:** The prose preserves an 'explicit admin wildcard' cross-user read. But every query is owner-scoped, and the AC says this holds for superadmin too, so an implementer might build an admin bypass into finance data.  
**Evidence:** SPEC-03:323-326 'No cross-user read at any permission level except explicit admin wildcard'; :308 'every query is owner-scoped'; :333-334 'holds for superadmin too'.  
**Fix (revised by verify):** Replace :323-326 with: '**No cross-user read or write at any level, including `*`.** The wildcard only passes the route gate; every query filters `user_id = caller`. Any future admin or household access is a newly specced surface (new `:any` code, audited endpoint, ADR), never a side effect of the wildcard.'  
**Applied:** ✅ P0.8 no admin-wildcard exception; cross-user access needs a new :any code, audited endpoint and ADR.

#### - [c] F131 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · §7 pagination note; §6 indexes and constraints
*Category:* factual inaccuracy / data-model  
**Problem:** The note claims cursor pagination avoids duplicates and skips under `occurred_at` edits, which keyset paging does not. The §6 indexes also do not cover the `id` tiebreaker, the dashboard `created_at` list, the category FK and reassign scans, or seed+own category reads. There is no self-parent guard and no currency format CHECK.  
**Evidence:** SPEC-03:480-482 'offset ... duplicates/skips rows under inserts and `occurred_at` edits'; §6 :441 `(user_id, occurred_at DESC)`; :266 dashboard `created_at DESC`; :189-195 reassign/409 scans on category_id; :409 parent_id; :399 `currency char(3)` with no CHECK. Shipped 0014 added these indexes ad hoc.  
**Fix (revised by verify):** Reword the §7 note: 'offset paging duplicates or skips rows under inserts; keyset paging (`occurred_at DESC, id DESC`) is stable under inserts, though a row whose `occurred_at` is edited mid-scroll may cross the cursor (acceptable for a personal list).' Make §6 mirror the shipped 0014: `bank_transactions (user_id, occurred_at DESC, id DESC)`, `(category_id) WHERE category_id IS NOT NULL`, `bank_categories (user_id)`, `(parent_id) WHERE parent_id IS NOT NULL`. Add: 'Follow-up migration `000N_bank_integrity`: `CREATE INDEX ON bank_transactions (user_id, created_at DESC)` (dashboard recency, P0.6); `CREATE INDEX ON bank_budgets (category_id)` (cascade and reassign scans); `ALTER TABLE bank_categories ADD CHECK (parent_id IS DISTINCT FROM id)` (a childless top-level category could otherwise parent itself); `ALTER TABLE bank_accounts ADD CHECK (currency ~ ''^[A-Z]{3}$'')`. The service also rejects `parent_id = id` with 422 `bank/invalid-category-parent`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §7 keyset note reworded; §6 indexes mirror shipped 0014; follow-up 000N_bank_integrity migration specced; P0.4 notes parent_id = id rejected. Code follow-up: integrity migration not shipped.

#### - [x] F132 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P1.10 Receipt attachments
*Category:* module boundary  
**Problem:** `receipt_asset_id` references a media asset, but the spec does not address the no-cross-module-FK rule, ownership validation, or cleanup on asset deletion. Bank is not a `media:asset_deleted` consumer.  
**Evidence:** SPEC-03:350-351; README.md:43-45; events.md:21 consumers omit bank.  
**Fix (revised by verify):** P1.10: '`receipt_asset_id uuid NULL`, **no FK** (module boundary). On write, validate via mediaapi that the asset exists, is owned by the caller and is an image (otherwise 404 `media/asset-not-found` / 422). Add a `bank:on_asset_deleted` consumer of `media:asset_deleted` that NULLs matching ids, and register it in events.md.'  
**Applied:** ✅ P1.10 receipt_asset_id uuid NULL (no FK), mediaapi validation, bank:on_asset_deleted consumer; events.md row added.

#### - [x] F133 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · §7 dashboard row
*Category:* authz-contract  
**Problem:** `/bank/dashboard` is gated only by `bank-accounts:read:own`, yet it returns transactions and budgets, which have their own read codes.  
**Evidence:** SPEC-03:476 '`bank-accounts:read:own`'; :258-267 returns budgets and recent transactions; :319-320 separate read codes.  
**Fix (revised by verify):** Add under §7: 'Dashboard is a deliberate composite read gated on `bank-accounts:read:own`; all bank read codes are always granted together to `user` (P0.8). Revisit if they diverge.'  
**Applied:** ✅ §7 note: dashboard is a composite read gated on bank-accounts:read:own.

#### - [x] F134 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · §10 Timeline
*Category:* stale ordinal  
**Problem:** 'The largest of the three specs' is stale: there are now nine sibling specs, although 8.5 days is still the largest P0.  
**Evidence:** SPEC-03:565 'P0 ≈ 8.5 dev-days — the largest of the three specs'.  
**Fix (revised by verify):** 'P0 ≈ 8.5 dev-days (category semantics and the budget tree grew it past the original 7)'. Drop the 'of the three specs' comparison, or re-verify it against the current spec set before restating it.  
**Applied:** ✅ §10 "largest of the three specs" → "P0 ≈ 8.5 dev-days (grew past 7)".

### SPEC-04-notification-module.md

#### - [x] F135 · 🟡 MINOR · `SPEC-04-notification-module.md` · §5 P0.2 step 2 / P1.3
*Category:* wrong cross-reference  
**Problem:** 'Overridable per type, §6' points at §6, which has only column defaults and no per-type mechanism. P1.3 GET needs the set of registered types, and PUT behaviour for unknown or non-mutable types is undefined.  
**Evidence:** SPEC-04:85 '— overridable per type, §6'; §6 :186-194; P1.3 :156.  
**Fix (revised by verify):** :85 → '— a type may override this default in the code-level type registry (`notify/types.go`, mirrored in `notify/README.md`), which also marks non-mutable and non-persisted types'. P1.3: 'GET returns every registered type with its effective setting (stored row or registry default) and a `mutable` flag. PUT on an unregistered type → 422 `notify/invalid-preference`. `muted=true` or `in_app/email=false` that would disable a non-mutable type's forced channel → 422 `notify/type-not-mutable`.' Register both in §7 and i18n.  
**Applied:** ✅ P0.2 step 2 per-type override → code-level type registry (notify/types.go); P1.3 GET/PUT semantics + notify/invalid-preference, notify/type-not-mutable in §7.

#### - [c] F136 · 🟡 MINOR · `SPEC-04-notification-module.md` · §5 P0.4 click-through / P1.4
*Category:* ambiguity  
**Problem:** `data.href` is required for in-app types, but P1.4's security alert has none, and dispatch behaviour for a missing href is undefined.  
**Evidence:** SPEC-04:133 'required `data.href`'; :157 P1.4 intent has no href; step 5 (:88) checks only user_id and type.  
**Fix (revised by verify):** Step 5: 'A malformed intent (missing `user_id`/`type`, or an in-app-persisted intent missing `data.href`) returns `asynq.SkipRetry`.' Optionally make P1.4's intent explicit with `data:{href:"/settings/security"}`.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 step 5 malformed includes in-app intent missing data.href + AC; P1.4 data.href /settings/security (route not built). Code follow-up: dispatchIntent checks only user_id/type.

#### - [c] F137 · 🟡 MINOR · `SPEC-04-notification-module.md` · §6 web_push_subscriptions; P1.1; §7
*Category:* ambiguity / data-model / api-contract  
**Problem:** `endpoint` is globally UNIQUE, so a second account subscribing from the same browser violates it, with undefined behaviour. DELETE has no identifier, `user_id` has no index for delivery, and security.md lists different paths.  
**Evidence:** SPEC-04:199 `endpoint text NOT NULL UNIQUE`; :154 and §7:220 'POST/DELETE `/api/v1/me/push-subscriptions`' (no `{id}`, no body); no index on user_id; security.md:489-493 '/me/web-push/subscribe', 'DELETE /me/web-push/{id}'.  
**Fix:** P1.1: 'POST upserts: `ON CONFLICT (endpoint) DO UPDATE SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth, last_used_at = NULL` (the endpoint belongs to whoever subscribed last); body `{endpoint, keys:{p256dh, auth}}` → 201 `{id}`. `DELETE /api/v1/me/push-subscriptions/{id}` deletes only the caller's row (204; 404 `notify/push-subscription-not-found`).' Split the §7 row per method; add `CREATE INDEX web_push_subscriptions_user_idx ON web_push_subscriptions (user_id);`; update security.md:489-493 to SPEC-04's paths.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P1.1 upsert ON CONFLICT (endpoint), 201 {id}, DELETE /{id} caller-only 204/404, notify/invalid-push-subscription; §6 web_push_subscriptions_user_idx; §7. Code follow-up: 0009 lacks the user_id index.

#### - [x] F138 · 🟡 MINOR · `SPEC-04-notification-module.md` · §4 story 4 / P0.5
*Category:* brief coverage  
**Problem:** The story says clicking an item navigates and clears the badge, but P0.5 never says a click marks the item read, and there is no AC for it.  
**Evidence:** SPEC-04:45 'click one to go to its target, and the badge clears'; P0.5 :143-150.  
**Fix:** P0.5: 'Clicking an unread item fires the optimistic mark-read mutation and navigates to `data.href`.' AC: 'Given an unread item, when clicked, the app navigates to data.href, the item renders read, and the badge decrements by 1.'  
**Applied:** ✅ P0.5 click → optimistic mark-read + navigate + AC. Already shipped.

#### - [x] F139 · 🟡 MINOR · `SPEC-04-notification-module.md` · §5 P1.2
*Category:* stale sibling doc  
**Problem:** P1.2 supersedes frontend.md Phase 6's SSE sketch but creates no obligation to edit it, so frontend.md still contradicts the one reconciliation rule.  
**Evidence:** SPEC-04:155 'This supersedes frontend.md Phase 6's `/api/v1/events/stream` + "mutate cache directly, no refetch" sketch'; frontend.md:940 still has it.  
**Fix:** Append: 'DoD: in the same PR, rewrite frontend.md Phase 6 to `/api/v1/me/notifications/stream` with invalidate-and-refetch semantics, linking here.'  
**Applied:** ✅ P1.2 DoD: rewrite frontend.md Phase 6 in the same PR.

#### - [x] F140 · 🟡 MINOR · `SPEC-04-notification-module.md` · §7 permission-grammar note
*Category:* rbac-grammar-fact  
**Problem:** The note says codes are 3-segment because 'rbac.Parse rejects anything else', then quotes 'must have 2 or 3 segments'. Two-segment codes are valid and are equivalent to `:any`.  
**Evidence:** SPEC-04:229; rbac/permission.go:62-75 accepts 2 or 3; :14-16,138-141.  
**Fix (revised by verify):** 'Permission codes above are 3-segment because they are owner-scoped (`:own`). `rbac.Parse` accepts 2 or 3 segments and rejects 4+ ("must have 2 or 3 segments"). A 4-segment module-prefixed code wired through `RequirePermission` panics at server start (`MustParse`), and any dynamic `AllowsCode` check fails closed.'  
**Applied:** ✅ §7 permission-grammar note: 3-segment because owner-scoped; Parse accepts 2–3, rejects 4+.

#### - [c] F141 · 🟡 MINOR · `SPEC-04-notification-module.md` · §6 notifications indexes
*Category:* data-model  
**Problem:** The unread partial index covers only `user_id`. It does not serve the `?status=unread` cursor page or the read-all watermark UPDATE.  
**Evidence:** SPEC-04:56, :66 order `created_at DESC, id DESC`; :58; §6 :183 `CREATE INDEX ON notifications (user_id) WHERE read_at IS NULL;`.  
**Fix (revised by verify):** Optional: `CREATE INDEX notifications_unread_idx ON notifications (user_id, created_at DESC, id DESC) WHERE read_at IS NULL;`. It serves the badge count, the unread page and the read-all UPDATE.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §6 notifications_unread_idx (user_id, created_at DESC, id DESC) WHERE read_at IS NULL. Code follow-up: shipped 0009 index is (user_id) only.

#### - [c] F142 · 🟡 MINOR · `SPEC-04-notification-module.md` · §5 P0.3 token storage / §6
*Category:* data-model  
**Problem:** The reset-token table is listed as columns only. Nothing makes `token_hash` UNIQUE or indexed, and the ON DELETE behaviour is unstated.  
**Evidence:** SPEC-04:104 '(id, user_id → users, token_hash, expires_at, used_at, created_at)', 'lookup by hash'; §6 (:170-206) has no DDL.  
**Fix:** Add to §6 (account-owned): `CREATE TABLE password_reset_tokens (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32), expires_at timestamptz NOT NULL, used_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()); CREATE INDEX password_reset_tokens_user_idx ON password_reset_tokens (user_id);`  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §6 account-owned password_reset_tokens DDL (UNIQUE token_hash, octet_length = 32 CHECK, CASCADE, user_idx); cites 0010. Code follow-up: 0010 lacks the CHECK.

#### - [c] F143 · 🟡 MINOR · `SPEC-04-notification-module.md` · P0.5 Bell wiring
*Category:* pagination-convention  
**Problem:** P0.5 prescribes a plain `useQuery` on a cursor-paginated list. frontend/CLAUDE.md requires `useInfiniteQuery` for such lists, and without it notifications older than the first page are unreachable.  
**Evidence:** SPEC-04:143 '`useQuery(["notifications"])`'; §7:216 `next_cursor`; frontend/CLAUDE.md:59-61.  
**Fix:** '`useInfiniteQuery(["notifications"])`; the dropdown renders page 1 and loads more via `useInfiniteScroll`; `unread_count` comes from the latest first page; optimistic patches map over `data.pages[].items`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.5 useInfiniteQuery + useInfiniteScroll, unread_count from latest first page. Code follow-up: NotificationsMenu uses plain useQuery.

#### - [c] F144 · 🟡 MINOR · `SPEC-04-notification-module.md` · P0.3 email channel links
*Category:* auth-cookie  
**Problem:** Email deep links into authenticated routes bounce logged-in users to /login. A link opened from webmail is a cross-site navigation, and the `portal_session` marker cookie the middleware checks is SameSite=Strict, so it is not sent.  
**Evidence:** account/handler/auth.go:706-714 `SameSite: http.SameSiteStrictMode`; middleware.ts `req.cookies.has("portal_session")` → /login; SPEC-04:133 every type carries a `data.href` deep link.  
**Fix (revised by verify):** P0.3: 'Email links never point directly at authenticated routes. They target a public trampoline, `/open?next=<relative path>`, which is outside the `src/middleware.ts` matcher. It validates that `next` is a same-origin relative path (it must start with `/` and must not start with `//`), then performs a client-side `location.replace(next)`. That navigation is same-site, so the Strict `portal_session` / `portal_access` cookies are sent. All cookies stay `SameSite=Strict` (security.md:509 unchanged). Email templates render `<origin>/open?next=${encodeURIComponent(data.href)}`.' AC: 'Given a logged-in user who clicks an email link to `/library/media/{id}` from a cross-site webmail page, then they land on that page with its server-rendered data and no login prompt. Given `next=//evil.example`, then the trampoline refuses and goes to `/`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.3 "Email links" /open?next= trampoline (outside matcher, same-origin check, location.replace; reset link exempt) + AC. Code follow-up: no /open route; email.go prints bare href.

### SPEC-05-journal.md

#### - [x] F145 · 🟡 MINOR · `SPEC-05-journal.md` · P0.2 / §6 / §7 / §9 / header (P1.5 after supersession)
*Category:* stale-reference  
**Problem:** P1.5 is superseded by SPEC-12, yet other sections still gate behaviour on 'P1.5 landing' and plan a P1 attachments phase.  
**Evidence:** SPEC-05:162 'superseded 2026-09-12 by SPEC-12'; :82-84 'until P1.5 lands'; :192 '(P1.5)'; :214; :226-227; :242 '4. P1 (attachments + mood picker)'.  
**Fix:** Replace each P1.5 gate with 'SPEC-12' (e.g. 'rejected with 422 `journal/invalid-asset` until SPEC-12 lands'). §9 step 4 → '4. P1.6 mood picker (0.5 day); attachments moved to SPEC-12'. Note in P0.2/§6 that SPEC-12 relaxes the body CHECK to 0–20,000 for photo-only entries.  
**Applied:** ✅ P1.5 gates replaced with SPEC-12 in P0.2, §6 DDL comment + asset_ids note, §7 POST row, invalid-asset note; SPEC-12 body CHECK ≤20000 noted; §9 step 4.

#### - [c] F146 · 🟡 MINOR · `SPEC-05-journal.md` · §3 / P0.4
*Category:* brief-coverage  
**Problem:** The brief and §3 both fix v1 as 'markdown-in-textarea with preview', but no requirement or AC builds the preview.  
**Evidence:** briefs/05:31 and SPEC-05:41-42 'with preview'; P0.4 (:130-158) has no preview requirement.  
**Fix:** P0.4: 'The composer has a Write/Preview toggle; Preview renders the draft through the card's sanitizing renderer.' AC: '`**bold**` renders bold in Preview; `<script>` renders inert.' Alternatively, drop 'with preview' and defer it.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.4 Write/Preview toggle via the card's sanitizing renderer + ACs. Code follow-up: Composer preview is raw text and StreamItemCard renders no markdown.

#### - [x] F147 · 🟡 MINOR · `SPEC-05-journal.md` · P0.2 / §6 CHECK
*Category:* ambiguity  
**Problem:** '1–20 000 chars' does not name the counting unit. Postgres counts code points, Go `len` counts bytes, and JS `.length` counts UTF-16 units, so the service and the CHECK can disagree (a 500, or wrongly rejected Vietnamese text).  
**Evidence:** SPEC-05:76; :190 `char_length(body_md) BETWEEN 1 AND 20000`; :98.  
**Fix (revised by verify):** P0.2: 'Lengths (body_md, mood) are Unicode code points, as char_length counts them; a frontend counter uses [...str].length.'  
**Applied:** ✅ P0.2 "Length unit": Unicode code points (char_length / utf8.RuneCountInString / [...str].length).

#### - [x] F148 · 🟡 MINOR · `SPEC-05-journal.md` · §9
*Category:* wrong-number  
**Problem:** The P0 estimate does not add up. The steps total 4–4.5 days, not '4–5', and with P1 the total is 5–5.5, not 'the brief's 5–6'.  
**Evidence:** SPEC-05:239-243 '1 + 1.5 + 1.5–2 ... P0 ≈ 4–5 dev-days; matches the brief's 5–6 including P1.'  
**Fix:** 'P0 ≈ 4–4.5 dev-days; 5–5.5 including P1 (brief estimated 5–6).' Recompute if P1 becomes the mood picker only.  
**Applied:** ✅ §9 estimates recomputed: P0 ≈ 4–4.5 dev-days, 4.5–5 with P1.6 (attachments moved to SPEC-12).

#### - [x] F149 · 🟡 MINOR · `SPEC-05-journal.md` · §1 Problem statement
*Category:* sequencing  
**Problem:** §1 says SPEC-01/02/03 are three producers whose only consumer is SPEC-04's bell. Only SPEC-01 ships before SPEC-05, and SPEC-04 consumes only `media:asset_ready`. Brief 05 repeats the claim.  
**Evidence:** SPEC-05:13-14; briefs/05:12-13; specs/README.md:93-98; SPEC-04:6.  
**Fix:** 'SPEC-01 (and later SPEC-02/03) create event producers; the only planned consumer ahead of this spec is SPEC-04's bell (for `media:asset_ready`); the timeline ADR-08 calls the product has no write path.' Apply the same to briefs/05:12-13.  
**Applied:** ✅ §1: SPEC-01 (later 02/03) are producers; the only planned consumer ahead of this spec is SPEC-04's bell.

### SPEC-06-life-stream-home.md

#### - [x] F150 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · P0.1(b) bank:transaction_updated
*Category:* false guarantee  
**Problem:** The `transaction_updated` upsert has no version guard, so two edits processed out of order leave a stale amount permanently, contrary to the stated guarantee. An edit that flips `is_transfer` re-keys the item and orphans the old row.  
**Evidence:** SPEC-06:91 '`ON CONFLICT ... DO UPDATE SET payload, occurred_at` ... the corrected payload wins (a corrected amount must not render wrong forever)'; :90 ref key switches on is_transfer. The SPEC-03 payload has no updated_at. The residuals (:102-109) list neither case.  
**Fix (revised by verify):** Choose the residual (no cross-spec payload change at v1). Soften SPEC-06:91 to '… so a reordered created retry then hits `DO NOTHING` and the latest-processed correction wins'. Append to the residuals paragraph at :102-109: 'Third accepted residual: two `bank:transaction_updated` events processed out of order leave the earlier edit\'s payload (last-processed-wins; the payload carries no version). This self-corrects on the next edit or the P2 reconcile sweep. If it ever bites, add `updated_at` to the SPEC-03 P0.7 payload and guard the upsert with `WHERE (stream_items.payload->>\'updated_at\')::timestamptz <= (EXCLUDED.payload->>\'updated_at\')::timestamptz`.' Do not add an is_transfer-flip residual: legs are immutable per SPEC-03 P0.3.  
**Applied:** ✅ P0.1(b) bank_updated "latest-processed correction wins"; out-of-order residual + updated_at guard seam; legs immutable.

#### - [c] F151 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · P0.1(b), §6
*Category:* ambiguity  
**Problem:** `stream_items.user_id` is NOT NULL, but the spec never says which payload field fills it. The payloads use `owner_user_id` (media, comic) or `user_id` (playback, bank, people).  
**Evidence:** SPEC-06:243 `user_id uuid NOT NULL`; :96-100; events.md:20-34 payloads.  
**Fix:** Add a 'user_id from' column to the P0.1(b) table (`owner_user_id` for media/comic, `user_id` for playback/bank/people): 'A payload missing its user field is skipped with a log line and `asynq.SkipRetry`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.1(b) "user_id from" column; malformed payload dropped with a log line, never retried. Code follow-up: handlers return nil silently.

#### - [c] F152 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · P0.1(b) occurred_at fallback; §8
*Category:* ordering  
**Problem:** `occurred_at` falls back to ingest time for events without a timestamp, so an event delivered late after worker downtime sorts at recovery time. The §8 durability metric counts that as success.  
**Evidence:** SPEC-06:98-100 'else ingest time'; :275-277; SPEC-07:167-168 payload has no timestamp although `completed_at` exists (:194).  
**Fix (revised by verify):** SPEC-07 P1.5: payload becomes `{asset_id, user_id, title, completed_at}` (the latched value); update events.md to match. SPEC-06 P0.1(b): 'occurred_at for `media:playback_completed` = payload `completed_at`.' For `people:birthday_upcoming` keep the ingest-time fallback and add it to the accepted residuals: 'a birthday notice delivered late (SPEC-08 outbox retry on the next scan) sorts at delivery time; it is a heads-up, not a timed fact.' §8 lagging metric: '… appears after worker recovery at its original `occurred_at` position (events whose payload carries a timestamp).'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.1 playback_completed occurred_at = payload completed_at; birthday late-delivery residual; §8 metric; AC. Code follow-up: HEAD uses time.Now(); payload lacks completed_at.

#### - [x] F153 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · P0.1(b), §7
*Category:* missing names / DoD  
**Problem:** The spec requires the stream to register its own consumer task types, and makes registering them in events.md part of the DoD, but never names them.  
**Evidence:** SPEC-06:81-83 'registers its own consumer task — never the raw event name'; :266-268 DoD; names exist only after the fact in events.md:26-27,58.  
**Fix (revised by verify):** Add a 'Consumer task' column to the P0.1(b) table with one task per row: `journal:stream_asset_ready`, `journal:stream_playback_completed`, `journal:stream_asset_deleted`, `journal:stream_bank_created`, `journal:stream_bank_updated`, `journal:stream_bank_deleted`, `journal:stream_chapter_published`, `journal:stream_birthday`. Add after the table: 'The cmd/worker subscription table maps each event to exactly its task. events.md\'s `journal:stream_ingest` example is illustrative only and is replaced by these names.' If the asset_ready and comic rows are later dropped from the table, drop their tasks with them.  
**Applied:** ✅ P0.1(b) Consumer task column (HEAD names); planned journal:stream_birthday_revoked / journal:stream_person_deleted; cmd/worker subscription note.

#### - [x] F154 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · §6
*Category:* data model vs prose  
**Problem:** The §6 migration omits the `stream:read:own` seed and the journal backfill that the prose puts in it. Its payload comment also says the payload holds `href`, which P0.2 rules out.  
**Evidence:** SPEC-06:136-137 'seeded + granted to the base `user` role in this module's migration'; :111-113 'the migration itself seeds rows for all existing `journal_entries`'; DDL :240-254 has neither. :249 comment '(title, href, amount?)' vs :98, :147-148 'none carries `href`'.  
**Fix (revised by verify):** Change the :249 comment to: `-- raw registered event payload; title/href synthesized at read time (P0.2); '{}' for journal rows`. Optionally add one line under the DDL: 'The same migration also ships the `stream:read:own` → `user` seed (0003 `WITH grants(...)` pattern) and the P0.1 journal backfill `INSERT … SELECT … ON CONFLICT DO NOTHING`.'  
**Applied:** ✅ §6 payload comment (raw payload, title/href synthesised at read, '{}' for journal); same migration ships stream:read:own seed + journal backfill; TC-STREAM-112.

#### - [x] F155 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · P0.1, P0.4, P1.5
*Category:* acceptance-criteria coverage  
**Problem:** Several P0 behaviours have no AC: the `transaction_updated` correction, `transaction_deleted` removal, the 404 'coming soon' rule, and widget behaviour on 403. P1.5 has no AC.  
**Evidence:** SPEC-06:91-92 (no AC at :119-132); :190-191 (no AC; :202-205 cover only 500); :209-213 P1.5.  
**Fix (revised by verify):** Add to the P0.1 ACs: 'Given a `bank:transaction_updated` with a new amount (even if delivered before its created event), then the single stream item shows the new amount.' and 'Given a `bank:transaction_deleted` (or a transfer delete emitting both legs), then the item is gone.'  
**Applied:** ✅ P0.1 ACs update-before-create and transaction_deleted/transfer; P0.4 404/403 ACs; P1.5 acceptance; TC-STREAM-018.

#### - [c] F156 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · §6 UNIQUE constraint
*Category:* data model latent collision  
**Problem:** The unique key has no `user_id`, and `media:playback_completed` is keyed on the shared `asset_id`. Once assets are shared (household tenancy), a second user's completion is silently dropped by ON CONFLICT DO NOTHING.  
**Evidence:** SPEC-06:251 `UNIQUE (source_module, event_type, ref_id)`; :88; :233-234 'keep `user_id` scoping clean'; SPEC-07:196 `PRIMARY KEY (user_id, asset_id)`.  
**Fix:** §6 → `UNIQUE (user_id, source_module, event_type, ref_id)`, and use the same ON CONFLICT target throughout P0.1. The asset_deleted row: 'delete ALL `source_module='media'` rows with this ref_id (all users — the asset is gone for everyone)'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §6 UNIQUE (user_id, source_module, event_type, ref_id); every ON CONFLICT uses it; asset_deleted "all users"; AC. Code follow-up: journal_stream.sql + migration omit user_id.

#### - [x] F157 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · §9
*Category:* number  
**Problem:** The timeline totals 6.5 dev-days but claims to 'match the brief's ~6'.  
**Evidence:** SPEC-06:281-286 (5 P0 + ~1.5 P1); briefs/06:5 '~6 days'.  
**Fix:** 'P0 ≈ 5 dev-days; P1 adds ~1.5 (≈ 6.5 total, +0.5 over the brief).' If P1.6 is retired, use 'P1 adds ~1 (≈ 6 total)'.  
**Applied:** ✅ Already resolved by cross-spec pass: §9 "P0 ≈ 5; P1 adds ~1" (P1.6 retired).

#### - [x] F158 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · §7 Problem types
*Category:* problem-type naming  
**Problem:** The Problem prefix is `stream/`, but the owning module is `journal`. Every other spec uses `<module>/`.  
**Evidence:** SPEC-06:4 'Module: `journal`'; :266 '`stream/invalid-cursor`'; SPEC-05:224-227 uses `journal/`.  
**Fix (revised by verify):** SPEC-06 §7: 'Problem types: `journal/invalid-cursor` (shared with SPEC-05\'s cursor-paginated `GET /journal/entries`; whichever spec ships first registers the i18n key), `journal/invalid-limit`.' Also add `journal/invalid-cursor` to SPEC-05 §7\'s Problem-type list, since its list endpoint takes `?cursor=` too.  
**Applied:** ✅ Already resolved: §7 journal/invalid-cursor (shared with SPEC-05); no journal/invalid-limit (lenient limit per HEAD); TC-STREAM-037 corrected.

#### - [c] F159 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · P0.4 Widget rail
*Category:* tanstack-defaults  
**Problem:** '404 → empty state' conflicts with TanStack's default retry (3 with backoff): an unmounted module spins for seconds before showing empty. 401 must stay retryable because focus refetches race SessionKeeper.  
**Evidence:** SPEC-06:188-191; providers.tsx:8 `new QueryClient()` (no defaults); SessionKeeper.tsx:49-53 refreshes on focus/visibilitychange.  
**Fix (revised by verify):** Append to P0.4: 'Each widget query sets `retry: false`: a 4xx is final, so a 404/403 renders the empty or "coming soon" state immediately, and a 5xx renders an inline retry affordance.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.4 widget queries retry:false; 4xx final, 5xx inline retry; AC. Code follow-up: rail queries use QueryClient defaults.

#### - [c] F160 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · P0.4 Activity feed widget
*Category:* d32-cache-ownership  
**Problem:** The Activity-feed widget and the SPEC-04 bell read the same `/me/notifications` data under different query keys. The bell's optimistic mark-read never reaches the rail, and the server state is held twice.  
**Evidence:** SPEC-06:196; SPEC-04:143 '`useQuery(["notifications"])` owns items + unread_count'. HEAD ActivityFeed.tsx uses `["notifications","rail"]`.  
**Fix:** 'The Activity feed reuses SPEC-04 P0.5's `["notifications"]` query (same key and queryFn, rendering a slice) and never defines its own key (D-32).'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.4 Activity feed reuses SPEC-04 P0.5 ["notifications"] query (D-32); AC. Code follow-up: ActivityFeed.tsx uses ["notifications","rail"].

### SPEC-07-continue-rail.md

#### - [c] F161 · 🟡 MINOR · `SPEC-07-continue-rail.md` · P0.1/P0.3 predicate / P0.4 / P1.5
*Category:* ambiguity  
**Problem:** The spec does not say whether the 95% thresholds use the rounded percentage. A rounded 94.6% becomes 95%, so the item leaves /continue before P1.5 fires. The 30 s boundary is worded both as ≥ and as 'above', and `duration_ms = 0` would divide by zero.  
**Evidence:** SPEC-07:85-86, :154 'a rounded percentage'; :130 '`position_ms ≥ 30 000` and `progress_pct < 95`'; :155-156 'above the 30 s threshold'; :169 '≥95%'.  
**Fix:** P0.1: 'The completion ratio is `floor(position_ms * 100 / duration_ms)` (integer). The /continue predicate, the resume gate and the P1.5 latch all use this value; `duration_ms <= 0` is treated as NULL.' :155 → 'at or above 30 s (`position_ms ≥ 30 000`) and below 95%'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.1 binding completion-ratio rule floor(position_ms*100/duration_ms), duration ≤ 0 → NULL; P0.3/P0.4/P1.5 cite it. Code follow-up: SQL ::int rounds, Go uses float comparisons.

#### - [x] F162 · 🟡 MINOR · `SPEC-07-continue-rail.md` · P0.3 limit
*Category:* ambiguity  
**Problem:** Behaviour for out-of-range or malformed `?limit=` values is undefined, yet the test cases already assume clamping.  
**Evidence:** SPEC-07:123 '(`?limit=`, default 10, max 50)'; TC-CONT-044 assumes '`?limit=100` ... clamped to max 50'.  
**Fix (revised by verify):** P0.3: '`?limit=` integer, default 10; values > 50 are clamped to 50; a missing, non-integer or < 1 value falls back to the default 10 (never an error). Each module's `Continue` is called with `limit`; the aggregator merges the lists, sorts by `updated_at DESC`, and truncates to `limit`.' Add AC: 'Given `?limit=100`, then at most 50 items; given `?limit=abc` or `?limit=0`, then 200 with the default 10.'  
**Applied:** ✅ P0.3 limit rule (clamp >50; missing/invalid/<1 → 10; merge, sort, truncate) + AC; §7 row; TC-CONT-044. HEAD already behaves this way.

#### - [x] F163 · 🟡 MINOR · `SPEC-07-continue-rail.md` · P0.2 / P0.3 / P0.4 ACs
*Category:* acceptance-criteria  
**Problem:** Several P0 behaviours have no AC: GET progress (owner-scoped 404, null progress_pct), /continue sort and limit, and click-through to the new player route at the saved position.  
**Evidence:** SPEC-07:81-86 (ACs :101-107 cover PUT only); :123 (no AC at :134-141); :42-43, :145-150 (no AC at :160-163).  
**Fix:** Add: 'another user's asset → GET progress 404'; 'duration_ms NULL with a saved row → progress_pct null and position_ms returned'; '12 in-progress items, no limit → 10 sorted by updated_at DESC'; 'opening a /continue item's href mounts /library/media/{id} at the saved position'.  
**Applied:** ✅ ACs: GET cross-owner 404, NULL-duration progress_pct null, 12 items → 10 sorted DESC, /continue href click-through; TC-CONT-065, TC-CONT-027/044.

#### - [x] F164 · 🟡 MINOR · `SPEC-07-continue-rail.md` · P0.2 AC vs P0.4
*Category:* contradiction  
**Problem:** The P0.2 AC says the player 'offers resume', which implies a prompt. P0.4 auto-starts at the saved position with a 'Start over' control.  
**Evidence:** SPEC-07:102-103 'the player offers resume at ~12:34' vs :155-158, :161.  
**Fix:** '...when reopened, the player starts at ~12:34 (±10 s) with a visible "Start over" control.'  
**Applied:** ✅ P0.2 + P0.3 ACs "starts at ~12:34 (±10 s) with a visible Start over control"; TC-CONT-020.

#### - [c] F165 · 🟡 MINOR · `SPEC-07-continue-rail.md` · P1.5 completion latch
*Category:* ambiguity  
**Problem:** The spec does not say whether the latch is set before or after Publish. If the latch commits and Publish then fails, the event is lost for good.  
**Evidence:** SPEC-07:169-171 'latched via a nullable `completed_at` ... emit only on the NULL→set transition'.  
**Fix (revised by verify):** P1.5: 'In one transaction: (1) upsert the position (`INSERT ... ON CONFLICT (user_id, asset_id) DO UPDATE SET position_ms = EXCLUDED.position_ms, updated_at = now()`); (2) if the floor ratio (P0.1) is ≥ 95, run `UPDATE media_playback_progress SET completed_at = now() WHERE user_id = $1 AND asset_id = $2 AND completed_at IS NULL RETURNING 1`. The row lock serialises concurrent beacons, so only one gets a row back. (3) Only when step 2 returned a row, call `events.Publish` before COMMIT; if Publish returns an error, roll back (the position save is lost too, and the next beacon retries both). Because Publish enqueues per subscriber and is not atomic, a retry or a crash between Publish and COMMIT may deliver the event twice; consumers must dedupe on (`asset_id`, `user_id`), which the payload already carries. Never discard the Publish error.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P1.5 single-transaction upsert → UPDATE … completed_at IS NULL RETURNING 1 → Publish before COMMIT, rollback on error, consumers dedupe on (asset_id,user_id); TC-CONT-080. Code follow-up: PutProgress publishes outside a tx, drops the error, read-then-upsert race.

#### - [x] F166 · 🟡 MINOR · `SPEC-07-continue-rail.md` · P2 Watched-history
*Category:* incorrect-claim  
**Problem:** The claim that `completed_at` and `updated_at` 'already hold the data' for watch history is only partly true. completed_at records only the first completion, and updated_at is overwritten by every beacon, so rewatches are lost.  
**Evidence:** SPEC-07:181-182; :194 'set once at first ≥95% crossing'; :170-171.  
**Fix:** 'The table holds only the first completion per asset, enough for a "watched once" list. Per-watch history needs an append-only `media_playback_completions` table later.'  
**Applied:** ✅ P2 Watched-history: table holds first completion only; per-watch history needs append-only media_playback_completions.

#### - [x] F167 · 🟡 MINOR · `SPEC-07-continue-rail.md` · §9 Timeline / P0.4
*Category:* omission  
**Problem:** The timeline omits work P0 requires: the GET progress endpoint and the new `/library/media/[id]` route with an unnamed TemplateManifest view key.  
**Evidence:** SPEC-07:145-148 'this route does not exist yet and is created as part of this spec'; :224-228 timeline steps; frontend.md:85 has no media player view.  
**Fix (revised by verify):** P0.4: 'The route resolves `TemplateManifest.views.libraryMediaDetail` (`ComponentType<{ id: string }>`, v1 → `views/library/media/MediaDetailView.tsx`), which mounts Vidstack. Add the key to `templates/types.ts` and the v1 manifest, and add it to frontend.md §2's `views` list (currently it lists only home/login/register/libraryComic/libraryNovelDetail).' §9 step 2: 'PUT + GET progress + `/library/media/[id]` route + `libraryMediaDetail` view + Vidstack throttle/pagehide + resume UX (2 days)'. Totals line: 'P0 ≈ 3.5 dev-days; P1 adds ½.'  
**Applied:** ✅ P0.4 names TemplateManifest.views.libraryMediaDetail (ships on HEAD); §9 step 2 re-estimated (P0 ≈ 3.5, P1 ~1); TC-CONT-064.

### SPEC-08-people-registry.md

#### - [c] F168 · 🟡 MINOR · `SPEC-08-people-registry.md` · §7 API summary
*Category:* api-contract  
**Problem:** §7 defines only request bodies. There is no Person response shape, no limit or next_cursor on the paged list, and no field for P1.6's 'last contact'.  
**Evidence:** SPEC-08:280 '`/api/v1/people?cursor=`'; :110; :203-204 'last contact ... shown on the person card'.  
**Fix (revised by verify):** §7 add below the table: 'Person response: `{id, display_name, relationship|null, birthday: {month, day, year?, calendar, leap_month?} | null, contact, note_md|null, avatar_asset_id|null (P1.7), last_contact_on: date|null (P1.6; max occurred_on), created_at, updated_at}`.' Change row :280 to '`GET /api/v1/people?cursor=&limit=` | `people:read:own` | ordered (display_name, id); limit default 50, max 200 (clamped); `{items: Person[], next_cursor: string|null}`; malformed cursor → 400 `people/invalid-cursor`'. :287-288 → 'Problem types: `people/person-not-found`, `people/invalid-birthday`, `people/invalid-cursor`, `people/invalid-asset` (P1.7).' (Register each as an i18n key per README.)  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §7 Person response shape (HEAD personJSON + 0035 columns; leap_month/last_contact_on not shipped); list limit 50/200, circle filter, invalid-cursor, envelope {people,next_cursor} as a noted deviation; problems.ts line. Code follow-up: no people/* slugs in problems.ts.

#### - [x] F169 · 🟡 MINOR · `SPEC-08-people-registry.md` · P1.6 Interactions log
*Category:* brief-coverage  
**Problem:** The kind 'other' is added beyond the brief without a note, and interactions have no DELETE, so a mis-logged one cannot be removed.  
**Evidence:** briefs/08:69 'met|called|messaged|gifted' vs SPEC-08:203, :264; §7:285 POST/GET only.  
**Fix (revised by verify):** In P1.6 add: '`other` is added beyond the brief as a catch-all (2026-07-10).' Add a §7 row: '| DELETE | `/api/v1/people/{id}/interactions/{interaction_id}` | `people:delete:own` | P1.6; 204; 404 for others' rows |'.  
**Applied:** ✅ P1.6 notes `other` goes beyond the brief; §7 DELETE interactions row (people:delete:own, 204, foreign 404).

#### - [c] F170 · 🟡 MINOR · `SPEC-08-people-registry.md` · P0.3 Upcoming birthdays
*Category:* acceptance-criteria  
**Problem:** The `days` window is ambiguous (0..14 or 0..13, and whether today is included). There are no ACs for sorting, clamping, `age_turning`, or no-birthday people.  
**Evidence:** SPEC-08:114-119; ACs :127-131.  
**Fix (revised by verify):** Replace 'sorted soonest-first' with: 'returns people with `0 ≤ days_until ≤ days` (inclusive; today = 0; people without a birthday are excluded), sorted by `days_until`, then `display_name`, then `id`.' Add ACs: 'Given a birthday today, then it is returned with days_until 0'; 'Given birth_year 1966 queried in 2026, then age_turning is 60; with no year, age_turning is absent'; 'Given days=0 or days=500, then the value is clamped to 1 or 366'.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.3 inclusive 0 ≤ days_until ≤ days, no-birthday excluded, sort days_until → display_name → id; days default 14 (<1/absent/invalid), clamp 366 (follows HEAD, not the worklog's "clamp 0 to 1"); ACs; §7 row. Code follow-up: HEAD sorts on days_until only (unstable ties).

#### - [x] F171 · 🟡 MINOR · `SPEC-08-people-registry.md` · P0.5; P1.6/P1.7
*Category:* acceptance-criteria  
**Problem:** The P0.5 middleware gate, template resolution and 422 handling have no ACs, and P1 has none at all. P1.7's 'optionally POST' leaves the contract undecided.  
**Evidence:** SPEC-08:188-194 vs ACs :196-198; :209-210 '(and optionally `POST`)'.  
**Fix (revised by verify):** Add P0.5 ACs: 'Given a signed-out visitor on /people or /people/{id}, then they are redirected to /login'; 'Given an invalid birthday submitted in the form, then the 422 `people/invalid-birthday` detail renders inline'. Add a P1.6 AC: 'the person card shows the latest `occurred_on`'. Add a P1.7 AC: 'Given `media:asset_deleted` for an avatar, then `avatar_asset_id` is NULL and the initials Avatar renders'. Replace '(and optionally `POST`)' with '(and `POST`, same validation)'.  
**Applied:** ✅ P0.5 ACs (signed-out redirect, inline 422 detail), P1.6 last-contact AC, P1.7 asset_deleted → initials AC; "(and POST, same validation)"; TC-PPL-072/074/090–093.

#### - [x] F172 · 🟡 MINOR · `SPEC-08-people-registry.md` · §8 Success metrics
*Category:* metrics  
**Problem:** The zero-missed and zero-duplicated metric is claimed to be auditable from the notices table, but edits and deletes remove notice rows.  
**Evidence:** SPEC-08:294-296 vs :86-88 and :180-181.  
**Fix (revised by verify):** Replace '(auditable from the notices table)' with '(auditable from the notices table for people whose birthday was not edited and who were not deleted in the window, cross-checked against SPEC-06 `stream_items` where `event_type = ''people:birthday_upcoming''`)'.  
**Applied:** ✅ §8 metric scoped to unedited, undeleted people; cross-checked against SPEC-06 stream_items.

#### - [c] F173 · 🟡 MINOR · `SPEC-08-people-registry.md` · §6 indexes / CHECKs
*Category:* data-model  
**Problem:** The (display_name, id) cursor has no index. `threshold` has no CHECK, the unemitted re-publish scan has no index, and birth_year has no bound.  
**Evidence:** SPEC-08:110; §6 :243 has only `(user_id, birth_month, birth_day)`; :255 `threshold int NOT NULL -- 3 | 0`; :233 `birth_year int` (prose :98 says 1900..current). Shipped 0016 added `(user_id, display_name, id)`.  
**Fix (revised by verify):** In §6 add `CREATE INDEX people_persons_user_idx ON people_persons (user_id, display_name, id);` to match 0016. Change `threshold int NOT NULL` to `threshold int NOT NULL CHECK (threshold IN (0, 3))` and `birth_year int` to `birth_year int CHECK (birth_year IS NULL OR birth_year >= 1900)` (the upper bound stays app-layer). Because 0016 has shipped, put the two CHECKs in a follow-up migration `000N_people_checks`.  
**Applied:** ✅ spec fixed · ⚙ code follow-up — §6 DDL mirrors shipped 0016 indexes + CHECK threshold IN (0,3), birth_year ≥ 1900, pending-notice partial index; follow-up 000N_people_checks migration. Code follow-up: that migration.

#### - [x] F174 · 🟡 MINOR · `SPEC-08-people-registry.md` · P0.5 FriendCard reuse vs §3
*Category:* repo-reality  
**Problem:** `FriendCard` has Friends/Photos/Videos stats and add/message friend controls. Reused as-is, it shows exactly the friend-graph actions §3 rules out.  
**Evidence:** SPEC-08:185 'reusing the `FriendCard`'; :30-32 'Not a friend graph ... no requests'; FriendCard.tsx:4 `Stats`, :70 `<ControlBlockButtons`.  
**Fix (revised by verify):** Replace SPEC-08:185 'reusing the `FriendCard` / `PersonalInfoWidget` kits' with: 'reusing the `FriendCard` layout (cover, Avatar, name) through a `PersonCard` wrapper, or through a new `actions` prop, that renders no stats row and no Add-friend/Message `ControlBlockButtons`; the subtitle shows relationship and next birthday; plus `PersonalInfoWidget` for the detail view'.  
**Applied:** ✅ P0.5: FriendCard layout reuse only via a PersonCard wrapper/actions prop (no stats row/ControlBlockButtons); HEAD uses base Card + initials Avatar; AC; TC-PPL-075.

#### - [x] F175 · 🟡 MINOR · `SPEC-08-people-registry.md` · P0.5 Frontend
*Category:* route-contract  
**Problem:** P0.5 mentions 'the detail route' without giving its path or view keys, and SPEC-06's birthday href depends on it.  
**Evidence:** SPEC-08:192-194 '(and the detail route)'; SPEC-06:152 '/people/id'.  
**Fix (revised by verify):** Replace '(and the detail route) resolve them via `activeTemplate().views.<x>`' with 'and `app/(app)/people/[id]/page.tsx` resolve `activeTemplate().views.peopleList` and `views.peopleDetail` (`ComponentType<{ id: string }>`)'. In SPEC-06:152 change `/people/id` to `/people/{person_id}`.  
**Applied:** ✅ P0.5 names app/(app)/people/page.tsx + people/[id]/page.tsx → views.peopleList / peopleDetail; detail path /people/{person_id} is the SPEC-06 link target; TC-PPL-073.

### SPEC-09-platform-ops.md

#### - [x] F176 · 🟡 MINOR · `SPEC-09-platform-ops.md` · §4 edge story vs P0.5 precedence
*Category:* contradiction  
**Problem:** The edge story expects `stale` while a failure event is in the audit log. Under rule 1, if the last completed run failed the state is `failed`.  
**Evidence:** SPEC-09:56-58 'screaming `stale` ... and the failure event is in the audit log' vs :161-162 '`failed` — the most recent completed run has status='failed''.  
**Fix (revised by verify):** Split the edge story: 'the scheduler silently stopped (no runs) → `stale` from hour 26'; 'last night's run failed → `failed`, with `ops.backup.failed` in the audit log'.  
**Applied:** ✅ §4 edge story split: scheduler stopped → stale from hour 26; last run failed → failed + ops.backup.failed audit.

#### - [x] F177 · 🟡 MINOR · `SPEC-09-platform-ops.md` · P0.5 response shape
*Category:* ambiguity  
**Problem:** The fields of `last_run: {...}` are undefined, as are its value when there are zero runs and the type of hours_since_success.  
**Evidence:** SPEC-09:156 '`{last_success_at, hours_since_success, state, last_run: {...}}`'; :181-182.  
**Fix (revised by verify):** P0.5: 'Returns `{last_success_at: RFC3339|null, hours_since_success: number|null (fractional hours, unrounded), state: "ok"|"stale"|"failed", last_run: {id, status, started_at, finished_at|null, size_bytes|null, storage_key|null, error|null} | null}`. `last_run` is the most recent row by `started_at` (any status), or null when there are zero runs.' Extend the zero-runs AC with '`last_run: null`'.  
**Applied:** ✅ P0.5 response fully typed (last_run null on zero runs, fractional hours); AC; TC-OPS-042/042a. Shipped code matches.

#### - [c] F178 · 🟡 MINOR · `SPEC-09-platform-ops.md` · P0.2 steps 3/6; P0.5
*Category:* security / info-disclosure  
**Problem:** The raw backup `error` string is stored, returned by /ops/status and published in `ops:backup_failed` with no sanitisation. pg_dump errors can leak host, user or DSN fragments.  
**Evidence:** SPEC-09:89, :96-97 '`ops:backup_failed {run_id, error}`'; :156; backup-restore.md shows a DSN containing a password.  
**Fix (revised by verify):** P0.2 step 3: '`error` is a single line truncated to 500 chars; the DSN and command line are never interpolated into it.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.2 step 3 error single line ≤ 500 chars, never DSN/command line; TC-OPS-003b. Code follow-up: backup.go keeps 1000 chars incl. multi-line pg_dump stderr.

#### - [c] F179 · 🟡 MINOR · `SPEC-09-platform-ops.md` · P0.4 step 3 sanity checks
*Category:* acceptance-criteria  
**Problem:** 'Plausible row counts' and 'one known row' cannot be tested, and the migration check ignores golang-migrate's `dirty` flag.  
**Evidence:** SPEC-09:138-141; the runbook substituted a 7-roles check (backup-restore.md:127).  
**Fix (revised by verify):** '(a) `schema_migrations.dirty = false` and `version` ≤ the repo's latest migration; (b) `count(*) FROM users` ≥ 1 and `SELECT count(*) FROM assets` succeeds; (c) the 7 system roles seeded by 0003 are present. Any failure → non-zero exit with `RESTORE DRILL FAILED: <check>`.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P0.4 step 3 checks (dirty=false + version ≤ latest; users ≥ 1 + assets count; 7 system roles) with "RESTORE DRILL FAILED: <check>"; runbook §4; TC-OPS-023. Code follow-up: restore-drill.sh lacks these checks/prefix.

#### - [x] F180 · 🟡 MINOR · `SPEC-09-platform-ops.md` · §6 ops_exports; P1.7 prune
*Category:* data-model  
**Problem:** `ON DELETE CASCADE` on user_id deletes export rows when a user is deleted. The purge is row-driven, so that user's archives (EXIF/GPS originals) stay in storage forever.  
**Evidence:** SPEC-09:244; :210-212 row-driven purge; :206-207.  
**Fix (revised by verify):** §6 `ops_exports.user_id` → `user_id uuid REFERENCES users(id) ON DELETE SET NULL` (nullable; comment: 'NULL = owner deleted; object purged by the next ops:purge_exports'). P1.7 prune: '`ops:purge_exports` deletes from storage (a) archives older than 7 days, setting status `expired`, and (b) immediately, any row whose `user_id IS NULL`, deleting its `storage_key` object and then the row. The purge stays row-driven, no storage listing is needed, and a deleted user's EXIF/GPS originals never outlive one janitor cycle.' Add AC: 'Given a user with a ready export is deleted, after the next purge the archive object is gone.'  
**Applied:** ✅ §6 ops_exports.user_id nullable ON DELETE SET NULL; P1.7 purge of user_id IS NULL rows; AC; TC-OPS-103c/121.

#### - [x] F181 · 🟡 MINOR · `SPEC-09-platform-ops.md` · P0.1 seeding; §4
*Category:* authz  
**Problem:** `ops:read` and `queues:read` go only to `admin`, but the README provisions the owner as 'creator or higher', so a creator-provisioned owner gets 403 on the ops surfaces.  
**Evidence:** SPEC-09:70-71; :52 'As the owner'; README.md:76-77.  
**Fix (revised by verify):** P0.1: 'The ops surfaces require `admin` (or superadmin via `*`). A `creator`-provisioned owner gets 403 by design; the runbook's first step verifies the owner holds `admin`.'  
**Applied:** ✅ P0.1 ops surfaces require admin (creator-provisioned owner 403 by design); runbook §3 verifies admin.

#### - [x] F182 · 🟡 MINOR · `SPEC-09-platform-ops.md` · P0.2 schedule/queue; step 6 audit
*Category:* completeness  
**Problem:** The schedule time and queue are unstated (events.md records both). System audit rows need `actor_kind='system'`, and the new audit types need a MODULES.md §5.3 row.  
**Evidence:** SPEC-09:77; events.md:60 'nightly 03:00 UTC ... "default" queue'; :97-98; 0005 `actor_kind DEFAULT 'user'`; MODULES.md §5.3.  
**Fix (revised by verify):** P0.2 opening: 'nightly at 03:00 UTC (`0 3 * * *`, a cron spec, not `@every`), enqueued on the `default` queue served by the worker's light server (matches events.md). If dump duration makes that queue's weight-1 slot contended, move the task to a dedicated `ops` queue in the same PR that registers it.' Step 6: 'Audit rows use `actor_kind='system'`, `actor_id=NULL`, `target_kind='ops_backup_run'`, `target_id=<run id>`. Register `ops.backup.completed` / `ops.backup.failed` in the D-25 audit-type catalogue (feature-inventory D-25 and the module-map audit-types table).'  
**Applied:** ✅ P0.2 03:00 UTC 0 3 * * * on default server (ops queue if contended); step 6 audit actor_kind='system', target fields, D-25 registration. Shipped code matches.

#### - [c] F183 · 🟡 MINOR · `SPEC-09-platform-ops.md` · P0.2 step 1; P0.4; §6 ops_backup_runs
*Category:* naming / data-model  
**Problem:** The prose calls the table `backup_runs`, but the DDL creates `ops_backup_runs`. The DDL also accepts states the prose rules out: 'ok' without storage_key or size, 'failed' without error, 'running' with finished_at.  
**Evidence:** SPEC-09:83, :124 '`backup_runs`' vs :229 `CREATE TABLE ops_backup_runs`; :89; :233-237 only a status CHECK.  
**Fix (revised by verify):** Replace `backup_runs` with `ops_backup_runs` at :83 and :124. Add to §6: 'Follow-up migration `000N_ops_backup_runs_state_chk` (0012 is consumed): `ALTER TABLE ops_backup_runs ADD CONSTRAINT ops_backup_runs_state_chk CHECK ((status='running' AND finished_at IS NULL) OR (status='ok' AND finished_at IS NOT NULL AND size_bytes IS NOT NULL AND storage_key IS NOT NULL) OR (status='failed' AND finished_at IS NOT NULL AND error IS NOT NULL)) NOT VALID; ALTER TABLE ops_backup_runs VALIDATE CONSTRAINT ops_backup_runs_state_chk;`. Retention never modifies rows (see P0.2 step 5), so ok rows keep their storage_key.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — backup_runs → ops_backup_runs (P0.2/P0.4/TCs); §6 000N_ops_backup_runs_state_chk NOT VALID + VALIDATE; retention never modifies rows; TC-OPS-121a. Code follow-up: the constraint migration.

#### - [c] F184 · 🟡 MINOR · `SPEC-09-platform-ops.md` · P1.6 / §4
*Category:* route-contract  
**Problem:** The user sees the console at `/admin/queues`, but it is mounted in cmd/api on the API origin. On the app origin `/admin/*` belongs to Next, so that URL returns a Next 404.  
**Evidence:** SPEC-09:52; :187-188 'inside `cmd/api`'; the Next app has `app/(app)/admin/{users,roles}`; api-client.ts:15-23 (the API is a separate host).  
**Fix (revised by verify):** P1.6: 'Served at `https://api.<domain>/admin/queues` (the API origin, not a Next route). The Next admin nav links to that absolute URL; no Next page is added.'  
**Applied:** ✅ spec fixed · ⚙ code follow-up — P1.6 served at https://api.<domain>/admin/queues (API origin), Next admin nav links there; §4 story; TC-OPS-080. Code follow-up: no nav link on HEAD.

---

## Open decisions left by the post-fix review

A consistency review of the finished diff fixed ten small defects in place (missing code-follow-up flags on SPEC-02/03/09, a stale backfill mention in the specs README, `completed_at` in TC-CONT-080, `P0.1(b)` references in events.md, the comic-leg `href` in SPEC-07). These need an owner decision, not an edit:

- **Pagination envelopes.** The README rule grandfathers existing endpoints' *limits*, not their envelopes. SPEC-08 keeps `{people}` / `{upcoming}` as a deliberate exception; SPEC-01/02/03 call `{assets}` / `{comics}` / `{transactions}` a pending retrofit to `{items}`. Pick one policy in the README and align the four specs.
- **Timezone source.** SPEC-03 (dashboard/budget month) cites `users.timezone` (D-17); SPEC-06 P1.5 and SPEC-08 P0.3 use `APP_TIMEZONE` at v1 because `users.timezone` has no write path; SPEC-05 P0.4 claims to match the stream's zone. Choose one v1 source and state it once in the README.
- **Media deep link for audio.** SPEC-07 P0.4 sends video *and audio* to `/library/media/{id}`, but the same spec says that player 404s any non-video asset. Either drop audio from the rule or widen the player.
- **Traceability matrix regrade.** `docs/reference/TRACEABILITY-MATRIX.md` has no rows for the new SPEC-01 P1.3, SPEC-02 P1.10, SPEC-07 P1.6, SPEC-09 P0.6; still counts the retired SPEC-06 P1.6; and marks ✅ requirements this pass tightened and HEAD now fails (e.g. SPEC-07 P0.2 cites `TestGetProgressNoRow`, which asserts the old 404). Regrade it together with the code follow-ups.

---

## Code follow-ups (not done here)

This pass edited documentation only. Each line below is a place where the corrected spec and the shipped code on `main` @ `99b5a0b` disagree, or where the fix needs a file outside `docs/` (`backend/MODULES.md`, `shared/openapi.yaml`, `frontend/src/lib/problems.ts`). Turn them into tickets before the next build pass; the spec text is the target.

- **F001** — Code follow-up: MediaDetailView.tsx and lib/comic.ts still call navigator.sendBeacon first.
- **F002** — Deferred: backend/MODULES.md §6 rule (outside docs/).
- **F006** — Code follow-up: dispatchIntent returns early on dedup conflict and sets no TaskID.
- **F009** — Code follow-up: the grant migration + RequirePermission enforcement.
- **F012** — Code follow-up: notify OnAssetReady skips only origin='import'.
- **F016** — Code follow-up: no currency in payload, no bankapi.Names, stream.go formats all as VND.
- **F017** — Code follow-up: neither event emitted on HEAD.
- **F018** — Code follow-up: HEAD builds three different hrefs and the grid does not read ?open.
- **F023** — Deferred: folding inline 2026-07-10 notes in SPEC-05..09 (owned by per-spec findings).
- **F024** — Code follow-up: media/notify answer bad cursor with about:blank; problems.ts lacks journal/invalid-cursor.
- **F028** — Code follow-up: cmd/api + cmd/worker build people.Deps without Timezone (scan runs in UTC).
- **F029** — Code follow-up: both components still ship fixture prop defaults.
- **F031** — Code follow-up: openapi.yaml Problem.type → uri-reference; add journal/invalid-cursor + media/asset-not-playable to problems.ts.
- **F032** — Code follow-up: add /admin, /calendar, /weather to the middleware matcher.
- **F035** — Code follow-up: OriginalContent blocks only uploading/deleting.
- **F036** — Code follow-up: handler returns 500 about:blank on purge failure.
- **F037** — Code follow-up: MarkAssetReady/MarkAssetImageReady lack status='processing' predicate; workers don't clean up.
- **F038** — Code follow-up: Ingest has no origin param (always 'upload').
- **F039** — Code follow-up: completeImage discards the sniffed type.
- **F042** — Code follow-up: COMIC_SCRAPER_URL unset should 404 comic/sync-disabled (shipped: CRUD served, trigger 500).
- **F043** — Code follow-up: constant-time compare; stop routing /api/v1/internal/* publicly; cmd/api/scraper.go sends no X-Internal-Secret; openapi 401 vs handler 404.
- **F048** — Code follow-up: comics_owner_cursor_idx target (0015 shipped only owner_user_id).
- **F049** — Code follow-up: GetComic/ReaderPages/chapter_count include empty chapters.
- **F050** — Code follow-up: RunImport ingests as the caller, so an editor's import is rejected by CreatePages.
- **F051** — Code follow-up: main.go wires only RequireOwnerOrPermission.
- **F056** — Code follow-up: category PATCH ignores kind instead of rejecting it.
- **F057** — Code follow-up: monthParam uses UTC month, malformed → about:blank.
- **F059** — Code follow-up: reset marks only the presented token, non-transactional; purge never scheduled.
- **F060** — Code follow-up: password_reset.go puts data.reset_url in dispatch/email payloads.
- **F062** — Code follow-up: no forgot/reset pages.
- **F063** — Code follow-up: items carry no cursor; watermarkCursor format rejected by backend; openapi item cursor.
- **F064** — Code follow-up: no RetryDelayFunc.
- **F065** — Code follow-up: one shared counter, no Retry-After, slugs missing from problems.ts.
- **F066** — Code follow-up: resolveChannels ignores unknown values.
- **F069** — Code follow-up: journal/invalid-cursor missing from problems.ts.
- **F073** — Code follow-up: home Composer has no occurred_at control.
- **F074** — Code follow-up: bankRef parses as UTC midnight; upsert overwrites occurred_at.
- **F075** — Code follow-up: limit > 50 resets to 30; /stream/memories unshipped.
- **F076** — Code follow-up: HEAD returns 404 on no row; openapi PlaybackProgress must relax required/nullable.
- **F077** — Code follow-up: ErrForbidden→403, deleting→not-playable, no 409 status check, media/asset-not-playable missing from problems.ts.
- **F078** — Code follow-up: GetContinueItems still falls back to a.source_key.
- **F079** — Code follow-up: HEAD type is mediaapi.ContinueItem and poster_url is never null.
- **F080** — Code follow-up: ScanBirthdays fires both thresholds at days_until=0.
- **F081** — Code follow-up: UpdatePerson resets on every PATCH carrying a birthday, outside the tx.
- **F082** — Code follow-up: retry publishes days_until = threshold; columns absent.
- **F083** — Code follow-up: validateBirthday applies Gregorian checks to lunar; no leap_month.
- **F084** — Code follow-up: restore-drill.sh hard-codes http://minio:9000.
- **F085** — Code follow-up: retention.go anchors weeks on Sunday dumps only.
- **F086** — Code follow-up: scheduler sets only Queue("default"); no reaper.
- **F087** — Code follow-up: single writable handler under queues:read, no CSRF check.
- **F091** — Code follow-up: no notify:on_backup_failed consumer/type on HEAD.
- **F095** — Code follow-up: comic/import.go enqueues on default with 12 h timeout.
- **F112** — Code follow-up: handler/OpenAPI return {assets} and media/bad_request.
- **F115** — Code follow-up: needs a 000N_media_uploading_idx migration.
- **F119** — Code follow-up: owner extractor resolves drafts for everyone (draft → 403). TC-COMIC-036/102/140 now stale.
- **F121** — Code follow-up: no cover index shipped.
- **F122** — Code follow-up: service never checks archived on write.
- **F123** — Code follow-up: UpdateTransaction does not check currency.
- **F128** — Code follow-up: UTC default, about:blank on malformed.
- **F131** — Code follow-up: integrity migration not shipped.
- **F136** — Code follow-up: dispatchIntent checks only user_id/type.
- **F137** — Code follow-up: 0009 lacks the user_id index.
- **F141** — Code follow-up: shipped 0009 index is (user_id) only.
- **F142** — Code follow-up: 0010 lacks the CHECK.
- **F143** — Code follow-up: NotificationsMenu uses plain useQuery.
- **F144** — Code follow-up: no /open route; email.go prints bare href.
- **F146** — Code follow-up: Composer preview is raw text and StreamItemCard renders no markdown.
- **F151** — Code follow-up: handlers return nil silently.
- **F152** — Code follow-up: HEAD uses time.Now(); payload lacks completed_at.
- **F156** — Code follow-up: journal_stream.sql + migration omit user_id.
- **F159** — Code follow-up: rail queries use QueryClient defaults.
- **F160** — Code follow-up: ActivityFeed.tsx uses ["notifications","rail"].
- **F161** — Code follow-up: SQL ::int rounds, Go uses float comparisons.
- **F165** — Code follow-up: PutProgress publishes outside a tx, drops the error, read-then-upsert race.
- **F168** — Code follow-up: no people/* slugs in problems.ts.
- **F170** — Code follow-up: HEAD sorts on days_until only (unstable ties).
- **F173** — Code follow-up: that migration.
- **F178** — Code follow-up: backup.go keeps 1000 chars incl. multi-line pg_dump stderr.
- **F179** — Code follow-up: restore-drill.sh lacks these checks/prefix.
- **F183** — Code follow-up: the constraint migration.
- **F184** — Code follow-up: no nav link on HEAD.

---

## Refuted (do not apply)
- `SPEC-01-media-image-pipeline.md` — The brief's open question (ffmpeg vs a Go imaging library) is silently resolved to ffmpeg. The README i18n DoD is not carried for the four Problem types.
- `SPEC-09-platform-ops.md` — The audit-trail export is assigned to `accountapi`, but `audit_log` is platform-owned (D-25), which breaks the providers-only rule.
- `SPEC-05-journal.md` — The DB accepts arrays with more than 10 elements or with NULL elements, although the spec (and SPEC-12) cap them at ten.
- `SPEC-06-life-stream-home.md` — The asset-deleted handler deletes by (source_module, ref_id) across event types. The only index has event_type in the middle, so this is a sequential scan on a never-purged table.
- `SPEC-08-people-registry.md` — The first P0.4 AC says a scan retry 'emits nothing extra', but the outbox is explicitly at-least-once, so the AC conflicts with the third AC.
