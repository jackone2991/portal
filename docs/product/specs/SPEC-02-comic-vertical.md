# SPEC-02 — Comic Vertical (end-to-end)

**Module:** `comic` (skeleton: `module.go` + `api/` stub) · **Depends on:** SPEC-01 (image kind)
**Upstream:** `feature/02-comic-vertical.md` · **Refs:** feature.md §7, frontend.md Phase 4
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
delete semantics sane.

**Acceptance criteria.**
- Given a page create with a nonexistent, non-image, non-ready, or not-owned
  asset id, then 422 Problem `comic/invalid-page-asset` and no row.
- Given pages inserted with sort_order 10,20,30, when 20 is deleted, then reader
  order is stable (10,30) with no renumbering required.
- Given a chapter delete, then its pages rows are removed (cascade) but the
  underlying media assets are **not** deleted (assets have their own lifecycle).
- Title > 200 chars or empty → 422.

### P0.2 — Publish flow + RBAC

Permissions (all via `RequirePermission`; wildcard `comics:*` covers every scope):
| Action | Permission |
|---|---|
| create comic / chapter / pages | `comics:create` |
| update / reorder / delete own | `comics:update:own` |
| publish / unpublish own | `comics:publish:own` |
| read published | `comics:read:published` |
| read own drafts | implied by `comics:update:own` |

Publish validation: a comic may be published only if it has ≥1 chapter and every
chapter has ≥1 page; otherwise 422 `comic/not-publishable` listing the offending
chapters. Unpublish returns it to `draft`.

**Acceptance criteria.**
- Given a user without `comics:create`, when POST /comics, then 403.
- Given reader R and creator C's draft, when R fetches detail/pages/list, then the
  draft is absent from lists and detail returns 404 (not 403 — don't leak existence).
- Given a comic with an empty chapter, when publish is attempted, then 422 naming
  the chapter.
- Given an admin with `comics:*`, then all scopes pass (regression test for the
  wildcard-action rule).

### P0.3 — Reader (vertical scroll)

Route `/library/comic/[id]/read/[chapterId]` (client component; catalogue/detail
stay RSC-first per D-33). Behavior: pages render top-to-bottom using the `medium`
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

Table `comic_reading_progress(user_id, comic_id, chapter_id, page_index)`,
PK `(user_id, comic_id)`. Reader upserts debounced: every 10 s while the furthest
visible page changes, plus on pagehide — mirroring the watch-progress convention
(frontend.md Phase 2-3). Detail page shows **Continue reading → ch. N, p. M** when
progress exists; opening via Continue scrolls to that page.

**Acceptance criteria.**
- Given reading to ch.2 p.14 then closing the tab, when the comic is reopened via
  Continue, then the view lands within ±1 page of p.14.
- Given a progress row pointing at a since-deleted chapter, then Continue falls back
  to the first chapter (no crash).
- Progress writes require only authentication on a comic the user can read; a reader
  cannot write progress on another user's draft (404 path already covers this).

### P0.5 — Library + detail pages

`/library/comic`: grid of published comics — cover (`thumb` variant), title, chapter
count, updated date; pagination; replaces the placeholder. Creator additionally sees
a "My comics" tab including drafts with status badges and a Create button.
`/library/comic/[id]`: cover, title, description, chapter list (title + created
date), Continue button, creator-only Edit/Publish controls.

### P1 — nice to have

- **P1.6 Reader modes**: single-page and double-page modes with keyboard/tap
  navigation; mode persisted per user (server-side pref or localStorage — note the
  artifact restriction doesn't apply to the real app; pick server-side pref for
  cross-device).
- **P1.7 Zip chapter upload**: `POST /api/v1/chapters/{id}/pages:import-zip` — one
  .zip of images → worker task `comic:import_zip` unpacks, creates media assets via
  `mediaapi` (batch), creates pages in **filename natural-sort order**. Guards
  (blocking question resolved here): max 500 MB zip, max 300 entries, images only by
  magic bytes, reject nested directories and path traversal (`../`), reject
  compression ratio > 100:1 (zip-bomb). Failures produce a per-file report; the
  chapter gets the pages that succeeded.
- **P1.8 Bookmarks**: per user, per page; list on the detail page.
- **P1.9 Event**: emit `comic:chapter_published` `{comic_id, chapter_id, owner_user_id,
  title}` — life-stream producer #2.

### P2 — future considerations

- Review/approval publish workflow (mirror story module when it lands).
- RTL reading direction; double-page spread pairing metadata.
- Per-comic visibility (unlisted/link-only) once the privacy layer exists.

## 6. Data model — migration `000N_comic_core`

```sql
CREATE TABLE comics (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id  uuid NOT NULL,            -- account module id, no cross-module FK
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
  UNIQUE (comic_id, sort_order)
);

CREATE TABLE comic_pages (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chapter_id uuid NOT NULL REFERENCES comic_chapters(id) ON DELETE CASCADE,
  asset_id   uuid NOT NULL UNIQUE,         -- media asset, validated via mediaapi
  sort_order int  NOT NULL,
  UNIQUE (chapter_id, sort_order)
);

CREATE TABLE comic_reading_progress (
  user_id    uuid NOT NULL,
  comic_id   uuid NOT NULL REFERENCES comics(id) ON DELETE CASCADE,
  chapter_id uuid NOT NULL,
  page_index int  NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, comic_id)
);
```

`UNIQUE (comic_id, sort_order)` means reorders use a two-phase update or negative
temp values — the service layer owns reordering; the API takes a full ordered id
list per chapter (`PUT .../pages:order`).

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission |
|---|---|---|
| GET | `/api/v1/comics?page=` | `comics:read:published` (drafts excluded) |
| GET | `/api/v1/comics/mine` | `comics:update:own` |
| POST | `/api/v1/comics` | `comics:create` |
| GET | `/api/v1/comics/{id}` | published: `comics:read:published`; own draft: owner |
| PATCH | `/api/v1/comics/{id}` | `comics:update:own` (incl. `{status}` w/ `comics:publish:own`) |
| DELETE | `/api/v1/comics/{id}` | `comics:update:own` |
| POST | `/api/v1/comics/{id}/chapters` | `comics:update:own` |
| PATCH/DELETE | `/api/v1/chapters/{id}` | `comics:update:own` |
| POST | `/api/v1/chapters/{id}/pages` | `comics:update:own` — `[{asset_id, sort_order}]` |
| PUT | `/api/v1/chapters/{id}/pages:order` | `comics:update:own` — `[page_id…]` |
| GET | `/api/v1/chapters/{id}/pages` | reader payload: `[{page_id, url(medium), width, height}]` |
| PUT | `/api/v1/comics/{id}/progress` | authenticated — `{chapter_id, page_index}` |
| POST | `/api/v1/chapters/{id}/pages:import-zip` | P1.7 |

Problem types: `comic/invalid-page-asset`, `comic/not-publishable`,
`comic/not-found`, `comic/zip-rejected`.

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
