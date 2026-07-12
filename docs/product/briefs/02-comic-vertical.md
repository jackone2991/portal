# 02 — Comic Vertical (end-to-end)

**Module:** `comic` (skeleton: `module.go` + `api/` stub) · **Depends on:** spec 01 (image kind).
**Ref:** [feature-inventory.md §7](../feature-inventory.md) (was `feature.md`). Replaces the placeholder `/library/comic` views. **Spec:** [SPEC-02](../specs/SPEC-02-comic-vertical.md).

## Problem statement

All four domain verticals are skeletons. Comic is chosen first because (a) it is the
entertainment-axis domain the user asked for, (b) it is the cheapest proof of the
**media → domain vertical** pattern (a comic reader is a sequenced-image viewer on
top of spec 01), and (c) the frontend already renders placeholder library views to replace.

## Goals

- One real comic readable end-to-end: create → upload pages → publish → read →
  progress remembered.
- The module is the reference implementation for the vertical pattern
  (migration → `query/` → repository → service/handler → `MountHTTP` → real view),
  to be copied by movie/music/story later.

## Non-goals

- Comments/ratings on comics (social layer, later).
- Follow/subscribe to a series, new-chapter notifications (needs notification module).
- Import from external sources / scraping.
- Single-page & double-page reader modes are P1, not P0 (vertical scroll first —
  dominant mode for webtoon-style reading and the simplest to build).

## User stories

- As a creator, I create a comic, add a chapter, upload its page images in order,
  and publish, so readers can see it in the library.
- As a reader, I open a published comic, scroll through a chapter, and when I come
  back later I resume where I left off.
- As a creator, my unpublished draft is invisible to other users.

## Requirements

### P0 — must have

1. **Entities + CRUD**: comic (title, description, cover asset, status
   draft|published), ordered chapters, ordered pages (each page = one image asset ID
   obtained via `mediaapi` — no cross-module FK/JOIN, validated at write time).
   - [ ] Creating a page with a non-existent or non-image asset ID is rejected (422).
   - [ ] Deleting a chapter re-orders cleanly; page order is stable and explicit
         (`sort_order` int, gaps allowed).
2. **RBAC**: `comics:create` / `comics:update:own` / `comics:publish:own` for the
   creator role; `comics:read:published` for authenticated users. Wildcard
   (`comics:*`) covers every scope per the existing grammar. All checks via
   `RequirePermission`.
   - [ ] A non-creator gets 403 on create; a reader gets 404/403 on someone's draft.
3. **Reader (vertical scroll)**: chapter view streams pages top-to-bottom using the
   `medium` variant, lazy-loading with the next 2–3 pages preloaded; chapter
   prev/next navigation.
   - [ ] First page visible < 2s on a normal connection (uses variants, never originals).
4. **Reading progress**: per user × comic — last chapter + page index; upsert
   debounced from the reader; "Continue reading" surfaces on the comic detail page.
   - [ ] Reopening the comic after reading to ch.2 p.14 lands within 1 page of that spot.
5. **Library + detail pages**: `/library/comic` lists published comics (cover, title,
   chapter count); detail page shows chapters + continue button. Replaces placeholders.

### P1 — nice to have

6. Single-page and double-page reader modes (mode toggle persisted per user).
7. **Zip upload**: upload one `.zip` of images → worker unpacks, creates page assets
   in filename order (consumes spec 01 P2 bulk-upload; this is its concrete use case).
8. Bookmarks (per user, per page).
9. Emit `comic:chapter_published` on the bus (life-stream producer #2).

### P2 — future considerations

10. Publish workflow with review (mirrors story module when it lands).
11. Reading-direction RTL support (manga).

## Data model sketch (next free migrations, `000N_comic_*`)

```
comics(id, owner_user_id, title, description, cover_asset_id uuid,
       status text check in ('draft','published'), created_at, updated_at)
comic_chapters(id, comic_id fk, title, sort_order int, created_at)
comic_pages(id, chapter_id fk, asset_id uuid /* media, no FK */, sort_order int)
comic_reading_progress(user_id, comic_id, chapter_id, page_index int,
                       updated_at, pk(user_id, comic_id))
```

## API sketch

```
GET    /api/v1/comics                      published list (paginated)
POST   /api/v1/comics                      creator
GET    /api/v1/comics/{id}                 detail + chapters
PATCH  /api/v1/comics/{id}                 update / publish
POST   /api/v1/comics/{id}/chapters
POST   /api/v1/chapters/{id}/pages         [{asset_id, sort_order}]
GET    /api/v1/chapters/{id}/pages         reader payload (variant URLs)
PUT    /api/v1/comics/{id}/progress        {chapter_id, page_index}
```

## Open questions

- **(product, non-blocking)** Does "published" mean visible to *all* authenticated
  users, or is per-comic visibility needed at v1? Recommendation: all-authenticated
  at v1; visibility scoping arrives with the social/privacy layer.
- **(engineering, blocking for P1.7)** Zip size ceiling & unpack sandboxing in the
  worker (zip-bomb guard) — decide before building zip upload.
