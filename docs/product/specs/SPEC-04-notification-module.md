# SPEC-04 — Notification Module (life-stream backbone)

**Status:** ready to build, rev 3 · **Drafted:** 2026-07-08
**Module:** `notify` (new — not yet scaffolded) · **Depends on:** SPEC-01 P1.2 for P0.4 (the `media:asset_ready` emit — see P0.4's dependency note); otherwise nothing hard
**Upstream:** the 2026-07 gap analysis — now folded into [backlog.md](../backlog.md) §5 *(the standalone `gap-audit-2026-07.md` file was never committed; link fixed 2026-07-10)* · **Refs:** backlog §1/§3/§5, [facebook-comparison](../analysis/facebook-comparison.md) §14, [MODULES.md](../../../backend/MODULES.md) §5.2
**Downstream consumers:** account (password reset, security alerts), the Olympus bell/activity UI, all future social types · **Consumes:** `media:asset_ready` (SPEC-01 P1.2)

---

## 1. Problem statement

Every "something happened → tell the user" path is currently dead:

- **Password reset can't ship** — it needs an email channel that doesn't exist (backlog §1 P1; admin/CLI only today).
- **`media:asset_ready` has no consumer** — SPEC-01 P1.2 makes media the first life-stream *producer*, but nothing turns that event into a user-visible notification.
- **The Olympus bell + "Activity Feed" dropdowns are hard-coded sample data** (backlog §3, facebook-comparison §14); badges are constants. There is no store, no `GET /me/notifications`, no realtime delivery, no preferences.
- **Security alerts have nowhere to go** — `account.refresh.reuse_detected` is audited but the user is never told their session was compromised.

[MODULES.md §5.2](../../../backend/MODULES.md) already **reserves the `notify:*` task prefix** ("the delivery fan-out that other modules enqueue into rather than sending mail/push themselves") and notes the account module currently *stubs* `RegisterTasks` for it. This spec makes `notify` a real module that **owns** that prefix. It is the backbone that unblocks the notification-dependent items now tracked in backlog §5 (the notify:* module — the priority-1 gap that unblocks password reset).

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
- **User-initiated notification deletion** — read/unread only at v1; the only deletion paths are account deletion (CASCADE) and the P2 retention janitor.
- **Being the life-stream system-of-record** — the `notifications` table is a *delivery store*. If/when a life-stream archive materializes (ADR-08), it gets its own store; retention here (P2) is hygiene, not history policy.

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
| GET | `/api/v1/me/notifications` | `notifications:read:own` | `?status=unread\|all&cursor=` (default: `all`) ; returns items + `unread_count` |
| POST | `/api/v1/me/notifications/{id}/read` | `notifications:write:own` | idempotent; returns `200 {unread_count}` |
| POST | `/api/v1/me/notifications/read-all` | `notifications:write:own` | `?before=` watermark cursor (absent = all); returns `200 {unread_count}` |

`status` defaults to `all` when omitted (the bell shows read+unread; the badge uses `unread_count`).

**Acceptance criteria.**
- Given 3 unread + 2 read notifications, when I GET `?status=unread`, then I receive exactly the 3 and `unread_count = 3`.
- Given a notification id I own, when I POST `/read`, then `read_at` is set once and a second call is an idempotent no-op that still returns `200 {unread_count}` (never 500) — matching the §5/§7 tables and the P0.5 optimistic settle, which reconciles from that body *(the earlier "204" here contradicted both; fixed 2026-07-10)*.
- Given user B's notification id, when user A marks it read, then 404 (never leaks existence).
- Given 500 notifications, when I page with `cursor`, then results are stable and ordered `created_at DESC, id DESC`.
- Given read-all with zero unread, then `200 {unread_count: 0}` (idempotent, never 500).
- Given a notification created after read-all's `before` watermark, then it stays unread (the user never saw it).
- Given no `status` param, the response contains both read and unread items ordered `created_at DESC, id DESC`.

**Permission seeding** *(added 2026-07-10 — previously unowned, and with the
fail-closed engine an unseeded code 403s everyone below superadmin, i.e. a dead
bell)*: the notify migration seeds `notifications:read:own`,
`notifications:write:own`, `notification-prefs:read:own`,
`notification-prefs:write:own`, `push-subscriptions:write:own`,
`push-subscriptions:delete:own` into the `permissions` catalog and grants all
six to the base `user` role (0003 `WITH grants(...)` pattern). "Self-service,
scoped to the caller" describes the row filtering, not the middleware — every
endpoint still runs `RequirePermission` per the tables.

### P0.2 — Dispatch fan-out (the public ingress)

**Behavior.** Producers enqueue a single task **`notify:dispatch`** with a `NotificationIntent` (`{user_id, type, title, body, data, channels?, dedup_key?}`) using the helper exported from `notify/api` (typed enqueue — no producer hardcodes the payload shape). `dedup_key` is optional and event-derived (e.g. `notice_id`, `asset_id`) — it exists so an Asynq retry of `notify:dispatch`, or SPEC-08's outbox re-publish, doesn't insert a duplicate bell row. The handler:
1. If the type is **muted** (prefs) and the type is not non-mutable (step 2) → stop: no row, no channel tasks. `muted` takes precedence over the per-channel booleans — it is the single "deliver nothing" switch (all-channels-off is equivalent today, but `muted` also survives future channel additions).
2. Resolve channels: the stored preference for `type` (default: in-app on, email off, push off — overridable per type, §6) **unioned with the intent's `channels` override**. `channels` exists precisely so transactional sends reach the user regardless of stored prefs — without it, the default email-off pref would silently eat the password-reset mail. **Non-mutable types** (`account.password_reset`; `account.security_alert` per P1.4) also ignore `muted`: a user who muted resets must still be able to recover their account.
3. If in-app enabled — and the type persists in-app at all (`account.password_reset` does **not**, P0.3) → insert a `notifications` row. When the intent carries `dedup_key`, the insert is `ON CONFLICT (user_id, type, dedup_key) WHERE dedup_key IS NOT NULL DO NOTHING` (§6) — a redelivered dispatch is a no-op against the existing row, not a duplicate bell item.
4. For each other enabled channel → enqueue the channel task (`notify:email`, `notify:web_push`) on the **`default` queue** (weight 1 — SPEC-01 P0.1's resource guardrails already assign light notify/janitor work there; a task enqueued to an unregistered queue name is silently never processed) — **unless step 3's insert hit the dedup conflict**, in which case channel fan-out is skipped too (the store row is the single idempotency gate for a redelivered intent). Handlers must stay lightweight/IO-bound with short timeouts so the weight-1 share of the worker's shared concurrency suffices. Channel sends themselves remain **at-least-once**: dedup only guards the in-app store, so a genuinely retried `notify:email`/`notify:web_push` task can still redeliver to the channel.
5. Records nothing that blocks the producer; delivery failures retry via Asynq, never surface to the caller. A malformed intent (missing `user_id`/`type`) returns `asynq.SkipRetry` — fail fast to the archive; a payload that can never become valid must not burn retries.

`notify` **owns** the `notify:*` task registration (worker `RegisterTasks`); the account module's existing stub is **removed** and account switches to enqueuing via `notify/api`.

**Acceptance criteria.**
- Given a `notify:dispatch` intent for a user whose prefs are default, when handled, then exactly one in-app row exists and no email/push task is enqueued.
- Given prefs {in_app:on, email:on} for that type, when handled, then one in-app row **and** one `notify:email` task are produced.
- Given a **mutable** `type` the user has muted, when handled, then no row and no channel tasks are produced.
- Given `channels:["email"]` on a non-mutable type for a default-prefs (or muted) user, when handled, then the email task is enqueued despite stored prefs — the P0.3 reset path.
- Given a malformed intent (missing `user_id`/`type`), when handled, then the task fails fast with a logged error and `asynq.SkipRetry` (straight to the archive — no retry burn).
- Given a redelivered dispatch intent with a `dedup_key`, exactly one `notifications` row exists.

### P0.3 — Email channel + password-reset integration

**Behavior.** `notify:email` handler renders a template (`type` → subject + HTML/text body from `data`) and sends via a configurable transport behind an `EmailSender` interface: **dev = Mailpit over SMTP** (add the `mailpit` service to docker-compose and `SMTP_HOST/PORT/FROM` to `.env.example` in the same PR; a log-sink `EmailSender` impl serves tests); **prod = SMTP or an API provider** (choice in §10). Account gains `POST /api/v1/auth/forgot-password {email}` and `POST /api/v1/auth/reset-password {token, new_password}`; `forgot` mints a single-use, short-TTL, hashed-at-rest reset token and enqueues `notify:dispatch {type:"account.password_reset", channels:["email"]}` (the P0.2 preference override — without it, the default email-off pref would eat the mail).

**Token storage is account-owned.** ≥256-bit CSPRNG, SHA-256 hash at rest, single-use, short TTL — mirror ADR-06's refresh-token construction (lookup by hash, constant-time). No such table exists anywhere today; per-module migration ownership means it cannot ride the notify migration: **P0.3 ships `000N_account_password_reset_tokens` (id, user_id → users, token_hash, expires_at, used_at, created_at)**. With this entropy, online token-guessing is moot and the per-IP limit on `reset-password` below is defense-in-depth, not load-bearing.

**`account.password_reset` is email-only and non-mutable** (P0.2 steps 1–3): no in-app `notifications` row is ever written for it — persisting the reset link in the store would defeat hashed-at-rest, and an in-app copy is useless to a locked-out user. The channel-only payload (the reset URL) rides the `notify:email` task, never the store.

**Abuse controls (public ingress).** These endpoints are unauthenticated and mint DB rows / send email; this **extends ADR-06's brute-force-defence responsibility** (named there only for `/auth/login`) to both new public auth endpoints:

- **Per-email throttle** (account service, *before* any token mint): key on the normalized email (reuse `normalizeEmail`), Redis `INCR`+`EXPIRE` on Dragonfly — the same live pattern as the login throttle in the account handler. (`platform/middleware/ratelimit.go` is currently dead code — never constructed, in-memory, single-instance by its own header comment — wiring it would be new work, not reuse.) Limits: ≥60 s between sends and ≤3 sends per email per hour. When throttled: **still 202**, silently skipping mint + dispatch — a 429 keyed on the email would leak account existence (only registered emails accumulate a counter). Match the login throttle's fail-open-on-Redis-outage semantics (log and proceed).
- **Per-IP throttle** on `/auth/forgot-password` **and** `/auth/reset-password`: 429 (IP-keyed leaks nothing about any email). Traefik's generic rate-limit middleware can back this as a coarse outer layer — one compose label; note the api router currently attaches **no** middleware — but a per-IP average limiter can never be the primary control for a per-email quota; the rule above is.
- **Global send ceiling** (budget insurance): a config-driven cap on `notify:email` sends per hour across all users; crossing it pauses the channel and error-logs. 3/email/hour does not bound aggregate spend — N registered addresses give an attacker 3N sends/hour, and §10's free-tier candidates make quota exhaustion a budget incident.
- **Uniform response timing**: respond 202 *before* the lookup/mint/enqueue work (or pad to uniform time) — otherwise the enumeration-safe 202 is a stopwatch oracle (registered emails do strictly more work).
- **Disabled accounts** (`users.disabled_at`): same 202, silently skip mint + dispatch; `reset-password` rejects tokens belonging to since-disabled users (`account/invalid-reset-token`).

**Acceptance criteria.**
- Given a registered email, when I POST `/auth/forgot-password`, then (dev) an email appears in Mailpit with a working reset link, and the response is an **enumeration-safe 202** regardless of whether the email exists.
- Given a valid unexpired reset token, when I POST `/auth/reset-password`, then the password is updated (Argon2id), `token_version` is bumped (all sessions revoked, per [ADR-06](../../adr/06-local-auth-model.md)), and the token is consumed.
- Given a reused or expired reset token, then 400 Problem `account/invalid-reset-token`; nothing changes.
- Given 10 rapid POSTs within one minute for one registered email, then all are 202 and exactly **one** dispatch intent is enqueued (the ≥60 s gap); spread over an hour, at most 3 — and throttled requests mint **no** reset-token rows.
- Given an IP-level flood, then 429 — with a body that reveals nothing about whether any email is registered.
- Given the global hourly send ceiling crossed, then the email channel pauses (tasks re-queue/park), an error is logged, and `/auth/forgot-password` still answers 202 *(AC added 2026-07-10 — the control existed with no test)*.
- Given one registered and one unregistered email POSTed to `/auth/forgot-password`, then the two responses are indistinguishable in status **and** timing within measurement noise — the 202 is returned before (or padded around) the lookup/mint work *(ditto)*.
- Given a disabled account (`users.disabled_at`), then `forgot-password` answers 202 with no token minted and no dispatch; a pre-disable token presented to `reset-password` is rejected with `account/invalid-reset-token` *(ditto)*.
- Email send failure retries (Asynq) without losing the in-app copy for types that persist one — testable with `media.asset_ready` (email channel enabled in prefs) or P1.4's `account.security_alert`; `account.password_reset` is untestable here by design, it never persists a row *(AC retargeted 2026-07-10 — it previously named no type it could test)*.

### P0.4 — First event consumer (`media:asset_ready`)

**Behavior.** `notify` subscribes to `media:asset_ready` (SPEC-01 P1.2 payload `{asset_id, kind, owner_user_id, title, origin}`) and dispatches an intent `{user_id: owner_user_id, type:"media.asset_ready", title, data:{asset_id, kind, href}}`. **Events with `origin='import'` are skipped** — a SPEC-02 zip import creates up to 300 assets and the bell must not receive 300 notifications for one chapter *(2026-07-10)*. No import of the media module — subscription goes through the `platform/events` fan-out (events.md "Delivery mechanics"): notify registers **`notify:on_asset_ready`**; the `cmd/worker` subscription table maps `media:asset_ready` → `notify:on_asset_ready` per events.md; the handler builds the intent and runs the P0.2 dispatch. Direct task-type handling would collide the moment a second consumer (SPEC-06's stream) registers for the same event — Asynq's ServeMux allows exactly one handler per task type.

**Dependency (the header's "nothing hard" does not cover this):** the emit side is SPEC-01 **P1.2 — a nice-to-have that may not ship with SPEC-01's P0**. **Decided: P0.4 is not gated.** If SPEC-01 P1.2 hasn't landed when phase 4 starts, this item includes the one-line `platform/events.Publish("media:asset_ready", …)` in media's ready-transition (coordinated with the media owner); the consumer never ships without a producer.

**Click-through contract** (user story 4 depends on it): every in-app type declares how its `data` becomes a link — the dispatch intent carries a required **`data.href`** (relative app path, e.g. the asset's library entry for `media.asset_ready`). The bell renders `title` + navigates to `data.href`; no per-type frontend mapping tables, no improvisation per type.

**Acceptance criteria.**
- Given a video that reaches `ready`, when the event fires, then a `notifications` row for the owner exists within a few seconds (queue latency), and it is visible in the bell by the next poll/focus refetch (P0.5) — or < 10 s once P1.2 SSE lands (§8).
- Given the notify module is down, when it recovers, then queued `media:asset_ready` tasks are still processed (Asynq durability) — no lost notifications.

### P0.5 — Bell wiring (frontend)

The bell UI is in scope, not an afterthought: Goal 1 and §8's "zero hard-coded sample data" metric both require it, and no other requirement owned it. `NotificationsMenu` in [NotifMenus.tsx](../../../frontend/src/templates/v1/components/headers/NotifMenus.tsx) currently renders a hard-coded `NOTIFS` fixture behind `{open, onToggle}`-only props — wiring it is an **interface change** (query-hook injection), not a data swap.

- Server state per D-32 ([frontend/CLAUDE.md](../../../frontend/CLAUDE.md)): `useQuery(["notifications"])` owns items + `unread_count` (`staleTime: 0` — personal counter). **P0 delivery is polling**: `refetchInterval` ≈ 60 s + refetch on window focus. (*Not* `SessionKeeper` — that is auth plumbing, D-34, and must not carry server state.)
- Mark-read / read-all are **optimistic** (D-32): `onMutate` patches items + badge, `onError` rolls back, settle reconciles from the mutation's `200 {unread_count}` response — no extra GET.
- **read-all sends the `before` watermark** (the `(created_at, id)` cursor of the newest rendered item), so rows that arrived after the dropdown rendered stay unread.
- `FriendRequestsMenu` / `MessagesMenu` in the same file keep their fixtures — their backends are non-goals (§3); §8's fixture-grep metric applies to `NotificationsMenu` only.

**Acceptance criteria.**
- Given mark-all-read, then the badge renders only the optimistic value or a server count no older than the mutation settle — a count computed before the settle is never rendered (the observable no-flicker rule).
- Given the fixtures grep (§8), then `NOTIFS` is gone from `NotificationsMenu` and every rendered item is a real `notifications` row.

### P1 — nice to have

- **P1.1 Web push:** VAPID-based Web Push. `POST/DELETE /api/v1/me/push-subscriptions`; `notify:web_push` handler delivers to all of a user's subscriptions and prunes `410 Gone` endpoints.
- **P1.2 Realtime in-app:** SSE `GET /api/v1/me/notifications/stream` pushes new-notification + unread-count events, replacing P0.5's poll. **Reconciliation rule (anti-flicker):** stream events are **invalidation signals** — the client refetches the notifications query rather than writing streamed payloads into the cache; TanStack stays the single writer, so a stream event racing an in-flight mark-read mutation can never render a stale count. (Events still carry the server-computed `unread_count` for future consumers; v1 clients treat it as a hint, not state.) **Hardening:** cap concurrent streams per user (~3, evict oldest); heartbeat comment every ~25 s (reaps dead clients, survives proxy idle timeouts); cap stream lifetime at the access-token TTL (~5 min, server-side close → reconnect) so `token_version` revocation actually severs streams — `RequireAuth` only re-checks per request; on (re)connect the client refetches instead of replaying events. This **supersedes** frontend.md Phase 6's `/api/v1/events/stream` + "mutate cache directly, no refetch" sketch — exactly one reconciliation rule exists, this one.
- **P1.3 Preferences UI:** `GET/PUT /api/v1/me/notification-preferences`; wire the profile-dropdown "settings" that is currently a placeholder.
- **P1.4 Security-alert type:** account emits on `account.refresh.reuse_detected` → `type:"account.security_alert"` (email + in-app, not mutable off). The emitting intent carries `channels:["email","in_app"]` — the P0.2 union is what makes the type undisableable; "not mutable off" is enforced by the override + the muted-ignore rule, not by the prefs schema.

### P2 — future considerations (design for, don't build)

- Social types (`social.friend_request`, `social.comment`, `social.reaction`, …) register against the same store the moment the social backend exists — keep `type` an open string with a documented registry (it lives in `notify/README.md`, cross-linked from [docs/reference/events.md](../../reference/events.md)), not an enum.
- Aggregation/digest ("and 4 others") — keep `data` jsonb flexible enough to fold.
- Quiet hours / timezone-aware delivery (users already carry a timezone).
- **Retention janitor `notify:purge_old`** — an Asynq **periodic task** (nightly, `default` queue), the same periodic-runner infrastructure SPEC-01 P0.3's `media:purge_orphans` introduces (no OS cron exists in this stack; account's committed-but-unscheduled `PurgeExpiredRefreshTokens` should ride the same runner). **Batched** deletes — the §6 indexes don't cover a global `read_at`/`created_at` predicate, so don't promise "indexed"; add one only if measured — of read > 90 d and unread > 180 d, **exempting non-mutable types** (`account.security_alert`). At n=1 volume (~10–20k rows/yr) this is hygiene, not performance: the realistic low-VPS pressure is dead-tuple churn on the partial unread index from mark-read updates, which autovacuum handles and purging does not — don't "fix" badge slowness with a more aggressive purge.

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
  dedup_key   text,                          -- optional event-derived natural id (e.g. notice_id, asset_id); idempotency key under at-least-once delivery
  read_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON notifications (user_id, created_at DESC, id DESC);
CREATE INDEX ON notifications (user_id) WHERE read_at IS NULL;   -- unread badge
CREATE UNIQUE INDEX ON notifications (user_id, type, dedup_key) WHERE dedup_key IS NOT NULL;  -- redelivered dispatch/outbox re-publish is a no-op, not a duplicate row

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

`user_id` FKs into the account module's `users` table. **This is a cross-module FK and is normally forbidden** by MODULES.md — **decided: option (a), the sanctioned identity-anchor exception**, matching the shipped precedent (`0007_media_assets.up.sql`: `owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`). `users(id)` is the one cross-module reference a module may FK; everything else stays event-coupled. Queries in `query/notify_*.sql`; regenerate with `make sqlc` — never hand-edit `*.sql.go`.

**Migration sequencing:** the account-owned `000N_account_password_reset_tokens` (P0.3) must land with or before the email phase; the notify tables land before P0.1. Both take the next free numbers in the shared sequence (SPEC-01's `media_variants` is also contending — "take the next free number" resolves it, just don't pre-assign).

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/me/notifications` | `notifications:read:own` | `?status=&cursor=` (default: `all`); `{items, unread_count, next_cursor}` |
| POST | `/api/v1/me/notifications/{id}/read` | `notifications:write:own` | `200 {unread_count}`, idempotent |
| POST | `/api/v1/me/notifications/read-all` | `notifications:write:own` | `?before=` watermark; `200 {unread_count}` |
| GET/PUT | `/api/v1/me/notification-preferences` | `notification-prefs:read/write:own` | P1.3 |
| POST/DELETE | `/api/v1/me/push-subscriptions` | `push-subscriptions:write/delete:own` | P1.1 |
| GET | `/api/v1/me/notifications/stream` | `notifications:read:own` | P1.2 SSE |
| POST | `/api/v1/auth/forgot-password` | *(public)* | enumeration-safe 202 |
| POST | `/api/v1/auth/reset-password` | *(public)* | account-owned; consumes token |

Problem types: `notify/notification-not-found`, `account/invalid-reset-token`, `account/password-policy` (reuses account's policy). Auth endpoints are **account-owned**; they are listed here only because this spec is what unblocks them.

`status` defaults to `all` (the bell shows read+unread; the badge uses `unread_count`). Permission codes follow the canonical scheme (README conventions): `write` covers create + update, matching the 0003 catalog — the earlier `notifications:update:own` / `notification-prefs:update:own` / `push-subscriptions:create:own` verbs added a style the catalog doesn't use *(reconciled 2026-07-10)*.

Permission codes above are deliberately **3-segment** (`<resource>:<action>:<scope>`): `rbac.Parse` rejects anything else ("must have 2 or 3 segments") — a 4-segment module-prefixed code is rejected by `rbac.Parse`: wired through `RequirePermission` it panics at server start (`MustParse` on the required code), and any dynamic `AllowsCode` check fails closed — returning false even for a `*` superadmin grant. *(The 4-segment drafts this note used to flag in SPEC-01/SPEC-03 were reconciled 2026-07-10 — see the canonical scheme in the specs README's AuthZ convention.)*

**Asynq task types owned by this module** — registered in [docs/reference/events.md](../../reference/events.md), which owns the task/event inventory and makes registration part of definition-of-done (MODULES.md §5.2 only reserves the `notify:*` prefix; the notification **`type`-string** registry is what lives in `notify/README.md`): `notify:dispatch`, `notify:email`, `notify:web_push` (P1.1), `notify:on_asset_ready`, `notify:purge_old` (P2). **Subscribes to:** `media:asset_ready` (via `notify:on_asset_ready`).

## 8. Success metrics (n=1 honest)

- Leading: password-reset email delivered in dev < 5 s from `forgot-password` (Mailpit timestamp); reset success end-to-end without admin intervention.
- Leading: `media:asset_ready` → in-app notification visible in the bell **< 10 s p95 with P1.2 SSE**; the P0 (poll-only) target is visible by the next poll/focus refetch (≤ 60 s, P0.5) — a 60 s poll cannot honestly claim 10 s.
- Lagging: the bell dropdown shows **zero hard-coded sample data** — every item is a real `notifications` row (grep `NotificationsMenu` for the removed `NOTIFS` fixture; the social menus' fixtures remain until their backends exist, §3/P0.5).

## 9. Timeline & phasing

1. Module scaffold (MODULES.md §8: subtree, `sqlc.yaml` block, migration) + store + read API + in-app dispatch (1.5 day)
2. `notify:dispatch` fan-out (channels override + muted precedence) + preferences default + `notify/api` enqueue helper (1 day)
3. Email channel (`EmailSender` + Mailpit compose service) + account `forgot/reset-password` + reset-token migration + abuse controls (per-email/per-IP throttles, global ceiling) (2 days)
4. `media:asset_ready` consumer + wire into `cmd/worker` — not gated on SPEC-01 P1.2; if P1.2 hasn't landed yet, includes the one-line emit in media's ready-transition (coordinated with media) (½ day)
5. Bell wiring (P0.5: TanStack query + optimistic mark-read + fixture removal) (1 day)
6. P1 (web push, SSE + hardening, prefs UI, security alert) (2 days, optional)
Total P0 ≈ 6 dev-days; P1 ≈ +2.

## 10. Open questions

- **(decide before P0.3)** Email transport for prod under the ≤$100/mo budget: self-hosted SMTP vs a free/cheap API tier (Resend/Postmark/SES). Dev is Mailpit regardless. *Blocks the email channel; not the store.* Worth an ADR or a `D-N` entry.
- **(resolved)** Cross-module `user_id` FK: option (a), the sanctioned identity-anchor exception — verified against the shipped precedent `0007_media_assets.up.sql` (`owner_id … REFERENCES users(id) ON DELETE CASCADE`). See §6.
- **(engineering, non-blocking)** Realtime transport for P1.2: SSE (chosen here) vs poll-only for v1 — SSE needs a per-user channel over the existing infra (Dragonfly pub/sub is available).
- **(resolved 2026-07-10)** `account.security_alert` non-mutability: **yes** — the requirements body already binds it (P0.2 steps 1–2, P1.4, and the P2 janitor's exemption all treat it as non-mutable); this entry was stale as an open question.
- **(engineering, non-blocking)** Where VAPID keypair + SMTP creds live (`platform/config` + secrets); rotation story.

## 11. Revision history

| Rev | Date | Change |
|---|---|---|
| r1 | 2026-07-08 | Initial spec. |
| r2 | 2026-07-10 | Reconciliation pass: mark-read response fixed to `200` (removed a `204` contradiction with §7/P0.5); permission seeding added (previously unowned, fail-closed engine would 403 everyone); `account.security_alert` non-mutability resolved; P0.4 `origin='import'` suppression added; abuse-control ACs added (global ceiling, uniform timing, disabled accounts); gap-audit link fixed. |
| r3 | 2026-07-11 | Spec-gap review fixes: `dedup_key` + unique index for idempotent dispatch under at-least-once delivery (F021); permission action verbs reconciled to `write` across §5/§7 (F022); P0.4/SPEC-01 P1.2 dependency decided as ungated, mirrored in §9 phase 4 (F023); GET `?status=` default documented + AC added (F053); `notify:on_asset_ready` consumer task named in P0.4 and §7's owned-task list (F054); P0.4 AC reworded to separate store latency from bell visibility (F055); P1.4 security-alert channel-forcing clarified (F056); header Drafted date + Downstream/Consumes split (F033/F034); malformed-permission-code note corrected to the panic-at-start/fail-closed behavior (F035); §1/header upstream citation resolved to backlog §5 (F052). |
