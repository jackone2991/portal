# SPEC-08 — People Registry (contacts + birthdays, n=1)

**Status:** current, rev 1 · **Drafted:** 2026-07-10 · **Last verified:** 2026-10-01
**Module:** `people` · **Depends on:** rides (or, if first, introduces) SPEC-01 P0.3 (shared periodic scheduler) and P0.6 (`platform/events` fan-out); P1.7 avatars need SPEC-01; birthday *delivery* compounds with SPEC-04/SPEC-06 — emission is day-one regardless
**Upstream:** brief 08 — contacts as *data*, not user accounts; the social facet at n=1 (folded into this spec, then deleted — `git show 2cdda7e:docs/product/briefs/08-people-registry.md`) · **Refs:** [ADR-08](../../adr/08-life-os-pivot.md), feature-inventory `D-17` (user timezone), the Facebook comparison (deleted — `git show 2cdda7e:docs/product/analysis/facebook-comparison.md`) row "Events / birthdays" (the former backlog §3 P2 item, re-scoped here), Monica-CRM pattern
**Downstream consumers:** SPEC-06 (stream + `BirthdayCard` widget), SPEC-09 P1.7 (takeout); SPEC-04 is a *future* consumer (needs a `notify:on_*` task + type row first)

---

## 1. Problem statement

The life-OS pivot's flagship life-stream example ([ADR-08](../../adr/08-life-os-pivot.md)) — "mom's birthday in 3 days" — is
impossible today: nothing stores who "mom" is or when her birthday falls. The
cataloged "Events/birthdays" backlog item assumes multi-user social events/RSVP,
which is parked behind real second users. Monica-CRM proves the loophole: **my
relationships are my data** — an owner-scoped address book needs no second
account and no friend graph, and it is the cheapest source of recurring,
emotionally relevant events for the stream, the bell, and the dormant
`BirthdayCard`/`FriendCard` UI kits.

## 2. Goals

1. The owner records people (family, friends) with birthdays and contact notes.
2. Upcoming birthdays surface as bus events days ahead, computed in the
   owner's own timezone (`users.timezone`, P0.3), exactly once per threshold
   per year.
3. `BirthdayCard` and the people list render real rows, not fixtures.

## 3. Non-goals

- **Not a friend graph.** People are rows, not accounts: no requests, no chat, no
  user search. The [backlog.md § Deferred](../backlog.md) friend-graph
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
`people_birthday_notices` rows with `year >=` the current year in the
owner's timezone (P0.3) — otherwise old-date notices suppress the corrected
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

**Circle and linked account** (`0035_people_circles`). POST also accepts
`circle` (`close_friend|family|other`, default `other`; any other value → 422
`people/validation`; PATCH may change it) and `linked_user_id` — the id of a
portal account this person **is**, set when the person is added from a
suggestion (`GET /people/suggestions`, unowned by this spec) so that account
stops being suggested; absent, `null` or `""` means none. PATCH does not take
`linked_user_id`. An owner's registry holds each linked account at most once (the
`0035` partial unique index `people_persons_linked_user_idx` on `(user_id,
linked_user_id) WHERE linked_user_id IS NOT NULL`): a POST whose
`linked_user_id` is already linked to one of the caller's people is **409
`people/already-in-registry`** and nothing is written. The insert runs `ON
CONFLICT … DO NOTHING` and maps "no row" to that 409 — letting the unique
violation raise would abort the request's tenant transaction and turn the 409
into a 500 at commit. A malformed `linked_user_id`, or one that names no
account, is 422 `people/validation`. *(Code follow-up: HEAD answers a malformed
id with 400 `about:blank` and lets an unknown one hit the `users(id)` FK → 500 —
§11 row 14.)*

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
- Given a person already created with `linked_user_id = U`, when the owner POSTs
  another person with `linked_user_id = U`, then 409
  `people/already-in-registry` and the registry still holds one person linked
  to U; another owner may still link U.
- Given a person with an emitted birthday stream item, when the person is
  deleted or the birthday edited, then `people:person_deleted` /
  `people:birthday_notice_revoked` fires and the stale stream item is gone.

### P0.3 — Upcoming birthdays endpoint

`GET /api/v1/people/upcoming-birthdays?days=14` (default 14; a value below 1,
absent or unparseable means the default; above 366 is clamped to 366) —
permission `people:read:own`. Computes each person's **next occurrence** in the
caller's timezone (below) and returns people with `0 ≤ days_until ≤ days`
(inclusive; today = 0; people without a birthday are excluded), sorted by
`days_until`, then `display_name`, then `id`. *(Code follow-up: HEAD sorts by
`days_until` only, with an unstable sort, so same-day ties have no fixed
order.)*

**TZ source** (specs README Timezone, D-17): the owner's own
`users.timezone`, read through `accountapi` (`UserSummary.Timezone`); the
frontend keeps it current from the device's location, with a manual override
in settings. Unknown or unparseable → `Asia/Ho_Chi_Minh`. "Today" is the
owner's local date in that zone; there is no instance-wide zone. *(Code
follow-up: HEAD injects one zone as `people.Deps.Timezone`, and `cmd/api` and
`cmd/worker` build `people.Deps` without it, so `people.New` falls back to
`Asia/Ho_Chi_Minh` for every owner (UTC only if the zone fails to load); the
fix drops `Deps.Timezone` for a per-owner lookup through `accountapi`, which
first needs the README Timezone follow-up.)* Response items:
`{person_id, display_name, next_occurrence (date), days_until, age_turning?}`
(`age_turning` only when `birth_year` is known). Powers SPEC-06's `BirthdayCard`.

**Feb-29 rule (locked from the brief's open question): celebrate Feb-28 in
non-leap years** — a reminder that fires a day early beats one that never fires;
the scan (P0.4) and this endpoint share one `nextOccurrence` function and one
test suite.

**Acceptance criteria.**
- Given a birthday tomorrow in the owner's `users.timezone` but today in UTC,
  then `days_until = 1` (timezone-correct, regression test).
- Given an owner who never set a zone, then the endpoint evaluates in
  `Asia/Ho_Chi_Minh`; given two owners in `Asia/Ho_Chi_Minh` and
  `America/Los_Angeles` queried at 20:00 UTC, then a birthday date that is 2
  days away for the Los Angeles owner is 1 day away for the Ho Chi Minh owner.
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
`days_until` in that user's own timezone (P0.3) and emit **`people:birthday_upcoming`**
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
re-publish recomputes `days_until = occurrence − today` (in the owner's timezone)
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

**Schedule** (D-17's hourly per-TZ pattern, specs README Timezone): **hourly**
at minute 5 on the shared scheduler, once per tenant via `forEachTenant`. Each
run resolves every owner's zone once through `accountapi` and evaluates
`days_until` against **that owner's local date** — no run uses one "today" for
everyone. Re-running every hour is safe because an emit needs a missing
`(person, occurrence-year, threshold)` notice row: a run that finds the row
emits nothing. So each threshold is recognized within the first hour of the
owner's local day, whatever the zone, and a DST change cannot skip a local
date. *(Code follow-up: HEAD registers the scan daily at 06:00 UTC and
evaluates one zone for all owners — `Asia/Ho_Chi_Minh` in practice, see P0.3.)*

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
- Given owners in `Asia/Ho_Chi_Minh` and `America/Los_Angeles` with a
  birthday on the same date, then each owner's day-of event fires in the first
  hourly run after that owner's local midnight (17:05 UTC the day before for
  Ho Chi Minh, 07:05 or 08:05 UTC for Los Angeles depending on DST), and later
  runs that day emit nothing.
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
  year       int  NOT NULL,           -- occurrence year, in the owner's timezone (P0.3)
  threshold  int  NOT NULL CHECK (threshold IN (0, 3)),
  occurrence date NOT NULL,           -- celebrated date (Feb-28-adjusted) in the owner's timezone at insert
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
| POST | `/api/v1/people` | `people:write:own` | `{display_name, relationship?, birthday?, contact?, note_md?, circle?, linked_user_id?}` — `birthday` shape, `circle` and `linked_user_id` per P0.2; 409 `people/already-in-registry` when `linked_user_id` is already in the caller's registry |
| GET | `/api/v1/people?cursor=&limit=&circle=` | `people:read:own` | ordered (`display_name`, `id`); `limit` default 50, max 200, lenient per the specs README rule (missing, non-integer or < 1 → 50; above 200 → clamped to 200; never a Problem); optional `circle` filter (unknown value → 422 `people/validation`); returns `{items: Person[], next_cursor?: string}` (specs README Pagination; *code follow-up: HEAD answers `{people, next_cursor}`*); malformed cursor → 400 `people/invalid-cursor` |
| GET | `/api/v1/people/{id}` | `people:read:own` | 404 for others' rows |
| PATCH | `/api/v1/people/{id}` | `people:write:own` | `birthday: null` clears; a birthday change that moves the occurrence resets current/future notice rows and emit `people:birthday_notice_revoked` per cleared emitted notice (P0.2); P1.7 adds `avatar_asset_id?: uuid|null` (validated via mediaapi: exists, kind image, status ready, owned — else 422 `people/invalid-asset`; null clears) |
| DELETE | `/api/v1/people/{id}` | `people:delete:own` | 204; idempotent 404; emits `people:person_deleted` after commit (P0.2) |
| GET | `/api/v1/people/upcoming-birthdays?days=` | `people:read:own` | caller's `users.timezone` (P0.3); sorted `days_until`, `display_name`, `id`; non-paginated, returns `{items: [...]}` (specs README Pagination; *code follow-up: HEAD answers `{upcoming: [...]}`*) |
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
cursor), `people/validation` (422, body/param shape),
`people/already-in-registry` (409, a POST whose `linked_user_id` is already
linked to one of the caller's people — P0.2; declared 2026-10-01). Each is
registered in `frontend/src/lib/problems.ts` per the specs README Errors
convention. *(Code follow-up: HEAD's `problems.ts` registers four of the six —
`people/already-in-registry` is emitted but unregistered (§11 row 12), and
`people/invalid-asset` lands with P1.7.)*

Both lists follow the specs README Pagination convention in full — cursor,
limit, errors and the `{items}` envelope. Pre-rule endpoints are retrofitted,
not grandfathered (owner decision 2026-09-30), so neither keeps its shipped
`people` / `upcoming` key. *(Code follow-up: the retrofit renames both keys to
`items` in the handler, `shared/openapi.yaml` and the frontend readers — the
`/people` list and `BirthdayCard` — in one PR.)*
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

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the docs commits on top of it change no
code). The spec text above is the target; this section lists every place the
shipped code still diverges from it, so an implementer needs nothing but this
spec. Rows are ordered by severity: lost or duplicated birthday events first,
then wrong dates, then schema, contract and UI, then unbuilt P1. A row closes
when the code matches the requirement it cites and the SPEC-08 row of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) is regraded on
a named test. File paths are relative to the repo root; "F-ids" refer to
[spec-gap-fix-worklog-2026-09-30.md](../analysis/spec-gap-fix-worklog-2026-09-30.md).
The module lives in `backend/internal/modules/people/`.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.2 notice reset on PATCH | Delete the person's notice rows with `year >=` the owner's current year **only when the effective occurrence changes** (month or day differ, calendar changes, or the birthday is cleared), **in the same transaction** as the update. Resending an identical birthday or changing only `birth_year` leaves notices untouched. | `service.go` `UpdatePerson`: whenever the PATCH carries `birthday` (`in.SetBirthday`), it calls `repo.DeleteFutureNotices` **after** `repo.UpdatePerson` returned, outside any transaction, and only logs a failure. Every ordinary save resends the whole birthday object, so a rename after the 3-day notice fired deletes it and the next scan re-emits under a new `notice_id` — **a duplicate stream item**; a failed delete instead leaves the old date's notices suppressing the corrected date for the rest of the year. The year comes from the single `Service.loc` (row 5). | **backend:** read the stored row and update it in one `pgx.Tx`; compute "occurrence changed" from (month, day, calendar, cleared); only then run `DeleteFutureNotices` in that tx, changed to `RETURNING id, emitted_at` so row 2 can revoke; use the owner's zone for the year. Return the delete error (no best-effort). **test:** TC-PPL-013, TC-PPL-014, TC-PPL-017. | F081 |
| 2 | P0.2 retraction events; §7 DELETE | After commit, `people:birthday_notice_revoked {notice_id, person_id, user_id}` per cleared notice that had `emitted_at IS NOT NULL`, and `people:person_deleted {person_id, user_id}` on DELETE, so SPEC-06 drops stale and dangling birthday cards. | Neither event exists: `api/api.go` declares only `EventBirthdayUpcoming`; `service.go` `DeletePerson` and `UpdatePerson` publish nothing. A deleted person's birthday card stays in the stream linking to a 404; an edited birthday keeps its old-date card. | **backend:** add both constants and payload structs to `people/api`; publish after commit from `DeletePerson` and from row 1's reset (one revoke per emitted cleared notice); `cmd/api` already passes `Events` to `people.Deps`. Flip both rows in events.md from planned to live; SPEC-06 P0.1(b) adds the two `journal:stream_*` consumers and the `cmd/worker` subscriptions. **test:** TC-STREAM-005 plus a people unit test that a delete and an occurrence-changing PATCH publish the expected payloads and a no-op PATCH publishes none. | F017 |
| 3 | P0.4 emit rule (day-0 collapse) | Emit threshold `T` when `lower(T) ≤ days_until ≤ T` with `lower(3) = 1`, `lower(0) = 0`: on the day itself only the day-of notice fires. | `service.go` `ScanBirthdays`: `if daysUntil >= 0 && daysUntil <= T` for both `T ∈ {3, 0}`, so a person created or edited on the birthday, or a scanner down days −3..−1, gets **two items for one day**. | **backend:** use the lower bound above. **test:** TC-PPL-058, TC-PPL-052 (catch-up still fires the 3-day notice at 1 or 2 days out). | F080 |
| 4 | P0.4 re-publish payload; §6 notices | Notice rows store `occurrence date NOT NULL` and `suppressed boolean`; a re-publish recomputes `days_until = occurrence − today` in the owner's zone, expires the row unpublished when `< 0`, and suppresses a pending larger threshold when a smaller one emits for the same (person, year). | `0016_people_persons` has no `occurrence` or `suppressed` column; `query/people.sql` `InsertNotice` stores `(person_id, year, threshold)` only; `ScanBirthdays` re-publishes every `emitted_at IS NULL` row with `DaysUntil: n.Threshold` however late, so a crash before the birthday re-publishes "in 3 days" after the date has passed. | **migration:** `000N_people_checks` adds `occurrence date` (backfill existing rows from the person's month/day and `year`, Feb-29 → Feb-28 in non-leap years, then `SET NOT NULL`) and `suppressed boolean NOT NULL DEFAULT false`. **backend:** `InsertNotice` stores the occurrence; `PendingNotices` returns it; the publish loop applies the expire and suppress rules and marks such rows `emitted_at = now()` without publishing. **test:** TC-PPL-054, TC-PPL-059. | F082 |
| 5 | P0.3 TZ source (scan and endpoint) | Each owner's `users.timezone`, read through `accountapi` (`UserSummary.Timezone`); unknown or unparseable → `Asia/Ho_Chi_Minh`; no instance-wide zone. | `module.go` `Deps.Timezone` → one `Service.loc` for every owner; `cmd/api/main.go` and `cmd/worker/main.go` build `people.Deps` without it, so `people.New` loads `Asia/Ho_Chi_Minh` for everyone (UTC only if loading fails; both images install `tzdata`). `0002_account_users` defaults `timezone` to `'UTC'`; nothing writes it; `account/api/api.go` `UserSummary` is `{ID, Email, DisplayName}`. | **migration (account):** default `'Asia/Ho_Chi_Minh'`, rewrite untouched `'UTC'` rows, add `timezone_manual` (specs README Timezone). **backend (account):** `PATCH /api/v1/auth/me {timezone}` with `account/invalid-timezone`; expose the zone on `accountapi` (single lookup plus a batch by user ids for the scan). **backend (people):** drop `Deps.Timezone`/`Service.loc`; inject a zone lookup from `accountapi` in both binaries; pass the owner's `*time.Location` to `nextOccurrence`, the endpoint and row 1's year. **openapi/frontend:** the `/auth/me` field and `problems.ts` slug (owned by the README Timezone follow-up). **test:** TC-PPL-030, TC-PPL-038. | F028, Decision 2026-09-30 (Timezone) |
| 6 | P0.4 schedule | Hourly at minute 5 on the shared scheduler, once per tenant via `forEachTenant`; each run evaluates every owner's own local date. | `cmd/worker/main.go` registers `scheduler.Register("0 6 * * *", people:scan_birthdays)` — daily at 06:00 UTC, one zone for all owners (row 5); a day-of notice lands at 13:00 local for a Ho Chi Minh owner and a DST shift can skip a local date. | **backend:** change the cron to `"5 * * * *"`; resolve each owner's zone once per run (row 5's batch lookup). Dedup already makes re-runs safe. **test:** TC-PPL-061. | F028, Decision 2026-09-30 (Timezone) |
| 7 | P0.2 lunar validation; `leap_month` | Gregorian and Feb-29 checks only for `calendar='solar'`; lunar: month 1–12, day 1–30, optional `leap_month` (default false), `leap_month: true` with solar → 422 `people/invalid-birthday`; `PATCH {birthday: null}` also resets `birth_leap_month`; the Person response carries `leap_month`. | `service.go` `validateBirthday` applies `validCalendarDate` to every calendar, so lunar 30/2 is rejected; `handler.go` `birthdayReq`, `types.go` `Birthday`, `personJSON` and `frontend/src/lib/people.ts` `Birthday` have no `leap_month`; `0016` has no `birth_leap_month` column. | **migration:** `000N_people_checks` adds `birth_leap_month boolean NOT NULL DEFAULT false`, `CHECK (NOT birth_leap_month OR birth_calendar = 'lunar')`, `CHECK (birth_calendar = 'solar' OR birth_day IS NULL OR birth_day <= 30)`. **backend:** branch `validateBirthday` on calendar; carry `leap_month` through request, domain, `CreatePerson`/`UpdatePerson` queries (cleared on `birthday: null`) and `personJSON`. **openapi:** `leap_month` on the birthday object. **frontend:** add it to `Birthday`. **test:** TC-PPL-018. | F083 |
| 8 | §6 CHECKs and pending index | `birth_year IS NULL OR birth_year >= 1900`; `threshold IN (0, 3)`; partial index `people_birthday_notices (person_id) WHERE emitted_at IS NULL`. | `0016` has `birth_year INT` with no bound, `threshold INT NOT NULL` with no CHECK, and no pending index (the app layer already rejects years < 1900). | **migration:** add both CHECKs (`NOT VALID` then `VALIDATE`) and the partial index in `000N_people_checks`. **test:** TC-PPL-114 (migration up/down). | F173 |
| 9 | §7 list envelopes | `GET /people` returns `{items: Person[], next_cursor?}`; `GET /people/upcoming-birthdays` returns `{items: [...]}`; handler, OpenAPI and readers change in one PR. | `handler.go` `List` writes `{"people": …}` and `Upcoming` writes `{"upcoming": …}`; `shared/openapi.yaml` `listPeople` requires `[people]` and `upcomingBirthdays` requires `[upcoming]`; `frontend/src/lib/people.ts` (`listPeople`, `upcomingBirthdays`) and `PeopleIndexView.tsx` (`people.data?.people`) read the old keys; `BirthdayCard` reads through `upcomingBirthdays`. | **backend:** rename both keys to `items`. **openapi:** both schemas → `required: [items]`. **frontend:** `PeoplePage.items`, `upcomingBirthdays` reads `r.items`, `PeopleIndexView` reads `.items`. The unowned `GET /people/suggestions` (`{suggestions}`) is on the same retrofit list. **test:** TC-PPL-019, TC-PPL-039. | Decision 2026-09-30 (Envelopes), F168 |
| 10 | P0.3 sort | Sorted by `days_until`, then `display_name`, then `id`. | `service.go` `UpcomingBirthdays`: `sort.Slice` on `DaysUntil` only — unstable, so same-day ties come back in arbitrary order. | **backend:** one comparator over (`DaysUntil`, `DisplayName`, `PersonID`). **test:** TC-PPL-035. | F170 |
| 11 | P0.5 `BirthdayCard` | Lists every upcoming person, shows a skeleton while loading, renders nothing when empty. | `frontend/src/templates/v1/components/widget/BirthdayCard.tsx` renders only `data[0]` and returns `null` while loading (no skeleton). Fixture props, the self-fetching query, the headlines and `age_turning` already match. | **frontend:** map over every item (headline per item); render a skeleton while `isLoading`. **test:** TC-PPL-070 (grep stays green) plus a render test for 0/1/5 days and empty. | F029 |
| 12 | §7 problem types | Every people slug the API emits is declared in §7 and registered in `problems.ts`. | `handler.go` `writePeopleErr` emits `people/already-in-registry` (409, declared in §7 and P0.2 since 2026-10-01), but `frontend/src/lib/problems.ts` registers only `people/person-not-found`, `people/invalid-birthday`, `people/validation` and `people/invalid-cursor`; `people/invalid-asset` waits for P1.7. | **frontend:** add `people/already-in-registry` to `ProblemType` and `PROBLEM_MESSAGES` (and `people/invalid-asset` with P1.7). **test:** TC-PPL-111. | F168 (corrected 2026-10-01); Decision 2026-10-01 |
| 13 | P1.6 interactions; P1.7 avatar | `people_interactions` table, three routes, `last_contact_on`; `avatar_asset_id` validated through `mediaapi`, NULLed on `media:asset_deleted`. | Not built: no `people_interactions` migration or query, no routes, no `last_contact_on` in `personJSON`; `avatar_asset_id` is stored but never validated or written by any route, and nothing subscribes to `media:asset_deleted`. | **migration · backend · openapi · frontend:** as P1.6/P1.7 and §6/§7 specify. **test:** TC-PPL-090…093. | TRACEABILITY-MATRIX SPEC-08 P1.6/P1.7 (✖) |
| 14 | P0.2 `linked_user_id` shape and existence | A malformed `linked_user_id`, or one naming no account, is 422 `people/validation` and nothing is written. | `handler.go` `Create` answers a malformed id with `server.BadRequest` (400 `about:blank`). `service.go` `CreatePerson` never checks that the id names an account, so an unknown uuid reaches the `0035` FK `REFERENCES users(id)`, which raises inside the request's tenant transaction: 500. | **backend:** a malformed id → `ErrValidation`; before the insert, resolve the id through `accountapi` (the directory `Suggestions` already uses) and answer `ErrValidation` when it names no account. **openapi:** 409 `people/already-in-registry` and 422 `people/validation` on `createPerson`. **test:** TC-PPL-022. | Found while documenting decision (c), 2026-10-01 (no F-ID) |
| 15 | §7 list `limit` (owner decision 2026-10-01) | Missing, non-integer or < 1 → 50; above 200 → **clamped to 200**; never a Problem. | `service.go` `ListPeople`: `if limit <= 0 \|\| limit > maxLimit { limit = defaultLimit }` — `?limit=500` returns 50. The unowned `Suggestions` (same file) does the same with the same constants. | **backend:** clamp instead of resetting (`> 200 → 200`, `≤ 0 → 50`) in both, e.g. `platform/server.Limit(r, 50, 200)` in `handler.go`. **openapi:** describe `limit` as defaulted and clamped. **test:** TC-PPL-020. | Decision 2026-10-01 (limit) |

**Already matching on HEAD.**
- `0016_people_persons` ships both `people_persons` indexes, the
  month/day-together and year-needs-month CHECKs, the notices table with the
  composite PRIMARY KEY and `id` UNIQUE surrogate, and seeds the three
  permission codes to `user`; `0020` adds `tenant_id` and the policy;
  `0035` adds `circle` and `linked_user_id`.
- CRUD routes carry `people:read:own` / `write:own` / `delete:own`; another
  owner's row and a missing row answer the same 404
  `people/person-not-found`; a second DELETE is 404.
- The list orders by (`display_name`, `id`) with a cursor, `limit` default 50
  for a missing or invalid value (above 200 resets to 50 — row 15), unknown
  `circle` → 422 `people/validation`,
  malformed cursor → 400 `people/invalid-cursor`.
- `0035` linked accounts: `CreatePerson` inserts `ON CONFLICT (user_id,
  linked_user_id) … DO NOTHING` and maps no row to `ErrDuplicate` → 409
  `people/already-in-registry` (`repository/adapter.go`, `handler.go`
  `writePeopleErr`), so the conflict never aborts the tenant transaction.
- Upcoming birthdays: `days` absent, unparseable or `< 1` → 14, `> 366` → 366;
  inclusive window with today = 0; lunar and birthday-less people excluded;
  `age_turning` only with a year; Feb-29 → Feb-28 in non-leap years through one
  `nextOccurrence` shared with the scan.
- The scan runs once per tenant through `forEachTenant`, skips lunar rows,
  inserts notices `ON CONFLICT DO NOTHING` and re-publishes `emitted_at IS NULL`
  rows (at-least-once).
- Frontend: `/people/:path*` is in the `middleware.ts` matcher;
  `views.peopleList` / `views.peopleDetail` resolve through the template
  registry; `BirthdayCard` is self-fetching (`["people","upcoming"]`) with no
  fixture props and the Today / Tomorrow / In N days headlines; `HomeView` has
  no inline copy (the card is the `birthdays` widget); list rows use the base
  `Card` with the initials `Avatar` (no friend-graph controls); `/people` has an
  empty state with a create action and renders 422 details inline.

**Test evidence to add or fix.**
- `people_test.go` builds `Service` with a fixed `loc` and
  `http_test.go` passes `Deps{Timezone: "UTC"}`; both change when row 5 drops
  the field — rewrite `TestNextOccurrenceTimezone` around per-owner zones
  (TC-PPL-030, TC-PPL-038).
- `TestScanDedupAndOutbox` only crosses T=3 and T=0 on separate days; add the
  day-0 collapse (TC-PPL-058) and the catch-up case (TC-PPL-052).
- `TestOutboxRetry` re-publishes on the same day only; add a retry after the
  occurrence (expires) and on the day itself (3-day row suppressed)
  (TC-PPL-059). The fake `MarkNoticeEmitted` marks every pending row, which
  hides per-notice bugs; key it on the notice id.
- `TestBirthdayValidation` has no lunar case; add lunar 30/2 accepted and
  `leap_month` with solar rejected (TC-PPL-018).
- `TestUpcomingBirthdays` has no tie; add two people on the same day
  (TC-PPL-035).
- Decision (c) and (e), 2026-10-01: no test covers the 409 on a second link
  (TC-PPL-021), the `linked_user_id` shape and existence rule (TC-PPL-022,
  row 14) or `limit` clamping (TC-PPL-020, row 15).
- No test covers the PATCH notice reset, the retraction events, the
  `{items}` envelopes or the hourly per-owner scan (TC-PPL-013/014/017,
  TC-STREAM-005, TC-PPL-019/039, TC-PPL-061).
