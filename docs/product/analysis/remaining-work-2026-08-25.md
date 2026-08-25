# Remaining Work — Code-Verified Audit

**Date:** 2026-08-25 · **Type:** point-in-time audit · **Question answered:** *"What is actually left to do?"*
**Method rule:** every claim about *state* is sourced to code, a migration, a test file, or `git log`. Documents are cited only as evidence of *intent*.

---

## 1. Method + caveat

I read the wiring layer (`backend/cmd/api/main.go`, `backend/cmd/worker/main.go`), every module's `module.go` route table, all 30 migration pairs in `backend/db/migrations/`, `shared/openapi.yaml`'s 58 path entries, all 15 `backend/**/*_test.go`, `backend/.golangci.yml`, `.github/workflows/ci.yml`, `Makefile`, `.env.example`, `docker-compose.yml`, and the whole `frontend/src` tree — then read the intent corpus (SPEC-01…09, ADR-00…10, `docs/product/backlog.md`, `feature-inventory.md`, `vision.md`, `briefs/`, `docs/testing/*`, `docs/reference/events.md`) and diffed intent against code.

**Two things cannot be established from the tree and are not claimed here.** (a) **Which migrations are applied** to the running database — `backend/db/migrations/` tells you what schema *exists as SQL*, not what the live cluster has run. Since Postgres moved to the host cluster, there is no in-repo signal at all. (b) **Runtime behaviour** — whether a handler returns what the spec says, whether the nightly backup has actually produced a dump, whether `make restore-drill` has ever passed. `httptest` appears **0 times** in the backend, so no test in the repo answers (b) either.

---

## 2. What is actually done

A module is live iff it is **constructed and `MountHTTP`'d** in `cmd/api/main.go`. All 12 pass. All 12 have a populated `repository/`. All 12 have a depguard isolation block (`backend/.golangci.yml:58-259`).

| Module | Constructed · Mounted | Tests | SPEC deliverables that have code |
|---|---|---|---|
| **account** | `cmd/api/main.go:161` · `:582` | 4 files, 10 funcs (`internal/modules/account/{auth/password_test.go,auth/reset_test.go,handler/password_reset_test.go,rbac/permission_test.go}`) | ADR-06 local password auth (login/register/refresh/logout/logout-all/me, `module.go:123-141`); RBAC engine + hierarchy; refresh rotation + reuse detection; audit; SPEC-04 P0.3 account half (forgot/reset-password, `module.go:130-131`, migration `0010`) |
| **tenant** | `:199` · `:583` | **0** | ADR-07 steps 1–4: `organizations` + `organization_memberships` (`0018_tenant_core.up.sql:12,28`), the two DB roles (`0019`), `BeginTenantScope`, `RequireTenant` (`internal/modules/tenant/middleware/require_tenant.go`), `GET /me/organizations` (`module.go:54-59`). **That one endpoint is the module's entire HTTP surface.** |
| **media** | `:283` · `:584` | 22 funcs (`internal/modules/media/service_test.go`) | SPEC-01 P0.1–P0.6 + P1.2; SPEC-07 P0.1–P0.3 + P1.5 (`handler.go` routes at `module.go:89-107`); three worker files (`worker/{process_image,thumbnail,transcode}.go`) |
| **notify** | `:307` · `:585` | 7 funcs (`notify/service_test.go`) | SPEC-04 P0.1 (`module.go:81-87`), P0.2 dispatch, P0.3 email + Mailpit (`cmd/worker/main.go:194-197`), P0.4 `notify:on_asset_ready` (`cmd/worker/main.go:282`) |
| **journal** | `:326` · `:586` | 10 funcs (`journal/journal_test.go`) | SPEC-05 P0.1–P0.3 (`module.go:65-73`); SPEC-06 P0.1 projection (9 `journal:stream_*` consumers, `cmd/worker/main.go:293-301`) + P0.2 `GET /stream` (`module.go:76-80`) |
| **ops** | `:348` · `:587` | 5 funcs (`ops/{retention,state}_test.go`) | SPEC-09 P0.1, P0.2 nightly backup (`ops/backup.go:41`, scheduled `cmd/worker/main.go:377`), P0.4 script (`Makefile:86` → `backend/scripts/restore-drill.sh`), P0.5 sentinel (`module.go:71`), P1.6 queue console (`cmd/api/main.go:602-608`) |
| **bank** | `:360` · `:588` | 14 funcs (`bank/bank_test.go`) — best-covered module | SPEC-03 P0.1–P0.9 in full (`module.go:55-92`, migration `0014`) |
| **comic** | `:416` · `:589` | 13 funcs (`comic/{comic,sourceguard}_test.go`) | SPEC-02 P0.1–P0.6, P1.7 zip import (`import.go`, 772 lines), P1.8-as-sync (`module.go:84-127`, migrations `0026`–`0030`), P1.9 events, R1–R5 reader. **The deepest vertical.** |
| **people** | `:450` · `:590` | 6 funcs (`people/people_test.go`) | SPEC-08 P0.1–P0.5 (`module.go:61-70`), outbox birthday scan (`cmd/worker/main.go:382`) |
| **movie** | `:475` · `:591` | **0** | No SPEC exists. CRUD + publish/unpublish (`module.go:65-76`), migration `0021`, emits `movie:published` |
| **music** | `:501` · `:592` | **0** | No SPEC exists. Same shape (`module.go:54-65`), migration `0022`, emits `music:track_published` |
| **story** | `:530` · `:593` | **0** | No SPEC exists. CRUD + chapters + reorder (`module.go:73-94`), migration `0023`, emits `story:published` |
| **frontend** | 16 URLs (14 `(app)`, 2 `(public)`) | **0 test files, no `vitest.config.*`** | comic, bank, people, notify, journal/stream, media/upload, auth all have real screens; 4,542 LOC of views + 4,872 LOC of components |

**Platform:** `config`, `db`, `events`, `storage`, `audit`, `middleware` exist. `internal/platform/cache/` and `internal/platform/jobs/` are **empty directories**; `internal/platform/server/` — reserved at `backend/MODULES.md:48` — was never created.

**Infrastructure that works:** three Asynq servers with the OOM guard intact (`cmd/worker/main.go:307,319,328`); the single periodic scheduler with 4 registrations (`:366-385`); the event fan-out with 14 subscriber edges registered twice (API-emitted set at `cmd/api/main.go:236-250`, worker-emitted set at `cmd/worker/main.go:282-301`); CI with 4 jobs including the sqlc drift gate (`.github/workflows/ci.yml:33-40`) and the OpenAPI codegen drift gate (`:101-107`).

---

## 3. What is specced but has no code

This is the core answer. Every row below was verified by absence in the route table, the migration set, the task-name grep, or the frontend tree.

### 3.1 Cross-cutting — the largest gaps

| Gap | Evidence of intent | Evidence of absence |
|---|---|---|
| **No HTTP handler is tested.** | `docs/testing/TEST-PLAN.md:57` describes an L2 integration layer with Postgres + Dragonfly + MinIO | `grep -rn httptest backend --include=*.go` → **0**. Every backend test drives services through in-memory fakes (`bank_test.go:15`, `journal_test.go:27`, `media/service_test.go:26`). No test opens a Postgres connection; no migration up/down test exists. |
| **No frontend test.** | `Makefile:95-96`, `frontend/package.json:12` | 0 files matching `*.test.*` / `*.spec.*`; no `vitest.config.*`. `vitest run` with zero files exits non-zero → **`make test` (`Makefile:90`) fails today.** CI never runs it (`ci.yml:109-131` runs `pnpm build` only). |
| **RFC 7807 is not the error contract.** | `docs/product/specs/README.md:47-52` makes it a DoD for every spec; ADR-10:66 makes it part of the cutover | `writeErr`/`writeError` emit `{code,message}` in 5 places (`media/handler.go:411`, `notify/handler.go:161`, `tenant/module.go:87`, `tenant/middleware/require_tenant.go:75`, `account/handler/util.go:18`). `writeProblem` exists in 11 files but is not universal. |
| **No handler implements the generated `ServerInterface`.** | ADR-10:47-71 + `:6` "retrofitting proceeds per module" | `backend/internal/handler/api.gen.go` is its own only referent in the tree. |
| **~30 live endpoints are outside the OpenAPI contract.** | `docs/product/specs/README.md:73-75`; ADR-10:71 "an endpoint missing from the spec fails CI" | `shared/openapi.yaml` has no `/movies*` (8 routes), no `/tracks*` (8), no `/stories*` + `/story-chapters*` (11), no `/me/organizations`, no `/time`, and none of the four comic-import paths (`/comics/{id}/imports`, `/chapters/{id}/imports`, `/imports/{id}`, `/imports/{id}/zip`). The CI gate proves codegen matches the spec — it cannot detect a route that was never specced. |
| **No search anywhere.** | D-2 (`feature-inventory.md:873`) "Postgres FTS first"; `story/README.md` FTS + `unaccent`; `backlog.md:53,81` | Zero `tsvector` / `to_tsquery` in any migration or query file. No `/search` path, no `?q=` handling in any list handler. |
| **No MFA/TOTP.** | D-27/D-28 (`feature-inventory.md:1363,1414`); ADR-06:90-99; `account/README.md:47` | Zero implementation. The only hits are a comment (`account/api/api.go:21`) and two generated `Problem` fields (`api.gen.go:1247,1253`). |
| **No brute-force protection on `/auth/login`.** | ADR-06:90-99 lists it as a responsibility Portal now owns; `backlog.md:19` claims it works; `backlog.md:74` correctly calls it "built, unused" | `internal/platform/middleware/ratelimit.go` has **zero importers** (`grep -rn "platform/middleware" backend` → nothing). `account/module.go:120` says the caller supplies the rate limit; `cmd/api/main.go:582` supplies none. Only the password-reset endpoints self-throttle (`account/handler/password_reset.go:31-36`). |
| **`cmd/sysjobs` + `internal/sysrepository` do not exist.** | ADR-07:114; CLAUDE.md; `backend/.golangci.yml:21-22` (guardrail is pre-armed) | Neither path exists. `backend/cmd/` holds `api`, `worker`, `opsenqueue`. |

### 3.2 ADR-07 tenancy — the cutover is the biggest single unfinished chain

Schema is complete: all 17 legacy tables get `tenant_id` + `FORCE RLS` in `0020_platform_rls_enable.up.sql`, and every table created since (`0021`–`0030`) ships tenant-scoped from birth. But the ADR's own exit criterion (`docs/adr/07-tenancy-rls-model.md:118`) is not met.

| ADR-07 step | Status |
|---|---|
| 1 · roles `portal_app` / `portal_sys` | **SQL done** (`0019_platform_rls_roles.up.sql`), **runtime not cut over.** `portal_app` appears nowhere outside that migration and one comment — no `.env`, no compose, no Go code. The app still connects as the superuser `portal`, which bypasses even `FORCE RLS`. **All RLS is therefore inert.** |
| 2 · `0018_tenant_core` + personal-org backfill | Done (`0018_tenant_core.up.sql:12,28,44`) |
| 3 · `BeginTenantScope` + pool config | Done (`internal/platform/db/db.go`) |
| 4 · `0020_rls_enable` | **SQL written; the file carries a `⚠️ DO NOT APPLY` gate** — see §4.1 |
| 5 · `RequireTenant` + `GET /me/organizations` + **`POST /auth/switch-tenant`** + **`POST/GET /admin/organizations`** | First two done; **last two missing** (`grep -rn "switch-tenant" backend` → nothing) |
| 6 · per-tenant RBAC: `user_roles(user_id, org_id, role_id)` | **Missing.** `org_id` appears only on `organization_memberships` (`0018:29,33`); `user_roles` is unchanged. |
| 7 · `cmd/sysjobs` + `internal/sysrepository` | **Missing** |
| 8 · **RLS isolation test** ("tenant B cannot read tenant A's rows on the `portal_app` role", ADR-07:104) | **Missing.** No test opens a DB connection at all. |
| 9 · observability profile in the same sprint | Deferred by ADR-01/ADR-03 — see §7 |
| — · the `/t/{org}/…` URL contract (D-23, ADR-07:118) | **Missing.** Every route mounts under plain `/api/v1` (`cmd/api/main.go:575`). |

### 3.3 Per-SPEC P1/P2 deliverables with nothing behind them

| SPEC | Deliverable | Spec line | Absence |
|---|---|---|---|
| 01 | **P1.1 metadata edit** `PATCH /assets/{id}` | `SPEC-01:308` | Not mounted (`media/module.go:89-107` has no PATCH) and not in `shared/openapi.yaml` (`/assets/{id}` at `:453` declares only `get` and `delete`) |
| 02 | **P1.8 Bookmarks** — `comic_bookmarks`, `PUT/DELETE /pages/{id}/bookmark`, `GET /comics/{id}/bookmarks` | `SPEC-02:301-303` | No table in any migration; no route. *(The shipped "P1.8" is external-source sync — a P-number collision, see §4.6.)* |
| 03 | **P1.10 receipt attachments** `receipt_asset_id` | `SPEC-03:350` | Column absent from `0014_bank_core.up.sql` |
| 03 | **P1.11 monthly report page** | `SPEC-03:352` | No route, no view |
| 03 | **P1.12 `bank:budget_exceeded`** | `SPEC-03:354` | Name absent from the whole task/event grep |
| 03 | **P1.13 structured transfer fees** | `SPEC-03:356` | `fee_amount` absent from `internal/modules/bank/**` |
| 04 | **P1.1 Web Push** — `POST/DELETE /me/push-subscriptions`, VAPID, 410-prune | `SPEC-04:154` | Table exists (`0009:31`) but the handler is an explicit stub (`notify/service.go:324-328`) and no route is mounted |
| 04 | **P1.2 SSE** `GET /me/notifications/stream` | `SPEC-04:155` | Not mounted |
| 04 | **P1.3 preferences UI** `GET/PUT /me/notification-preferences` | `SPEC-04:156` | Table exists (`0009:21`); no route |
| 04 | **P1.4 `account.security_alert`** from refresh-reuse | `SPEC-04:157` | No dispatch call on the reuse path |
| 04 | **P2 `notify:purge_old`** | `SPEC-04:164` | Handler registered (`cmd/worker/main.go:348`) but **never scheduled** — the scheduler holds only 4 entries (`:366-385`). Dead code. |
| 05 | **P1.5 photo attachments** | `SPEC-05:162` | `asset_ids` column exists (`0011:11`) and the handler deliberately 422s it (`journal/handler.go:226`) |
| 05 | **P1.6 mood picker** | `SPEC-05:173` | No preset-emoji component |
| 06 | **P1.5 on-this-day** `GET /stream/memories` | `SPEC-06:209` | Not mounted (`journal/module.go:76-80` has only `GET /stream`) |
| 06 | **P1.6 `journal:backfill_stream`** | `SPEC-06:214` | Task name absent from the code grep |
| 06 | **P0.4 widget rail** — `PersonalInfoWidget` → `GET /auth/me` | `SPEC-06:193-199` | `frontend/src/templates/v1/components/widget/PersonalInfoWidget.tsx` has **0 importers**. The other 6 rail widgets are wired. |
| 07 | **P2 comic leg of `/continue`** | `SPEC-07:177-182` | `handleContinue` calls only `mediaMod.API().Continue` (`cmd/api/main.go:698`); `comicapi.Continue` does not exist |
| 08 | **P1.6 interactions log** `people_interactions` | `SPEC-08:202` | Table absent from `0016_people_persons.up.sql` |
| 08 | **P1.7 avatar reap** | `SPEC-08:205` | `avatar_asset_id` exists (`0016:17`) but `people` is **not** subscribed to `media:asset_deleted` — the 5 subscribers are comic, movie, music, story, journal (`cmd/worker/main.go:285-288,295`) |
| 09 | **P1.7 owner takeout** — `POST /me/export`, `ops_exports`, `ops:takeout`, `ops:purge_exports`, `ExportProvider` | `SPEC-09:194-215` | No table (`0012:3` says it "ships in a later migration"), no route, no task |
| 09 | **P0.4 restore drill *executed*** — "code without one exercised restore does not count as done" | `SPEC-09:146-150` | Script exists (`backend/scripts/restore-drill.sh`); **whether it has ever passed against a real nightly dump is not verifiable from the tree.** |

### 3.4 The three newest verticals are half-built

`movie`, `music`, and `story` landed in the four undated `update` commits between `f11cf3f` (2026-07-19) and `feb703a` (2026-07-23). They have **no SPEC, no test, no OpenAPI path, and no UI**. Concretely:

- **No frontend at all.** Zero `/movies`, `/tracks`, `/stories` references in `frontend/src`. `frontend/src/templates/v1/views/library/novel/NovelDetailView.tsx` is a 26-line static placeholder ending in `{/* TODO: synopsis, metadata, chapter list */}`, and `frontend/src/app/(app)/library/page.tsx:23-24` links to it with an `as Route` cast plus a comment admitting the index route does not exist. The three `components/music/*` files (272 LOC) have zero importers.
- **Their publish events fall into the void.** `movie:published`, `music:track_published`, `story:published` are emitted (`movie/service.go:138`, `music/service.go:142`, `story/service.go`) but have **no subscriber** in either binary. So publishing a movie produces no life-stream card, while publishing a comic chapter does (`cmd/worker/main.go:300`). SPEC-06's stream is blind to three of the four verticals.

---

## 4. Doc↔code contradictions

### 4.1 The RLS cutover gate — read this one carefully

`backend/db/migrations/0020_platform_rls_enable.up.sql:5-14` carries:

> `⚠️ DO NOT APPLY until worker tenant-scoping (Increment "1b") has landed.` … *"its event consumers + scan insert into `stream_items`, `media_asset_variants`, `notifications`, `people_birthday_notices`. Applying this before the worker opens `BeginTenantScope` will break those async paths. Sequence: 1b (worker) → live-verify 2a/1b → THEN this migration."*

**The stated gate now appears met, by two different mechanisms.** `cmd/worker/main.go:129-147` defines `runInUserTenant` and wires it into journal (`:214` → `stream_items`), notify (`:206` → `notifications`), and people (`:258` → `people_birthday_notices`). The fourth table, `media_asset_variants`, is **not** covered that way — media's `Deps` has no `RunInUserTenant` field — but its INSERT never relies on the GUC default: `internal/modules/media/query/media_variants.sql:8-9` supplies `tenant_id` from a subquery, `(SELECT tenant_id FROM assets WHERE id = $1)`.

**That workaround dies at cutover, and nothing in the warning says so.** Under `portal_app`, the `assets` RLS policy filters that subquery, it returns NULL, and the `NOT NULL` constraint (`0020:99`) kills every variant insert. So: 0020 is *now safe to apply* (RLS stays inert under the superuser), but **the media worker needs a tenant tx before `DATABASE_URL` flips to `portal_app`.** The migration's warning is stale in a way that will bite at exactly the wrong moment.

Separately: the ADR that governs all of this, `docs/adr/07-tenancy-rls-model.md:3`, still reads **"Proposed — deferred … no code lands until then"** and `:10` still asserts *"there is no `tenant_id` column anywhere today."* Both are false — migrations `0018`–`0030` and a live `tenant` module contradict them. `docs/adr/README.md:24` calls the same ADR "accepted (deferred design)". Its implementation-plan checklist (`:106-116`) is entirely unchecked despite steps 2–4 being done.

### 4.2 `docs/reference/events.md` — the event registry is materially wrong

| Line | Claim | Code |
|---|---|---|
| `:20` | `media:asset_ready` — "emitter only; no consumer yet" | **2 consumers**: `cmd/worker/main.go:282` (notify), `:293` (journal stream) |
| `:21` | `media:asset_deleted` — "1 consumer (comic P0.6)" | **5 consumers**: `cmd/worker/main.go:285-288,295` |
| `:22` | `media:playback_completed` — "no consumer yet" | Consumed at `cmd/worker/main.go:294`, `cmd/api/main.go:242` |
| `:25-27` | `bank:transaction_*` — "no consumer yet" | Consumed at `cmd/worker/main.go:296-298`, `cmd/api/main.go:244-246` |
| `:30` | `people:birthday_upcoming` — "no consumer yet" | Consumed at `cmd/worker/main.go:299` |
| `:40` | `media:process_image` — "heavy queue" | Its **own `image` queue and its own server** (`cmd/worker/main.go:319-324`) |
| `:43` | `comic:import_zip` — "planned (SPEC-02 P1.7)" | Live: `import.go:139`, routes at `comic/module.go:83,114,122-127`, migrations `0026`/`0027` |
| — | Missing entirely | `movie:on_asset_deleted`, `music:on_asset_deleted`, `story:on_asset_deleted`, `movie:published`, `music:track_published`, `story:published`, and all 9 comic-sync endpoints' tasks |

`SPEC-06:266-268` makes updating this table's Consumers column a **DoD condition**. It was not done.

### 4.3 `.env.example` points at hosts that no longer exist

`make up` auto-creates `.env` from it (`Makefile:11`). It still says `POSTGRES_HOST=postgres`, `DATABASE_URL=…@pgbouncer:6432/…`, and `BACKUP_DATABASE_URL=…@postgres:5432/…`. Both services are commented out of `docker-compose.yml` (the DB moved to the host cluster). **A fresh clone cannot start.** It also declares `GRAFANA_ADMIN_*` and `GLITCHTIP_DSN` for services that do not exist and profiles that do not exist (`grep -n profiles docker-compose.yml` → nothing).

### 4.4 Refresh-token TTL: docs say 30 d, code says 24 h

`CLAUDE.md` and the ADR-06 narrative say "long-lived random refresh token (256-bit, SHA-256-hashed at rest, **30d**)". `internal/platform/config/config.go:38` is `envDefault:"24h"` with the comment "1 day (remember-me window)", and `.env.example` agrees. ADR-06's own update note (`06-local-auth-model.md:11`) correctly records 24 h — so CLAUDE.md is the stale one.

### 4.5 Module `README.md` "Open work" sections are wholesale wrong

Each names a migration number that does not exist and lists work that is done:

| File | Claims open | Reality |
|---|---|---|
| `comic/README.md:14-19` | "Migration `0009_comic_init`", CRUD, RTL flag | `0015_comic_core`; CRUD live; RTL shipped in `0024_comic_reading_direction` |
| `movie/README.md:18-22` | "Migration `0006_movie_init`", CRUD | `0021_movie_core`; CRUD live |
| `music/README.md:18-22` | "Migration `0007_music_init`", CRUD | `0022_music_core`; CRUD live |
| `story/README.md:14-19` | "Migration `0008_story_init`", CRUD | `0023_story_core`; CRUD live |
| `tenant/README.md:27-31` | "Migration `0004_tenant_organizations`", "`RequireTenant` middleware (skeleton)" | `0018_tenant_core`; middleware live and wrapping every domain module |
| `media/README.md:33` | "Real FFmpeg pipeline … currently logs and returns nil" | `worker/transcode.go` is real |
| `journal/README.md:27-30` | "SPEC-06 stream projection … and `GET /stream`" | Both shipped |
| `account/README.md:50` | links `MILESTONE_CHECKS.md` and `doc/en/authoration.md` | Both paths deleted |

Genuinely still open from those lists: TOTP/step-up (account), HLS ladder + S3 multipart (media), FTS (movie/story), audio transcode profile (music), story reading progress, journal P1.5, tenant lifecycle endpoints.

An in-code comment is stale the same way: `tenant/module.go:47-48` says "Domain modules wrap their authenticated routes with it in a later increment; today it guards `/me/organizations` as the reference wiring" — but `cmd/api/main.go:210-212` builds `authTenant` and hands it to **all eleven** domain modules.

### 4.6 Everything else

- **`MILESTONE_CHECKS.md` is cited as authoritative in 5 live docs** — `docs/reference/README.md:12`, `docs/adr/09-docs-architecture.md:36-37`, `feature-inventory.md:3`, `analysis/facebook-comparison.md:3`, `testing/SESSION-HANDOFF-2026-07-12.md:138` — and was deleted in `f11cf3f`.
- **`docs/reference/README.md:11` claims the OpenAPI drift is "stale `/auth/callback`, missing `/auth/register`."** Both are wrong: `/auth/register` is at `shared/openapi.yaml:100` and `/auth/callback` does not appear. The *real* drift is §3.1's ~30 unspecced routes. ADR-06's action item 8 (`:141`) and ADR-01's update (`:11`) repeat the same dead claim.
- **P-number collision in SPEC-02.** `SPEC-02:301` defines P1.8 as *Bookmarks*; the rev-13/14 headers (`:4-5`) use P1.8 for *external-source sync*. Sync shipped; Bookmarks did not. Any P-keyed tracking mis-maps one of them.
- **SPEC-02's own non-goal is stale.** `SPEC-02:43` lists "import/scraping from external sources" as out of scope; rev 13/14 shipped exactly that.
- **ADR status lines disagree with themselves.** ADR-10 declares `**accepted** 2026-07-11` at `:3` while `docs/adr/README.md:27` says "proposed" and its own action item `:126` is "Accept this ADR and flip status to accepted". ADR-08 says "proposed" at `:3` while `briefs/00:3`, `briefs/README:20` and `briefs/03:6` all call it landed and binding.
- **`backlog.md` is 7 weeks stale and inverted in places.** `:19` claims brute-force lockout works (it does not); `:38` claims the thumbnail worker is a stub (it is not); `:60` claims movie/music/story/comic are "just `module.go` + an `api/` stub" (all four are full verticals); `:92` says Bank is deferred pending MFA (ADR-08 and SPEC-03 shipped it). Its "Suggested next order (P1)" (`:99-105`) is pre-ADR-08 and should not be followed.
- **`analysis/facebook-comparison.md:3`** is `Last verified: 2026-07-06` and asserts no image pipeline, no notifications backend, no birthdays — all now false. It also links `missing-features.md`, which no longer exists.
- **`docs/testing/TRACEABILITY-MATRIX.md` names zero real tests.** All 436 `TC-` ids are doc rows; `grep _test.go docs/testing/*.md` → 0. Its Cov column is 100 % `✅` across 47 rows with no `⚠`/`✖`; its Result column is entirely empty; SPEC-08 P0.1 has no row at all, violating its own gate at `:5-6`; the "every P0 requirement mapped" claim at `:153` is false (47 of 48).
- **`docs/testing/TEST-PLAN.md:57`** describes a container-backed integration layer that does not exist; `:86` says `make up` starts Postgres and PgBouncer (both commented out); `:87` says CI runs `vitest run` (`ci.yml:109-131` does not).
- **`docs/testing/SESSION-HANDOFF-2026-07-12.md:3-6`** declares itself "NOT committed (contains dev test creds)" — it is committed, and `:38` holds a plaintext password.
- **`feature-inventory.md` has no `D-30`** — the sequence jumps `D-29` (`:1438`) → `D-31` (`:1455`).
- **`frontend/src/templates/README.md`** — the file CLAUDE.md tells you to read before adding a page — still says auth is OIDC via Authentik and the password fields are "visual scaffold only". `frontend/src/templates/v1/views/auth/AuthForm.tsx:73` POSTs email+password to `/api/v1/auth/{login|register}`.
- **`frontend/CLAUDE.md:10`** lists React Hook Form as the form-state owner (D-32). It is **not in `frontend/package.json`** and has zero imports; all forms hand-roll `useState`.
- **D-33 RSC-first is inverted in practice** — 65 files carry `"use client"`; every catalogue and detail view is a full client component; `next.revalidate` and `cache: 'no-store'` appear **nowhere** in `frontend/src`.

---

## 5. Correctness bugs — ranked by blast radius

**1 · A failed COMMIT is silently swallowed on every authenticated domain write.**
`internal/modules/tenant/middleware/require_tenant.go:60` — `_ = tx.Commit(ctx)`. This runs in a `defer` **after** the handler has already written its status and body. If the commit fails (deadlock, connection reset over the `host.docker.internal` NAT, constraint deferred to commit), the client sees `201 Created` and the row does not exist. Every route wrapped in `authTenant` (`cmd/api/main.go:210-212`) — i.e. all eleven domain modules — is exposed. **Fix requires restructuring: commit before writing the response, or buffer the response.**

**2 · `imageSrv.Shutdown()` is missing.**
`cmd/worker/main.go:417-419` shuts down `scheduler`, `heavySrv`, `lightSrv` — the image server started at `:319` is never drained. On SIGTERM its in-flight `media:process_image` tasks are killed mid-decode. During a comic zip import that is `IMAGE_CONCURRENCY` pages lost per restart, surfacing as silently-incomplete chapters. **One line.**

**3 · No brute-force protection on `/auth/login`.**
Covered in §3.1. The limiter is written and unused. For an internet-facing single-VPS deployment this is the highest-severity *security* gap in the tree.

**4 · The media worker will break at RLS cutover.**
Covered in §4.1. Not a bug today; a guaranteed outage the moment `DATABASE_URL` flips to `portal_app`.

**5 · `comic.SaveProgress` skips the published-or-owner gate.**
`internal/modules/comic/service.go:332-350` validates chapter↔comic and page↔chapter membership but never calls `s.GetComic(ctx, userID, comicID)` — the check its sibling `ReaderPagesVisible` (`:301-310`) performs for exactly this reason (SPEC-02 P0.5 draft-invisibility). The route is mounted without an owner guard (`comic/module.go:86`). Impact is an existence oracle over other users' unpublished comics, plus junk rows; RLS would neutralise it but RLS is inert.

**6 · `account/api.HasPermission` unconditionally returns `false`.**
`internal/modules/account/api/api.go:67-74` — `return false` with a "deferred until cmd/api wiring is in place" comment. Harmless today because `account/api` has **zero cross-module importers**, but it is a fail-open-looking trap: the first module to use it silently denies everything.

**7 · `media/api.SignedURL` returns `("", nil)`.**
`internal/modules/media/api/api.go:126-128` — returns empty string **and no error**. A caller gets a valid-looking empty URL. `movie`/`music` are its intended consumers.

**8 · Rollback errors discarded in 4 places.**
`internal/platform/db/db.go:85,102,112` and `require_tenant.go:57`. Lower severity than #1 (a failed rollback still aborts) but it hides connection death.

**9 · `/calendar` and `/weather` are not auth-gated.**
`frontend/src/middleware.ts:40` matches `/`, `/login`, `/register`, `/upload`, `/library/:path*`, `/bank/:path*`, `/people/:path*` — the other two `(app)` routes are absent.

**10 · `make test` fails.** §3.1. Nobody can run the suite as documented.

---

## 6. Recommended order

Opinionated, and sequenced by what unblocks what.

### Tier A — blocks other work (do first, ~1 week)

1. **Fix bug #1 (silent commit failure) and #2 (`imageSrv.Shutdown`).** #2 is one line. #1 is the only thing in this document that can lose data a user believes was saved. Nothing else should ship on top of it.
2. **Repair `.env.example`** (§4.3). A clone that cannot boot invalidates every downstream instruction, including your own future self after a reinstall.
3. **Create `internal/platform/server/`** — the package `MODULES.md:48` reserved. Move `writeJSON` (13 copies), `writeProblem` (11), `encodeCursor` (9) into it, and make `writeProblem` the *only* error writer, retiring the 5 `writeErr`/`writeError` variants. This is the precondition for the RFC 7807 DoD and it shrinks every handler you are about to test.
4. **Land the first `httptest` suite.** Pick `comic` (deepest, most routes) and `bank` (best service coverage, worst HTTP coverage). Assert the three cross-cutting rules no test currently checks: 404-not-403 on cross-owner access, RFC 7807 shape, idempotent DELETE. **Zero handler tests is the single largest quality risk in the repo** and it blocks confident refactoring of everything below.

### Tier B — closes the tenancy chain (the largest coherent unit of unfinished work)

5. **Give the media worker a tenant scope** (`RunInUserTenant` in `media.Deps`, mirroring journal/notify/people) and switch `InsertVariant` off its subquery workaround. This is the actual, undocumented prerequisite for the cutover.
6. **Apply `0020` if it has not been applied** (verify against the live cluster — the tree cannot tell you), then **write the RLS isolation test** ADR-07:104 demands. It needs a real Postgres connection, which is also the L2 integration layer `TEST-PLAN.md:57` promised and never got. Build it once; both obligations close.
7. **Cut over `DATABASE_URL` to `portal_app`.** Until this happens, all 30 migrations' worth of RLS is decorative and the isolation guarantee the architecture is built on has never been exercised.
8. **Then, and only then, decide whether you need steps 5–7 of ADR-07** (`switch-tenant`, `/admin/organizations`, per-tenant `user_roles`, `/t/{org}` prefix, `cmd/sysjobs`). At n=1 with one personal org, **my recommendation is to defer all of them and write that down as an ADR-07 update** — they are multi-user machinery for a single-user system, and leaving them as unchecked boxes makes the ADR read as failure rather than as scope.

### Tier C — pays down debt (do in the gaps)

9. **Rewrite `docs/reference/events.md`** (§4.2). It is the one cross-module contract with no CI gate, it is wrong in 8 rows, and it is the file you will consult when wiring the next consumer.
10. **Delete or rewrite the seven stale module `README.md` "Open work" sections** (§4.5). They cost more than they give: each one will send you to write code that exists.
11. **Fix the OpenAPI gap** — add `/movies*`, `/tracks*`, `/stories*`, `/me/organizations`, the 4 import paths, and `/time` to `shared/openapi.yaml`. The CI gate is real but it only guards what is already in the spec.
12. **Retire `docs/product/backlog.md` and `analysis/facebook-comparison.md`**, or stamp them archived. Both are pre-ADR-08 Facebook-parity thinking and both actively misdescribe the current system. `vision.md` + the SPECs are the live yardstick.
13. **Add `--passWithNoTests` or a first frontend test** so `make test` passes, and add `pnpm test` to the CI frontend job.

### Tier D — new feature work, in priority order

14. **Finish movie / music / story, or delete them.** They are the clearest waste in the tree: 4,600 LOC of backend with no spec, no test, no UI, and publish events nobody consumes. Two honest options — (a) write the thin frontend (they were built to mirror comic; `SPEC-02:475-482` targeted "≥80 % shape reuse for movie") plus the three `journal:stream_*` subscriptions so publishing shows up on the home stream; or (b) revert them and re-add when a spec exists. **Do not leave them in the current state** — dead endpoints rot.
15. **Rate-limit `/auth/login`** (§3.1). Small, and the limiter already exists.
16. **Prove the restore drill.** `SPEC-09:146-150` is explicit that unexercised restore code does not count as done, and this is the one failure mode with no recovery path. Run it against a real nightly dump and record the date.
17. **SPEC-05 P1.5 photo attachments** — the highest-value single P1 left. `asset_ids` is already in the schema, the media pipeline is done, and it turns the journal from text-only into the thing `vision.md` describes.
18. Then, by value: SPEC-06 P1.5 on-this-day · SPEC-04 P1.3 notification preferences · SPEC-03 P1.11 monthly report · SPEC-09 P1.7 takeout · SPEC-02 P1.8 bookmarks.

**Explicitly not recommended now:** MFA/TOTP (D-27/D-28 gate it on *real-bank* credentials, which are deferred; the manual ledger holds none — `briefs/03:16-18`), full-text search (no corpus yet at n=1), and Web Push (P1.1 — the 60 s poll is adequate for one user).

---

## 7. Explicitly out of scope — not a to-do list

The following are **deliberately deferred** by ADR-01 as re-affirmed by ADR-08:80-91 and `briefs/04-deferred.md`. They appear nowhere above and their absence is not a gap:

- **Social layer** — posts, newsfeed ranking, comments/reactions, friend graph, messaging, groups, pages, stories, follow. *(The ~1,450 LOC of unimported `components/{social,post,comment,blog,profile}/*` are unported Blade scaffolding for this, not work-in-progress.)*
- **Advanced social** — D-35 "For You" feed, hashtags/mentions, feed ranking.
- **Creator economy** — D-40 payouts, Stripe Connect, subscriptions, ads.
- **Marketplace** — listings, seller chat, shops.
- **ML safety / trust & safety** — D-38 classifiers, CSAM quarantine, moderation dashboard.
- **LiveKit / mediamtx** — D-36 live streaming, D-39 group calls. Compose profiles `--profile live` and `--calls` stay off (they are not even defined in `docker-compose.yml`).
- **The 5-service observability stack** — D-8, ADR-03:33-37. `--profile observability` stays off. *(Note: ADR-07:116 wanted it landed in the same sprint as tenancy — that coupling should be explicitly dropped, not silently ignored.)*
- **Real bank integration** — ADR-08:38-41. Only the manual ledger (SPEC-03) is in scope.
- **Native mobile apps, i18n, APNS/FCM** — D-5, D-6.
- **`template-main/`** — reference material, not code.

---

*Generated 2026-08-25 from the tree at commit `ec61e2b`. Applied-migration state and runtime behaviour were not verifiable from the repository and are flagged as such throughout.*
