# Missing Features — Gap Analysis / Backlog

**Last verified:** 2026-07-06 — snapshot taken after the v1 demo loop closed; see [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md) for the living status tracker.

What is built today vs. what the spec ([feature.md](feature.md)) describes. This is
the **backlog after the v1 demo loop closed** (auth → upload → transcode → HLS
playback → logout). Use it to pick the next thing to build.

**Legend:** ✅ done · ◐ partial (UI-only or schema-only) · ○ not started · ⛔ deferred (out of v1 scope, [ADR-01](architecture/01-v1-scope-cut.md)).
**Priority:** `P1` = obvious next step / unblocks a shipped surface · `P2` = soon · `P3` = later.

> The recurring theme: **the Olympus UI shell is shipped, but most of it is
> sample data + local-only actions.** The biggest wins are wiring existing screens
> to real backends (posts, friends, messages, notifications, search).

---

## 0. Baseline — what actually works
- ✅ Local password auth (login/register/refresh/logout, remember-me, brute-force lockout, RBAC engine, audit).
- ✅ Media slice: upload → `ffmpeg` HLS → `ready` → Vidstack playback (`/upload`).
- ✅ Object storage (`platform/storage`, MinIO/R2), Asynq worker, Postgres+PgBouncer+Dragonfly, Traefik, CI + tests.
- ✅ Olympus light-theme UI shell (header, left menu, right friends panel, newsfeed, profile/notification dropdowns).

---

## 1. Account & Auth — module built, features missing
The auth core is solid; these are the surrounding account features (several have UI already).
- ○ **P1** Password reset (`forgot` / `reset` with emailed single-use token) — needs the notification module. Today: admin/CLI only.
- ○ **P1** Change password (authenticated) — UI menu item exists ("Profile Settings"), no endpoint.
- ○ **P2** Sessions / devices page — `GET /me/sessions` + revoke a specific session (query `ListActiveRefreshTokensForUser` already exists; no handler/UI).
- ○ **P2** MFA / TOTP enrol + step-up (`/auth/totp/*`) — speced ([D-27]/[D-28]), no code. Needed before the bank module.
- ○ **P2** Email verification on register.
- ○ **P3** "Login with Google" (social login) — now Portal-owned (ADR-06), previously an IdP one-liner.
- ○ **P2** Profile: real `users` profile fields (avatar upload, bio, locale/timezone editing) + a Profile page. Avatars are initials-only today.
- ○ **P2** Admin: user list / disable / role assignment UI + endpoints (`users:*`, `rbac:role:*` perms exist; no handlers/pages).

## 2. Media — slice built, gaps
- ○ **P1** Thumbnail worker — `worker.HandleThumbnail` is still a stub (extract a frame → upload → store).
- ○ **P1** Asset management: `DELETE /assets/{id}` (+ purge storage), rename/metadata edit; a media **library page** (list beyond the small `/upload` list).
- ○ **P2** Multi-rendition HLS ladder (240/480/720/1080) + master playlist — today is a single rendition.
- ○ **P2** Playback access control — HLS is currently **public**; add signed/short-lived playback or per-asset visibility.
- ○ **P2** Direct presigned upload for prod (browser→bucket): add a browser-reachable `PublicEndpoint` (Traefik route for MinIO's S3 API / R2 host). Dev uses the API-proxied path.
- ○ **P3** `media:asset_ready` event emit + a subscriber (wires into the domain verticals below).
- ○ **P3** Audio/image asset kinds (schema allows `audio`/`image`; pipeline only handles video).

## 3. Social layer — UI shipped, backend missing (the big gap)
Every item here has a **screen already built with sample data**; none has a backend. See [feature.md §9](feature.md).
- ○ **P1** **Posts / newsfeed API** — the composer posts to local state only. Need `posts` table + create/list/feed endpoints + wire `HomeView` composer & feed.
- ○ **P1** **Comments, likes/reactions, shares** on posts — counters are static.
- ○ **P1** **Friend graph** — friend requests (the header dropdown), accept/decline, friends list, "Friend Suggestions", friend groups (Close Friends/Family/Uncategorized). All sample data.
- ○ **P1** **Notifications (real)** — the bell dropdown + "Activity Feed" are hard-coded; need a notifications store + `GET /me/notifications` + SSE/poll + web-push.
- ○ **P1** **Messaging / chat** — "Olympus Chat" bar + messages dropdown are decorative; need conversations/messages + realtime.
- ○ **P1** **Search** — the header "Search here people or pages…" and "Find Friends" have no backend/results page.
- ○ **P2** **Communities / Favourite Pages** — left-menu "Fav Pages Feed" + "Pages You May Like" widget; no pages entity.
- ○ **P2** **Events / birthdays / calendar** — left-menu "Calendar and Events" / "Friends Birthdays" + Birthday card + calendar widget are static.
- ○ **P2** **Weather widget** — static; wire a weather API (or drop for v1).
- ○ **P3** Profile pages (about/photos/videos/friends), stories (24h ephemeral), follow graph, hashtags/mentions, bookmarks, feed ranking, moderation — all in §9, none started.

## 4. Domain verticals — skeleton only
`movie` / `music` / `story` / `comic` are just `module.go` + an `api/` stub (no queries, handlers, migrations, or real UI). `/library/comic` and `/library/novel/[id]` render placeholder views.
- ○ **P2** **Movies** ([feature.md §4]): `movies` schema + CRUD + list/detail pages, playback wired to a media asset, publish flow.
- ○ **P2** **Music** (§5): tracks/playlists + player (the "Music & Playlists" menu item).
- ○ **P2** **Stories** (§6): chapters/reader (the novel detail view is a skeleton).
- ○ **P2** **Comics** (§7): pages/reader (the comic index view is a skeleton).
- Each needs: migration (`000N_<name>_…`), `query/`, repository, service/handler, `MountHTTP`, and a real frontend view. They all depend on **media** for assets (already available).

## 5. Notifications module (`notify:*`) — not started
- ○ **P1** New module owning the reserved `notify:*` tasks ([MODULES.md §5.2](../../backend/MODULES.md)): email (SMTP/provider), web-push, in-app. Unblocks password reset, friend-request/notification delivery, refresh-reuse alerts. `account` already stubs `RegisterTasks` for it.

## 6. Multi-tenancy & RLS — deferred (⛔ for v1)
- ⛔ `tenant` module (organizations, memberships), Postgres **RLS** bootstrap, `cmd/sysjobs` (BYPASSRLS). Skeleton only; explicitly cut from v1 ([ADR-01](architecture/01-v1-scope-cut.md), [feature.md §2]). Revisit if multi-org is needed.

## 7. Platform / Ops
- ○ **P2** Wire the existing `platform/middleware` IP rate-limiter onto `/auth/*` at the router (built, unused).
- ○ **P3** Observability stack (metrics/logs/traces) — cut for v1 (`--profile observability`), 5-service stack in the long-horizon spec.
- ○ **P3** `cmd/sysjobs` binary (cross-tenant batch) — planned, not present.
- ○ **P3** Backups / retention (Postgres dumps, R2 lifecycle), health/readiness beyond `/healthz`.

## 8. Frontend — pages & wiring
- ◐ **P1** Placeholder actions made real: profile dropdown (Profile Settings, Create Fav Page, status), notification/friend "Settings"/"⋯", left-menu items with no route (Friend Groups, Weather App, Community Badges, Account Stats, Manage Widgets).
- ○ **P1** Header **search** input + results page; **Find Friends** page.
- ○ **P2** Missing pages: Profile, Account Settings, Messages, Friends, Events, Communities, Notifications, Search results, real Library detail pages.
- ○ **P3** Replace all UI **sample data** with API calls as the modules above land.
- ○ **P3** Bundle hls.js (Vidstack currently loads it from CDN → needs internet for playback).
- ○ **P3** Frontend tests (none yet); a11y pass on the dropdowns/menus.

## 9. API contract (OpenAPI)
- ○ **P2** `shared/openapi.yaml` exists but the generated stubs (`internal/handler/api.gen.go`, `frontend/src/lib/types.gen.ts`) are **not generated/committed** and handlers are hand-written — and the spec itself has drifted: it still lists the retired `/auth/callback` (removed by ADR-06) and is missing `/auth/register`. Whichever way the decision goes, the auth paths need updating first. Decide: adopt `oapi-codegen`/`openapi-typescript` (then wire full openapi-drift in CI), or drop the spec as the source of truth.

## 10. Deferred big modules (⛔ out of v1)
Speced in feature.md, explicitly cut by [ADR-01]:
- ⛔ **Bank / Personal Finance** (§8) — accounts, transactions, budgets, net worth; needs MFA/step-up first.
- ⛔ **Creator economy & monetisation** (§10), **Marketplace / commerce** (§11).
- ⛔ **Advanced social** (§9.13–9.37): reels, live streaming, audio rooms, karma, wiki, AMAs, verification, etc.
- ⛔ **ML safety / trust & safety dashboard** (§12.3), **company microsite** (§13).

---

## Suggested next order (P1)
1. **Notifications module** (`notify:*` + email) — unblocks password reset and every social notification.
2. **Posts + comments/likes** — makes the newsfeed real (the flagship screen).
3. **Friend graph** — friend requests/accept + friends list (wires the header dropdown + right panel).
4. **Search** — people/pages, header input + results.
5. **First domain vertical (Movies)** — proves the media→domain pattern end-to-end.
6. **Media**: thumbnail worker + delete + library page.
