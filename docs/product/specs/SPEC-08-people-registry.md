# SPEC-08 — People Registry (contacts + birthdays, n=1)

**Status:** current, rev 1 · **Drafted:** 2026-07-10 · **Last verified:** 2026-09-30
**Module:** `people` · **Depends on:** rides (or, if first, introduces) SPEC-01 P0.3 (shared periodic scheduler) and P0.6 (`platform/events` fan-out); P1.7 avatars need SPEC-01; birthday *delivery* compounds with SPEC-04/SPEC-06 — emission is day-one regardless
**Upstream:** [briefs/08-people-registry.md](../briefs/08-people-registry.md) · **Refs:** [ADR-08](../../adr/08-life-os-pivot.md), feature-inventory `D-17` (user timezone), [analysis/facebook-comparison.md](../analysis/facebook-comparison.md) row "Events / birthdays" (the former backlog §3 P2 item, re-scoped here), Monica-CRM pattern
**Downstream consumers:** SPEC-06 (stream + `BirthdayCard` widget), SPEC-09 P1.7 (takeout); SPEC-04 is a *future* consumer (needs a `notify:on_*` task + type row first)

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
2. Upcoming birthdays surface as bus events days ahead, computed in the v1
   timezone (`APP_TIMEZONE`, P0.3), exactly once per threshold per year.
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
birthday: { month: 1-12, day: 1-31, year?: int, calendar?: 'solar'|'lunar',
            leap_month?: boolean /* lunar only */ } | null
```

`POST`/`PATCH` accept it whole; `PATCH { birthday: null }` NULLs
`birth_month`/`birth_day`/`birth_year`, resets `birth_calendar` to its
`'solar'` default and `birth_leap_month` to `false` (both columns are NOT NULL
per §6); partial inner updates are not supported (send the whole object —
month/day travel together by design).
**When a PATCH changes the effective occurrence** (`birth_month` or
`birth_day` differ from the stored values, `birth_calendar` changes, or the
birthday is cleared), the same transaction deletes the person's
`people_birthday_notices` rows with `year >=` the current year in
`APP_TIMEZONE` (P0.3) — otherwise old-date notices suppress the corrected
date's events for the rest of the year (stale-dedup bug), while past years'
history stays. Resending an identical birthday object, or changing only
`birth_year`, leaves the notice rows untouched: every PATCH that carries a
birthday must resend the whole object, so resetting on any birthday-bearing
PATCH would re-emit an already-fired notice under a new `notice_id` and
duplicate it in the stream. *(Code follow-up: HEAD resets on every PATCH that
carries `birthday`, after the update rather than in its transaction.)*
For every cleared notice that had already been emitted (`emitted_at IS NOT
NULL`), the service emits **`people:birthday_notice_revoked`**
`{notice_id, person_id, user_id}` after commit, so SPEC-06 deletes the stale
stream item; the corrected date emits fresh events under new notice_ids.
Deleting a person emits **`people:person_deleted`** `{person_id, user_id}`
after commit (§7 DELETE), so SPEC-06 drops every birthday card that would link
to the deleted person's 404. Both are registered in
[events.md](../../reference/events.md). Consumers must not re-fetch by
notice_id (the row may be gone); the payload is self-sufficient for rendering.
**A rename does not re-emit**: the stream renders the stored `display_name`
snapshot, accepted v1 staleness until the card ages out. *(Code follow-up:
neither event is emitted on HEAD.)*

Birthday validation (app layer, on top of the §6 CHECKs): month and day must be
set together; `birth_year` requires month+day and must be plausible
(1900..current year) for both calendars. The real-calendar-date and
Feb-29/leap-year checks apply only when `calendar='solar'`: the (month, day)
pair must be a real Gregorian date — Feb-29 is allowed with no year or a leap
year, rejected against a non-leap `birth_year`. For `calendar='lunar'`: month
1–12, day 1–30, optional `leap_month` (default `false`); `leap_month: true`
with `calendar='solar'` is rejected. Violations: 422 Problem
`people/invalid-birthday`. *(Code follow-up: HEAD applies the Gregorian checks
to lunar birthdays too, so lunar 30/2 is rejected, and has no `leap_month`
field or column.)*

**Acceptance criteria.**
- Given person rows of user B, then user A's list/fetch excludes them; direct
  fetch is 404.
- Given a birthday without a year, then it renders and schedules correctly (no
  age shown, no crash).
- Given Feb-30, or a day without a month, then 422 `people/invalid-birthday`.
- Given lunar 30/2, then 201 (accepted — lunar dates skip the Gregorian check).
- Given a birthday edited from 15/03 to 20/09 after the 3-day 15/03 notice
  fired, then the 20/09 occurrence emits normally this year (notice rows for
  current/future years were cleared by the edit).
- Given a PATCH that changes only `display_name` and resends the unchanged
  birthday after the 3-day notice fired, then no second 3-day event is emitted.
- Given 200 people, cursor paging is stable (ordered `display_name`, `id`).
- Given a person with an emitted birthday stream item, when the person is
  deleted or the birthday edited, then `people:person_deleted` /
  `people:birthday_notice_revoked` fires and the stale stream item is gone.

### P0.3 — Upcoming birthdays endpoint

`GET /api/v1/people/upcoming-birthdays?days=14` (default 14; a value below 1,
absent or unparseable means the default; above 366 is clamped to 366) —
permission `people:read:own`. Computes each person's **next occurrence** in the
v1 timezone (below) and returns people with `0 ≤ days_until ≤ days`
(inclusive; today = 0; people without a birthday are excluded), sorted by
`days_until`, then `display_name`, then `id`. *(Code follow-up: HEAD sorts by
`days_until` only, with an unstable sort, so same-day ties have no fixed
order.)*

**TZ source at v1** (D-17 names user-TZ boundaries; this is a declared v1
deviation): the existing platform setting `APP_TIMEZONE` (`platform/config`,
IANA, default `UTC`), injected into people as `Deps.Timezone`.
`users.timezone` has no write path and is not in `accountapi.UserSummary`;
per-user TZ needs both and is out of scope. An unparseable name falls back to
UTC with a warning. *(Code follow-up: `cmd/api` and `cmd/worker` build
`people.Deps` without `Timezone`, so it runs in UTC regardless of
`APP_TIMEZONE`.)* Response items:
`{person_id, display_name, next_occurrence (date), days_until, age_turning?}`
(`age_turning` only when `birth_year` is known). Powers SPEC-06's `BirthdayCard`.

**Feb-29 rule (locked from the brief's open question): celebrate Feb-28 in
non-leap years** — a reminder that fires a day early beats one that never fires;
the scan (P0.4) and this endpoint share one `nextOccurrence` function and one
test suite.

**Acceptance criteria.**
- Given a birthday tomorrow in `APP_TIMEZONE` but today in UTC, then
  `days_until = 1` (timezone-correct, regression test).
- Given a Feb-29 birthday queried in a non-leap year, then `next_occurrence` is
  Feb-28.
- Given `lunar` rows, then they are omitted at v1 (not wrong dates — absent).
- Given a birthday today, then it is returned with `days_until = 0`.
- Given `birth_year` 1966 queried in 2026, then `age_turning` is 60; with no
  year, `age_turning` is absent.
- Given `days=0` (or no `days`), then the window is the default 14; given
  `days=500`, then it is clamped to 366.
- Given two people with the same `days_until`, then they are ordered by
  `display_name`, then `id`; a person with no birthday never appears.

### P0.4 — Birthday scan + event

Daily periodic task **`people:scan_birthdays`** on the shared periodic runner
(SPEC-01 P0.3's convention — no OS cron). For each tenant (`forEachTenant` in
`cmd/worker`, one committed tenant scope each), for every user's people, evaluate
`days_until` in `APP_TIMEZONE` (P0.3) and emit **`people:birthday_upcoming`**
`{notice_id, person_id, user_id, display_name, days_until}` for thresholds
`T ∈ {3, 0}` — **once per (person, threshold, occurrence-year)**. Rows with
`birth_calendar='lunar'` are skipped at v1 (§3).

**Emit rule (catch-up included, 2026-07-10):** for each threshold `T`, emit
when `lower(T) ≤ days_until ≤ T` **and** no notice row exists for
`(person, occurrence_year, T)`, where `lower(3) = 1` and `lower(0) = 0` — not
only on exact equality, which would permanently drop a threshold whenever the
scan missed a day (worker down past retries): a 3-day notice arriving 1 day
out is late but still useful; a missed day-of notice is never emitted after
the day passes (`days_until ≥ 0` bound). A 3-day notice is never emitted on
the day itself: the day-of notice supersedes it, so a person created (or a
birthday edited) on the birthday, or a scanner down days −3..−1, yields one
item, not two. *(Code follow-up: HEAD uses `0 ≤ days_until ≤ T` for both
thresholds, so both fire at `days_until = 0`.)*

**Outbox-style dedup + delivery (2026-07-10 — insert-then-emit was
at-most-once):** inside the scan's transaction, insert the
`people_birthday_notices` row with `emitted_at NULL` (`ON CONFLICT DO
NOTHING`); after commit, publish the event (via `platform/events`; v1
consumer: stream only — notify attaches when a `notify:on_*` task is specced in
SPEC-04 §7) and set `emitted_at`. Rows left with `emitted_at
NULL` (crash/enqueue failure between commit and publish) are re-published by
the next scan — at-least-once delivery, with consumers idempotent by
`notice_id` (SPEC-06 keys stream items on it). `notice_id` is the notices
row's `id` column — a UNIQUE surrogate; the table's PRIMARY KEY is the
composite (person_id, year, threshold) (§6).

**Re-publish payload.** The insert stores the row's `occurrence` date (§6). A
re-publish recomputes `days_until = occurrence − today` (in `APP_TIMEZONE`)
and rebuilds the payload from the current person row. If `days_until < 0`, it
does not publish: it sets `emitted_at = now()` and logs the notice as expired.
If a smaller threshold also emits for the same (person, year) in this scan,
the pending larger-threshold row is marked suppressed (`emitted_at = now()`,
`suppressed = true`, no publish) — the same rule as the day-0 collapse above.
*(Code follow-up: HEAD has no `occurrence` or `suppressed` column and
publishes `days_until` = the matched threshold on every retry, however late.)*

Register the event in [events.md](../../reference/events.md). Consumers —
stream (SPEC-06) today; notify (SPEC-04) once specced — attach as they land;
**emission is day-one regardless** (ADR-08 rule).

**Schedule:** daily at 06:00 UTC on the shared scheduler, once per tenant via
`forEachTenant`, evaluated in `APP_TIMEZONE`. This deliberately deviates from
D-17's hourly per-TZ pattern: a threshold can be recognized up to ~a day late
depending on run hour vs the TZ (the catch-up rule absorbs it), and a fixed-UTC
run near local midnight can skip one local date across a DST change, losing
that day-of notice. The re-entry fix is D-17's hourly pattern.

**Acceptance criteria.**
- Given a birthday 3 days out, then exactly one 3-day event fires; day-of fires
  once; an Asynq retry of the scan emits nothing extra (dedup-table test).
- Given the scanner down on day −3 and back on day −2, then the 3-day-threshold
  event fires once on day −2 (catch-up); given it down on the birthday itself
  and back the day after, then that year's day-of event is skipped, not
  emitted late.
- Given a person created (or a birthday edited) on the birthday itself, or the
  scanner down days −3..−1, then only the day-of event fires.
- Given a crash between the notice insert and the publish, then the next scan
  publishes exactly that notice (`emitted_at NULL` retry) and consumers see no
  duplicate item; if the next scan runs after the occurrence date, the row
  expires unpublished; if it runs on the day itself, only the day-of event is
  published and the pending 3-day row is suppressed.
- Given a lunar row, then the scan inserts no notice and emits nothing.
- Given the same person next year, then events fire again (new occurrence-year).
- Given a person deleted between scans, then no event (and their notice rows are
  gone via cascade).

### P0.5 — Frontend

`/people` list + person detail (RSC-first shell, client islands per D-33 —
"RSC shell" as defined in the specs README Frontend convention); create/edit
forms. `FriendCard` is **not** reused as-is: its stats row
(Friends/Photos/Videos) and Add-friend/Message `ControlBlockButtons` are the
friend-graph actions §3 rules out. A card view reuses only its layout (cover,
`Avatar`, name) through a `PersonCard` wrapper, or a new `actions` prop, that
renders no stats row and no `ControlBlockButtons`; its subtitle shows
relationship and next birthday. The detail view may use `PersonalInfoWidget`.
Both kits hard-code sample data as prop defaults today; those defaults are
removed before either renders people (SPEC-06 P0.4 covers
`PersonalInfoWidget`). On HEAD the list and detail views use neither kit: rows
are the base `Card` with the initials `Avatar`. **`BirthdayCard` is reworked, not wired** (the
widget slot itself is SPEC-06 P0.4's rail): it becomes a self-fetching client
island (TanStack query `["people","upcoming"]` →
`GET /people/upcoming-birthdays?days=14`) with no fixture defaults. The
headline is "Today is" at `days_until=0`, "Tomorrow is" at 1, else "In N days";
it lists every upcoming person, shows a skeleton while loading, renders nothing
when the list is empty, and shows `age_turning` when present. `HomeView`'s
inline `BirthdayCard` is replaced by it. Extend `config.matcher` in
`frontend/src/middleware.ts` with `'/people/:path*'` so the D-34 auth gate
covers the new route. The people list and detail views are declared in
`TemplateManifest.views` (`templates/types.ts`), implemented under
`templates/v1/views/...`, and `app/(app)/people/page.tsx` and
`app/(app)/people/[id]/page.tsx` resolve `activeTemplate().views.peopleList`
and `views.peopleDetail` (`ComponentType<{ id: string }>`) — no
version-specific import in `app/`. The detail path `/people/{person_id}` is
the link target of SPEC-06's birthday stream item.

**Acceptance criteria.**
- Grep test: `BirthdayCard` and the people views render zero fixture rows (no
  sample-data prop defaults).
- Given upcoming birthdays at 0, 1 and 5 days, then the card headlines read
  "Today is", "Tomorrow is" and "In 5 days"; given none, the card renders
  nothing.
- Given no people yet, then `/people` shows an empty state with a create action.
- Given a signed-out visitor on `/people` or `/people/{id}`, then they are
  redirected to `/login`.
- Given an invalid birthday submitted in the form, then the 422
  `people/invalid-birthday` detail renders inline.
- Given a person rendered as a card, then no stats row and no
  Add-friend/Message control appears.

### P1 — nice to have

- **P1.6 Interactions log**: `people_interactions` (§6) — kinds
  `met|called|messaged|gifted|other`, `occurred_on` date, note; "last contact"
  (max `occurred_on`, the Person's `last_contact_on`, §7) shown on the person
  card; log-from-detail-page UI; a mis-logged interaction can be deleted (§7).
  `other` is added beyond the brief as a catch-all (2026-07-10).
  *AC:* given interactions logged on two dates, then the person card shows the
  latest `occurred_on`; given one deleted, then it no longer counts.
- **P1.7 Avatar**: `avatar_asset_id` via `mediaapi` (ready image, owner —
  SPEC-01), falling back to the deterministic-hue initials `Avatar`. Subscribe to
  `media:asset_deleted` — via the `platform/events` fan-out (events.md "Delivery
  mechanics") — → NULL matching `avatar_asset_id` (SPEC-02 P0.6 pattern). The
  field lands on `PATCH` (and `POST`, same validation) once P1.7 ships (§7).
  Uploading the avatar asset needs `assets:write:own`, granted to `user` by the
  SPEC-01 grant migration (SPEC-01 §7; README AuthZ floor).
  *AC:* given `media:asset_deleted` for an avatar, then `avatar_asset_id` is
  NULL and the initials `Avatar` renders.

### P2 — future considerations (design for, don't build)

- **vCard import** (file upload → rows; no live sync).
- **Lunar recurrence math** for `birth_calendar='lunar'` (giỗ, âm lịch) — the
  column already exists; re-entry: the owner asks for a lunar reminder.
- **Household sharing** of selected people — arrives with tenant
  `kind: household`; keep `user_id` scoping clean so a tenant scope layers on.

## 6. Data model — migration `000N_people_persons`

**Tenancy** (specs README convention, ADR-07). Tenant-scoped: `people_persons` and `people_birthday_notices` (`tenant_id` + index + `tenant_isolation` policy, added by `0020_platform_rls_enable`); circles are a column on `people_persons` (`0035`), not a table. The DDL below predates ADR-07 and omits the columns.

```sql
CREATE TABLE people_persons (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                    -- identity-anchor exception (SPEC-04 §6 precedent)
  display_name    text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 120),
  relationship    text,                     -- 'mẹ', 'bạn đại học' — freeform
  birth_month     int  CHECK (birth_month BETWEEN 1 AND 12),
  birth_day       int  CHECK (birth_day BETWEEN 1 AND 31),
  birth_year      int  CHECK (birth_year IS NULL OR birth_year >= 1900),
                    -- nullable by design; upper bound (current year) is app-layer
  birth_calendar  text NOT NULL DEFAULT 'solar' CHECK (birth_calendar IN ('solar','lunar')),
  birth_leap_month boolean NOT NULL DEFAULT false,  -- lunar leap month (tháng nhuận)
  contact         jsonb NOT NULL DEFAULT '{}',  -- phones/emails/addresses; convention, not schema
  note_md         text,
  avatar_asset_id uuid,                     -- media asset, validated via mediaapi (P1.7)
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK ((birth_month IS NULL) = (birth_day IS NULL)),
  CHECK (birth_year IS NULL OR birth_month IS NOT NULL),
  CHECK (NOT birth_leap_month OR birth_calendar = 'lunar'),
  CHECK (birth_calendar = 'solar' OR birth_day IS NULL OR birth_day <= 30)
);
CREATE INDEX people_persons_user_idx ON people_persons (user_id, display_name, id);  -- list cursor
CREATE INDEX people_persons_user_bday_idx ON people_persons (user_id, birth_month, birth_day);

-- P0.4 dedup + outbox: one emit per (person, occurrence-year, threshold).
-- `id` is the event's notice_id — SPEC-06 keys stream items on it, which is
-- what lets recurring years/thresholds coexist under the stream's unique key.
-- `threshold` stores the T that matched (3|0) — the observed days_until may
-- be smaller under the catch-up rule. `emitted_at NULL` = written but not
-- yet published; the next scan retries it.
CREATE TABLE people_birthday_notices (
  id         uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  person_id  uuid NOT NULL REFERENCES people_persons(id) ON DELETE CASCADE,
  year       int  NOT NULL,           -- occurrence year, in APP_TIMEZONE (P0.3)
  threshold  int  NOT NULL CHECK (threshold IN (0, 3)),
  occurrence date NOT NULL,           -- celebrated date (Feb-28-adjusted) in APP_TIMEZONE at insert
  suppressed boolean NOT NULL DEFAULT false,  -- superseded by a smaller threshold; never published
  emitted_at timestamptz,             -- NULL until published, expired or suppressed
  PRIMARY KEY (person_id, year, threshold)
);
-- the re-publish pass reads only unemitted rows
CREATE INDEX people_birthday_notices_pending_idx
  ON people_birthday_notices (person_id) WHERE emitted_at IS NULL;

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
(P0.2). **Shipped vs. target:** `0016_people_persons` shipped both indexes on
`people_persons` but none of `birth_leap_month`, `occurrence`, `suppressed`,
the `birth_year`/`threshold`/lunar CHECKs, or the pending index. Because 0016
has shipped, they land in a follow-up migration `000N_people_checks` (backfill
`occurrence` for existing notice rows from the person's birthday and `year`
before setting NOT NULL). *(Code follow-up.)* `0035_people_circles` later added
`circle` (`close_friend|family|other`, default `other`) and `linked_user_id`
(nullable `users(id)`, unique per owner). Queries in `query/people_*.sql`; regenerate via `make sqlc`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| POST | `/api/v1/people` | `people:write:own` | `{display_name, relationship?, birthday?, contact?, note_md?}` — `birthday` shape per P0.2 |
| GET | `/api/v1/people?cursor=&limit=&circle=` | `people:read:own` | ordered (`display_name`, `id`); `limit` default 50, max 200 (a value ≤ 0 or > 200 falls back to 50); optional `circle` filter (unknown value → 422 `people/validation`); shipped envelope `{people: Person[], next_cursor?: string}`, not the README's `{items}` (existing shape kept); malformed cursor → 400 `people/invalid-cursor` |
| GET | `/api/v1/people/{id}` | `people:read:own` | 404 for others' rows |
| PATCH | `/api/v1/people/{id}` | `people:write:own` | `birthday: null` clears; a birthday change that moves the occurrence resets current/future notice rows and emit `people:birthday_notice_revoked` per cleared emitted notice (P0.2); P1.7 adds `avatar_asset_id?: uuid|null` (validated via mediaapi: exists, kind image, status ready, owned — else 422 `people/invalid-asset`; null clears) |
| DELETE | `/api/v1/people/{id}` | `people:delete:own` | 204; idempotent 404; emits `people:person_deleted` after commit (P0.2) |
| GET | `/api/v1/people/upcoming-birthdays?days=` | `people:read:own` | `APP_TIMEZONE` (P0.3); sorted `days_until`, `display_name`, `id`; non-paginated — shipped as `{upcoming: [...]}`, not the README's `{items}` (existing shape kept) |
| POST | `/api/v1/people/{id}/interactions` | `people:write:own` | P1.6 |
| GET | `/api/v1/people/{id}/interactions` | `people:read:own` | P1.6 |
| DELETE | `/api/v1/people/{id}/interactions/{interaction_id}` | `people:delete:own` | P1.6; 204; 404 for others' rows |

**Person response** (every person-returning operation):
`{id, display_name, relationship|null, birthday: {month, day, year|null, calendar, leap_month?} | null, contact, note_md|null, avatar_asset_id|null (P1.7), circle, linked_user_id|null, last_contact_on: date|null (P1.6; max occurred_on), created_at, updated_at}`.
`circle` and `linked_user_id` come from `0035_people_circles` (§6). HEAD
returns every field except `leap_month` (P0.2 follow-up) and
`last_contact_on` (P1.6, unbuilt).

Problem types: `people/person-not-found`, `people/invalid-birthday`,
`people/invalid-asset` (P1.7), `people/invalid-cursor` (400, malformed list
cursor), `people/validation` (422, body/param shape). Each is registered in
`frontend/src/lib/problems.ts` per the specs README Errors convention.
*(Code follow-up: none of the people slugs is in `problems.ts` on HEAD.)*

The list follows the specs README Pagination convention for cursor, limit and
errors; its envelope is the shipped `{people, next_cursor}` (row above).
Annotate each operation per the specs README AuthZ **OpenAPI encoding**.

## 8. Success metrics (n=1 honest)

- Leading: the owner records ≥ 10 real people in the first week (data-entry
  friction test).
- Leading: over the first month, zero missed and zero duplicated
  `people:birthday_upcoming` emissions against the known registry (auditable from
  the notices table for people whose birthday was not edited and who were not
  deleted in the window, cross-checked against SPEC-06 `stream_items` where
  `event_type = 'people:birthday_upcoming'`).
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
