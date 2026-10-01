# Asynq Events & Tasks Registry

**Status:** current · **Last verified:** 2026-09-30 (`bank:scan_debts_due` checked against `cmd/worker/main.go` + `bank/debts.go` at HEAD; rows otherwise re-derived 2026-09-19 from the `Subscribe(` and `scheduler.Register(` calls in `cmd/api/main.go` + `cmd/worker/main.go` — `grep -n 'Subscribe(' backend/cmd/*/main.go` is the source; this table is the copy)

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
| `media:asset_ready` | `{asset_id, kind, owner_user_id, title, origin: 'upload'\|'import'}` | media | live — **1 consumer** | notify — in-app notification only for `kind='video'` with `origin='upload'` (SPEC-04 P0.4; images and imports are skipped — *the shipped handler still skips only `origin='import'`*). **Not projected into the stream** since `0033` (2026-08-28): an upload finishing is a library event, not a moment in the day. `origin` exists so a SPEC-02 zip import (thousands of assets — the whole-comic cap is `importMaxEntries` in `comic/import.go`, 100,000 as of 2026-09; a 9,129-image archive is verified) can't flood the bell |
| `media:asset_deleted` | `{asset_id, owner_user_id}` | media | live — **5 consumers** | comic — drop dangling pages / null covers (SPEC-02 P0.6); movie, music, story — null the asset references (`*:on_asset_deleted`; SPEC-15 P0.5, SPEC-14 P0.4, SPEC-16 P0.6) — *code follow-up: these four consumers run with no tenant scope, so under `portal_app` FORCE RLS refuses their UPDATE/DELETE (SPEC-02 §11 row 18, SPEC-14 §12 row 1, SPEC-15 §11 row 1, SPEC-16 §11 row 1)*; journal — `journal:stream_asset_deleted` removes every media-sourced stream item with this ref (SPEC-06 P0.1) **and strips the id from `asset_ids` of every Entry of the owner that showed it** (SPEC-12 T4, #13), both inside the owner's tenant scope, idempotent. Not yet: people avatars (SPEC-08 P1.7); bank (planned, P1.10) — `bank:on_asset_deleted` NULLs `bank_transactions.receipt_asset_id` (SPEC-03 P1.10) |
| `media:playback_completed` | `{asset_id, user_id, title, completed_at}` — `completed_at` is the latched value from SPEC-07 P1.5's step-2 UPDATE (`RETURNING completed_at`); the field is specced, *code follow-up: the shipped payload is still `{asset_id, user_id, title}`* | media | live — **1 consumer** (stream) | stream — `journal:stream_playback_completed` uses `completed_at` as the item's `occurred_at` (SPEC-06). Planned: notify, once a `notify:on_*` task is specced in SPEC-04 §7 |
| `comic:published` | `{comic_id, owner_user_id, title, chapter_count}` | comic | live (SPEC-02 P1.9) — emitted **once per publish action**, never per chapter | notify — `notify:on_comic_published`, one bell entry with a click-through, `dedup_key = comic_id:chapter_count` (a re-publish of an unchanged comic is silent). **Not projected into the stream** since `0034` (2026-08-28), which also replaced the per-chapter `comic:chapter_published` |
| `comic:chapter_deleted` | `{comic_id, chapter_id, owner_user_id}` | comic | live (SPEC-02 P1.9) — emitted per chapter on chapter/comic delete | **none** since `0034` removed the comic stream projection; emit-only |
| `bank:transaction_created` | `{transaction_id, user_id, account_id, amount, currency, direction, category_id, occurred_at, is_transfer, transfer_id, counterparty_account_id}` — `currency` is specced (SPEC-03 P0.7), not yet emitted | bank | live — **1 consumer** (stream) | stream — `journal:stream_bank_created` |
| `bank:transaction_updated` | same as created | bank | live — **1 consumer** (stream) | stream — `journal:stream_bank_updated` refreshes the matching item's payload/occurred_at |
| `bank:transaction_deleted` | same as created | bank | live — **1 consumer** (stream) | stream — `journal:stream_bank_deleted` removes the item |
| `movie:published` | `{movie_id, owner_user_id, title}` | movie | live — **1 consumer** (*code follow-up: published before the request transaction commits — SPEC-15 §11 row 3*) | notify — `notify:on_movie_published` (bell). Stream projection removed in `0040` (2026-08-28) |
| `music:track_published` | `{track_id, owner_user_id, title}` | music | live — **1 consumer**; emitted by publish and by bulk status (*code follow-up: published before the request transaction commits — SPEC-14 §12 row 4*) | notify — `notify:on_track_published` (bell). Stream projection removed in `0040` |
| `story:published` | `{story_id, owner_user_id, title}` | story | live — **1 consumer** (*code follow-up: published before the request transaction commits — SPEC-16 §11 row 4*) | notify — `notify:on_story_published` (bell). Stream projection removed in `0040` |
| `social:connection_requested` / `social:connection_accepted` | `{connection_id, requester_id, addressee_id, requester_name, addressee_name}` (`socialapi.ConnectionEvent`; notify builds the bell title from the names) | social | live (0037) — **1 consumer** each; both `Subscribe` calls in `cmd/api` only, the sole emitter (*code follow-up: published before the request transaction commits — SPEC-17 §11 row 1*) | notify — `notify:on_connection_requested` / `notify:on_connection_accepted` |
| `bank:budget_exceeded` | `{user_id, category_id, month}` | bank | planned (SPEC-03 P1.12) | notify later |
| `bank:import_completed` | `{import_batch_id, user_id, account_id, row_count}` | bank | planned (SPEC-10 phase 7) | none — emit-only; **not projected by the stream** |
| `journal:entry_created` | `{entry_id, user_id, occurred_at}` | journal | planned (SPEC-05 P0.3) | — emit-only for future external consumers. The stream projection is maintained **transactionally in-module** (SPEC-06 P0.1), not via this event; no updated/deleted events for the same reason (SPEC-05 P0.3) |
| `people:birthday_upcoming` | `{notice_id, person_id, user_id, display_name, days_until}` | people | live — **1 consumer** (stream) | stream — `journal:stream_birthday`; `ref_id = notice_id`, so recurring years/thresholds never collide (SPEC-06 P0.1). Planned: notify, once a `notify:on_*` task is specced in SPEC-04 §7 |
| `people:birthday_notice_revoked` | `{notice_id, person_id, user_id}` | people | planned (SPEC-08 P0.2) — per cleared, already-emitted notice on a birthday edit | stream — `journal:stream_birthday_revoked` (planned) deletes the item with `ref_id = notice_id` (SPEC-06 P0.1) |
| `people:person_deleted` | `{person_id, user_id}` | people | planned (SPEC-08 P0.2, §7 DELETE) | stream — `journal:stream_person_deleted` (planned) deletes every `people` item for that `person_id` (SPEC-06 P0.1) |
| `ops:backup_completed` | `{run_id, size_bytes}` | ops | live — emitter only (SPEC-09 P0.2); no consumer yet | — (audit + `/ops/status` today; notify later) |
| `ops:backup_failed` | `{run_id, error}` | ops | live — emitter only (SPEC-09 P0.2); its consumer is not built yet | notify — `notify:on_backup_failed` (SPEC-09 P0.6; planned) |
| `ops:export_ready` | `{export_id, user_id}` | ops | planned (SPEC-09 P1.7) | notify later |
| `account:user_registered` | `accountapi.AdminEvent` — `{event_id, occurred_at, actor_id\|null, actor_name, user_id, user_email, user_name}` + `approval_status` | account | planned (SPEC-13 P1.3; Decision 2026-10-01b (D3)) — published from `cmd/api` after the registration commits | notify — `notify:on_user_registered`: `account.registration_pending` to superadmins except the registrant, `dedup_key = registration:<user_id>` (the approver dispatch's key, so nobody gets it twice); none for the founder (`approved`) — SPEC-04 P1.5 |
| `account:user_approval_decided` | `accountapi.AdminEvent` — `{event_id, occurred_at, actor_id\|null, actor_name, user_id, user_email, user_name}` + `decision` (`approved`\|`rejected`\|`approval_revoked`), `note` | account | planned (SPEC-13 P1.3) | notify — `notify:on_user_approval_decided` (`account.approval_decided`, superadmins except the actor, `dedup_key = event_id`) |
| `account:user_access_changed` | `accountapi.AdminEvent` — `{event_id, occurred_at, actor_id\|null, actor_name, user_id, user_email, user_name}` + `access` (`disabled`\|`enabled`) | account | planned (SPEC-13 P1.3) | notify — `notify:on_user_access_changed` (`account.user_access_changed`, as above) |
| `account:user_deleted` | `accountapi.AdminEvent` — `{event_id, occurred_at, actor_id\|null, actor_name, user_id, user_email, user_name}` (user fields read before the delete) | account | planned (SPEC-13 P1.3) — only after the delete commits; a refused delete (P0.10) publishes none | notify — `notify:on_user_deleted` (`account.user_deleted`, as above) |
| `account:user_roles_changed` | `accountapi.AdminEvent` — `{event_id, occurred_at, actor_id\|null, actor_name, user_id, user_email, user_name}` + `roles_before`, `roles_after` | account | planned (SPEC-13 P1.3) | notify — `notify:on_user_roles_changed` (`account.user_roles_changed`, as above) |
| `account:role_changed` | `accountapi.AdminEvent` — `{event_id, occurred_at, actor_id, actor_name}` + `role_id`, `role_code`, `change` (`created`\|`updated`\|`deleted`\|`permissions_changed`) | account | planned (SPEC-13 P1.3) | notify — `notify:on_role_changed` (`account.role_changed`, link `/admin/roles`, as above) |
| `account:refresh_reuse_detected` | `accountapi.AdminEvent` — `{event_id, occurred_at, actor_id\|null, actor_name, user_id, user_email, user_name}` with `actor_id = null` | account | planned (SPEC-13 P1.3) — after the chain is revoked | notify — `notify:on_refresh_reuse_detected` (`account.refresh_reuse_detected`, every superadmin, in-app + email override). The affected user's own alert is SPEC-04 P1.4's `account.security_alert` |
| `layout:changed` | `layoutapi.ChangedEvent` — `{event_id, occurred_at, part: 'menu'\|'widgets', actor_id, actor_name, count}` | layout | planned (SPEC-18 P1.4; Decision 2026-10-01b (D3)) — published from `cmd/api` after the save commits | notify — `notify:on_layout_changed` (`layout.changed`, link `/admin/layout`, superadmins except the saver, `dedup_key = event_id`) |

`account` and `layout` emit nothing on `HEAD`; the eight planned rows above are
the owner's decision of 2026-10-01 (specs README, "Decisions recorded 2026-10-01
(second round)", D3) that neither is exempt from ADR-08's "≥ 1 event" rule
(SPEC-13 §8, SPEC-18 §8; code follow-ups SPEC-13 §11 row 24, SPEC-18 §11
row 14). Each is subscribed in `cmd/api`, the only binary that emits it.

## Tasks

| Name | Payload | Owner | Status |
|---|---|---|---|
| `media:transcode` | `{asset_id, source_key, output_key, owner_user_id}` | media | live (video → HLS; `heavy` queue, concurrency 1) |
| `media:process_image` | `{asset_id, source_key, owner_user_id}` | media | live (SPEC-01 P0.1; image → WebP variants; **its own `image` queue on its own server**, concurrency `IMAGE_CONCURRENCY` — NOT the heavy pool) |
| `media:thumbnail` | `{asset_id, source_key, owner_user_id}` | media | live (SPEC-01 P0.2; video → poster variant; "thumbnail" queue) |
| `media:purge_orphans` | — (janitor sweep) | media | live (SPEC-01 P0.3; hourly on the shared scheduler; runs **once per tenant** via `forEachTenant` since the RLS cutover) |
| `comic:import_zip` | `{import_id}` | comic | live (SPEC-02 P1.7; **never heavy** — it polls the asset statuses its own `media:process_image` tasks produce and must not occupy a slot they need. Target queue: the dedicated `bulk` queue on its own server (SPEC-04 P0.2 step 4); shipped on `default` with a 12 h timeout) |
| `comic:on_asset_deleted` | `{asset_id, owner_user_id}` (consumer; subscribes to `media:asset_deleted`) | comic | live (SPEC-02 P0.6; reaps dangling pages + NULL covers; *code follow-up: runs with no tenant scope — SPEC-02 §11 row 18*) |
| `movie:on_asset_deleted` | `{asset_id, owner_user_id}` (consumer; subscribes to `media:asset_deleted`) | movie | live (nulls dangling video/poster refs; a reaped video also returns the movie to `draft` — SPEC-15 P0.5; *code follow-up: runs with no tenant scope — SPEC-15 §11 row 1*) |
| `music:on_asset_deleted` | `{asset_id, owner_user_id}` (consumer; subscribes to `media:asset_deleted`) | music | live (nulls dangling audio/cover refs; a reaped audio asset also returns the track to `draft` — SPEC-14 P0.4; *code follow-up: runs with no tenant scope — SPEC-14 §12 row 1*) |
| `music:import_zip` | `{import_id, owner_id}` | music | live (`0038`, SPEC-14 P0.6; light server, `default` queue, `Timeout(6h)`, `MaxRetry(0)` — a retry would re-create every imported track; opens the owner's tenant scope per entry. Target queue: the `bulk` server — SPEC-14 P1.3) |
| `music:enrich_track` | `{track_id, owner_id}` | music | live (SPEC-14 P0.7; ffprobe tags + embedded cover, fills gaps only; light server, `default` queue, `Timeout(10m)`, `MaxRetry(1)`) |
| `music:lookup_track` | `{track_id, owner_id}` | music | live (`0039`, SPEC-14 P0.8; MusicBrainz + Cover Art Archive, off unless `MUSICBRAINZ_ENABLED` and `MUSICBRAINZ_CONTACT`; light server, `default` queue, `Timeout(5m)`, `MaxRetry(2)`) |
| `story:on_asset_deleted` | `{asset_id, owner_user_id}` (consumer; subscribes to `media:asset_deleted`) | story | live (nulls dangling cover refs; *code follow-up: runs with no tenant scope — SPEC-16 §11 row 1*) |
| `bank:on_asset_deleted` | `{asset_id, owner_user_id}` (consumer; subscribes to `media:asset_deleted`) | bank | planned (SPEC-03 P1.10; NULLs `bank_transactions.receipt_asset_id`) |
| `bank:scan_debts_due` | — (hourly sweep; days counted from each owner's local date in `users.timezone` — specs README Timezone; leads 7 / 1 / 0 days before `due_on`; each due debt enqueues `notify:dispatch` with type `bank.debt_due`, `dedup_key = <debt_id>\|<due_on>\|<lead>`, at most once per (debt, due date, lead)) | bank | live (SPEC-10 phase 1; specced hourly — *code follow-up: HEAD registers it daily at 07:00 UTC and counts days from the UTC date*; shared scheduler in `cmd/worker`; `default` queue; runs **once per tenant** via `forEachTenant`) |
| `notify:dispatch` | `{user_id, type, title, body, data, channels?, dedup_key?}` | notify | live (SPEC-04 P0.2; "default" queue) |
| `notify:email` | `{user_id, type, title, data}` (template rendered in handler) | notify | live (SPEC-04 P0.3; "default" queue) |
| `notify:web_push` | `{user_id, type, data}` | notify | planned (SPEC-04 P1.1; handler is a registered stub) |
| `notify:on_asset_ready` | `{asset_id, kind, owner_user_id, title, origin}` (consumer; subscribes to `media:asset_ready`) | notify | live (SPEC-04 P0.4) |
| `notify:on_comic_published` | `{comic_id, owner_user_id, title, chapter_count}` (consumer; subscribes to `comic:published`; `dedup_key = comic_id:chapter_count`) | notify | live (SPEC-04 §7, SPEC-02 P1.9) |
| `notify:on_movie_published` | `{movie_id, owner_user_id, title}` (consumer; subscribes to `movie:published`; `dedup_key` = movie id) | notify | live |
| `notify:on_track_published` | `{track_id, owner_user_id, title}` (consumer; subscribes to `music:track_published`; `dedup_key` = track id) | notify | live |
| `notify:on_story_published` | `{story_id, owner_user_id, title}` (consumer; subscribes to `story:published`; `dedup_key` = story id) | notify | live |
| `notify:on_connection_requested` / `notify:on_connection_accepted` | `{connection_id, requester_id, addressee_id, requester_name, addressee_name}` (consumers; subscribe to `social:connection_*`; `dedup_key = connection_id:phase`) | notify | live (0037) |
| `notify:on_user_registered` | the `account:user_registered` payload (consumer; subscribes to `account:user_registered`; type `account.registration_pending`, `dedup_key = registration:<user_id>`; `dedup_key` = `event_id` unless stated) | notify | planned (SPEC-04 P1.5; Decision 2026-10-01b (D3); light server, `default` queue; fans out to `SuperadminIDs` minus the actor) |
| `notify:on_user_approval_decided` | the `account:user_approval_decided` payload (consumer; subscribes to `account:user_approval_decided`; type `account.approval_decided`; `dedup_key` = `event_id` unless stated) | notify | planned (SPEC-04 P1.5; Decision 2026-10-01b (D3); light server, `default` queue; fans out to `SuperadminIDs` minus the actor) |
| `notify:on_user_access_changed` | the `account:user_access_changed` payload (consumer; subscribes to `account:user_access_changed`; type `account.user_access_changed`; `dedup_key` = `event_id` unless stated) | notify | planned (SPEC-04 P1.5; Decision 2026-10-01b (D3); light server, `default` queue; fans out to `SuperadminIDs` minus the actor) |
| `notify:on_user_deleted` | the `account:user_deleted` payload (consumer; subscribes to `account:user_deleted`; type `account.user_deleted`; `dedup_key` = `event_id` unless stated) | notify | planned (SPEC-04 P1.5; Decision 2026-10-01b (D3); light server, `default` queue; fans out to `SuperadminIDs` minus the actor) |
| `notify:on_user_roles_changed` | the `account:user_roles_changed` payload (consumer; subscribes to `account:user_roles_changed`; type `account.user_roles_changed`; `dedup_key` = `event_id` unless stated) | notify | planned (SPEC-04 P1.5; Decision 2026-10-01b (D3); light server, `default` queue; fans out to `SuperadminIDs` minus the actor) |
| `notify:on_role_changed` | the `account:role_changed` payload (consumer; subscribes to `account:role_changed`; type `account.role_changed`; `dedup_key` = `event_id` unless stated) | notify | planned (SPEC-04 P1.5; Decision 2026-10-01b (D3); light server, `default` queue; fans out to `SuperadminIDs` minus the actor) |
| `notify:on_refresh_reuse_detected` | the `account:refresh_reuse_detected` payload (consumer; subscribes to `account:refresh_reuse_detected`; type `account.refresh_reuse_detected`, in-app + email; `dedup_key` = `event_id` unless stated) | notify | planned (SPEC-04 P1.5; Decision 2026-10-01b (D3); light server, `default` queue; fans out to `SuperadminIDs` minus the actor) |
| `notify:on_layout_changed` | the `layout:changed` payload (consumer; subscribes to `layout:changed`; type `layout.changed`; `dedup_key` = `event_id` unless stated) | notify | planned (SPEC-04 P1.5; Decision 2026-10-01b (D3); light server, `default` queue; fans out to `SuperadminIDs` minus the actor) |
| `notify:on_backup_failed` | `{run_id, error}` (consumer; subscribes to `ops:backup_failed`, subscription in `cmd/worker`) | notify | planned (SPEC-09 P0.6 owns it; "default" queue; unbuilt) |
| `notify:purge_old` | — (janitor sweep) | notify | planned (SPEC-04 P2; handler is a registered stub, unscheduled) |
| `journal:backfill_stream` | — | journal | **retired** (SPEC-06 P1.6): it seeded `media:asset_ready` rows, which the stream no longer projects (`0033`) |
| `journal:stream_*` | consumer tasks — `playback_completed`, `asset_deleted`, `bank_{created,updated,deleted}`, `birthday` (`journal:stream_birthday`). Planned retractions (SPEC-06 P0.1(b)): `journal:stream_birthday_revoked` (consumes `people:birthday_notice_revoked`) and `journal:stream_person_deleted` (consumes `people:person_deleted`) | journal | live (SPEC-06 P0.1(b); journal owns `stream_items`). The stream projects **moments** (a playback finished, money moved, a birthday is near), not **library events**: `asset_ready`, `comic_published/deleted` and the three catalogue publishes were dropped in `0033`/`0034`/`0040` and go to the bell instead |
| `people:scan_birthdays` | — (hourly scan; each owner's `days_until` evaluated in that owner's `users.timezone` — SPEC-08 P0.3, specs README Timezone) | people | live (SPEC-08 P0.4; specced hourly at minute 5 — *code follow-up: HEAD registers it daily at 06:00 UTC and evaluates in UTC*; `default` queue; runs **once per tenant** via `forEachTenant` since the RLS cutover) |
| `account:purge_refresh_tokens` | — (janitor sweep: hard-deletes expired refresh tokens) | account | live (`@every 24h` on the shared scheduler, `default` queue; wired directly in `cmd/worker` against the account repository, since the account module is not constructed in the worker — SPEC-13 P0.15) |
| `account:purge_reset_tokens` | — (janitor sweep: hard-deletes long-expired password-reset tokens) | account | planned (SPEC-13 P0.15; the query `PurgeExpiredPasswordResetTokens` exists, nothing schedules it — SPEC-13 §11 row 20) |
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
`media:asset_deleted → comic:on_asset_deleted` + `journal:stream_asset_deleted`) is
registered in the **wiring layer** of every binary that emits the event
(`cmd/api/main.go` for HTTP-emitted events, `cmd/worker/main.go` for
worker-emitted ones — see "What is NOT enforced"), mirroring the sanctioned
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

## What is NOT enforced

`platform/events.Subscribe(eventName, consumerTaskType string, …)` takes **two
bare strings**, and `Publish(ctx, name string, payload any)` takes `any`. Nothing
relates an event name to a payload type, and nothing relates a consumer task to
a handler that can decode it. Concretely:

- A **typo in either argument** is a runtime-silent no-op — `Publish` on an
  unregistered name returns nil by design.
- **Renaming a payload field breaks consumers silently.** `encoding/json` ignores
  unknown fields and zeroes missing ones, so a consumer keeps succeeding with
  wrong data. `media:asset_ready`'s `origin` is the sharp case: it is the
  flood-guard that stops a comic zip import (thousands of assets —
  `importMaxEntries` in `comic/import.go`) emitting one bell notification per
  page. Rename it and the guard stops firing, with no error.
- The **subscription table is per-binary and hand-duplicated** —
  `cmd/api/main.go` and `cmd/worker/main.go` each restate it. `Subscribe` is
  idempotent so divergence is not fatal, but it is invisible.

That is why this file has to be updated by hand, and why it had drifted on eight
rows before 2026-08-25. Until the payloads are typed at the emitting module's
`api/` package, **this table is the only contract** — treat updating it as part
of a PR's definition of done, not as documentation.
