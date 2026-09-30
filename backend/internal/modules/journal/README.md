# Journal module

Owns the life-stream **write path** (SPEC-05): human-authored journal entries.

- Owner-scoped CRUD under `/api/v1/journal/entries` (create / list / fetch / patch / delete)
- Timeline orders + paginates on `(occurred_at DESC, id DESC)` — a backdated entry sits at its date
- Emits **`journal:entry_created`** (emit-only) after a create commits — no consumer required at v1

## Owns these tables

`journal_entries` (migration `0011_journal_entries`; `0044` makes `asset_ids` live and relaxes the body CHECK to a length bound). Also owns SPEC-06's `stream_items` projection (`0017`).

## Talks to

- **media** — `GetAsset`, through the `MediaAPI` interface journal declares and `media.Module.API()` satisfies in `cmd/api`. An Entry's Attachments (`asset_ids`, SPEC-12) are validated as a whole before a write: at most ten, no duplicates, every id a `ready` image Asset the caller owns, else 422 `journal/invalid-asset` naming the id and the reason. The lookup runs inside the request's tenant transaction, so another tenant's Asset answers "not found" (ADR-07). The worker constructs the module without it; a nil lookup refuses any `asset_ids` rather than trusting them.

## Boundaries

- `user_id` FKs into `users(id)` — the sanctioned identity-anchor exception (matches `0007`/`0009`).
- `asset_ids` carries **no FK** (cross-module) — validated through media's public API on write (above). The only place that reads `assets` directly is migration `0044`'s backfill, which is a migration, not module code.
- An Entry is text, or at least one Attachment, or both — never neither. That is a **service** write rule (422 `journal/invalid-body`), not a CHECK, so a later consumer can strip the last Attachment from a photo-only Entry and leave it standing (SPEC-12 T4).
- The Location is three all-or-nothing columns (`location_name`/`location_lat`/`location_lon`, migration `0045`, SPEC-12 T3): a CHECK holds the shape (a non-empty trimmed name, lat in [−90, 90], lon in [−180, 180]) and the service refuses the same with 422 `journal/invalid-location` before the database can. On PATCH an object sets it, `null` clears it, absent keeps it. A Location alone does not make an Entry — it plays no part in the text-or-Attachment rule. Bodies are plain markdown for every row: `0044`/`0045` moved the photo and the place out of the old link forms and assert no body still carries one.
- Other modules import only `journal/api` (the `journal:entry_created` event contract). No synchronous call surface yet.

## Emits events

- `journal:entry_created` `{entry_id, user_id, occurred_at}` — **emit-only**. Deliberately no `entry_updated`/`entry_deleted`: SPEC-06's projection lives in the same module and is maintained transactionally, not via the bus (P0.3).

## Subscribes to

The life-stream projection consumers (SPEC-06 P0.1b; wired in `cmd/worker`): `media:playback_completed`, `bank:transaction_created` / `_updated` / `_deleted`, `people:birthday_upcoming` — each writes or removes a `stream_items` row inside the target user's tenant scope.

- `media:asset_deleted` `{asset_id, owner_user_id}` — removes the Asset's own stream card **and strips the id from `asset_ids` of every Entry of the owner that showed it** (SPEC-12 T4), so a card shows one photo fewer rather than a broken frame. Runs inside the owner's tenant scope (both tables are RLS-fenced; the worker's role errors on an unscoped touch), is idempotent (a redelivery finds no row still carrying the id), and keeps an Entry that ends up with neither text nor Attachment — the text-or-Attachment rule is for user writes, not for this consumer. An event without `owner_user_id` cannot be scoped and is dropped.

## Permissions

`journal:read:own`, `journal:write:own`, `journal:delete:own` — seeded to the base `user` role by `0011` (`write` covers create + update).

## Open work

None listed here on purpose. Implementation status has one written owner
(`/CLAUDE.md` § Current status) and open work one list
(`docs/product/backlog.md`) — ADR-11. A status claim in a module README was
wrong within weeks every time it was tried (the 2026-08-25 audit found seven
of eight sections stale).

