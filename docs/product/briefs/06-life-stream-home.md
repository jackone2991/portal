# 06 — Life-Stream Home (read path: projection + dashboard)

**Module:** `journal` (extends brief 05) + frontend home · **Effort:** ~6 days · **Depends on:** brief 05 (first content); system events arrive as SPEC-01 P1.2 / SPEC-04 / SPEC-03 land — every widget degrades to an empty state.
**Unlocks:** the ADR-08 proof screen — money + entertainment + notifications in one glance; resolves brief 00's open question about the newsfeed.
**Provenance:** merges three research candidates (stream projection, "Today" dashboard, Home-Assistant-pattern home) — researched 2026-07-10. **Spec:** [SPEC-06](../specs/SPEC-06-life-stream-home.md).

## Problem statement

Brief 00 deferred the question "does the life stream replace the newsfeed on `/` or
live alongside it?" until two event producers exist — brief 05 plus SPEC-01 P1.2
gets there. Today `/` is fixture data end to end: fake posts, fake widgets, fake
activity. ADR-08's thesis is that **integration beats one-app-per-domain**, and the
only place that thesis is visible is a home screen where the facets meet. If the
first screen stays fake, no individual module creates the daily habit.

**Decision this brief carries:** the life stream **replaces** the newsfeed on `/`
(no tab). The journal composer sits on top; the widget rail carries the facets.

## Goals

- `/` renders a real reverse-chron life stream: journal entries + system events.
- A `stream_items` projection captures bus events durably from day one (events not
  captured now are lost — same asymmetry as brief 05).
- The Olympus widget rail shows real data per facet, empty-state-safe.
- Zero fixture data left on the home route (extends brief 05's grep test).

## Non-goals

- Social feed mechanics (likes, comments, follows) — n=1.
- The bell/notification UX — that is SPEC-04 (the stream is a timeline, not an
  unread queue; the two share producers, not storage).
- Push/email digest delivery — P2 here, promotes SPEC-04's P2 seam.
- Weather widget — dropped, not wired (kills the backlog §3 P2 question).

## User stories

- As the owner, I open `/` and see today: what I wrote, what finished transcoding,
  what I spent — one timeline, newest first.
- As the owner, I glance at the rail: month-to-date spend, continue-watching,
  upcoming birthdays — without opening three apps.
- As the owner, a year from now, "on this day" resurfaces what I did today.

## Requirements

### P0 — must have

1. **Projection table** `stream_items` (journal module, brief 05's open question
   resolved as one-module): fed by an Asynq consumer subscribing to
   `journal:entry_created` + `media:asset_ready` (as SPEC-01 P1.2 lands), then
   `bank:transaction_created` / `comic:chapter_published` as their modules land.
   Dedup on `(source_module, event_type, ref_id)`.
   - [ ] Given an event replay (Asynq retry), then no duplicate stream item.
2. **`GET /api/v1/stream?cursor=`** — merged timeline (journal entries render full,
   system items render compact), RBAC `stream:read:own`.
3. **Home `/` replacement**: life stream + brief 05 composer on top; TanStack Query,
   RSC shell per D-33.
4. **Widget rail on real data** (each independent, empty-state-safe):
   `PersonalInfoWidget` ← `/auth/me` · ActivityFeed ← `GET /me/notifications`
   (SPEC-04) · finance month card ← `GET /bank/dashboard` (SPEC-03) · continue rail
   ← `GET /continue` (brief 07) · birthdays ← `GET /people/upcoming-birthdays`
   (brief 08).
   - [ ] With only account+media wired, the rail renders without errors or fixtures.

### P1 — nice to have

5. **On-this-day memories**: `GET /api/v1/stream/memories` — month/day anniversary
   matches in the user's timezone, grouped by years-ago; one `WidgetCard` widget.
6. **Backfill task** `journal:backfill_stream` — one-shot seeding of stream_items
   from existing assets (upload dates) so the stream isn't empty on day one.

### P2 — future considerations (design for, don't build)

7. **Daily digest** — promotes SPEC-04's P2 digest seam with a concrete consumer:
   7am rollup of yesterday's stream into one `digest.daily` notification (in-app +
   email via SPEC-04 channels). Needs SPEC-04 P0 + this projection; keep the
   watermark pattern in mind when shaping stream queries.
8. Life-stream privacy tiers (per-item visibility) arrive with household tenancy.

## Data model sketch

```
stream_items(
  id uuid pk, user_id uuid not null,
  source_module text not null,                -- 'journal' | 'media' | 'bank' | 'comic' | ...
  event_type text not null,                   -- registry name, e.g. 'media:asset_ready'
  ref_id uuid not null,                       -- the source row (entry, asset, transaction)
  payload jsonb not null default '{}',        -- render-minimum snapshot (title, amount, href)
  occurred_at timestamptz not null,
  unique (source_module, event_type, ref_id)
)
index (user_id, occurred_at desc, id desc)
```

Payloads follow events.md's rule: IDs + render-minimum, not documents; deep data is
fetched through the owning module's `api/` when a card needs more.

## API sketch (add to `shared/openapi.yaml`)

```
GET /api/v1/stream?cursor=
GET /api/v1/stream/memories          (P1)
```

## Open questions

- **(engineering, non-blocking)** Consume events directly (one more handler per
  task type) vs via SPEC-04's dispatch fan-out. Recommendation: direct subscription
  like SPEC-04 P0.4 does — the stream is a peer consumer, not a notification channel.
- **(product, non-blocking)** Do `bank:*` amounts render in the stream? Same privacy
  question as SPEC-03 §11 — the consumer decides; default to showing (n=1, own data),
  revisit at household tenancy.
