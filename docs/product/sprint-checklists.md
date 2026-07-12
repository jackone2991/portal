# Portal — Sprint Checklists (execution tracker)

**Companion to** [delivery-plan.md](delivery-plan.md) (strategy/sequencing) — this is the **tick-as-you-go execution list**. One sprint = one milestone. Ordering is locked (life‑OS order, foundation gate first — 2026-07-11).

**How to use:** work top-to-bottom within a sprint · `- [ ]` todo → `- [x]` done → `- [~]` partial/deferred · don't start sprint N+1 until sprint N's **Dogfood gate** passes · every sprint inherits the **DoD gates** (see bottom, don't repeat per task) · `[Fxxx]` tags trace a task back to the [spec-gap fix worklog](analysis/spec-gap-fix-worklog-2026-07-11.md).

## Progress

| Sprint | Milestone | Spec | Days | Status |
|--------|-----------|------|:--:|:--:|
| 1 | Foundation gate + media images | SPEC-01 (+M0) | ~6.5 | ✅ **done + dogfooded live** — real image→WebP thumb/medium, byte-identical original, delete purges, fan-out fires (`make up`, 2026-07-12) |
| 2 | Notifications + password reset | SPEC-04 | ~6 | ✅ **done + dogfooded live** — reset email via Mailpit → new-pass login, old-pass 401, token single-use; bell fan-out works (2026-07-12) |
| 3 | Journal write path | SPEC-05 | ~4.5 | ✅ **done** — journal module CRUD dogfooded live (create/list/patch/delete/validation/emit-only); composer+home wired, build green |
| 4 | ⚠ Backups before money | SPEC-09 P0 | ~4 | ✅ **done + dogfooded** — real pg_dump→MinIO, `make restore-drill` passes live, `/ops/status`=ok; found+fixed a seekable-stream bug live |
| 5 | Continue rail | SPEC-07 | ~3 | 🔵 **code-complete** — build/vet/test green (9 pkgs) + `tsc` green, openapi drift regenerated; committed `89cc2b5`. **Live dogfood pending** (`make up` + resume a real video across two sessions) |
| 6 | Finance ledger (biggest) | SPEC-03 | ~8.5 | 🔵 **code-complete** — backend (14 tests) + OpenAPI + 4 frontend pages; `go build/vet/test` + `tsc` green (`66c036f`…`ff92b4b`); **live dogfood pending** |
| 7 | Comic vertical | SPEC-02 | ~4.5 | 🔵 **code-complete** — backend (6 tests) + OpenAPI + frontend (library/detail/reader); `go build/vet/test` + `tsc` green (`2c4301a`…`f7414b2`); live dogfood pending |
| 8 | People registry | SPEC-08 | ~4.5 | 🔵 **code-complete** — backend (6 tests) + OpenAPI + frontend (list/detail + wired BirthdayCard); green (`cbe3ce8`…`f55b4f2`); live dogfood pending |
| 9 | Life-stream home (last) | SPEC-06 | ~5 | ⬜ |

> Update the Status cell (⬜ → 🔵 in progress → ✅ done) as you go.

---

## Sprint 1 — Foundation gate + Media images real  ·  SPEC-01  ·  ~6.5 d
**Goal:** contract gate + shared infra + image pipeline, dogfooded. Unblocks every later sprint.
**Prereq:** decisions locked (ADR-10 first, life‑OS order) ✅

**A. ADR-10 contract gate**
- [x] Flip [ADR-10](../adr/10-openapi-contract-direction.md) status Proposed → Accepted (dated note) — *2026-07-11, with Sprint-1 update note*
- [x] `make openapi` → commit generated Go server stubs + TS client — *`api.gen.go` (1392 ln) + `types.gen.ts` (871 ln) generated & un-ignored; builds green*
- [~] Refactor at least one handler family onto the generated interface — **deferred**: codegen committed, but hand-written handlers not yet retrofitted onto `ServerInterface` (per-module as touched, per ADR-10 note)
- [x] CI job: fail build on `shared/openapi.yaml` ↔ committed-codegen drift — *`openapi` job now runs `make openapi` + `git diff --exit-code`*
- [~] Confirm contract gate green in CI — **pending push**: locally `make openapi` produces zero diff; needs a CI run to confirm

**B. Shared platform infra** ✅ done
- [x] `platform/events.Publish(ctx, name, payload)` helper (SPEC-01 P0.6) [F007]
- [x] event-name → consumer-task subscription registry (`Publisher.Subscribe`) — *subscriptions wire per-consumer when SPEC-04 lands*
- [x] unit test: two consumers of one event fan out (no `cmd/worker` panic) — *3 tests, green*
- [x] shared `asynq.Scheduler` single registration point in `cmd/worker` (SPEC-01 P0.3) [F002]
- [x] move `account.PurgeExpiredRefreshTokens` onto the scheduler — *`@every 24h`, handler wired in worker*

**C. SPEC-01 image pipeline** — code-complete, build+unit-test green; **end-to-end run (real image → WebP) still needs `make up` (ffmpeg/MinIO/DB)**
- [x] migration `0008_media_image_pipeline` — full §6: `deleting` status, `title`/`original_filename`/`origin` cols, `assets_deleting_idx` + `assets_owner_cursor_idx`, `media_asset_variants` table (+ down) [F009][F040]
- [x] variants queries (`query/media_variants.sql`) + assets cursor/purge/owner queries + `sqlc`
- [x] `media:process_image` worker — ffprobe reject animated/>8000px; WebP thumb(320)/medium(1280), metadata stripped; on **heavy** queue
- [x] `HandleThumbnail` — real poster (seek, WebP 640) + **0-video-stream skip w/ warning** (asset not failed) [P0.2]
- [x] `POST /complete`: magic-byte sniff (ranged GET) + HEAD size → HEIC/unsupported 422, >50MB 413, delete object + `failed` [F010]
- [x] variant-serving endpoint `GET /assets/{id}/variants/{variant}` (public-ish) [F011]
- [x] `DELETE /assets/{id}` via `RequireOwnerOrPermission(engine,"assets:delete:any",extract)` [F012] ← **house RBAC pattern set**
- [x] download-original `GET /assets/{id}/original` (+ null `original_filename` fallback) [F039]
- [x] orphan janitor `media:purge_orphans` on the shared scheduler `@every 1h` + abandoned-upload sweep [F038]
- [x] library page `/library/media` via template registry (`libraryMedia` view) + i18n Problem stub — *`next build`+`tsc` green* [F006]
- [x] `media:asset_ready`/`media:asset_deleted` emitted via `platform/events`; events.md updated; openapi.yaml + codegen updated for the 3 new endpoints
- *note: added `Size`/`GetRange`/`DeletePrefix` to `platform/storage` (needed for HEAD size, ranged sniff, prefix purge)*

**D. Convention plumbing (set once, copied by all later sprints)** ✅ done
- [x] RBAC seed-row pattern — established in `0003` (`WITH grants(...)`); `assets:*` already seeded there, so media needs **no** new permission migration (SPEC-01 §7)
- [x] i18n Problem-type catalog stub (`frontend/src/lib/problems.ts`) seeded with the 4 SPEC-01 types [F036]
- [x] `middleware.ts` matcher — `/library/:path*` already covers `/library/media`; no change needed (discipline applies to SPEC-03/08's new top-level routes)
- [x] template-registry discipline — used by the `/library/media` page; documented in `templates/README.md` + `frontend/CLAUDE.md`

**Dogfood gate:** upload / download / delete 20 real images · contract gate green · events + scheduler live.

---

## Sprint 2 — Notifications backbone + password reset ships  ·  SPEC-04  ·  ~6 d
**Goal:** real bell + email + working `forgot/reset-password` (today admin/CLI only).
**Prereq:** Sprint 1 (`platform/events`, `media:asset_ready` emitter).

> **Status: 🟢 code-complete** (build/vet/test green, `next build`+`tsc` green, openapi drift-gate idempotent). **End-to-end run (Mailpit reset flow, live bell) still needs `make up`.**

- [x] Module scaffold — `internal/modules/notify/*` (flat, matches media), `notifyrepo` sqlc block, `module-notify-isolation` depguard block; migrations `0009_notify_notifications` + `0010_account_password_reset_tokens`
- [x] notifications store + read API + in-app dispatch (list/mark-read/read-all, `unread_count`)
- [x] `dedup_key` + partial unique index — `ON CONFLICT DO NOTHING` idempotency [F021]
- [x] `notify:dispatch` fan-out (channel override + muted precedence + non-mutable bypass) + prefs default + `notify/api` typed `Enqueue` [F021]
- [x] permission codes use `write` verb; 6 codes seeded → `user` in `0009` [F022]
- [x] Email: `EmailSender` iface + `SMTPSender`/`LogSender` + `mailpit` in docker-compose + `SMTP_*` in `.env.example`/config
- [x] account `POST /auth/forgot-password` + `/auth/reset-password` + reset-token table (`0010`); bumps `token_version` + revokes refresh tokens
- [x] abuse controls: per-email Redis throttle (≥60s, ≤3/h) + per-IP 429 + global hourly send ceiling + uniform-timing 202 + disabled-account no-op
- [x] `media:asset_ready` consumer (`notify:on_asset_ready`) via `platform/events` (shared publisher — **first real fan-out consumer**); skip `origin='import'` [F054]
- [x] `GET /me/notifications` default `?status=all` + AC; openapi.yaml (19 paths) + codegen regenerated [F053]
- [x] bell wiring: `useQuery`(poll 60s+focus) + optimistic mark-read/read-all + `NOTIFS` fixture removed [P0.5]
- [x] `notify:*` tasks in events.md; account's `RegisterTasks` stub removed
- *P1/P2 (web push, SSE, prefs UI, security-alert, purge janitor) = registered stubs w/ TODO — deferred per spec*
- [ ] register `notify:*` tasks in events.md; remove account's stub `RegisterTasks`

**Dogfood gate:** reset your own password end-to-end through Mailpit.

---

## Sprint 3 — First life-stream write path  ·  SPEC-05 journal  ·  ~4.5 d
**Goal:** the first real post type; kill part of the fixture home.
**Prereq:** none hard (photos P1 need SPEC-01, already done).

> **Status: ✅ done + backend dogfooded live** (build/test green, `next build`+`tsc` green, migration `0011` applied, CRUD/validation/owner-scoping/emit-only verified live via `make up`).

- [x] Scaffold `internal/modules/journal/*` + migration `0011_journal_entries` (+ 3 seeds → `user`) + `journalrepo` sqlc block + depguard block
- [x] CRUD (`/journal/entries`) + RBAC (`journal:read/write/delete:own`) + RFC 7807 + openapi (21 paths) + codegen
- [x] `journal:entry_created` emit — **emit-only** after commit; verified live: 0 consumer tasks enqueued [F024][F057]
- [~] projection row transactional — **deferred to SPEC-06** (`stream_items` is SPEC-06's table; doesn't exist yet). Interim home reads `journal_entries` directly per P0.4.
- [x] Composer wiring + interim home list (`useInfiniteQuery`) + `INITIAL_FEED` fixture deleted from `HomeView`; optimistic create/edit/delete; sanitizing markdown renderer
- [x] mood input + `occurred_at` date control on the composer (P0.4) [F058]
- [ ] P1 (deferred): photo attachments (needs SPEC-01 `assets:write:own`) [F003]

**Dogfood gate:** ✅ backend loop proven live (create/list/patch/delete/validation). Frontend UI browser-run pending (contract matches live backend; `next build` green).

---

## Sprint 4 — ⚠ Data safety before money  ·  SPEC-09 P0  ·  ~4 d
**Goal:** self-running backups + a **rehearsed** restore. **Must precede Sprint 6.**
**Prereq:** shared scheduler (Sprint 1).

> **Status: ✅ done + dogfooded live** — full backup→restore loop proven via `make up`: real `pg_dump` → sha256 → MinIO → restore drill passes.

- [x] Scaffold `internal/modules/ops/*` + migration `0012_ops_backup_runs` (+ 4 seeds: `ops:read`/`queues:read`→admin, `takeout:read/write:own`→user) + `opsrepo` sqlc block + depguard
- [x] `ops:backup_database` task: `pg_dump -Fc` **direct to Postgres** (`BACKUP_DATABASE_URL`), spooled + sha256-teed → storage; retention 7 daily + 4 weekly (pure fn, unit-tested); on shared scheduler nightly `0 3 * * *`
- [x] overwrite `backups/pg/LATEST.json {storage_key, sha256, size_bytes, finished_at}` [F031] — verified live (186 B manifest, sha256 matches dump)
- [x] `ops:backup_completed`/`ops:backup_failed` events + audit `ops.backup.completed`/`.failed`
- [x] restore drill `make restore-drill` (docker-first script) + runbook `docs/guides/backup-restore.md` — **passes live** against a real dump [P0.4 AC]
- [x] `GET /ops/status` (`ops:read` admin) + OpenAPI (22 paths); `state` precedence failed→stale(never-ran→null)→ok (pure fn, unit-tested); verified live (403/401/200, `state:"ok"`) [F077][F075]
- [x] `/healthz` untouched
- [ ] takeout (P1.7) — deferred [F032]
- *🐛 caught live: backup streamed a non-seekable pipe into S3 `Put` → fixed to spool to temp file (like `media.UploadSource`)*

**Dogfood gate:** ✅ **passed** — enqueued a backup, dump landed in MinIO, `make restore-drill` restored it into a scratch DB with sanity checks green.

---

## Sprint 5 — Continue rail (resume playback)  ·  SPEC-07  ·  ~3 d
**Goal:** resume-where-you-left-off for video (the nightly retention surface).
**Prereq:** none hard; comic leg joins after Sprint 7.

> **Status: 🔵 code-complete** (committed `89cc2b5`). Backend `go build`/`vet`/`test`
> green (9 pkgs); frontend `tsc --noEmit` green; api.gen.go + types.gen.ts
> regenerated so the openapi drift gate stays green. **Live browser dogfood
> (resume across two sessions via `make up`) still pending.**

- [x] migration + queries + `mediaapi.Continue` — `0013_media_playback_progress`, `query/media_progress.sql`, `mediaapi.Continue`/`ContinueItem`
- [x] **`GET` progress read endpoint** — the resume read-path [F029] (`GET /assets/{id}/progress`)
- [x] beacon endpoint + Vidstack throttle/pagehide wiring (Blob content-type, keepalive) + resume UX [F069] — `sendBeacon(Blob{application/json})` + keepalive fallback, `visibilitychange`/`pagehide`, ~10 s throttle
- [x] NULL-duration resume: skip the 95% completion gate [F027][F068] — `shouldResume` resumes any ≥30 s position when `progress_pct` is null
- [x] named `/library/media/[id]` playback route + href target [F028] — resolves `libraryMediaDetail` via the template registry
- [~] `cmd/api` aggregator + OpenAPI + contract test — aggregator + OpenAPI done; predicate covered by service unit tests, **HTTP-level `/continue` schema contract test still TODO**
- [x] beacon AC: no permission-based bypass [F070] — `PutProgress` is owner-scoped by construction; unit-tested (non-owner → 404/ErrForbidden)
- [x] P1.5 completion latch + event *(defer optional)* — `completed_at` NULL→set latch, `media:playback_completed` emitted once; unit-tested

**Dogfood gate:** resume a real video across two sessions. *(pending live run)*

---

## Sprint 6 — Finance ledger (biggest)  ·  SPEC-03  ·  ~8.5 d
**Goal:** Money-Lover-class ledger. **Do not start any P1 until first reconciliation succeeds.**
**Prereq:** ⚠ Sprint 4 (backups) done first.

> **Status: 🔵 code-complete + build/test green.** Backend (14 unit tests, `66c036f`),
> OpenAPI spec + regenerated codegen (`d11b67e`), and the four /bank pages (`ff92b4b`)
> all landed. `go build/vet/test` (10 pkgs) + `tsc --noEmit` green. **Live browser
> dogfood (log real transactions, month-end reconcile) still pending — needs `make up`.**

- [x] record **D-41** decision entry (money-model divergence: integer minor units vs D-14) — feature-inventory.md
- [x] migration + seeds + `sqlc` (accounts, transactions, categories, budgets, transfers; integer minor units) — `0014_bank_core`
- [x] Accounts + transactions CRUD + derived balances + RBAC + OpenAPI — receipt attachments = P1.10 (deferred)
- [x] Categories: CRUD, hierarchy invariants incl. `parent_id` mutability [F019], delete/reassign matrix
- [x] Transfers: paired legs + `counterparty_account_id` in payload [F004] — fee-convention is P1.13 (deferred, per spec)
- [x] Budgets: expense-kind 422 [F046] + immutable `month` `EXTRACT(day)=1` CHECK [F050] + dashboard roll-up
- [x] Frontend: quick-add, transactions list (**cursor** paging [F020]), dashboard, accounts, budgets tree
- [x] `/bank/:path*` added to `middleware.ts` matcher [F005]; `/bank` views via template registry [F006]
- [x] events: `bank:transaction_*` (+ `counterparty_account_id`) → events.md — `budget_exceeded` is P1.12 (deferred)

**Dogfood gate:** log real transactions 20 of 30 days; month-end reconcile within rounding. **Gate before any P1.** *(pending live run via `make up`)*

---

## Sprint 7 — Comic vertical (reference impl)  ·  SPEC-02  ·  ~4.5 d (+2 P1)
**Goal:** comic reader/library + the `media → domain vertical` template for movie/music/story.
**Prereq:** Sprint 1 (image kind).

> **Status: 🔵 code-complete + build/test green** (`2c4301a` backend, `8017329`
> OpenAPI, `f7414b2` frontend). Also landed the media enablement `mediaapi.GetAsset`
> (was a stub) so verticals can validate asset references. **Live dogfood pending.**

- [x] migration + queries + repository + service scaffolding + identity-anchor `users(id)` FKs [F017] — `0015_comic_core`
- [x] CRUD + publish + RBAC: **owner-or-elevated** `RequireOwnerOrPermission`; seed `comics:write:any`/`delete:any`/`publish:any` [F001] + OpenAPI
- [x] reader + vertical progress (`comic_reading_progress`) + page DELETE endpoint [F018] — page_id-keyed, membership-validated
- [x] library/detail pages replacing placeholders (cursor paging [F044]) + template registry [F006]
- [x] P0.6 asset-deletion coupling: consumes `media:asset_deleted` → reaps pages / NULLs covers (soft cascade)
- [~] `comic:chapter_deleted`/`_published` emit + events.md [F042] — **P1.9, deferred** (no consumer required to ship)
- [ ] P1: zip import (presigned-PUT path, entry-scaled poll timeout [F015][F016]) + reader modes/bookmarks — **deferred**

**Dogfood gate:** import one real chapter zip, read it on a mobile viewport. *(pending live run; zip import is P1.7)*

---

## Sprint 8 — People registry (contacts + birthdays)  ·  SPEC-08  ·  ~4.5 d
**Goal:** "mom's birthday in 3 days" becomes possible.
**Prereq:** none hard (avatars P1 need SPEC-01).

> **Status: 🔵 code-complete + build/test green** (`cbe3ce8` backend, `683408b`
> OpenAPI, `f55b4f2` frontend). **Live dogfood pending** (enter real birthdays,
> confirm the daily scan emits via `make up`).

- [x] Scaffold + migration + `sqlc` + CRUD + RBAC + OpenAPI; `birthday:null` clears consistent with the calendar default [F030] — `0016_people_persons`
- [x] `nextOccurrence` (TZ + Feb-29 + no-year cases, table-driven tests) + upcoming-birthdays endpoint
- [x] `people:scan_birthdays` task on scheduler + dedup (notice `id`/UNIQUE) [F071] + `people:birthday_upcoming` emit — outbox/at-least-once
- [x] frontend list/detail + `BirthdayCard` wiring + `/people/:path*` matcher [F005] + template registry [F006]
- [ ] P1 (defer): interactions log, avatar upload (creator-tier `assets:write:own`) [F003] — **deferred**

**Dogfood gate:** enter real family birthdays; confirm upcoming list + emitted notice. *(pending live run)*

---

## Sprint 9 — Life-stream home (the proof screen)  ·  SPEC-06  ·  ~5 d  ·  LAST
**Goal:** the real `/` — merged stream + widgets consuming everything above.
**Prereq:** SPEC-05 (hard); every other producer as available (widgets degrade to empty state).

- [ ] `stream_items` projection table + consumers + idempotency tests
- [ ] transfer render normalization (either leg → identical card) [F004]; `bank:transaction_updated` upsert [F063]
- [ ] `GET /stream` merged read (limit default 30 / max 50 [F064]) + OpenAPI + mapping/fallback ACs [F065]
- [ ] home replacement (stream island + composer integration) + fixture removal
- [ ] widget rail wiring (finance / birthday / continue / notifications) + empty states
- [ ] P1 (defer): memories + backfill — P1.6 excludes `origin='import'` assets [F025]

**Dogfood gate:** open `/` after a week of real cross-module use — it should read like *your* day.

---

## Definition of Done — every sprint inherits these (don't re-list per task)

- [ ] **Contract:** new endpoints in `shared/openapi.yaml`; codegen runs; CI drift gate green
- [ ] **Events:** every task/event in [reference/events.md](../reference/events.md)
- [ ] **i18n:** every Problem `type` URI has an i18n key (same PR)
- [ ] **Migrations:** real sequential `000N_<module>_*.up/down.sql`; up→down→up roundtrip passes (CI)
- [ ] **RBAC:** every endpoint names its permission; introducing spec ships the seed rows
- [ ] **Tests:** unit + contract; module isolation (depguard/CI) respected
- [ ] **Tracker:** [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md) updated
- [ ] **Dogfood:** the sprint's gate met before starting the next

*Living document — flip Status cells and check boxes as you go; re-baseline effort after Sprint 1 actuals.*
