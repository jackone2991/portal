# Portal — Feature List

Derived from [CLAUDE.md](CLAUDE.md) (architecture + module split) and [template-main/social/](template-main/social/) (visual/UX reference for the social layer). Each feature is mapped to the backend module that should own it ([backend/MODULES.md](backend/MODULES.md) rules apply: cross-module access goes through `api/` only).

Status legend: **✓ scaffolded** = module + some code exists, **○ planned** = directory/spec exists but empty, **△ inferred** = derived from template, not yet a module.

---

## 1. Identity, Auth & Access — module `account` ✓

Source: [backend/internal/modules/account/](backend/internal/modules/account/), CLAUDE.md §"Account module".

- **OIDC sign-in via Authentik** — `/auth/login` → `/auth/callback`; CSRF + nonce bound in a 5-min `portal_oidc` cookie. No local passwords.
- **Dual-token session** — 5-min HS256 access JWT (rotating `kid`) + 30-day 256-bit refresh token (SHA-256 at rest).
- **Cookie + Bearer modes** — `portal_access` / `portal_refresh` HttpOnly Secure cookies for browser, `Authorization: Bearer` for API clients.
- **Logout** — single-session `/auth/logout` + global `/auth/logout-all` (bumps `users.token_version`).
- **Refresh-token reuse detection** — presenting a rotated token revokes the whole chain and emits `auth.refresh.reuse_detected`.
- **`/auth/me`** — returns the current user snapshot.
- **RBAC engine** — `<resource>:<action>[:<scope>]` permission grammar, wildcards, fail-closed parser, role hierarchy (guest → user → creator → editor → moderator → admin → superadmin) with recursive-CTE effective-permission walk.
- **Permission cache** — Redis-backed, namespaced by `token_version` so revocation = cache bust in one bump.
- **Account-settings UI** (△ from template): `Account Settings`, `Change Password` (for non-OIDC fallback if added), `Personal Information`, `Education & Employment`, `Hobbies & Interests`, `Notifications` preferences.
- **Audit log** — best-effort writes via `audit.Logger`; never blocks the request.

---

## 2. Multi-tenancy — module `tenant` ○

Source: [backend/internal/modules/tenant/](backend/internal/modules/tenant/) (skeleton), `0010_rls_enable` migration in [backend/MODULES.md](backend/MODULES.md).

- **Organizations** — top-level tenant entity.
- **Memberships** — user ↔ org assignments, scoped roles.
- **RLS bootstrap** — per-request `BeginTenantScope` sets the session GUC so Postgres RLS filters every tenant-scoped table.
- **Cross-tenant batch path** — `cmd/sysjobs` uses `internal/sysrepository` (BYPASSRLS); depguard blocks anyone else from importing it.

---

## 3. Media Pipeline — module `media` ✓

Source: [backend/internal/modules/media/](backend/internal/modules/media/) (has `worker/transcode.go`, `worker/thumbnail.go`, `query/assets.sql`).

- **Asset upload** — multipart → MinIO origin bucket, tenant-prefixed key.
- **Transcode worker** (Asynq queue `transcode`, priority 5) — FFmpeg HLS ladder.
- **Thumbnail worker** (queue `thumbnail`, priority 3) — poster + sprite generation.
- **Asset state machine** — `pending → processing → ready | failed`; transitions emit `media:asset_ready` for downstream modules.
- **Signed URLs** — `mediaapi.SignedURL(assetID, ttl)` for time-limited playback.
- **CDN edge** — Cloudflare R2 in front of MinIO origin; cache invalidation hook on asset replacement.
- **HLS playback** — frontend uses Vidstack.

---

## 4. Movies — module `movie` ○

Source: [backend/internal/modules/movie/](backend/internal/modules/movie/) (skeleton); CLAUDE.md mentions `(movies)` route group in Next.js.

- **Catalog CRUD** — title, synopsis, cast, genre, year, rating.
- **Episodes / seasons** — for series.
- **Asset binding** — depends on `mediaapi` for the playable HLS asset; subscribes to `media:asset_ready` to flip `status=ready`.
- **Browse / search / filter** by genre, year, rating.
- **Watch progress** — per-user resume timestamp.
- **Continue watching** rail.
- **Ratings & reviews** (△ likely overlaps with social comments).

---

## 5. Music — module `music` ○

Source: [backend/internal/modules/music/](backend/internal/modules/music/) (skeleton); template page `Music And Playlists.html`.

- **Tracks** — metadata, artist, album, duration, asset binding via `mediaapi`.
- **Albums**, **artists**.
- **Playlists** — user-curated, public/private, collaborative (△).
- **Queue & playback state** — frontend-side, persisted per-user.
- **"Now playing"** widget.

---

## 6. Stories — module `story` ○

Source: [backend/internal/modules/story/](backend/internal/modules/story/) (skeleton); CLAUDE.md mentions `(stories)` route group.

- **Story** with **chapters** (ordered).
- **Reader** (paginated or long-scroll).
- **Reading progress** per-user.
- **Bookmarks**, **drafts**, **publish workflow** (requires `creator` role+).

---

## 7. Comics — module `comic` ○

Source: [backend/internal/modules/comic/](backend/internal/modules/comic/) (skeleton).

- **Comic** with **chapters** and **pages** (image assets via `mediaapi`).
- **Reader** — single-page, double-page, vertical-scroll modes.
- **Progress tracking**, **bookmarks**.
- **Publishing workflow** mirroring stories.

---

## 8. Social Layer △ (planned — not yet a module)

Source: [template-main/social/](template-main/social/) page inventory. Will likely become a `social/` module (or be split across `social`, `messaging`, `community`).

### 8.1 Newsfeed
- Reverse-chronological + algorithmic feed (`Newsfeed.html`, `Newsfeed - Masonry.html`).
- **Post composer** — text, image, video, link, poll (post versions: `Post Versions.html`).
- **Reactions, comments, shares**.
- **Masonry vs list** layout toggle.

### 8.2 Profile
- **Public profile** (`Profile Page.html`, `ProfilePage-LoggedOut.html`).
- Tabs: **About**, **Friends**, **Photos**, **Videos** (`Profile Page - About/Friends/Photos/Videos.html`).
- **Cover & avatar**, custom widgets (`Manage Widgets.html`).

### 8.3 Friend graph
- **Friend requests** (`Your Account - Friends Requests.html`).
- **Friend groups** (`Friend Groups.html`) — close friends, work, family, etc.
- **Block / mute**.

### 8.4 Communities / "Favourite Pages"
- **Page Feed** (`Favorit Page Feed.html`), **About** (`Favorit Page - About.html`), **Events** (`Favorit Page - Events.html`), **Tabs** (`Favourite Page With Tabs.html`).
- **Page settings & create-page popup** (`Fav Page - Settings And Create Popup.html`).
- **Roles within a page** (admin / mod / member) — slots into the existing RBAC engine with a page-scoped resource.

### 8.5 Events
- **Calendar view**, **create event** popup with **private / public** scope (`Calendar and Events - Create Event POPUP (Private_Public).html`).
- **RSVP** (going / interested / declined).
- Reminders → Asynq `notify:event_reminder`.

### 8.6 Messaging
- **Direct chat** (`Your Account - Chat Messages.html`).
- 1:1 and group threads, typing/read indicators, attachments via `mediaapi`.

### 8.7 Notifications
- In-app feed + email + push (`Your Account - Notifications.html`).
- Preferences per notification category.
- Asynq-driven: every module emits notification tasks; a single notifications module fans them out.

### 8.8 Search
- **Unified search** across people, posts, movies, music, stories, comics, events, pages (`Social Search Results.html`).

### 8.9 Community badges & gamification
- **Badges** (`Community Badges.html`) — earned for contributions, streaks, etc.

### 8.10 Statistics dashboard
- Per-user engagement view (`Statistics.html`).

### 8.11 Widgets
- **Weather widget** (`Weather Widget.html`), **sticky sidebars**, customisable per profile (`Sticky Sidebars.html`, `Manage Widgets.html`).

---

## 9. Company / Marketing Microsite △

Source: [template-main/social/Olympus Company/](template-main/social/Olympus%20Company/) — likely a separate Next.js route group, served from the same Traefik entry, possibly tenant-aware.

- **Landing / Home** (`Company Page - Home.html`).
- **About**, **Careers**, **Contacts**, **FAQs**.
- **Help & Support** + topic detail page.
- **Blog** — grid, masonry, list; **post** layouts V1 / V2 / V3.
- **Merchandise store** — product grid, masonry, product detail, shopping cart, checkout (out of scope for v1 unless explicitly needed; flag as optional).
- **Error pages** — 404 / 500.

---

## 10. Platform & Cross-cutting — `internal/platform/` ✓

Source: [backend/internal/platform/](backend/internal/platform/).

- **Config loader** (env-based) — `internal/platform/config/`.
- **DB pool** (pgx) + `BeginTenantScope` — `internal/platform/db/` ○.
- **Cache** (Redis/Dragonfly) with tenant-aware key helpers — `internal/platform/cache/` ○.
- **Storage** (S3/MinIO + R2) with tenant-prefixed keys — `internal/platform/storage/` ○.
- **Jobs** (Asynq client setup) — `internal/platform/jobs/` ○.
- **Middleware** — rate limit ✓ (`ratelimit.go`), request ID, logging, recovery.
- **Reverse proxy** — Traefik v3 routes via `docker-compose.yml` labels.

---

## 11. API Contract — `shared/openapi.yaml`

- OpenAPI is the source of truth — every endpoint flows: edit spec → `make openapi` → implement generated interface.
- Generated Go server stub: `backend/internal/handler/api.gen.go` (gitignored).
- Generated TS client types: `frontend/src/lib/types.gen.ts` (gitignored).

---

## 12. Frontend — `frontend/` (Next.js 15)

- App Router + RSC.
- Route groups: `(movies)`, `(music)`, `(stories)`, plus `(social)` / `(comics)` to add.
- **State**: Zustand (UI), TanStack Query (server state).
- **Styling**: Tailwind v4.
- **Player**: Vidstack for HLS.
- **API client**: generated from OpenAPI; auth header injected from cookie-authed session.

---

## 13. Out-of-band / Operational

- **Migrations** — single numeric sequence in `backend/db/migrations/`, files prefixed by owning module.
- **sqlc** — per-module blocks in `backend/sqlc.yaml`; output lives inside each module's `repository/`.
- **Hot-reload dev** — `make dev` (`air` for Go, `pnpm dev` for Next).
- **Tests** — `go test ./... -race -count=1` + `pnpm test`. Single test: `cd backend && go test ./internal/modules/account/rbac -run TestMatches -v`.
- **Lint** — `golangci-lint` (incl. depguard enforcing module-boundary rules) + `pnpm lint`.
- **GitNexus** — code-intelligence index (see `<!-- gitnexus:start -->` section in `CLAUDE.md`); run impact analysis before symbol edits.

---

## Roadmap priority (suggested ordering — not committed)

1. Finish wiring: `cmd/api/main.go` constructs every existing module; sqlc adapters land for `account` first.
2. Tenant module: organizations, memberships, RLS migration `0010`.
3. Media pipeline end-to-end on one asset type.
4. One vertical end-to-end (movies suggested) — catalog → playback → resume.
5. Repeat verticals for music / stories / comics, sharing the media plumbing.
6. Social layer (newsfeed → profile → friends → pages → events → messaging → notifications).
7. Company microsite + marketing pages.
8. Search & badges/gamification last.
