# SPEC-05 — Journal (life-stream write path)

**Status:** ready to build, rev 1 · **Drafted:** 2026-07-10
**Module:** `journal` (new — not scaffolded) · **Depends on:** nothing hard ([ADR-10](../../adr/10-openapi-contract-direction.md) codegen cutover preferred first; P1.5 photo attachments need SPEC-01)
**Upstream:** [briefs/05-journal-life-stream.md](../briefs/05-journal-life-stream.md) · **Refs:** [ADR-08](../../adr/08-life-os-pivot.md), backlog §3 P1, [MODULES.md](../../../backend/MODULES.md) §8
**Downstream consumers:** SPEC-06 (stream projection reads this table; its projection rows are maintained transactionally in this module's service — journal:entry_created stays emit-only, see P0.3), SPEC-09 P1.7 (takeout exports it).

---

## 1. Problem statement

ADR-08 declares the first real post type to be **a journal / life event of the
user**, not a status for friends — but nothing builds it. SPEC-01/02/03 create
three event *producers* and SPEC-04's bell is their only consumer; the browsable
timeline that ADR-08 calls the product has no **write path**. Meanwhile `HomeView`
renders a ~685-line hard-coded newsfeed, and the ported `Composer`/post/comment
kits are exported but imported by nothing.

The urgency is asymmetric: memories not captured cannot be backfilled. Every week
without a capture surface is life-stream data lost forever — which is why this
spec has zero hard dependencies and is sequenced immediately after SPEC-04 in the
[briefs build order](../briefs/README.md).

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
  social stays parked per [briefs/04-deferred.md](../briefs/04-deferred.md).
- **The merged journal + system-event timeline** — that is SPEC-06. This spec ships
  an interim journal-only list on `/` (P0.4) that SPEC-06 upgrades in place.
- **Rich-text editing** — v1 is markdown-in-textarea with preview. No WYSIWYG, no
  embeds.
- **Auto-entries from bus events** — SPEC-06's projection owns system events; the
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
SPEC-01/04 are also claiming), wired into `cmd/api/main.go` **and**
`cmd/worker/main.go` (the worker side registers nothing at P0 but the wiring
exists for SPEC-06's consumers). Add the module's depguard isolation block to
`backend/.golangci.yml` per the template comment.

### P0.2 — Entries CRUD

**Endpoints** (§7): create / list / fetch / patch / delete under
`/api/v1/journal/entries`. Validation: `body_md` 1–20 000 chars, else 422
`journal/invalid-body`; `mood` optional freeform, but when present must be
1–80 chars after trimming (an empty or whitespace-only string is 422
`journal/invalid-mood`, matching the §6 CHECK — otherwise it would surface as
a 500 at the DB); `occurred_at` optional, defaults to now, **backdating and
future-dating unlimited** (it's a journal; resolved from the brief's open
question). `asset_ids` in the request body is **rejected with 422
`journal/invalid-asset` until P1.5 lands** — fail closed rather than store
unvalidated cross-module references.

**Ordering & pagination (brief inconsistency resolved).** The brief's P0.2 text
said cursor on `created_at` while its index sketch and stream semantics use
`occurred_at`. Resolved: the timeline orders and paginates on
**`(occurred_at DESC, id DESC)`** — a backdated entry belongs at its date, and
SPEC-06's merged stream orders the same way. `created_at` remains the audit
timestamp only.

**Acceptance criteria.**
- Given entries by user B, when user A lists or fetches, then B's rows are absent
  and a direct fetch is 404 (existence never leaks).
- Given 500 entries, when paging by cursor, then results are stable, ordered
  `occurred_at DESC, id DESC`, with no duplicates or gaps across pages.
- Given a body of 0 or > 20 000 chars, then 422 Problem `journal/invalid-body`.
- Given an edit, then `updated_at` changes and the entry keeps its `occurred_at`
  position unless `occurred_at` itself was edited.
- Given a delete, then the entry is gone from list/fetch (and SPEC-06's stream
  row for it is removed — same-module correction, see the P0.3 note).

### P0.3 — Event emit

Emit **`journal:entry_created`** `{entry_id, user_id, occurred_at}` after the
create transaction commits — published via `platform/events` (events.md
"Delivery mechanics"), so a future second consumer is a wiring change.
Register in [docs/reference/events.md](../../reference/events.md) (definition
of done). No consumer is required to ship — the stream projection is
maintained transactionally in-module, not through this event (SPEC-06 P0.1).

**Deliberately no `entry_updated` / `entry_deleted` events at v1**: the only
planned consumer (SPEC-06's projection) lives in the **same module** and is
maintained **transactionally** — entry create inserts the projection row,
`occurred_at` edits update it, delete removes it, all inside the entry's own
transaction (SPEC-06 P0.1 owns the details; resolved 2026-07-10 — the earlier
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
rollback on error. The composer exposes an optional **date control**
(`occurred_at`, defaults to now — the backdating user story needs a UI, not
just an API field) and a minimal freeform mood text input (P0 — Goal 1 and
the primary user story include mood; P1.6 upgrades it with the preset emoji
row). Rendered entry cards carry
**edit and delete affordances** (inline dialog; optimistic per D-32) — user
stories 2–3 were previously API-only *(gap closed 2026-07-10)*. The fixture
post array and the fake composer are **deleted** from `HomeView`. Until
SPEC-06 lands, `/` renders a journal-only list (cursor infinite scroll, newest
`occurred_at` first) in the feed slot — SPEC-06 swaps this query for
`GET /stream` without moving the composer.

Markdown renders through a **sanitizing renderer** — no raw-HTML passthrough
(D-33's RSC shell stays; the list is a client island).

**Acceptance criteria.**
- Given a post from the composer, then the entry appears at its `occurred_at`
  position (top, when now-dated) without a full refetch; on server error it
  disappears and the composer restores its text.
- Given an entry edited to a different `occurred_at`, then it re-sorts to that
  position; given a delete, the card leaves the list optimistically.
- Grep test: the fixture post array is gone from `HomeView`; every rendered entry
  is a DB row.
- Given `<script>alert(1)</script>` in a body, then it renders as inert text.
- Given a mood entered, the created entry stores and renders it.

### P1 — nice to have

- **P1.5 Photo attachments**: `asset_ids uuid[]`, ≤ 10 per entry, each validated
  via `mediaapi` at write time (exists, `kind=image`, `status=ready`, owned by the
  caller) — 422 Problem `journal/invalid-asset` otherwise. Entry cards render
  `thumb` variants; lightbox shows `medium`. **Needs SPEC-01.** Also subscribe to
  `media:asset_deleted` — via the `platform/events` fan-out, events.md "Delivery
  mechanics"; the event has several consumers — and strip the deleted id from any
  `asset_ids` (idempotent — the SPEC-02 P0.6 soft-cascade pattern; only relevant
  once attachments exist). This needs creator-tier `assets:write:own` on
  mediaapi's write path — available because the v1 owner is provisioned
  `creator` or higher (see README AuthZ).
- **P1.6 Mood picker**: composer surfaces a preset emoji row + freeform field
  (schema carries `mood` from P0).

### P2 — future considerations (design for, don't build)

- **Takeout**: entries export as markdown files via SPEC-09 P1.7's
  `ExportProvider` — keep bodies plain markdown, no proprietary markup.
- **On-this-day** (SPEC-06 P1.5) reads this table by month/day of
  `occurred_at`; a streak widget is a possible later read of the same column
  (not specced in SPEC-06) — either way, don't denormalize dates away.

## 6. Data model — migration `000N_journal_entries`

```sql
CREATE TABLE journal_entries (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                -- identity-anchor exception, per SPEC-04 §6 / 0007_media_assets precedent
  body_md     text NOT NULL CHECK (char_length(body_md) BETWEEN 1 AND 20000),
  mood        text CHECK (mood IS NULL OR char_length(mood) BETWEEN 1 AND 80),
  asset_ids   uuid[] NOT NULL DEFAULT '{}',  -- media assets, validated via mediaapi (P1.5)
  occurred_at timestamptz NOT NULL DEFAULT now(),  -- user-editable ("last night")
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON journal_entries (user_id, occurred_at DESC, id DESC);
```

`asset_ids` carries **no FK** (cross-module; validated via `mediaapi`, corrected
via the `media:asset_deleted` subscription — P1.5). Queries in
`query/journal_entries.sql`; regenerate via `make sqlc` — never hand-edit `*.sql.go`.

**Module-scope decision (brief's open question, locked):** one module named
`journal` owns both this table and SPEC-06's `stream_items`. Two micro-modules
for one surface is boundary theater at this size; if the stream ever grows its
own roadmap, splitting later is a mechanical move because the projection already
only consumes bus events.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| POST | `/api/v1/journal/entries` | `journal:write:own` | `{body_md, mood?, occurred_at?}`; `asset_ids?` accepted from P1.5, rejected before |
| GET | `/api/v1/journal/entries?cursor=` | `journal:read:own` | ordered `occurred_at DESC, id DESC` |
| GET | `/api/v1/journal/entries/{id}` | `journal:read:own` | 404 for others' rows |
| PATCH | `/api/v1/journal/entries/{id}` | `journal:write:own` | any subset of create fields |
| DELETE | `/api/v1/journal/entries/{id}` | `journal:delete:own` | 204; idempotent 404 |

Permission codes follow the canonical scheme (README conventions): `write`
covers create + update, matching the 0003 catalog — the earlier
`journal:create`/`journal:update:own` split added a verb style the catalog
doesn't use *(reconciled 2026-07-10)*. The journal migration seeds all three
codes and grants them to the base `user` role. Problem types:
`journal/entry-not-found`, `journal/invalid-body`, `journal/invalid-mood`,
`journal/invalid-asset` (pre-P1.5: any `asset_ids`; post-P1.5: failed
validation).

## 8. Success metrics (n=1 honest)

- Leading: composer-open → saved < 10 s for a routine entry (client-side timing
  during dogfood); entries logged on ≥ 15 of the first 30 days (habit test — the
  journal analogue of SPEC-03's friction metric).
- Lagging: grep shows zero fixture posts in `HomeView`; entries survive `make up`
  restart (persistence, not cache).

## 9. Timeline & phasing

1. Scaffold + migration + sqlc + depguard block (1 day)
2. CRUD + RBAC + OpenAPI + event emit (1.5 days)
3. Composer wiring + interim home list + fixture deletion (1.5–2 days)
4. P1 (attachments + mood picker) (1 day, needs SPEC-01)
P0 ≈ 4–5 dev-days; matches the brief's 5–6 including P1.

## 10. Open questions

- **(resolved)** Module naming: one `journal` module owns entries + SPEC-06's
  projection (§6).
- **(resolved)** Backdating: unlimited; timeline orders by `occurred_at` (P0.2).
- **(engineering, non-blocking)** Markdown renderer choice on the frontend
  (sanitization is the requirement; the library is an implementation detail).
- **(product, non-blocking)** Should `mood` graduate to a curated set once SPEC-06
  wants to aggregate it ("mood this month")? Freeform at v1; revisit with data.
