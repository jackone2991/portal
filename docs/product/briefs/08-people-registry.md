# 08 — People Registry (contacts + birthdays, n=1)

**Module:** `people` (new — not scaffolded) · **Effort:** ~4.5 days · **Depends on:** nothing hard; avatars want spec 01, birthday *delivery* wants SPEC-04 — both degrade gracefully.
**Unlocks:** "mom's birthday in 3 days" — the canonical life-stream example from brief 00; real data for the `BirthdayCard` widget; the social facet at n=1.
**Provenance:** Monica-CRM pattern (contacts as *data*, not user accounts) — researched 2026-07-10. **Spec:** [SPEC-08](../specs/SPEC-08-people-registry.md).

## Problem statement

Brief 00's flagship life-stream example — "mom's birthday in 3 days" — is impossible
today: nothing stores who "mom" is or when her birthday falls. The cataloged
"Events/birthdays" item (backlog §3 P2) assumes multi-user social events/RSVP, which
is parked behind real second users. Monica proves the loophole: **my relationships
are my data** — an owner-scoped address book needs no second account, no friend
graph, and it is the cheapest source of recurring, emotionally relevant events for
the stream, the bell, and the dormant `BirthdayCard`/`FriendCard` UI kits.

## Goals

- The owner records people (family, friends) with birthdays and contact notes.
- Upcoming birthdays surface as bus events days ahead, in the user's timezone.
- `BirthdayCard` and the people list render real rows, not fixtures.

## Non-goals

- **Not a friend graph.** People are rows, not accounts: no requests, no chat, no
  user search — the [04-deferred](04-deferred.md) friend-graph row stays parked and
  its re-entry condition (real second users) is neither met nor needed here.
- Full Monica parity (activities, gifts, debts-between-people) — P1 keeps one
  interactions log; the rest waits for demonstrated use.
- Syncing with CardDAV/Google Contacts — import only, later (P2).

## User stories

- As the owner, I add "Mẹ — sinh nhật 15/03" once, and every year the stream and
  bell warn me 3 days out and on the day.
- As the owner, I open a person and see their birthday, phone, and my notes about
  them ("thích trà ô long, không cà phê").
- As the owner, I log "called grandma" so next month I notice how long it's been.

## Requirements

### P0 — must have

1. **Module scaffold** per MODULES.md §8; migration `000N_people_persons` (next free
   number).
2. **CRUD** `POST/GET/PATCH/DELETE /api/v1/people` — RBAC `people:create` /
   `people:read:own` / `people:update:own` / `people:delete:own` (3-segment grammar).
   Birthday stores month/day with **optional year** (many people won't share a year).
   - [ ] Given person rows of user B, user A's list/fetch excludes them; direct
         fetch 404.
   - [ ] A birthday without a year renders and schedules correctly.
3. **`GET /api/v1/people/upcoming-birthdays?days=14`** — next occurrences computed in
   the user's timezone (D-17), sorted soonest-first; powers the `BirthdayCard` widget.
4. **Birthday scan**: daily periodic task `people:scan_birthdays` (shared periodic
   runner convention, SPEC-01 P0.3) emits `people:birthday_upcoming`
   `{person_id, user_id, display_name, days_until: 3|0}` — once per threshold per
   year (dedup table or deterministic key). Register in events.md. Consumers: stream
   (brief 06), notify (SPEC-04) as they land — emission is day-one regardless
   (ADR-08 rule).
   - [ ] Given a birthday 3 days out, exactly one 3-day event fires; day-of fires
         once; no repeat on task retry.
5. **Frontend**: `/people` list + person detail, reusing `FriendCard` /
   `PersonalInfoWidget` kits; `BirthdayCard` on home wired to the upcoming endpoint.

### P1 — nice to have

6. **Interactions log**: `people_interactions (person_id, kind met|called|messaged|gifted, occurred_on, note)`
   + "last contact" on the person card.
7. **Avatar** via `mediaapi` image asset (needs spec 01); falls back to the
   deterministic-hue initials `Avatar`.

### P2 — future considerations (design for, don't build)

8. vCard import (file upload → rows); no live sync.
9. Household sharing of selected people — arrives with tenant `kind: household`;
   keep `user_id` scoping clean so a tenant scope can layer on.

## Data model sketch

```
people_persons(
  id uuid pk, user_id uuid not null,
  display_name text not null check (char_length(display_name) between 1 and 120),
  relationship text,                        -- 'mẹ', 'bạn đại học', freeform
  birth_month int check (birth_month between 1 and 12),
  birth_day   int check (birth_day between 1 and 31),
  birth_year  int,                          -- nullable by design
  contact jsonb not null default '{}',      -- phones/emails/addresses, schemaless
  note_md text,
  avatar_asset_id uuid,                     -- media asset, validated via mediaapi (P1)
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
index (user_id, birth_month, birth_day)
```

## API sketch (add to `shared/openapi.yaml`)

```
POST   /api/v1/people                    {display_name, relationship?, birthday?, contact?, note_md?}
GET    /api/v1/people?cursor=
GET    /api/v1/people/{id}
PATCH  /api/v1/people/{id}
DELETE /api/v1/people/{id}
GET    /api/v1/people/upcoming-birthdays?days=
```

## Open questions

- **(product, non-blocking)** Lunar-calendar birthdays (giỗ, âm lịch) — real need in
  the owner's context; store a `calendar: solar|lunar` flag now or defer? Recommendation:
  add the column defaulted `solar` (cheap in migration #1), implement lunar math later.
- **(engineering, non-blocking)** Feb-29 birthdays: celebrate Feb-28 or Mar-1 in
  non-leap years — pick one in the scan task and test it.
