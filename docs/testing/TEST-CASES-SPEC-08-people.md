# Test Cases — SPEC-08 People Registry

**Status:** current · **Last verified:** never

**Spec:** [SPEC-08](../product/specs/SPEC-08-people-registry.md) · **Module:** `people`
**Prefix:** `TC-PPL-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md)

### Endpoints under test

| Method | Path | Perm |
|---|---|---|
| POST | `/api/v1/people` | `people:write:own` |
| GET | `/api/v1/people?cursor=&limit=&circle=` | `people:read:own` |
| GET | `/api/v1/people/{id}` | `people:read:own` |
| PATCH | `/api/v1/people/{id}` | `people:write:own` |
| DELETE | `/api/v1/people/{id}` | `people:delete:own` |
| GET | `/api/v1/people/upcoming-birthdays?days=` | `people:read:own` |
| POST | `/api/v1/people/{id}/interactions` | `people:write:own` (P1.6) |
| GET | `/api/v1/people/{id}/interactions` | `people:read:own` (P1.6) |
| DELETE | `/api/v1/people/{id}/interactions/{interaction_id}` | `people:delete:own` (P1.6) |

### Preconditions

- Accounts `owner`,`userA`,`userB`,`guest`. Owner TZ is `users.timezone`, default `Asia/Ho_Chi_Minh` (specs README Timezone, D-17).
- Birthday wire shape: `birthday: { month:1-12, day:1-31, year?, calendar?:'solar'|'lunar', leap_month?:boolean /* lunar only */ } | null`.
- Problem types: `people/person-not-found`, `people/invalid-birthday`, `people/invalid-cursor`, `people/validation`, `people/already-in-registry` (409, P0.2), `people/invalid-asset` (P1.7).
- Event `people:birthday_upcoming {notice_id, person_id, user_id, display_name, days_until}` (T∈{3,0}).
- A controllable/relative clock for the birthday-timing cases (plan §5).

---

## P0.2 — CRUD

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-PPL-001 | Create person full fields | Functional | P0 | POST {display_name, relationship, birthday, contact, note_md} | 201; contact jsonb stored | ☐ |
| TC-PPL-002 | Person with no birthday valid | Functional | P0 | POST without birthday | 201; valid contact row (birthday null) | ☐ |
| TC-PPL-003 | Birthday without year renders/schedules | Functional | P0 | POST birthday {month, day} (no year) | 201; renders no age; schedules correctly; no crash | ☐ |
| TC-PPL-004 | Owner isolation | AuthZ | P0(S1) | userA list/fetch userB's people | absent; direct fetch 404 | ☐ (CC-3) |
| TC-PPL-005 | display_name empty / >120 → 422 | Boundary/Neg | P0 | POST name="" and 121-char | 422 | ☐ |
| TC-PPL-006 | Feb-30 → 422 | Boundary/Neg | P0 | POST birthday {month:2, day:30} | 422 `people/invalid-birthday` | ☐ |
| TC-PPL-007 | Day without month → 422 | Boundary/Neg | P0 | POST birthday {day:15} (no month) | 422 `people/invalid-birthday` (month+day travel together) | ☐ |
| TC-PPL-008 | Feb-29 no year / leap year OK | Boundary | P0 | POST {month:2,day:29} no year; and with leap birth_year | accepted | ☐ |
| TC-PPL-009 | Feb-29 against non-leap year → 422 | Boundary/Neg | P0 | POST {month:2,day:29, year:2027} | 422 `people/invalid-birthday` | ☐ |
| TC-PPL-010 | birth_year out of range → 422 | Boundary/Neg | P0 | year 1899 / next year+1 | 422 (plausible 1900..current year) | ☐ |
| TC-PPL-011 | Invalid calendar value → 422 | Boundary/Neg | P1 | calendar="gregorian" | 422/validation (only solar\|lunar) | ☐ |
| TC-PPL-012 | PATCH birthday:null clears + resets calendar | Functional | P0 | PATCH {birthday:null} | birth_month/day/year NULL; birth_calendar reset to solar | ☐ |
| TC-PPL-013 | Birthday edit clears current/future notices | Integration | P0(S1) | after 3-day 15/03 notice fired, PATCH birthday→20/09 | notices for current/future years cleared; 20/09 emits normally this year (no stale-dedup suppression) | ☐ |
| TC-PPL-014 | Past-year notices retained on edit | Integration | P1 | edit birthday | past years' notice history stays | ☐ |
| TC-PPL-015 | Cursor stable by name | Functional | P0 | 200 people; page | ordered `display_name, id`; stable, no dupes | ☐ (CC-4) |
| TC-PPL-016 | Idempotent delete | Idempotency | P0 | DELETE twice | 2nd → 404, never 500 | ☐ (CC-8) |
| TC-PPL-017 | Unchanged birthday resend does not reset notices | Integration | P0(S1) | after the 3-day notice fired, PATCH {display_name, birthday: <unchanged>}; then PATCH changing only birth_year; run scan | notice rows untouched; no second 3-day event | ☐ |
| TC-PPL-018 | Lunar 30/2 accepted; leap_month lunar-only | Boundary | P1 | POST {month:2, day:30, calendar:'lunar'}; POST {month:2, day:30, calendar:'lunar', leap_month:true}; POST solar with leap_month:true; lunar day 31 | 201; 201; 422 `people/invalid-birthday`; 422 | ☐ |
| TC-PPL-019 | People list envelope | Contract | P0 | GET `/people?limit=2` over 3 people | `{items: Person[], next_cursor}` then `{items}` without cursor; no `people` key (specs README Pagination) | ☐ |
| TC-PPL-020 | `limit` lenient: default and clamp | Boundary | P0 | GET `/people` with no `limit`, `limit=abc`, `limit=0`, `limit=500` over 250 people | 50, 50, 50, then **200** items (clamped to the max, owner decision 2026-10-01); never a Problem | ☐ (CC-4) |
| TC-PPL-021 | Second link of one account → 409 | Negative | P0 | POST `{display_name, linked_user_id: U}`; POST again with the same U; as another owner, POST with U | second POST → 409 `people/already-in-registry`, nothing written (one person linked to U); the other owner → 201 | ☐ |
| TC-PPL-022 | `linked_user_id` shape and existence | Negative | P1 | POST `linked_user_id: "nope"`; POST a random uuid naming no account | both 422 `people/validation`, nothing written, never 400 `about:blank` or 500 (§11 row 14) | ☐ |

## P0.3 — Upcoming birthdays endpoint

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-PPL-030 | days_until timezone-correct | Functional | P0(S1) | birthday tomorrow in the owner's `users.timezone` but today in UTC | `days_until=1` (regression test) | ☐ |
| TC-PPL-031 | Feb-29 → Feb-28 in non-leap year | Boundary | P0 | query Feb-29 person in non-leap year | `next_occurrence` = Feb-28 | ☐ |
| TC-PPL-032 | Lunar rows omitted | Functional | P0 | person with calendar=lunar | omitted at v1 (absent, not wrong date) | ☐ |
| TC-PPL-033 | days default 14, max 366 | Boundary | P0 | GET `?days=0`, `?days=500`, no param | 14-day window; clamped to 366; 14-day window | ☐ |
| TC-PPL-034 | age_turning only when year known | Functional | P1 | birth_year 1966 queried in 2026; person without birth_year | `age_turning` = 60; absent without a year | ☐ |
| TC-PPL-035 | Sort order and exclusions | Functional | P0 | multiple upcoming incl. two on the same day; one person with no birthday | sorted by `days_until`, then `display_name`, then `id`; the no-birthday person is absent | ☐ |
| TC-PPL-037 | Birthday today included | Boundary | P0 | person whose birthday is today | returned with `days_until = 0` | ☐ |
| TC-PPL-036 | Shared nextOccurrence with scan | Contract | P1 | compare endpoint vs scan | both use one `nextOccurrence` + one test suite | ☐ [AUTO] |
| TC-PPL-038 | Per-owner zone and default | Functional | P0(S1) | owners in `Asia/Ho_Chi_Minh` and `America/Los_Angeles`, same birthday date, queried at 20:00 UTC; an owner who never set a zone | the Ho Chi Minh owner's `days_until` is one smaller; the unset owner is evaluated in `Asia/Ho_Chi_Minh` | ☐ |
| TC-PPL-039 | Upcoming envelope | Contract | P0 | GET `/people/upcoming-birthdays` | `{items: [...]}`, no `upcoming` key (specs README Pagination) | ☐ |

## P0.4 — Birthday scan + event

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-PPL-050 | 3-day + day-of fire once each | Integration | P0(S1) | person 3 days out; run scan on day −3 and day 0 | exactly one 3-day event; one day-of event | ☐ |
| TC-PPL-051 | Scan retry emits nothing extra | Idempotency | P0(S1) | Asynq retry of scan | no extra events (dedup table) | ☐ |
| TC-PPL-052 | Catch-up: missed day −3, run day −2 | Reliability | P0(S1) | scanner down day −3, up day −2 | 3-day threshold fires once on day −2 (0≤days_until≤T + no notice row) | ☐ |
| TC-PPL-053 | Missed day-of not emitted late | Reliability | P0 | scanner down on birthday, up day after | that year's day-of event skipped, not emitted late (`days_until ≥ 0` bound) | ☐ |
| TC-PPL-054 | Outbox: crash between insert and publish | Reliability | P0(S1) | crash after notice insert (emitted_at NULL), before publish; next scan | next scan publishes exactly that notice with `days_until` recomputed from `occurrence`; consumers see no duplicate | ☐ |
| TC-PPL-058 | Day-0 collapse: only day-of fires | Reliability | P0(S1) | person created on the birthday; birthday edited to today; scanner down days −3..−1 then run on day 0 | only the day-of event; no 3-day notice row emitted | ☐ |
| TC-PPL-059 | Pending retry expires or is suppressed | Reliability | P0 | leave a 3-day row `emitted_at NULL`; next scan after the occurrence date / on the day itself | after: row expires unpublished (`emitted_at` set, no event); on the day: only day-of published, 3-day row `suppressed = true` | ☐ |
| TC-PPL-060 | Lunar rows skipped by the scan | Functional | P0 | lunar person 3 days / 0 days out; run scan | no notice row inserted; no event | ☐ |
| TC-PPL-061 | Hourly scan uses each owner's local date | Integration | P0(S1) | owners in `Asia/Ho_Chi_Minh` and `America/Los_Angeles` with a birthday on the same date; run the scan hourly across the day | each day-of event fires in the first run after that owner's local midnight (17:05 UTC the day before / 07:05 or 08:05 UTC); every later run that day emits nothing | ☐ |
| TC-PPL-055 | Next year fires again | Integration | P0 | same person next occurrence-year | events fire again (new year) | ☐ |
| TC-PPL-056 | Deleted person → no event, notices cascade | Reliability | P0 | delete person between scans | no event; notice rows gone via cascade | ☐ |
| TC-PPL-057 | notice_id is UNIQUE surrogate | Contract | P1 | inspect schema | `id` UNIQUE = notice_id; PK composite (person_id, year, threshold) | ☐ |

## P0.5 — Frontend

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-PPL-070 | BirthdayCard zero fixtures (grep) | Frontend | P0 | grep BirthdayCard + people views | zero fixture rows; real queries | ☐ (CC-9) |
| TC-PPL-071 | Empty state with create action | Frontend | P0 | no people yet | `/people` empty state + create action | ☐ |
| TC-PPL-072 | Auth gate on /people | Frontend | P0 | unauthenticated visit `/people` and `/people/{id}` | redirected to `/login` (middleware matcher `/people/:path*`) | ☐ |
| TC-PPL-074 | 422 renders inline | Frontend | P0 | submit an invalid birthday in the create/edit form | the `people/invalid-birthday` detail renders inline | ☐ |
| TC-PPL-075 | No friend-graph controls | Frontend | P1 | render a person card | no stats row; no Add-friend/Message control | ☐ |
| TC-PPL-073 | List/detail via template registry | Frontend | P1 | inspect `app/(app)/people/page.tsx` and `people/[id]/page.tsx` | resolve `activeTemplate().views.peopleList` / `views.peopleDetail` | ☐ |

## P1 — nice to have

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-PPL-090 | Interactions log + last contact | Functional | P1 | POST interactions on two dates | `last_contact_on` = latest occurred_on, shown on card | ☐ [P1] |
| TC-PPL-093 | Delete a mis-logged interaction | Functional | P1 | DELETE own interaction; DELETE another user's | 204 and no longer counted in last contact; 404 | ☐ [P1] |
| TC-PPL-091 | Avatar via mediaapi | Functional | P1 | PATCH and POST avatar_asset_id (ready image, owned) | accepted; else 422 `people/invalid-asset`; null clears | ☐ [P1] |
| TC-PPL-092 | media:asset_deleted NULLs avatar | Integration | P1 | delete avatar asset media-side | matching avatar_asset_id → NULL (soft cascade); initials Avatar renders | ☐ [P1] |

## Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-PPL-110 | All non-2xx RFC-7807 | Contract | P0 | error paths | Problem+json + stable type | ☐ (CC-1) |
| TC-PPL-111 | Problem types have i18n keys | Contract | P1 | grep problems.ts | all people types present, incl. `people/already-in-registry` | ☐ (CC-1) |
| TC-PPL-112 | Permission seeding | AuthZ | P0 | inspect grants | `people:read/write/delete:own` seeded → `user` | ☐ (CC-2) |
| TC-PPL-113 | birthday_upcoming reaches stream | Integration | P0 | run scan | event projects a stream item (keyed on notice_id) | ☐ (CC-5) |
| TC-PPL-114 | Migration up/down | Contract | P1 | migrate + down `people_persons` | clean; CHECKs (month/day together, year needs month); notices table; identity-anchor FK | ☐ |
