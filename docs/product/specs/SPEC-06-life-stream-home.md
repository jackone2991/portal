# SPEC-06 — Life-Stream Home (read path: projection + dashboard)

**Status:** ready to build, rev 1 · **Drafted:** 2026-07-10
**Module:** `journal` (extends SPEC-05; owns `stream_items` per its §6 decision) + frontend home · **Depends on:** SPEC-05 (hard — first content + module home); system events attach as their producers land (SPEC-01 P1.2/P0.3, SPEC-02 P1.9, SPEC-03 P0.7, SPEC-07 P1.5, SPEC-08 P0.4); the widget rail additionally consumes SPEC-04's GET /me/notifications — **every widget and consumer degrades to an empty state**, none is a blocker
**Upstream:** [briefs/06-life-stream-home.md](../briefs/06-life-stream-home.md) · **Refs:** [ADR-08](../../adr/08-life-os-pivot.md), [events.md](../../reference/events.md), frontend.md
**Downstream consumers:** SPEC-04 P2 daily digest (reads this projection), future on-this-day widgets

---

## 1. Problem statement

Brief 00 deferred "does the life stream replace the newsfeed on `/` or live
alongside it?" until two event producers existed — SPEC-05 plus SPEC-01 P1.2 gets
there. Today `/` is fixture data end to end: fake posts, fake widgets, fake
activity. ADR-08's thesis is that **integration beats one-app-per-domain**, and
the only place that thesis is visible is a home screen where the facets meet. If
the first screen stays fake, no individual module creates the daily habit.

There is also a durability asymmetry: bus events not captured now are lost — the
projection must exist **before** the producer modules land, not after, or the
stream starts with holes.

**Decision this spec carries (resolves brief 00's open question):** the life
stream **replaces** the newsfeed on `/` — no tab, no toggle. The SPEC-05 composer
sits on top; the widget rail carries the facets.

## 2. Goals

1. `/` renders a real reverse-chron life stream: journal entries + system events,
   one timeline.
2. A `stream_items` projection captures bus events **durably from day one**,
   idempotent under Asynq redelivery.
3. The Olympus widget rail shows real data per facet, empty-state-safe, with each
   widget failure-isolated.
4. Zero fixture data left on the home route (extends SPEC-05's grep test).

## 3. Non-goals

- **Social feed mechanics** (likes, comments, follows) — n=1.
- **The bell/notification UX** — that is SPEC-04. The stream is a *timeline*, not
  an unread queue; the two share producers, never storage. (SPEC-04 §3 explicitly
  anticipated this store: notifications are a delivery store, the life-stream
  archive is its own system-of-record — `stream_items` is that archive.)
- **Push/email digest delivery** — P2 here, promotes SPEC-04's P2 digest seam.
- **Weather widget** — dropped, not wired (closes the backlog §3 P2 question).
- Editing/deleting *system* stream items — they are projections of facts; the fix
  for a wrong fact is in the owning module.

## 4. User stories

- As the owner, I open `/` and see today: what I wrote, what finished
  transcoding, what I spent — one timeline, newest first. *(the ADR-08 proof)*
- As the owner, I glance at the rail: month-to-date spend, continue-watching,
  upcoming birthdays — without opening three apps.
- As the owner, a year from now, "on this day" resurfaces what I did today.
- Edge: as the owner on a fresh instance with only account + media wired, the
  home page renders cleanly with empty states — no errors, no fixtures.

## 5. Requirements

### P0.1 — Projection: `stream_items` + consumers

Table per §6, owned by `journal` (SPEC-05 §6 decision). Two ingestion paths
*(rev 2026-07-10 — the original single async design had a dedup collision, a
transfer-duplication bug, and a create/refetch race; all fixed here)*:

**(a) Journal rows — transactional, no bus.** SPEC-05's service maintains the
projection **inside the entry's own transaction**: create inserts the stream
row, an `occurred_at` edit updates the row's `occurred_at` (otherwise a
backdated edit would leave the item at its stale position — the merged cursor
orders on `stream_items.occurred_at`), delete removes it. `journal:entry_created`
stays emit-only for future external consumers. This kills the race where a
post-create refetch beat the async consumer, and needs no redelivery
protection at all. Journal projection rows are written as
`source_module='journal'`, `event_type='journal:entry_created'` (registry
name, uniform even without bus delivery), `ref_id`=entry id, `payload='{}'`;
journal items render by joining `journal_entries` (P0.2). The
`000N_journal_stream_items` backfill `INSERT … SELECT` writes the same values.

**(b) System events — via the `platform/events` fan-out** (events.md "Delivery
mechanics"; Asynq gives one handler per task type, and notify already consumes
some of these events, so the stream registers its own consumer task —
never the raw event name):

| Event | Arrives with | `ref_id` used | Handler action |
|---|---|---|---|
| `media:asset_ready` | SPEC-01 P1.2 | `asset_id` | insert; **skip `origin='import'`** (zip-import flood guard) |
| `media:playback_completed` | SPEC-07 P1.5 | `asset_id` | insert (distinct `event_type` — no collision with asset_ready) |
| `media:asset_deleted` | SPEC-01 P0.3 | `asset_id` | **delete ALL** `source_module='media'` rows with this `ref_id`, any event_type — else a "watched X" card dangles |
| `bank:transaction_created` | SPEC-03 P0.7 | **`transfer_id` when `is_transfer`, else `transaction_id`** | insert — both transfer legs share `transfer_id`, so the unique key collapses them into ONE "moved X" item (SPEC-03 P0.7's intent; two items per transfer was a bug) |
| `bank:transaction_updated` | SPEC-03 P0.7 | same rule | upsert: `INSERT … ON CONFLICT (source_module, event_type, ref_id) DO UPDATE SET payload, occurred_at` — writing under the created-event key when absent, so a reordered created retry then hits `DO NOTHING` and the corrected payload wins (a corrected amount must not render wrong forever) |
| `bank:transaction_deleted` | SPEC-03 P0.7 | same rule | delete the matching row |
| `comic:chapter_published` | SPEC-02 P1.9 | `chapter_id` | insert |
| `people:birthday_upcoming` | SPEC-08 P0.4 | **`notice_id`** (in the payload — SPEC-08 §6) | insert — keying on `person_id` would make the unique constraint swallow the day-of event and every later year (the original critical bug) |

Inserts use `ON CONFLICT (source_module, event_type, ref_id) DO NOTHING` —
idempotency under redelivery is structural. `payload` stores the event's raw
registered payload (render-minimum per events.md); `occurred_at` = the
payload's own timestamp field where one exists (`occurred_at` on bank rows),
else ingest time.

**Known residual risk (accepted at v1, documented):** a `*_created` retry
processed *after* the corresponding `*_deleted` resurrects an item —
`ON CONFLICT` can't prevent re-insert once the row is gone. Rare,
self-correcting on the next delete, and a P2 reconcile sweep is the seam.
Second accepted residual: producers other than SPEC-08 emit post-commit without
an outbox, so a producer crash between commit and `Publish` drops that item
permanently. Accepted at v1; a P2 reconcile sweep can diff `stream_items`
against each producer's rows via their `api/` packages.

**Backfill of pre-existing journal entries:** the `000N_journal_stream_items`
migration itself seeds rows for **all existing `journal_entries`**
(`INSERT … SELECT`, same module, using the P0.1(a) journal projection values) —
SPEC-05 ships before this spec, and entries
written in between fired their events with no consumer; without this, they'd
vanish from home the day the stream replaces the interim list.

**Acceptance criteria.**
- Given an event redelivered by Asynq retry, then no duplicate stream item exists.
- Given a journal entry edited (body or `occurred_at`) after projection, then the
  stream renders the edited body at the edited position.
- Given a journal entry or media asset deleted, then its stream item(s) are gone —
  including a `playback_completed` item for a deleted asset.
- Given one transfer (two `bank:transaction_created` legs), then exactly one
  stream item exists.
- Given SPEC-08's 3-day and day-of birthday events for the same person, and again
  the following year, then each becomes its own stream item (four items).
- Given a `media:asset_ready` with `origin='import'`, then no stream item.
- Given an event type the consumer doesn't recognize (future producer), then it
  is skipped with a log line, never an error loop.
- Given journal entries created before this spec landed, then they appear in
  `/stream` (migration backfill).

### P0.2 — Stream read API

`GET /api/v1/stream?cursor=&limit=` — permission `stream:read:own` (seeded +
granted to the base `user` role in this module's migration). Merged timeline
ordered `occurred_at DESC, id DESC` (same key as SPEC-05's list),
cursor-paginated. `?limit=` defaults to 30, hard max 50 (clamped above; aligns
with the 50-item LCP budget in §8).
Response items are discriminated by `source_module`: journal items render
**full** (`body_md`, `mood`, asset thumbs — joined from `journal_entries`),
system items render **compact** (`{event_type, title, href, occurred_at}` +
selected payload fields).

**`title`/`href` synthesis (2026-07-10 — previously hand-waved):** registered
event payloads do **not** uniformly carry `title`, and none carries `href`
(producers don't own frontend routes). The stream service owns a small
**per-event-type render mapping** — `event_type → (title template, href
builder)`, e.g. `media:asset_ready → ("<title> is ready", /library/media#id)`,
`bank:transaction_created → (amount/direction summary, /bank/transactions)`,
`people:birthday_upcoming → ("<display_name> — birthday in N days", /people/id)`
— applied at read time from the stored payload. This mirrors SPEC-04's
`data.href` philosophy with the mapping consumer-owned; adding an event type
without a mapping renders a generic card, never an error. For `is_transfer`
payloads, normalize on direction — source = `account_id` when
`direction='debit'` else `counterparty_account_id` (SPEC-03 P0.7 adds it to the
payload) — so either collapsed leg renders the identical "moved <amount>
<source>→<dest>" card.

**Acceptance criteria.**
- Given a mix of journal + system items, then one stable merged order with
  correct cursor traversal (no dupes/gaps across pages).
- Given user B's items, then user A's stream never contains them.
- Given stored system items of each mapped event type, then the response carries
  the synthesized title and href per the mapping (e.g. `media:asset_ready` →
  "<title> is ready", `/library/media#id`).
- Given a stored item whose `event_type` has no mapping, then `GET /stream`
  returns 200 with a generic card (no href), never a 5xx.

### P0.3 — Home `/` replacement

RSC shell per D-33; the stream is a client island using TanStack infinite query
(D-32) against `GET /stream`, with the SPEC-05 composer on top — a successful
post is optimistically inserted at its `occurred_at` position in the stream
query. The fixture newsfeed is already
gone (SPEC-05 P0.4); this item swaps the interim journal-only query for `/stream`
and removes any remaining fixture blocks on the route.

**Acceptance criteria.**
- Grep test: zero fixture data anywhere on the home route.
- Given a new journal post, then it appears at its `occurred_at` position (top,
  when now-dated) optimistically and survives an immediate refetch — guaranteed
  because the projection row is written in the create transaction (P0.1(a)).

### P0.4 — Widget rail on real data

Each widget is **independent**: its own query, its own empty state, and
failure-isolated — one failing/absent endpoint never blanks the rail or throws a
toast storm. If a backing module isn't mounted yet (404), the widget renders its
empty/"coming soon" state.

| Widget | Source | Arrives with |
|---|---|---|
| `PersonalInfoWidget` | `GET /auth/me` | live today |
| Activity feed | `GET /me/notifications` | SPEC-04 |
| Finance month card | `GET /bank/dashboard` | SPEC-03 |
| Continue rail | `GET /continue` | SPEC-07 |
| Birthdays (`BirthdayCard`) | `GET /people/upcoming-birthdays` | SPEC-08 |

**Acceptance criteria.**
- Given only account + media wired, then the rail renders without errors or
  fixtures (empty states where backends are absent).
- Given one widget's endpoint returning 500, then the other widgets render
  normally.

### P1 — nice to have

- **P1.5 On-this-day memories**: `GET /api/v1/stream/memories` — **journal
  entries only** (system items are noise as memories; scope crisped 2026-07-10)
  whose `occurred_at` month/day matches today **in the user's timezone** (D-17),
  from prior years, grouped by years-ago; rendered as one `WidgetCard`. Feb-29
  memories surface on Feb-28 in non-leap years (match SPEC-08's rule).
- **P1.6 Backfill task** `journal:backfill_stream` — one-shot, admin/CLI-triggered:
  seeds `stream_items` from existing **media assets'** upload dates **via
  `mediaapi` listing** (never raw table reads — boundary rule), so the stream
  isn't empty on day one. Journal entries need no task — the §P0.1 migration
  backfill covers them. Idempotent via the same unique constraint. Seeded rows
  use `source_module='media'`, `event_type='media:asset_ready'`,
  `ref_id=asset_id` — identical to the live consumer's key, so the unique
  constraint dedups against real events. Apply the same import exclusion as the
  live consumer. Since SPEC-01 does not persist `origin` on assets, the
  `mediaapi` listing must expose an import/batch discriminator (coordinate with
  SPEC-01/SPEC-02); until it does, the backfill is restricted to instances with
  no comic imports.

### P2 — future considerations (design for, don't build)

- **Daily digest** — promotes SPEC-04's P2 seam with a concrete consumer: a 7am
  rollup of yesterday's stream into one `digest.daily` notification (in-app +
  email via SPEC-04 channels). Keep the watermark pattern in mind when shaping
  stream queries.
- **Privacy tiers** (per-item visibility) arrive with household tenancy — keep
  `user_id` scoping clean so a tenant scope can layer on.
- Retention: none — this table **is** the archive (ADR-08). Revisit only if row
  volume ever matters (years away at n=1).

## 6. Data model — migration `000N_journal_stream_items`

```sql
CREATE TABLE stream_items (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                  -- identity-anchor exception (SPEC-04 §6 precedent)
  source_module text NOT NULL,          -- 'journal' | 'media' | 'bank' | 'comic' | 'people' | ...
  event_type    text NOT NULL,          -- registry name, e.g. 'media:asset_ready'
  ref_id        uuid NOT NULL,          -- per-event ref (P0.1 table): entry, asset,
                                        -- transaction OR transfer, chapter, birthday-notice
  payload       jsonb NOT NULL DEFAULT '{}',  -- render-minimum snapshot (title, href, amount?)
  occurred_at   timestamptz NOT NULL,
  UNIQUE (source_module, event_type, ref_id)
);
CREATE INDEX ON stream_items (user_id, occurred_at DESC, id DESC);
```

No FK on `ref_id` (polymorphic and mostly cross-module). Queries in
`query/journal_stream.sql`; regenerate via `make sqlc`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/stream?cursor=` | `stream:read:own` | merged timeline |
| GET | `/api/v1/stream/memories` | `stream:read:own` | P1.5; user-TZ month/day match |

Problem types: `stream/invalid-cursor`. Consumer registrations (the table in
P0.1) update the **Consumers** column in
[events.md](../../reference/events.md) — definition of done.

## 8. Success metrics (n=1 honest)

- Leading: home LCP < 2.5 s with a 50-item stream (frontend.md §8 budget applies).
- Leading: the owner opens `/` on ≥ 20 of the first 30 days after landing (the
  habit ADR-08 predicts integration creates).
- Lagging: zero fixture data on the home route (grep); a `media:asset_ready`
  fired while the stream consumer was down appears after worker recovery (Asynq
  durability, no lost items).

## 9. Timeline & phasing

1. Projection table + consumers + idempotency tests (1.5 days)
2. `GET /stream` merged read + OpenAPI (1 day)
3. Home replacement (stream island + composer integration) (1.5 days)
4. Widget rail wiring + empty states (1 day)
5. P1 (memories + backfill) (1.5 days)
P0 ≈ 5 dev-days; P1 adds ~1.5. Matches the brief's ~6.

## 10. Open questions

- **(resolved)** Consume events directly vs via SPEC-04's dispatch: the stream
  is a **peer consumer via the `platform/events` fan-out** (events.md "Delivery
  mechanics"), not a notification channel — and never a raw task-type handler,
  since notify consumes several of the same events and Asynq allows one handler
  per task type.
- **(resolved)** The stream replaces the newsfeed on `/` (§1).
- **(product, non-blocking)** Do `bank:*` amounts render in the stream? Default
  **show** (n=1, own data) — the consumer decides per events.md's privacy note;
  revisit at household tenancy alongside SPEC-03 §11.
- **(engineering, non-blocking)** Should system cards deep-fetch via owning
  modules' `api/` when payload isn't enough? Start payload-only; add per-type
  fetchers only when a card demonstrably needs more.
