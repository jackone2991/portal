# SPEC-06 — Life-Stream Home (read path: projection + dashboard)

**Status:** current, rev 1 · **Drafted:** 2026-07-10 · **Last verified:** 2026-10-01
**Module:** `journal` (extends SPEC-05; owns `stream_items` per its §6 decision) + frontend home · **Depends on:** SPEC-05 (hard — first content + module home); system events attach as their producers land (SPEC-01 P0.3, SPEC-03 P0.7, SPEC-07 P1.5, SPEC-08 P0.4); the widget rail additionally consumes SPEC-04's GET /me/notifications — **every widget and consumer degrades to an empty state**, none is a blocker
**Upstream:** brief 06, which merged three candidates from the 2026-07-10 research pass — stream projection, "Today" dashboard, Home-Assistant-pattern home (folded into this spec, then deleted — `git show 2cdda7e:docs/product/briefs/06-life-stream-home.md`) · **Refs:** [ADR-08](../../adr/08-life-os-pivot.md), [events.md](../../reference/events.md), frontend.md
**Downstream consumers:** this spec's own P2 daily digest (reads this projection; delivered via SPEC-04 channels), future on-this-day widgets

---

## 1. Problem statement

The life-OS brief (ratified as ADR-08) deferred "does the life stream replace
the newsfeed on `/` or live alongside it?" until two event producers existed —
SPEC-05 plus SPEC-01 P1.2 gets
there. Today `/` is fixture data end to end: fake posts, fake widgets, fake
activity. ADR-08's thesis is that **integration beats one-app-per-domain**, and
the only place that thesis is visible is a home screen where the facets meet. If
the first screen stays fake, no individual module creates the daily habit.

There is also a durability asymmetry: bus events not captured now are lost — the
projection must exist **before** the producer modules land, not after, or the
stream starts with holes.

**Decision this spec carries (resolves the life-OS brief's open question):** the life
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
- *(No longer a non-goal: the weather widget.)* It was listed here as "dropped,
  not wired" (closing the 2026-07 backlog §3 P2 question — archived;
  `git show 8d382d2^:docs/product/backlog.md`), but the shipped
  `WeatherWidget` was accepted into scope by the owner on 2026-10-01 and is
  specified in P0.4.
- Editing/deleting *system* stream items — they are projections of facts; the fix
  for a wrong fact is in the owning module.

## 4. User stories

- As the owner, I open `/` and see today: what I wrote, what I finished
  watching, what I spent — one timeline, newest first. *(the ADR-08 proof)*
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

| Event | Consumer task | Arrives with | `user_id` from | `ref_id` used | Handler action |
|---|---|---|---|---|---|
| `media:playback_completed` | `journal:stream_playback_completed` | SPEC-07 P1.5 | `user_id` | `asset_id` | insert |
| `media:asset_deleted` | `journal:stream_asset_deleted` | SPEC-01 P0.3 | `owner_user_id` | `asset_id` | **delete ALL** `source_module='media'` rows with this `ref_id`, any event_type and **all users** (the asset is gone for everyone) — else a "watched X" card dangles |
| `bank:transaction_created` | `journal:stream_bank_created` | SPEC-03 P0.7 | `user_id` | **`transfer_id` when `is_transfer`, else `transaction_id`** | insert — both transfer legs share `transfer_id`, so the unique key collapses them into ONE "moved X" item (SPEC-03 P0.7's intent; two items per transfer was a bug) |
| `bank:transaction_updated` | `journal:stream_bank_updated` | SPEC-03 P0.7 | `user_id` | same rule | upsert: `INSERT … ON CONFLICT (user_id, source_module, event_type, ref_id) DO UPDATE SET payload, occurred_at` (`occurred_at` per the rule below) — writing under the created-event key when absent, so a reordered created retry then hits `DO NOTHING` and the latest-processed correction wins |
| `bank:transaction_deleted` | `journal:stream_bank_deleted` | SPEC-03 P0.7 | `user_id` | same rule | delete the matching row |
| `people:birthday_upcoming` | `journal:stream_birthday` | SPEC-08 P0.4 | `user_id` | **`notice_id`** (in the payload — SPEC-08 §6) | insert — keying on `person_id` would make the unique constraint swallow the day-of event and every later year (the original critical bug) |
| `people:birthday_notice_revoked` | `journal:stream_birthday_revoked` | SPEC-08 P0.2 | `user_id` | `notice_id` | delete the matching `people` row — an edited birthday leaves no stale card |
| `people:person_deleted` | `journal:stream_person_deleted` | SPEC-08 §7 DELETE | `user_id` | `person_id` | delete **all** `source_module='people'` rows whose payload `person_id` matches — no card links to a deleted person's 404 |

The `cmd/worker` subscription table (specs README, per-binary event
subscription) maps each event to exactly its task above. The
`journal:stream_ingest` name in `platform/events`' own test is an illustrative
fixture, not a registered task; these names are the real ones.
The last two tasks are specced, not shipped.

The stream projects **moments**, not library events: `media:asset_ready`,
`comic:published` and the catalogue publishes (`movie:published`,
`music:track_published`, `story:published`) go to the bell (SPEC-04) only —
their stream projections were removed in `0033`/`0034`/`0040`
([events.md](../../reference/events.md)). The two `people:*` retraction rows
are specced, not shipped: SPEC-08 emits neither event on HEAD (code follow-up,
§11 row 2).

Inserts use `ON CONFLICT (user_id, source_module, event_type, ref_id) DO
NOTHING` (the §6 unique key; every `ON CONFLICT` in this spec uses the same
target) — idempotency under redelivery is structural. `payload` stores the
event's raw registered payload (render-minimum per events.md). `user_id` is
taken from the payload's user field (column above); a payload missing it, or
carrying an unparseable id, is dropped with a log line and never retried (the
handler returns without error, or with `asynq.SkipRetry`).

`occurred_at` per event:
- **Date-only payloads** (bank `occurred_at` is a `date`, SPEC-03 §6): convert
  explicitly in the owner's timezone — the owner's `users.timezone`, read
  through `accountapi` (specs README Timezone, D-17; unknown →
  `Asia/Ho_Chi_Minh`). Use the ingest
  instant if the date equals the owner's local today at ingest, else 12:00
  local on that date. Never rely on the implicit date→timestamptz cast (the
  session TZ is UTC, so a bare date lands at 07:00 ICT and on the wrong local
  day at day boundaries).
- **`bank:transaction_updated`** keeps the stored `occurred_at` when the payload
  date is unchanged — `DO UPDATE SET payload = EXCLUDED.payload, occurred_at =
  CASE WHEN (stream_items.occurred_at AT TIME ZONE <tz>)::date = <payload date>
  THEN stream_items.occurred_at ELSE EXCLUDED.occurred_at END` — so a metadata
  edit (re-categorize, note) never moves the item; it is re-derived only when
  the date itself changed.
- **`media:playback_completed`**: the payload's `completed_at` (the latched
  completion instant, SPEC-07 P1.5), so an event delivered late after worker
  downtime still sorts at its real position.
- **Other events**: the payload timestamp if present, else ingest time.
  `people:birthday_upcoming` carries none and keeps the ingest-time fallback
  (accepted residual below).

*(Code follow-up: on HEAD `journal/stream.go` `bankRef` parses the date as UTC
midnight, the upsert overwrites `occurred_at` unconditionally, the
`playback_completed` handler uses `time.Now()` because the payload has no
`completed_at`, and malformed payloads are dropped without a log line — §11
rows 3–5.)*

**Known residual risk (accepted at v1, documented):** a `*_created` retry
processed *after* the corresponding `*_deleted` resurrects an item —
`ON CONFLICT` can't prevent re-insert once the row is gone. Rare,
self-correcting on the next delete, and a P2 reconcile sweep is the seam.
Second accepted residual: producers other than SPEC-08 emit post-commit without
an outbox, so a producer crash between commit and `Publish` drops that item
permanently. Accepted at v1; a P2 reconcile sweep can diff `stream_items`
against each producer's rows via their `api/` packages.
Third accepted residual: two `bank:transaction_updated` events processed out of
order leave the earlier edit's payload (last-processed-wins; the payload
carries no version). This self-corrects on the next edit or the P2 reconcile
sweep. If it ever bites, add `updated_at` to the SPEC-03 P0.7 payload and guard
the upsert with `WHERE (stream_items.payload->>'updated_at')::timestamptz <=
(EXCLUDED.payload->>'updated_at')::timestamptz`. (A transfer flag never flips
on an existing leg — legs are immutable per SPEC-03 P0.3 — so the ref key
cannot change under an update.)
Fourth accepted residual: a birthday notice delivered late (SPEC-08 outbox
retry on the next scan) sorts at delivery time — its payload has no timestamp,
and it is a heads-up, not a timed fact.

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
- Given a `bank:transaction_updated` with a new amount (even if delivered before
  its created event), then the single stream item shows the new amount.
- Given a `bank:transaction_deleted` (or a transfer delete emitting both legs),
  then the item is gone.
- Given a transaction dated today and created at 21:00 local, then it sorts
  above a journal entry written at 08:00 local that day.
- Given that transaction later re-categorized, then its stream position does
  not change.
- Given a transaction dated 2026-07-01 for an owner whose `users.timezone` is
  `Asia/Ho_Chi_Minh` (UTC+7), then its item falls on 2026-07-01 local, not
  2026-06-30; given the same date for an owner in `America/New_York`, then it
  falls on 2026-07-01 in New York.
- Given two users completing playback of the same shared asset, then each has
  their own `playback_completed` item.
- Given a `playback_completed` event delivered after worker downtime, then its
  item sits at the payload's `completed_at`, not at the recovery time.
- Given SPEC-08's 3-day and day-of birthday events for the same person, and again
  the following year, then each becomes its own stream item (four items).
- Given a `media:asset_ready` (any origin), then no stream item — it reaches the
  bell only.
- Given a person deleted, or a birthday edited so an emitted notice is revoked,
  then that person's / notice's birthday stream items are gone.
- Given an event type the consumer doesn't recognize (future producer), then it
  is skipped with a log line, never an error loop.
- Given journal entries created before this spec landed, then they appear in
  `/stream` (migration backfill).

### P0.2 — Stream read API

`GET /api/v1/stream?cursor=&limit=` — permission `stream:read:own` (seeded +
granted to the base `user` role in this module's migration). Merged timeline
ordered `occurred_at DESC, id DESC` (same key as SPEC-05's list),
cursor-paginated (specs README Pagination convention). `?limit=` defaults to
30, hard max 50 (values above are clamped to 50; aligns with the 50-item LCP
budget in §8); a missing, non-integer or < 1 value falls back to 30 — lenient,
no Problem type. *(Code follow-up: on HEAD a value above 50 falls back to 30
instead of clamping — §11 row 8.)*

Response `200 {items: StreamItem[], next_cursor?: string}` (`next_cursor`
absent on the last page). Every `StreamItem` carries `{id (stream_items.id),
source_module, event_type, ref_id, occurred_at}` and is discriminated by
`source_module`:
- `journal` items render **full**: `body_md`, `mood`, `asset_ids`, `location`
  — joined from `journal_entries`, in the Entry's own field shapes (SPEC-05
  OpenAPI `JournalEntry`; SPEC-12), never a parallel shape.
- every other module's items render **compact**: `title` and `href` (absent
  when the mapping has none) synthesized per the render mapping below; the raw
  payload is not returned.

**`title`/`href` synthesis (2026-07-10 — previously hand-waved):** registered
event payloads do **not** uniformly carry `title`, and none carries `href`
(producers don't own frontend routes). The stream service owns a small
**per-event-type render mapping** — `event_type → (title template, href
builder)`, e.g. `media:playback_completed → ("Finished watching <title>",
/library/media/{asset_id})` (media hrefs follow SPEC-07 P0.4's **media
deep-link rule**), `bank:transaction_created → (amount/direction summary,
/bank/transactions)`,
`people:birthday_upcoming → ("<display_name> — birthday in N days", /people/{person_id})`
— applied at read time from the stored payload. This mirrors SPEC-04's
`data.href` philosophy with the mapping consumer-owned; adding an event type
without a mapping renders a generic card, never an error. For `is_transfer`
payloads, normalize on direction — source = `account_id` when
`direction='debit'` else `counterparty_account_id` (SPEC-03 P0.7 adds it to the
payload) — so either collapsed leg renders the identical "moved <amount>
<source>→<dest>" card.

**Bank cards are the first per-type fetcher (§10).** At read time the stream
service batch-resolves the page's account and category ids via one
`bankapi.Names(ctx, userID, accountIDs, categoryIDs)` call per page (the
payload holds ids only, and `category_id` goes stale after a reassign-delete
that emits no event). Unresolvable ids render "(deleted account)" /
"Uncategorized". Amounts are formatted with the payload `currency`'s exponent
(VND = 0; SPEC-03 P0.7 adds `currency`), never a hard-coded ₫. *(Code
follow-up: on HEAD `bankapi.Names` does not exist, the payload has no
`currency`, and `journal/stream.go` formats every amount as VND — §11 row 6.)*

**Optimistic journal item** (P0.3). It is built from the composer draft, before
the `POST /journal/entries` response, in the StreamItem shape `{id: <temp client
id>, source_module: "journal", event_type: "journal:entry_created", ref_id:
<temp client id>, occurred_at, body_md, mood, asset_ids, location}`; when the
response arrives its `ref_id` becomes `entry.id`. On refetch the server item
(real `id`) replaces it by `ref_id`. Ties at equal `occurred_at` are placed
first and are corrected by the refetch.

**Acceptance criteria.**
- Given a mix of journal + system items, then one stable merged order with
  correct cursor traversal (no dupes/gaps across pages).
- Given user B's items, then user A's stream never contains them.
- Given stored system items of each mapped event type, then the response carries
  the synthesized title and href per the mapping (e.g. `media:playback_completed`
  → "Finished watching <title>", `/library/media/{asset_id}`).
- Given a USD account transfer, then the card shows USD, never ₫.
- Given a stored item whose `event_type` has no mapping, then `GET /stream`
  returns 200 with a generic card (no href), never a 5xx.

### P0.3 — Home `/` replacement

RSC shell per D-33 (as defined in the specs README Frontend convention); the stream is a client island using TanStack infinite query
(D-32) against `GET /stream`, with the SPEC-05 composer on top — a successful
post is optimistically inserted at its `occurred_at` position in the stream
query (**optimistic placement** below). The fixture newsfeed is already
gone (SPEC-05 P0.4); this item swaps the interim journal-only query for `/stream`
and removes any remaining fixture blocks on the route.

**Optimistic placement** (same rule as SPEC-05 P0.4): insert into the loaded
page whose range contains `occurred_at`. If it is older than the last loaded
item and `hasNextPage`, do not insert; show a "Saved to <date>" toast. Dedupe by
`ref_id` (= entry id) against fetched pages. The same rule applies to edits
that change `occurred_at`. On error, restore body, mood and `occurred_at`. The
optimistic item's shape is defined in P0.2.

**Day grouping.** Card dates, any day separators ("Today", "Yesterday", a
date) and the "Saved to <date>" toast are computed in the user's
`users.timezone` from `GET /auth/me` (specs README Timezone) — the same zone
the server uses for date-only payloads (P0.2) and on-this-day (P1.5), and the
zone SPEC-05's composer picks in — never the browser's zone. *(Code follow-up:
the home view takes its zone from `lib/time.ts`, i.e. `GET /api/v1/time` /
`APP_TIMEZONE` — README Timezone follow-up; §11 row 9.)*

**Acceptance criteria.**
- Grep test: zero fixture data anywhere on the home route.
- Given a new journal post, then it appears at its `occurred_at` position (top,
  when now-dated) optimistically and survives an immediate refetch — guaranteed
  because the projection row is written in the create transaction (P0.1(a)).
- Given a post backdated older than the last loaded item while more pages
  exist, then it is not inserted, a "Saved to <date>" toast shows, and it
  appears exactly once when its page loads.

### P0.4 — Widget rail on real data

Each widget is **independent**: its own query, its own empty state, and
failure-isolated — one failing/absent endpoint never blanks the rail or throws a
toast storm. If a backing module isn't mounted yet (404), the widget renders its
empty/"coming soon" state. Each widget query sets `retry: false`: a 4xx is
final, so a 404/403 renders the empty or "coming soon" state immediately
(TanStack's default three retries would spin for seconds first), and a 5xx
renders an inline retry affordance. *(Code follow-up: on HEAD `BirthdayCard`
uses the `QueryClient` defaults — the other rail widgets already set
`retry: false` — and no widget offers an inline retry on a 5xx — §11 row 12.)*

The Activity feed reuses SPEC-04 P0.5's `["notifications"]` query (same key and
queryFn, rendering a slice) and never defines its own key (D-32), so the bell's
optimistic mark-read reaches the rail. *(Code follow-up: on HEAD
`ActivityFeed.tsx` uses `["notifications", "rail"]` — §11 row 11.)*

| Widget | Source | Arrives with |
|---|---|---|
| `PersonalInfoWidget` | `GET /auth/me` | live today |
| Activity feed | `GET /me/notifications` | SPEC-04 |
| Finance month card | `GET /bank/dashboard` | SPEC-03 |
| Continue rail | `GET /continue` | SPEC-07 |
| Birthdays (`BirthdayCard`) | `GET /people/upcoming-birthdays` | SPEC-08 |
| Weather (`WeatherWidget`) | Open-Meteo forecast API, called from the browser (no Portal endpoint) | live today (accepted 2026-10-01; below) |

`PersonalInfoWidget`'s `DEFAULT_ITEMS` sample data is deleted and `items` made
required, built from `GET /auth/me`; otherwise §2 goal 4's grep test fails.
`BirthdayCard` is reworked, not merely wired (SPEC-08 P0.5).

**Weather widget** *(accepted into scope by the owner on 2026-10-01; shipped)*.
`WeatherWidget` (`templates/v1/components/widget/WeatherWidget.tsx`, registry key
`weather`) is seeded on the **left** rail at position 40 by `0036_layout_core`, with
no permission, so every user gets it; the layout editor moves or removes it like
any other widget. It has no Portal backend: the browser calls **Open-Meteo**'s
keyless, CORS-open forecast API directly (`lib/weather.ts` `fetchWeather`,
`https://api.open-meteo.com/v1/forecast`: current temperature, apparent
temperature, humidity, precipitation probability, WMO weather code and wind; 24
hourly points; seven daily min/max and precipitation maxima; metric °C;
`timezone=auto`). Nothing about the location reaches the Portal API.

- **Location source**, in order: the build-time `NEXT_PUBLIC_WEATHER_LAT` /
  `NEXT_PUBLIC_WEATHER_LON` (with an optional `NEXT_PUBLIC_WEATHER_PLACE` label)
  when both parse and are not 0,0 — no geolocation prompt then; otherwise the
  browser's Geolocation API (`getCurrentPosition`, 8 s timeout, a cached position
  up to 30 min old accepted). Coordinates go out rounded to three decimals.
- **Refresh:** one fetch each time the widget mounts; no polling, no TanStack
  query, no retry.
- **States:** while locating or fetching, a compact card "Thời tiết ·
  Đang định vị…"; when geolocation is unsupported or denied, or the fetch fails,
  the same compact card asking to enable location ("Bật định vị để xem thời
  tiết") — never a toast, never fabricated or fixture weather; on success, the
  place label (or "Vị trí của bạn"), the current temperature, today's high/low,
  the WMO label and icon, "feels like" and rain chance, and a seven-day strip
  starting "Nay". The card links to the `/weather` page (`views.weather`:
  current, hourly and seven-day).
- **Days** are the forecast location's local days (Open-Meteo
  `timezone=auto`) — a deliberate exception to the specs README Timezone
  convention: they describe that place's weather, not the user's day.
- Failure isolation holds as above; the `retry: false` / inline-retry rule does
  not apply because the widget issues no Portal query. *(Code follow-up:
  `/weather` is missing from the `middleware.ts` matcher — §11 row 14.)*

**Acceptance criteria (weather).**
- Given `NEXT_PUBLIC_WEATHER_LAT/LON` set, then the widget fetches for those
  coordinates with no geolocation prompt and shows the configured place label.
- Given no configured location and geolocation denied, then the compact
  "enable location" card renders, no weather values are shown, and the rest of
  the rail renders normally.
- Given Open-Meteo unreachable or answering 5xx, then the same compact card, no
  toast, other widgets unaffected.
- Given a successful fetch, then current temperature, high/low, label,
  feels-like and rain chance, and seven daily entries starting "Nay"; activating
  the card opens `/weather`.

**Acceptance criteria.**
- Given only account + media wired, then the rail renders without errors or
  fixtures (empty states where backends are absent).
- Given one widget's endpoint returning 500, then the other widgets render
  normally.
- Given a widget's endpoint returning 404 or 403, then that widget shows its
  empty/"coming soon" state at once, with no retries.
- Given a notification marked read from the bell, then the Activity feed shows
  it read without a refetch.

### P1 — nice to have

- **P1.5 On-this-day memories**: `GET /api/v1/stream/memories` — **journal
  entries only** (system items are noise as memories; scope crisped 2026-07-10)
  whose `occurred_at` month/day, taken in the caller's `users.timezone`,
  matches today in that same zone (specs README Timezone — the same source as
  SPEC-08 P0.3 and the stream's day grouping),
  from prior years, grouped by years-ago; rendered as one `WidgetCard`. Feb-29
  memories surface on Feb-28 in non-leap years (match SPEC-08's rule).
  Response `200 {items: [{years_ago: int, entries: JournalEntry[]}]}` (a
  non-paginated list, specs README Pagination), groups ordered `years_ago`
  ASC; an empty `items` means none. *Acceptance:* given
  entries on today's month/day one and three years ago plus a system item on
  the same day, then two groups (1, 3) holding only the journal entries; given
  none, then `200 {items: []}`.
- **P1.6 Backfill task — retired.** It seeded `media:asset_ready` rows, which
  the stream no longer projects (`0033`; P0.1(b) note). Journal entries need no
  task — the §P0.1 migration backfill covers them.

### P2 — future considerations (design for, don't build)

- **Daily digest** (needs SPEC-04 P0 + this projection) — promotes SPEC-04's P2
  seam with a concrete consumer: a 7am rollup of yesterday's stream into one
  `digest.daily` notification (in-app + email via SPEC-04 channels). Keep the watermark pattern in mind when shaping
  stream queries.
- **Privacy tiers** (per-item visibility) arrive with household tenancy — keep
  `user_id` scoping clean so a tenant scope can layer on.
- Retention: none — this table **is** the archive (ADR-08). Revisit only if row
  volume ever matters (years away at n=1).

## 6. Data model — migration `000N_journal_stream_items`

**Tenancy** (specs README convention, ADR-07). Tenant-scoped: `stream_items` (`tenant_id` + index + `tenant_isolation` policy, added by `0020_platform_rls_enable`); consumers write inside the payload user's tenant scope. The DDL below predates ADR-07 and omits the columns.

```sql
CREATE TABLE stream_items (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                  -- identity-anchor exception (SPEC-04 §6 precedent)
  source_module text NOT NULL,          -- 'journal' | 'media' | 'bank' | 'people' | ...
  event_type    text NOT NULL,          -- registry name, e.g. 'media:playback_completed'
  ref_id        uuid NOT NULL,          -- per-event ref (P0.1 table): entry, asset,
                                        -- transaction OR transfer, birthday-notice
  payload       jsonb NOT NULL DEFAULT '{}',  -- raw registered event payload; title/href
                                        -- synthesized at read time (P0.2); '{}' for journal rows
  occurred_at   timestamptz NOT NULL,
  UNIQUE (user_id, source_module, event_type, ref_id)
                                        -- user_id first: a shared asset's playback is per user
);
CREATE INDEX ON stream_items (user_id, occurred_at DESC, id DESC);
```

The same migration also ships the `stream:read:own` → `user` seed (the `0003`
`WITH grants(...)` pattern) and the P0.1 journal backfill `INSERT … SELECT … ON
CONFLICT DO NOTHING`. *(Code follow-up: on HEAD the unique key and every
`ON CONFLICT` in `query/journal_stream.sql` omit `user_id`, so a second user's
completion of a shared asset would be dropped — §11 row 1.)*

No FK on `ref_id` (polymorphic and mostly cross-module). Queries in
`query/journal_stream.sql`; regenerate via `make sqlc`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/stream?cursor=&limit=` | `stream:read:own` | merged timeline; `limit` default 30, max 50 (above is clamped; missing/invalid → 30); returns `{items, next_cursor?}` (P0.2) |
| GET | `/api/v1/stream/memories` | `stream:read:own` | P1.5; month/day match in the caller's `users.timezone` (specs README Timezone); non-paginated, returns `{items: [{years_ago, entries}]}` (specs README Pagination) |

Problem types: `journal/invalid-cursor` (400; the stream is served by the
`journal` module, so the slug is `journal/…` — shared with SPEC-05's
cursor-paginated `GET /journal/entries`; the shipped handler emits it; the
earlier `stream/invalid-cursor` in `problems.ts` names no emitted type). No
limit Problem: `limit` parsing is lenient (P0.2), and the stream takes no body,
so no `journal/validation` is emitted here. The stream follows the specs README
Pagination convention (`{items, next_cursor}`).
Annotate each operation per the specs README AuthZ **OpenAPI encoding**. Consumer registrations (the table in
P0.1) update the **Consumers** column in
[events.md](../../reference/events.md) — definition of done.

## 8. Success metrics (n=1 honest)

- Leading: home LCP < 2.5 s with a 50-item stream (frontend.md §8 budget applies).
- Leading: the owner opens `/` on ≥ 20 of the first 30 days after landing (the
  habit ADR-08 predicts integration creates).
- Lagging: zero fixture data on the home route (grep); a
  `bank:transaction_created` fired while the stream consumer was down appears
  after worker recovery at its original `occurred_at` position (Asynq
  durability, no lost items; holds for events whose payload carries a
  timestamp — P0.1).

## 9. Timeline & phasing

1. Projection table + consumers + idempotency tests (1.5 days) — lands with
   SPEC-05, before SPEC-02/03/07/08 emit (§1; specs README build order)
2. `GET /stream` merged read + OpenAPI (1 day)
3. Home replacement (stream island + composer integration) (1.5 days)
4. Widget rail wiring + empty states (1 day)
5. P1 (memories) (1 day; the P1.6 backfill is retired)
P0 ≈ 5 dev-days; P1 adds ~1. Matches the brief's ~6.

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
  modules' `api/` when payload isn't enough? **Resolved for `bank:*`** (P0.2:
  `bankapi.Names` batch resolve per page); other types stay payload-only until a
  card demonstrably needs more.

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

Baseline: `main` @ `99b5a0b` (the docs commits on top of it change no code; read
it with `git show 99b5a0b:<path>`). The text above is the target; every row below
is a place where the shipped code still diverges from it, ordered by severity —
data loss and stale personal data first, then ordering and rendering, then the
home page, then contract hygiene and the P1 item. A row closes when the code
matches the requirement **and** the SPEC-06 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded on
test evidence; delete the row in that PR. The inline *Code follow-up* notes above
point here. Paths are relative to `backend/` or `frontend/src/`; `stream.go` is
`internal/modules/journal/stream.go`.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | §6 unique key; P0.1 `ON CONFLICT` target | `UNIQUE (user_id, source_module, event_type, ref_id)`, and every `ON CONFLICT` uses it — a shared asset's playback is one item **per user**. | `db/migrations/0017_journal_stream_items.up.sql`: `UNIQUE (source_module, event_type, ref_id)`. `internal/modules/journal/query/journal_stream.sql`: `InsertStreamItem` / `UpsertStreamItem` use `ON CONFLICT (source_module, event_type, ref_id)`; `UpdateStreamOccurredAt` and `DeleteStreamItem` match without `user_id`. A second user's `media:playback_completed` for the same asset is silently swallowed by `DO NOTHING`. | migration `000N_journal_stream_items_user_key`: drop the old unique, add the new one (no data rewrite — the old key is stricter, so no row collides). backend: every `ON CONFLICT` target gains `user_id`; `DeleteStreamItem` and `UpdateStreamOccurredAt` gain `AND user_id = $n` (`OnBankDeleted` passes the payload user, `repository/adapter.go` `PatchEntry` / `DeleteEntry` the owner); `DeleteStreamByRef` stays user-less (the asset is gone for everyone); regenerate sqlc. test: the `journal_test.go` fake's `streamKey` gains the user; two users completing one asset → two items (TC-STREAM-018, TC-STREAM-112). | F156 |
| 2 | P0.1(b) `people:birthday_notice_revoked`, `people:person_deleted` | Two consumer tasks delete a revoked notice's item and every `people` item whose payload `person_id` matches a deleted person. | `internal/modules/people/api/api.go` declares only `EventBirthdayUpcoming`; `people/service.go` emits neither event. `internal/modules/journal/api/api.go` has no `TaskStreamBirthdayRevoked` / `TaskStreamPersonDeleted`; `module.go` `RegisterTasks` registers six tasks. A deleted person's birthday card stays and links to a 404; an edited birthday leaves a stale card. | backend (SPEC-08): publish both events after commit (payloads per events.md). backend (journal): two task constants; `OnBirthdayRevoked` deletes `('people', 'people:birthday_upcoming', notice_id)` for the user; `OnPersonDeleted` runs a new `DeleteStreamPeopleByPerson` (`DELETE FROM stream_items WHERE user_id = $1 AND source_module = 'people' AND payload->>'person_id' = $2`); both through `runScoped`; register both in `RegisterTasks`; subscribe them in `cmd/api` (the people PATCH/DELETE publish there) and in `cmd/worker`; events.md Consumers column. test: TC-STREAM-005. | F017 |
| 3 | P0.1 `occurred_at` for date-only payloads; `bank:transaction_updated` keeps position | A bank `date` becomes an instant in the owner's `users.timezone` (unknown → `Asia/Ho_Chi_Minh`): the ingest instant when the date is the owner's local today, else 12:00 local. An update moves the item only when the date changed. | `stream.go` `bankRef` uses `time.Parse("2006-01-02", …)` — UTC midnight, i.e. 07:00 ICT and the wrong local day west of UTC. `bankUpsert(update=true)` → `UpsertStreamItem` `DO UPDATE SET payload = EXCLUDED.payload, occurred_at = EXCLUDED.occurred_at` unconditionally, so a re-categorize moves the card. The journal module has no account dependency, and `internal/modules/account/api/api.go` `UserSummary` has no timezone. | backend (account): expose the timezone through accountapi (`UserSummary.Timezone` or a `Timezone(ctx, userID)` call). backend (journal): `Deps.Accounts`, wired in `cmd/worker` and `cmd/api`; `bankRef` converts as specified. query: `UpsertStreamItem` → `DO UPDATE SET payload = EXCLUDED.payload, occurred_at = CASE WHEN (stream_items.occurred_at AT TIME ZONE @tz)::date = @payload_date THEN stream_items.occurred_at ELSE EXCLUDED.occurred_at END`; regenerate sqlc. Prerequisite: the `users.timezone` default and write path (SPEC-05 §11 row 2). test: TC-STREAM-017. | F074; Decision 2026-09-30 (Timezone) |
| 4 | P0.1 `media:playback_completed` sorts at `completed_at` | The item sits at the payload's `completed_at` (the latched completion instant), so a late delivery sorts at its real position. | `internal/modules/media/service.go` `Service.PutProgress` (which latches completion) publishes `{asset_id, user_id, title}` — it computes `completedAt := time.Now()` but does not send it; `stream.go` `OnPlaybackCompleted` passes `time.Now()`. | backend (media): add `completed_at` (RFC 3339, the latched value) to the payload; events.md. backend (journal): parse it and use it; fall back to ingest time only when absent (events already queued). test: TC-STREAM-006 plus a late-delivery case (P0.1 AC). | F152 |
| 5 | P0.1 malformed payload | A payload missing its user field, or carrying an unparseable id, is dropped **with a log line** and never retried. | `stream.go` `insertSystem`, `OnPlaybackCompleted`, `OnBirthdayUpcoming`, the `bankRef` callers and the JSON / asset-id branches of `OnAssetDeleted` all `return nil` silently; only a missing `owner_user_id` on `media:asset_deleted` is logged. | backend: `log.Warn()` with task type and reason on every drop branch, still returning nil (or `asynq.SkipRetry`). test: TC-STREAM-015 — a malformed payload returns no error and writes nothing. | F151 |
| 6 | P0.2 bank cards — names and currency | One `bankapi.Names(ctx, userID, accountIDs, categoryIDs)` per page; "(deleted account)" / "Uncategorized" fallbacks; amounts formatted with the payload `currency`'s exponent; a transfer renders "moved <amount> <source>→<dest>", normalized on direction. | `internal/modules/bank/api/api.go` `TransactionEvent` has no `Currency`, and bankapi has no `Names`. `stream.go` `renderSystem` formats every amount with `formatVND` and titles a transfer just "Moved <amount>" (no accounts, no direction normalization). | backend (bank, SPEC-03 P0.7): `currency` in the payload (events.md); `bankapi.Names` reading inside the caller's tenant scope. backend (journal): a `Bank` dependency; `Service.Stream` collects the page's account and category ids, resolves them once, applies the fallbacks, formats by currency exponent (VND = 0), and normalizes transfers (source = `account_id` when `direction = 'debit'`, else `counterparty_account_id`). test: TC-STREAM-033, TC-STREAM-034 incl. "a USD transfer shows USD, never ₫". | F016 |
| 7 | P0.2 media deep link | `media:playback_completed` → `/library/media/{asset_id}` (SPEC-07 P0.4 media deep-link rule; playback is video or audio, both open the player). | `stream.go` `renderSystem` returns href `/library/media` (the grid, no id). | backend: href `/library/media/` + `asset_id`. test: extend `journal_test.go: TestStreamReadMapping` with a playback item (TC-STREAM-033). | F018; Decision 2026-09-30 (Audio) |
| 8 | P0.2 / §7 `limit` | Default 30; above 50 is **clamped to 50**; missing, non-integer or < 1 → 30. | `stream.go` `Service.Stream`: `if limit <= 0 \|\| limit > maxStreamLimit { limit = defaultStreamLimit }` — `?limit=100` returns 30. | backend: `> 50 → 50`, `≤ 0 → 30` (non-integers already arrive as 0 via `server.AtoiSafe`), e.g. `platform/server.Limit(r, 30, 50)` in `handler.go`; openapi: describe `limit` as defaulted and clamped. test: TC-STREAM-036 over the router (`?limit=100` → at most 50 items; `?limit=abc` → 30). | F075; Decision 2026-10-01 (limit) |
| 9 | P0.3 day grouping in the user's zone | Card dates, day separators and the "Saved to <date>" toast use `users.timezone` from `GET /auth/me`, never the browser's or the instance's zone. | `templates/v1/components/stream/StreamItemCard.tsx` formats with `formatDate(item.occurred_at, tc?.timezone ?? "UTC")`, where `tc` comes from `lib/time.ts` `useTimeConfig` → `GET /api/v1/time` (`APP_TIMEZONE`). | frontend: take the zone from `/auth/me` (prerequisite SPEC-05 §11 row 2: `/auth/me` returns `timezone`). test: TC-STREAM-053. | Decision 2026-09-30 (Timezone) |
| 10 | P0.3 optimistic placement; P0.2 optimistic item | Insert into the loaded page whose range holds `occurred_at`; too old with more pages → no insert, "Saved to <date>" toast; dedupe by `ref_id`; same rule for `occurred_at` edits; the optimistic item swaps its temp `ref_id` for `entry.id` when the POST answers. | `templates/v1/views/home/HomeView.tsx` `create.onMutate` always `prepend`s into page 0; the item keeps `ref_id` = temp id until the refetch replaces it; no range check, toast or dedupe; edits never re-sort. | frontend: the `placeOptimistic` helper and `ref_id` swap of SPEC-05 §11 row 4 (one change closes both). test: TC-STREAM-051, TC-JRNL-050…052. | F015 |
| 11 | P0.4 Activity feed shares the bell's query | The rail's Activity feed reuses SPEC-04 P0.5's `["notifications"]` query (same key and queryFn), so the bell's optimistic mark-read reaches it. | `templates/v1/components/widget/ActivityFeed.tsx` queries `["notifications", "rail"]`; the bell (`templates/v1/components/headers/NotifMenus.tsx`) owns `NOTIFICATIONS_KEY = ["notifications"]`. | frontend: move the bell's query into a shared hook (e.g. `lib/notifications.ts` `useNotifications()`) and have the feed render a slice of it. test: TC-STREAM-072 + "marked read from the bell → feed shows it read without a refetch". | F160 |
| 12 | P0.4 widget failure handling | Each widget query sets `retry: false` (a 4xx is final → empty / "coming soon" at once); a 5xx renders an inline retry affordance. | All rail widgets set `retry: false` except `templates/v1/components/widget/BirthdayCard.tsx` (`useQuery({queryKey: ["people", "upcoming"], …})` → three retries). No widget renders a retry on a 5xx: `MusicWidget` returns null on error, the others render their empty state. | frontend: `retry: false` on `BirthdayCard`; a shared error state in `WidgetCard` with a Retry button for 5xx. test: TC-STREAM-070, TC-STREAM-071. | F159 (corrected — the worklog said every rail query used the defaults) |
| 13 | P0.4 `PersonalInfoWidget` on real data | `DEFAULT_ITEMS` deleted, `items` required and built from `GET /auth/me`; the widget is on the rail ("live today"). | `templates/v1/components/widget/PersonalInfoWidget.tsx` still declares `DEFAULT_ITEMS` sample data as the `items` default; the widget is absent from `widget/registry.ts` and from the `0036_layout_core` seed, so it is not on the home rail at all. | frontend: delete `DEFAULT_ITEMS`, make the widget self-fetch `/auth/me`, add a `personal-info` key to the registry. migration: seed the `personal-info` layout row (the `0036` pattern). test: TC-STREAM-050 (grep), TC-STREAM-072. | F029 |
| 14 | P0.4 weather widget — `/weather` behind the D-34 gate (specs README Frontend convention) | The widget links to `/weather`, a page under `app/(app)/`; every `(app)` route is in `config.matcher` of `frontend/src/middleware.ts`, so an unauthenticated visitor is redirected to `/login` at the edge. | `middleware.ts` `config.matcher` is `["/", "/login", "/register", "/upload", "/library/:path*", "/bank/:path*", "/people/:path*"]` — no `/weather` (nor `/admin` and `/calendar`, the rest of F032); `app/(app)/weather/page.tsx` renders `views.weather` without the edge gate. | frontend: add `'/weather'` to the matcher (with the other F032 routes). test: TC-STREAM-076. | F032; Decision 2026-10-01 (weather accepted) |
| 15 | §7 Problem types and OpenAPI encoding | `journal/invalid-cursor` has an i18n key; each operation carries `x-required-permission`; the contract describes only projected sources. | `lib/problems.ts` lacks `journal/invalid-cursor` and keeps the never-emitted `stream/invalid-cursor`. `shared/openapi.yaml` `getStream` declares no 400 and no `x-required-permission`; `StreamItem.source_module` lists `comic` and `ref_id` mentions "chapter id", neither projected since `0034`. | frontend: add `journal/invalid-cursor`, delete `stream/invalid-cursor`. openapi: `x-required-permission: stream:read:own`, a 400 `journal/invalid-cursor` response, the stale `comic` / chapter wording removed; regenerate and commit (ADR-10). test: TC-STREAM-037. | F158, F024, F025, F010 |
| 16 | P1.5 on-this-day memories | `GET /api/v1/stream/memories` → `200 {items: [{years_ago, entries}]}`: journal entries whose `occurred_at` month/day, taken in the caller's `users.timezone`, matches today in that zone, prior years, `years_ago` ASC; Feb-29 surfaces on Feb-28 in non-leap years. | Not shipped: no route in `module.go` `MountHTTP`, no query, no `/stream/memories` in `shared/openapi.yaml`, no widget. | backend: a `ListMemories` query in `query/journal_entries.sql` (`(occurred_at AT TIME ZONE @tz)` month/day match, earlier years, the Feb-28 rule), handler under `stream:read:own`, the zone from accountapi (row 3). openapi: the path and response. frontend: one `WidgetCard` on the rail. test: TC-STREAM-090, TC-STREAM-091. | F075; Decision 2026-09-30 (Timezone) |

**Already matching on `99b5a0b`** (verified, not to be re-built):
- P0.1(a) — journal rows are written, moved and deleted inside the entry's own transaction (`repository/adapter.go` `CreateEntry` / `PatchEntry` / `DeleteEntry`) with `source_module='journal'`, `event_type='journal:entry_created'`, `ref_id` = entry id, `payload='{}'`; `0017` backfills existing entries with the same values and seeds `stream:read:own` to `user`.
- P0.1(b) — the six shipped consumers use their own task names (`journal/api/api.go` `TaskStream*`), subscribed in `cmd/worker` (and the five media/bank ones in `cmd/api`), never the raw event name; each writes inside the payload user's tenant scope (`runScoped`).
- P0.1(b) — `media:asset_ready`, `comic:published` and the catalogue publishes are not projected (`0033` / `0034` / `0040`; `renderSystem` has no `asset_ready` case); a transfer collapses on `transfer_id`; `bank:transaction_updated` upserts under the created-event key; `bank:transaction_deleted` deletes it; birthdays key on `notice_id`; `media:asset_deleted` deletes every `media` row for the ref across event types and users.
- P0.2 — `GET /stream` returns `{items, next_cursor?}` with flat items carrying `id`, `source_module`, `event_type`, `ref_id`, `occurred_at`; journal items join `body_md`, `mood`, `asset_ids`, `location` from `journal_entries` in the Entry's shapes; system items carry `title` / `href` and never the raw payload; an unmapped type renders a generic card without `href`; the bank and people hrefs follow the mapping; a malformed cursor is 400 `journal/invalid-cursor`; a missing or invalid `limit` is 30.
- P0.3 / P0.4 — the home route has no fixtures; the rail is composed from `GET /layout`, each widget has its own query, and an unknown key is skipped.
- P0.4 weather — `WeatherWidget` and `lib/weather.ts` match the weather requirement: Open-Meteo from the browser, configured location before geolocation, one fetch per mount, the compact card while loading and on denial or failure, no fixture data, a link to `/weather`; `0036_layout_core` seeds it on the left rail with no permission.

**Test evidence to add or fix:**
- `journal_test.go` fake repository: `streamKey{src, evt, ref}` mirrors the superseded unique key (no user), so a per-user test cannot fail against it — add the user with row 1.
- TC-STREAM-010 ("item payload + occurred_at updated") predates the rule that an update moves the item only when the date changed — align it with TC-STREAM-017.
- `journal_test.go: TestStreamReadMapping` covers only `people:birthday_upcoming`; extend it to `media:playback_completed` (href with the asset id) and the bank cards (TC-STREAM-033, TC-STREAM-034).
- Behaviour that already holds but has no test: TC-STREAM-016 (migration backfill), TC-STREAM-030 (merged cursor traversal over the router), TC-STREAM-031 (owner isolation on `/stream`), TC-STREAM-035 (unmapped type → generic card).
- New with the rows above: TC-STREAM-005, 006, 015, 017, 018, 036, 037, 051, 053, 070…072, 090, 091, 112.
- The weather widget (accepted 2026-10-01) has no test: TC-STREAM-073…075 (configured location, denied/failed fetch, success render) and TC-STREAM-076 (row 14's matcher).
