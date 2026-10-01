# SPEC-08 — Journal attachments: Attachments and Location leave the body

**Status:** executed 2026-09-19 on branch `feat/journal-attachments` (T0–T6, #9–#15; rev 1 drafted 2026-09-12; manual run against the stack done 2026-09-19 — [§ Manual run](#manual-run-against-the-stack-2026-09-19)) · **Last verified:** 2026-10-01
**Module:** `journal` (extends: one migration, service, handler, one consumer) · frontend journal composer, entry card, stream card · **Depends on:** SPEC-04 (image variants — shipped), SPEC-07 (entries — shipped), SPEC-09 (stream join — shipped). Nothing pending.
**Upstream:** `/grill-with-docs` session 2026-09-12 (sixteen settled decisions, four glossary terms) · [backlog](../backlog.md) P2 #18 · **Refs:** [CONTEXT.md](../../../CONTEXT.md) (Entry, Attachment, Location, Asset) · [SPEC-07 P1.5](SPEC-07-journal.md) (the original photo-attachments item this supersedes) · [SPEC-09](SPEC-09-life-stream-home.md) (stream card promise "asset thumbs — joined from journal_entries") · [ADR-07](SPEC-01-account-identity-admin.md#adr-07) (tenant scope on every lookup) · [ADR-10](README.md#adr-10) (contract first, codegen committed)
**Downstream consumers:** the stream (SPEC-09) reads the new columns through its existing join; takeout (SPEC-03 P1.7) — bodies stay plain markdown, which this spec makes true for every row
**Tracker:** GitHub issue [#8](https://github.com/jackone2991/portal/issues/8), label `ready-for-agent`
**Tickets (`/to-tickets`, 2026-09-13 — the T-labels code comments cite):** T0 [#9](https://github.com/jackone2991/portal/issues/9) prefactor · T1 [#10](https://github.com/jackone2991/portal/issues/10) Attachments in columns (one photo) · T2 [#11](https://github.com/jackone2991/portal/issues/11) many photos, card, lightbox · T3 [#12](https://github.com/jackone2991/portal/issues/12) Location in columns · T4 [#13](https://github.com/jackone2991/portal/issues/13) deleted Asset strips · T5 [#14](https://github.com/jackone2991/portal/issues/14) edit in place · T6 [#15](https://github.com/jackone2991/portal/issues/15) close-out. Edges: T0 → T1 → {T2, T3, T4} → T5 → T6.

## Problem Statement

I write journal Entries with a photo and a place. The app lets me do that today, but it
cheats: the photo and the place are smuggled into the Entry's markdown body as two
specially-shaped links, because the server refuses the `asset_ids` field it has carried
since the table was created. Everything downstream of that cheat is fragile:

- I can attach exactly **one** photo and **one** place — the second is silently dropped.
- Editing an Entry means editing raw markdown with the magic links in it; a stray
  keystroke breaks the photo.
- If I delete the photo from my media library, the Entry keeps pointing at it and the card
  shows a broken frame.
- The body is no longer plain markdown, so the promised takeout ("entries export as
  markdown files, no proprietary markup") would export the cheat too.
- The server knows nothing about any of this, so nothing is validated: an Entry can point
  at a photo that never existed, or at someone else's.

Measured on the live database on 2026-09-12: ten Entries, three users, two Entries with a
photo, one with a place, none with more than one photo, none with `asset_ids` set.

## Solution

Make Attachments and Location first-class properties of an Entry, validated by the server,
stored in columns, rendered from those columns everywhere an Entry is shown — and take the
cheat out of every existing body in the same migration, so that "the body is plain
markdown" is true for all rows, not only new ones.

An Entry may carry up to ten Attachments (ordered, image Assets owned by the author,
processed and ready) and one Location (a name and coordinates). An Entry may be text, or
pictures, or both — never a Location alone. When an Asset is deleted, every Entry that
showed it simply stops showing it. The composer uploads several photos at once, prevents
the mistakes the server would reject, and will not save until every photo is ready, so a
saved Entry always renders. Editing happens in the same composer, in place.

## User Stories

1. As a journal writer, I want to attach several photos to one Entry, so that one moment
   with more than one picture is one Entry, not several.
2. As a journal writer, I want the photos to appear in the order I added them, so that the
   Entry reads the way I composed it.
3. As a journal writer, I want to remove one photo from an Entry without touching the
   others, so that a wrong upload costs one tap.
4. As a journal writer, I want to record where an Entry happened, so that I can remember
   the place as well as the day.
5. As a journal writer, I want to clear the Location from an Entry, so that a place I
   added by mistake does not stick.
6. As a journal writer, I want to save an Entry that is only a photo, so that a moment I
   have nothing to say about still gets kept.
7. As a journal writer, I want the app to refuse an Entry that is only a Location, so that
   I never end up with an empty card that says nothing.
8. As a journal writer, I want the composer to stop me at ten photos with a clear message,
   so that I do not compose an Entry the server will reject.
9. As a journal writer, I want the composer to ignore a photo I add twice, so that a
   double-click does not produce a duplicate.
10. As a journal writer, I want to see each photo's upload and processing progress in the
    composer, so that I know why Save is not yet available.
11. As a journal writer, I want Save to become available only when every photo is ready,
    so that an Entry I have saved always shows its pictures.
12. As a journal writer, I want a photo that failed processing to show an error and a way
    to drop it, so that one bad file does not block the Entry.
13. As a journal writer, I want to edit an Entry's text, mood, photos and Location in one
    place, so that I do not have to guess which part is edited where.
14. As a journal writer, I want that edit surface to open where the card is, so that fixing
    a typo still feels quick.
15. As a journal reader, I want an Entry with one photo to look exactly as it does today, so
    that nothing I am used to moves.
16. As a journal reader, I want an Entry with several photos to show the first one large
    and the rest as small thumbnails, so that I see everything without tapping.
17. As a journal reader, I want a "+N" badge when there are more thumbnails than fit, so
    that I know there is more.
18. As a journal reader, I want to open a photo full-size and swipe through the Entry's
    photos in order, so that I can look at each one properly.
19. As a journal reader, I want the Location shown as a chip that opens a map, so that a
    name I no longer recognise can be placed.
20. As a journal reader, I want the stream card to show the same photos and Location as the
    Entry card, so that the home stream does not lose information the journal has.
21. As a journal reader, I want an Entry whose photo I deleted to simply show one photo
    fewer, so that I never see a broken frame.
22. As a journal writer, I want the body of every Entry — old and new — to be plain
    markdown, so that exporting my journal gives me my words and nothing else.
23. As a journal writer, I want my existing Entries with a photo or place to keep them
    after the upgrade, so that the migration is invisible to me.
24. As the operator, I want the migration to refuse to complete if any body still carries
    the old link syntax, so that a half-migrated database is impossible.
25. As the operator, I want the migration to be reversible, so that rolling back restores
    the old bodies rather than losing the photos.
26. As a security-conscious owner, I want the server to accept only my own image Assets as
    Attachments, so that an Entry cannot point at someone else's picture.
27. As a security-conscious owner, I want every Asset lookup to run inside my tenant scope,
    so that isolation holds on this path exactly as it does everywhere else.
28. As an API client, I want an invalid Attachment to fail the whole request with a
    problem that names the offending id and the reason, so that I can fix the right thing.
29. As an API client, I want an invalid Location to fail with its own problem code, so that
    I can tell a bad place from a bad photo.
30. As an API client, I want sending `asset_ids` to replace the whole list, so that the
    semantics are the same as every other field on the Entry.
31. As an API client, I want sending `location: null` to clear the Location, so that
    "remove" needs no separate endpoint.
32. As an API client, I want the stream item and the Entry to expose the same
    `asset_ids` and `location` shapes, so that one renderer serves both.
33. As a maintainer, I want the frontend's markdown link encoder and decoder deleted, so
    that the cheat cannot be reintroduced by habit.
34. As a maintainer, I want the frontend's `Place` type renamed to `Location`, so that the
    code speaks the glossary.
35. As a maintainer, I want the journal module's README to say it talks to media and
    subscribes to asset deletion, so that the module map stays true.
36. As a maintainer, I want the OpenAPI contract changed first and the generated code
    committed with it, so that the drift gate stays green.
37. As a maintainer, I want the new behaviour asserted over the real router with fakes, so
    that the contract is pinned the way comic's and bank's are.

## Implementation Decisions

**Vocabulary.** *Entry*, *Attachment*, *Location* and *Asset* as defined in `CONTEXT.md`.
An Attachment is a reference from an Entry to an image Asset the Entry shows; a Location is
a property of the Entry, not an Attachment; the body is plain markdown and carries no
structure.

**Schema — one migration in the journal module (next free number).**

- `asset_ids uuid[]` (already present, never written until now) becomes live: at most ten
  elements, array order is display order.
- Three new nullable columns on the entries table: `location_name` (text),
  `location_lat`, `location_lon` (`numeric(7,4)` — four decimal places, ~11 m; a rollback
  re-encodes those four, whatever an old body held). A CHECK makes them
  all-or-nothing, requires a non-empty trimmed name, and bounds latitude to [−90, 90] and
  longitude to [−180, 180].
- The body CHECK relaxes from "1–20 000 characters" to "at most 20 000 characters". The
  rule "an Entry is text, or at least one Attachment, or both — never a Location alone" is
  a **service-level write rule**, not a CHECK: the asset-deleted consumer must be able to
  strip the last Attachment from a photo-only Entry and leave it standing (see Further
  Notes), and a CHECK would refuse exactly that update.
- **Backfill in the same migration.** For every existing body: the first markdown image
  link whose target is `asset:` followed by a uuid is moved into `asset_ids`; the first
  markdown link whose target is `geo:` followed by `lat,lon` (its link text is the name)
  is moved into the three Location columns; both links are
  stripped from the body and surrounding blank lines trimmed. A `geo:` link whose point is
  off the Earth makes the migration raise naming the row rather than guess. A closing DO
  block raises if any body still matches either marker, so the migration cannot complete
  half-done. On an
  empty database (CI) it is a no-op. On the live database it touches two rows, both of
  which keep text after stripping (verified 2026-09-12: 47 and 24 characters).
- **The down migration re-encodes**: it appends the same two link forms from the columns
  back onto the body, drops the Location columns and restores the previous body CHECK —
  `NOT VALID`, because a row the up emptied (a link-only body whose Asset was already gone,
  so the link was dropped) cannot satisfy the old lower bound; the rollback completes, the
  old rule binds every new row version — so that emptied row cannot be written to under the
  old code until it is fixed by hand — and `VALIDATE CONSTRAINT` finishes the job then.
  Rolling back loses nothing the up kept.
- No foreign key from `asset_ids` to the media module's table (cross-module; validated
  through the media module's public API and corrected by event, as SPEC-07 decided).

**Service — journal.**

- Create and update accept `asset_ids` and `location`, and update treats `mood` the way it
  treats `location` (T5). `asset_ids` is validated as a whole:
  duplicates, more than ten elements, or any element that is not an existing image Asset
  with status ready and owned by the caller → the whole request is refused with a 422
  problem `journal/invalid-asset` whose `detail` names the id and the reason. Nothing is
  saved on failure.
- Validation calls the media module's public `GetAsset` through a narrow interface declared
  by the journal module, exactly as comic, music, movie and story already do. It runs inside
  the request's tenant transaction (ADR-07): an Asset in another tenant answers "not found".
- Location is validated by the same rules as the CHECK; a violation is 422
  `journal/invalid-location`.
- The body rule ("text, or at least one Attachment") is enforced in the service with the
  same problem code as today's empty-body error, not left to the database.
- The `media:asset_deleted` consumer, which today removes the media module's own stream
  rows, additionally strips the deleted id from every Entry's `asset_ids` for that owner.
  It runs inside the owner's tenant scope (the event carries the owner) and is idempotent.
  An Entry whose last Attachment is stripped keeps existing even if its body is empty —
  which is why the "text or Attachment" rule lives in the service's write path and not in
  a CHECK (see Further Notes).
- No new event. No new permission: attaching only *reads* an Asset; uploading is the media
  module's existing write path (`assets:write:own`, granted to `user` by the
  SPEC-04 grant migration — SPEC-04 §7).

**Stream — SPEC-09.** The existing left join that already fetches `body_md` and `mood`
from the entries table also fetches `asset_ids` and the three Location columns. Stream
items for journal Entries expose them with the same shapes as the Entry itself. The
stream's `payload` stays unused for this. No backfill of stream rows is needed — nothing is
copied.

**Contract — OpenAPI first (ADR-10), regenerated code committed with it.**

- `JournalEntry` gains `location` (object `{name, lat, lon}` or null); `asset_ids` stays as
  declared.
- `JournalEntryWrite` gains `asset_ids` (array of uuid, optional; when present it
  **replaces the whole array**) and `location` (the object, or `null` to clear; optional);
  `mood` follows the same three states on PATCH — a string sets, `null` clears, absent
  keeps — since the schema already declared it nullable and the edit surface must be able
  to take a mood away (T5).
  `body_md` becomes optional-but-bounded: absent or empty is allowed only when the
  resulting Entry has at least one Attachment.
- `StreamItem` gains `asset_ids` and `location` with the same shapes.
- New problem code `journal/invalid-location`; `journal/invalid-asset` is reused with
  the meaning "the Attachment list as a whole is invalid".

**Frontend.**

- The composer uploads several files at once; each shows a thumbnail, its processing state,
  and a remove control. The client deduplicates on selection, caps at ten with a message,
  disables Save until every photo reports ready, and disables Save while the Entry has
  neither text nor a photo. A photo that fails processing shows the error and can be
  removed. Order is selection order; reordering is not built.
- The same composer serves as the edit surface, rendered in place of the card (no
  navigation). The raw-markdown inline editor is removed. Edit submits body, mood, the whole
  `asset_ids` array, and `location` (object or null).
- Entry card and stream card share one renderer: the first Attachment as a hero at the
  `medium` variant, the remaining Attachments as a row of `thumb` variants (up to four, then
  a "+N" badge); tapping opens a lightbox at `medium` that moves through the Attachments in
  array order. The Location renders as the existing map chip. Variants are served by the
  media module's existing variant route; nothing new is signed.
- The markdown link encoder/decoder is deleted; the `Place` type is renamed `Location`.
  Geocoding stays where it is (client-side); the server never calls out.

**Documentation.** The journal module's README gains "talks to: media (asset lookup)" and
"subscribes to: `media:asset_deleted`". The traceability matrix gains SPEC-08 rows graded
on evidence. SPEC-07 P1.5 is marked superseded by this spec (a pointer, not a rewrite).
Backlog #18 links this spec and its issue and closes when the tracker issue does.

## Testing Decisions

**What a good test is here.** The unit under test is the journal's HTTP contract: what a
client sends, what it gets back, and what is stored. A good test drives the real router and
handlers with a fake repository and a fake media lookup, asserts on status, problem code,
`detail`, and the JSON shape — never on how the service is structured internally.

**One primary seam, an existing pattern.** A journal HTTP-contract test file in the style
of the comic and bank ones from 2026-09-11 (real chi router, real handler and service,
fakes underneath) pins every decision above that a client can observe:

- create/patch with a valid list stores it in order; with a duplicate, an eleventh
  element, an unknown id, a non-image, a not-ready Asset, or another user's Asset → 422
  `journal/invalid-asset`, `detail` naming the id and reason, nothing stored;
- Location all-or-nothing, name non-empty, bounds → 422 `journal/invalid-location`;
- patch with `asset_ids` replaces the whole array; patch with `location: null` clears it;
  patch with `mood: null` clears the mood and a blank mood is refused (T5);
- empty body with one Attachment is created; empty body with none, or with only a
  Location, is refused;
- the Entry JSON and the stream item JSON carry `asset_ids` and `location` in the same
  shape;
- lookups happen inside the tenant scope (the fake records the scope it was called under).

**One existing seam extended.** The journal service tests' asset-deleted case grows to
assert that the id is stripped from every Entry that carried it, that other ids survive,
and that a second delivery is a no-op.

**The migration is self-checking, and its loops are under test.** Its closing DO block is
the assertion; it runs on CI's fresh database (no rows → passes trivially) and ran on the
live database before cutover. The regexes are the ones the frontend used to write the
links, copied verbatim into the migration. `backfill_test.go` (journal) additionally builds
a throwaway database to 0043 from the migration files, seeds old-style bodies and runs
up → down → up plus the refused input, on the `RLS_TEST_ADMIN_URL` CI provides.

**Frontend: no runner in CI.** vitest is not wired into CI (backlog #10); the frontend is
covered by typecheck and `next build` in CI and by running the composer by hand against
the stack. This spec does not introduce a runner. *(T2 note, 2026-09-19: the composer's
photo-list rules and the card's hero/thumbs/"+N" split are pure functions with vitest
unit tests — `pnpm test`, under a second — and since the same day the `frontend` CI job
runs them, so they are CI evidence for the rules, not for the components around them.)*

**Prior art.** Comic and bank HTTP-contract tests (router-driven, fakes); the four
modules' identical Asset-validation pattern; the comic importer's poll-to-ready loop for
the composer's Save gating; SPEC-14 P0.6's soft-cascade consumer for asset deletion.

## Manual run against the stack (2026-09-19)

*Folded in from `docs/testing/TEST-RUN-2026-09-19-spec-12.md` (deleted — `git show ea100d8:docs/testing/TEST-RUN-2026-09-19-spec-12.md`). A record, not CI evidence.*

**Build:** branch `feat/journal-attachments` at `3ecf06f` (T0–T6), images rebuilt with `docker compose build api worker frontend` · **Env:** DEV-LOCAL — the compose stack over Traefik (`https://portal.localhost`, `https://api.portal.localhost`), host Postgres at schema `0045`, MinIO, the real worker · **Executed by:** agent session (API half scripted, UI half driven in Chrome as `demo@portal.localhost`) · **Closes:** the manual-run half of [backlog](../backlog.md) P1 17a

Every step below ran against the live stack; nothing is inferred from tests.

### API half — 28 checks, 28 pass

Five 640×480 PNGs went through the real pipeline (`POST /assets` → `PUT /source` → `POST /complete` → poll to `ready`), then:

| # | Check | Result |
|---|-------|--------|
| 1 | `POST /journal/entries` with five `asset_ids`, a mood and a Location → 201; `asset_ids` back in the order sent; `location` and `mood` on the Entry | pass |
| 2 | `GET /stream`: the journal item carries the same `asset_ids` and `location` | pass |
| 3 | Duplicate id → 422 `journal/invalid-asset`, `detail` names it; off-Earth Location → 422 `journal/invalid-location`; Location alone → 422 `journal/invalid-body`; unknown id → 422 `journal/invalid-asset` | pass |
| 4 | `PATCH` the whole Entry — one photo dropped (order kept), `location: null`, `mood: null` → 200; `GET` agrees; a later `PATCH {location}` sets it back and leaves `asset_ids` alone | pass |
| 5 | `DELETE /assets/{id}` of a photo on two Entries → 204; within 3 s the worker consumer strips it from both, the others survive in order, the stream item shows one fewer, the variant answers 404 | pass |
| 6 | A photo-only Entry whose only photo is deleted still exists with `[]` and `""`; a mood-only PATCH on it is allowed; a body write that leaves it empty → 422 `journal/invalid-body` | pass |

Client note: PowerShell 5.1's `Invoke-WebRequest` drops 4xx bodies and sends non-ASCII JSON in the ANSI code page (the first run showed eight false failures for exactly that); the script switched to `curl.exe` with UTF-8 file bodies. Script: session scratchpad, not committed.

### UI half — composer, card, lightbox, edit in place

| Step | Observed |
|------|----------|
| Camera → Add Photo → five files at once | five tiles, each `Sẵn sàng` once processed; popup closed itself; Post enabled with no text (photo-only draft) |
| Eleven files into the five (six new, five re-picked) | ten tiles; notice `Tối đa 10 ảnh mỗi bài — 6 ảnh bị bỏ qua.`; camera reads `Đã đủ 10 ảnh`. **Harness artifact:** the Chrome extension's file upload stamps `lastModified = now` on every pick (verified: two picks of the same file 13 s apart differ), so the five re-picks were not duplicates to the client and filled the strip — a real file picker keeps the file's mtime, and the rule is under vitest (`composer-photos.test.ts`) |
| Tap camera on a full strip | notice `Đã đủ 10 ảnh mỗi bài — bỏ bớt một ảnh để thêm ảnh khác.`; no dialog |
| Remove one tile, add a file that is not an image | tile shows `Không hoàn tất được tải lên.`; Post disabled with title `Bỏ ảnh lỗi trước khi đăng`; removing the tile re-enables Post |
| Post with text and nine photos | card: hero + four thumbs + `+4`; composer reset |
| Tap the hero | lightbox `Ảnh 1 / 9` at `medium`, previous arrow disabled; `ArrowRight` ×2 → `Ảnh 3 / 9`; `Escape` closes |
| Post options → Edit Post | the card is replaced by the composer: tab `Edit Post`, the text and nine tiles pre-filled, Cancel and Save |
| Remove one tile, set the mood `vui`, Save | card back with `Mở ảnh 1 / 8`, `+3`, `19 Sept · vui`; server: 8 ids, mood `vui` |
| Edit again, remove a tile, Cancel | card unchanged (still 8) |
| Edit → pin → search `Hoan Kiem` → pick → Add Location → Save | chip `Hoàn Kiếm` on the card linking to OpenStreetMap; server `location` `{Hoàn Kiếm, 21.0286, 105.8506}` |
| Delete one attached Asset (API), reload | card `Mở ảnh 1 / 7`, `+2`, chip intact — one photo fewer, no broken frame |

**Defect found and fixed in the same session:** opening the mood chip focused it with `requestAnimationFrame`, which never fired in the automation's backgrounded tab, so the typed mood landed in the textarea. Replaced by an effect keyed on an open-counter (`Composer.tsx`). Not reproducible by a person on a focused tab, but the effect is the right mechanism regardless.

### Not covered here

The `0044`/`0045` backfill loops (proven by hand on a throwaway database on 2026-09-13 and 2026-09-19, still without an automated test — the other half of backlog 17a) and the migration path itself, which the `backend` CI job runs from zero.

## Implementation gaps vs shipped code (as of 2026-10-01)

Baseline: `main` @ `99b5a0b` (the docs commits on top of it change no code; read
it with `git show 99b5a0b:<path>`). The decisions above are the target. Everything
this spec builds itself matches the shipped code; the two rows below are
divergences in **other modules** that this spec's flows depend on, ordered by
severity (permission floor first). Each is owned and fixed by the spec named in
the row; it closes here when that code lands **and** the SPEC-08 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded.
Paths are relative to `backend/`.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | Implementation Decisions › Service — "uploading is the media module's existing write path (`assets:write:own`, granted to `user` by the SPEC-04 grant migration)" | A `user` uploads journal photos because `user` holds `assets:write:own`, and the upload routes enforce it. | No grant migration exists: `db/migrations/0003_account_rbac.up.sql` grants `assets:write:own` to `creator` only (no later migration, up to `0045`, grants it to `user`). `internal/modules/media/module.go` `MountHTTP` mounts `POST /assets` (upload session), `PUT /assets/{id}/source`, `POST /assets/{id}/complete` and `PATCH /assets/{id}` behind authentication only, with no `RequirePermission`. A `user` can attach photos today only because nothing checks the permission. | migration (owned by SPEC-04 §7): `000N_media_user_asset_grants` seeding `('user','assets:write:own')` and `('user','assets:delete:own')` with the `0003` `WITH grants(...)` pattern. backend: **then**, in a later deploy, `RequirePermission("assets:write:own")` on those routes. The order is load-bearing: enforcing before the grant is applied breaks the journal composer for every `user`. openapi: `x-required-permission` on the upload operations. test: a router test that a `user` with the seeded grant can open an upload session and one without any grant gets 403. | F009 |
| 2 | Frontend — "the composer uploads several files at once" (up to ten); SPEC-05 P0.4 notification rule | A journal photo upload produces no bell notification: only `kind='video'` with `origin='upload'` notifies (SPEC-05 P0.4). | `internal/modules/notify/service.go` `Service.OnAssetReady` skips only `origin == "import"`, so every image the composer uploads (up to ten per Entry) raises its own "ready" notification. | backend (owned by SPEC-05 P0.4): notify only when `kind == "video" && origin == "upload"`; events.md consumer text already says so. test: "10 image uploads for one Entry → zero notifications; one video upload reaching ready → exactly one". | F012 |

No other divergence: no `[c]` finding of the 2026-09-30 review names this spec's own
behaviour, and the 2026-10-01 decisions (Envelopes, Timezone, Audio) do not touch it —
the journal list and `GET /stream` already return `{items, next_cursor?}`, nothing here
reads a timezone, and video or audio Attachments stay out of scope.

**Already matching on `99b5a0b`** (verified, not to be re-built):
- Migrations `0044_journal_attachments_in_columns` / `0045_journal_location_in_columns` exist with the backfill, the self-checking DO blocks, the relaxed body CHECK, the all-or-nothing Location CHECK and the re-encoding `down` (restoring the old CHECK `NOT VALID`).
- `internal/modules/journal/service.go` `validateAssets`: at most ten, no duplicates, each id an existing image Asset with status `ready` owned by the caller, through the narrow `MediaAPI` inside the request's tenant transaction; a nil lookup (the worker) fails closed; the first violation is a 422 `journal/invalid-asset` whose `detail` names the id and the reason (`handler.go` `writeJournalErr`; a non-uuid element is named too, `entryReq.assetIDs`).
- `validBody` judges "text or at least one Attachment" on the **patched result**; a Location alone is refused; `normalizeLocation` + `entryReq.location` give 422 `journal/invalid-location` for partial, blank or out-of-range places; PATCH `asset_ids` replaces the whole list, `location: null` and `mood: null` clear, absent keeps (`query/journal_entries.sql` `PatchEntry`).
- `OnAssetDeleted` strips the id from every Entry of the owner (`StripAssetFromEntries`, `array_remove` keeps order) inside the owner's tenant scope, idempotently, keeps an emptied Entry, and drops an owner-less event with a log line.
- The stream join (`query/journal_stream.sql` `ListStreamCursor`) returns `asset_ids` and the three Location columns; `handler.go` renders Entry and stream item with the same shapes (`[]`, explicit `null`); `shared/openapi.yaml` `JournalEntry`, `JournalEntryWrite` and `StreamItem` carry them.
- Frontend: the composer dedups, caps at ten, gates Save on every photo ready and on text-or-photo (`lib/composer-photos.ts`), edits in place, and the shared `AttachmentGallery` shows a `medium` hero, up to four `thumb` tiles with "+N", and a `medium` lightbox in array order; the markdown link codec is gone; the module README names the media lookup and the `media:asset_deleted` subscription.

**Test evidence to add or fix:**
- No test on `99b5a0b` asserts superseded behaviour for this spec.
- Add with row 1: the upload-permission router test above (SPEC-04 owns it). Add with row 2: the notify test above (SPEC-05 owns it).
- Still without evidence (unchanged, TRACEABILITY-MATRIX T2/T5 ✖): the lightbox, the in-place edit composer and the Location chip — a component or browser test, now that vitest runs in the `frontend` CI job.

## Out of Scope

- Choosing photos from the media library instead of uploading (a picker is a new screen;
  the server does not distinguish the two paths, so it can come later without a contract
  change).
- Reordering Attachments by drag and drop.
- Video or audio Attachments (need a player and transcode state in the card).
- Attaching another user's public Asset (needs a visibility surface on the media API and a
  decision about what happens when the owner changes visibility).
- A Location registry ("the same café again") or server-side geocoding.
- On-this-day (SPEC-09 P1.5) and takeout (SPEC-03 P1.7) — designed for, not built; both
  benefit from bodies being plain markdown.
- A frontend test runner.

## Further Notes

**Why the migration rewrites user text.** "The body is plain markdown" has to hold for
every row or the frontend keeps the decoder as a fallback forever — which is the
workaround under a different name. Two rows are affected; the migration asserts its own
post-condition and the down path re-encodes, so the rewrite is deterministic, visible in
git, and reversible.

**The empty-body edge after deletion.** An Entry saved as photo-only whose photo is later
deleted ends up with no text and no Attachment. It is kept, not deleted: deleting a
journal Entry because a picture went away would surprise the writer more than an empty
card does. The card shows the date, mood and Location, and the composer will not let it be
*saved again* without text or a photo. This is the reason the "text or Attachment" rule is
enforced by the service on user writes and the database CHECK keeps only the length bound:
a CHECK would refuse the consumer's strip on precisely this row.

**Sequence.** Contract and codegen → migration (with backfill) → service, handler and
consumer with the HTTP-contract test → stream join and stream item shape → frontend
composer, card, lightbox, removal of the decoder → docs. The backend half and the
frontend half are separable sessions; tickets declare the edge.
