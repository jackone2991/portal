# notify module

The notification backbone (SPEC-04). One typed intent in → an in-app row plus
fan-out to every enabled channel. Producers never send mail/push themselves.

## Owns these tables

`notifications`, `notification_preferences`, `web_push_subscriptions` (migration
`0009_notify_notifications`). `user_id` FKs into account's `users(id)` — the one
sanctioned cross-module identity-anchor FK (§6, matches `0007_media_assets`).

## Public surface (`api/`)

`notifyapi.Enqueue(ctx, client, NotificationIntent)` — the ONLY way another
module reaches notify. Intent = `{user_id, type, title, body, data, channels?,
dedup_key?}`. `channels` is unioned with the stored per-type preference (so a
transactional send bypasses an email-off default); `dedup_key` makes a
redelivered dispatch a no-op against the in-app store.

## Talks to

- `platform/events` fan-out — subscribes to `media:asset_ready` via the
  `notify:on_asset_ready` consumer task (wired in `cmd/worker`).
- account, via the wiring layer's `UserResolver` adapter over `account/api`
  (recipient email for the email channel). notify never imports account internals.

## Asynq tasks (owns the `notify:*` prefix — MODULES.md §5.2)

| Task | Role |
|---|---|
| `notify:dispatch` | ingress fan-out (P0.2) |
| `notify:email` | email channel (P0.3) |
| `notify:web_push` | web push (P1.1 — registered stub) |
| `notify:on_asset_ready` | consumer of `media:asset_ready` (P0.4) |
| `notify:purge_old` | retention janitor (P2 — registered stub) |

## Notification `type` registry (open string, not an enum)

New types slot in with zero schema change. Live producers today:

| `type` | Producer | Persists in-app? | Mutable? | Channels |
|---|---|---|---|---|
| `media.asset_ready` | media consumer (P0.4) | yes | yes | in_app (email if opted in) |
| `account.password_reset` | account forgot-password (P0.3) | **no** | **no** | email only (override) |
| `account.security_alert` | account refresh-reuse (P1.4) | yes | **no** | email + in_app |

Non-mutable types ignore the `muted` switch (a user who muted resets must still
recover). `account.password_reset` never writes an in-app row — the reset link
rides the email task only (persisting it would defeat hashed-at-rest).

Future (design-only): `social.*`, `bank:budget_exceeded`, `people:birthday_*`,
`media:playback_completed` — register against the same store when their backends
land. Cross-linked from [docs/reference/events.md](../../../../docs/reference/events.md).

## Preferences default

Absent `notification_preferences` row = in-app on, email off, push off, not
muted. `muted` is the single "deliver nothing" switch (survives future channels).
