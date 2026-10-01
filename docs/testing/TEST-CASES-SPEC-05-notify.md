# Test Cases — SPEC-05 Notification Module

**Status:** current · **Last verified:** never

**Spec:** [SPEC-05](../product/specs/SPEC-05-notification-module.md) · **Module:** `notify` (+ `account` for reset)
**Prefix:** `TC-NOTIFY-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md) · **Risk:** R5 (account takeover)

### Endpoints under test

| Method | Path | Perm |
|---|---|---|
| GET | `/api/v1/me/notifications?status=&cursor=&limit=` | `notifications:read:own` |
| POST | `/api/v1/me/notifications/{id}/read` | `notifications:write:own` |
| POST | `/api/v1/me/notifications/read-all?before=` | `notifications:write:own` |
| GET | `/api/v1/me/notification-preferences` | `notification-prefs:read:own` (P1.3) |
| PUT | `/api/v1/me/notification-preferences` | `notification-prefs:write:own` (P1.3) |
| POST | `/api/v1/me/push-subscriptions` | `push-subscriptions:write:own` (P1.1) |
| DELETE | `/api/v1/me/push-subscriptions/{id}` | `push-subscriptions:delete:own` (P1.1) |
| GET | `/api/v1/me/notifications/stream` | `notifications:read:own` (P1.2 SSE) |
| POST | `/api/v1/auth/forgot-password` | public (enumeration-safe 202) |
| POST | `/api/v1/auth/reset-password` | public (consumes token) |

### Preconditions

- Mailpit reachable in dev (`SMTP_HOST/PORT/FROM` set). Accounts `owner`,`userA`,`userB`,`guest`; one `disabled` account (`users.disabled_at` set).
- Problem types: `notify/notification-not-found`, `notify/invalid-cursor`, `notify/validation`, `notify/invalid-preference`, `notify/type-not-mutable`, `notify/invalid-push-subscription`, `notify/push-subscription-not-found`, `account/invalid-reset-token`, `account/password-policy`, `account/rate-limited`.
- Asynq tasks: `notify:dispatch`, `notify:email`, `notify:web_push`, `notify:on_asset_ready`, `notify:purge_old`.

---

## P0.1 — Store + read API

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-NOTIFY-001 | status=unread returns only unread | Functional | P0 | seed 3 unread + 2 read; GET `?status=unread` | exactly 3 items; `unread_count=3` | ☐ |
| TC-NOTIFY-002 | Default status=all | Functional | P0 | GET without status | read+unread, ordered `created_at DESC, id DESC`; badge uses `unread_count` | ☐ |
| TC-NOTIFY-003 | Mark-read idempotent 200 | Idempotency | P0 | POST `/{id}/read` twice | `read_at` set once; both return **200 {unread_count}** (never 204/500) | ☐ |
| TC-NOTIFY-004 | Foreign notification → 404 | AuthZ | P0(S1) | userA marks userB's id read | 404 (never leaks existence) | ☐ (CC-3) |
| TC-NOTIFY-005 | Cursor stable over 500 | Functional | P0 | page with cursor | stable, ordered `created_at DESC, id DESC`; no dupes/gaps | ☐ (CC-4) |
| TC-NOTIFY-006 | read-all zero unread → 200 | Idempotency | P0 | read-all with nothing unread | `200 {unread_count:0}` (never 500) | ☐ |
| TC-NOTIFY-007 | read-all `before` watermark (inclusive) | Functional | P0 | read-all with `before` = newest rendered item's `cursor`; a row created after it | that item is read; post-watermark row stays unread | ☐ |
| TC-NOTIFY-009 | Opaque cursors + limit | Contract | P0 | GET with `limit=500` (over 120 rows), with `limit=abc`, with no `limit`; GET with a hand-built `<created_at>_<id>` cursor | `limit=500` → 100 items (clamped to the max, owner decision 2026-10-01); `abc` / absent → 50; never a Problem; each item carries `cursor`; hand-built cursor → 400 `notify/invalid-cursor` | ☐ |
| TC-NOTIFY-008 | Permission seeding | AuthZ | P0 | inspect grants | all six notify codes seeded → `user` role (else dead bell) | ☐ (CC-2) |

## P0.2 — Dispatch fan-out

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-NOTIFY-020 | Default prefs → in-app only | Functional | P0 | enqueue `notify:dispatch` for default-prefs user | exactly one in-app row; **no** email/push task | ☐ |
| TC-NOTIFY-021 | prefs email:on → in-app + email | Functional | P0 | prefs {in_app:on,email:on}; dispatch | one in-app row **and** one `notify:email` task | ☐ |
| TC-NOTIFY-022 | Muted mutable type → nothing | Functional | P0 | user mutes a mutable type; dispatch | no row, no channel tasks (muted = single "deliver nothing") | ☐ |
| TC-NOTIFY-023 | channels override on non-mutable | Functional | P0(S1) | dispatch non-mutable type `channels:["email"]` to muted/default user | email task enqueued despite prefs (the reset path) | ☐ |
| TC-NOTIFY-024 | Malformed intent → SkipRetry | Reliability | P0 | dispatch missing user_id/type | fails fast, logged, `asynq.SkipRetry` (no retry burn, straight to archive) | ☐ |
| TC-NOTIFY-025 | dedup_key idempotent | Idempotency | P0(S1) | redeliver dispatch with same dedup_key | exactly one `notifications` row (ON CONFLICT DO NOTHING); fan-out still runs, deduplicated by the channel `TaskID` | ☐ |
| TC-NOTIFY-028 | Retry after insert-then-enqueue failure | Idempotency | P0(S1) | fail the email enqueue after the row insert; let Asynq retry | exactly one row and exactly one `notify:email` task | ☐ |
| TC-NOTIFY-029 | Channel dedup window | Idempotency | P0 | redeliver a `dedup_key` intent within 24 h after its email was sent | no second email (`ErrTaskIDConflict` treated as success) | ☐ |
| TC-NOTIFY-030 | Unknown channel / missing href | Reliability | P0 | dispatch `channels:["sms"]`; dispatch an in-app intent without `data.href` | both fail fast with `asynq.SkipRetry` | ☐ |
| TC-NOTIFY-026 | Channel task on default queue | Reliability | P1 | inspect queue of channel tasks | `notify:email`/`notify:web_push` land on `default` (weight-1) queue, actually processed | ☐ |
| TC-NOTIFY-027 | account stub removed, uses notify/api | Contract | P1 | inspect account module | account enqueues via `notify/api`; old `notify:*` stub removed | ☐ |

## P0.3 — Email + password reset

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-NOTIFY-040 | Reset email delivered (dev) | Functional | P0(S1) | POST `/auth/forgot-password` {registered email} | Mailpit shows email w/ working reset link within < 5 s; response **202** | ☐ |
| TC-NOTIFY-041 | Enumeration-safe status | Security | P0(S1) | forgot-password with registered vs unregistered email | both **202**, indistinguishable status | ☐ |
| TC-NOTIFY-042 | Enumeration-safe timing | Security | P0(S1) | measure response time registered vs unregistered | indistinguishable within measurement noise (202 before/padded around lookup) | ☐ [MANUAL] |
| TC-NOTIFY-043 | Valid token resets + revokes sessions | Functional | P0(S1) | POST `/auth/reset-password` {valid token, new pw} | password updated (Argon2id); `token_version` bumped (all sessions revoked); token consumed; every other outstanding token of that user → 400 `account/invalid-reset-token` | ☐ |
| TC-NOTIFY-044 | Reused token → 400 | Security | P0(S1) | reset-password with an already-used token | 400 `account/invalid-reset-token`; nothing changes | ☐ |
| TC-NOTIFY-045 | Expired token → 400 | Security | P0(S1) | reset with expired token | 400 `account/invalid-reset-token` | ☐ |
| TC-NOTIFY-046 | Per-email throttle | Security | P0 | 10 rapid forgot-password for one registered email in 1 min | all 202; **exactly one** dispatch enqueued (≥60 s gap); ≤3/hour; throttled requests mint **no** token rows | ☐ |
| TC-NOTIFY-047 | Per-IP flood → 429 | Security | P0 | 11 POSTs from one IP within one minute, per endpoint | 11th is 429 `account/rate-limited` with `Retry-After`; body reveals nothing about any email | ☐ |
| TC-NOTIFY-048 | Global send ceiling defers email | Security | P1 | cross `NOTIFY_EMAIL_HOURLY_CAP` | `notify:email` retried after the hour rolls over; `default` queue not paused (other tasks run); error logged; forgot-password still 202 | ☐ |
| TC-NOTIFY-049 | Disabled account silent | Security | P0 | forgot-password for disabled account | 202, no token, no dispatch; pre-disable token → reset rejected `account/invalid-reset-token` | ☐ |
| TC-NOTIFY-050 | Reset-token storage hardening | Security | P0(S1) | inspect token table | ≥256-bit CSPRNG, SHA-256 at rest, single-use, short TTL (`000N_account_password_reset_tokens`) | ☐ |
| TC-NOTIFY-051 | password.reset never persists in-app row | Security | P0 | trigger reset dispatch | no `notifications` row for `account.password_reset` (email-only, non-mutable) | ☐ |
| TC-NOTIFY-052 | Email retry keeps in-app copy | Reliability | P1 | fail `notify:email` for a persisting type (media.asset_ready) | Asynq retries email; in-app row intact | ☐ |
| TC-NOTIFY-053 | Password policy enforced on reset | Negative | P0 | reset with weak pw | rejected `account/password-policy` | ☐ |
| TC-NOTIFY-054 | No plaintext token in task payloads | Security | P0(S1) | forgot-password; inspect pending/retried/archived `notify:dispatch` + `notify:email` payloads | no reset token or reset URL in any payload; token minted at send time | ☐ |
| TC-NOTIFY-055 | Recipient resolved at send time | Reliability | P0 | `notify:email` for a disabled/deleted user; then with a lookup error | disabled/deleted: no send, no retry; lookup error: retried; address never in a payload | ☐ |
| TC-NOTIFY-056 | Reset pages (frontend) | Frontend | P0 | `/login` → "Forgot password?"; open the emailed `/reset-password?token=` logged out | public pages load; same "check your inbox" message after 202; Problem types mapped; `Referrer-Policy: no-referrer` | ☐ |
| TC-NOTIFY-057 | Email link trampoline | Security | P0 | logged-in user clicks an email link to `/library/media/{id}` from a cross-site page; then `next=//evil.example` | lands on the page with no login prompt; `//evil.example` refused → `/` | ☐ [MANUAL] |

## P0.4 — First event consumer

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-NOTIFY-070 | asset_ready → in-app row | Integration | P0 | video reaches ready | `notifications` row for owner within a few seconds; visible in bell next poll | ☐ (CC-5) |
| TC-NOTIFY-071 | origin=import skipped | Integration | P0 | asset_ready with origin=import (zip import) | **no** notification (bell not flooded with 300) | ☐ |
| TC-NOTIFY-072 | Click-through href present | Contract | P0 | inspect the notification | carries `data.href` (relative app path); bell renders title + navigates | ☐ |
| TC-NOTIFY-073 | Durability across notify downtime | Reliability | P0 | notify down; asset_ready fires; notify recovers | queued `media:asset_ready`→`notify:on_asset_ready` still processed (no lost notifications) | ☐ |
| TC-NOTIFY-075 | asset_ready redelivery dedup | Idempotency | P0 | deliver `notify:on_asset_ready` twice for one asset | exactly one `notifications` row (`dedup_key = asset_id`) | ☐ |
| TC-NOTIFY-074 | Single handler per task type | Integration | P0 | inspect worker mux | `notify:on_asset_ready` (not raw event) — no ServeMux collision with stream consumer | ☐ (CC-5) |

## P0.5 — Bell wiring (frontend)

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-NOTIFY-090 | No fixture data (grep) | Frontend | P0 | grep `NotificationsMenu` | `NOTIFS` fixture gone; every item is a real row | ☐ (CC-9) |
| TC-NOTIFY-091 | Poll + focus refetch + paging | Frontend | P0 | observe query; scroll the dropdown past page 1 | `useInfiniteQuery(["notifications"])` `staleTime:0`, refetchInterval ~60 s + on focus; older pages load via `useInfiniteScroll` | ☐ |
| TC-NOTIFY-092 | Optimistic mark-read, no flicker | Frontend | P0 | mark-all-read | badge shows optimistic value or settle count; never a pre-settle stale count | ☐ (CC-9) |
| TC-NOTIFY-093 | read-all sends before watermark | Frontend | P1 | trigger read-all | client sends the newest rendered item's server `cursor` field (never a client-built codec) | ☐ |
| TC-NOTIFY-095 | Click marks read + navigates | Frontend | P0 | click an unread item | navigates to `data.href`; item renders read; badge decrements by 1 | ☐ |
| TC-NOTIFY-094 | Social menus keep fixtures | Frontend | P1 | inspect Friend/Messages menus | fixtures remain (backends are non-goals) | ☐ |

## P1 — nice to have

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-NOTIFY-110 | Web push subscribe + deliver | Functional | P1 | POST push-subscription; trigger push | 201 `{id}`; delivers to all subs; prunes 410 Gone endpoints | ☐ [P1] |
| TC-NOTIFY-114 | Push endpoint takeover + delete | Functional | P1 | userB POSTs userA's endpoint; userA DELETEs that `{id}`; userB DELETEs it | endpoint re-owned by userB (upsert); userA → 404 `notify/push-subscription-not-found`; userB → 204 | ☐ [P1] |
| TC-NOTIFY-111 | SSE stream invalidation | Functional | P1 | connect SSE; new notification fires | client refetches (invalidation signal, not cache write); heartbeat ~25 s; lifetime ≤ token TTL | ☐ [P1] |
| TC-NOTIFY-112 | Preferences UI GET/PUT | Functional | P1 | PUT prefs; GET; PUT an unregistered type; PUT `muted=true` on a non-mutable type | prefs persisted; GET lists every registered type + `mutable`; 422 `notify/invalid-preference`; 422 `notify/type-not-mutable` | ☐ [P1] |
| TC-NOTIFY-113 | Security-alert on reuse detected | Security | P1 | replay a rotated refresh token | `account.security_alert` email+in-app; not mutable off | ☐ [P1] |

## Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-NOTIFY-130 | All non-2xx RFC-7807 | Contract | P0 | error paths | Problem+json + stable type | ☐ (CC-1) |
| TC-NOTIFY-131 | dedup unique index present | Contract | P1 | inspect schema | `UNIQUE (user_id, type, dedup_key) WHERE dedup_key IS NOT NULL` | ☐ |
| TC-NOTIFY-132 | Task/event registry | Contract | P1 | check events.md | all notify tasks + `media:asset_ready` subscription registered | ☐ (CC-5) |
