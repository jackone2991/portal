# Test Cases — SPEC-02 Comic Vertical

**Spec:** [SPEC-02](../product/specs/SPEC-02-comic-vertical.md) · **Module:** `comic`
**Prefix:** `TC-COMIC-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md) · **Depends on:** SPEC-01 (ready image assets)

### Endpoints under test

| Method | Path | Perm |
|---|---|---|
| GET | `/api/v1/comics?cursor=` | `comics:read` (drafts excluded) |
| GET | `/api/v1/comics/mine?cursor=` | `comics:write:own` (incl. drafts) |
| POST | `/api/v1/comics` | `comics:write:own` |
| GET | `/api/v1/comics/{id}` | published `comics:read`; own draft: owner |
| PATCH | `/api/v1/comics/{id}` | owner or `comics:write:any`; `{status}` → owner+`comics:publish:own` or `comics:publish:any` |
| DELETE | `/api/v1/comics/{id}` | owner or `comics:delete:any` |
| POST | `/api/v1/comics/{id}/chapters` | owner or `comics:write:any` |
| PATCH/DELETE | `/api/v1/chapters/{id}` | owner or `comics:write:any` |
| PUT | `/api/v1/comics/{id}/chapters:order` | owner or `comics:write:any` |
| POST | `/api/v1/chapters/{id}/pages` | owner or `comics:write:any` |
| PUT | `/api/v1/chapters/{id}/pages:order` | owner or `comics:write:any` |
| DELETE | `/api/v1/pages/{id}` | owner or `comics:delete:any` |
| GET | `/api/v1/chapters/{id}/pages` | published `comics:read`; own draft: owner |
| PUT | `/api/v1/comics/{id}/progress` | authenticated |
| POST | `/api/v1/chapters/{id}/pages:import-zip` | owner or `comics:write:any` (P1.7) |

### Preconditions

- Accounts `owner`(creator), `userA`(user), `userB`(user), `editor`(comics:write:any),
  `admin`(comics:delete:any), `guest`.
- `owner` has ≥3 ready **image** assets (from SPEC-01) + a non-image (video) asset +
  a `processing` asset for negative validation.
- Problem types: `comic/invalid-page-asset`, `comic/invalid-cover-asset`,
  `comic/invalid-progress-target`, `comic/not-publishable`, `comic/not-found`,
  `comic/zip-rejected`.

---

## P0.1 — Entities + CRUD + asset validation

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-COMIC-001 | Create comic (min fields) | Functional | P0 | POST `/comics`{title} as owner | 201; `status=draft`; owner set | ☐ |
| TC-COMIC-002 | Title empty → 422 | Boundary/Neg | P0 | POST title="" | 422 | ☐ |
| TC-COMIC-003 | Title > 200 chars → 422 | Boundary/Neg | P0 | POST 201-char title | 422 | ☐ |
| TC-COMIC-004 | Cover asset invalid (nonexistent) | Negative | P0 | POST/PATCH cover_asset_id=random uuid | 422 `comic/invalid-cover-asset`; no change | ☐ |
| TC-COMIC-005 | Cover asset non-image | Negative | P0 | cover_asset_id = video asset | 422 `comic/invalid-cover-asset` | ☐ |
| TC-COMIC-006 | Cover asset non-ready | Negative | P0 | cover = `processing` asset | 422 `comic/invalid-cover-asset` | ☐ |
| TC-COMIC-007 | Cover asset not owned | Negative | P0(S1) | owner sets cover = userB's asset | 422 `comic/invalid-cover-asset` (validated via mediaapi) | ☐ |
| TC-COMIC-008 | Add page invalid asset (all 4 cases) | Negative | P0 | POST pages with nonexistent/non-image/non-ready/not-owned asset | 422 `comic/invalid-page-asset`; **no row** | ☐ |
| TC-COMIC-009 | Add valid pages | Functional | P0 | POST chapter then pages [{asset,sort}] | 201; rows created | ☐ |
| TC-COMIC-010 | Page asset uniqueness | Negative | P0 | reference same asset_id in two pages | 2nd rejected (unique index on asset_id) | ☐ |
| TC-COMIC-011 | Delete page drops row only | Functional | P0 | DELETE `/pages/{id}` | `comic_pages` row gone; **referenced media asset untouched** | ☐ |
| TC-COMIC-012 | Sort_order stability after delete | Functional | P0 | pages 10,20,30; delete 20; GET pages | reader order stable (10,30), no renumber needed | ☐ |
| TC-COMIC-013 | Chapter delete cascades pages, not assets | Functional | P0 | DELETE chapter with pages | page rows removed (cascade); underlying assets **not** deleted | ☐ |
| TC-COMIC-014 | Reorder pages (full list) | Functional | P0 | PUT `pages:order` [page_id…] reversed | all sort_orders rewritten in one tx (DEFERRABLE unique); no conflict | ☐ |
| TC-COMIC-015 | Reorder chapters | Functional | P1 | PUT `chapters:order` [chapter_id…] | order applied atomically | ☐ |

## P0.2 — Publish flow + RBAC (owner-or-elevated)

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-COMIC-030 | No `comics:write:own` → 403 | AuthZ | P0 | user without grant POST `/comics` | 403 | ☐ (CC-2) |
| TC-COMIC-031 | Draft invisible to other reader | AuthZ | P0(S1) | userA reader fetches userB creator's **draft** detail/pages/list | absent from lists; detail/pages **404 not 403** (no existence leak) | ☐ (CC-3) |
| TC-COMIC-032 | Publish empty chapter → 422 | Negative | P0 | publish comic with a chapter having 0 pages | 422 `comic/not-publishable` naming offending chapter(s) | ☐ |
| TC-COMIC-033 | Publish requires ≥1 chapter | Negative | P0 | publish comic with 0 chapters | 422 `comic/not-publishable` | ☐ |
| TC-COMIC-034 | Valid publish | Functional | P0 | publish comic w/ ≥1 chapter each ≥1 page (owner+`comics:publish:own`) | 200; `status=published`; appears in `/comics` | ☐ |
| TC-COMIC-035 | Unpublish → draft | Functional | P1 | PATCH status=draft | returns to `draft`; leaves public list | ☐ |
| TC-COMIC-036 | `:own` alone can't cross-edit | AuthZ | P0(S1) | creatorD (comics:write:own) edits creatorC's comic | 404/403; no change (`:own` must not grant cross-owner writes) | ☐ (CC-2) |
| TC-COMIC-037 | `comics:write:any` cross-edit allowed | AuthZ | P0 | editor edits another creator's comic | 200; change applied | ☐ |
| TC-COMIC-038 | `comics:delete:any` allowed | AuthZ | P0 | admin DELETE another creator's comic | 2xx; deleted | ☐ |
| TC-COMIC-039 | `comics:publish:any` allowed | AuthZ | P1 | editor publishes another creator's comic | 200 | ☐ |
| TC-COMIC-040 | Permission seeding present | AuthZ | P0 | inspect seed grants | `comics:read`→user, `write/publish:own`→creator, `write/publish:any`→editor, `delete:any`→admin | ☐ (CC-2) |

## P0.3 — Reader (vertical scroll)

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-COMIC-060 | First page < 2 s (Fast 3G) | Performance | P0 | open 40-page chapter on throttled Fast-3G | page 1 visible < 2 s | ☐ [MANUAL] |
| TC-COMIC-061 | No layout shift (CLS < 0.1) | Performance | P0 | scroll to page 5 | no layout jump; `<img>` carries width/height from variant row | ☐ [MANUAL] |
| TC-COMIC-062 | Uses `medium` variants | Functional | P0 | inspect page image URLs | reader uses `medium` variant, never originals | ☐ |
| TC-COMIC-063 | Lazy-load + preload next 2–3 | Frontend | P1 | scroll; watch network | next 2–3 pages preloaded via IntersectionObserver; not whole chapter up front | ☐ [MANUAL] |
| TC-COMIC-064 | End-of-last-chapter panel | Frontend | P0 | reach last chapter end | panel offers "back to comic" (no dead end) | ☐ |
| TC-COMIC-065 | Per-page error tile on bad asset | Reliability | P0 | one page's asset failed/deleted | per-page error tile; chapter continues (one bad page never blanks it) | ☐ |
| TC-COMIC-066 | Reader via template registry | Frontend | P1 | inspect route resolution | view via `activeTemplate().views.<x>`; no version import in `app/` | ☐ |

## P0.4 — Reading progress (keyed by page_id)

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-COMIC-080 | Resume within ±1 page | Functional | P0 | read ch2 p14, close, reopen via Continue | lands within ±1 page of p14 | ☐ |
| TC-COMIC-081 | Insert/reorder drift regression | Functional | P0(S1) | save progress on page; creator inserts pages before it / reorders | Continue still lands on the **same page** (page_id keyed, not index) | ☐ |
| TC-COMIC-082 | Deleted page → chapter-top fallback | Reliability | P0 | delete the progress page (FK SET NULL) | Continue falls back to chapter top; no crash | ☐ |
| TC-COMIC-083 | Deleted chapter → first-chapter fallback | Reliability | P0 | delete the progress chapter | Continue falls back to first chapter; no crash | ☐ |
| TC-COMIC-084 | Membership validation | Negative | P0(S1) | PUT progress with chapter_id not in comic, or page_id not in chapter | 422 `comic/invalid-progress-target`; **no row** (prevents cross-comic deep-link) | ☐ |
| TC-COMIC-085 | Cannot write progress on foreign draft | AuthZ | P0 | reader writes progress on another user's draft comic | 404 (draft invisibility covers it) | ☐ (CC-3) |
| TC-COMIC-086 | Detail shows "Continue → ch N, p M" | Functional | P1 | with progress, load detail | shows current 1-based position (computed at fetch time) | ☐ |
| TC-COMIC-087 | Debounced beacon cadence | Frontend | P1 | read; watch progress writes | upsert every ~10 s while furthest page changes + on pagehide | ☐ [MANUAL] |
| TC-COMIC-088 | Progress survives stack restart | Reliability | P0 | save progress; `make up` restart; reopen | progress persists (DB not cache) | ☐ |

## P0.5 — Library + detail pages

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-COMIC-100 | Reader sees only published | AuthZ | P0 | userA opens `/library/comic` (published + others' draft exist) | only published shown (cover=thumb, title, chapter count, updated date) | ☐ (CC-3) |
| TC-COMIC-101 | My comics tab shows drafts | Functional | P0 | creator opens My comics | both draft+published w/ status badges + Create button | ☐ |
| TC-COMIC-102 | Pagination no duplicates | Functional | P0 | >1 page of comics | cursor paging, no dupes; order `status, updated_at DESC` | ☐ (CC-4) |
| TC-COMIC-103 | Placeholder component gone | Frontend | P0 | grep route | pre-existing placeholder removed | ☐ (CC-9) |
| TC-COMIC-104 | Detail Continue iff progress | Functional | P1 | detail with/without progress | Continue shown only when progress exists | ☐ |

## P0.6 — Asset-deletion coupling

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-COMIC-120 | Page reaped on media delete | Integration | P0(S1) | delete a page's asset media-side; event handled | `comic_pages` row gone; remaining pages keep stable order; progress FK SET NULL | ☐ (CC-5) |
| TC-COMIC-121 | Cover NULLed on media delete | Integration | P0 | delete cover asset media-side | `cover_asset_id=NULL`; library card falls back to placeholder (no broken image) | ☐ |
| TC-COMIC-122 | Idempotent + order-tolerant | Idempotency | P0 | redeliver same `asset_id`; deliver unknown `asset_id` | no-op both times (best-effort) | ☐ (CC-5) |
| TC-COMIC-123 | Consumer registered on emitting binary | Integration | P0(S1) | delete asset via api; verify comic reap fires | api publisher has `media:asset_deleted → comic:on_asset_deleted` edge (regression for empty-routing bug) | ☐ (CC-5) |

## P1 — nice to have

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-COMIC-140 | Zip import happy path | Functional | P1 | pre-upload valid `chapter.zip` (import/ presign, 500 MB range); POST `pages:import-zip`{upload_ref} | worker on `default` queue unpacks, batch-creates assets (origin=import), polls ready, creates pages in filename natural-sort; zip object deleted after | ☐ [P1] |
| TC-COMIC-141 | Zip > 500 MB rejected | Boundary | P1 | oversize zip | rejected `comic/zip-rejected` | ☐ [P1] |
| TC-COMIC-142 | Zip > 300 entries rejected | Boundary | P1 | 301-entry zip | rejected | ☐ [P1] |
| TC-COMIC-143 | Zip path traversal / nested dirs rejected | Security | P1 | `traversal.zip` (`../`), `nested_dirs.zip` | rejected | ☐ [P1] |
| TC-COMIC-144 | Zip-bomb (ratio >100:1) rejected | Security | P1 | `zipbomb.zip` | rejected | ☐ [P1] |
| TC-COMIC-145 | Non-image entries rejected by magic bytes | Negative | P1 | zip with a .txt | that entry excluded/failed; per-file report | ☐ [P1] |
| TC-COMIC-146 | Poll timeout → per-file failure report | Reliability | P1 | asset stuck > timeout | reported as failure; chapter gets the succeeded pages | ☐ [P1] |
| TC-COMIC-147 | `comic:chapter_published` emitted | Integration | P1 | publish comic | `comic:chapter_published {comic_id,chapter_id,owner_user_id,title}` per chapter | ☐ (CC-5) |
| TC-COMIC-148 | `comic:chapter_deleted` on delete | Integration | P1 | delete chapter / delete comic | `comic:chapter_deleted` on chapter delete + once per chapter on comic delete (stream removes cards) | ☐ (CC-5) |

## Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-COMIC-160 | All non-2xx RFC-7807 | Contract | P0 | exercise error paths | Problem+json + stable type | ☐ (CC-1) |
| TC-COMIC-161 | Problem types have i18n keys | Contract | P1 | grep problems.ts | all comic types present | ☐ (CC-1) |
| TC-COMIC-162 | Owner isolation sweep | AuthZ | P0(S1) | userA vs userB across all comic reads | 404 for foreign ids; lists never leak | ☐ (CC-3) |
| TC-COMIC-163 | Idempotent delete | Idempotency | P0 | DELETE comic/page twice | 2nd → 404, never 500 | ☐ (CC-8) |
| TC-COMIC-164 | Migration up/down | Contract | P1 | migrate + down `comic_core` | clean; DEFERRABLE uniques + identity-anchor FKs present | ☐ |
