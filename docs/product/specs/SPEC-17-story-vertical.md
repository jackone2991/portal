# SPEC-17 — Story Vertical (long-form text: stories and chapters)

**Status:** current, rev 2 · **Drafted:** 2026-10-01 · **Last verified:** 2026-10-01
**Module:** `story` · **Depends on:** SPEC-04 (image assets for covers, `media:asset_deleted`, P0.8 `tenant` visibility and `mediaapi.SetVisibility` for the cover — Decision 2026-10-02b (B13)); pattern copied from SPEC-14 (parent + ordered children, `DEFERRABLE` reorder, publish validation)
**Upstream:** as-built spec, written retroactively on 2026-10-01 from the shipped code (`backend/internal/modules/story/`, migration `0023_story_core`) and the decisions it rests on — there was never a brief. It also takes ownership of the story list envelopes the specs README Pagination convention listed as unowned (`{chapters}`, and `{stories}`) · **Refs:** [ADR-07](SPEC-01-account-identity-admin.md#adr-07) (tenancy), [ADR-08](README.md#adr-08), [SPEC-14](SPEC-14-comic-vertical.md) (the reference vertical), feature-inventory §6, `D-7` (RFC 7807), `D-20` (per-domain progress), `D-29` (spec-first OpenAPI, `{items}`), `D-32`/`D-33` (frontend), [backlog.md](../backlog.md) P2 lines 28–29
**Downstream consumers:** SPEC-05 (`notify:on_story_published` bell), SPEC-10 (a future `story` leg of `/continue`), SPEC-03 P1.7 (takeout)

---

## 1. Problem statement

Portal stores videos, music, images and comics, but nothing a person *writes at
length*: a novel in progress, a serial, a translated web-novel read chapter by
chapter. The journal (SPEC-07) is for moments, not for a 40-chapter text with an
order, a cover and a "this is ready" switch.

The `story` module is that shelf. A story owns ordered chapters whose bodies are
inline Markdown; it has an optional cover image and moves between `draft` and
`published`. It was built on 2026-07-19 (`f11cf3f`, migration `0023_story_core`)
as a slimmed copy of comic (SPEC-14): text chapters instead of image pages, no
reading progress. No spec was written at the time; this one records what
shipped, states the contract the code should meet, and lists every place it
does not (§11). The reading UI was never built — `/library/novel/[id]` renders a
placeholder — so today the module is a working API without a reader.

## 2. Goals

1. The owner creates a story, writes chapters in Markdown, orders them, and
   publishes it once every chapter has a body.
2. A published story and its chapters are readable by every authenticated member
   of the owner's tenant; a draft — detail, chapter list or reader payload — is
   invisible to everyone but its owner (404, never 403).
3. Chapter order is explicit and survives inserts, deletes and reorders without
   renumbering storms or unique-constraint failures.
4. A deleted cover never leaves a story pointing at a missing asset.

## 3. Non-goals

- **Authors, bookmarks, ratings, comments.** The module README's planned
  `story_authors` / `story_bookmarks` tables are not built and not scheduled.
- **Inline media in chapters.** Bodies are Markdown text; images in a chapter
  would be SPEC-08-style attachments, not specced. The README's "audio
  narration assets" is likewise unbuilt.
- **Rendering policy for Markdown on the server.** The API stores and returns
  raw Markdown; sanitising and rendering is the reader's job (P1.1).
- **Visibility beyond the tenant** — "published" never crosses the tenant
  fence, as for movies (Decision 2026-10-02b (B6), SPEC-16 §10).
- **Search** — FTS (`D-2`) waits for a corpus ([backlog.md](../backlog.md) P2
  line 29).

## 4. User stories

- As the owner, I create a story with a title, a blurb and a cover, add chapters
  as I write them, and publish when every chapter has text.
- As the owner, I try to publish with an empty chapter and am told exactly which
  chapter is empty.
- As the owner, I move chapter 7 before chapter 3 in one operation.
- As the owner, I keep a story in draft while writing; nobody else sees it in
  lists, by id, or through its chapter list.
- As a reader in the owner's tenant, I open a published story, see the table of
  contents, and read chapter bodies in order. *(API only until P1.1.)*
- Edge: as an editor (`stories:write:any`), I fix a chapter in someone else's
  story; I cannot set my own image as its cover.

## 5. Requirements

### P0.1 — Module scaffold and wiring

`backend/internal/modules/story/` follows MODULES.md §8: `module.go` (`New`,
`MountHTTP`, `RegisterTasks`, `OwnerByStory`, `OwnerByChapter`), `api/` (task and
event names, payload structs), `query/story.sql`, `repository/` (sqlc output +
`adapter.go`, which takes a `RunInTx` for the reorder), handler, service and
types at the package root. `cmd/api/main.go` constructs it with the media API,
the shared publisher, `tdb.RunInTx` and four owner-or-elevated guards and mounts
it; `cmd/worker/main.go` constructs it with the repository and registers
`story:on_asset_deleted` on the light server's mux. Chapter-level routes live
under `/story-chapters/{id}` so they never collide with comic's `/chapters/{id}`.

**Acceptance criteria.**
- `grep -n 'storyMod' backend/cmd/api/main.go backend/cmd/worker/main.go` shows
  `story.New(` and `MountHTTP` in the API and `RegisterTasks(lightMux)` in the
  worker.

### P0.2 — Story entity, create and update

A story is `{title, description?, cover_asset_id?, status}` owned by
`owner_user_id` (§6).

- **Title**: trimmed, 1–200 runes; otherwise 422 `story/validation`.
- **Description**: optional free text.
- **Cover**: optional; must name an asset `mediaapi.GetAsset` returns with
  `kind = image`, `status = ready`, `owner_id = stories.owner_user_id` (the
  story's owner, whoever the caller is) — else 422 `story/invalid-cover-asset`
  and nothing is written. No cross-module FK; reaped by P0.6.

`POST /stories` creates a `draft` (201, the Story). `PATCH /stories/{id}` is
partial; for `cover_asset_id` absent leaves it, `null` clears it. For
`description`, `null` clears the field, as `StoryPatch` documents. *(Code
follow-up: `query/story.sql` `UpdateStory` uses `COALESCE`, so `null` is
ignored — §11 row 13.)* `status` is never changed by PATCH; every update sets
`updated_at = now()`. A `mediaapi` lookup failure (not "no such asset") is a
500, not a 422. *(Code follow-up: §11 row 6.)*

**Acceptance criteria.**
- Given a cover owned by another user, or a video as the cover, then 422
  `story/invalid-cover-asset` and no row. *(TC-STY-001, 002)*
- Given a blank title or one of 201 runes, then 422 `story/validation`.
  *(TC-STY-003)*
- Given an editor PATCHing C's story with a cover the editor owns, then 422
  `story/invalid-cover-asset`. *(TC-STY-004)*
- Given `PATCH {description: null}`, then `description` is null afterwards.
  *(TC-STY-005)*
- Given malformed JSON or `cover_asset_id: "x"`, then 422 `story/validation`.
  *(TC-STY-006)*

### P0.3 — Chapters and ordering

A chapter is `{title, body_md, sort_order}` belonging to one story.

- **Title**: trimmed, 1–200 runes; **body**: Markdown, at most 200,000 runes
  (`maxBodyLen`); violations → 422 `story/validation`. Request bodies on every
  story route may be up to 4 MiB (`storyBodyLimit`, four times the platform
  default — a chapter is prose). An empty body is allowed on a draft; it is what
  blocks publish (P0.5). After publish a blank chapter is hidden from
  non-owners instead (P0.4).
- **Create** `POST /stories/{id}/chapters` (201, the Chapter). `sort_order` is
  optional: absent → the server appends after the story's current maximum
  (`MAX(sort_order) + 10`, or 10 for the first chapter); present → it must not
  equal another chapter's `sort_order`, else 422 `story/validation`. Gaps are
  allowed; display numbering is derived from position, never from
  `sort_order` (SPEC-14 §10 recommendation). *(Code follow-up: HEAD inserts the
  client value verbatim, an absent value decodes to 0, and a duplicate trips
  the `DEFERRABLE` unique only at COMMIT — the client gets a bare 500; §11
  row 2.)*
- **Update** `PATCH /story-chapters/{id}` takes `title?` and `body_md?` (absent
  = unchanged); 404 `story/not-found` when the chapter is gone.
- **Delete** `DELETE /story-chapters/{id}` is 204; the remaining chapters keep
  their order with no renumbering.
- **Reorder** `PUT /stories/{id}/chapters:order {order: [chapter_id…]}` (204) is
  the SPEC-14 §6 canonical pattern: the body is the **complete** ordered set of
  the story's chapter ids, and inside one transaction every row is rewritten to
  `(index + 1) × 10`; the `UNIQUE (story_id, sort_order) DEFERRABLE INITIALLY
  DEFERRED` constraint makes transient duplicates legal until COMMIT. A list
  that omits a chapter, repeats one, or names an id that is not one of this
  story's chapters is 422 `story/validation` and nothing changes. *(Code
  follow-up: HEAD applies whatever it is given — foreign ids match no row,
  omitted chapters keep their old value and can collide at COMMIT (500), and
  OpenAPI calls a partial list "a truncation", which it is not; §11 row 3.)*
- Every UPDATE sets `updated_at = now()`, the reorder included. *(Code
  follow-up: `UpdateStoryChapterOrder` does not — §11 row 16.)*

Chapter mutations are guarded like the story they belong to (P0.5 table); they
emit no event and never change the story's status.

**Acceptance criteria.**
- Given two chapters created without `sort_order`, then they get 10 and 20 and
  both requests are 201. *(TC-STY-020)*
- Given a create whose `sort_order` equals an existing chapter's, then 422
  `story/validation` and no row. *(TC-STY-021)*
- Given chapters at 10, 20, 30 and the middle one deleted, then the reader
  order is 10, 30. *(TC-STY-022)*
- Given `order = [c3, c1, c2]`, then the reader returns c3, c1, c2 with
  `sort_order` 10, 20, 30. *(TC-STY-023)*
- Given an `order` missing one of the story's chapters, or containing a chapter
  of another story, then 422 `story/validation` and the order is unchanged.
  *(TC-STY-024)*
- Given a body of 200,001 runes, then 422; given an empty title on PATCH, then
  422. *(TC-STY-025, 026)*

### P0.4 — Visibility, lists and the reader payload

- **Detail** `GET /stories/{id}` returns a `StoryDetail`: the Story plus
  `chapter_count` and `chapters` — summaries `{id, title, sort_order}` in
  reading order, never `body_md` (a composite read keeps its named array,
  specs README Pagination). Published stories are visible to any `stories:read`
  holder; a draft only to its owner, and to anyone else it answers exactly what
  a missing id answers: 404 `story/not-found`. A failure to load the chapters is
  a 500, never a silently empty table of contents. *(Code follow-up: `GetStory`
  discards the error — §11 row 5.)*
- **Reader payload** `GET /stories/{id}/chapters` returns every chapter **with**
  its body in reading order (`sort_order`, then `id`), behind the same
  published-or-owner gate on the story. It is a non-paginated collection, so it
  answers `{items: StoryChapter[]}`. *(Code follow-up: HEAD answers
  `{chapters}` — §11 row 9; this spec owns the retrofit the specs README listed
  as unowned.)*
- **Lists** `GET /stories` (published) and `GET /stories/mine` (own, drafts
  included) are keyset-paginated on (`updated_at DESC`, `id DESC`) and answer
  `{items: Story[], next_cursor?}`. Each item carries `chapter_count`, `0`
  included. *(Code follow-up: HEAD answers `{stories}` and omits
  `chapter_count` when it is 0 — §11 rows 9, 14.)* `limit` default 30, max 50,
  lenient (missing / non-integer / < 1 → 30; above 50 → clamped to 50; never a
  Problem). *(Code follow-up: HEAD resets above-max values to 30 — §11
  row 10.)* A malformed cursor is 400 `story/invalid-cursor`.
- **Tenancy fences visibility first**: `stories` and `story_chapters` are
  tenant-scoped under FORCE RLS (§6), so "published" means visible within the
  owner's tenant — the owner alone with personal orgs — and never beyond it
  (Decision 2026-10-02b (B6); SPEC-16 P0.3).
- **Blank chapters on a published story** (Decision 2026-10-02b (B7), SPEC-14
  P0.2 (a) for consistency with comic). The P0.5 invariant is enforced at
  publish only; afterwards a chapter may be emptied, or a blank one added. A
  chapter is **blank** by the publish predicate: `btrim(body_md) = ''`
  (`EmptyChapters`). On a published story, for every caller but the owner:
  (a) a blank chapter is omitted from the detail's `chapters`, from
  `chapter_count` and from the reader payload — so the reader's table of
  contents and previous/next skip it — until it has a non-blank body. There is
  no per-chapter read route, so the reader payload is the whole fence (comic's
  per-chapter 404 has no counterpart). `GET /stories` counts non-blank chapters
  only, whoever asks (the list is not per caller);
  (b) adding, filling or emptying chapters of a published story emits nothing
  (§8): `story:published` fires only on a publish action;
  (c) emptying or deleting chapters never auto-unpublishes; a published story
  left with no non-blank chapter shows no chapters and `chapter_count: 0`.
  The owner sees every chapter, blank ones included, in the detail, the reader
  payload, `GET /stories/mine` and every mutation response. A holder of
  `stories:write:any` is not the owner and reads what any reader reads. *(Code
  follow-up: HEAD returns and counts every chapter to everyone — §11 row 21.)*

**Acceptance criteria.**
- Given C's draft with one chapter, when stranger S in C's tenant GETs the
  story or its chapters, then both are 404 `story/not-found`, byte-identical to
  a random id; C gets 200. *(TC-STY-040)*
- Given C's draft, then it is absent from `GET /stories` and present in
  `GET /stories/mine` with its `chapter_count`. *(TC-STY-041)*
- Given a story with three chapters, then the detail lists three summaries
  without `body_md`, and the reader payload returns three bodies in the same
  order under `items`. *(TC-STY-042)*
- Given a story with no chapters, then its list item carries
  `chapter_count: 0`. *(TC-STY-043)*
- Given `?limit=500`, then 50; given `?cursor=garbage`, then 400
  `story/invalid-cursor`. *(TC-STY-044, 045)*
- Given 75 published stories, then pages of 30/30/15 under `items`, no
  `stories` key, the last page without `next_cursor`. *(TC-STY-046)*
- Given C's published story with chapters "1" (text) and "2" (body emptied to
  `"   "` after publish), when reader R in C's tenant GETs the detail, the
  reader payload and `GET /stories`, then each shows only chapter 1 and
  `chapter_count: 1`; C sees both chapters and `chapter_count: 2` in the
  detail, the reader payload and `GET /stories/mine`. Once C writes a body
  into chapter 2, R sees it, and no `story:published` is emitted.
  *(TC-STY-047)*
- Given C's published story whose every chapter is then emptied or deleted,
  then it stays `published`, and R gets 200 with `chapters: []`,
  `chapter_count: 0` and an empty reader payload. *(TC-STY-048)*

### P0.5 — Publish, unpublish, delete and RBAC

`POST /stories/{id}/publish` requires **≥ 1 chapter and no chapter whose body is
blank after trimming whitespace** (`btrim(body_md) = ''`). Otherwise 422
`story/not-publishable` with an extension member `chapters: [{id, title}]`
naming every blank chapter in reading order (empty when the story has no
chapters), and the status is unchanged. The Problem is written through
`platform/server.ProblemWith`, the single error writer. *(Code follow-up:
`writeStoryErr` hand-encodes it — §11 row 15.)* On success the status becomes
`published` (200, the Story) and **`story:published`** `{story_id,
owner_user_id, title}` is emitted **after the request transaction commits**.
*(Code follow-up: HEAD publishes before COMMIT — §11 row 4.)* Re-publishing is
allowed; SPEC-05's consumer dedups on the story id. `POST
/stories/{id}/unpublish` sets `draft` (200), emits nothing and is idempotent.
`DELETE /stories/{id}` is 204 (chapters cascade), then 404 `story/not-found`. A
nil or failing publisher is logged and never fails a committed publish.

**The cover follows the story** *(Decision 2026-10-02b (B13), applied to story
because P0.4 already makes a published story readable by every member of the
tenant; unbuilt — §11 row 22)*. The text is a story row and reaches members
through RLS alone, but the cover is a media asset and, while `private`, its
variants are hidden from everyone but the owner. So, exactly as for a movie's
poster (SPEC-16 P0.4), publish raises `cover_asset_id` to SPEC-04 P0.8's
`tenant` visibility through `mediaapi.SetVisibility` with the **story owner**
as `ownerID`, in the same transaction; unpublish, `DELETE` (before the row
goes) and a `PATCH` that clears it lower it to `private` unless another
published story of the owner uses it; a `PATCH` that sets a new cover on a
published story raises it and lowers the old one. Members get the cover's
variants only, never its original (SPEC-04 P0.8). `SetVisibility` never
touches a `public` asset; the `stories:publish:any` and cross-module edges are
SPEC-16 P0.4's.

**Permissions:**

| Action | Permission |
|---|---|
| list published, read a published story, read its chapters | `stories:read` (`RequirePermission`) |
| read own draft (detail, chapters) | owner check in `Service.GetStory` / `ChaptersVisible` |
| create a story, list own | `stories:write:own` (`RequirePermission`) — held by `user` and up once P1.3 lands; `creator` and up on `HEAD` |
| update a story; add or reorder its chapters | owner, or `stories:write:any` — `RequireOwnerOrPermission(engine, "stories:write:any", byStory)` |
| update or delete a chapter | owner, or `stories:write:any`, resolved from the chapter id (`byStoryChapter`) |
| delete a story | owner, or `stories:delete:any` |
| publish / unpublish | `RequirePermission("stories:publish:own")` chained before `RequireOwnerOrPermission(engine, "stories:publish:any", byStory)` (SPEC-14 P0.2). *(Code follow-up: only the second half is wired — §11 row 7.)* |

A guard answers 404 for a draft when the caller is neither the owner nor a
holder of the endpoint's `:any` code. *(Code follow-up: the extractor resolves
the owner whatever the status — §11 row 8; only reachable inside a shared
tenant.)*

**Seeding** (`0023_story_core`): `stories:read` → `user`; `stories:write:own` and
`stories:publish:own` → `creator`; `stories:write:any` and `stories:publish:any`
→ `editor`; `stories:delete:any` → `admin`. 0003 had already seeded
`stories:read` to `guest` and the two-segment `stories:publish` to `editor`. On
`HEAD` a plain `user` reads but cannot write; P1.3 widens the two `:own` codes
to `user`, as for movies (SPEC-16 P1.3) and comics (`0025`).

**Acceptance criteria.**
- Given a story with no chapters, then publish is 422 `story/not-publishable`
  with `chapters: []`. *(TC-STY-060)*
- Given chapters "1" (text) and "2" (`"   "`), then 422 naming only chapter 2.
  *(TC-STY-061)*
- Given every chapter non-blank, then 200 `published` and exactly one
  `story:published` after commit; a failing COMMIT emits none; a failing
  publisher still answers 200. *(TC-STY-062, 063, 064)*
- Given an owner lacking `stories:publish:own`, then publish is 403; given an
  editor with `stories:publish:any`, then unpublishing C's story is 200.
  *(TC-STY-065, 066)*
- Given a `stories:read`-only caller, then `POST /stories` is 403.
  *(TC-STY-067)*
- Given DELETE twice, then 204 then 404 `story/not-found`; the chapters are
  gone. *(TC-STY-068)*
- Given creator D in C's tenant, when D adds a chapter to C's draft, then 404.
  *(TC-STY-069)*
- Given C's draft story with a `private` cover, when it is published, then the
  cover is `tenant` and member M of C's tenant gets its `thumb` variant (and
  no bytes of its original); when it is unpublished, then the cover is
  `private` and M's variant request is 404. *(TC-STY-112)*

### P0.6 — Cover lifecycle (`media:asset_deleted`)

The module consumes `media:asset_deleted` `{asset_id, owner_user_id}` as task
**`story:on_asset_deleted`** and clears `cover_asset_id` (setting `updated_at`)
on every story that used the deleted asset. It never changes the status. It is
idempotent and skips, without retrying, a payload that does not decode or whose
`asset_id` is not a uuid. It runs **inside the payload owner's tenant scope**
(specs README Tenancy). *(Code follow-up: it runs unscoped — §11 row 1.)*

**Acceptance criteria.**
- Given a published story whose cover is deleted, then after the task
  `cover_asset_id` is null and the story is still published. *(TC-STY-080)*
- Given the same event three times, then every run succeeds with the same end
  state. *(TC-STY-081)*
- Given the worker connected as `portal_app`, then the UPDATE commits inside
  the owner's tenant scope. *(TC-STY-082)*

### P1 — next

- **P1.1 Reader and manager UI** *(committed scope — Decision 2026-10-01b
  (D2): story is finished to the music standard, not reverted; unbuilt)*.
  `/library/novel` (published + mine) and `/library/novel/[id]` replace today's placeholder: cover, blurb, table of
  contents from `StoryDetail.chapters`, and a reader that fetches
  `GET /stories/{id}/chapters` and renders sanitised Markdown one chapter at a
  time with previous/next. The owner's manager adds metadata and cover upload
  (`lib/media-upload.ts`), chapter add / edit / delete, drag-to-reorder (one
  `PUT …/chapters:order`), and publish showing the `story/not-publishable`
  chapter list inline. RSC shell plus TanStack islands (D-32, D-33), views in
  `TemplateManifest.views` via `activeTemplate()`. Today:
  `frontend/src/templates/v1/views/library/novel/NovelDetailView.tsx` renders
  "Novel #{id}" and a TODO; `app/(app)/library/page.tsx` links `/library/novel`,
  which has no route; no `lib/story.ts` exists; the bell's
  `/library/novel/{id}` link (`notify/service.go` `workKinds`) lands on the
  placeholder. *AC:* the owner writes, orders, publishes and reads a
  three-chapter story without leaving `/library/novel`.
- **P1.2 Reading progress and the `story` leg of `/continue`.** Per `D-20`, a
  story-owned `story_reading_progress (user_id, story_id, chapter_id,
  position, updated_at)` (tenant-scoped; chapter anchor like SPEC-14 P0.4, so a
  reorder does not move the reader), `PUT /stories/{id}/progress`, and
  `storyapi.Continue` returning SPEC-10 items with `module: "story"`. *AC:*
  closing the reader mid-chapter and reopening another day resumes in that
  chapter; the story appears on `/continue`.
- **P1.3 `user` may author stories** *(Decision 2026-10-01b (D4); unbuilt —
  §11 row 20)*. A story-owned migration `000N_story_user_write_grant` (the
  `0025_comic_user_write_grant` shape: `WITH grants(...)`, `ON CONFLICT DO
  NOTHING`, idempotent) grants `stories:write:own` and `stories:publish:own` to
  `user`; the role hierarchy carries them upward. `:any` codes and
  `stories:delete:any` stay with `editor` / `admin`. A cover upload needs F009
  as well — `user` holding `assets:write:own` (SPEC-04 §11 row 9). The down
  migration deletes only those two `role_permissions` rows. *AC:* given an
  account holding only `user`, then `POST /stories`, adding a chapter and
  publishing its own story succeed; writing another member's story is still
  403 *(TC-STY-070)*.

### P2 — future considerations (design for, don't build)

- FTS over titles and bodies (`D-2`), bookmarks, reading statistics, EPUB
  import/export.

## 6. Data model — migration `0023_story_core`

**Tenancy** (specs README, ADR-07): both tables are tenant-scoped from birth —
`tenant_id` with the `current_setting('app.current_tenant')` DEFAULT, a
`*_tenant_idx`, `ENABLE` + `FORCE ROW LEVEL SECURITY` and policy
`tenant_isolation` (USING and WITH CHECK). The repository never writes
`tenant_id`.

```sql
CREATE TABLE stories (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- identity anchor
    tenant_id      UUID NOT NULL DEFAULT current_setting('app.current_tenant')::uuid
                       REFERENCES organizations(id),
    title          TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description    TEXT,
    cover_asset_id UUID,                      -- media image asset; validated via mediaapi, no FK
    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX stories_status_updated_idx ON stories (status, updated_at DESC, id DESC);
CREATE INDEX stories_owner_idx  ON stories (owner_user_id);
CREATE INDEX stories_tenant_idx ON stories (tenant_id);

CREATE TABLE story_chapters (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    story_id   UUID NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    tenant_id  UUID NOT NULL DEFAULT current_setting('app.current_tenant')::uuid
                   REFERENCES organizations(id),
    title      TEXT NOT NULL,                 -- 1..200 enforced in the app only
    body_md    TEXT NOT NULL DEFAULT '',      -- ≤ 200,000 runes enforced in the app only
    sort_order INT  NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (story_id, sort_order) DEFERRABLE INITIALLY DEFERRED  -- checked at COMMIT (reorder)
);
CREATE INDEX story_chapters_story_idx  ON story_chapters (story_id, sort_order);
CREATE INDEX story_chapters_tenant_idx ON story_chapters (tenant_id);
```

The `DEFERRABLE` unique cannot be an `ON CONFLICT` arbiter (SPEC-14 §6 caveat),
and because it fires at COMMIT, a duplicate surfaces after the handler has
answered — which `RequireTenant`'s buffered mutating path turns into a 500. That
is why P0.3 checks duplicates in the service before the INSERT. List
`chapter_count` is a correlated `count(*)` subquery in `ListPublishedStories` /
`ListOwnStories`; `EmptyChapters` uses `char_length(btrim(body_md)) = 0`.
**Target additions** in a new `000N_story_checks` (`ls backend/db/migrations |
tail -2`): `CHECK (char_length(title) BETWEEN 1 AND 200)` on chapters
(`NOT VALID` then `VALIDATE`), `stories_owner_cursor_idx (owner_user_id,
updated_at DESC, id DESC)` for `/stories/mine`, and a partial index on
`cover_asset_id WHERE cover_asset_id IS NOT NULL` for P0.6.

**Takeout** (specs README): user-authored, so `story/api` implements
`opsapi.ExportProvider` when SPEC-03 P1.7 lands — one JSON document per story
(the Story fields plus `chapters: [{title, sort_order, body_md}]`), the cover
as an asset reference.

## 7. API summary (`shared/openapi.yaml`, tag `stories`)

All routes sit under `/api/v1`, behind `authTenant`. Unauthenticated → 401
`about:blank`; guard denial → 403 `about:blank`; a guard that cannot resolve the
id → 404 `about:blank`.

| Method | Path | Permission | Request | 2xx response | Errors |
|---|---|---|---|---|---|
| GET | `/stories?cursor=&limit=` | `stories:read` | — | 200 `{items: Story[], next_cursor?}` (`chapter_count` excludes blank chapters, P0.4) | 400 `story/invalid-cursor` |
| GET | `/stories/mine?cursor=&limit=` | `stories:write:own` (held by `user` after P1.3) | — | 200 `{items: Story[], next_cursor?}` | 400 `story/invalid-cursor`, 403 |
| POST | `/stories` | `stories:write:own` (held by `user` after P1.3) | `StoryCreate {title, description?, cover_asset_id?}` | 201 `Story` | 422 `story/validation`, `story/invalid-cover-asset` |
| GET | `/stories/{id}` | `stories:read` (draft: owner only; blank chapters owner only, P0.4) | — | 200 `StoryDetail` | 404 `story/not-found` |
| PATCH | `/stories/{id}` | owner, or `stories:write:any` | `StoryPatch` (absent = unchanged, `null` clears) | 200 `Story` | 404, 422 as POST |
| DELETE | `/stories/{id}` | owner, or `stories:delete:any` | — | 204 | 404 `story/not-found` |
| POST | `/stories/{id}/publish` | `stories:publish:own` (held by `user` after P1.3), then owner or `stories:publish:any` | — | 200 `Story` | 422 `story/not-publishable` (+ `chapters`), 404 |
| POST | `/stories/{id}/unpublish` | `stories:publish:own` (held by `user` after P1.3), then owner or `stories:publish:any` | — | 200 `Story` | 404 |
| GET | `/stories/{id}/chapters` | `stories:read` (draft: owner only; blank chapters owner only, P0.4) | — | 200 `{items: StoryChapter[]}` | 404 `story/not-found` |
| POST | `/stories/{id}/chapters` | owner, or `stories:write:any` | `StoryChapterCreate {title, body_md?, sort_order?}` | 201 `StoryChapter` | 422 `story/validation`, 404 |
| PUT | `/stories/{id}/chapters:order` | owner, or `stories:write:any` | `ReorderRequest {order: [uuid]}` — the complete set | 204 | 422 `story/validation`, 404 |
| PATCH | `/story-chapters/{id}` | owner, or `stories:write:any` (by chapter) | `StoryChapterPatch {title?, body_md?}` | 200 `StoryChapter` | 422 `story/validation`, 404 `story/not-found` |
| DELETE | `/story-chapters/{id}` | owner, or `stories:write:any` (by chapter) | — | 204 | 404 |

**Story**: `{id, owner_id, title, description|null, cover_asset_id|null,
status, chapter_count, created_at, updated_at}`. **StoryDetail**: Story +
`chapters: [{id, title, sort_order}]`. **StoryChapter**: `{id, story_id, title,
body_md, sort_order, created_at, updated_at}`.

**Pagination**: both story lists — default 30, max 50, clamped; order
(`updated_at DESC`, `id DESC`); malformed cursor 400 `story/invalid-cursor`. The
reader payload is non-paginated (`{items}`). A body or parameter shape failure
without a named type (malformed JSON, a non-uuid cover or `order` entry) is 422
`story/validation`. *(Code follow-up: HEAD answers 400 `about:blank` — §11
row 12.)*

**Problem types**: `story/not-found` (404), `story/not-publishable` (422, with
`chapters`), `story/invalid-cover-asset` (422), `story/validation` (422),
`story/invalid-cursor` (400), each registered in `frontend/src/lib/problems.ts`.
*(Code follow-up: none is — §11 row 11.)* Annotate every operation with
`x-required-permission` (specs README AuthZ OpenAPI encoding).

## 8. Events and tasks

| Name | Kind | Payload | Emitted / handled | Wiring |
|---|---|---|---|---|
| `story:published` | event | `{story_id, owner_user_id, title}` (`storyapi.StoryPublishedEvent`) | `Service.Publish` → `emitPublished`, in `cmd/api` | `Subscribe(storyapi.EventStoryPublished, notifyapi.TaskOnStoryPublished)` in both binaries; one consumer, `notify:on_story_published` (bell, `dedup_key` = story id, link `/library/novel/{id}`) |
| `story:on_asset_deleted` | consumer task | `{asset_id, owner_user_id}` | `Module.handleAssetDeleted`, light server, `default` queue | `Subscribe(media.EventAssetDeleted, storyapi.TaskOnAssetDeleted)` in both binaries |

Not projected into the life stream (`0040_journal_drop_catalogue_publish_stream`,
SPEC-09 P0.1). No event on unpublish, edit, chapter change or delete.

**Drift against [events.md](../../reference/events.md).** Names, payloads and
the single consumer match. Missing there: the consumer runs unscoped today (§11
row 1), `story:published` is published before COMMIT (§11 row 4), and the bell
link lands on a placeholder page until P1.1.

## 9. Success metrics (n=1, honest)

- Leading: once P1.1 ships, the owner keeps one real serial (≥ 10 chapters) in
  the module for a month and reads it through `/library/novel`. Until then the
  honest figure is zero — no UI has ever written a story.
- Leading: zero 500s on chapter create and reorder in the API logs after §11
  rows 2–3 close (today an omitted `sort_order` on the second chapter is one).
- Lagging: a reader returning after a week resumes in the right chapter (P1.2).

## 10. Open questions

Two questions were decided on 2026-10-01 (Decision 2026-10-01b): story is
finished, not reverted (D2 — P1.1 is committed scope), and `user` may write
(D4 — P1.3). One more was decided on 2026-10-02 (Decision 2026-10-02b): story
adopts SPEC-14 P0.2 (a) for consistency with comic (B7 — P0.4). After publish,
blank chapters are hidden from non-owner readers and excluded from
`chapter_count`, instead of the publish-time-only check that shipped; the code
change is §11 row 21. The cross-tenant rule decided with it (B6, SPEC-16 §10)
applies here unchanged; B13, which made published music and movies playable by
tenant members through SPEC-04 P0.8's `tenant` asset visibility, applies to the
story cover (P0.5; §11 row 22).

No open questions remain.

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

Baseline: `main` @ `99b5a0b` (the docs commits on top change no code). The spec
text above is the target; this section lists every place the shipped code still
diverges from it. Rows are ordered by severity: lost or wrong data first, then
integrity, authorization, contract, hygiene and unbuilt work; row 20 (P1, added
by Decision 2026-10-01b) is appended after row 19, row 21 (P0.4, added by
Decision 2026-10-02b (B7)) after row 20, and row 22 (P0.5, added by B13) after
row 21. A row closes when
the code matches the requirement it cites and the SPEC-17 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded.
Paths are relative to `backend/internal/modules/story/` unless stated otherwise.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.6 tenant scope | `story:on_asset_deleted` runs inside the payload owner's tenant scope. | `cmd/worker/main.go` builds `story.New(story.Deps{Repo: storyrepo.NewAdapter(conn, tdb.RunInTx)})` with no tenant runner; `module.go` `handleAssetDeleted` → `service.go` `HandleAssetDeleted` → `NullCoverByAsset` runs on the bare pool. Under `portal_app` the FORCE RLS policy refuses an unscoped statement, so the task fails and retries out; covers stay dangling. `comic:on_asset_deleted` and `movie:on_asset_deleted` are built the same way. | **backend:** `Deps.RunInTenant` (`runInUserTenant` in `cmd/worker`); the handler parses `owner_user_id` and runs inside it. **test:** TC-STY-082 (RLS suite). | Found while writing SPEC-17, 2026-10-01 (same as SPEC-16 §11 row 1) |
| 2 | P0.3 chapter `sort_order` on create | Optional; absent → `MAX + 10`; a duplicate → 422 `story/validation`, checked before the INSERT. | `handler.go` `CreateChapter` decodes `sort_order` as a plain `int` (absent → 0); `service.go` `CreateChapter` passes it through; `query/story.sql` `CreateStoryChapter` inserts it. The `DEFERRABLE INITIALLY DEFERRED` unique fires at the request's COMMIT, and `tenant/middleware/require_tenant.go` replaces the buffered 201 with a 500: the second chapter created without `sort_order`, or any duplicate, is a bare 500. | **backend:** `sort_order *int`; when nil, `SELECT COALESCE(MAX(sort_order), 0) + 10`; when set, an `EXISTS` check → `ErrValidation` (new queries, `make sqlc`). **openapi:** `StoryChapterCreate.required` drops `sort_order`. **test:** TC-STY-020, TC-STY-021. | Found while writing SPEC-17, 2026-10-01 (comic's `CreateChapter` has the same shape) |
| 3 | P0.3 reorder takes the complete set | `order` must be exactly the story's chapter ids, each once; otherwise 422 and nothing changes. | `repository/adapter.go` `ReorderChapters` loops `UpdateStoryChapterOrder ... WHERE id = $1 AND story_id = $3`: a foreign id updates nothing, a repeated id is renumbered twice, an omitted chapter keeps its old value and can collide with a renumbered one at COMMIT (500). `service.go` `ReorderChapters` validates nothing. `shared/openapi.yaml` `reorderStoryChapters` says a partial list "is a truncation" — nothing is deleted. | **backend:** in `ReorderChapters`, load the story's chapter ids and compare as sets (length, no repeats, same members) → `ErrValidation`. **openapi:** replace the truncation sentence with the 422 rule. **test:** TC-STY-024 (extend `TestReorderPassesTheOrderThrough`). | Found while writing SPEC-17, 2026-10-01 (openapi vs handler) |
| 4 | P0.5 event after commit | `story:published` is enqueued only after the request transaction commits. | `service.go` `Publish` → `emitPublished` runs right after `repo.SetStatus`, inside the open request transaction. | **backend:** emit through the `db.AfterCommit` hook SPEC-07 §11 row 1 introduces. **test:** TC-STY-063. | Found while writing SPEC-17, 2026-10-01 (SPEC-07 row 1 pattern) |
| 5 | P0.4 detail chapters | A chapter-load failure is a 500. | `handler.go` `GetStory`: `chapters, _ := h.svc.ListChapters(...)` — on error the detail answers 200 with `chapter_count: 0` and `chapters: []`. | **backend:** return the error through `writeStoryErr`. **test:** fake `ListChapters` error → 500. | Found while writing SPEC-17, 2026-10-01 |
| 6 | P0.2 lookup failures | A `mediaapi.GetAsset` infrastructure error is a 500. | `service.go` `CreateStory`/`UpdateStory` replace any `validateImageAsset` error with `ErrInvalidCoverAsset`. | **backend:** sentinel only for validation / not-found; pass the rest through. **test:** fake lookup error → 500. | Found while writing SPEC-17, 2026-10-01 |
| 7 | P0.5 publish RBAC | `stories:publish:own` chained before the owner-or-`stories:publish:any` guard. | `cmd/api/main.go` `PublishMW` = `RequireOwnerOrPermission(engine, "stories:publish:any", byStory)` only; `module.go` mounts `/publish` and `/unpublish` with it. | **backend:** mount `m.perm("stories:publish:own")` first. **test:** TC-STY-065, 066. | SPEC-14 §11 row 4 pattern (F051) |
| 8 | P0.5 draft is 404 on guarded routes | A non-owner without the `:any` code gets 404 for a draft on every guarded route. | `cmd/api/main.go` `ownerExtractor` over `OwnerByStory` / `OwnerByChapter` (`query/story.sql` `GetStoryOwner`, `GetStoryOwnerByChapter`) ignores status → 403. | **backend:** both owner queries return `status`; draft → `ErrOwnerNotFound` unless owner or `:any`. **test:** TC-STY-069. | SPEC-14 §11 row 5 pattern (F119) |
| 9 | P0.4 / §7 envelopes | Lists answer `{items, next_cursor?}`; the reader payload answers `{items}`. | `handler.go` `writeStoryList` → `{"stories": …}`; `Chapters` → `{"chapters": …}`; `shared/openapi.yaml` `StoryList` `required: [stories]`, `listStoryChapters` `required: [chapters]`. No frontend reader exists. | **backend:** both keys → `items`. **openapi:** both schemas. **test:** TC-STY-042, TC-STY-046. | Decision 2026-09-30 (Envelopes); specs README unowned-list note |
| 10 | P0.4 `limit` | Above 50 → clamped to 50. | `service.go` `list`: `if limit <= 0 \|\| limit > maxLimit { limit = defaultLimit }`. | **backend:** `server.Limit(r, 30, 50)`. **openapi:** describe the clamp. **test:** TC-STY-044. | Decision 2026-10-01 (limit) |
| 11 | §7 problem types | Every `story/*` slug is in `problems.ts`. | `grep -n 'story/' frontend/src/lib/problems.ts` finds nothing; `writeStoryErr` emits five. | **frontend:** add them. **test:** TC-STY-110. | Errors convention (D-7) |
| 12 | §7 shape failures | Malformed JSON or a non-uuid id in a body → 422 `story/validation`. | `handler.go` `decode` → `server.DecodeLimit`'s 400 `about:blank`; `CreateStory`/`UpdateStory` and `decodeOrder` answer `server.BadRequest` (400 `about:blank`). | **backend:** map to `ErrValidation`. **openapi:** drop the 400s. **test:** TC-STY-006. | README Pagination/Errors convention |
| 13 | P0.2 `null` clears | `PATCH {description: null}` clears it, as `StoryPatch` documents. | `query/story.sql` `UpdateStory` `COALESCE(sqlc.narg('description'), description)`; `handler.go` decodes `description` as `*string`. | **backend:** three-state decode + `set_description` flag; `make sqlc`. **test:** TC-STY-005. | Found while writing SPEC-17, 2026-10-01 (openapi vs handler) |
| 14 | P0.4 `chapter_count` | Present on every Story, `0` included; `required` in OpenAPI. | `handler.go` `storyJSON` sets it only `if st.ChapterCount > 0`; create, PATCH, publish and unpublish responses never carry it; `Story.chapter_count` is optional in `shared/openapi.yaml`. | **backend:** always emit it (Get/SetStatus/Update return it via the same subquery, or the handler counts). **openapi:** `required`. **test:** TC-STY-043. | Found while writing SPEC-17, 2026-10-01 |
| 15 | P0.5 single error writer | `story/not-publishable` goes through `server.ProblemWith`. | `handler.go` `writeStoryErr` sets headers and encodes the map by hand (comic uses `server.ProblemWith`). | **backend:** `server.ProblemWith(w, 422, "story/not-publishable", …, map[string]any{"chapters": np.Chapters})`. **test:** the existing CC-1 writer tests cover the helper; add an HTTP assertion (TC-STY-061). | `/CLAUDE.md` Known drift (single error writer since 2026-08-27) |
| 16 | P0.3 / §6 hygiene | Every UPDATE sets `updated_at`; chapter title length in the schema; owner cursor and cover indexes; `0023`'s down removes only what it added. | `query/story.sql` `UpdateStoryChapterOrder` sets only `sort_order`; `0023` has no chapter-title CHECK and only `stories_owner_idx`; `0023_story_core.down.sql` deletes `permissions WHERE code LIKE 'stories:%'`, including the 0003-seeded codes and grants. | **query:** `updated_at = now()` in the reorder. **migration:** `000N_story_checks` (§6). The down file is applied history; record it. **test:** TC-STY-111 (up/down). | Found while writing SPEC-17, 2026-10-01 |
| 17 | §7 OpenAPI annotations | `x-required-permission` on every operation; id parameters named for stories and chapters. | No story operation is annotated; every `/stories/{id}*` and `/story-chapters/{id}` path reuses `#/components/parameters/AssetID`. | **openapi:** annotate the thirteen operations; `StoryID` / `StoryChapterID` parameters. | AuthZ convention (OpenAPI encoding) |
| 18 | P1.1 frontend | `/library/novel` list, reader and manager. | `templates/v1/views/library/novel/NovelDetailView.tsx` is a 26-line placeholder; `app/(app)/library/page.tsx` links `/library/novel`, which has no `page.tsx` (404); no `lib/story.ts`. | **frontend:** P1.1. **test:** TC-STY-090…094. | [backlog.md](../backlog.md) P2 line 28 |
| 19 | P1.2 progress | Story-owned reading progress and a `/continue` leg. | No table, route or `storyapi.Continue`; `handleContinue` calls only media. | **migration · backend · openapi · frontend:** P1.2. **test:** TC-STY-095…097. | `D-20`; [backlog.md](../backlog.md) P2 line 29 |
| 20 | P1.3 `user` authoring grant | `user` holds `stories:write:own` and `stories:publish:own` (a story-owned grant migration); `:any` and delete-any unchanged. | `backend/db/migrations/0023_story_core.up.sql` grants both codes to `creator` only and no later migration widens them, so a `user` gets 403 from `POST /stories` and `GET /stories/mine` (`module.go` `m.perm("stories:write:own")`). | **migration:** `000N_story_user_write_grant` (`ls backend/db/migrations \| tail -2` for the number). **test:** TC-STY-070. Lands with or after F009 (SPEC-04 §11 row 9). | Decision 2026-10-01b (D4) |
| 21 | P0.4 blank chapters after publish | On a published story, a non-owner gets no blank chapter (`btrim(body_md) = ''`) in the detail, `chapter_count` or the reader payload; `GET /stories` counts non-blank chapters only. | `handler.go` `GetStory` sets `chapter_count = len(chapters)` and lists every chapter from `service.go` `ListChapters` → `query/story.sql` `ListStoryChapters` (no body predicate); `service.go` `ChaptersVisible` gates on the story only and returns `ListStoryChapters` whole; `ListPublishedStories` counts `(SELECT count(*) FROM story_chapters ch WHERE ch.story_id = s.id)`. The owner is never distinguished from a reader after the draft gate in `Service.GetStory`. | **query:** a `ListReadableStoryChapters` with `AND btrim(body_md) <> ''`, and the same predicate in `ListPublishedStories`' `chapter_count` subquery (`ListOwnStories` unchanged); `make sqlc`. **backend:** `GetStory` and `ChaptersVisible` use the filtered query when the caller is not the owner and the story is published, and the detail's `chapter_count` comes from the list it returns. **openapi:** say so on `Story.chapter_count`, `getStory` and `listStoryChapters`. **test:** TC-STY-047, 048. | Decision 2026-10-02b (B7); SPEC-14 §11 row 9 pattern (F049) |
| 22 | P0.5 the cover follows the story | Publish raises `cover_asset_id` to `tenant` through `mediaapi.SetVisibility` with the story owner; unpublish, delete, clearing and replacing the cover lower what no other published story of the owner still uses; in the same transaction. | `service.go` `Publish`, `Unpublish`, `DeleteStory` and `UpdateStory` write `stories` only; `types.go` `MediaAPI` has `GetAsset` alone. A published story's cover stays `private`, so `0032`'s `variant_select` hides its variants from every other member while `GET /stories` lists the story to them. Latent while each user has a personal organisation, and invisible until P1.1 renders covers. | **backend:** `MediaAPI` gains `SetVisibility`; a story query listing which cover ids another published story of the owner still uses; the four methods raise or lower per P0.5, passing the story's `owner_user_id`. **test:** TC-STY-112. Needs SPEC-04 §11 rows 20–21. | Decision 2026-10-02b (B13) |

**Already matching on HEAD.**
- `0023_story_core`: both tables, the story CHECKs, the `DEFERRABLE` chapter
  unique, `ON DELETE CASCADE` from stories to chapters, `tenant_id` + FORCE RLS
  + `tenant_isolation` on both tables, and the six-code seed.
- Story create validates the title (trimmed, 1–200 runes) and the cover against
  `mediaapi` (image, ready, owned by the story's owner); PATCH validates against
  the stored owner and distinguishes absent from `null` for the cover.
- Chapter title (trimmed, 1–200) and body (≤ 200,000 runes) validation on
  create and PATCH; a 4 MiB request-body cap for prose.
- Detail and reader payload are published-or-owner, and a stranger's draft
  answers the same 404 body as a missing id (`http_test.go`
  `TestHTTPDraftIsNotFoundToAStranger`, `story_test.go`
  `TestChaptersVisibleFollowsTheStoryGate`); a repeat DELETE is 404
  (`TestHTTPDeleteTwiceIs404`, `DeleteStory :execrows`).
- Detail chapter summaries carry no `body_md`; both lists keyset on
  (`updated_at DESC`, `id DESC`) with `chapter_count`; malformed cursor → 400.
- Publish requires ≥ 1 chapter and no blank body, naming every blank chapter in
  reading order (`TestPublishRejectsAStoryWithNoChapters`,
  `TestPublishNamesTheEmptyChapters`), then emits `story:published`; a nil
  publisher never fails it.
- Reorder renumbers from array position to 10, 20, 30… inside one transaction.
- `media:asset_deleted` → `story:on_asset_deleted` is subscribed in both
  binaries and clears the cover idempotently without touching status.

**Test evidence to add or fix.**
- No test creates, updates or deletes a chapter through the service or the
  router (TC-STY-020…026), or covers the reorder set rule — the fake
  `ReorderChapters` only records its input (TC-STY-024).
- No test covers the lists, `limit`, envelopes or `chapter_count: 0`
  (TC-STY-041…046), `UpdateStory` (TC-STY-004, 005) or the after-commit emit
  (TC-STY-063).
- Nothing distinguishes owner from reader on a published story's chapters
  (TC-STY-047, 048); `TestChaptersVisibleFollowsTheStoryGate` covers the draft
  gate only.
- The consumer is proven on a fake only (`TestAssetDeletedClearsTheCover`);
  nothing proves it under RLS (TC-STY-082).
- RBAC rows (TC-STY-065…069) need a router built with a real engine; the HTTP
  tests mount the guards as `nil`.

## 12. Out of scope

- Everything in §3: authors, bookmarks, ratings, inline chapter media, narration,
  server-side Markdown rendering, cross-tenant publishing, FTS.
- The comic vertical's own copies of rows 1–3 and 7–8 (SPEC-14 owns them).
- The account-level `assets:write:own` grant needed to upload a cover
  (SPEC-04 §11 row 9).
- Movie (SPEC-16) and music (SPEC-15).
