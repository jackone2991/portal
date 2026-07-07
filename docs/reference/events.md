# Asynq Events & Tasks Registry

**Status:** current · **Last verified:** 2026-07-07

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
| `media:asset_ready` | `{asset_id, kind, owner_user_id, title}` | media | planned (SPEC-01 P1.2) | notification module (future life stream) |
| `comic:chapter_published` | `{comic_id, chapter_id, owner_user_id, title}` | comic | planned (SPEC-02 P1.9) | ditto |
| `bank:transaction_created` | `{transaction_id, user_id, account_id, amount, direction, category_id, occurred_at, is_transfer}` | bank | planned (SPEC-03 P0.7) | ditto |
| `bank:transaction_updated` | same as created | bank | planned (SPEC-03 P0.7) | ditto |
| `bank:transaction_deleted` | same as created | bank | planned (SPEC-03 P0.7) | ditto |
| `bank:budget_exceeded` | `{user_id, category_id, month}` | bank | planned (SPEC-03 P1.12) | ditto |

## Tasks

| Name | Payload | Owner | Status |
|---|---|---|---|
| `media:transcode` *(verify exact name in repo)* | `{asset_id}` | media | live (video → HLS) |
| `media:process_image` | `{asset_id}` | media | planned (SPEC-01 P0.1) |
| `media:purge_orphans` | — (janitor sweep) | media | planned (SPEC-01 P0.3) |
| `comic:import_zip` | `{chapter_id, upload_ref}` | comic | planned (SPEC-02 P1.7) |

## Reserved namespaces

- **`notify:*`** — reserved for the future notification module (MODULES.md §5.2).
  No other module may emit or consume names in this space; `account` already stubs
  `RegisterTasks` against it. Known planned name: `notify:loan_due`
  (feature-inventory §8.4, far future).

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
