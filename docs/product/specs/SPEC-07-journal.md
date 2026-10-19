# SPEC-07 — Journal (life-stream write path)

**Status:** current, rev 1 · **Drafted:** 2026-07-10 · **Last verified:** 2026-10-01
**Module:** `journal` · **Depends on:** SPEC-04 P0.6 (`platform/events` fan-out) only; photo attachments (P1.5) are superseded by SPEC-08, which needs SPEC-04
**Upstream:** brief 05 (folded into this spec, then deleted — `git show ea100d8:docs/product/briefs/05-journal-life-stream.md`) · **Refs:** [ADR-08](SPEC-09-life-stream-home.md#adr-08), 2026-07 backlog §3 P1 "Posts/newsfeed API", reframed per ADR-08 (archived; `git show 8d382d2^:docs/product/backlog.md`), [MODULES.md](../../../backend/MODULES.md) §8
**Downstream consumers:** SPEC-09 (stream projection reads this table; its projection rows are maintained transactionally in this module's service — journal:entry_created stays emit-only, see P0.3), SPEC-03 P1.7 (takeout exports it).

---

## 1. Problem statement

ADR-08 declares the first real post type to be **a journal / life event of the
user**, not a status for friends — but nothing builds it. SPEC-04 (and later
SPEC-14/12) create event *producers*; the only planned consumer ahead of this
spec is SPEC-05's bell (for `media:asset_ready`); the browsable timeline that
ADR-08 calls the product has no **write path**. Meanwhile `HomeView`
renders a ~685-line hard-coded newsfeed, and the ported `Composer`/post/comment
kits are exported but imported by nothing.

The urgency is asymmetric: memories not captured cannot be backfilled. Every week
without a capture surface is life-stream data lost forever — which is why this
spec has zero hard dependencies and is sequenced immediately after SPEC-05 in the
[build order](README.md#build-state-and-what-remains).

## 2. Goals

1. The owner captures a journal entry (text + mood) in **< 10 s from `/`** —
   composer open → saved.
2. Entries are owner-scoped rows in a real module with the standard layout
   (MODULES.md §8) — a new module wired end-to-end after account and media
   (and after `notify`, when the build order holds).
3. `journal:entry_created` is on the bus from day one (ADR-08's day-one-event rule).
4. The fake composer and fixture posts are **deleted** from `HomeView`; every
   rendered entry is a DB row.

## 3. Non-goals

- **Comments, reactions, sharing** — single user; entries are flat. Multi-user
  social stays parked per [backlog.md § Deferred](../backlog.md).
- **The merged journal + system-event timeline** — that is SPEC-09. This spec ships
  an interim journal-only list on `/` (P0.4) that SPEC-09 upgrades in place.
- **Rich-text editing** — v1 is markdown-in-textarea with preview. No WYSIWYG, no
  embeds.
- **Auto-entries from bus events** — SPEC-09's projection owns system events; the
  `journal_entries` table stores **only human-authored rows**. This keeps the
  table's meaning honest and is load-bearing for P2's on-this-day/streak reads.
- Full-text search over entries — future; don't preclude (bodies stay plain markdown).

## 4. User stories

- As the owner, I jot "ăn tối với mẹ, vui" with a mood in < 10 s from the home
  page, and it appears at the top of my stream. *(primary)*
- As the owner, I edit or delete an entry I regret — it's my journal, not a ledger.
- As the owner, I backdate an entry to last night, and it sits at last night's
  position in the timeline, not at the top.
- As the owner, I scroll back months of entries, newest first, without the page
  degrading.
- Edge: as the owner, I paste markdown containing raw HTML/script — it renders as
  inert text, never as live markup.

## 5. Requirements

### P0.1 — Module scaffold

Full MODULES.md §8 checklist: `internal/modules/journal/` subtree (`module.go`
with `New(Deps)` / `MountHTTP` / `RegisterTasks`, `api/`, `handler/`, `service/`,
`query/`, `repository/`), its own `sqlc.yaml` block, migration
`000N_journal_entries` (**take the next free number** — 0007 is consumed and
SPEC-04/05 are also claiming), wired into `cmd/api/main.go` **and**
`cmd/worker/main.go` (the worker side registers nothing at P0 but the wiring
exists for SPEC-09's consumers). Add the module's depguard isolation block to
`backend/.golangci.yml` per the template comment.

### P0.2 — Entries CRUD

**Endpoints** (§7): create / list / fetch / patch / delete under
`/api/v1/journal/entries`. Validation: `body_md` 1–20 000 chars, else 422
`journal/invalid-body` (SPEC-08 later relaxes this to 0–20 000 so a photo-only
entry can exist; a text-only entry still needs non-blank text); `mood` optional
freeform — absent or `null` means no mood. The service validates and **stores
`strings.TrimSpace(mood)`**: the trimmed value must be 1–80 chars, otherwise 422
`journal/invalid-mood`; a whitespace-only mood trims to empty and is 422. Any
stored trimmed value satisfies the §6 CHECK, so padded input can never reach
the DB as a 500 at COMMIT. `occurred_at` optional, defaults to now, **backdating and
future-dating unlimited** (it's a journal; resolved from the brief's open
question). `asset_ids` in the request body is **rejected with 422
`journal/invalid-asset` until SPEC-08 lands** — fail closed rather than store
unvalidated cross-module references.

**Length unit.** Lengths (`body_md`, `mood`) are Unicode code points, as
Postgres `char_length` counts them — the service counts with
`utf8.RuneCountInString`, never `len` (bytes); a frontend counter uses
`[...str].length`, never `.length` (UTF-16 units). Vietnamese text must not be
rejected early nor pass the service and fail the CHECK.

**Ordering & pagination (brief inconsistency resolved).** The brief's P0.2 text
said cursor on `created_at` while its index sketch and stream semantics use
`occurred_at`. Resolved: the timeline orders and paginates on
**`(occurred_at DESC, id DESC)`** — a backdated entry belongs at its date, and
SPEC-09's merged stream orders the same way. `created_at` remains the audit
timestamp only.

**Acceptance criteria.**
- Given entries by user B, when user A lists, fetches, PATCHes or DELETEs one of
  B's entries, then the list omits it and fetch/PATCH/DELETE return 404
  `journal/entry-not-found` (existence never leaks); B's row is unchanged.
- Given 500 entries, when paging by cursor, then results are stable, ordered
  `occurred_at DESC, id DESC`, with no duplicates or gaps across pages.
- Given 500 entries and `limit=50`, then exactly 10 pages, the last without
  `next_cursor`; a malformed cursor is 400 `journal/invalid-cursor`.
- Given a body of 0 or > 20 000 chars, then 422 Problem `journal/invalid-body`.
- Given mood `" vui "`, then it is stored as `"vui"`; given 80 non-space code
  points padded with spaces, then 201 (not 500); given 81 non-space code points,
  then 422 `journal/invalid-mood`.
- Given a PATCH with `mood: null`, then the mood is cleared; given a PATCH that
  omits a field, then that field is unchanged.
- Given an edit, then `updated_at` changes and the entry keeps its `occurred_at`
  position unless `occurred_at` itself was edited.
- Given a delete, then the entry is gone from list/fetch (and SPEC-09's stream
  row for it is removed — same-module correction, see the P0.3 note).

### P0.3 — Event emit

Emit **`journal:entry_created`** `{entry_id, user_id, occurred_at}` after the
create transaction commits — published via `platform/events` (events.md
"Delivery mechanics"), so a future second consumer is a wiring change.
Register in [docs/reference/events.md](../../reference/events.md) (definition
of done). No consumer is required to ship — the stream projection is
maintained transactionally in-module, not through this event (SPEC-09 P0.1).

**Deliberately no `entry_updated` / `entry_deleted` events at v1**: the only
planned consumer (SPEC-09's projection) lives in the **same module** and is
maintained **transactionally** — entry create inserts the projection row,
`occurred_at` edits update it, delete removes it, all inside the entry's own
transaction (SPEC-09 P0.1 owns the details; resolved 2026-07-10 — the earlier
async-consumer sketch raced the composer's post-create refetch). The event
above is therefore **emit-only**, for future external consumers. Revisit the
moment one exists.

**Acceptance criteria.**
- Given a created entry, then `events.Publish` is called exactly once with
  `journal:entry_created` and the registered payload, strictly after commit (a
  rolled-back create publishes nothing). With zero registered subscribers at v1
  this enqueues no consumer tasks — assert on the Publish call (spy publisher),
  not on queue contents, and never enqueue the raw event name as a task type
  (no handler exists).

### P0.4 — Composer wiring + interim home list (frontend)

The home composer becomes real: the ported `Composer` kit posts to
`POST /journal/entries` via a TanStack mutation with **optimistic insert** (D-32),
rollback on error. The composer exposes an optional **date-and-time control**
(`occurred_at`, defaults to now — the backdating user story needs a UI, not
just an API field). The picker works in the user's timezone — `users.timezone`
from `GET /auth/me` (specs README Timezone, D-17) — which is by construction
the zone the stream groups days in and on-this-day matches in (SPEC-09 cites
the same rule), not the browser's zone when the two differ; the client sends
RFC 3339 with that zone's offset, and the server stores the `timestamptz`
instant unchanged. A date picked without a time defaults to 12:00 in that zone.
*(Code follow-up: the home Composer has no `occurred_at` control yet, and
`lib/time.ts` reads its zone from `GET /api/v1/time` (`APP_TIMEZONE`), not the
user — README Timezone follow-up; §11 row 2.)* The composer
also has a minimal freeform mood text input (P0 — Goal 1 and
the primary user story include mood; P1.6 upgrades it with the preset emoji
row). Rendered entry cards carry
**edit and delete affordances** (inline dialog; optimistic per D-32) — user
stories 2–3 were previously API-only *(gap closed 2026-07-10)*. The fixture
post array and the fake composer are **deleted** from `HomeView`. Until
SPEC-09 lands, `/` renders a journal-only list (cursor infinite scroll, newest
`occurred_at` first) in the feed slot — SPEC-09 swaps this query for
`GET /stream` without moving the composer.

**Optimistic placement** (the list is a cursor-paginated infinite query, so a
backdated entry may have no loaded position): insert into the loaded page whose
range contains `occurred_at`. If it is older than the last loaded item and
`hasNextPage`, do not insert; show a "Saved to <date>" toast. Dedupe by entry
id (the stream's `ref_id`, SPEC-09 P0.2) against fetched pages. The same rule
applies to edits that change `occurred_at`. On error, restore body, mood and
`occurred_at`.

Markdown renders through a **sanitizing renderer** — no raw-HTML passthrough
(D-33's RSC shell stays — as defined in the specs README Frontend convention;
the list is a client island). The composer has a **Write/Preview toggle**
(§3's "markdown-in-textarea with preview"); Preview renders the draft through
the same sanitizing renderer as the entry card, not as raw text.

**Acceptance criteria.**
- Given a post from the composer, then the entry appears at its `occurred_at`
  position (top, when now-dated) without a full refetch; on server error it
  disappears and the composer restores its body, mood and `occurred_at`.
- Given a post backdated older than the last loaded entry while more pages
  exist, then it is not inserted, a "Saved to <date>" toast shows, and it
  appears exactly once when its page loads.
- Given an entry edited to a different `occurred_at`, then it re-sorts to that
  position; given a delete, the card leaves the list optimistically.
- Grep test: the fixture post array is gone from `HomeView`; every rendered entry
  is a DB row.
- Given `<script>alert(1)</script>` in a body, then it renders as inert text.
- Given a mood entered, the created entry stores and renders it.
- Given `occurred_at` = yesterday 21:00 in the user's zone, then it is stored at that instant
  and listed between its neighbours.
- Given a draft `**bold**`, then Preview renders it bold; given `<script>` in a
  draft, then Preview renders it inert.

### P1 — nice to have

- **P1.5 Photo attachments** — *superseded 2026-09-12 by [SPEC-08](SPEC-08-journal-attachments.md), which keeps the ≤ 10 / image / ready / owned rules below, adds Location as Entry columns, allows a photo-only Entry, and backfills the interim markdown-link workaround out of every body. Kept here as the original decision; build from SPEC-08.* Original text: `asset_ids uuid[]`, ≤ 10 per entry, each validated
  via `mediaapi` at write time (exists, `kind=image`, `status=ready`, owned by the
  caller) — 422 Problem `journal/invalid-asset` otherwise. Entry cards render
  `thumb` variants; lightbox shows `medium`. **Needs SPEC-04.** Also subscribe to
  `media:asset_deleted` — via the `platform/events` fan-out, events.md "Delivery
  mechanics"; the event has several consumers — and strip the deleted id from any
  `asset_ids` (idempotent — the SPEC-14 P0.6 soft-cascade pattern; only relevant
  once attachments exist). Uploading needs `assets:write:own`, granted to `user` by the
  SPEC-04 grant migration (SPEC-04 §7; README AuthZ floor).
- **P1.6 Mood picker**: composer surfaces a preset emoji row + freeform field
  (schema carries `mood` from P0).

### P2 — future considerations (design for, don't build)

- **Takeout**: entries export as markdown files via SPEC-03 P1.7's
  `ExportProvider` — keep bodies plain markdown, no proprietary markup.
- **On-this-day** (SPEC-09 P1.5) reads this table by month/day of
  `occurred_at`; a streak widget is a possible later read of the same column
  (not specced in SPEC-09) — either way, don't denormalize dates away.

## 6. Data model — migration `000N_journal_entries`

**Tenancy** (specs README convention, ADR-07). Tenant-scoped: `journal_entries` (`tenant_id` + index + `tenant_isolation` policy, added by `0020_platform_rls_enable`). The DDL below predates ADR-07 and omits the columns.

```sql
CREATE TABLE journal_entries (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                -- identity-anchor exception, per SPEC-05 §6 / 0007_media_assets precedent
  body_md     text NOT NULL CHECK (char_length(body_md) BETWEEN 1 AND 20000),
  mood        text CHECK (mood IS NULL OR char_length(mood) BETWEEN 1 AND 80),
  asset_ids   uuid[] NOT NULL DEFAULT '{}',  -- media assets, validated via mediaapi (SPEC-08)
  occurred_at timestamptz NOT NULL DEFAULT now(),  -- user-editable ("last night")
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON journal_entries (user_id, occurred_at DESC, id DESC);
```

`asset_ids` carries **no FK** (cross-module; validated via `mediaapi`, corrected
via the `media:asset_deleted` subscription — SPEC-08). SPEC-08's migration
relaxes the body CHECK to `char_length(body_md) <= 20000` (0–20 000) for
photo-only entries; the text-or-attachment rule is a service rule. Queries in
`query/journal_entries.sql`; regenerate via `make sqlc` — never hand-edit `*.sql.go`.

**Module-scope decision (brief's open question, locked):** one module named
`journal` owns both this table and SPEC-09's `stream_items`. Two micro-modules
for one surface is boundary theater at this size. If the stream ever grows its
own roadmap, a split is **not** mechanical: system rows arrive via bus events,
but journal rows are projected inside the entry transaction (P0.3, SPEC-09
P0.1(a)). A split must first replace that write with
`journal:entry_created`/`entry_updated`/`entry_deleted` events plus an
idempotent consumer, and accept eventual consistency on the composer's
post-create refetch.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| POST | `/api/v1/journal/entries` | `journal:write:own` | `{body_md, mood?, occurred_at?}`; `asset_ids?` accepted from SPEC-08, rejected before |
| GET | `/api/v1/journal/entries?cursor=&limit=` | `journal:read:own` | ordered `occurred_at DESC, id DESC`; `limit` default 50, max 100, lenient per the specs README rule (missing, non-integer or < 1 → 50; above 100 → clamped to 100; never a Problem); response `{items, next_cursor}`, `next_cursor` an opaque base64url keyset of `(occurred_at, id)`, omitted on the last page; malformed cursor → 400 `journal/invalid-cursor` |
| GET | `/api/v1/journal/entries/{id}` | `journal:read:own` | 404 for others' rows |
| PATCH | `/api/v1/journal/entries/{id}` | `journal:write:own` | any subset of create fields. Absent field = unchanged; `mood: null` clears it (per SPEC-08); `body_md: null` and `occurred_at: null` are treated as absent (unchanged — neither column can be cleared); a present `body_md` or `mood` is validated as on create. The UPDATE sets `updated_at = now()` (specs README updated_at convention) |
| DELETE | `/api/v1/journal/entries/{id}` | `journal:delete:own` | 204 on delete; 404 `journal/entry-not-found` when the id is unknown, already deleted, or foreign (a repeat DELETE is 404, never 500) |

Permission codes follow the canonical scheme (README conventions): `write`
covers create + update, matching the 0003 catalog — the earlier
`journal:create`/`journal:update:own` split added a verb style the catalog
doesn't use *(reconciled 2026-07-10)*. The journal migration seeds all three
codes and grants them to the base `user` role. Problem types:
`journal/entry-not-found`, `journal/invalid-body`, `journal/invalid-mood`,
`journal/invalid-asset` (before SPEC-08: any `asset_ids`; after SPEC-08: failed
validation), `journal/invalid-cursor` (400, malformed list cursor).

The list follows the specs README Pagination convention (`{items, next_cursor}`);
body-shape failures use the named `journal/invalid-*` types above rather than
`journal/validation`. Annotate each operation per the specs README AuthZ **OpenAPI encoding**.

## 8. Success metrics (n=1 honest)

- Leading: composer-open → saved < 10 s for a routine entry (client-side timing
  during dogfood); entries logged on ≥ 15 of the first 30 days (habit test — the
  journal analogue of SPEC-12's friction metric).
- Lagging: grep shows zero fixture posts in `HomeView`; entries survive `make up`
  restart (persistence, not cache).

## 9. Timeline & phasing

1. Scaffold + migration + sqlc + depguard block (1 day)
2. CRUD + RBAC + OpenAPI + event emit (1.5 days)
3. Composer wiring + interim home list + fixture deletion (1.5–2 days)
4. P1.6 mood picker (0.5 day); attachments moved to SPEC-08

P0 ≈ 4–4.5 dev-days; 4.5–5 including P1.6 (the brief estimated 5–6, which
included attachments, now SPEC-08).

## 10. Open questions

- **(resolved)** Module naming: one `journal` module owns entries + SPEC-09's
  projection (§6).
- **(resolved)** Backdating: unlimited; timeline orders by `occurred_at` (P0.2).
- **(engineering, non-blocking)** Markdown renderer choice on the frontend
  (sanitization is the requirement; the library is an implementation detail).
- **(product, non-blocking)** Should `mood` graduate to a curated set once SPEC-09
  wants to aggregate it ("mood this month")? Freeform at v1; revisit with data.

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

Baseline: `main` @ `99b5a0b` (the docs commits on top of it change no code; read
it with `git show 99b5a0b:<path>`). The text above is the target; every row below
is a place where the shipped code still diverges from it, ordered by severity —
data integrity first, then user-visible behaviour, then contract hygiene. A row
closes when the code matches the requirement **and** the SPEC-07 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded on
test evidence; delete the row in that PR. The inline *Code follow-up* notes above
point here. Paths are relative to `backend/` or `frontend/src/`.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.3 Event emit — "strictly after commit" | `journal:entry_created` is published only after the create transaction has committed; a create that does not commit publishes nothing. | `internal/modules/journal/service.go` `Service.Create` calls `emitCreated` as soon as `repo.CreateEntry` returns. On the API the adapter's `RunInTx` (`internal/platform/db/db.go` `DB.RunInTx`) runs on a **savepoint** of the request transaction that `internal/modules/tenant/middleware/require_tenant.go` opens and commits only after the handler has returned. `Publish` therefore precedes the real COMMIT; a COMMIT that fails (the middleware then discards the buffered 201) leaves an event for a row that never existed. Harmless today (zero subscribers), wrong the day a consumer lands. | backend: give the request scope an after-commit hook (e.g. `db.AfterCommit(ctx, fn)`: queued on the request tx, run by `RequireTenant` only after `tx.Commit` succeeds, run at once when there is no outer tx) and register `emitCreated` through it. test: a router-level test whose outer COMMIT fails asserts zero `Publish` calls (TC-JRNL-030/031). | Found while verifying this section (no F-ID) |
| 2 | P0.4 composer date-and-time control, in the user's `users.timezone` | Optional `occurred_at` control (defaults to now; date without time → 12:00) in the zone `users.timezone` from `GET /auth/me`, sent as RFC 3339 with that zone's offset; edits that change it re-sort the card. | `templates/v1/components/composer/Composer.tsx`: `ComposerDraft` is `{bodyMd, mood, assetIds, location}` — no `occurred_at`; `templates/v1/views/home/HomeView.tsx` `handleCreate` / `handleSaveEdit` never send it (the optimistic item uses `new Date()`). `lib/time.ts` `useTimeConfig` reads the zone from `GET /api/v1/time` (`cmd/api/main.go` `handleServerTime(cfg.AppTimezone)`, `APP_TIMEZONE`). `GET /auth/me` returns no `timezone`; `db/migrations/0002_account_users.up.sql` defaults `timezone` to `'UTC'` and nothing writes it. | migration (account, README Timezone; SPEC-01 P0.13): `users.timezone` NULLable with no default, never-chosen `'UTC'` rows set to NULL (the composer uses the device zone until one is stored). backend: `/auth/me` returns `timezone`; `PATCH /auth/me {timezone}` with `account/invalid-timezone`. openapi: `CurrentUser.timezone`, the PATCH. frontend: a date-and-time control in `Composer` (create and edit; edit pre-fills from the Entry and PATCHes `occurred_at`), built on `toDatetimeLocalInTz` / `fromDatetimeLocalInTz` with the `/auth/me` zone; `lib/time.ts` takes its display zone from `/auth/me`, and `/time` reports only the server clock. test: TC-JRNL-058, TC-JRNL-061, TC-JRNL-062 (vitest on the zone conversion). | F073; Decision 2026-09-30 (Timezone) |
| 3 | P0.4 sanitizing markdown renderer + Write/Preview toggle | Entry bodies render as markdown through one sanitizing renderer (no raw-HTML passthrough); the composer's Preview renders the draft through the same renderer. | `templates/v1/components/stream/StreamItemCard.tsx` `RichText` prints `body_md` as plain text (only a leading `#` heading is lifted by `splitHeading`; its comment says "there is no markdown renderer in the bundle"). `Composer.tsx` Preview shows `bodyMd` verbatim in a `whitespace-pre-wrap` div. Output is inert (React text nodes), but no markdown renders. | frontend: add one renderer (markdown → React elements with raw HTML disabled, e.g. `react-markdown` without `rehype-raw`; links `rel="noreferrer noopener"`) used by the journal card body and by Preview; keep the shared-link lift (`lib/links.ts`) on top of it. test: vitest on the renderer — `**bold**` → `<strong>`, `<script>` and `<img onerror>` inert (TC-JRNL-055, TC-JRNL-056, TC-JRNL-060). | F146 |
| 4 | P0.4 optimistic placement | Insert into the loaded page whose range holds `occurred_at`; older than the last loaded item while `hasNextPage` → no insert, "Saved to <date>" toast; dedupe by entry id (`ref_id`) against fetched pages; the same rule for edits that change `occurred_at`; on error restore body, mood and `occurred_at`. | `HomeView.tsx` `create.onMutate` always `prepend`s into page 0 with `ref_id` = a temp id; there is no range check, no toast and no dedupe; `update.onMutate` patches the card in place and never re-sorts. (Body and mood are restored on error — matching.) | frontend: a pure `placeOptimistic(pages, item, hasNextPage)` helper in `lib/`; on POST success swap the temp `ref_id` for `entry.id` and dedupe fetched pages by `ref_id`; an `occurred_at` edit removes and re-places the card; the toast date is in the user's zone (row 2). test: vitest on the helper (TC-JRNL-050, TC-JRNL-051, TC-JRNL-052). One change closes SPEC-09 §11 row 10 too — the home stream is this list. | F015 |
| 5 | P1.6 mood picker | The composer offers a preset emoji row plus the freeform field. | `Composer.tsx` has only the freeform mood chip (`moodOpen`, one text input). | frontend: preset emoji row that fills the same `mood` field. test: TC-JRNL-074 (vitest for any pure rule). | TRACEABILITY-MATRIX SPEC-07 P1.5/P1.6 ⚠ |
| 6 | §7 Problem types — `journal/invalid-cursor` | Every emitted journal Problem type has an i18n key (README Definition of done). | `internal/modules/journal/handler.go` `writeJournalErr` emits 400 `journal/invalid-cursor`, but `lib/problems.ts` has no `journal/invalid-cursor` key and still carries `stream/invalid-cursor`, which nothing emits. | frontend: add `journal/invalid-cursor` to `lib/problems.ts`, delete `stream/invalid-cursor`. test: TC-JRNL-091. | F069, F024, F031 |
| 7 | §7 OpenAPI encoding | Each operation carries its permission per the README AuthZ **OpenAPI encoding**; the list documents the 400 and the lenient `limit`. | `shared/openapi.yaml` `createJournalEntry` / `listJournalEntries` / `getJournalEntry` / `patchJournalEntry` / `deleteJournalEntry` carry `security: bearerAuth` but no `x-required-permission`; `listJournalEntries` declares no 400 `journal/invalid-cursor`, and its `limit` schema (`minimum: 1, maximum: 100`) does not say that `limit` is lenient (defaulted below 1, clamped above 100). | openapi: `x-required-permission: journal:write:own` (POST, PATCH), `journal:read:own` (both GETs), `journal:delete:own` (DELETE); a 400 Problem response on the list; a `limit` description stating the default and the clamp; regenerate and commit the generated code (ADR-10). test: the drift gate once it checks annotations. | F025, F069 |
| 8 | §7 list `limit` (owner decision 2026-10-01) | Missing, non-integer or < 1 → 50; above 100 → **clamped to 100**; never a Problem. | `service.go` `Service.List`: `if limit <= 0 \|\| limit > maxListLimit { limit = defaultListLimit }` — `?limit=500` returns 50 (`handler.go` `List` parses with `server.AtoiSafe`). | backend: clamp instead of resetting (`> 100 → 100`, `≤ 0 → 50`), e.g. `platform/server.Limit(r, 50, 100)`. openapi: row 7's `limit` description. test: TC-JRNL-029. | Decision 2026-10-01 (limit) |

**Already matching on `99b5a0b`** (verified, not to be re-built):
- P0.1 — the module has the MODULES.md §8 layout, its `sqlc.yaml` block and the `module-journal-isolation` depguard block, and is constructed in both `cmd/api` and `cmd/worker`.
- P0.2 — `GetEntry` / `PatchEntry` / `DeleteEntry` are owner-scoped (`WHERE … AND user_id`), so a stranger's fetch, PATCH or DELETE and a repeat DELETE all answer 404 `journal/entry-not-found`; the list is keyset `(occurred_at DESC, id DESC)`, `limit` defaults to 50 for a missing or invalid value (above 100 also resets to 50 instead of clamping — row 8), the envelope is `{items, next_cursor}` with `next_cursor` omitted on the last page, and a malformed cursor is 400 `journal/invalid-cursor`.
- P0.2 — `normalizeMood` stores `strings.TrimSpace(mood)` and counts 1–80 code points; `validBody` counts with `utf8.RuneCountInString`; PATCH keeps absent fields, `mood: null` clears, `body_md: null` / `occurred_at: null` are unchanged (`COALESCE`), and every UPDATE sets `updated_at = now()`; backdating is unlimited.
- P0.3 / SPEC-09 P0.1(a) — the stream row is inserted, moved and deleted inside the entry's own transaction (`repository/adapter.go` `CreateEntry` / `PatchEntry` / `DeleteEntry`); no `entry_updated` / `entry_deleted` event exists; the payload is `{entry_id, user_id, occurred_at}`.
- P0.4 — `HomeView` has no fixtures; journal cards carry Edit/Delete (optimistic delete, edit in place); the composer has a freeform mood; bodies render inert; on a failed post the composer keeps body, mood, photos and Location.
- §7 — `0011_journal_entries` seeds `journal:read:own` / `write:own` / `delete:own` to `user`, and each route sits behind its `RequirePermission`.

**Test evidence to add or fix:**
- `journal_test.go: TestCreateEmitsExactlyOnceAfterCommit`, `TestCreateRollbackPublishesNothing` — they drive a fake repository and cannot observe the request-transaction COMMIT, so the name over-claims; keep them and add the commit-failure test of row 1 (TC-JRNL-030/031).
- TC-JRNL-019 ("`asset_ids` rejected pre-SPEC-08") is superseded by SPEC-08; its evidence is `http_test.go: TestHTTPCreateStoresAttachmentsInOrder`. TC-JRNL-013 now reads "empty body **without an Attachment** → 422" (`http_test.go: TestHTTPTextOrAttachment`).
- Behaviour that already holds but has no test: TC-JRNL-025 (500 entries, `limit=50` → 10 pages; `TestListCursorPaginates` uses 5 rows), TC-JRNL-026 (malformed cursor → 400), TC-JRNL-027 (80 code points padded with spaces → 201). Cite the existing `http_test.go: TestHTTPEntryIsNotFoundToAStranger` (TC-JRNL-011/024) and `TestHTTPDeleteTwiceIs404` (TC-JRNL-023) in the matrix's P0.2 row.
- New with the rows above: TC-JRNL-050…052, TC-JRNL-055, TC-JRNL-056, TC-JRNL-058, TC-JRNL-060…062, TC-JRNL-074, TC-JRNL-091.
