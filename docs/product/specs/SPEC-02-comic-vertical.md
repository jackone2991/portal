# SPEC-02 — Comic Vertical (end-to-end)

**Status:** ready to build, rev 3 · **Drafted:** 2026-07-05 · **Last-verified:** 2026-07-10
**Module:** `comic` (skeleton: `module.go` + `api/` stub) · **Depends on:** SPEC-01 (image kind)
**Upstream:** [briefs/02-comic-vertical.md](../briefs/02-comic-vertical.md) · **Refs:** feature-inventory.md §7, frontend.md Phase 4
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
  module; the `comic:chapter_published` event is emitted anyway — P1.9).
- Import/scraping from external comic sources.
- Per-comic visibility/ACL: `published` = visible to **all authenticated users** at
  v1. Visibility scoping arrives with the privacy layer. (Recommendation locked
  from the feature doc's open question.)
- Reading-direction RTL (manga) and page-spread layout intelligence — P2.
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

**Acceptance criteria.**
- Given a page create with a nonexistent, non-image, non-ready, or not-owned
  asset id, then 422 Problem `comic/invalid-page-asset` and no row.
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
`comics:delete:any` as the elevated code; a `{status}` publish/unpublish change
uses a new elevated `comics:publish:any` (owner still needs `comics:publish:own`).
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
| publish / unpublish (`{status}` change) | owner + `comics:publish:own`, **or** `comics:publish:any` |
| delete a comic, or delete a page | owner, **or** `comics:delete:any` |

**Seeding (this module's migration ships it, 0003 pattern):** `comics:read` →
`user` role (v1 = all-authenticated readers; not `guest`, unlike
`movies:read`, per the §3 visibility decision); `comics:write:own` +
`comics:publish:own` → `creator`; `comics:write:any` + `comics:publish:any` →
`editor` (the write/publish-any moderation tier, mirroring `movies:write:any` +
`movies:publish`); `comics:delete:any` → `admin` (the movies precedent —
`movies:delete:any` is admin-tier). An unseeded code 403s everyone below
superadmin.

Publish validation: a comic may be published only if it has ≥1 chapter and every
chapter has ≥1 page; otherwise 422 `comic/not-publishable` listing the offending
chapters. Unpublish returns it to `draft`.

**Acceptance criteria.**
- Given a user without `comics:write:own`, when POST /comics, then 403.
- Given reader R and creator C's draft, when R fetches detail/pages/list, then the
  draft is absent from lists and detail returns 404 (not 403 — don't leak existence).
- Given a comic with an empty chapter, when publish is attempted, then 422 naming
  the chapter.
- Given creator D (holds `comics:write:own`) editing creator C's comic, then
  404/403 and no change — `:own` alone must not grant cross-owner writes.
- Given an editor with `comics:write:any` editing another creator's comic, then
  200; and an admin with `comics:delete:any` calling DELETE passes.

### P0.3 — Reader (vertical scroll)

Route `/library/comic/[id]/read/[chapterId]` (client component; catalogue/detail
stay RSC-first per D-33). **Presentation goes through the version-switched template
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
Phase 2-3). Detail page shows **Continue reading → ch. N, p. M** (M = the
page's current 1-based position, computed when the detail page is fetched —
only `page_id` is stored, so a position "at read time" is not recoverable
after a reorder, and the current position is what Continue actually scrolls
to) when progress exists; opening via Continue scrolls to that page.

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
- A result set of >1 page paginates without duplicates.
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

- **P1.6 Reader modes**: single-page and double-page modes with keyboard/tap
  navigation; mode persisted per user — server-side pref, for cross-device
  continuity. Needs a store + endpoint — a `reader_mode` column on
  `comic_reading_progress` (or a dedicated prefs row) and `GET/PUT
  /api/v1/comics/reader-prefs`; **schema + endpoints to be added to §6/§7 when
  scheduled.**
- **P1.7 Zip chapter upload**: the .zip is uploaded via a **dedicated presigned
  PUT** — a separate `import/` prefix, a **500 MB content-length-range**, and **no
  `assets` row** (it is not a media asset, so SPEC-01's 50 MB asset-upload path does
  not apply). The client then POSTs `pages:import-zip {upload_ref: <storage key>}`
  to `POST /api/v1/chapters/{id}/pages:import-zip`, which enqueues worker task
  `comic:import_zip {chapter_id, upload_ref}` (events.md). The worker **streams the
  object and deletes it after processing**: it unpacks, creates media assets via
  `mediaapi` (batch, marked `origin='import'` so `media:asset_ready` consumers
  suppress the flood — SPEC-01 P1.2), creates pages in **filename natural-sort
  order**. **Ready-race rule (2026-07-10):** P0.1 requires a page's asset to be
  `ready` at write time, but batch-created assets start `processing` (variants are
  generated asynchronously by `media:process_image`) — so the import task runs on
  the **`default` queue** and, after creating the assets, **polls their status via
  `mediaapi`** (~2 s interval, **bounded timeout = max(10 min, entry_count × 15 s ÷
  heavy-queue concurrency)** — sized so a full 300-entry import at concurrency 1
  (SPEC-01 P0.1) cannot time out while its assets are still queued; only assets
  individually stuck > that window are reported as failures), creating pages only for
  assets that reached `ready`. It must NOT run on the heavy queue: with heavy
  concurrency 1–2 (SPEC-01 P0.1), an import polling there would occupy the very
  slot its `media:process_image` tasks need — a self-deadlock. Guards (blocking
  question resolved here): max 500 MB zip, max 300 entries, images only by
  magic bytes, reject nested directories and path traversal (`../`), reject
  compression ratio > 100:1 (zip-bomb). Failures (including poll timeouts)
  produce a per-file report; the chapter gets the pages that succeeded.
- **P1.8 Bookmarks**: per user, per page; list on the detail page. Needs a
  `comic_bookmarks(user_id, page_id fk ON DELETE CASCADE, created_at,
  PRIMARY KEY (user_id, page_id))` table and `PUT/DELETE
  /api/v1/pages/{id}/bookmark` + `GET /api/v1/comics/{id}/bookmarks`; **schema +
  endpoints to be added to §6/§7 when scheduled.**
- **P1.9 Event**: emit `comic:chapter_published` `{comic_id, chapter_id, owner_user_id,
  title}` — life-stream producer #2. Also emit `comic:chapter_deleted`
  `{comic_id, chapter_id, owner_user_id}` on a **chapter delete** and **once per
  chapter on a comic delete**, so no published-chapter history card is left with a
  404ing href (media and bank deletions remove their stream items the same way).
  Register it in events.md with consumer *"stream — delete the
  (`source_module='comic'`, `ref_id=chapter_id`) item (SPEC-06 P0.1)"*, and add the
  matching handler row to SPEC-06's P0.1 table (both are part of this item's
  definition of done).

### P2 — future considerations

- Review/approval publish workflow (mirror story module when it lands).
- RTL reading direction; double-page spread pairing metadata.
- Per-comic visibility (unlisted/link-only) once the privacy layer exists.

## 6. Data model — migration `000N_comic_core`

```sql
CREATE TABLE comics (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- identity-anchor exception (0007_media_assets precedent)
  title          text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
  description    text,
  cover_asset_id uuid,                     -- media asset, validated via mediaapi
  status         text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON comics (status, updated_at DESC);
CREATE INDEX ON comics (owner_user_id);

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
```

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
next dev to add unused columns. comic adds its own columns (tags, RTL direction — P2)
if and when its roadmap needs them.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/comics?cursor=` | `comics:read` | drafts excluded; cursor paging, order `status, updated_at DESC` (the §6 index) — matches the SPEC-01/04/05/06/08 cursor convention |
| GET | `/api/v1/comics/mine?cursor=` | `comics:write:own` | incl. drafts, status badges; cursor paging |
| POST | `/api/v1/comics` | `comics:write:own` | |
| GET | `/api/v1/comics/{id}` | published: `comics:read`; own draft: owner | |
| PATCH | `/api/v1/comics/{id}` | owner, or `comics:write:any` (`RequireOwnerOrPermission`) | update title/description/cover only (`ComicPatch`) — **status is NOT changed here** |
| POST | `/api/v1/comics/{id}/publish` | owner+`comics:publish:own`, or `comics:publish:any` | **dedicated endpoint** (not a `PATCH {status}`); 200 `Comic`, or 422 `comic/not-publishable` listing offending chapters |
| POST | `/api/v1/comics/{id}/unpublish` | owner+`comics:publish:own`, or `comics:publish:any` | back to `draft`; 200 `Comic` |
| DELETE | `/api/v1/comics/{id}` | owner, or `comics:delete:any` | |
| POST | `/api/v1/comics/{id}/chapters` | owner, or `comics:write:any` | |
| PATCH/DELETE | `/api/v1/chapters/{id}` | owner, or `comics:write:any` | |
| PUT | `/api/v1/comics/{id}/chapters:order` | owner, or `comics:write:any` | `[chapter_id…]` — the §6 reorder pattern's "chapter equivalent", previously missing here |
| POST | `/api/v1/chapters/{id}/pages` | owner, or `comics:write:any` | body `{pages: [{asset_id, sort_order}]}` (`PagesCreate`) — **wrapped object, not a bare array** |
| PUT | `/api/v1/chapters/{id}/pages:order` | owner, or `comics:write:any` | `[page_id…]` |
| DELETE | `/api/v1/pages/{id}` | owner, or `comics:delete:any` (moderation) | removes the page row only; the media asset is untouched (P0.1) |
| GET | `/api/v1/chapters/{id}/pages` | published: `comics:read`; own draft: owner | reader payload: `{pages: [{page_id, url(medium), width, height}]}` — draft invisibility applies here too (404, not 403) |
| PUT | `/api/v1/comics/{id}/progress` | authenticated | `{chapter_id, page_id}`; membership-validated (P0.4) |
| POST | `/api/v1/chapters/{id}/pages:import-zip` | owner, or `comics:write:any` | P1.7; body `{upload_ref: <storage key>}` (the zip is pre-uploaded via a dedicated presigned PUT — P1.7) |

Problem types: `comic/invalid-page-asset`, `comic/invalid-cover-asset`,
`comic/invalid-progress-target`, `comic/not-publishable`, `comic/not-found`,
`comic/zip-rejected`.

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
5. P1: zip import (1 day), reader modes + bookmarks (1 day)
P0 ≈ 4–5 dev-days; P1 adds ~2.

## 10. Open questions

- **(engineering, non-blocking)** Reader payload URL strategy: direct storage URLs
  (current HLS behavior — public-ish) vs API-proxied. Follow the current media
  convention now; both flip together when playback ACL lands (deferred item).
- **(product, non-blocking)** Should `comic_chapters.sort_order` be user-visible
  chapter numbers, or is display numbering derived (1..N)? Recommendation: derived
  display numbering; `sort_order` stays an internal ordering key.

## 11. Revision history

| Rev | Date | Change |
|---|---|---|
| r1 | 2026-07-05 | Initial spec from brief 02 — the reference media→domain vertical. |
| r2 | 2026-07-08 | Reading progress keyed by `page_id` (not array index); asset-deletion coupling via `media:asset_deleted` (P0.6); `DEFERRABLE`-unique reorder pattern. |
| r3 | 2026-07-10 | Code-verified reconciliations: RBAC reworked to **owner-or-elevated** (`RequireOwnerOrPermission`; new `comics:write:any`/`comics:publish:any`/`comics:delete:any` moderation grants seeded to editor/admin per the movies precedent) — the earlier `all-via-RequirePermission` on `:own` neither checked ownership nor admitted moderation; permission codes aligned to the 0003 2–3-segment catalog (dropped `comics:read:published`); identity-anchor FK to `users(id)` added to `comics`/`comic_reading_progress` (§6); `DELETE /pages/{id}` added to §7 and P0.1; list endpoints switched to cursor paging (§7); zip-import upload path + entry-scaled poll-timeout defined (P1.7); `comic:chapter_deleted` stream-removal event added (P1.9); P0.5 gained acceptance criteria; template-registry note added to the reader (P0.3) and library (P0.5) views. |
