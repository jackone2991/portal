# Asynq Events & Tasks Registry

**Status:** current · **Last verified:** 2026-07-10

Cross-module coupling happens **only** through this bus (hard rule). Naming:
`<module>:<event_or_task>`; the emitting/owning module is the prefix. Two kinds:

- **Task** — a work item one module enqueues for its own worker (implementation
  detail, listed for grep-ability).
- **Event** — a fact announced to whoever cares (the integration surface, and the
  raw material of the life stream per [ADR-08](../adr/08-life-os-pivot.md)).

Adding a name here is part of a spec/PR's definition of done; MODULES.md §5 owns
the naming *rules*, this file owns the *inventory*.

## Events

| Name | Payload (sketch) | Emitter | Status | Consumers |
|---|---|---|---|---|
| `media:asset_ready` | `{asset_id, kind, owner_user_id, title, origin: 'upload'\|'import'}` | media | live — emitter only (SPEC-01 P1.2); no consumer yet | notify — in-app notification, skips `origin='import'` (SPEC-04 P0.4); stream projection, skips `origin='import'` (SPEC-06 P0.1). `origin` exists so a SPEC-02 zip import (≤300 assets) can't flood the bell/stream — the meaningful signal there is `comic:chapter_published` |
| `media:asset_deleted` | `{asset_id, owner_user_id}` | media | live — 1 consumer (comic P0.6) | **comic — drop dangling pages / null covers (SPEC-02 P0.6, live)**; stream — remove **all** media-sourced items with this ref (SPEC-06 P0.1); journal — strip attachment ids (SPEC-05 P1.5); people — null avatars (SPEC-08 P1.7); movie/music/story later |
| `media:playback_completed` | `{asset_id, user_id, title}` | media | live — emitter only (SPEC-07 P1.5); no consumer yet | stream (SPEC-06); notify (SPEC-04 open type registry) |
| `comic:chapter_published` | `{comic_id, chapter_id, owner_user_id, title}` | comic | planned (SPEC-02 P1.9) | stream (SPEC-06 P0.1); notify later |
| `bank:transaction_created` | `{transaction_id, user_id, account_id, amount, direction, category_id, occurred_at, is_transfer, transfer_id, counterparty_account_id}` | bank | live — emitter only (SPEC-03 P0.7); no consumer yet | stream (SPEC-06 P0.1) |
| `bank:transaction_updated` | same as created | bank | live — emitter only (SPEC-03 P0.7); no consumer yet | stream — refresh the matching item's payload/occurred_at (SPEC-06 P0.1) |
| `bank:transaction_deleted` | same as created | bank | live — emitter only (SPEC-03 P0.7); no consumer yet | stream — remove item (SPEC-06 P0.1) |
| `bank:budget_exceeded` | `{user_id, category_id, month}` | bank | planned (SPEC-03 P1.12) | notify later |
| `journal:entry_created` | `{entry_id, user_id, occurred_at}` | journal | planned (SPEC-05 P0.3) | — emit-only for future external consumers. The stream projection is maintained **transactionally in-module** (SPEC-06 P0.1), not via this event; no updated/deleted events for the same reason (SPEC-05 P0.3) |
| `people:birthday_upcoming` | `{notice_id, person_id, user_id, display_name, days_until}` | people | live — emitter only (SPEC-08 P0.4); no consumer yet | stream — `ref_id = notice_id`, so recurring years/thresholds never collide (SPEC-06 P0.1); notify (SPEC-04) as they land |
| `ops:backup_completed` | `{run_id, size_bytes}` | ops | live — emitter only (SPEC-09 P0.2); no consumer yet | — (audit + `/ops/status` today; notify later) |
| `ops:backup_failed` | `{run_id, error}` | ops | live — emitter only (SPEC-09 P0.2); no consumer yet | — (same) |
| `ops:export_ready` | `{export_id, user_id}` | ops | planned (SPEC-09 P1.7) | notify later |

## Tasks

| Name | Payload | Owner | Status |
|---|---|---|---|
| `media:transcode` | `{asset_id, source_key, output_key}` | media | live (video → HLS; "heavy" queue) |
| `media:process_image` | `{asset_id, source_key}` | media | live (SPEC-01 P0.1; image → WebP variants; "heavy" queue) |
| `media:thumbnail` | `{asset_id, source_key}` | media | live (SPEC-01 P0.2; video → poster variant; "thumbnail" queue) |
| `media:purge_orphans` | — (janitor sweep) | media | live (SPEC-01 P0.3; hourly on the shared scheduler) |
| `comic:import_zip` | `{chapter_id, upload_ref}` | comic | planned (SPEC-02 P1.7) |
| `comic:on_asset_deleted` | `{asset_id, owner_user_id}` (consumer; subscribes to `media:asset_deleted`) | comic | live (SPEC-02 P0.6; reaps dangling pages + NULL covers) |
| `notify:dispatch` | `{user_id, type, title, body, data, channels?, dedup_key?}` | notify | live (SPEC-04 P0.2; "default" queue) |
| `notify:email` | `{user_id, type, title, data}` (template rendered in handler) | notify | live (SPEC-04 P0.3; "default" queue) |
| `notify:web_push` | `{user_id, type, data}` | notify | planned (SPEC-04 P1.1; handler is a registered stub) |
| `notify:on_asset_ready` | `{asset_id, kind, owner_user_id, title, origin}` (consumer; subscribes to `media:asset_ready`) | notify | live (SPEC-04 P0.4) |
| `notify:purge_old` | — (janitor sweep) | notify | planned (SPEC-04 P2; handler is a registered stub, unscheduled) |
| `journal:backfill_stream` | — (one-shot stream seed, via `mediaapi`) | journal | planned (SPEC-06 P1.6) |
| `journal:stream_*` | consumer tasks (asset_ready/playback_completed/asset_deleted, bank transaction_{created,updated,deleted}, birthday, comic_published) | journal | live (SPEC-06 P0.1b; journal owns `stream_items` and projects every producer event — `media:asset_deleted` now fans out to BOTH comic reap + stream removal) |
| `people:scan_birthdays` | — (daily scan, instance-TZ) | people | live (SPEC-08 P0.4; daily 06:00 UTC on the shared scheduler; "default" queue) |
| `ops:backup_database` | — (nightly pg_dump → storage) | ops | live (SPEC-09 P0.2; nightly 03:00 UTC on the shared scheduler; "default" queue) |
| `ops:takeout` | `{export_id, user_id}` | ops | planned (SPEC-09 P1.7) |
| `ops:purge_exports` | — (nightly janitor: expire takeout archives > 7 d) | ops | planned (SPEC-09 P1.7) |

## Reserved namespaces

- **`notify:*`** — owned by the notification module specced in SPEC-04 (task rows
  above; MODULES.md §5.2 reserved the prefix). No other module may emit or consume
  names in this space; `account`'s legacy `RegisterTasks` stub is **removed** by
  SPEC-04 P0.2 — producers enqueue via `notify/api`. Known far-future names:
  `notify:loan_due` (feature-inventory §8.4), `ops:job_dead` (SPEC-09 P2 —
  dead-letter archive → notify).

## Delivery mechanics (multi-consumer fan-out)

Asynq is a task queue, not pub/sub: a task type is handled by **exactly one**
registered handler (`ServeMux` panics on duplicate registration), and one
enqueued task is processed once. Two modules can therefore never both
"subscribe to `media:asset_ready`" directly — the moment an event gains a
second consumer (SPEC-06's stream joins SPEC-04's notify), naive
subscribe-by-task-type either panics `cmd/worker` at startup or starves one
consumer.

**The convention:** events are published through a small `platform/events`
helper. `Publish(ctx, name, payload)` enqueues **one task per subscriber**;
the subscription table (event name → consumer task type, e.g.
`media:asset_ready → notify:on_asset_ready` + `journal:stream_ingest`) is
registered in the **wiring layer** (`cmd/worker`), mirroring the sanctioned
`Engine()` composition pattern — emitters never import consumers, consumers
handle only their own task types. While an event has a single consumer,
direct task-type handling is behaviorally identical; new specs should still
publish via the helper so adding consumer #2 is a wiring change, not a
migration of the producer.

## Conventions

- Events are **facts, past tense** (`transaction_created`), tasks are
  **imperatives** (`process_image`).
- Payloads carry IDs + the minimum for a consumer to render/decide; consumers fetch
  details through the owning module's `api/` package — payloads are not documents.
- Emitting with zero consumers is normal and encouraged (life-stream groundwork);
  consuming another module's event never entitles importing its internals.
- Privacy note: whether `bank:*` amounts surface in any UI is the **consumer's**
  decision (SPEC-03 open question) — the payload carrying an amount is not
  permission to display it.
