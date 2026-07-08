# SPEC-04 — Notification Module (life-stream backbone)

**Module:** `notify` (new — not yet scaffolded) · **Status:** ready to build · **Depends on:** nothing hard
**Upstream:** [gap-audit-2026-07](../analysis/gap-audit-2026-07.md) priority #2 · **Refs:** backlog §1/§3/§5, [facebook-comparison](../analysis/facebook-comparison.md) §14, [MODULES.md](../../../backend/MODULES.md) §5.2
**Downstream consumers:** account (password reset, security alerts), media (`media:asset_ready`), the Olympus bell/activity UI, all future social types

---

## 1. Problem statement

Every "something happened → tell the user" path is currently dead:

- **Password reset can't ship** — it needs an email channel that doesn't exist (backlog §1 P1; admin/CLI only today).
- **`media:asset_ready` has no consumer** — SPEC-01 P1.2 makes media the first life-stream *producer*, but nothing turns that event into a user-visible notification.
- **The Olympus bell + "Activity Feed" dropdowns are hard-coded sample data** (backlog §3, facebook-comparison §14); badges are constants. There is no store, no `GET /me/notifications`, no realtime delivery, no preferences.
- **Security alerts have nowhere to go** — `auth.refresh.reuse_detected` is audited but the user is never told their session was compromised.

[MODULES.md §5.2](../../../backend/MODULES.md) already **reserves the `notify:*` task prefix** ("the delivery fan-out that other modules enqueue into rather than sending mail/push themselves") and notes the account module currently *stubs* `RegisterTasks` for it. This spec makes `notify` a real module that **owns** that prefix. It is the backbone that unblocks §2 of the gap audit's priorities.

## 2. Goals

1. A durable in-app **notification store** + `GET /me/notifications` with unread badge and mark-read — backs the Olympus bell with real data.
2. A **delivery fan-out**: any module enqueues one typed intent; `notify` writes the in-app row and dispatches to enabled channels. Producers never send mail/push themselves (MODULES.md §5.2).
3. **Email channel** works end-to-end → **unblocks password reset** (closes backlog §1 P1) and security alerts.
4. **Per-type preferences** (in-app / email / push, or muted) with a sane default when no row exists.
5. First **event consumer**: subscribe to `media:asset_ready` and produce an in-app notification — proving the producer→bus→consumer loop.
6. Boundary-clean: `notify` never imports another module's internals, and producers depend only on `notify/api`.

## 3. Non-goals

- **Social notification *types*** (friend request, comment, reaction, mention) — they depend on the social backend, which does not exist (backlog §3). This spec ships the **mechanism** and registers the types that have real producers *today* (media, account). New types slot in later with zero schema change.
- **SMS / native mobile push** — no mobile app exists; web-push only.
- **Digest / aggregation emails** ("3 people liked…") — P2 design seam only.
- **Full WebSocket transport** — SSE (P1) is sufficient; bidirectional WS is deferred.
- **Marketing / campaign email** — transactional + activity only.

## 4. User stories

- As a user who forgot my password, I request a reset and receive an email with a one-time link, so I can get back in without an admin. *(primary — unblocks account)*
- As a creator, when my upload finishes transcoding, a notification appears in my bell so I know it's ready to share.
- As a security-conscious user, when a stolen refresh token is replayed, I get an email + in-app alert that my sessions were revoked.
- As a user, I open the bell, see unread items, click one to go to its target, and the badge clears; I can "mark all read."
- As a user, I turn off email for "asset ready" but keep it for "security," and the system respects that.

## 5. Requirements

### P0.1 — Notification store + read API

**Behavior.** A `notifications` row per in-app notification (§6). Endpoints (all self-service, `RequireAuth`, scoped to the caller):

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/me/notifications` | `notify:notification:read:own` | `?status=unread\|all&cursor=` ; returns items + `unread_count` |
| POST | `/api/v1/me/notifications/{id}/read` | `notify:notification:update:own` | idempotent |
| POST | `/api/v1/me/notifications/read-all` | `notify:notification:update:own` | marks every unread read |

**Acceptance criteria.**
- Given 3 unread + 2 read notifications, when I GET `?status=unread`, then I receive exactly the 3 and `unread_count = 3`.
- Given a notification id I own, when I POST `/read`, then `read_at` is set once and a second call is a no-op 204 (never 500).
- Given user B's notification id, when user A marks it read, then 404 (never leaks existence).
- Given 500 notifications, when I page with `cursor`, then results are stable and ordered `created_at DESC, id DESC`.

### P0.2 — Dispatch fan-out (the public ingress)

**Behavior.** Producers enqueue a single task **`notify:dispatch`** with a `NotificationIntent` (`{user_id, type, title, body, data, channels?}`) using the helper exported from `notify/api` (typed enqueue — no producer hardcodes the payload shape). The handler:
1. Loads the recipient's preference for `type` (default: in-app on, email off, push off — overridable per type, §6).
2. If in-app enabled → insert a `notifications` row.
3. For each other enabled channel → enqueue the channel task (`notify:email`, `notify:web_push`).
4. Records nothing that blocks the producer; delivery failures retry via Asynq, never surface to the caller.

`notify` **owns** the `notify:*` task registration (worker `RegisterTasks`); the account module's existing stub is **removed** and account switches to enqueuing via `notify/api`.

**Acceptance criteria.**
- Given a `notify:dispatch` intent for a user whose prefs are default, when handled, then exactly one in-app row exists and no email/push task is enqueued.
- Given prefs {in_app:on, email:on} for that type, when handled, then one in-app row **and** one `notify:email` task are produced.
- Given a `type` the user has muted, when handled, then no row and no channel tasks are produced.
- Given a malformed intent (missing `user_id`/`type`), when handled, then the task fails fast with a logged error and is not retried indefinitely (moves to the Asynq archive after max retries).

### P0.3 — Email channel + password-reset integration

**Behavior.** `notify:email` handler renders a template (`type` → subject + HTML/text body from `data`) and sends via a configurable transport behind an `EmailSender` interface: **dev = Mailpit/log sink; prod = SMTP or an API provider** (choice in §10). Account gains `POST /api/v1/auth/forgot-password {email}` and `POST /api/v1/auth/reset-password {token, new_password}`; `forgot` mints a single-use, short-TTL, hashed-at-rest reset token and enqueues `notify:dispatch {type:"account.password_reset", channels:["email"]}`.

**Acceptance criteria.**
- Given a registered email, when I POST `/auth/forgot-password`, then (dev) an email appears in Mailpit with a working reset link, and the response is an **enumeration-safe 202** regardless of whether the email exists.
- Given a valid unexpired reset token, when I POST `/auth/reset-password`, then the password is updated (Argon2id), `token_version` is bumped (all sessions revoked, per [ADR-06](../../adr/06-local-auth-model.md)), and the token is consumed.
- Given a reused or expired reset token, then 400 Problem `account/invalid-reset-token`; nothing changes.
- Email send failure retries (Asynq) and does not lose the notification's in-app copy.

### P0.4 — First event consumer (`media:asset_ready`)

**Behavior.** `notify` subscribes to the `media:asset_ready` task type (SPEC-01 P1.2 payload `{asset_id, kind, owner_user_id, title}`) and dispatches an intent `{user_id: owner_user_id, type:"media.asset_ready", title, data:{asset_id, kind}}`. No import of the media module — subscription is by task-type string on the shared queue.

**Acceptance criteria.**
- Given a video that reaches `ready`, when the event fires, then the owner has a new in-app notification linking to the asset within a few seconds.
- Given the notify module is down, when it recovers, then queued `media:asset_ready` tasks are still processed (Asynq durability) — no lost notifications.

### P1 — nice to have

- **P1.1 Web push:** VAPID-based Web Push. `POST/DELETE /api/v1/me/push-subscriptions`; `notify:web_push` handler delivers to all of a user's subscriptions and prunes `410 Gone` endpoints.
- **P1.2 Realtime in-app:** SSE `GET /api/v1/me/notifications/stream` pushes new-notification + unread-count events, replacing the P0 poll. (Frontend already polls in `SessionKeeper` — reuse the cadence until SSE lands.)
- **P1.3 Preferences UI:** `GET/PUT /api/v1/me/notification-preferences`; wire the profile-dropdown "settings" that is currently a placeholder.
- **P1.4 Security-alert type:** account emits on `auth.refresh.reuse_detected` → `type:"account.security_alert"` (email + in-app, not mutable off).

### P2 — future considerations (design for, don't build)

- Social types (`social.friend_request`, `social.comment`, `social.reaction`, …) register against the same store the moment the social backend exists — keep `type` an open string with a documented registry, not an enum.
- Aggregation/digest ("and 4 others") — keep `data` jsonb flexible enough to fold.
- Quiet hours / timezone-aware delivery (users already carry a timezone).

## 6. Data model

Migration `000N_notify_notifications` (**take the next free number — verify the repo**; 0007 was consumed by `media_assets`, and SPEC-01 adds `media_variants`):

```sql
CREATE TABLE notifications (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type        text NOT NULL,                 -- open registry, e.g. 'media.asset_ready'
  title       text NOT NULL,
  body        text,
  data        jsonb NOT NULL DEFAULT '{}',   -- target ids, links, aggregation payload
  read_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON notifications (user_id, created_at DESC, id DESC);
CREATE INDEX ON notifications (user_id) WHERE read_at IS NULL;   -- unread badge

CREATE TABLE notification_preferences (
  user_id  uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type     text NOT NULL,
  in_app   boolean NOT NULL DEFAULT true,
  email    boolean NOT NULL DEFAULT false,
  push     boolean NOT NULL DEFAULT false,
  muted    boolean NOT NULL DEFAULT false,
  PRIMARY KEY (user_id, type)               -- absent row = module default
);

CREATE TABLE web_push_subscriptions (       -- P1.1
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint   text NOT NULL UNIQUE,
  p256dh     text NOT NULL,
  auth       text NOT NULL,
  user_agent text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz
);
```

`user_id` FKs into the account module's `users` table. **This is a cross-module FK and is normally forbidden** by MODULES.md — resolve one of two ways before building (§10): (a) treat `users(id)` as the one shared identity anchor every module already references (`media_assets.owner_id` does the same today), documented as the sanctioned exception; or (b) drop the FK and rely on application-level integrity. Match whatever `media` did for `owner_id`. Queries in `query/notify_*.sql`; regenerate with `make sqlc` — never hand-edit `*.sql.go`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/me/notifications` | `notify:notification:read:own` | `?status=&cursor=`; `{items, unread_count, next_cursor}` |
| POST | `/api/v1/me/notifications/{id}/read` | `notify:notification:update:own` | 204 idempotent |
| POST | `/api/v1/me/notifications/read-all` | `notify:notification:update:own` | 204 |
| GET/PUT | `/api/v1/me/notification-preferences` | `notify:preference:*:own` | P1.3 |
| POST/DELETE | `/api/v1/me/push-subscriptions` | `notify:push:*:own` | P1.1 |
| GET | `/api/v1/me/notifications/stream` | `notify:notification:read:own` | P1.2 SSE |
| POST | `/api/v1/auth/forgot-password` | *(public)* | enumeration-safe 202 |
| POST | `/api/v1/auth/reset-password` | *(public)* | account-owned; consumes token |

Problem types: `notify/notification-not-found`, `account/invalid-reset-token`, `account/password-policy` (reuses account's policy). Auth endpoints are **account-owned**; they are listed here only because this spec is what unblocks them.

**Asynq task types owned by this module** (register in `notify/README.md` per MODULES.md §5.2): `notify:dispatch`, `notify:email`, `notify:web_push`. **Subscribes to:** `media:asset_ready`.

## 8. Success metrics (n=1 honest)

- Leading: password-reset email delivered in dev < 5 s from `forgot-password` (Mailpit timestamp); reset success end-to-end without admin intervention.
- Leading: `media:asset_ready` → in-app notification visible in the bell < 10 s p95.
- Lagging: the bell dropdown shows **zero hard-coded sample data** — every item is a real `notifications` row (grep the frontend for the removed fixtures).

## 9. Timeline & phasing

1. Module scaffold (MODULES.md §8: subtree, `sqlc.yaml` block, migration) + store + read API + in-app dispatch (1.5 day)
2. `notify:dispatch` fan-out + preferences default + `notify/api` enqueue helper (1 day)
3. Email channel (`EmailSender` + Mailpit dev) + account `forgot/reset-password` (1.5 day)
4. `media:asset_ready` consumer + wire into `cmd/worker` (½ day)
5. P1 (web push, SSE, prefs UI, security alert) (1.5 day, optional)
Total P0 ≈ 4–5 dev-days; P1 ≈ +1.5.

## 10. Open questions

- **(decide before P0.3)** Email transport for prod under the ≤$100/mo budget: self-hosted SMTP vs a free/cheap API tier (Resend/Postmark/SES). Dev is Mailpit regardless. *Blocks the email channel; not the store.* Worth an ADR or a `D-N` entry.
- **(decide before §6 migration)** Cross-module `user_id` FK: sanctioned identity-anchor exception vs app-level integrity — **must match how `media_assets.owner_id` was resolved.** Verify in the repo.
- **(engineering, non-blocking)** Realtime transport for P1.2: SSE (chosen here) vs poll-only for v1 — SSE needs a per-user channel over the existing infra (Dragonfly pub/sub is available).
- **(product, non-blocking)** Should `account.security_alert` be non-mutable (can't be turned off)? Proposed yes.
- **(engineering, non-blocking)** Where VAPID keypair + SMTP creds live (`platform/config` + secrets); rotation story.
