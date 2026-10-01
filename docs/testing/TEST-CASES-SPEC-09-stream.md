# Test Cases — SPEC-09 Life-Stream Home

**Status:** current · **Last verified:** never

**Spec:** [SPEC-09](../product/specs/SPEC-09-life-stream-home.md) · **Module:** `journal`/stream + home
**Prefix:** `TC-STREAM-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md) · **Risk:** R4 (stream/event correctness)

### Endpoints under test

| Method | Path | Perm |
|---|---|---|
| GET | `/api/v1/stream?cursor=&limit=` | `stream:read:own` |
| GET | `/api/v1/stream/memories` | `stream:read:own` (P1.5) |

### Preconditions

- All producer modules wired (media, bank, comic, people, journal) so real events flow.
- Accounts `owner`,`userA`,`userB`. Problem type: `stream/invalid-cursor`.
- **Regression focus:** the fan-out edges that project system events into `stream_items`
  must be registered on the **api** publisher (bank/comic/media) **and** the **worker**
  (media_ready, playback_completed, birthday) — an unregistered edge silently drops the item.

---

## P0.1 — Projection: `stream_items` + consumers

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-STREAM-001 | Journal row written transactionally | Integration | P0(S1) | create journal entry | `stream_items` row (`source_module=journal`, `event_type=journal:entry_created`, ref_id=entry, payload `{}`) written in the entry's own tx | ☐ |
| TC-STREAM-002 | Journal edit updates projection | Integration | P0 | edit body / occurred_at | stream renders edited body at edited position | ☐ |
| TC-STREAM-003 | Journal delete removes projection | Integration | P0 | delete entry | stream item gone | ☐ |
| TC-STREAM-004 | media:asset_ready not projected | Integration | P0 | asset reaches ready (any origin) | **no** stream item — bell only since `0033` (SPEC-09 P0.1(b) note) | ☐ (CC-5) |
| TC-STREAM-005 | People retractions | Integration | P0 | delete a person / edit a birthday after its notice was emitted | `people:person_deleted` / `people:birthday_notice_revoked` remove the matching items (specced; not emitted on HEAD) | ☐ |
| TC-STREAM-006 | media:playback_completed → insert | Integration | P1 | complete ≥95% playback | one item, ref_id=asset_id | ☐ |
| TC-STREAM-007 | media:asset_deleted deletes ALL media rows | Integration | P0(S1) | delete asset with ready + playback_completed items | **all** `source_module=media` rows for ref_id gone (no dangling "watched X") | ☐ |
| TC-STREAM-008 | Transfer collapses to ONE item | Integration | P0(S1) | one transfer (two bank:transaction_created legs) | **exactly one** stream item (ref_id=transfer_id collapses the pair) | ☐ |
| TC-STREAM-009 | Non-transfer txn keyed on transaction_id | Integration | P0 | normal expense | item ref_id=transaction_id | ☐ |
| TC-STREAM-010 | bank:transaction_updated upsert | Integration | P0 | update a txn amount | item payload+occurred_at updated (corrected amount wins) | ☐ |
| TC-STREAM-011 | bank:transaction_deleted removes | Integration | P0 | delete txn | matching stream item gone | ☐ |
| TC-STREAM-012 | comic:published not projected | Integration | P1 | publish comic | **no** stream item — bell only since `0034` | ☐ |
| TC-STREAM-013 | birthday keyed on notice_id | Integration | P0(S1) | SPEC-11 3-day + day-of events for same person, this year + next year | **four** distinct items (keyed on notice_id, not person_id) | ☐ |
| TC-STREAM-014 | Redelivery idempotent | Idempotency | P0(S1) | Asynq retry any consumer | no duplicate item (ON CONFLICT DO NOTHING) | ☐ (CC-5) |
| TC-STREAM-015 | Unknown event_type skipped | Reliability | P0 | inject a future/unmapped event | skipped with a log line, never an error loop | ☐ |
| TC-STREAM-016 | Migration backfill of pre-existing entries | Integration | P0 | journal entries created before SPEC-09 landed | appear in `/stream` (migration `INSERT…SELECT` backfill) | ☐ |
| TC-STREAM-017 | occurred_at from payload, TZ-explicit | Functional | P0 | bank txn dated today created 21:00 local; bank txn dated 2026-07-01 for an owner whose `users.timezone` is `Asia/Ho_Chi_Minh`; re-categorize a txn; playback_completed delivered late | today's txn sorts above an 08:00-local journal entry; the 07-01 item falls on 07-01 local; re-categorize leaves its position unchanged; playback item sits at payload `completed_at` (SPEC-09 P0.1) | ☐ |
| TC-STREAM-018 | Update before create; per-user unique key | Integration | P0 | `bank:transaction_updated` delivered before its created event; two users complete playback of one shared asset | one bank item showing the updated amount; each user has their own playback item (`UNIQUE (user_id, source_module, event_type, ref_id)`) | ☐ |

## P0.2 — Stream read API

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-STREAM-030 | Merged order + cursor traversal | Functional | P0 | mix journal + system items; page | one stable merged order `occurred_at DESC, id DESC`; no dupes/gaps | ☐ (CC-4) |
| TC-STREAM-031 | Owner isolation | AuthZ | P0(S1) | userA GET `/stream` | never contains userB's items | ☐ (CC-3) |
| TC-STREAM-032 | Journal items render full | Functional | P0 | journal item in response | carries body_md, mood, asset_ids, location (Entry shapes), joined from journal_entries; every item has id, source_module, event_type, ref_id, occurred_at; response `{items, next_cursor?}` | ☐ |
| TC-STREAM-033 | System items render compact w/ title+href | Functional | P0 | each mapped event type | synthesized title+href per mapping (e.g. playback_completed → "Finished watching <title>", `/library/media/{asset_id}` per SPEC-10 P0.4's media deep-link rule) | ☐ |
| TC-STREAM-034 | Transfer direction-normalized card | Functional | P0 | transfer item (either leg collapsed) | renders identical "moved <amount> <source>→<dest>" | ☐ |
| TC-STREAM-035 | Unmapped event_type → generic card, no 5xx | Reliability | P0 | stored item with no mapping | 200 with generic card (no href), never 5xx | ☐ |
| TC-STREAM-036 | limit default 30, hard max 50 | Boundary | P0 | GET `?limit=100`; `?limit=abc`; no limit | clamped to 50; 30 for invalid or absent (no Problem) | ☐ |
| TC-STREAM-037 | Invalid cursor → Problem | Negative | P1 | GET `?cursor=garbage` | 400 `journal/invalid-cursor` | ☐ (CC-1) |

## P0.3 — Home `/` replacement

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-STREAM-050 | Zero fixture data (grep) | Frontend | P0 | grep home route | no fixture data anywhere on `/` | ☐ (CC-9) |
| TC-STREAM-051 | New post survives refetch | Frontend | P0 | post; immediate refetch | appears at occurred_at position optimistically **and** survives refetch (projection in create tx) | ☐ |
| TC-STREAM-052 | Infinite scroll | Frontend | P1 | scroll stream | TanStack infinite query against `/stream`; composer stays on top | ☐ |
| TC-STREAM-053 | Day grouping in the user's zone | Frontend | P0 | user zone `Asia/Ho_Chi_Minh`, browser zone `America/New_York`; journal entry at 2026-07-01T08:00+07:00 (2026-06-30 21:00 in New York) | card dated 2026-07-01 under that day's separator, never 2026-06-30; "Today"/"Yesterday" computed in `Asia/Ho_Chi_Minh` | ☐ |

## P0.4 — Widget rail

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-STREAM-070 | Rail renders w/ only account+media | Frontend | P0 | fresh instance, only account+media wired | rail renders w/o errors; empty states where backends absent (404) | ☐ (CC-9) |
| TC-STREAM-071 | One widget 500 doesn't blank rail | Reliability | P0 | force one widget endpoint to 500 | other widgets render normally; no toast storm | ☐ |
| TC-STREAM-072 | Widget sources correct | Contract | P1 | inspect widget queries | PersonalInfo←/auth/me, Activity←/me/notifications, Finance←/bank/dashboard, Continue←/continue, Birthdays←/people/upcoming-birthdays, Weather←Open-Meteo forecast API from the browser (no Portal endpoint) | ☐ |
| TC-STREAM-073 | Weather: configured location wins | Frontend | P1 | build with `NEXT_PUBLIC_WEATHER_LAT/LON/PLACE`; open `/` | no geolocation prompt; one Open-Meteo request for those coordinates; the configured place label shown | ☐ |
| TC-STREAM-074 | Weather: denied or failed → compact card | Reliability | P1 | no configured location; deny geolocation; separately, make `api.open-meteo.com` answer 503 | compact "Bật định vị để xem thời tiết" card; no weather values, no toast; the other widgets render normally | ☐ |
| TC-STREAM-075 | Weather: success render, no fixtures | Frontend | P1 | allow geolocation; open `/`; grep `WeatherWidget.tsx` and `lib/weather.ts` | current temperature, high/low, WMO label and icon, feels-like and rain chance, seven daily entries starting "Nay"; the card opens `/weather`; no fixture weather in either file | ☐ (CC-9) |
| TC-STREAM-076 | `/weather` behind the edge gate | AuthZ | P1 | unauthenticated GET `/weather` | redirected to `/login` by `middleware.ts` (SPEC-09 §11 row 14) | ☐ |

## P1 — nice to have

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-STREAM-090 | On-this-day memories (journal only) | Functional | P1 | GET `/stream/memories` | journal entries whose occurred_at month/day = today, both taken in the caller's `users.timezone`, prior years; response `{items: [{years_ago, entries}]}` ordered `years_ago` ASC (`{items: []}` when none); no system items | ☐ [P1] |
| TC-STREAM-091 | Feb-29 memory on Feb-28 | Boundary | P1 | Feb-29 memory in non-leap year | surfaces on Feb-28 (matches SPEC-11 rule) | ☐ [P1] |
| TC-STREAM-092 | *(retired with SPEC-09 P1.6 — the backfill seeded `media:asset_ready` rows, which are no longer projected)* | — | — | — | — | — |

## Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-STREAM-110 | Producer-crash residual risk documented | Reliability | P1 | kill producer between commit and Publish | item dropped (accepted v1 risk); P2 reconcile is the seam — verify behavior, not a fix | ☐ |
| TC-STREAM-111 | Home LCP < 2.5 s (50 items) | Performance | P0 | measure home LCP | < 2.5 s | ☐ [MANUAL] |
| TC-STREAM-112 | Migration up/down | Contract | P1 | migrate + down `journal_stream_items` | clean; UNIQUE(user_id,source_module,event_type,ref_id); cursor index; `stream:read:own` seed and backfill included | ☐ |
| TC-STREAM-113 | events.md Consumers column updated | Contract | P1 | check events.md | each consumed event lists the stream consumer | ☐ (CC-5) |
