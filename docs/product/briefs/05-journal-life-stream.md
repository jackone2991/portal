# 05 — Journal (life-stream write path)

**Module:** `journal` (new — not scaffolded) · **Effort:** ~5–6 days · **Depends on:** nothing hard (ADR-10 codegen cutover preferred first; photo attachments reuse spec 01).
**Unlocks:** the life-stream surface (brief 06), the first real post type, deletion of the largest fixture block in the app.
**Provenance:** promotion of backlog §3 P1 "Posts/newsfeed API", reframed per [ADR-08](../../adr/08-life-os-pivot.md) — researched 2026-07-10. **Spec:** [SPEC-05](../specs/SPEC-05-journal.md).

## Problem statement

ADR-08 says the first real post type is **a journal / life event of the user**, not a
status for friends — but nothing specs it. SPEC-01/02/03 create three event
*producers* and SPEC-04's bell is their only consumer; the browsable timeline that
ADR-08 calls the product has no write path. Meanwhile `HomeView` renders a ~685-line
hard-coded newsfeed, and the ported `Composer`/`Post`/comment kits are exported but
imported by nothing. Every week without a capture surface is life-stream data lost
forever (the asymmetry: memories not captured can't be backfilled).

## Goals

- The owner can capture a journal entry (text + mood, later photos) in seconds, from `/`.
- Entries are owner-scoped rows in a real module with the standard layout (MODULES.md §8).
- `journal:entry_created` is on the bus from day one (ADR-08's day-one-event rule).
- The fake composer and fixture posts leave `HomeView`.

## Non-goals

- Comments, reactions, sharing — single user; entries are flat. (Multi-user social
  stays parked per [04-deferred](04-deferred.md).)
- The read-path timeline mixing journal + system events — that is brief 06.
- Rich-text editor; v1 is markdown-in-textarea with preview.
- Auto-entries from bus events — brief 06's projection owns system events; the
  journal table stores only human-authored rows (keeps `kind` honest).

## User stories

- As the owner, I jot "ăn tối với mẹ, vui" with a mood in <10 s from the home page,
  and it appears at the top of my stream.
- As the owner, I edit or delete an entry I regret — it's my journal, not a ledger.
- As the owner, I scroll back months and see what I wrote, newest first, fast.

## Requirements

### P0 — must have

1. **Module scaffold** per MODULES.md §8: `internal/modules/journal/`, own `sqlc.yaml`
   block, migration `000N_journal_entries` (next free number).
2. **Entries CRUD**: `POST/GET/PATCH/DELETE /api/v1/journal/entries`, cursor-paginated
   (`created_at DESC, id DESC`), RBAC `journal:create` / `journal:read:own` /
   `journal:update:own` / `journal:delete:own` (3-segment grammar — see spec 04 §7's
   note; do not copy the 4-segment style).
   - [ ] Given entries by user B, when user A lists/fetches, then B's rows are absent
         and direct fetch is 404.
   - [ ] Given 500 entries, cursor paging is stable and ordered.
3. **Emit `journal:entry_created`** `{entry_id, user_id, occurred_at}` on create
   (register in [events.md](../../reference/events.md) — definition of done).
4. **Composer wiring**: the home composer becomes real (TanStack mutation, optimistic
   insert per D-32), posting to the journal API; fixture posts and the fake composer
   are deleted from `HomeView`.
   - [ ] Grep test: the fixture post array is gone; every rendered entry is a DB row.

### P1 — nice to have

5. **Photo attachments**: `asset_ids uuid[]` validated via `mediaapi` (ready, image
   kind, owner) — needs spec 01 landed; render `thumb` variants in the entry card.
6. **Mood picker** surfaced in the composer (schema carries `mood` from P0).

### P2 — future considerations (design for, don't build)

7. Entry export rides the ops takeout (brief 09) — keep bodies plain markdown.
8. On-this-day / streaks read this table (brief 06) — don't denormalize dates away.

## Data model sketch

```
journal_entries(
  id uuid pk, user_id uuid not null,          -- identity-anchor FK per spec 04 §6 precedent
  body_md text not null check (char_length(body_md) between 1 and 20000),
  mood text,                                   -- freeform emoji/word, nullable
  asset_ids uuid[] not null default '{}',      -- media assets, validated via mediaapi (P1)
  occurred_at timestamptz not null default now(),  -- user-editable ("last night")
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
)
index (user_id, occurred_at desc, id desc)
```

## API sketch (add to `shared/openapi.yaml`)

```
POST   /api/v1/journal/entries            {body_md, mood?, occurred_at?}
GET    /api/v1/journal/entries?cursor=
PATCH  /api/v1/journal/entries/{id}       {body_md?, mood?, occurred_at?}
DELETE /api/v1/journal/entries/{id}
```

## Open questions

- **(product, non-blocking)** Module name `journal` vs a broader `stream` module that
  also owns brief 06's projection. Recommendation: one module named `journal` owning
  both tables — two micro-modules for one surface is boundary theater at this size.
- **(product, non-blocking)** Is `occurred_at` backdating unlimited? Recommendation:
  yes (it's a journal); the stream just orders by it.
