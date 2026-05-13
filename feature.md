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

## 8. Personal Finance / Bank — module `bank` △ (planned — not yet a module)

Tracks **every form of money a user has** in one place: accounts, transactions, debts (owed), loans (lent out), investments, savings, budgets. New module; no scaffold yet. Will follow the standard layout in [backend/MODULES.md](backend/MODULES.md) §3.

### 8.1 Accounts
- **Account types**: cash, checking, savings, credit card, loan account, investment account, retirement, crypto wallet, gift card, "other".
- **Currency per account**; multi-currency reporting via daily FX rates.
- **Opening balance**, **active / archived** state.
- **Institution metadata** (bank name, account number masked at rest).

### 8.2 Transactions
- **Debit / credit / transfer** primitives — every entry has a source or destination account (transfer = one source + one destination, no category needed).
- **Splits** — one transaction across multiple categories (e.g. supermarket bill = groceries + household + alcohol).
- **Categories** — hierarchical (Income → Salary; Expense → Food → Groceries…); seed defaults, user-extensible.
- **Tags** for cross-cutting labels (`vacation-2026`, `tax-deductible`).
- **Recurring transactions** — rent, salary, subscriptions; cron-style schedule, generated as drafts the user confirms.
- **Notes & attachments** (receipts) — via `mediaapi`.

### 8.3 Debts — money the user owes
- **Counterparty** (person or institution).
- **Principal**, **interest rate**, **schedule** (one-shot or amortising).
- **Repayment plan** — generated installments; on-time / late tracking.
- **Outstanding balance** derived from payments, not stored mutable.
- **Status**: active, paid, defaulted.

### 8.4 Loans — money the user lent out
- Mirror of Debts; same fields, opposite cash-flow direction.
- **Due-date reminders** via Asynq `notify:loan_due`.

### 8.5 Investments
- **Holdings** — positions of securities / crypto inside an investment account.
- **Cost basis** vs **current market value**; unrealised gain / loss.
- **Lots / FIFO** for accurate cost basis on partial sells.
- **Price feed** — manual entry first, pluggable provider later.
- **Dividends / interest** booked as income transactions tied to the holding.

### 8.6 Savings goals
- **Goal** = target amount + target date + linked account(s).
- **Progress** computed from contributions.
- **Auto-contribute rules** — "round up every transaction", "X% of every salary".

### 8.7 Budgets
- **Per-category caps** for a period (week / month / custom).
- **Roll-over** option for unused budget.
- **Threshold alerts** (50% / 80% / 100%) — Asynq `notify:budget_alert`.

### 8.8 Net worth & reports
- **Net worth = assets − liabilities**, time-series (daily snapshot in a denormalised table for fast reads).
- **Cash-flow report** — income vs expense by category, by period.
- **Savings rate**, **debt-to-income**, **investment performance**.
- **Forecast** — project balance trajectory from recurring rules + active goals.

### 8.9 Multi-currency
- **Per-account currency** + **reporting currency** preference on the user.
- **FX rates** snapshotted daily; historical reports value foreign accounts at the as-of-date FX.

### 8.10 Import / Export
- **CSV import** with column mapping + dedupe (hash on date+amount+counterparty).
- **OFX / QFX** later.
- **Bank API integrations** (Plaid-style) deferred to v2.
- **Export** to CSV / JSON for user-owned backup.

### 8.11 Permissions & sharing
- Default: each user owns their own bank data — RBAC scope `bank:*:own`.
- **Household sharing** — invite another user to view/edit shared accounts; uses `tenant` for the household + RBAC `bank:*:any` within it.

### 8.12 Privacy
- Bank data is the most sensitive in the system. **Encrypt account numbers and counterparty names at rest** via envelope encryption with a platform-managed key.
- **Audit every read** by anyone other than the owner; route via the shared `audit.Logger`.

### Schema ownership
Tables under `bank.*`: `accounts`, `transactions`, `transaction_splits`, `categories`, `tags`, `transaction_tags`, `recurring_rules`, `debts`, `loans`, `repayments`, `holdings`, `holding_lots`, `price_history`, `fx_rates`, `goals`, `budgets`, `budget_periods`, `networth_snapshots`. All RLS-scoped on `user_id` (or `household_id` once sharing lands). Owning module: `bank` — no other module joins these.

### Async events
- **Emits**: `bank:transaction_created`, `bank:debt_overdue`, `bank:budget_threshold_crossed`, `bank:goal_reached`.
- **Subscribes**: none initially; could consume `media:asset_ready` once receipt-attachment flow is wired.

### Migration sequencing
Bank tables land in their own migration block, e.g. `00NN_bank_init.up.sql` followed by `00NN+1_bank_investments.up.sql` etc. RLS enablement folds into the existing `*_rls_enable` migration.

---

## 9. Social Layer △ (planned — not yet a module)

Source: [template-main/social/](template-main/social/) page inventory. Will likely become a `social/` module (or be split across `social`, `messaging`, `community`).

### 9.1 Newsfeed
- Reverse-chronological + algorithmic feed (`Newsfeed.html`, `Newsfeed - Masonry.html`).
- **Post composer** — text, image, video, link, poll (post versions: `Post Versions.html`).
- **Reactions, comments, shares**.
- **Masonry vs list** layout toggle.

### 9.2 Profile
- **Public profile** (`Profile Page.html`, `ProfilePage-LoggedOut.html`).
- Tabs: **About**, **Friends**, **Photos**, **Videos** (`Profile Page - About/Friends/Photos/Videos.html`).
- **Cover & avatar**, custom widgets (`Manage Widgets.html`).

### 9.3 Friend graph
- **Friend requests** (`Your Account - Friends Requests.html`).
- **Friend groups** (`Friend Groups.html`) — close friends, work, family, etc.
- **Block / mute**.

### 9.4 Communities / "Favourite Pages"
- **Page Feed** (`Favorit Page Feed.html`), **About** (`Favorit Page - About.html`), **Events** (`Favorit Page - Events.html`), **Tabs** (`Favourite Page With Tabs.html`).
- **Page settings & create-page popup** (`Fav Page - Settings And Create Popup.html`).
- **Roles within a page** (admin / mod / member) — slots into the existing RBAC engine with a page-scoped resource.

### 9.5 Events
- **Calendar view**, **create event** popup with **private / public** scope (`Calendar and Events - Create Event POPUP (Private_Public).html`).
- **RSVP** (going / interested / declined).
- Reminders → Asynq `notify:event_reminder`.

### 9.6 Messaging
- **Direct chat** (`Your Account - Chat Messages.html`).
- 1:1 and group threads, typing/read indicators, attachments via `mediaapi`.

### 9.7 Notifications
- In-app feed + email + push (`Your Account - Notifications.html`).
- Preferences per notification category.
- Asynq-driven: every module emits notification tasks; a single notifications module fans them out.

### 9.8 Search
- **Unified search** across people, posts, movies, music, stories, comics, events, pages (`Social Search Results.html`).

### 9.9 Community badges & gamification
- **Badges** (`Community Badges.html`) — earned for contributions, streaks, etc.

### 9.10 Statistics dashboard
- Per-user engagement view (`Statistics.html`).

### 9.11 Widgets
- **Weather widget** (`Weather Widget.html`), **sticky sidebars**, customisable per profile (`Sticky Sidebars.html`, `Manage Widgets.html`).

---

## 10. Company / Marketing Microsite △

Source: [template-main/social/Olympus Company/](template-main/social/Olympus%20Company/) — likely a separate Next.js route group, served from the same Traefik entry, possibly tenant-aware.

- **Landing / Home** (`Company Page - Home.html`).
- **About**, **Careers**, **Contacts**, **FAQs**.
- **Help & Support** + topic detail page.
- **Blog** — grid, masonry, list; **post** layouts V1 / V2 / V3.
- **Merchandise store** — product grid, masonry, product detail, shopping cart, checkout (out of scope for v1 unless explicitly needed; flag as optional).
- **Error pages** — 404 / 500.

---

## 11. Platform & Cross-cutting — `internal/platform/` ✓

Source: [backend/internal/platform/](backend/internal/platform/).

- **Config loader** (env-based) — `internal/platform/config/`.
- **DB pool** (pgx) + `BeginTenantScope` — `internal/platform/db/` ○.
- **Cache** (Redis/Dragonfly) with tenant-aware key helpers — `internal/platform/cache/` ○.
- **Storage** (S3/MinIO + R2) with tenant-prefixed keys — `internal/platform/storage/` ○.
- **Jobs** (Asynq client setup) — `internal/platform/jobs/` ○.
- **Middleware** — rate limit ✓ (`ratelimit.go`), request ID, logging, recovery.
- **Reverse proxy** — Traefik v3 routes via `docker-compose.yml` labels.

---

## 12. API Contract — `shared/openapi.yaml`

- OpenAPI is the source of truth — every endpoint flows: edit spec → `make openapi` → implement generated interface.
- Generated Go server stub: `backend/internal/handler/api.gen.go` (gitignored).
- Generated TS client types: `frontend/src/lib/types.gen.ts` (gitignored).

---

## 13. Frontend — `frontend/` (Next.js 15)

- App Router + RSC.
- Route groups: `(movies)`, `(music)`, `(stories)`, plus `(social)` / `(comics)` to add.
- **State**: Zustand (UI), TanStack Query (server state).
- **Styling**: Tailwind v4.
- **Player**: Vidstack for HLS.
- **API client**: generated from OpenAPI; auth header injected from cookie-authed session.

---

## 14. Out-of-band / Operational

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
6. **Bank module** — accounts + transactions + categories first; debts, loans, investments, budgets, goals as later passes. Encryption-at-rest and audit are non-negotiable from day one.
7. Social layer (newsfeed → profile → friends → pages → events → messaging → notifications).
8. Company microsite + marketing pages.
9. Search & badges / gamification last.
