# Journal module

Owns the life-stream **write path** (SPEC-05): human-authored journal entries.

- Owner-scoped CRUD under `/api/v1/journal/entries` (create / list / fetch / patch / delete)
- Timeline orders + paginates on `(occurred_at DESC, id DESC)` — a backdated entry sits at its date
- Emits **`journal:entry_created`** (emit-only) after a create commits — no consumer required at v1

## Owns these tables

`journal_entries` (migration `0011_journal_entries`). Also owns SPEC-06's `stream_items` projection — a **later** migration; not shipped here.

## Boundaries

- `user_id` FKs into `users(id)` — the sanctioned identity-anchor exception (matches `0007`/`0009`).
- `asset_ids` carries **no FK** (cross-module); validated via `mediaapi` at P1.5, rejected with 422 `journal/invalid-asset` until then.
- Other modules import only `journal/api` (the `journal:entry_created` event contract). No synchronous call surface yet.

## Emits events

- `journal:entry_created` `{entry_id, user_id, occurred_at}` — **emit-only**. Deliberately no `entry_updated`/`entry_deleted`: SPEC-06's projection lives in the same module and is maintained transactionally, not via the bus (P0.3).

## Permissions

`journal:read:own`, `journal:write:own`, `journal:delete:own` — seeded to the base `user` role by `0011` (`write` covers create + update).

## Open work

SPEC-06's `stream_items` projection and `GET /stream` both shipped; the
projection now covers movie/music/story publishes too (wired 2026-08-25).
Genuinely open:

- **P1.5 photo attachments** — `asset_ids uuid[]` exists in `0011` and the
  handler deliberately 422s it. This is the highest-value P1 left in the repo:
  the column is there, the media pipeline is done, and it turns the journal from
  text-only into what `vision.md` describes.
- **P1.6 mood picker** — no preset-emoji component.
- **P1.5 on-this-day** — `GET /stream/memories` (SPEC-06:209) is not mounted.
- **`journal:backfill_stream`** (SPEC-06 P1.6) — the one-shot stream seed.
