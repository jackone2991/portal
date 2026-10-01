# SPEC-02 — Comic Vertical (end-to-end)

**Status:** current, rev 15 · **Drafted:** 2026-07-05 · **Last verified:** 2026-10-01
**rev 14 (2026-08-15):** **P1.10 external-source sync imports incrementally per chapter.** A whole-comic sync no longer scrapes-all-then-imports-once (slow, all-or-nothing, opaque). The scraper discovers chapters and processes them in batches of **`SCRAPER_BATCH_SIZE` (default 1 = each chapter imports the moment it's scraped)**, each batch a **separate `comic_imports` job** requested via the new internal `POST /internal/comic/sync-batch {source_id}` → chapters appear in the UI one-by-one as scraped (verified: `scraped`↔`chapters` track 1:1). Discovery itself retries `BATCH_RETRIES`× (Cloudflare can challenge the index page too). **Stop button (0030):** `POST /sync-sources/{id}/cancel` → the api signals the scraper (`/cancel`, in-memory set) and marks the source `cancelled` (new status); the scraper checks before each chapter and stops (the in-flight chapter finishes), keeping chapters already imported. UI shows a "⏹ Ngừng" button while syncing (and an "Đã ngừng" badge after). A batch that yields zero images (Cloudflare re-challenge) is **retried up to 3×** (`SCRAPER_BATCH_RETRIES`); chapters still failing are collected and shown on the source row as **"⚠ N chương lỗi: …"** (last_error). Migration `0028` gains `total_chapters`/`scraped_chapters` (0029) so the source shows **live "Cào X/Y chương (%)"** across all batches (progress → source, not a per-import job). New internal endpoints `sync-batch`/`sync-progress`(source_id)/`sync-finalize`; `sync-callback` now enqueues one batch's import only. **Fix:** `sync-batch` runs outside `authTenant`, so `RequestSyncBatch` creates the batch import via an API-side `runInUserTenant` (comic_imports.tenant_id default). **Verified:** 22-chapter sync → 2 batches (20+2), batch 1's 20 chapters imported *before* batch 2 finished, 636 pages, pages render. (Scraper host-mode still required — Cloudflare blocks the container; see [scraper/README.md](../../../scraper/README.md).)
**rev 13 (2026-08-15):** **P1.10 external-source sync (full UI feature).** A comic can bind to external source URLs (`comic_sync_sources`, migration `0028`, tenant_id+RLS). Clicking "Đồng bộ" → `TriggerSync` creates a comic-level import job (reusing P1.7) and calls a **new Python scraper service** (`scraper/`, FastAPI + SeleniumBase undetected-Chrome on xvfb); the scraper discovers chapters, scrapes image URLs, downloads a folder-per-chapter tree, zips it, uploads to MinIO at `import/{importID}.zip`, and hits the shared-secret `POST /internal/comic/sync-callback` → Go enqueues `comic:import_zip` (the whole optimized import pipeline is reused untouched). Endpoints: `GET/POST /comics/{id}/sync-sources`, `POST /sync-sources/{id}/sync` (202), `DELETE /sync-sources/{id}`, internal `sync-callback`. Wiring: `COMIC_SCRAPER_URL` (nil ⇒ feature off), `COMIC_SYNC_SECRET`; new `scraper` docker service (internal-only, `shm_size` for Chrome). UI: "Nguồn đồng bộ" section on the comic manager (add source, list, Sync button, live scrape+import status). **Verified:** api→scraper→callback→import plumbing end-to-end (error propagates to `last_error`). **Known limit:** the *containerized* Chrome is fingerprinted+blocked by Cloudflare on truyenqq (`title='Just a moment…'`, xvfb-headful and `uc_gui_click_captcha` both fail); the container shares the host IP, so it's a fingerprint problem — run the scraper **on the host** (real Chrome passes) via the recipe in [scraper/README.md](../../../scraper/README.md).
**rev 12 (2026-08-15):** **P1.7 import performance optimized + hardened.** (1) `media:process_image` moved off the concurrency-1 `heavy` queue onto a new **`image` queue** with its own pool (`IMAGE_CONCURRENCY`, default 3) — variants transcode in parallel while video stays serial (SPEC-01 P0.1 OOM guard preserved: image decodes are dimension-capped). (2) **Parallel ingest** (`IMPORT_INGEST_CONCURRENCY`, default 4): `RunImport` now (A) creates all chapters, (B) reads the zip serially but fans the DB+storage ingest out to a bounded pool (I/O-bound: MinIO PUT + tenant tx per image) — measured ingest **5.8 → ~22 img/s**, no longer starving the transcode pool; then (C) drains, paging **each chapter the moment its images are ready** so pages commit incrementally (crash-resilient) and `succeeded` climbs live. (3) New `mediaapi.AssetStatuses(ids)` → **one** `WHERE id = ANY(...)` query per poll round instead of a `GetAsset` (one tenant tx) per asset. (4) `media.UploadSource` skips the temp-file spool for `io.ReadSeeker` bodies. **Fixes found in load-testing the 1.68 GB / 9129-image archive:** (a) the import task is enqueued *outside* the request pg tx, so the worker could dequeue before `SetImportUpload` committed → `RunImport` now waits briefly for `upload_ref` instead of failing (`no upload`); (b) the API's global 30 s request timeout cancelled the multi-GB API-proxied upload's S3 PutObject → raised to a generous per-request window (`cmd/api/main.go`, `ReadHeaderTimeout` still guards headers). Transcode throughput is I/O-bound on dev's bind-mounted MinIO (~6 img/s, few cores used); it scales further on prod storage. *(Tradeoff: `succeeded` stays 0 during the up-front parallel ingest, then climbs during the drain.)*
**rev 4 (2026-08-07):** reader-experience redesign added as phased R1–R4 (end of §5). Reader prefs reframed as client-side (D-32), superseding P1.6's server pref.
**rev 5 (2026-08-07):** R2–R4 implemented — page slider + chapter menu + prev/next chapter, double-page + RTL (migration `0024_comic_reading_direction`), preloader + seamless webtoon, zoom. `reading_direction` is live end-to-end.
**rev 6 (2026-08-07):** R5 implemented — full-screen immersive reader (breaks out of the app shell into a fixed overlay; webtoon scrolls inside it), `prefers-reduced-motion`, and a `?` keyboard-shortcut help.
**rev 7 (2026-08-07):** `/library/comic` list redesigned (renamed sidebar "Truyện tranh", dropped the toolbar strip, "new comic" is now a grid tile + dialog); migration `0025_comic_user_write_grant` lets every `user` create & publish their own comics (see P0.2).
**rev 8 (2026-08-07):** owner CRUD manager implemented on `/library/comic/[id]` — edit metadata + cover, publish/unpublish, delete, chapter add/rename/reorder/delete, and page upload/reorder/delete. Page & cover images go through the SPEC-01 pipeline via `lib/media-upload.ts` (verified: upload → ffmpeg WebP variant → thumbnail renders). Backend CRUD was already complete; this closes the frontend gap.
**rev 11 (2026-08-08):** import task now enqueued with `asynq.Timeout(12h)` + `MaxRetry(0)` — a long whole-comic import used to exceed asynq's ~30-min lease, get its context cancelled, and re-run → duplicate chapters. **rev 10 (2026-08-07):** P1.7 extended to **whole-comic (multi-chapter) zip import** — migration `0027_comic_import_comic_level` (nullable `chapter_id`), endpoint `POST /comics/{id}/imports`, worker groups images by top-level folder → one chapter per folder (natural-sorted, handles a wrapper folder); limits raised to 3 GB / 20000 entries (the entry cap is `importMaxEntries` in `comic/import.go`, since raised to 100,000; that constant, not this note, is current). Frontend "Nhập bộ từ ZIP" on the comic manager (comic-level) alongside per-chapter "Nhập ZIP". Verified on a real 1.68 GB / ~9100-image / ~100-chapter archive.
**rev 9 (2026-08-07):** **P1.7 zip chapter import implemented (full server-side).** Migration `0026_comic_imports` (persisted job + per-file report, tenant_id+RLS). Endpoints `POST /chapters/{id}/imports`, `PUT /imports/{id}/zip` (API-proxied — dev MinIO presign isn't browser-reachable; 500 MB cap), `GET /imports/{id}` (poll). Worker task `comic:import_zip` (default queue) spools the zip → unpacks (guards: 300 entries, image-only, no traversal, zip-bomb ratio) → natural-sort → `mediaapi.IngestImage` per image in a committed tenant tx → poll assets ready → create pages. Frontend `lib/comic-import.ts` (create→upload→poll) with "Nhập chương từ ZIP" (new chapter) + per-chapter "Nhập ZIP". Verified: 4-image zip `002,10,1,003` → done, natural order `1,002,003,10`, variants render. *(Note: found+fixed a latent pgx bug — under `QueryExecModeExec`, a jsonb param passed as `[]byte` is sent as bytea and rejected; pass the json as a string. The audit logger has the same latent bug.)*
**Module:** `comic` · **Depends on:** SPEC-01 (image kind)
**Downstream consumers:** SPEC-07 P1.6 (comic leg, `comicapi.Continue`), SPEC-04 (`comic:published` bell)
**Upstream:** brief 02 (folded into this spec, then deleted — `git show ea100d8:docs/product/briefs/02-comic-vertical.md`) · **Refs:** feature-inventory.md §7, frontend.md Phase 4
**Role:** reference implementation of the *media → domain vertical* pattern
(migration → `query/` → repository → service/handler → `MountHTTP` → real view),
to be copied by movie/music/story.

---

## 1. Problem statement

All four domain verticals are skeletons; `/library/comic` and the reader views render
placeholders. Comic is the chosen first vertical: it is on the entertainment axis the
owner prioritized, it is the cheapest proof of the vertical pattern (a reader over
SPEC-01's image variants), and the frontend shells already exist to be replaced.
Until one vertical is real, every future vertical estimate is a guess.

## 2. Goals

1. One real comic is creatable, publishable, and readable end-to-end with progress
   resume — by the dev, on the running stack, with no seed-script hacks.
2. The module lands as the canonical vertical template: a later "build movies"
   session should be mostly mechanical copying.
3. Reader first-page visible < 2 s on a normal connection (uses `medium` variants,
   never originals).

## 3. Non-goals

- Comments, ratings, reactions on comics (social layer, later).
- Follow/subscribe to a series; new-chapter notifications (needs the notification
  module; `comic:published` is emitted anyway and reaches the bell — P1.9).
- Import/scraping from external sources was a v1 non-goal; reversed by rev 13
  (2026-08-15), see P1.10. Still out of scope: scheduled/automatic re-sync, and
  passing anti-bot challenges from the containerised scraper (host mode only).
- Per-comic visibility/ACL: `published` = visible to **all authenticated users** at
  v1. Visibility scoping arrives with the privacy layer. (Recommendation locked
  from brief 02's open question.)
- Automatic page-spread detection (double-page art); RTL and manual double-page
  pairing shipped as R3.
- Offline reading / PWA caching.

## 4. User stories

- As a creator, I create a comic, add chapters, attach uploaded page images in
  order, and publish, so it appears in the library.
- As a creator, I reorder or remove pages before publishing, because upload order
  is rarely final order.
- As a reader, I open a published comic, scroll a chapter vertically, tap through to
  the next chapter, and when I return days later I resume within one page of where
  I stopped.
- As a creator, my draft is invisible to everyone else — including in listings,
  detail fetches, and reader payloads.
- As a reader on mobile data, pages ahead of me preload so scrolling never stalls,
  but the app doesn't download the whole chapter up front.

## 5. Requirements

### P0.1 — Entities + CRUD

Comic: `title` (required, ≤200 chars), `description` (optional), `cover_asset_id`
(optional; must reference a **ready image** asset owned by the creator — validated
via `mediaapi`, no cross-module FK), `status` `draft|published`. Chapters: ordered
by explicit `sort_order` int (gaps allowed). Pages: each is `{asset_id, sort_order}`;
asset must be a ready image owned by the creator, validated via `mediaapi` at write
time. A page's asset may be referenced by only one page (unique index) to keep
delete semantics sane. A page is removed via `DELETE /api/v1/pages/{id}` (§7),
which drops the `comic_pages` row only — the referenced media asset keeps its own
lifecycle (assets are never deleted as a side effect of page deletion).

**Owned by the creator** means the asset's owner, as `mediaapi` returns it
(`assets.owner_id`), equals `comics.owner_user_id` of the target comic, whoever the
caller is (the owner or a `comics:write:any` editor). Assets created by zip import
(P1.7) or sync (P1.10) are ingested via `mediaapi.Ingest` with
owner = `comics.owner_user_id`, never the requesting editor. *(Code follow-up: the
shipped `RunImport` ingests with owner = `comic_imports.owner_user_id`, i.e. the
caller who created the job, so an editor's import on another creator's comic
creates assets that the P0.1 page check then rejects, and the chapter gets no pages.)*

**Acceptance criteria.**
- Given a page create with a nonexistent, non-image, non-ready, or not-owned
  asset id, then 422 Problem `comic/invalid-page-asset` and no row.
- Given editor E (`comics:write:any`) attaching an asset E owns to creator C's
  comic, then 422 `comic/invalid-page-asset` (or `comic/invalid-cover-asset` for a
  cover) and no change.
- Given an editor runs a zip import on C's comic, then every created asset's owner
  is C.
- Given a comic create/update whose `cover_asset_id` is nonexistent, non-image,
  non-ready, or not owned by the creator, then 422 Problem
  `comic/invalid-cover-asset` and no change (same `mediaapi` validation path
  as pages — previously declared but untested and untyped).
- Given pages inserted with sort_order 10,20,30, when 20 is deleted, then reader
  order is stable (10,30) with no renumbering required.
- Given a chapter delete, then its pages rows are removed (cascade) but the
  underlying media assets are **not** deleted (assets have their own lifecycle).
- Title > 200 chars or empty → 422.

### P0.2 — Publish flow + RBAC

Permissions. **Reads** go through `RequirePermission`. **Every mutation on an
existing comic/chapter/page** goes through `RequireOwnerOrPermission(engine,
"comics:write:any", extractComicOwner)` — a bare `comics:write:own` grant only
lets the caller touch **their own** content; editing someone else's requires the
elevated `comics:write:any` moderation grant. Owner-only creation (`POST /comics`)
and own-listing (`GET /comics/mine`) keep plain `RequirePermission` on
`comics:write:own`. Destructive/elevated variants: `DELETE /comics/{id}` uses
`comics:delete:any` as the elevated code; `POST /comics/{id}/publish` /
`/unpublish` use a new elevated `comics:publish:any`, and the owner additionally
needs `comics:publish:own`, enforced by the chained check in the table below. Unlike
SPEC-01's `assets:delete:own`, this `:own` code is not merely documentary.
**Who can create (rev 7, 2026-08-07):** migration `0025_comic_user_write_grant`
widened `comics:write:own` + `comics:publish:own` from `creator` to the base `user`
role — every authenticated user creates & publishes **their own** comics (life-OS
direction). `:any` write/publish moderation stays `editor`, and
`comics:delete:any` stays `admin`. So the "Thêm truyện" tile on `/library/comic`
works for all users.
*(2026-07-10 reconciliation: codes follow the 0003 catalog shape — `read|write|
delete` plus the sparing verb `publish`, scope `own|any` only. The earlier
`comics:read:published` used a content state as a scope token: it parses, but the
matcher only special-cases `own`/`any`, so a house-style `comics:read` grant would
never satisfy it — "published-only" is endpoint semantics, not a permission scope.
An `all-via-RequirePermission` model on `comics:write:own` was rejected: `:own`
does not check ownership by itself, so it would let any grant-holder edit any
creator's comic while giving admins no moderation path — hence the owner-or-elevated
rework above.)*

| Action | Permission |
|---|---|
| read published | `comics:read` |
| read own drafts | implied by owner check on `comics:write:own` |
| create comic (`POST /comics`), list own (`GET /comics/mine`) | `comics:write:own` |
| update / reorder / add pages / delete a chapter on an existing comic or chapter | owner, **or** `comics:write:any` — `RequireOwnerOrPermission(engine, "comics:write:any", extractComicOwner)` |
| publish / unpublish (`POST /comics/{id}/publish`, `/unpublish`) | `RequirePermission("comics:publish:own")` chained before `RequireOwnerOrPermission(engine, "comics:publish:any", extractComicOwner)`. An owner passes only while holding `comics:publish:own`; an editor passes the first check because the matcher lets an `:any` grant satisfy the same code's `:own` check (and every `user` already holds `:own` via `0025`). *(Code follow-up: `cmd/api/main.go` wires only the `RequireOwnerOrPermission` half, so `comics:publish:own` is never checked.)* |
| delete a comic, or delete a page | owner, **or** `comics:delete:any` |

**Seeding (this module's migration ships it, 0003 pattern):** `comics:read` →
`user` role (v1 = all-authenticated readers; not `guest`, unlike
`movies:read`, per the §3 visibility decision); `comics:write:own` +
`comics:publish:own` → `creator` (widened to `user` by
`0025_comic_user_write_grant`, rev 7); `comics:write:any` + `comics:publish:any` →
`editor` (the write/publish-any moderation tier, mirroring `movies:write:any` +
`movies:publish:any`, `0021`); `comics:delete:any` → `admin` (the movies precedent —
`movies:delete:any` is admin-tier). An unseeded code 403s everyone below
superadmin. A wildcard (`comics:*`) covers every scope, per the permission grammar.

Publish validation: a comic may be published only if it has ≥1 chapter and every
chapter has ≥1 page; otherwise 422 `comic/not-publishable` listing the offending
chapters. Unpublish returns it to `draft`.

The invariant is enforced **at publish only**. On a published comic:
(a) a chapter with 0 pages is omitted from the chapter list, the chapter count, the
reader chapter picker and prev/next navigation, and `GET /chapters/{id}/pages` for
it returns 404 `comic/not-found` to non-owners, until it has ≥ 1 page;
(b) adding chapters or pages to an already-published comic (manual add, zip import
or sync) emits nothing: `comic:published` fires only on a publish action (P1.9), and
a creator who wants the bell re-runs `POST /comics/{id}/publish`, which is allowed on
a published comic and is deduplicated by `chapter_count`;
(c) deleting pages or chapters, including via the P0.6 reaper, never
auto-unpublishes; a published comic left with no non-empty chapters simply shows no
chapters. *(Code follow-up for (a): the shipped `GetComic` and `ReaderPages` return
empty chapters, and `chapter_count` counts them.)*

**Acceptance criteria.**
- Given a user without `comics:write:own`, when POST /comics, then 403.
- Given reader R and creator C's draft, when R fetches detail/pages/list, then the
  draft is absent from lists and detail returns 404 (not 403 — don't leak existence).
- Given a comic with an empty chapter, when publish is attempted, then 422 naming
  the chapter.
- Given creator D (holds `comics:write:own` only) mutating creator C's comic, then
  403 if C's comic is published, or 404 `comic/not-found` if it is a draft, and no
  change. `extractComicOwner` returns 404 `comic/not-found` for a draft when the
  caller is neither the owner nor a holder of the endpoint's `:any` code; holders of
  `:any` (editors) resolve drafts normally. *(Code follow-up: the shipped extractor
  resolves the owner regardless of status, so D gets 403 on a draft too.)*
- Given an editor with `comics:write:any` editing another creator's comic, then
  200; and an admin with `comics:delete:any` calling DELETE passes.
- Given an editor with `comics:publish:any`, when `POST /comics/{C}/unpublish` on
  C's published comic, then 200 and status `draft`.
- Given an owner whose effective permissions lack `comics:publish:own`, when
  `POST /comics/{id}/publish` on their draft, then 403 and status unchanged.
- Given a published comic, when the creator adds an empty chapter, then readers do
  not see it in the chapter list, the chapter count or the reader navigation until
  it has a page. (a)
- Given a published comic, when a chapter gains its first page, then no event is
  emitted; a later `POST /comics/{id}/publish` emits one `comic:published` with the
  new `chapter_count`. (b)
- Given a published comic whose last page is reaped by P0.6, then the comic stays
  `published` and shows no chapters. (c)

### P0.3 — Reader (vertical scroll)

Vertical scroll comes first because it is the dominant mode for webtoon-style
reading and the simplest to build; paged and RTL modes follow in the phased
redesign below.

Route `/library/comic/[id]/read/[chapterId]` (client component; catalogue/detail
stay RSC-first per D-33 — "RSC shell" as defined in the specs README Frontend
convention). **Presentation goes through the version-switched template
registry:** the reader view is declared in `TemplateManifest.views`
(`templates/types.ts`), implemented under `templates/v1/views/...`, and the
`app/(app)/library/comic/[id]/read/[chapterId]/page.tsx` route resolves it via
`activeTemplate().views.<x>` — never a version-specific import in `app/` (that
would break the v2 switch). Behavior: pages render top-to-bottom using the `medium`
variant URL; each `<img>` carries width/height from the variant row so layout never
shifts (CLS < 0.1 budget); lazy-load with the next 2–3 pages preloaded via
IntersectionObserver; sticky minimal chrome: comic title, chapter picker,
prev/next chapter; end-of-chapter panel: next chapter or back to detail.

**Acceptance criteria.**
- Given a 40-page chapter on a throttled "Fast 3G" profile, when opened, then page 1
  is visible < 2 s and scrolling to page 5 never shows a layout jump.
- Given the last chapter's end, then the panel offers "back to comic" (no dead end).
- Given a `failed`/deleted page asset, then the reader shows a per-page error tile
  and continues (one bad page never blanks the chapter).

### P0.4 — Reading progress

Table `comic_reading_progress(user_id, comic_id, chapter_id, page_id)`,
PK `(user_id, comic_id)`. **Progress is keyed by `page_id`, not by array position.**
A positional `page_index` silently points at the wrong page the instant a creator
inserts a page before it or reorders the chapter (read to the page at index 1, someone
inserts a page at the front, and resume now lands on a different page). A `page_id` —
with the `ON DELETE SET NULL` FK to `comic_pages` above — always resolves to the exact
page the reader stopped on, or degrades cleanly to the chapter top if that page was
later deleted. Reader upserts debounced: every 10 s while the furthest visible page
changes, plus on pagehide — mirroring the watch-progress convention (frontend.md
Phase 2-3). Every save, the pagehide one included, is a keepalive `PUT` per
SPEC-07 P0.2's **transport rule**; never `navigator.sendBeacon`, which can only
POST against this PUT-only route. *(Code follow-up: the shipped
`frontend/src/lib/comic.ts` still calls `sendBeacon` first.)*
Detail page shows **Continue reading → ch. N, p. M** when progress exists; opening
via Continue scrolls to that page. The server returns only ids
(`ComicDetail.progress = {chapter_id, page_id|null, updated_at} | null`, §7); the
client derives N from the chapter's 1-based index in `ComicDetail.chapters[]` and M
from the page's 1-based index in `GET /chapters/{id}/pages`. Only `page_id` is
stored, so a position "at read time" is not recoverable after a reorder, and the
current position is what Continue actually scrolls to.

**Acceptance criteria.**
- Given reading to ch.2 p.14 then closing the tab, when the comic is reopened via
  Continue, then the view lands within ±1 page of p.14.
- Given progress saved on a page, when the creator later inserts pages before it or
  reorders the chapter, then Continue still lands on the **same page** — because
  progress is keyed by `page_id`, not by array position (regression test for the
  insert/reorder drift).
- Given a progress row whose `page_id` was since deleted (FK SET NULL) or whose
  chapter was deleted, then Continue falls back to the chapter top — or to the first
  chapter if the whole chapter is gone (no crash).
- Progress writes require only authentication on a comic the user can read; a reader
  cannot write progress on another user's draft (404 path already covers this).
- Given a progress write whose `chapter_id` does not belong to the comic, or whose
  `page_id` does not belong to that chapter, then 422 Problem
  `comic/invalid-progress-target` and no row — the FK alone only guarantees the
  page exists in *some* comic, so an unvalidated write could make Continue
  deep-link into a different comic.

### P0.5 — Library + detail pages

`/library/comic`: grid of published comics — cover (`thumb` variant), title, chapter
count, updated date; pagination; replaces the placeholder. Creator additionally sees
a "My comics" tab including drafts with status badges and a Create button.
`/library/comic/[id]`: cover, title, description, chapter list (title + created
date), Continue button, creator-only Edit/Publish controls.

Both views go through the **version-switched template registry**: each is declared
in `TemplateManifest.views` (`templates/types.ts`), implemented under
`templates/v1/views/...`, and the `app/(app)/library/comic/page.tsx` /
`app/(app)/library/comic/[id]/page.tsx` routes resolve them via
`activeTemplate().views.<x>` — never a version-specific import in `app/`.

**Acceptance criteria.**
- Given a published and a draft comic by another creator, a reader opening
  `/library/comic` sees only the published one (cover = `thumb` variant, title,
  chapter count, updated date).
- The creator's "My comics" tab lists both with status badges; a reader never sees
  drafts.
- A result set of >1 page paginates with no duplicates or skips, provided no comic
  in the set is modified between page fetches (an edited comic legitimately moves
  to the head).
- The pre-existing placeholder component is gone from the route.
- The detail page shows **Continue** iff progress exists (P0.4).

### P0.6 — Asset-deletion coupling (no dangling references)

comic stores `comic_pages.asset_id` and `comics.cover_asset_id` with **no
cross-module FK** (module boundary). SPEC-01 P0.3 is a *hard* delete — it removes the
media rows and every storage object — so absent a signal, a media-side delete leaves
`comic_pages` rows pointing at a nonexistent asset (reader hits the P0.3 error tile,
but the row is orphaned **forever** with no way to reap it) and a stale
`cover_asset_id` that renders a broken cover. The reader's graceful degradation is a
display fix, not a data fix.

comic subscribes to the **`media:asset_deleted` `{asset_id, owner_user_id}`** event
(SPEC-01 must emit it on delete — see `docs/reference/events.md`; the event has
multiple consumers, so delivery is via the `platform/events` fan-out described
in events.md "Delivery mechanics" — comic handles its own consumer task type,
never the raw event task) and, idempotently:
deletes any `comic_pages` row whose `asset_id` matches (each deleted page's
`comic_reading_progress.page_id` FK then `SET NULL`s itself — see P0.4), and sets any
`comics.cover_asset_id = NULL` that matches. This is the **soft cross-module cascade**
the modular-monolith pattern prescribes: async event, never a foreign key. It is also
the reference pattern for movie/music/story, which hold the same media references.

**Acceptance criteria.**
- Given a page whose asset is deleted media-side, when the event is handled, then the
  `comic_pages` row is gone and the chapter's remaining pages keep stable order.
- Given a cover asset deleted media-side, then `cover_asset_id` becomes NULL and the
  library card falls back to the placeholder (no broken image).
- Handling is idempotent and order-tolerant: redelivery of the same `asset_id`, or an
  `asset_id` this comic never referenced, is a no-op (best-effort, mirrors the
  media→comic decoupling; a missed event is recoverable by a future reconcile sweep).

### P1 — nice to have

- **P1.6 Reader modes** — *reframed as the R1–R4 reader redesign (end of §5,
  2026-08-07); single/double-page + keyboard/tap navigation land there.*
  **Correction:** reading mode, fit, brightness and quality are **client-side** UI
  prefs (Zustand `persist`, frontend/CLAUDE.md D-32) — the `reader_mode` column +
  `GET/PUT /api/v1/comics/reader-prefs` originally sketched here is **dropped** (no
  cross-device sync in v1; accepted trade-off). The only server-side reader field is
  `comics.reading_direction`, a property of the *work* (R3), not a per-user pref.
- **P1.7 Zip import (per-chapter and whole-comic)** — *implemented revs 9–12.* Flow:
  1. The client creates a job: `POST /api/v1/chapters/{id}/imports` (per-chapter:
     pages go into that existing chapter) or `POST /api/v1/comics/{id}/imports`
     (whole-comic: one chapter per zip folder). 201 `ImportJob`, status `pending`;
     `comic_imports.owner_user_id` records the caller, the only user who may upload
     to and poll the job.
  2. The client uploads the zip body with `PUT /api/v1/imports/{id}/zip`. The API
     spools it (API-proxied: dev MinIO presign is not browser-reachable), stores it
     at `import/{import_id}.zip` with **no `assets` row** (it is not a media asset,
     so SPEC-01's asset-upload path does not apply), sets `upload_ref`, status
     `uploaded`, and enqueues `comic:import_zip {import_id}` (events.md).
  3. The client polls `GET /api/v1/imports/{id}` for `status`
     (`pending|uploaded|processing|done|failed`), `total`, `succeeded`, `failed`,
     the per-file `report` (capped at 500 entries, failures first) and a job-level
     `error`.

  The worker **spools the object and deletes it after processing**. It creates one
  media asset per image by calling SPEC-01 P1.3 `mediaapi.Ingest(..., origin="import")`
  (the `origin='import'` mark makes `media:asset_ready` consumers suppress the
  flood — SPEC-01 P1.2) with owner = `comics.owner_user_id` (P0.1),
  then pages in **filename natural-sort order**. **Ready-race rule:** P0.1 requires a
  page's asset to be `ready` at write time, but ingested assets start `processing`
  (variants come from `media:process_image` on its own `image` queue,
  `IMAGE_CONCURRENCY`, default 3, rev 12). So the worker ingests every image up front
  (`IMPORT_INGEST_CONCURRENCY`, default 4), bulk-polls `mediaapi.AssetStatuses` every
  ~2 s, and pages each chapter as soon as its assets resolve, creating pages only for
  assets that reached `ready`. **Poll timeout = max(2 min, entry_count × 5 s), capped
  at 11 h (under the 12 h task lease, rev 11), for both modes;** assets still pending
  at the deadline are reported as failures. Failures produce per-file report rows;
  the chapter gets the pages that succeeded.

  **Guards (both modes share the shipped caps in `comic/import.go`; that file, not
  this table, is authoritative for the numbers):**

  | | Per-chapter (`POST /chapters/{id}/imports`) | Whole-comic (`POST /comics/{id}/imports`) |
  |---|---|---|
  | Zip size | ≤ `importMaxZipBytes` (16 GiB); larger → 422 `comic/validation` at `PUT /imports/{id}/zip`, nothing stored | same |
  | Image entries | ≤ `importMaxEntries` (100,000); more → job `failed` with `error` | same |
  | Compression ratio (zip-bomb guard) | an image entry with a ratio > `importMaxRatio` (100:1) is skipped unextracted and unreported (below); the upload itself is never refused for it | same |
  | Folders | ignored: every image is flattened into the target chapter, ordered by base filename (natural sort) | each distinct folder path becomes one chapter titled by its last segment, in natural-sort order; a wrapper folder is therefore transparent (`wrapper/ch1/*` → chapter `ch1`), and loose root-level images form one chapter |
  | Chapter placement | the existing chapter; new pages append after its current pages | `sort_order` from the chapter number in the title (numberless titles append after `MAX(sort_order)`); a title that already exists **with** pages is skipped, one that exists **empty** is refilled in place |

  Both modes: entries are selected as images by extension; directory entries, names
  containing `..`, absolute paths and `__MACOSX/` entries are skipped and never
  extracted. **Zip-bomb guard:** an image entry whose uncompressed size divided by
  its compressed size — both as declared in the zip directory, integer division,
  entries with a compressed size of 0 not checked — exceeds `importMaxRatio` (100)
  is skipped the same way, before extraction. A skipped entry is silent: it gets no
  report row, counts in none of `total`, `succeeded` or `failed`, and does not count
  toward `importMaxEntries`; it never fails the job by itself. Every extracted entry
  is read through a 60 MiB cap (above SPEC-01's 50 MB image limit), so an entry whose
  real data outgrows its declared size cannot exhaust the worker: it fails on read or
  at ingest and is reported failed. An image the media pipeline cannot process fails
  in the per-file report. A zip that is unreadable, or has no valid image left after
  the skips (for example, one whose every image is a bomb entry), leaves the job
  `failed` with `error`; none of this is a 4xx, because the guards run in the worker
  after `PUT /imports/{id}/zip` has answered.
  **Queue (target):** a 12 h job must not share the light server's `default`
  queue with notify (SPEC-04 P0.2 step 4), so `comic:import_zip` runs on a
  dedicated **`bulk`** queue served by its own `asynq.Server` (Concurrency 1),
  shared with SPEC-09's `ops:takeout`. *(Code follow-up: `comic/import.go` still
  enqueues on `default`.)*

  **Acceptance criteria.**
  - Given a per-chapter zip containing `a/2.png`, `b/1.png`, then both images become
    pages of the target chapter in order `1.png`, `2.png` (folders ignored).
  - Given a whole-comic zip `wrapper/ch1/*`, `wrapper/ch2/*`, then two chapters
    `ch1`, `ch2` in natural order.
  - Given a zip over `importMaxZipBytes`, when `PUT /imports/{id}/zip`, then 422
    `comic/validation` and no object stored.
  - Given a zip entry named `../x.png`, then it is skipped and nothing is written
    outside the import.
  - Given a zip of three ordinary images plus one image entry compressed at more
    than 100:1, then the job is `done` with three pages and `total` = 3, and the
    bomb entry is never extracted and has no report row; given a zip whose only
    image is such an entry, then the job is `failed` with a job-level `error` and
    no pages.
  - Given user U polls another user's import id, then 404 `comic/not-found`.
- **P1.8 Bookmarks**: per user, per page; list on the detail page. Needs a
  `comic_bookmarks(user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  page_id uuid NOT NULL REFERENCES comic_pages(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (user_id, page_id))`
  table (tenant-scoped per the specs README Tenancy convention) and `PUT/DELETE
  /api/v1/pages/{id}/bookmark` + `GET /api/v1/comics/{id}/bookmarks`; **schema +
  endpoints to be added to §6/§7 when scheduled.**
- **P1.9 Events**: on publish, emit **one** `comic:published`
  `{comic_id, owner_user_id, title, chapter_count}` per publish action — never one
  per chapter (a whole-comic import holds ~100 chapters), and nothing when a
  published comic gains chapters or pages (P0.2 (b)). Its consumer is
  `notify:on_comic_published` (SPEC-04 §7): one bell entry with
  `dedup_key = comic_id + ":" + chapter_count`, so a repeat publish of an
  unchanged comic is silent and a publish after new chapters notifies once. It is
  **not projected into the stream** (`0034` removed the comic stream rows: a
  publish is a library event, not a moment). `comic:chapter_deleted`
  `{comic_id, chapter_id, owner_user_id}` is emitted on a **chapter delete** and
  **once per chapter on a comic delete**; since `0034` it is **emit-only** (no
  consumer). Both rows live in events.md.

  **Acceptance criteria.**
  - Given a comic with 100 chapters published at once, then exactly one bell
    notification; given it re-published unchanged, then none.

- **P1.10 External-source sync (revs 13–14)** — *implemented 2026-08-15; code
  comments and migrations `0028`–`0030` call it "P1.8".* A comic binds to external
  source URLs (`comic_sync_sources`, §6) and pulls chapters from them through the
  P1.7 import pipeline. Flow:
  1. The owner (or a `comics:write:any` editor) adds a source with
     `POST /comics/{id}/sync-sources {source_url, chapters_hint?}`. The URL must be
     absolute http(s) and pass the SSRF guard (`sourceguard.go`: private and
     internal addresses are always rejected; `COMIC_SOURCE_ALLOWLIST` restricts
     hosts when set), else 422 `comic/validation`. The row's `owner_user_id` is the
     caller.
  2. `POST /sync-sources/{id}/sync` (**TriggerSync**, 202) re-runs the SSRF guard,
     resets progress, sets `last_status='syncing'` and calls the scraper service
     with `{source_id, owner_id, source_url, chapters, existing}`. `existing` lists
     the comic's chapter titles so an incremental sync skips them unless
     `chapters_hint` forces a list. A second trigger while `syncing` is 422
     `comic/validation`, unless the source has been `syncing` for over 15 min
     (a lost callback must not strand it).
  3. The scraper discovers chapters (discovery retried like a batch) and works in
     batches of `SCRAPER_BATCH_SIZE` chapters (default 1). Per batch it calls
     `POST /internal/comic/sync-batch` → one new comic-level `comic_imports` job +
     its upload key `import/{import_id}.zip`; it uploads the folder-per-chapter zip
     there and calls `POST /internal/comic/sync-callback {import_id, owner_id, ok}`,
     which sets `upload_ref` and enqueues `comic:import_zip {import_id}`. It reports
     overall progress with `sync-progress {scraped, total}` (→ `scraped_chapters`,
     `total_chapters`) and ends with `sync-finalize {ok, failed}` (→ `last_status`
     `done`/`failed`, the failed-chapter summary in `last_error`).
  4. A batch that yields 0 images (for example a Cloudflare re-challenge) is retried
     up to `SCRAPER_BATCH_RETRIES` times (default 3); chapters still failing are
     collected into `last_error` and the sync continues with the next batch.
  5. **Cancel:** `POST /sync-sources/{id}/cancel` is accepted only while
     `last_status='syncing'` (otherwise 422 `comic/validation`). The API signals the
     scraper (`/cancel`, an in-memory set); if the scraper refuses, the call fails
     and the source is unchanged. On success the source becomes `cancelled`; the
     scraper checks before each chapter, so the in-flight chapter finishes, and
     chapters already imported are kept.

  **Config:** `COMIC_SCRAPER_URL` (unset ⇒ feature off: every sync-source endpoint
  returns 404 Problem `comic/sync-disabled`; *code follow-up: the shipped API still
  serves source list/create/delete and answers a trigger with 500*);
  `COMIC_SYNC_SECRET` (the internal-endpoint secret, §7); `COMIC_SOURCE_ALLOWLIST`;
  scraper-side `SCRAPER_BATCH_SIZE` (default 1) and `SCRAPER_BATCH_RETRIES`
  (default 3). The containerised scraper is blocked by Cloudflare on some sources;
  run it on the host per [scraper/README.md](../../../scraper/README.md).

  **Acceptance criteria.**
  - (a) Given batch size 1, then each chapter appears in the chapter list as soon as
    it is scraped, and `scraped_chapters` counts imported chapters 1:1.
  - (b) Given a sync cancelled mid-run, then `last_status='cancelled'`, the
    in-flight chapter finishes, and chapters already imported are kept.
  - (c) Given a batch that still yields 0 images after `SCRAPER_BATCH_RETRIES`
    attempts, then its chapters are listed in `last_error` and the sync continues.
  - Given a re-sync of an unchanged source, then no chapter is duplicated (titles
    already present with pages are skipped, P1.7).
  - Given user U calling sync, cancel or delete on a source whose `owner_user_id` is
    not U, then 404 `comic/not-found`.

### P2 — future considerations

- Review/approval publish workflow (mirror story module when it lands).
- RTL reading direction + double-page spread pairing — shipped as R3 (2026-08-07)
  of the reader redesign below, with the `comics.reading_direction` column.
- Per-comic visibility (unlisted/link-only) once the privacy layer exists.

### Reader experience — phased redesign (R1–R4)  *(rev 4, 2026-08-07)*

P0.3 shipped a single-mode vertical-scroll reader. This redesign turns it into a
real comic/manga reader **without reopening the backend for prefs**: one new column
(R3), the rest is frontend. An interactive prototype of the target UX exists (design
artifact — three modes, auto-hiding chrome, manga RTL, settings sheet).

**Ownership decision — reader prefs are client state, not a server pref**
(supersedes P1.6). Reading mode / fit / brightness / image-quality are UI
preferences → **Zustand `persist`** (D-32), keyed per device. No migration, no
endpoint, no cross-device sync in v1 (a synced pref can layer on later without
touching the reader). The only server-side reader concept is
`comics.reading_direction` — a property of the *work* (manga = `rtl`), owner-set.

**The one DB addition (R3).** `comics.reading_direction text NOT NULL DEFAULT
'vertical' CHECK (reading_direction IN ('ltr','rtl','vertical'))` — migration
`0024_comic_reading_direction`, surfaced on the comic payloads (§7). This is the
"RTL flag" long noted in `comic/README.md`.

**Component architecture (frontend).** Split `ComicReaderView` (client-primary,
D-33) into a container + mode renderers:
- `ComicReaderView` — data (`getComic` + `getChapterPages`), keyboard, progress wiring
- `ReaderChrome` — top/bottom bars, auto-hide + immersive toggle
- `StripReader` — the P0.3 vertical scroll (kept), + seamless next-chapter load (R3)
- `PagedReader` — single/double page, LTR/RTL, tap-zones, swipe (R2/R3)
- `ReaderSettings` — sheet: mode · fit · brightness · quality
- hooks: `useReaderSettings` (Zustand persist), `useReaderProgress` (extract the P0.4
  furthest-page + throttle + keepalive `PUT` — SPEC-07 P0.2 transport rule; never
  `sendBeacon`), `usePagePreloader` (R3)

**Image quality → variant.** Data-saver = `thumb`, Standard = `medium` (default);
High = `medium` until SPEC-01 adds a larger image variant (images have no `poster`;
originals are never served to the reader). The client builds the URL with
the existing `variantURL(assetId, variant)`, passing the reader payload's
`asset_id` (§7).

**Phases** (each a standalone, shippable PR):
- **R1 — reading frame.** Split container/renderers; **single-page mode** + kept
  webtoon; **auto-hide chrome + immersive** (tap-center / `F`); **settings sheet**
  persisted (Zustand). Single-page usable via tap-zones + arrow keys + page counter.
  *Status: implemented 2026-08-07.*
- **R2 — navigation.** Bottom page slider + `N/total`; chapter menu in-reader;
  prev/next chapter; swipe gestures. *Status: implemented 2026-08-07.*
- **R3 — manga & smoothness.** `reading_direction` (DB+API — migration
  `0024_comic_reading_direction`, on `Comic`/`ComicPatch`); double-page + RTL
  pairing; `usePagePreloader` (next 4 pages + next-chapter prefetch); seamless
  next-chapter continuity (webtoon auto-append). *Status: implemented 2026-08-07.*
- **R4 — polish.** Double-tap / pinch / wheel / keyboard zoom + drag-pan; brightness
  + quality wired to variants; keyboard + ARIA; per-page error tile (P0.3) kept —
  now via React state (a DOM-replace on the paged `key` change crashed React).
  *Status: implemented 2026-08-07.*
- **R5 — full-screen & finish.** Reader breaks out of the `(app)` shell into a fixed
  full-viewport overlay (webtoon scrolls inside the overlay, not the window; body
  scroll locked); `prefers-reduced-motion` drops chrome-slide + zoom transitions; a
  `?` key / button opens a keyboard-shortcut help. *Status: implemented 2026-08-07.*

**R1 acceptance.**
- Mode toggle (Webtoon ↔ Single) persists across reload (same device).
- Single-page: `←`/`→` and left/right tap flip pages; center tap / `F` toggles chrome;
  in paged mode chrome auto-hides ~2.5 s after interaction.
- Webtoon behavior (P0.3) and progress resume (P0.4) are unchanged in both modes.
- Brightness dims the page; quality selects the image variant; both persist.

## 6. Data model — migration `000N_comic_core`

**Tenancy** (specs README convention, ADR-07). Tenant-scoped: `comics`, `comic_chapters`, `comic_pages`, `comic_reading_progress` (`tenant_id` + index + `tenant_isolation` policy added by `0020_platform_rls_enable`), `comic_imports` (`0026`) and `comic_sync_sources` (`0028`), both created with them. The four core tables' DDL below predates ADR-07 and omits the columns; the two P1 tables show them.

```sql
CREATE TABLE comics (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- identity-anchor exception (0007_media_assets precedent)
  title          text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
  description    text,
  cover_asset_id uuid,                     -- media asset, validated via mediaapi
  status         text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  reading_direction text NOT NULL DEFAULT 'vertical'
                 CHECK (reading_direction IN ('ltr','rtl','vertical')), -- R3, 0024_comic_reading_direction
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
-- Keyset indexes (§7): both lists order by (updated_at DESC, id DESC); the id tiebreaker is mandatory.
CREATE INDEX comics_status_updated_idx ON comics (status, updated_at DESC, id DESC);   -- shipped in 0015; serves GET /comics (status = 'published')
CREATE INDEX comics_owner_cursor_idx ON comics (owner_user_id, updated_at DESC, id DESC); -- serves GET /comics/mine
CREATE INDEX comics_cover_asset_idx ON comics (cover_asset_id) WHERE cover_asset_id IS NOT NULL; -- P0.6 cover-nulling UPDATE

CREATE TABLE comic_chapters (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  comic_id   uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
  title      text NOT NULL,
  sort_order int  NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (comic_id, sort_order) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE comic_pages (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chapter_id uuid NOT NULL REFERENCES comic_chapters(id) ON DELETE CASCADE,
  asset_id   uuid NOT NULL UNIQUE,         -- media asset, validated via mediaapi
  sort_order int  NOT NULL,
  UNIQUE (chapter_id, sort_order) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE comic_reading_progress (
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  comic_id   uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
  chapter_id uuid NOT NULL,                -- resume anchor; may dangle if chapter deleted → fall back to first chapter
  page_id    uuid REFERENCES comic_pages(id) ON DELETE SET NULL,  -- exact page; NULL after that page is deleted → resume at chapter top
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, comic_id)
);

-- P1.7 import jobs: 0026_comic_imports + 0027_comic_import_comic_level (verbatim shape).
CREATE TABLE comic_imports (
  id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  comic_id      uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
  chapter_id    uuid REFERENCES comic_chapters(id) ON DELETE CASCADE, -- NULL = whole-comic job (0027)
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- the caller who created the job
  status        text NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','uploaded','processing','done','failed')),
  upload_ref    text,                               -- storage key of the uploaded zip
  total         int  NOT NULL DEFAULT 0,            -- image entries to process
  succeeded     int  NOT NULL DEFAULT 0,            -- pages created
  failed        int  NOT NULL DEFAULT 0,            -- entries that failed
  report        jsonb NOT NULL DEFAULT '[]'::jsonb, -- [{name, ok, error?}], capped at 500 rows
  error         text,                               -- job-level failure
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  tenant_id     uuid NOT NULL DEFAULT current_setting('app.current_tenant')::uuid REFERENCES organizations(id)
);
CREATE INDEX comic_imports_chapter_idx ON comic_imports (chapter_id);
CREATE INDEX comic_imports_owner_idx   ON comic_imports (owner_user_id);
CREATE INDEX comic_imports_tenant_idx  ON comic_imports (tenant_id);
-- ENABLE + FORCE ROW LEVEL SECURITY; POLICY tenant_isolation (specs README Tenancy).

-- P1.10 sync sources: 0028_comic_sync_sources + 0029_comic_sync_batch + 0030_comic_sync_cancel.
CREATE TABLE comic_sync_sources (
  id               uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  comic_id         uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
  owner_user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- the user who added the source; the internal endpoints scope to this user (§7)
  source_url       text NOT NULL,                  -- external comic page URL
  source_site      text NOT NULL DEFAULT '',       -- host, e.g. truyenqqno.com
  chapters_hint    text NOT NULL DEFAULT '',       -- optional explicit chapter list/range (blank = all)
  last_status      text NOT NULL DEFAULT 'idle'
                     CHECK (last_status IN ('idle','syncing','done','failed','cancelled')), -- 'cancelled' added by 0030
  last_import_id   uuid,                           -- comic_imports job of the current/most recent batch
  last_error       text,                           -- failure, or the failed-chapter summary
  last_synced_at   timestamptz,
  total_chapters   int NOT NULL DEFAULT 0,         -- 0029
  scraped_chapters int NOT NULL DEFAULT 0,         -- 0029
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  tenant_id        uuid NOT NULL DEFAULT current_setting('app.current_tenant')::uuid REFERENCES organizations(id)
);
CREATE INDEX comic_sync_sources_comic_idx  ON comic_sync_sources (comic_id);
CREATE INDEX comic_sync_sources_owner_idx  ON comic_sync_sources (owner_user_id);
CREATE INDEX comic_sync_sources_tenant_idx ON comic_sync_sources (tenant_id);
-- ENABLE + FORCE ROW LEVEL SECURITY; POLICY tenant_isolation (specs README Tenancy).
```

*(Code follow-up: `0015` shipped only `comics_owner_idx ON comics (owner_user_id)`
and no cover index; `comics_owner_cursor_idx` and `comics_cover_asset_idx` above are
the target. The two P1 tables were migrated with `ADD COLUMN tenant_id` → `SET
DEFAULT` → `SET NOT NULL` steps, which the inline column above summarises.)*

**Reordering (canonical pattern — movie/music/story copy this verbatim).** Both
sort-order uniques are declared `DEFERRABLE INITIALLY DEFERRED`, so the constraint is
checked once at `COMMIT` instead of per-statement. `PUT .../pages:order` (and the
chapter equivalent) takes the **full ordered id list**; the service, inside one
transaction, rewrites every row's `sort_order` to its new value (10,20,30…) in any
order. Transient duplicates (e.g. two rows momentarily at `20` while a swap is
half-applied) are legal mid-transaction and resolve before commit — so the naïve
"update A to B's slot" that would instantly trip a per-statement unique just works.
No negative-temp or `+offset` dance, and no ordering gymnastics in Go/sqlc.

> Caveat: a `DEFERRABLE` unique cannot be an `ON CONFLICT` arbiter, so page/chapter
> writes must not upsert on `(…, sort_order)` — they don't (inserts assign explicit
> orders). If a plain non-deferrable unique is ever required instead, the portable
> fallback is the two-phase update: bump all affected rows by a large constant offset,
> then set finals in a second pass within the same transaction.

**Scope of this schema (reference note — re: "what movie/music inherit").** Copy the
*flow* — migration → `query/` → repository → service/handler → `MountHTTP` → view —
not the columns. Domain-specific fields (movie/music per-item `duration`, etc.) belong
to each vertical's **own** migration under its own schema ownership; do **not** pre-add
a speculative `meta jsonb` here that comic never reads — an unused column teaches the
next dev to add unused columns. comic adds its own columns (e.g. tags)
if and when its roadmap needs them.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/comics?cursor=&limit=` | `comics:read` | drafts excluded; keyset paging ordered `updated_at DESC, id DESC`; the opaque cursor encodes `(updated_at, id)`, and the `id` tiebreaker is mandatory. This deliberately differs from SPEC-01's `created_at DESC, id DESC` because the library sorts by recent activity (specs README Pagination: the key ends in `id`). Limit default 30, max 50; lenient per the README rule (missing, non-integer or < 1 → 30; above 50 → clamped to 50) |
| GET | `/api/v1/comics/mine?cursor=&limit=` | `comics:write:own` | incl. drafts, status badges; same keyset (`updated_at DESC, id DESC`) over `owner_user_id`; limit default 30, max 50, lenient as above |
| POST | `/api/v1/comics` | `comics:write:own` | |
| GET | `/api/v1/comics/{id}` | published: `comics:read`; own draft: owner | returns `ComicDetail` = `Comic` + `chapters[]` (ordered) + `progress: {chapter_id, page_id\|null, updated_at} \| null` for the caller; the client derives ch. N from the chapter's index and p. M from the page's index in `GET /chapters/{id}/pages` (P0.4) |
| PATCH | `/api/v1/comics/{id}` | owner, or `comics:write:any` (`RequireOwnerOrPermission`) | update title/description/cover/reading_direction (`ComicPatch`) — **status is NOT changed here**; `… SET …, updated_at = now()` (specs README updated_at convention) |
| POST | `/api/v1/comics/{id}/publish` | `comics:publish:own` (`RequirePermission`), then owner, or `comics:publish:any` (`RequireOwnerOrPermission`) — chained, P0.2 | **dedicated endpoint** (not a `PATCH {status}`); 200 `Comic`, or 422 `comic/not-publishable` listing offending chapters |
| POST | `/api/v1/comics/{id}/unpublish` | `comics:publish:own` (`RequirePermission`), then owner, or `comics:publish:any` (`RequireOwnerOrPermission`) — chained, P0.2 | back to `draft`; 200 `Comic` |
| DELETE | `/api/v1/comics/{id}` | owner, or `comics:delete:any` | |
| POST | `/api/v1/comics/{id}/chapters` | owner, or `comics:write:any` | |
| PATCH/DELETE | `/api/v1/chapters/{id}` | owner, or `comics:write:any` | |
| PUT | `/api/v1/comics/{id}/chapters:order` | owner, or `comics:write:any` | body `{order: [chapter_id…]}` (`ReorderRequest`) — the §6 reorder pattern's "chapter equivalent", previously missing here |
| POST | `/api/v1/chapters/{id}/pages` | owner, or `comics:write:any` | body `{pages: [{asset_id, sort_order}]}` (`PagesCreate`) — **wrapped object, not a bare array** |
| PUT | `/api/v1/chapters/{id}/pages:order` | owner, or `comics:write:any` | body `{order: [page_id…]}` (`ReorderRequest`) |
| DELETE | `/api/v1/pages/{id}` | owner, or `comics:delete:any` (moderation) | removes the page row only; the media asset is untouched (P0.1) |
| GET | `/api/v1/chapters/{id}/pages` | published: `comics:read`; own draft: owner | reader payload: `{items: [{page_id, asset_id, width, height}]}` in reading order (non-paginated list — specs README Pagination; *code follow-up: HEAD answers `{pages: [...]}`*); the client renders the `medium` variant via `variantURL(asset_id, 'medium')` (no URL in the payload); `width`/`height` are the asset's dimensions (nullable) for layout reservation. Draft invisibility applies here too (404, not 403) |
| PUT | `/api/v1/comics/{id}/progress` | authenticated | `{chapter_id, page_id}`; membership-validated (P0.4) |
| POST | `/api/v1/chapters/{id}/imports` | owner, or `comics:write:any` | P1.7 per-chapter job; 201 `ImportJob` (`owner_user_id` = caller) |
| POST | `/api/v1/comics/{id}/imports` | owner, or `comics:write:any` | P1.7 whole-comic job; 201 `ImportJob` |
| PUT | `/api/v1/imports/{id}/zip` | authenticated; caller must be the job's `owner_user_id` (else 404 `comic/not-found`) | raw zip body; 200 `ImportJob`; over `importMaxZipBytes` → 422 `comic/validation` |
| GET | `/api/v1/imports/{id}` | authenticated; caller must be the job's `owner_user_id` (else 404 `comic/not-found`) | 200 `ImportJob` (poll) |
| GET | `/api/v1/comics/{id}/sync-sources` | owner, or `comics:write:any` | P1.10; 200 `{items: [SyncSource]}` (unpaginated — specs README Pagination; *code follow-up: HEAD answers `{sources: [...]}`*) |
| POST | `/api/v1/comics/{id}/sync-sources` | owner, or `comics:write:any` | P1.10; body `{source_url, chapters_hint?}`; 201 `SyncSource` (`owner_user_id` = caller); SSRF-guard failure → 422 `comic/validation` |
| POST | `/api/v1/sync-sources/{id}/sync` | authenticated; caller must be the source's `owner_user_id` (else 404 `comic/not-found`) | P1.10; 202 `SyncSource`; already syncing → 422 `comic/validation` |
| POST | `/api/v1/sync-sources/{id}/cancel` | authenticated; caller must be the source's `owner_user_id` (else 404 `comic/not-found`) | P1.10; 200 `SyncSource` (`cancelled`); not syncing → 422 `comic/validation` |
| DELETE | `/api/v1/sync-sources/{id}` | authenticated; caller must be the source's `owner_user_id` (else 404 `comic/not-found`) | P1.10; 204 |

The import-job and sync-source rows keyed by their own id (`/imports/*`,
`/sync-sources/*`) are **owner-only**, as shipped: the handler compares the caller
with the row's `owner_user_id`, and a `comics:write:any` editor reaches only the
jobs and sources they created. When `COMIC_SCRAPER_URL` is unset, every sync-source
row returns 404 `comic/sync-disabled` (P1.10; code follow-up).

**Internal (scraper → API) endpoints** — P1.10, no user session, outside
`authTenant`:

| Method | Path | Permission | Notes |
|---|---|---|---|
| POST | `/api/v1/internal/comic/sync-batch` | internal secret (below) | `{source_id, owner_id}` → `{import_id, upload_key}`: a new comic-level `comic_imports` job for one batch |
| POST | `/api/v1/internal/comic/sync-callback` | internal secret | `{import_id, owner_id, ok, error?}`: on `ok`, set `upload_ref` and enqueue `comic:import_zip {import_id}`; otherwise fail that job |
| POST | `/api/v1/internal/comic/sync-progress` | internal secret | `{source_id, owner_id, scraped, total}` → `scraped_chapters`/`total_chapters` |
| POST | `/api/v1/internal/comic/sync-finalize` | internal secret | `{source_id, owner_id, ok, failed}` → `last_status` `done`/`failed`, `last_error` = failed-chapter summary |

- **Secret.** Header `X-Internal-Secret` must equal `COMIC_SYNC_SECRET`, compared in
  constant time. Missing or mismatched → 404 (the route reads as absent); when
  `COMIC_SYNC_SECRET` is empty every request is 404, so an empty secret never
  authenticates. *(Code follow-up: `requireInternalSecret` compares with `!=`, not
  `subtle.ConstantTimeCompare`; `shared/openapi.yaml` declares 401 for a bad secret
  where the handler answers 404.)*
- **Exposure.** These routes must not be reachable from the public edge: the reverse
  proxy does not route `/api/v1/internal/*`. *(Code follow-up: the Traefik `api`
  router matches the whole API host, so the routes are publicly reachable today and
  the secret is the only guard.)* They are declared in `shared/openapi.yaml` under
  the `internalSecret` security scheme.
- **Tenant resolution.** The scraper echoes the `owner_id` the API sent it when
  starting the scrape. The API never trusts it alone: `runInUserTenant` opens that
  user's personal-org scope, re-reads the source (or import) row by id inside it,
  and requires the row's `owner_user_id` to equal `owner_id`; a mismatch resolves to
  not found and nothing is written. A missing `owner_id` is rejected before any
  write.
- **API → scraper.** Calls to the scraper (`/sync`, `/cancel`) send the same
  `X-Internal-Secret`; the scraper service is internal-only (compose network
  `internal`, no published port). *(Code follow-up: `cmd/api/scraper.go` sends no
  secret header.)*

Problem types: `comic/invalid-page-asset`, `comic/invalid-cover-asset`,
`comic/invalid-progress-target`, `comic/not-publishable`, `comic/not-found` (also
for a missing or foreign import job or sync source), `comic/invalid-cursor` (400,
malformed cursor on either list), `comic/validation` (422; also an oversized import
zip, an SSRF-rejected source URL, and sync/cancel in the wrong state), and
`comic/sync-disabled` (404, P1.10; not yet emitted or in `problems.ts` — code
follow-up). The earlier `comic/zip-rejected` was never emitted and is dropped:
archive-level import failures surface as the job's `status=failed` + `error` (P1.7).

Annotate each operation per the specs README AuthZ **OpenAPI encoding** (combined-method rows split per operation). Both list endpoints return `{items, next_cursor}` (specs README Pagination, which retrofits pre-rule endpoints — owner decision 2026-09-30; there is no `comics` key), and the two non-paginated lists (chapter pages, sync sources) return `{items}`. *(Code follow-up: the shipped handlers and `shared/openapi.yaml` still answer `{comics: [...], next_cursor?}`, `{pages: [...]}` and `{sources: [...]}`; the retrofit renames each key to `items` in the handler, `shared/openapi.yaml` and the frontend readers (library grid, reader, sync panel) in one PR.)*

## 8. Success metrics

- Leading: dev publishes ≥1 real multi-chapter comic and reads it on mobile over
  the LAN; time-to-first-page < 2 s measured via Lighthouse on the reader route.
- Leading: the vertical-pattern claim is tested — starting movie later reuses ≥80%
  of this module's shape (subjective but reviewable).
- Lagging: reading progress survives a stack restart (`make up`) — persistence, not
  cache.

## 9. Timeline & phasing

1. Migration + queries + repository + service scaffolding (1 day)
2. CRUD + publish + RBAC wiring, OpenAPI (1 day)
3. Reader + progress (1–1.5 days)
4. Library/detail pages replacing placeholders (1 day)
5. P1: zip import (per-chapter + whole-comic), reader R1–R5, external-source sync
   (header revs 13–14); bookmarks (P1.8) unscheduled
P0 ≈ 4–5 dev-days; P1 adds ~2.

## 10. Open questions

- **(engineering, non-blocking)** Reader payload URL strategy: direct storage URLs
  (current HLS behavior — public-ish) vs API-proxied. Follow the current media
  convention now; both flip together when playback ACL lands (deferred item).
- **(product, non-blocking)** Should `comic_chapters.sort_order` be user-visible
  chapter numbers, or is display numbering derived (1..N)? Recommendation: derived
  display numbering; `sort_order` stays an internal ordering key.

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

Baseline: `main` @ `99b5a0b` (the docs commits on top changed no code). The spec
text above is the target; this section lists every place the shipped code still
diverges from it, so an implementer needs nothing beyond this file. Rows are
ordered by severity — security and data integrity first, then contract and
polish. A row closes when the code matches the target and the SPEC-02 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded.
Paths are relative to `backend/internal/modules/comic/` unless stated otherwise.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | §7 Internal endpoints — Exposure | `/api/v1/internal/*` is not reachable from the public edge. | `docker-compose.yml` Traefik router `api` matches `Host(api.${APP_DOMAIN})` with no path exclusion, so the four internal routes are public and the shared secret is the only guard. | **infra:** exclude the `/api/v1/internal` path prefix from the `api` router rule (`&& !PathPrefix(...)`) (or serve the internal routes on an unexposed listener). **test:** a request through the edge to `/api/v1/internal/comic/sync-batch` gets 404. | F043 |
| 2 | §7 Internal endpoints — Secret | `X-Internal-Secret` compared in constant time; missing/mismatched → 404; empty secret never authenticates; `shared/openapi.yaml` matches the 404. | `module.go` `requireInternalSecret` compares with `!=` (the empty-secret → 404 part matches). `shared/openapi.yaml` declares `401 Bad or missing X-Internal-Secret` on all four internal operations. | **backend:** `subtle.ConstantTimeCompare`. **openapi:** replace the 401 responses with 404. **test:** mismatched and empty secret → 404. | F043 |
| 3 | §7 Internal endpoints — API → scraper | Calls to the scraper (`/sync`, `/cancel`) carry the same `X-Internal-Secret`. | `cmd/api/scraper.go` `StartScrape` and `CancelScrape` set only `Content-Type`; `scraper/app.py` `/sync` and `/cancel` verify nothing. | **backend:** pass `COMIC_SYNC_SECRET` to `newHTTPScraper` and set the header on both calls. **scraper:** `app.py` rejects a missing/mismatched header (404). **test:** scraper client sends the header. | F043 |
| 4 | P0.2 publish RBAC; §7 publish/unpublish | `RequirePermission("comics:publish:own")` chained before `RequireOwnerOrPermission(engine, "comics:publish:any", extractComicOwner)`. | `cmd/api/main.go` builds `PublishMW` as `RequireOwnerOrPermission(engine, "comics:publish:any", byComic)` only; `module.go` mounts `/publish` and `/unpublish` with it, so `comics:publish:own` is never checked. | **backend:** mount `m.perm("comics:publish:own")` before `PublishMW` on both routes. **test:** TC-COMIC-042 (owner without `:own` → 403), TC-COMIC-041 (editor unpublishes → 200). | F051 |
| 5 | P0.2 AC (draft is 404 to a non-owner without `:any`) | `extractComicOwner` returns 404 `comic/not-found` for a draft when the caller is neither the owner nor a holder of the endpoint's `:any` code. | `cmd/api/main.go` `ownerExtractor` with `OwnerByComic`/`OwnerByChapter`/`OwnerByPage` (`query/comic.sql` `GetComicOwner*`) resolve the owner whatever the status, so creator D mutating C's draft gets 403 and learns the draft exists. | **backend:** the owner queries also return `comics.status`; the extractor (or a comic-side wrapper) returns `accountmw.ErrOwnerNotFound` for a draft unless caller = owner or the engine grants the endpoint's `:any` code. **test:** TC-COMIC-036 expects 404 on a draft and 403 on a published comic. | F119 |
| 6 | P0.1 asset ownership; P1.7; P1.10 | Imported and synced assets are ingested with owner = `comics.owner_user_id`, never the requesting editor. | `import.go` `RunImport` sets `owner := job.OwnerUserID` (the caller) and uses it for `runInTenant`, `media.Ingest` and `CreatePages`; `sync.go` `RequestSyncBatch` creates the job with `src.OwnerUserID`. An editor's import creates assets the P0.1 check in `CreatePages` then rejects (and runs in the editor's tenant), so the chapter gets no pages. | **backend:** `RunImport` resolves `OwnerByComic(job.ComicID)` and uses it for the tenant scope, `Ingest` and paging; `comic_imports.owner_user_id` stays the caller and keeps gating `/imports/{id}`. **test:** an editor's import on C's comic yields assets owned by C and pages created. | F050 |
| 7 | P1.7 (`origin='import'`); SPEC-01 P1.2 | Imports call `mediaapi.Ingest(..., origin="import")` so `media:asset_ready` consumers suppress the flood. | `import.go` `RunImport` calls `s.media.Ingest(ctx, owner, name, mime, data)`; `mediaapi.Ingest` has no `origin` parameter and every imported asset is `origin='upload'`. | **backend:** after SPEC-01 §11 row 10 lands, pass `"import"`; update `fakeMedia.Ingest` in `comic_test.go`. **test:** TC-COMIC-140 asserts `origin='import'` on created assets. | F038 |
| 8 | P1.7 Queue | `comic:import_zip` runs on a dedicated `bulk` queue served by its own `asynq.Server` (Concurrency 1), shared with SPEC-09's `ops:takeout`. | `import.go` `enqueueImportZip` uses `asynq.Queue("default")` with `Timeout(12h)`; `cmd/worker/main.go` registers it via `comicMod.RegisterTasks(lightMux)` on the light server that also serves notify. | **backend:** `Queue("bulk")`; new `bulkSrv` in `cmd/worker` (`Concurrency: 1`, `Queues: {"bulk": 1}`) with its own mux carrying `comic:import_zip`; keep `comic:on_asset_deleted` on the light mux. **test:** task options assert queue `bulk`. | F095 |
| 9 | P0.2 (a) empty chapters on a published comic | For non-owners, a 0-page chapter is omitted from the chapter list, `chapter_count` and reader navigation, and `GET /chapters/{id}/pages` for it is 404 `comic/not-found`. | `handler.go` `GetComic` returns every chapter and sets `chapter_count = len(chapters)`; `query/comic.sql` `ListPublishedComics`/`ListOwnComics` count all chapters; `service.go` `ReaderPagesVisible` returns an empty list. | **backend:** a non-owner chapter query with `EXISTS (SELECT 1 FROM comic_pages p WHERE p.chapter_id = ch.id)`, the same predicate in the `chapter_count` subqueries for the public list; `ReaderPagesVisible` returns `ErrNotFound` when the caller is not the owner and the chapter has 0 pages. **test:** the three P0.2 (a)–(c) ACs. | F049 |
| 10 | P1.10 Config (`COMIC_SCRAPER_URL` unset) | Every sync-source endpoint returns 404 Problem `comic/sync-disabled`. | `module.go` mounts list/create/delete/cancel unconditionally; `sync.go` `TriggerSync` returns a plain `errors.New("comic: sync not configured")`, which `writeComicErr` maps to 500 `about:blank`. `frontend/src/lib/problems.ts` has no `comic/sync-disabled`. | **backend:** when `Deps.Scraper` is nil, the `/comics/{id}/sync-sources` and `/sync-sources/*` routes answer 404 `comic/sync-disabled`. **openapi:** declare it. **frontend:** add the slug + i18n string to `problems.ts`; hide the sync panel on it. **test:** each route with no scraper → 404 `comic/sync-disabled`. | F042 |
| 11 | P0.4 transport rule | Every progress save, pagehide included, is a keepalive `PUT`; never `navigator.sendBeacon`. | `frontend/src/lib/comic.ts` (save-progress helper, lines ~118–121) calls `navigator.sendBeacon` first (a POST → 405 on this PUT-only route) and only falls back to the keepalive `fetch` when `sendBeacon` is missing. | **frontend:** always `fetch(url, {method:'PUT', keepalive:true, credentials:'include', headers:{'Content-Type':'application/json'}, body})`. **test:** TC-COMIC-087. | F001 |
| 12 | §7 list envelopes; Pagination | Both cursor lists answer `{items, next_cursor?}`; chapter pages and sync sources answer `{items}`; `/comics/mine` documents `limit` (default 30, max 50). | `handler.go` `writeComicList` → `{comics}`, `ReaderPages` → `{pages}`, `ListSyncSources` → `{sources}`. `shared/openapi.yaml` `listComics`/`listMyComics` `required: [comics]`, `/chapters/{id}/pages` `required: [pages]`, `SyncSourceList` `required: [sources]`; `listMyComics` has no `limit` parameter. Frontend `lib/comic.ts` reads `r.comics`/`r.pages`; the sync panel reads `sources`. | One PR: **backend** renames the three keys to `items`; **openapi** `required: [items]` and the `limit` parameter on `/comics/mine`; **frontend** library grid, reader and sync panel read `items`. **test:** TC-COMIC-067, TC-COMIC-102. | Decision 2026-09-30 (Envelopes), F024 |
| 13 | §6 indexes | `comics_owner_cursor_idx (owner_user_id, updated_at DESC, id DESC)` serves `/comics/mine`; `comics_cover_asset_idx (cover_asset_id) WHERE cover_asset_id IS NOT NULL` serves the P0.6 cover-nulling UPDATE. | `0015_comic_core` ships only `comics_owner_idx ON comics (owner_user_id)` and no cover index; `ListOwnComics` and `NullCoverByAsset` run without them. | **migration:** new `000N_comic_indexes` creating both and dropping `comics_owner_idx` (0015 is applied and is not edited). **test:** TC-COMIC-164 up/down. | F048, F121 |
| 14 | §7 `GET /comics/{id}` (`ComicDetail.progress`) | `progress` is `{chapter_id, page_id\|null, updated_at} \| null`. | `handler.go` `GetComic` omits the `progress` key when there is no row; `shared/openapi.yaml` `ComicDetail.progress` is not nullable. | **backend:** always emit `progress`, `null` when absent. **openapi:** `nullable: true`. **frontend:** reader of `ComicDetail` treats `null`. **test:** TC-COMIC-104. | found while verifying (2026-10-01) |
| 15 | P1.7 step 3 (`ImportJob.status`) | `status ∈ pending\|uploaded\|processing\|done\|failed` (the `0026` CHECK). | `shared/openapi.yaml` `ImportJob.status` enum lists `running` instead of `processing`. | **openapi:** `processing`. **test:** TC-COMIC-160 drift check. | found while verifying (2026-10-01) |
| 16 | §7 OpenAPI encoding | Each operation carries `x-required-permission` per the specs README (combined rows split per operation). | `shared/openapi.yaml` has no `x-required-permission` anywhere. | **openapi:** annotate every comic operation. **test:** TC-COMIC-160. | F025 (README OpenAPI encoding) |
| 17 | §7 list `limit` (owner decision 2026-10-01) | Both cursor lists: missing, non-integer or < 1 → 30; above 50 → **clamped to 50**; never a Problem. | `service.go` `Service.list` (behind `ListPublished` and `ListOwn`): `if limit <= 0 \|\| limit > maxLimit { limit = defaultLimit }` — `?limit=500` returns 30. | **backend:** clamp instead of resetting (`> 50 → 50`, `≤ 0 → 30`), e.g. `platform/server.Limit(r, 30, 50)` in the two handlers. **openapi:** describe `limit` as defaulted and clamped on both lists. **test:** TC-COMIC-105. | Decision 2026-10-01 (limit) |

**Already matching (verified on HEAD — do not redo).**
- Draft invisibility for reads: `service.go` `GetComic`, `ReaderPagesVisible` and `SaveProgress` answer 404 for someone else's draft before any membership check.
- Progress membership: `SaveProgress` → 422 `comic/invalid-progress-target` when the chapter is not in the comic or the page not in the chapter; keyed by `page_id`.
- Asset validation: `validateImageAsset` requires a ready image owned by the comic's owner (`c.OwnerID` / the chapter's owner), so an editor's own asset is rejected.
- Owner-or-elevated middlewares for write (`comics:write:any`), comic delete and page delete (`comics:delete:any`) in `cmd/api/main.go`; chapter delete uses the write gate.
- Publish validation (≥ 1 chapter, no empty chapter → 422 listing them); one `comic:published` per publish with `chapter_count`; `comic:chapter_deleted` per deleted chapter, once per chapter on comic delete.
- Lists: keyset `updated_at DESC, id DESC`, limit default 30 for a missing or invalid value (above 50 resets to 30 — row 17), bad cursor 400 `comic/invalid-cursor`; `comics_status_updated_idx` (0015).
- P0.6 consumer `HandleAssetDeleted` (pages deleted, cover nulled, idempotent) via `platform/events`.
- P1.7 guards: `importMaxZipBytes` 16 GiB (422 at upload), `importMaxEntries` 100,000, ratio 100:1 (`RunImport` skips the entry silently before extraction; nothing left → job `failed`), 60 MiB per-entry read cap (`readZipEntry`), natural sort, poll timeout max(2 min, n × 5 s) ≤ 11 h, `Timeout(12h)` + `MaxRetry(0)`; `/imports/{id}` owner-only (404).
- P1.10: SSRF guard on create and before every scrape; second trigger while `syncing` is 422 unless stale > 15 min; cancel only while `syncing` and fails if the scraper refuses; internal handlers re-read the row inside `runInUserTenant` and compare the echoed `owner_id`; missing `owner_id` rejected; the scraper container is on the `internal` network only.

**Test evidence to add/fix.**
- `comic_test.go: TestDraftVisibility` and `http_test.go: TestHTTPDraftIsNotFoundToAStranger` cover reads only; add the mutation case (draft → 404, published → 403) — TC-COMIC-036 currently documents the old 404/403 ambiguity and must be rewritten.
- TC-COMIC-141, TC-COMIC-142 and TC-COMIC-143 still describe the rev-9 caps (500 MB, 300 entries, nested dirs rejected); rewrite them to the P1.7 guard table, and TC-COMIC-102/TC-COMIC-140 to the `items` envelope and `origin='import'`. (TC-COMIC-144 was rewritten on 2026-10-01 to the zip-bomb behaviour above — a skipped entry, not a rejected upload; no test covers it yet.)
- `comic_test.go: TestPublishValidation` does not exercise the middleware chain; add HTTP tests for TC-COMIC-039/041/042.
- New: editor import owned by the comic owner (P0.1 AC), `bulk` queue option (F095), empty-chapter hiding (P0.2 (a)), `comic/sync-disabled` (P1.10), internal secret constant-time + 404 (F043), keepalive PUT (TC-COMIC-087).
- P1.10 has no TEST-CASES rows yet; add rows for TriggerSync (202, stale rule), cancel, batch retries and finalize.

## 12. Revision history

| Rev | Date | Change |
|---|---|---|
| r1 | 2026-07-05 | Initial spec from brief 02 — the reference media→domain vertical. |
| r2 | 2026-07-08 | Reading progress keyed by `page_id` (not array index); asset-deletion coupling via `media:asset_deleted` (P0.6); `DEFERRABLE`-unique reorder pattern. |
| r3 | 2026-07-10 | Code-verified reconciliations: RBAC reworked to **owner-or-elevated** (`RequireOwnerOrPermission`; new `comics:write:any`/`comics:publish:any`/`comics:delete:any` moderation grants seeded to editor/admin per the movies precedent) — the earlier `all-via-RequirePermission` on `:own` neither checked ownership nor admitted moderation; permission codes aligned to the 0003 2–3-segment catalog (dropped `comics:read:published`); identity-anchor FK to `users(id)` added to `comics`/`comic_reading_progress` (§6); `DELETE /pages/{id}` added to §7 and P0.1; list endpoints switched to cursor paging (§7); zip-import upload path + entry-scaled poll-timeout defined (P1.7); `comic:chapter_deleted` stream-removal event added (P1.9); P0.5 gained acceptance criteria; template-registry note added to the reader (P0.3) and library (P0.5) views. |
| r4–r14 | 2026-08-07 → 2026-08-15 | Recorded in the header's rev notes (reader redesign R1–R5, zip import per-chapter + whole-comic, external-source sync); not restated here. |
| r15 | 2026-09-30 | Spec-gap fixes: external-source sync gets its own section (P1.10, renamed from the colliding "P1.8"), `comic_imports`/`comic_sync_sources` DDL, and §7 rows for the import, sync-source and internal endpoints (secret, exposure, tenant resolution); P1.7 rewritten around the shipped job endpoints and per-mode guard table; `reading_direction` in §6/§7; keyset order `updated_at DESC, id DESC`; publish chains `comics:publish:own`; post-publish invariant; asset-ownership rule for editors and imports; `ComicDetail.progress` and reader payload (`asset_id`) aligned with the shipped shapes. Code follow-ups are marked inline. |
| r15 | 2026-10-01 | Added §11 implementation gaps (self-contained follow-up list); Revision history renumbered §12; §7 reorder rows name the shipped `{order: [...]}` body. |
