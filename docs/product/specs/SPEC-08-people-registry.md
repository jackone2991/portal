# SPEC-08 — People Registry (contacts + birthdays, n=1)

**Status:** ready to build, rev 1 · **Drafted:** 2026-07-10
**Module:** `people` (new — not scaffolded) · **Depends on:** nothing hard; P1.7 avatars need SPEC-01; birthday *delivery* compounds with SPEC-04/SPEC-06 — emission is day-one regardless
**Upstream:** [briefs/08-people-registry.md](../briefs/08-people-registry.md) · **Refs:** [ADR-08](../../adr/08-life-os-pivot.md), feature-inventory `D-17` (user timezone), backlog §3 P2 ("Events/birthdays", re-scoped here), Monica-CRM pattern
**Downstream consumers:** SPEC-06 (stream + `BirthdayCard` widget), SPEC-04 (notification type, when wired), SPEC-09 P1.7 (takeout)

---

## 1. Problem statement

Brief 00's flagship life-stream example — "mom's birthday in 3 days" — is
impossible today: nothing stores who "mom" is or when her birthday falls. The
cataloged "Events/birthdays" backlog item assumes multi-user social events/RSVP,
which is parked behind real second users. Monica-CRM proves the loophole: **my
relationships are my data** — an owner-scoped address book needs no second
account and no friend graph, and it is the cheapest source of recurring,
emotionally relevant events for the stream, the bell, and the dormant
`BirthdayCard`/`FriendCard` UI kits.

## 2. Goals

1. The owner records people (family, friends) with birthdays and contact notes.
2. Upcoming birthdays surface as bus events days ahead, computed in the owner's
   timezone, exactly once per threshold per year.
3. `BirthdayCard` and the people list render real rows, not fixtures.

## 3. Non-goals

- **Not a friend graph.** People are rows, not accounts: no requests, no chat, no
  user search. The [briefs/04-deferred.md](../briefs/04-deferred.md) friend-graph
  row stays parked; its re-entry condition (real second users) is neither met nor
  needed here.
- **Full Monica parity** (activities, gifts, debts-between-people) — P1 keeps one
  interactions log; the rest waits for demonstrated use.
- **CardDAV / Google Contacts sync** — import-only later (P2), never live sync.
- **Lunar-calendar math at v1** — the `birth_calendar` column ships now (locked
  from the brief's open question: cheap in migration #1), the recurrence math
  ships when giỗ/âm lịch reminders are actually requested. v1 scan skips
  `lunar` rows.

## 4. User stories

- As the owner, I add "Mẹ — sinh nhật 15/03" once, and every year the stream and
  bell warn me 3 days out and on the day. *(primary — the ADR-08 example)*
- As the owner, I open a person and see their birthday, phone, and my notes
  ("thích trà ô long, không cà phê").
- As the owner, I record someone whose birth year I don't know, and everything
  still works (no fake ages).
- As the owner, I log "called grandma" so next month I notice how long it's been.
  *(P1)*
- Edge: a person with no birthday at all is a perfectly valid contact row.

## 5. Requirements

### P0.1 — Module scaffold

Full MODULES.md §8 checklist: `internal/modules/people/` subtree, own `sqlc.yaml`
block, migration `000N_people_persons` (next free number), wired into
`cmd/api/main.go` and `cmd/worker/main.go` (the scan task, P0.4, lives worker-side),
depguard isolation block added.

### P0.2 — CRUD

`POST/GET/PATCH/DELETE /api/v1/people` — permissions `people:read:own` /
`people:write:own` / `people:delete:own` (canonical scheme: `write` covers
create + update, matching the 0003 catalog; reconciled 2026-07-10). The people
migration seeds the three codes and grants them to the base `user` role.
Fields per §6: display name (1–120), freeform `relationship`, birthday as
**month/day with optional year** (many people won't share a year),
`birth_calendar` `solar|lunar` (default `solar`), schemaless `contact` jsonb,
markdown note.

**Birthday wire shape** *(2026-07-10 — previously undefined while being the
spec's central concept)*: the API carries one nested object, mapped to the
four flat columns internally:

```
birthday: { month: 1-12, day: 1-31, year?: int, calendar?: 'solar'|'lunar' } | null
```

`POST`/`PATCH` accept it whole; `PATCH { birthday: null }` NULLs
`birth_month`/`birth_day`/`birth_year` and resets `birth_calendar` to its
`'solar'` default (the column is NOT NULL per §6); partial inner updates are
not supported (send the whole object — month/day travel together by design).
**Editing or clearing a birthday also deletes the person's
`people_birthday_notices` rows for the current and future occurrence years**
— otherwise old-date notices suppress the corrected date's events for the
rest of the year (stale-dedup bug), while past years' history stays.
Already-emitted stream/notification items from the deleted notices are NOT
retracted at v1 — accepted staleness (they age out; the corrected date emits
fresh events under new notice_ids). Consumers must not re-fetch by notice_id
(the row may be gone); the payload is self-sufficient for rendering.

Birthday validation (app layer, on top of the §6 CHECKs): month and day must be
set together; the (month, day) pair must be a real calendar date — Feb-29 is
allowed with no year or a leap year, rejected against a non-leap `birth_year`;
`birth_year` requires month+day and must be plausible (1900..current year).
Violations: 422 Problem `people/invalid-birthday`.

**Acceptance criteria.**
- Given person rows of user B, then user A's list/fetch excludes them; direct
  fetch is 404.
- Given a birthday without a year, then it renders and schedules correctly (no
  age shown, no crash).
- Given Feb-30, or a day without a month, then 422 `people/invalid-birthday`.
- Given a birthday edited from 15/03 to 20/09 after the 3-day 15/03 notice
  fired, then the 20/09 occurrence emits normally this year (notice rows for
  current/future years were cleared by the edit).
- Given 200 people, cursor paging is stable (ordered `display_name`, `id`).

### P0.3 — Upcoming birthdays endpoint

`GET /api/v1/people/upcoming-birthdays?days=14` (clamp 1–366, default 14) —
permission `people:read:own`. Computes each person's **next occurrence** in the
owner's timezone (D-17 — the stored user timezone; fall back to the instance
default when unset), sorted soonest-first. Response items:
`{person_id, display_name, next_occurrence (date), days_until, age_turning?}`
(`age_turning` only when `birth_year` is known). Powers SPEC-06's `BirthdayCard`.

**Feb-29 rule (locked from the brief's open question): celebrate Feb-28 in
non-leap years** — a reminder that fires a day early beats one that never fires;
the scan (P0.4) and this endpoint share one `nextOccurrence` function and one
test suite.

**Acceptance criteria.**
- Given a birthday tomorrow in the owner's TZ but today in UTC, then
  `days_until = 1` (timezone-correct, regression test).
- Given a Feb-29 birthday queried in a non-leap year, then `next_occurrence` is
  Feb-28.
- Given `lunar` rows, then they are omitted at v1 (not wrong dates — absent).

### P0.4 — Birthday scan + event

Daily periodic task **`people:scan_birthdays`** on the shared periodic runner
(SPEC-01 P0.3's convention — no OS cron). For every user's people, evaluate
`days_until` in that user's timezone and emit **`people:birthday_upcoming`**
`{notice_id, person_id, user_id, display_name, days_until}` for thresholds
`T ∈ {3, 0}` — **once per (person, threshold, occurrence-year)**.

**Emit rule (catch-up included, 2026-07-10):** for each threshold `T`, emit
when `0 ≤ days_until ≤ T` **and** no notice row exists for
`(person, occurrence_year, T)` — not only on exact equality, which would
permanently drop a threshold whenever the scan missed a day (worker down past
retries): a 3-day notice arriving 1 day out is late but still useful; a
missed day-of notice is never emitted after the day passes (`days_until ≥ 0`
bound).

**Outbox-style dedup + delivery (2026-07-10 — insert-then-emit was
at-most-once):** inside the scan's transaction, insert the
`people_birthday_notices` row with `emitted_at NULL` (`ON CONFLICT DO
NOTHING`); after commit, publish the event (via `platform/events` — two
consumers are registered) and set `emitted_at`. Rows left with `emitted_at
NULL` (crash/enqueue failure between commit and publish) are re-published by
the next scan — at-least-once delivery, with consumers idempotent by
`notice_id` (SPEC-06 keys stream items on it). `notice_id` is the notices
row's `id` column — a UNIQUE surrogate; the table's PRIMARY KEY is the
composite (person_id, year, threshold) (§6).

Register the event in [events.md](../../reference/events.md). Consumers —
stream (SPEC-06), notify (SPEC-04) — attach as they land; **emission is
day-one regardless** (ADR-08 rule).

Timing precision note: one daily run evaluated against per-user local dates is
deliberately coarse (a threshold can be recognized up to ~a day late depending on
run hour vs user TZ). Acceptable at v1 — the catch-up rule absorbs it; tighten
the schedule only if a real miss annoys.

**Acceptance criteria.**
- Given a birthday 3 days out, then exactly one 3-day event fires; day-of fires
  once; an Asynq retry of the scan emits nothing extra (dedup-table test).
- Given the scanner down on day −3 and back on day −2, then the 3-day-threshold
  event fires once on day −2 (catch-up); given it down on the birthday itself
  and back the day after, then that year's day-of event is skipped, not
  emitted late.
- Given a crash between the notice insert and the publish, then the next scan
  publishes exactly that notice (`emitted_at NULL` retry) and consumers see no
  duplicate item.
- Given the same person next year, then events fire again (new occurrence-year).
- Given a person deleted between scans, then no event (and their notice rows are
  gone via cascade).

### P0.5 — Frontend

`/people` list + person detail, reusing the `FriendCard` / `PersonalInfoWidget`
kits (RSC-first shell, client islands per D-33); create/edit forms; `BirthdayCard`
on home wired to P0.3 (the widget slot itself is SPEC-06 P0.4's rail — this item
delivers the card's real query + states). Extend `config.matcher` in
`frontend/src/middleware.ts` with `'/people/:path*'` so the D-34 auth gate
covers the new route. The people list and detail views are declared in
`TemplateManifest.views` (`templates/types.ts`), implemented under
`templates/v1/views/...`, and `app/(app)/people/page.tsx` (and the detail
route) resolve them via `activeTemplate().views.<x>` — no version-specific
import in `app/`.

**Acceptance criteria.**
- Grep test: `BirthdayCard` and the people views render zero fixture rows.
- Given no people yet, then `/people` shows an empty state with a create action.

### P1 — nice to have

- **P1.6 Interactions log**: `people_interactions` (§6) — kinds
  `met|called|messaged|gifted|other`, `occurred_on` date, note; "last contact"
  (max `occurred_on`) shown on the person card; log-from-detail-page UI.
- **P1.7 Avatar**: `avatar_asset_id` via `mediaapi` (ready image, owner —
  SPEC-01), falling back to the deterministic-hue initials `Avatar`. Subscribe to
  `media:asset_deleted` — via the `platform/events` fan-out (events.md "Delivery
  mechanics") — → NULL matching `avatar_asset_id` (SPEC-02 P0.6 pattern). The
  field lands on `PATCH` (and optionally `POST`) once P1.7 ships (§7). Uploading
  the avatar asset requires the media catalog's creator-tier `assets:write:own`
  (0003 seed) — the v1 owner account is provisioned `creator` or higher, so this
  is available by inheritance (README AuthZ).

### P2 — future considerations (design for, don't build)

- **vCard import** (file upload → rows; no live sync).
- **Lunar recurrence math** for `birth_calendar='lunar'` (giỗ, âm lịch) — the
  column already exists; re-entry: the owner asks for a lunar reminder.
- **Household sharing** of selected people — arrives with tenant
  `kind: household`; keep `user_id` scoping clean so a tenant scope layers on.

## 6. Data model — migration `000N_people_persons`

```sql
CREATE TABLE people_persons (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    -- identity-anchor exception (SPEC-04 §6 precedent)
  display_name    text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 120),
  relationship    text,                     -- 'mẹ', 'bạn đại học' — freeform
  birth_month     int  CHECK (birth_month BETWEEN 1 AND 12),
  birth_day       int  CHECK (birth_day BETWEEN 1 AND 31),
  birth_year      int,                      -- nullable by design
  birth_calendar  text NOT NULL DEFAULT 'solar' CHECK (birth_calendar IN ('solar','lunar')),
  contact         jsonb NOT NULL DEFAULT '{}',  -- phones/emails/addresses; convention, not schema
  note_md         text,
  avatar_asset_id uuid,                     -- media asset, validated via mediaapi (P1.7)
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK ((birth_month IS NULL) = (birth_day IS NULL)),
  CHECK (birth_year IS NULL OR birth_month IS NOT NULL)
);
CREATE INDEX ON people_persons (user_id, birth_month, birth_day);

-- P0.4 dedup + outbox: one emit per (person, occurrence-year, threshold).
-- `id` is the event's notice_id — SPEC-06 keys stream items on it, which is
-- what lets recurring years/thresholds coexist under the stream's unique key.
-- `threshold` stores the T that matched (3|0) — the observed days_until may
-- be smaller under the catch-up rule. `emitted_at NULL` = written but not
-- yet published; the next scan retries it.
CREATE TABLE people_birthday_notices (
  id         uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  person_id  uuid NOT NULL REFERENCES people_persons(id) ON DELETE CASCADE,
  year       int  NOT NULL,           -- occurrence year, in the owner's TZ
  threshold  int  NOT NULL,           -- 3 | 0
  emitted_at timestamptz,             -- NULL until successfully published
  PRIMARY KEY (person_id, year, threshold)
);

-- P1.6
CREATE TABLE people_interactions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  person_id   uuid NOT NULL REFERENCES people_persons(id) ON DELETE CASCADE,
  kind        text NOT NULL CHECK (kind IN ('met','called','messaged','gifted','other')),
  occurred_on date NOT NULL,
  note        text,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON people_interactions (person_id, occurred_on DESC);
```

Calendar-validity beyond the coarse CHECKs (Feb-30, leap years) is app-layer
(P0.2). Queries in `query/people_*.sql`; regenerate via `make sqlc`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| POST | `/api/v1/people` | `people:write:own` | `{display_name, relationship?, birthday?, contact?, note_md?}` — `birthday` shape per P0.2 |
| GET | `/api/v1/people?cursor=` | `people:read:own` | ordered by name |
| GET | `/api/v1/people/{id}` | `people:read:own` | 404 for others' rows |
| PATCH | `/api/v1/people/{id}` | `people:write:own` | `birthday: null` clears; birthday edits reset current/future notice rows (P0.2); P1.7 adds `avatar_asset_id?: uuid|null` (validated via mediaapi: exists, kind image, status ready, owned — else 422 `people/invalid-asset`; null clears) |
| DELETE | `/api/v1/people/{id}` | `people:delete:own` | 204; idempotent 404 |
| GET | `/api/v1/people/upcoming-birthdays?days=` | `people:read:own` | user-TZ; soonest first |
| POST/GET | `/api/v1/people/{id}/interactions` | `people:write:own` / `people:read:own` | P1.6 |

Problem types: `people/person-not-found`, `people/invalid-birthday`,
`people/invalid-asset` (P1.7).

## 8. Success metrics (n=1 honest)

- Leading: the owner records ≥ 10 real people in the first week (data-entry
  friction test).
- Leading: over the first month, zero missed and zero duplicated
  `people:birthday_upcoming` emissions against the known registry (auditable from
  the notices table).
- Lagging: `BirthdayCard` renders real rows (grep); the "mom's birthday" story
  demonstrably works end-to-end once SPEC-06 lands.

## 9. Timeline & phasing

1. Scaffold + migration + sqlc + CRUD + RBAC + OpenAPI (1.5 days)
2. `nextOccurrence` (TZ + Feb-29 + no-year cases, table-driven tests) +
   upcoming-birthdays endpoint (1 day)
3. Scan task + dedup + event emit (1 day)
4. Frontend list/detail + BirthdayCard wiring (1 day)
5. P1 (interactions, avatar) (1 day)
P0 ≈ 4.5 dev-days, matching the brief; P1 adds ~1.

## 10. Open questions

- **(resolved)** Lunar birthdays: column now (`birth_calendar`), math later (§3).
- **(resolved)** Feb-29: celebrate Feb-28 in non-leap years (P0.3).
- **(product, non-blocking)** `contact` jsonb key convention (`phones[]`,
  `emails[]`, `addresses[]`) — document in the module README at implementation;
  it is deliberately not schema.
- **(product, non-blocking)** Should day-of events also carry `age_turning` for
  the stream card copy ("Mẹ turns 60")? Cheap to add to the payload when known —
  decide at implementation with SPEC-06's card design.
