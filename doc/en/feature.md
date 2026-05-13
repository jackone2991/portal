# Portal — Feature List

Derived from [CLAUDE.md](../../CLAUDE.md) (architecture + module split) and [template-main/social/](../../template-main/social/) (visual/UX reference for the social layer). Each feature is mapped to the backend module that should own it ([backend/MODULES.md](../../backend/MODULES.md) rules apply: cross-module access goes through `api/` only).

Status legend: **✓ scaffolded** = module + some code exists, **○ planned** = directory/spec exists but empty, **△ inferred** = derived from template, not yet a module.

---

## 1. Identity, Auth & Access — module `account` ✓

Source: [backend/internal/modules/account/](../../backend/internal/modules/account/), CLAUDE.md §"Account module".

- **OIDC sign-in via Authentik** — `/auth/login` → `/auth/callback`; CSRF + nonce bound in a 5-min `portal_oidc` cookie. No local passwords.
- **Dual-token session** — 5-min HS256 access JWT (rotating `kid`) + 30-day 256-bit refresh token (SHA-256 at rest).
- **Cookie + Bearer modes** — `portal_access` / `portal_refresh` HttpOnly Secure cookies for browser, `Authorization: Bearer` for API clients.
- **Logout** — single-session `/auth/logout` + global `/auth/logout-all` (bumps `users.token_version`).
- **Refresh-token reuse detection** — presenting a rotated token revokes the whole chain and emits `auth.refresh.reuse_detected`.
- **`/auth/me`** — returns the current user snapshot.
- **RBAC engine** — `<resource>:<action>[:<scope>]` permission grammar, wildcards, fail-closed parser, role hierarchy (guest → user → creator → editor → moderator → admin → superadmin) with recursive-CTE effective-permission walk.
- **OIDC group → role sync** — `OIDC_GROUP_ROLE_MAP` env maps Authentik groups → global Portal roles; reconciled into `user_oidc_roles` on every callback. Effective permissions are union with Portal-managed `user_roles`. Tenant-scoped grants are Portal-only. Bootstrap via `BOOTSTRAP_ADMIN_OIDC_SUBJECTS`. [D-26]
- **Step-up auth** — `account.RequireACR("acr:portal:recent_mfa")` middleware on sensitive routes; 403 + `step_up_required` Problem triggers a re-auth round trip with `acr_values=mfa prompt=login`. 5-min default window. [D-27]
- **MFA enforcement** — entirely Authentik-managed (no 2FA secrets in Portal). At login, if user has any `bank:*` permission and `amr` claim lacks `mfa`, return `mfa_enrollment_required` with deep-link to Authentik's MFA dashboard. [D-28]
- **Permission cache** — Redis-backed, namespaced by `token_version` so revocation = cache bust in one bump.
- **Account-settings UI** (△ from template): `Account Settings`, `Change Password` (for non-OIDC fallback if added), `Personal Information`, `Education & Employment`, `Hobbies & Interests`, `Notifications` preferences.
- **Audit log** — best-effort writes via `audit.Logger`; never blocks the request.

---

## 2. Multi-tenancy — module `tenant` ○

Source: [backend/internal/modules/tenant/](../../backend/internal/modules/tenant/) (skeleton), `0010_rls_enable` migration in [backend/MODULES.md](../../backend/MODULES.md).

- **Organizations** — top-level tenant entity.
- **Memberships** — user ↔ org assignments, scoped roles.
- **RLS bootstrap** — per-request `BeginTenantScope` sets the session GUC so Postgres RLS filters every tenant-scoped table.
- **Cross-tenant batch path** — `cmd/sysjobs` uses `internal/sysrepository` (BYPASSRLS); depguard blocks anyone else from importing it.

---

## 3. Media Pipeline — module `media` ✓

Source: [backend/internal/modules/media/](../../backend/internal/modules/media/) (has `worker/transcode.go`, `worker/thumbnail.go`, `query/assets.sql`).

- **Asset upload** — multipart → MinIO origin bucket, tenant-prefixed key.
- **Transcode worker** (Asynq queue `transcode`, priority 5) — FFmpeg HLS ladder.
- **Thumbnail worker** (queue `thumbnail`, priority 3) — poster + sprite generation.
- **Asset state machine** — `pending → processing → ready | failed`; transitions emit `media:asset_ready` for downstream modules.
- **Signed URLs** — `mediaapi.SignedURL(assetID, ttl)` for time-limited playback.
- **CDN edge** — Cloudflare R2 in front of MinIO origin; cache invalidation hook on asset replacement.
- **HLS playback** — frontend uses Vidstack.

---

## 4. Movies — module `movie` ○

Source: [backend/internal/modules/movie/](../../backend/internal/modules/movie/) (skeleton); CLAUDE.md mentions `(movies)` route group in Next.js.

- **Catalog CRUD** — title, synopsis, cast, genre, year, rating.
- **Episodes / seasons** — for series.
- **Asset binding** — depends on `mediaapi` for the playable HLS asset; subscribes to `media:asset_ready` to flip `status=ready`.
- **Browse / search / filter** by genre, year, rating.
- **Watch progress** — per-user resume timestamp.
- **Continue watching** rail.
- **Ratings & reviews** (△ likely overlaps with social comments).

---

## 5. Music — module `music` ○

Source: [backend/internal/modules/music/](../../backend/internal/modules/music/) (skeleton); template page `Music And Playlists.html`.

- **Tracks** — metadata, artist, album, duration, asset binding via `mediaapi`.
- **Albums**, **artists**.
- **Playlists** — user-curated, public/private, collaborative (△).
- **Queue & playback state** — frontend-side, persisted per-user.
- **"Now playing"** widget.

---

## 6. Stories — module `story` ○

Source: [backend/internal/modules/story/](../../backend/internal/modules/story/) (skeleton); CLAUDE.md mentions `(stories)` route group.

- **Story** with **chapters** (ordered).
- **Reader** (paginated or long-scroll).
- **Reading progress** per-user.
- **Bookmarks**, **drafts**, **publish workflow** (requires `creator` role+).

---

## 7. Comics — module `comic` ○

Source: [backend/internal/modules/comic/](../../backend/internal/modules/comic/) (skeleton).

- **Comic** with **chapters** and **pages** (image assets via `mediaapi`).
- **Reader** — single-page, double-page, vertical-scroll modes.
- **Progress tracking**, **bookmarks**.
- **Publishing workflow** mirroring stories.

---

## 8. Personal Finance / Bank — module `bank` △ (planned — not yet a module)

Tracks **every form of money a user has** in one place: accounts, transactions, debts (owed), loans (lent out), investments, savings, budgets. New module; no scaffold yet. Will follow the standard layout in [backend/MODULES.md](../../backend/MODULES.md) §3.

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

Source: [template-main/social/](../../template-main/social/) page inventory. Will likely become a `social/` module (or be split across `social`, `messaging`, `community`).

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

### 9.12 Asymmetric follow graph (alongside friendship)

The template's social model is Facebook-style symmetric friendship (§9.3), but the project's content modules (movie/music/story/comic) all need a **creator → followers** model. Both coexist.

- **Following / followers** — one-way, no acceptance step (unless target's profile is private).
- **Following feed** distinct from algorithmic feed — only posts from people you follow.
- **Follower count visibility** setting (public / private).
- **Suggested users to follow** — algorithm + admin curation.
- **Notifications** on new follower (rate-limited if popular).
- Distinct from §9.3 friendship: you can follow without friending; you can friend without following (configurable per user).

### 9.13 Feed ranking & discovery

- **Sort modes** per feed: Hot / New / Top / Controversial (Reddit-style), per-community opt-in.
- **"For You"** algorithmic feed (Twitter-style; weight = recency × follow-graph × engagement × diversity).
- **Trending topics / hashtags** surface, scoped per locale + per community.
- **Explore / Discover** — algorithmic recommendation across content types (movies + music + stories + posts).
- **"Why am I seeing this?"** transparency popover on every algorithmic surface (regulatory hedge for EU DSA compliance).

### 9.14 Stories (ephemeral 24h)

- Photo / video stories; auto-expire after 24h (background job deletes media after 7 days for cost).
- **Story replies** routed to DM (§9.6) as quote-context.
- **Highlights** — curate saved stories permanently on profile (escape-hatch from expiry).
- Privacy: public / followers / **close-friends list**.
- Stories use media pipeline (`media:asset_ready` consumed; thumbnails generated as posters).

### 9.15 Threads, replies & quote-shares

- **Quote-share** (with commentary) vs plain re-share (no commentary). Different `post_kind`; different feed rendering.
- **Reply chains** form a **thread** on the original poster's profile (Twitter threads).
- **Nested comment threading** — configurable max depth per community (default 8); UI collapses long branches.
- **Conversation view** — render thread graph as a tree with permalinks per node.

### 9.16 Tagging & mentions

- `@mention` users in posts, comments, stories, DMs.
- Tag users in **photos** with bounding-box (clickable hot-spot).
- Tagged user can untag themselves; can require approval before tag is visible (privacy setting).
- Mentions emit `notify:social.mention` Asynq task [D-1].

### 9.17 Hashtags

- Free-text `#hashtags` parsed from post body.
- **Hashtag landing pages**: aggregate feed of all public posts using that tag.
- **Follow a hashtag** — posts under followed tags surface in your feed even when you don't follow the author.
- **Trending hashtags** widget (windowed: last 1h / 24h / 7d).
- Per-community tag prefix conventions allowed (`#tech:rust`).

### 9.18 Bookmarks / save-for-later

- Save posts (and comments) for later. **Strictly private** — no one else sees your saves.
- Optional **collections** (folders): "Read later", "Recipes", "Tax docs".
- Move/copy between collections; export collection to CSV/JSON (ties into the bank-style export pattern).

### 9.19 Reactions (rich, beyond binary)

- Six default reactions per Facebook norm: 👍 like, ❤️ love, 😂 haha, 😮 wow, 😢 sad, 😡 angry. Per-community admin can add up to 4 custom emoji.
- Per-post-type configurable (memorial posts can disable laughter).
- Reaction counts visible to author and viewers; reactor list visible to author + the reactor themselves.

### 9.20 Reddit-style voting (per-community opt-in)

- **Upvote / downvote** on posts and comments.
- Score = upvotes − downvotes; Wilson-confidence-interval ranking for "Hot" sort.
- **Karma score** aggregation (see §9.32).
- Communities choose **reactions XOR voting** (not both simultaneously, by default — both confuses users).

### 9.21 Privacy & visibility controls

- **Per-post visibility**: public / followers / friends / close-friends list / custom list / only-me (archive).
- **Profile visibility**: public / followers-only / private (request-to-follow gates posts).
- **DM gate**: anyone / followers / friends / nobody.
- **Block** (mutual cut; both sides notice) vs **Mute** (silent hide; the muted user doesn't know).
- **Mute keywords / topics** in your feed.
- **Hide online-status** indicator.
- **Privacy presets** ("Public profile", "Friends only", "Locked down") for fast onboarding.
- Privacy settings have a `https://portal/errors/privacy.changed` audit event [D-25] — security flag if changes are bulk + unusual.

### 9.22 Lists & custom feeds

- User-curated lists ("Family", "News I read", "Tech twitter"), optionally public.
- **Multireddit-style** community collections.
- **Filtered feed** — only video, only links, only text-posts, only original (no re-shares).
- Subscribe to others' public lists.

### 9.23 Photo galleries / carousels / albums

- **Multi-image post (carousel)** — up to 20 images, swipe navigation.
- **Albums** on profile ("Trip 2026", "Wedding").
- **Per-image captions**; reorder support.
- **Alt-text per image** (accessibility + i18n searchability).

### 9.24 Reels / short-form video

- Vertical short-form video feed (max 60s, hard cap 90s).
- Algorithmic discovery via shared search/ranking infra ([D-2]).
- **Music / sound overlay** — reusable audio tracks; any user's reel's audio becomes a "sound" for derivative reels (credit to original).
- **Duets / stitches** — response videos placed side-by-side or as continuation.
- Browser-side effects/filters (camera filters, AR stickers) — large frontend lift; Phase 10+ scope.

### 9.25 Live streaming

- Go-live → media pipeline runs **low-latency HLS** (LL-HLS).
- Viewer chat via `platform/realtime/` WebSocket [D-3].
- **Replay** — HLS-VOD saved automatically; streamer can delete.
- **Live reactions overlay** — floating thumbs/hearts on player.
- Concurrent viewer count.
- New Asynq events: `media:live_started`, `media:live_ended`.
- Transcode capacity ([D-13]) extended with LL-HLS settings; new env `LIVE_LATENCY_SECONDS=4`.
- **Moderation gates** — live chat enforces auto-mod rules in real-time; mods can timeout / kick.

### 9.26 Drafts, scheduling, edit history

- **Draft** state — post is private to author, no one else sees.
- **Schedule for future publish** — Asynq scheduled task fires at TZ-aware `publish_at` [D-17].
- **Edit window** — 15 min silent edit; afterwards edits show "edited" marker + full revision history.
- **Edit history view** per post (every revision diff).
- Drafts + scheduled posts live in a private "Studio" surface on profile.

### 9.27 Pinned content

- **Pin one post to top of your profile** (or up to 3 with verified-creator badge).
- **Pin one comment per post** (author privilege; mods can also pin).
- **Mod-pinned posts at community top** (up to 2).
- Pinned content carries a `pinned_at` timestamp + audit event.

### 9.28 Long-form articles

- Rich-text composer (block-based: headings, paragraphs, code, embed, image, callout, table of contents).
- Auto TOC + reading-time estimate.
- Distinct `post_kind = 'article'` — feed rendering differs (cover image + excerpt + read-time).
- Articles can be drafted, scheduled, edited (§9.26).
- Articles are also part of the search index ([D-2]).

### 9.29 Audio rooms (Spaces / Clubhouse-style)

- Live audio room with roles: **host**, **co-host**, **speaker**, **listener**.
- **Hand-raise** → request to speak; host approves.
- Recording → replay (uses media audio pipeline; AAC).
- Phase 10+ scope; opt-in feature.

### 9.30 Moderation tools

- **Report content** workflow (categories: spam, harassment, NSFW-in-SFW-context, illegal, copyright, other). Reports queue per community.
- **Mod queue** per community — sortable by report count / severity / freshness.
- Mod actions: **remove** (visible to author with reason), **lock** thread, **hide**, **pin**, **ban** member, **mute** member, **warn**, **shadow-ban** (content visible only to author).
- **Auto-mod rules** per community:
  - keyword block-list
  - link-domain block-list
  - new-account post throttle (no posts in first 24h)
  - low-karma comment delay
  - duplicate-content filter
- Per-community **audit log** via `platform/audit/` [D-25]: every mod action recorded.
- **Appeal** workflow — banned/restricted user submits appeal; community mods review; rejected appeals can escalate to platform admins.
- Platform-level **trust & safety dashboard** for `superadmin` role.

### 9.31 Content warnings & tags

- **NSFW** tag (per-community policy: allowed / forbidden / opt-in).
- **Spoiler** tag (per-post; reader sees blur until click-through).
- **Trigger warnings** (configurable category list: violence, self-harm, eating disorders, etc.).
- Reader preferences: auto-blur NSFW, hide spoiler-tagged, click-to-reveal.

### 9.32 Karma / reputation

- Per-user score from votes received.
- Two flavours: **post karma** + **comment karma**.
- **Per-community** + **global** aggregates.
- Communities can disable voting → no karma earned in that community.
- **"Cake day"** — account anniversary surface (UI flag on profile that day).
- Karma is a **mild feed-ranking signal**, never a hard gate.

### 9.33 Community wiki

- Per-community wiki pages (Reddit-style).
- **Mod-editable** by default; can opt members in (per-page permission grant).
- **Versioned history** — every revision diff-viewable; rollback supported.
- Per-community wiki search.

### 9.34 Memories / on-this-day

- **"On this day"** surface for posts from N years ago (1, 5, 10).
- **Account anniversary** post each year ("You joined Portal N years ago"); shareable.
- **Birthday reminders** for friends (opt-in; off by default for privacy).

### 9.35 AMAs / scheduled Q&A

- Scheduled Q&A session announcement; users can subscribe to be notified.
- **Question submission window** before the session.
- **Voted-up questions** float to top.
- Host marks **"answered"**; answer threads under the question.
- Replay-friendly — full Q+A persists post-session.

### 9.36 Verification & identity trust

- **Verified badge** — assigned by Portal admin via RBAC permission `account:verify`.
- **Notable-creator badges** per module (verified-movie-creator, top-musician, etc.).
- **"About this account"** card on profile: joined date, location (if shared), links, verification reason, total content count.
- **Restricted / suspended** account markers (visible to viewers, with reason if public).
- Verification is **not paid** in v1 (no Twitter Blue equivalent yet).

### 9.37 Messaging extensions (extends §9.6)

- **Message reactions** (subset of §9.19 emoji).
- **Reply-quote** a specific message in-thread.
- **Voice notes** — recorded audio messages via media pipeline.
- **Disappearing messages** — auto-delete after N seconds-read (config per thread).
- **Voice / video calls** — WebRTC; significant lift; Phase 10+ scope. Uses `platform/realtime/` for signalling [D-3] but media plane is peer-to-peer or via SFU (new infra).
- **Message search** within a thread (Postgres FTS [D-2]).
- **Forward** a message to another thread or pin as a post.

---

## 10. Creator economy & monetisation △ (planned — new module bridge)

Bridges `social` ↔ `bank`. Lives as a new module `internal/modules/creator/` whose public api/ exposes "tipping" + "subscription" surfaces. Touches social posts + bank ledger ([D-15]) so payouts route through proper double-entry.

- **Tips / awards** — viewer sends $X tip on a post, reel, or live stream. Creates a balanced ledger entry: viewer's bank account debited; creator's bank account credited (minus optional platform fee).
- **Creator subscriptions** — recurring monthly payment from subscriber to creator; subscriber gets badge + private subscriber-only posts. Asynq scheduled tasks emit `bank:transaction_created` events on each billing cycle.
- **Paid posts / paywalls** — author marks a post as paid; viewer pays once for permanent access; ledger entry similar to tip.
- **Creator analytics** — subscriber count, revenue by period, top-paying fans, churn rate.
- **Payouts** — creator can withdraw accumulated balance to an external bank account (Phase 11+; requires KYC + payment provider integration).

Roadmap: Phase 11+, after bank + social are stable.

---

## 11. Marketplace / commerce △ (planned — new module bridge)

Facebook Marketplace / Reddit r/HardwareSwap style listings. Module `internal/modules/marketplace/` bridges social listings + optional bank payments.

- **Listings** — title, price (`Money` type [D-14]), images via media, location, category, condition.
- **Categories** + **search** (per [D-2] FTS).
- **Buyer/seller chat** via §9.6 messaging (separate inbox tab).
- **Optional payment integration** — escrow via bank module's ledger; release-on-confirmation flow.
- **Fraud protection** — listing report workflow; verified-seller badge after N successful sales.
- **Out-of-scope v1** unless explicitly demanded; the template's company-merchandise pages partially cover commerce surface (see §13).

Roadmap: Phase 12+, only if commerce is in scope.

---

## 12. Privacy, data rights & anti-abuse △ (cross-cutting; not a module on its own)

Owned across **`account`**, **`platform/audit/`**, and a new **`safety`** module. Mostly cross-cutting features that compose existing modules.

### 12.1 User data rights (GDPR / CCPA)

- **Data export** — user requests a ZIP of all their data (posts, comments, bank, profile, messages). Asynq long-running task; emails download link when ready (link expires after 7 days). Lands in account module's settings UI.
- **Account deletion** — soft-delete (mark `users.deleted_at`; content remains 30 days for recovery), then hard-delete after grace period.
- **Account pause** — temporarily deactivate (cannot log in; profile invisible; doesn't trigger ban-related notifications).
- **Right to rectification** — user can edit any of their own data; backed by the edit-history pattern (§9.26).
- **Activity log** — see your own login history, recent actions, security events; sourced from `platform/audit/` [D-25].
- **Connected sessions** — list all active refresh tokens with device/UA; revoke any individually (uses the existing rotation chain).

### 12.2 Anti-abuse

- **Spam detection** — rate-limit middleware ([D-11] existing) + per-account post-velocity limits.
- **Auto-mod rules** at platform level (in addition to per-community §9.30).
- **Image classifier** (NSFW + CSAM) — out of band; integrates with `media:asset_ready` event. Open-source models (e.g. NSFWJS, Apple-style perceptual hashing); flagged content goes to a `safety_review` queue.
- **Text classifier** — toxicity, hate speech. Pluggable provider; open-source (Detoxify) or external (Perspective API). Self-hosters can disable.
- **Shadow-ban** lifecycle — content remains visible to author but no one else; uses RLS-like filter on social queries.
- **Appeals** — banned user submits appeal (§9.30); audit-logged.

### 12.3 Platform-level trust & safety dashboard

- `superadmin` role view: open mod queue, pending appeals, flagged content, abuse trends.
- Bulk-action support (ban a wave of spam accounts).
- Integration with [D-8] observability — abuse metrics on Grafana dashboard.

Roadmap: Phase 7 includes basic §9.30 community moderation; Phase 11+ adds the ML classifiers + appeals workflow + safety dashboard.

---

## 13. Company / Marketing Microsite △

Source: [template-main/social/Olympus Company/](../../template-main/social/Olympus%20Company/) — likely a separate Next.js route group, served from the same Traefik entry, possibly tenant-aware.

- **Landing / Home** (`Company Page - Home.html`).
- **About**, **Careers**, **Contacts**, **FAQs**.
- **Help & Support** + topic detail page.
- **Blog** — grid, masonry, list; **post** layouts V1 / V2 / V3.
- **Merchandise store** — product grid, masonry, product detail, shopping cart, checkout (out of scope for v1 unless explicitly needed; flag as optional).
- **Error pages** — 404 / 500.

---

## 14. Platform & Cross-cutting — `internal/platform/` ✓

Source: [backend/internal/platform/](../../backend/internal/platform/).

- **Config loader** (env-based) — `internal/platform/config/`.
- **DB pool** (pgx) + `BeginTenantScope` — `internal/platform/db/` ○.
- **Cache** (Redis/Dragonfly) with tenant-aware key helpers — `internal/platform/cache/` ○.
- **Storage** (S3/MinIO + R2) with tenant-prefixed keys — `internal/platform/storage/` ○.
- **Jobs** (Asynq client setup) — `internal/platform/jobs/` ○.
- **Realtime** (SSE + WebSocket, Dragonfly pub/sub backplane) — `internal/platform/realtime/` ○. [D-3]
- **Mail** (SMTP) — `internal/platform/mail/` ○. [D-4]
- **Observability** (OTel SDK, Prometheus `/metrics`, Sentry/GlitchTip init) — `internal/platform/observability/` ○. [D-8]
- **Audit** (cross-cutting event log, moved out of `account`) — `internal/platform/audit/` ○. [D-25]
- **Middleware** — rate limit ✓ (`ratelimit.go`), request ID, logging, recovery, **tenant URL-prefix resolver** [D-23].
- **Reverse proxy** — Traefik v3 routes via `docker-compose.yml` labels.

---

## 15. API Contract — `shared/openapi.yaml`

- OpenAPI is the source of truth — every endpoint flows: edit spec → `make openapi` → implement generated interface. **Spec-first is non-negotiable** [D-29]; CI drift check fails any handler-without-spec PR.
- **URL versioning** `/api/v{N}/`; currently `/api/v1/`. Additive changes free within a major; breaking changes require a new major + 6-month RFC 9745 sunset [D-31].
- **File layout:** single `shared/openapi.yaml` until ~2000 lines; split per-module via `$ref` afterwards [D-29].
- **Cross-module schemas** (must land in Phase 0): `Problem` (RFC 7807), `Money`, `PaginatedResult<T>`, `TenantContext` path param, `ContinuingItem` for `/api/v1/continue` [D-29].
- Generated Go server stub: `backend/internal/handler/api.gen.go` (gitignored).
- Generated TS client types: `frontend/src/lib/types.gen.ts` (gitignored).

---

## 16. Frontend — `frontend/` (Next.js 15)

- App Router + RSC. **RSC-first by default**; opt into `'use client'` only when actually needed (event handlers, hooks, browser APIs) [D-33].
- Route groups: `(movies)`, `(music)`, `(stories)`, plus `(social)` / `(comics)` to add.
- **Rendering strategy by surface** [D-33]: catalogue/detail = server components with client interactivity islands; player/reader = mostly client; account/bank = server shell + client interactivity; newsfeed = client primary with server-rendered first page.
- **State boundary** [D-32]:
  - TanStack Query — all server state (no Zustand store may hold API-fetched data).
  - Zustand — persistent UI preferences (theme, sidebar) and ephemeral UI state (modals, toasts).
  - React Hook Form — form state.
  - URL query params — shareable filter / pagination (read by TanStack).
- **Auth handoff for RSC** [D-34]: `frontend/src/lib/api-server.ts` (`import "server-only"`) wraps `fetch`, reads `cookies()`, injects `Cookie:` on outgoing requests. 401 from API → `redirect()` to `/auth/refresh-and-return?return_to=...` which calls `/auth/refresh` server-side then redirects back.
- **Same-site domain mandate** [D-34]: Next.js host + Portal API host MUST share a registrable domain (e.g. `portal.example.com` + `api.portal.example.com`) for SameSite=Strict to work. Single-domain deployments use one Traefik host with path-based routing.
- **Styling**: Tailwind v4.
- **Player**: Vidstack for HLS.
- **API client**: generated from OpenAPI; cookies forwarded automatically by the server-only wrapper.
- **Conventions doc** at `frontend/CLAUDE.md` (created in Phase 0) records the boundary rules + an anti-pattern example [D-32, D-33].

---

## 17. Out-of-band / Operational

- **Migrations** — single numeric sequence in `backend/db/migrations/`, files prefixed by owning module. **Forward-only in production** [D-12]; `migrate-down` is dev + CI roundtrip only. Use expand → migrate-data → contract across two deploys for breaking changes.
- **sqlc** — per-module blocks in `backend/sqlc.yaml`; output lives inside each module's `repository/`. CI fails on drift [D-9].
- **Hot-reload dev** — `make dev` (`air` for Go, `pnpm dev` for Next).
- **Tests** — `go test ./... -race -count=1` + `pnpm test`. Single test: `cd backend && go test ./internal/modules/account/rbac -run TestMatches -v`. Coverage targets per module in [D-9].
- **Lint** — `golangci-lint` (incl. depguard enforcing module-boundary rules) + `pnpm lint`. Optional pre-commit hook via `lefthook` [D-9].
- **CI/CD** — GitHub Actions: `.github/workflows/{ci,release}.yml` with lint, test, sqlc/openapi-drift, migration-roundtrip, build, security [D-9].
- **Observability** — opt-in `--profile observability` in `docker-compose.yml`: Loki + Prometheus + Tempo + Grafana + GlitchTip [D-8].
- **Backups** — `pgbackrest` (Postgres), MinIO → R2 replication, Dragonfly `BGSAVE`; quarterly restore drill. Targets + procedures in `docs/operations/backups.md` [D-10].
- **Secrets** — `.env` in dev, Compose/K8s secrets (or optional SOPS) in prod; rotation policy per secret class in `docs/operations/secrets.md` [D-11].
- **GitNexus** — code-intelligence index (see `<!-- gitnexus:start -->` section in `CLAUDE.md`); run impact analysis before symbol edits.

---

## 18. Roadmap (phased — not yet committed)

Each phase has explicit **deliverables** and an **exit criterion**. Phases are sequential because each lands a layer the next depends on; sub-phases inside a phase can parallelise.

### Phase 0 — Foundation wiring (immediate)

*Goal: turn the existing scaffolds into a running, end-to-end auth flow.*

- **Wire `cmd/api/main.go`** — load `platform/config`, open the pgx pool, construct `account.Module`, mount `MountHTTP(r)` under a `/api/v1` chain with the standard middleware (request-id, CORS, rate-limit, tenant). Replace the `TODO: mount OpenAPI-generated handlers` comment.
- **Run `make sqlc`** for the `account` block; commit the generated `internal/modules/account/repository/*.sql.go` artefacts (these are gitignored — they're regenerated locally, not checked in).
- **Write repository adapters** behind the interfaces account already consumes: `AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserUpserter`.
- **Split migration `0001`** — currently mixes `users` + `assets` (different modules) and has an orphan `users.role` text col that overlaps with the RBAC tables. Rewrite into `0001_platform_init` (extensions), `0002_account_users` (users only, +`locale`+`timezone`), `0003_account_rbac` (renumbered), `0005_media_assets`, etc. [D-18]
- **Add `users.locale` (BCP 47, default `'en-US'`) and `users.timezone` (IANA, default `'UTC'`)** as part of `0002_account_users`. [D-7]
- **Move `audit/` from account → `platform/audit/`** — audit is cross-cutting; account becomes a consumer. Rename event `auth.refresh.reuse_detected` → `account.refresh.reuse_detected` to fit the new `<module>.<resource>.<action>` taxonomy. [D-25]
- **Define event-type taxonomy registry** in `backend/MODULES.md` §5.3 to prevent collisions. [D-25]
- **Surface `amr`, `acr`, `auth_time` claims** into the auth context (`account/auth/context.go`) so step-up middleware [D-27] and MFA enforcement [D-28] can plug in later without rewriting the auth middleware.
- **Add `user_oidc_roles` table** to `0003_account_rbac` so the OIDC group → role sync [D-26] has somewhere to write on the first callback.
- **Adopt RFC 7807 `Problem` shape** for every 4xx/5xx in `shared/openapi.yaml`; stable `type` URIs become the i18n keys. [D-7]
- **Reserve the `notify:*` Asynq task prefix** in `backend/MODULES.md` §5.2 so future modules don't accidentally collide. [D-1]
- **Extend OpenAPI spec** — add comics + tenant tags. **Eager cross-module schemas** must land before Phase 0 closes [D-29]: `Problem` (RFC 7807 with Portal extensions like `required_acr`/`enrollment_url`), `Money`, `PaginatedResult<T>`, `TenantContext` path param, `ContinuingItem`, standard 4xx/5xx response components.
- **Lock URL versioning** — every route lives under `/api/v1/`; document the additive-only policy + RFC 9745 deprecation procedure in `docs/api/versioning.md`. [D-31]
- **Frontend server-only API client** — `frontend/src/lib/api-server.ts` wraps `fetch` with `cookies()` forwarding; `/auth/refresh-and-return` route handles RSC 401s. [D-34]
- **Frontend conventions doc** — `frontend/CLAUDE.md` documents the Zustand/TanStack/RHF state boundary [D-32] and the RSC-first rendering decision tree [D-33] with worked anti-pattern examples.
- **Land CI workflows** — `.github/workflows/ci.yml` with lint + test + sqlc-drift + openapi-drift + migration-roundtrip + build + security jobs. Drift detection from day one. [D-9]

**Exit:** a developer can `make up && make dev`, sign in via Authentik, hit `/auth/me`, and have `RequireAuth` + `RequirePermission` reject an unauthenticated call. CI fails any PR that lets generated code drift.

### Phase 1 — Tenancy + RLS

- `tenant.organizations` schema includes a **`kind` column (`'org' | 'household'`)** from day one so adding household support in Phase 5i doesn't require migrating a populated table. [D-24]
- `tenant.memberships` schema + queries; role granularity differs per kind (orgs: full hierarchy; households: owner + member only, soft cap 6).
- `0010_rls_enable.up.sql` — enable RLS on every tenant-scoped table; `USING (tenant_id = current_setting('app.tenant_id')::uuid)`.
- `platform/db.BeginTenantScope(ctx, tenantID)` sets the GUC inside the per-request tx.
- **Tenant-resolution middleware** — extract slug from `/t/{tenant}/...` URL prefix; resolve to `tenant_id`; verify membership (or `tenant=me` matches caller); set GUC. Synthetic `me` tenant per user for personal-data routes (`/t/me/bank/...`). Single-tenant deployments map `/api/v1/...` directly to a default tenant via Traefik. [D-23]
- `cmd/sysjobs` skeleton wired with the BYPASSRLS pool.
- **Observability profile** — add Loki + Prometheus + Tempo + Grafana + GlitchTip to `docker-compose.yml` behind `--profile observability`; `/metrics` endpoint on a separate port; OTel SDK auto-instruments chi + pgx + asynq. RLS performance becomes measurable from day one. [D-8]

**Exit:** an integration test proves rows for tenant A are invisible to a request bound to tenant B, while `cmd/sysjobs` sees both. Grafana shows per-route request latency split by tenant.

### Phase 2 — Media pipeline end-to-end

- Pick **video** first (it's the highest-fidelity test of the full pipeline).
- Upload endpoint → `platform/storage` → MinIO origin → enqueue Asynq `transcode`.
- Worker: FFmpeg HLS ladder (1080p/720p/480p/360p, 6s segments) + poster + sprite; writes outputs to a sibling key prefix. [D-13]
- Encoder selectable via `TRANSCODE_ENCODER` (`libx264` default; `h264_nvenc` / `h264_vaapi` / `h264_qsv` opt-in). [D-13]
- **Per-user + per-tenant transcode quotas + backpressure** — middleware checks `asynq inspect` at enqueue, rejects with 429 + Retry-After when limits crossed. [D-13]
- Emit `media:asset_ready { asset_id, hls_master_url, duration_ms, thumbnail_url }`.
- Failed transcodes go to `transcode:dead` after 3 retries; operator action required. [D-13]
- `mediaapi.GetAsset(ctx, id)` and `mediaapi.SignedURL(ctx, id, ttl)`.

**Exit:** a 30-second mp4 round-trips: upload → transcode → HLS playable in the frontend with Vidstack. A second user can't starve the queue.

### Phase 3 — First domain vertical: Movies

- `movies`, `seasons`, `episodes` schema + queries.
- Movie subscribes to `media:asset_ready` and flips `movies.status = ready`.
- Catalog endpoints: list with pagination + filters (genre, year, rating), detail, watch-progress upsert.
- Frontend `(movies)` route group: list, detail, player wired to `progress` upserts.

**Exit:** end-to-end happy path through the frontend — add a movie, transcode, browse, play, resume.

### Phase 4 — Repeat verticals: Music, Stories, Comics

- Each follows Phase 3's template.
- **Per-domain progress tables** — `movie.watch_progress`, `music.listen_progress`, `story.read_progress`, `comic.read_progress` with identical column layout. [D-20]
- **Per-domain ratings tables** — `<module>.ratings(user_id, content_id, rating, review, ...)`. [D-21]
- **`GET /api/v1/continue` aggregator** in `cmd/api` fans out to each module's `<module>api.Continue(ctx, userID, limit)`, merges, returns sorted by `updated_at DESC`. [D-20]
- Music adds playlists.
- Story/comic add chapter ordering + drafts (`creator` role gate).

**Exit:** each domain has a working browse → consume → resume loop in the frontend, plus a unified "continue" rail on the home page.

### Phase 5 — Bank (Personal Finance)

Substantial; ship in sub-phases each delivering user-visible value. Encryption-at-rest and audit are non-negotiable from sub-phase 5a.

**Phase 5 prerequisites** (gate Phase 5a):

- **`RequireACR` step-up middleware** wired into `account` module. Implementation reads `acr` + `auth_time` claims; sensitive route annotation pattern in place; frontend recognises `auth.step_up_required` Problem and runs the re-auth round trip. [D-27]
- **MFA-enforcement login gate** — at OIDC callback, if user has any `bank:*` permission and `amr` lacks `mfa`, refuse session with `auth.mfa_enrollment_required` Problem (carries Authentik enrollment URL). [D-28]
- **Authentik configured** with TOTP + WebAuthn stages and an ACR policy that elevates to `mfa` on demand. Documented in `docs/operations/authentik.md`.

- **5a — Core ledger** — `bank.currencies` (ISO 4217 + cryptos seed), `accounts` (with `type` ∈ `ASSET|LIABILITY|INCOME|EXPENSE|EQUITY`) [D-15], `categories` (auto-created income/expense accounts, hierarchical), `transactions`, `ledger_entries` with per-tx-per-currency `CHECK SUM(amount)=0` [D-15]. Money columns are `numeric(20,8)`; Go uses `shopspring/decimal` wrapped in a currency-safe `Money` value type [D-14]. Destructive operations (`accounts.delete`, `transactions.delete`) gated by `RequireACR("acr:portal:recent_mfa")` [D-27].
- **5b — Multi-currency** — `fx_rates` daily snapshots; reporting currency on `users`. Cross-currency arithmetic explicit via FX conversion entries on transactions.
- **5c — Debts** — `debts`, `repayments`, `bank.counterparties` (per-owner, optional `user_id` FK for portal-user links) [D-16].
- **5d — Loans** — mirror of debts. Two-way confirmation for settlements when counterparty is a portal user [D-16].
- **5e — Investments** — `holdings`, `holding_lots` (FIFO), `price_history`; manual price feed first. Buys/sells naturally produce balanced ledger entries [D-15].
- **5f — Budgets + goals** — `budgets`, `budget_periods`, `goals`; threshold alerts emit Asynq tasks.
- **5g — Net-worth + reports** — `networth_snapshots`; **hourly per-TZ scheduler** iterates users whose local 00:05 just passed and enqueues `bank:snapshot_daily` per user [D-17]. Cash-flow, savings rate, debt-to-income, investment performance endpoints; all date ranges computed in `users.timezone` [D-17].
- **5h — Import/export** — CSV import with column mapper + dedupe; CSV/JSON export.
- **5i — Household sharing** — create a `tenant.kind = 'household'` tenant; assign both users as `owner`; bank module's RLS predicate unchanged [D-24]. `bank:*:any` within the household.

**Exit per sub-phase:** the operation is doable through the frontend with the audit log entry present.

### Phase 6 — Notifications

Standalone `notification` module — decision settled in [D-1]. Channel decisions settled in [D-3], [D-4], [D-5].

- `notification` module owns `notifications`, `notification_preferences`, `delivery_attempts`, `push_subscriptions`.
- Asynq fan-out: every emitter publishes `notify:*` tasks; the module's worker dispatches per-channel.
- **Channels:**
  - **In-app feed** — DB row + live update via `platform/realtime/` SSE endpoint `GET /api/v1/events/stream`. [D-3]
  - **Email** — `platform/mail/` SMTP (`wneessen/go-mail`); templates under `backend/templates/email/<category>/`. [D-4]
  - **Web Push** — VAPID via `SherClockHolmes/webpush-go`; subscriptions in `notification.push_subscriptions`. No APNS/FCM in v1. [D-5]
- User preferences per category × per channel.
- Re-emit historical events into the new module on cutover (best-effort backfill from `audit_log`).

**Exit:** at least one notification per emitting module (`bank:budget_threshold_crossed`, `media:asset_ready`, `auth.refresh.reuse_detected`, `loan_due`) is delivered end-to-end through each enabled channel.

### Phase 7 — Social layer (baseline)

Core social baseline. Advanced formats (stories/reels/live/audio/voting/articles) defer to [Phase 10](#phase-10--social-advanced-formats--engagement). In sequence:

1. **Newsfeed** — posts (text/image/link/poll), **rich reactions** (§9.19), comments, **quote-shares** (§9.15), nested threading.
2. **Profile** — `social.profiles` (1:1 with `users`) for bio/education/employment/hobbies/cover/widgets. Identity-critical fields stay on `users` [D-19].
3. **Asymmetric follow graph** (§9.12) — distinct from §9.3 friendship. Following / followers, "Following" feed.
4. **Friend graph** — requests, groups, block/mute (§9.3).
5. **Communities** — pages, memberships, page-scoped RBAC, **basic moderation** (§9.30 core: report, mod queue, remove/lock/pin/ban).
6. **Events** — calendar, RSVP, reminders.
7. **Messaging** — DM 1:1 + group via [D-3].
8. **Hashtags + mentions** (§9.16, §9.17) — `@user` and `#tag` parsing, landing pages, follow-a-tag.
9. **Bookmarks** (§9.18), **pinned content** (§9.27), **drafts + scheduled posts** (§9.26).
10. **Privacy controls** (§9.21) — per-post visibility, DM gate, mute keywords, presets.
11. **Search integration** — social posts indexed via [D-2] `social/api.Search`.

**Exit:** a user can post, follow another user, join a community, react/comment/quote-share, RSVP, DM, mention via `@`, use `#hashtags`, save to bookmarks, pin a post, schedule a draft, and tune privacy settings. Mods can run a basic community.

### Phase 8 — Search & discovery

- Resolve the search engine choice (open question 2) before this phase opens.
- Per-module index builders subscribe to the relevant `*` events to keep the index hot.
- `/search?q=...&type=…` aggregator endpoint.
- Frontend command-palette / global search bar.

**Exit:** typeahead works across people, posts, movies, music, stories, comics, events, pages.

### Phase 9 — Marketing microsite + extras

- Company pages, blog (lightweight CMS).
- Badges / gamification (§9.36 verification + §9.32 karma).
- Optional merchandise store (defer unless explicitly requested).
- 404 / 500 polish.

### Phase 10 — Social: advanced formats & engagement

The "stories + reels + live + long-form" expansion. Each independent enough to ship:

1. **Stories** (§9.14) — 24h ephemeral; replies → DM; highlights; close-friends visibility tier.
2. **Reels / short-form video** (§9.24) — vertical feed; **user-uploaded audio with viral-sound attribution chain** via `social.sounds` [D-37]; duets/stitches. Browser-side effects deferred unless requested.
3. **Live streaming** (§9.25) — **RTMP ingest + LL-HLS distribution via `mediamtx` sidecar** [D-36]; live chat via `platform/realtime/`; replay is auto-VOD; per-tenant `MAX_CONCURRENT_LIVE_STREAMS_PER_TENANT` cap; new `LIVE_LATENCY_SECONDS=4` env.
4. **Photo carousels & albums** (§9.23).
5. **Long-form articles** (§9.28) — distinct `post_kind = 'article'`; rich-text composer.
6. **Reddit-style voting + karma** (§9.20, §9.32) — opt-in per community; affects feed ranking as mild signal.
7. **Feed ranking** (§9.13) — Hot/New/Top/Controversial per-community sort modes; **hand-tuned three-layer "For You" pipeline** (candidate generation → ranking → diversity) with `/settings/feed` transparency UI; chronological "Following" remains the default tab to hedge DSA risk [D-35].
8. **Lists & custom feeds** (§9.22).
9. **Content warnings** (§9.31) — NSFW / spoiler / trigger-warning tags; reader prefs.
10. **Messaging extensions** (§9.37) — reactions, reply-quote, voice notes, disappearing messages. Voice/video calls deferred to Phase 12.
11. **Advanced moderation** (§9.30) — auto-mod rules, shadow-ban, appeal workflow.
12. **Community wiki** (§9.33), **AMAs** (§9.35), **memories / on-this-day** (§9.34).
13. **Verification & creator badges** (§9.36).

**Exit:** a creator can record a story, post a reel with a reused sound (attribution chain visible), go live with chat, write a long-form article. A community can run voting + karma + auto-mod. Users see "On this day" memories. "For You" feed has a transparency popover and chronological fallback.

### Phase 11 — Creator economy

Module bridge — `internal/modules/creator/` ↔ `bank`.

1. **Tips / awards** (§10) — viewer sends tip on post/reel/live; balanced ledger entry routes platform fee + creator credit [D-15].
2. **Creator subscriptions** — recurring monthly billing via Asynq scheduled tasks → bank ledger entries.
3. **Paid posts / paywalls** — one-time payment grants permanent access; access check on render.
4. **Creator analytics** dashboard — subscriber count, MRR, churn, top fans.
5. **Payouts** — pluggable `bank/payout/Provider` interface [D-40]; **`manual` provider ships as v1 default** (operator runs payouts via wire transfer, marks the bank module); Stripe Connect lands per operator demand. Every payout gates on `RequireACR("acr:portal:recent_mfa")` [D-27].
6. **MFA mandatory** for creator-with-active-monetisation accounts [D-28].
7. **DMCA take-down workflow** for reels music + paid content [D-37].

**Exit:** a creator can publish a paid post and receive tips; subscribers are billed monthly; balances flow through the bank module's ledger; operator can complete a payout (manual or Stripe).

### Phase 12 — Marketplace + safety + voice calls

Three independent tracks; ship in any order.

1. **Marketplace** (§11) — `internal/modules/marketplace/`; listings + chat (via §9.6) + optional escrow via bank.
2. **Anti-abuse / ML moderation** (§12.2) — `internal/modules/safety/` with pluggable `ImageClassifier` + `TextClassifier` interfaces [D-38]; **defaults NSFWJS + pHash** for self-host; CSAM hash matches block + quarantine + page; NSFW flag advisory to mods; vendor APIs (AWS, Hive, Perspective) available as plug-ins.
3. **GDPR data export + account deletion** (§12.1) — Asynq long-running export task; soft-delete with 30-day grace; right-to-rectification surfaces.
4. **Voice / video calls** (§9.37) — **LiveKit SFU** for group calls; **P2P for 1:1** [D-39]; new `livekit` compose service gated behind `--profile calls`; signalling routed through Portal API for RBAC + privacy checks; `coturn` for NAT traversal.
5. **Audio rooms / Spaces** (§9.29) — LiveKit audio-only rooms with host/co-host/speaker/listener roles + hand-raise [D-39]; recording via LiveKit egress → MinIO → AAC.
6. **Platform-level T&S dashboard** for `superadmin` (§12.3) — open mod queue, flagged content, abuse trends, bulk actions; integrates with [D-8] Grafana metrics.

**Exit:** users can sell items + chat with buyers + escrow payments via bank; NSFW content is auto-tagged before publish; CSAM-matched assets are quarantined + operator paged; users can export and delete their data; 1:1 calls work peer-to-peer and group calls work via LiveKit; trust & safety has a dedicated dashboard.

---

## 19. Open questions — what we still need to analyse

Decisions deferred. Each affects at least one upcoming phase; many should land before code does. Numbers are stable refs (don't renumber when resolving — strike through and link to the decision doc).

### 16.A — Architecture / cross-cutting ✓ all resolved

1. ~~**Notifications: own module, or a sub-feature of social?**~~ → **Resolved [D-1]** — standalone `notification` module; emitters publish `notify:*` Asynq tasks, no reverse dependency.
2. ~~**Search engine choice.**~~ → **Resolved [D-2]** — Postgres FTS first; re-evaluate Meilisearch in Phase 8.
3. ~~**Realtime transport.**~~ → **Resolved [D-3]** — SSE for push streams, WebSocket for chat, Dragonfly pub/sub backplane via `platform/realtime/`.
4. ~~**Email provider.**~~ → **Resolved [D-4]** — SMTP-only via `platform/mail/`; user-configurable provider; Mailpit in dev.
5. ~~**Web/mobile push.**~~ → **Resolved [D-5]** — Web Push (VAPID) only; APNS/FCM deferred.
6. ~~**Mobile clients.**~~ → **Resolved [D-6]** — PWA-first; preserve bearer-token compatibility on every API route.
7. ~~**i18n / l10n.**~~ → **Resolved [D-7]** — frontend-only via `next-intl`; backend returns codes + RFC 7807; new `users.locale` + `users.timezone` columns.

### 16.B — Operations / infra ✓ all resolved

8. ~~**Observability stack.**~~ → **Resolved [D-8]** — Grafana stack (Loki + Prometheus + Tempo + Grafana) + GlitchTip behind opt-in `--profile observability` compose flag.
9. ~~**CI/CD.**~~ → **Resolved [D-9]** — GitHub Actions; drift + roundtrip checks; coverage targets per module.
10. ~~**Backups.**~~ → **Resolved [D-10]** — `pgbackrest` + MinIO→R2 replication + Dragonfly `BGSAVE`; quarterly restore drill; RPO/RTO matrix per surface.
11. ~~**Secret management in production.**~~ → **Resolved [D-11]** — tiered: `.env` (dev), Compose/K8s secrets (prod), SOPS optional, Vault deferred. Rotation policy per secret class.
12. ~~**Down-migration policy.**~~ → **Resolved [D-12]** — production is forward-only; downs are dev + CI roundtrip only; revert via new forward migration.
13. ~~**Transcode capacity planning.**~~ → **Resolved [D-13]** — software x264 default, NVENC/VAAPI opt-in; per-user + per-tenant quotas; backpressure on queue depth.

### 16.C — Schema / data model ✓ all resolved

14. ~~**Money / decimal representation.**~~ → **Resolved [D-14]** — `numeric(20,8)` + `shopspring/decimal` + `Money` value type with currency-safe arithmetic; `bank.currencies` seed table drives display.
15. ~~**Single-entry vs double-entry bookkeeping in bank.**~~ → **Resolved [D-15]** — hybrid double-entry internals, single-entry-feel UI; categories are auto-created income/expense accounts; `ledger_entries` has a balance `CHECK`.
16. ~~**Counterparty model in bank.**~~ → **Resolved [D-16]** — per-owner `bank.counterparties` table with optional `user_id` link; two-way confirmation for portal-user settlements.
17. ~~**Time-zone canonicalisation.**~~ → **Resolved [D-17]** — UTC `timestamptz` storage; user-TZ date boundaries everywhere; hourly per-TZ scheduler for daily snapshots.
18. ~~**Migration `0001` audit.**~~ → **Resolved [D-18]** — full split into `0001_platform_init` / `0002_account_users` (+locale/tz) / `0003_account_rbac` / `0005_media_assets` / etc.
19. ~~**Profile vs Account split.**~~ → **Resolved [D-19]** — identity-critical fields stay on `users`; bio/education/employment/hobbies move to `social.profiles` in Phase 7.
20. ~~**Shared `progress` module vs per-domain tables.**~~ → **Resolved [D-20]** — per-domain tables with shared shape; aggregator `GET /api/v1/continue` in `cmd/api`.
21. ~~**Shared ratings/reviews module vs per-domain.**~~ → **Resolved [D-21]** — per-domain `<module>.ratings` tables; no shared module; cross-domain aggregator deferred.
22. ~~**Tag/taxonomy unification.**~~ → **Resolved [D-22]** — hybrid: enumerated `text[]` genres per content table; free-text tags via `text[]` + GIN; bank categories stay in module.
23. ~~**Tenant identification path.**~~ → **Resolved [D-23]** — URL prefix `/t/{tenant}/...`; synthetic `me` tenant for personal data.
24. ~~**"Household" vs "Tenant".**~~ → **Resolved [D-24]** — household = small tenant; `tenant.kind` (`'org' | 'household'`) lands in Phase 1's initial schema.
25. ~~**Audit log table location.**~~ → **Resolved [D-25]** — moved to `platform/audit/`; `<module>.<resource>.<action>` event taxonomy registered in `backend/MODULES.md` §5.3.

### 16.D — Auth / RBAC ✓ all resolved

26. ~~**OIDC group → role sync.**~~ → **Resolved [D-26]** — hybrid two-axis grants; Authentik groups → global roles via `OIDC_GROUP_ROLE_MAP`; tenant-scoped grants are Portal-only; bootstrap via `BOOTSTRAP_ADMIN_OIDC_SUBJECTS`.
27. ~~**Step-up auth.**~~ → **Resolved [D-27]** — OIDC ACR-based; `RequireACR` middleware returns 403 + `step_up_required` Problem; explicit per-route opt-in; 5-min default window.
28. ~~**2FA / TOTP.**~~ → **Resolved [D-28]** — entirely Authentik-managed; Portal enforces "MFA required for bank-permission users" at login via the `amr` claim; settings deep-links to Authentik's MFA dashboard.

### 16.E — API / contract ✓ all resolved

29. ~~**OpenAPI coverage gap.**~~ → **Resolved [D-29]** — spec-first non-negotiable; monolith until ~2000 lines; eager cross-module schemas (Problem, Money, PaginatedResult, TenantContext, ContinuingItem) land in Phase 0.
30. ~~**Error shape.** Adopt RFC 7807 `Problem`?~~ → **Resolved [D-7]** — RFC 7807 `Problem` adopted in Phase 0 (folded into the i18n decision; `type` URIs are the i18n keys).
31. ~~**API versioning policy.**~~ → **Resolved [D-31]** — URL versioning `/api/v{N}/`; additive within major; RFC 9745 deprecation (6-month sunset) when v2 is forced.

### 16.F — Frontend ✓ all resolved

32. ~~**Zustand vs TanStack boundary.**~~ → **Resolved [D-32]** — TanStack owns server state (hard rule: no API data in Zustand); Zustand owns UI state; React Hook Form owns form state; URL params for shareable filters. Documented in `frontend/CLAUDE.md`.
33. ~~**SSR vs CSR for catalogues.**~~ → **Resolved [D-33]** — RSC-first catalogue/detail shells; client islands for interactivity; player/reader mostly client; default to server components, opt into `'use client'` only when needed.
34. ~~**Auth handoff for RSC.**~~ → **Resolved [D-34]** — server-only API client wraps `fetch` with `cookies()` forwarding; refresh-and-return route handles 401s; Next.js + API must share a registrable domain.

### 16.G — Advanced social, creator economy, safety ✓ all resolved

These surfaced from the §9-13 feature expansion. Each was deferred until its Phase came into focus; resolved as architecture choices with plug-in interfaces so individual operators can substitute alternatives.

35. ~~**"For You" algorithm.**~~ → **Resolved [D-35]** — hand-tuned three-layer pipeline (candidates → ranking → diversity); chronological "Following" is the default tab; "Why am I seeing this?" + ranking-preferences UI hedge DSA risk; ML deferred to v2.
36. ~~**Live streaming infrastructure.**~~ → **Resolved [D-36]** — RTMP ingest + LL-HLS distribution via `mediamtx` sidecar; replay is auto-VOD; live chat reuses realtime layer; per-tenant concurrent-stream cap.
37. ~~**Reels music attribution & licensing.**~~ → **Resolved [D-37]** — user-uploaded audio only; viral-sound attribution chain via `social.sounds`; DMCA take-down workflow in Phase 11; commercial library deferred to plug-in.
38. ~~**NSFW / CSAM classifier choice.**~~ → **Resolved [D-38]** — pluggable `ImageClassifier` + `TextClassifier` interfaces; defaults NSFWJS + pHash for self-host; CSAM matches block + quarantine + page; vendor APIs (AWS, Hive, Perspective) as plug-ins.
39. ~~**WebRTC SFU choice.**~~ → **Resolved [D-39]** — LiveKit SFU for group calls + audio rooms; P2P for 1:1; new `livekit` compose service gated behind `--profile calls`.
40. ~~**Creator-payout provider.**~~ → **Resolved [D-40]** — pluggable `Provider` interface; `manual` is v1 default; Stripe Connect as first real integration; other providers per-operator demand. Step-up auth ([D-27]) gates every payout.

---

## 20. Decisions log

Resolved open questions with rationale. Each entry has a stable `D-N` id. **Never edit a decision in place** — if a future PR overturns it, append a new `D-N.r1` revision below it with the new rationale and the date.

### D-1 — Notifications: standalone `notification` module *(resolves §16.A-1)*

Emitters span account, bank, media, every content module, tenant, and eventually social. If notifications were a sub-feature of social, then bank / account / media would depend on social internals — direct violation of the api-only rule in [backend/MODULES.md](../../backend/MODULES.md) §4.

**Decision:** standalone module at `backend/internal/modules/notification/`. Owns `notifications`, `notification_preferences`, `delivery_attempts`, `push_subscriptions`. Subscribes to all `notify:*` Asynq tasks; emitters never call into the module.

**Side effects:** reserve the `notify:*` Asynq prefix in `backend/MODULES.md` §5.2 (Phase 0 deliverable).

### D-2 — Search: Postgres FTS first, defer Meilisearch *(resolves §16.A-2)*

Self-hosted users push back on every extra service. Postgres FTS (`tsvector` + `pg_trgm` + GIN) covers most needs with zero new infra and inherits RLS natively. Meilisearch / Typesense win on typo-tolerance and ranking but add a service.

**Decision:** every module exposes `<module>api.Search(ctx, q, opts)` backed by `tsvector` columns and `GIN` indexes. A thin aggregator endpoint in `cmd/api` fans out across module APIs and merges. **Phase 8 re-evaluates** — if quality is insufficient, swap each module's index-builder to push docs to Meilisearch; API surface doesn't change.

### D-3 — Realtime: SSE for push, WebSocket for chat, Dragonfly backplane *(resolves §16.A-3)*

Three real-time needs: notification stream (push-only), media events (push-only), chat (bi-directional with typing/presence). The first two are SSE-shaped; only chat genuinely needs WS.

**Decision:** new `backend/internal/platform/realtime/` package exposing `Publish(ctx, channel, event)` / `Subscribe(ctx, channel) <-chan Event` over Dragonfly pub/sub. Endpoints:
- `GET /api/v1/events/stream` (SSE, authed, channel = `user:<id>`) — Phase 6.
- `GET /api/v1/chat/ws` (WebSocket via `coder/websocket`, formerly `nhooyr/websocket`) — Phase 7.

No external service (Centrifugo, Soketi, etc.) unless scale demands.

### D-4 — Email: SMTP-only via `platform/mail/` *(resolves §16.A-4)*

Locking in a vendor SDK (Resend, Postmark, SendGrid) excludes self-hosters with corporate SMTP, AWS SES, Mailgun, Mailpit, Postfix, or Gmail SMTP. SMTP is the universal floor.

**Decision:** `backend/internal/platform/mail/` exposes a `Mailer` interface with one SMTP implementation (`wneessen/go-mail`). Mailpit in dev. Templates: Go `html/template` under `backend/templates/email/<category>/*.html.tmpl`.

**New env vars** (must land in `.env.example` before Phase 6):

```
MAIL_HOST=mailpit
MAIL_PORT=1025
MAIL_USERNAME=
MAIL_PASSWORD=
MAIL_FROM_NAME=Portal
MAIL_FROM_ADDRESS=no-reply@portal.localhost
MAIL_ENCRYPTION=none           # none | tls | starttls
```

### D-5 — Push: Web Push (VAPID) only; defer APNS/FCM *(resolves §16.A-5)*

No native mobile in v1 (see [D-6]) ⇒ APNS/FCM out of scope. Web Push covers desktop Chrome/Firefox/Edge, Android Chrome, and iOS 16.4+ installed PWAs. Library: `SherClockHolmes/webpush-go`. VAPID keys are generated locally; no vendor account needed.

**Decision:** notification module owns `push_subscriptions(user_id, endpoint, p256dh, auth, user_agent, created_at, last_seen_at)`. Frontend service worker subscribes with VAPID public key; subscription POSTed to `/api/v1/notification/subscriptions`. Dispatcher prunes endpoints returning 410 Gone.

**New env vars:**

```
WEB_PUSH_VAPID_PUBLIC=
WEB_PUSH_VAPID_PRIVATE=
WEB_PUSH_SUBJECT=mailto:admin@portal.localhost
```

### D-6 — Mobile: PWA-first; preserve bearer-token compatibility *(resolves §16.A-6)*

Native mobile is real cost (React Native: shared OpenAPI client but separate auth + push; native: 2× maintenance). For v1, Next.js as installable PWA covers most usage; Web Push handles the notification surface.

**Decision:** no native client in v1. **Hard constraint preserved for v2:** every API route MUST accept `Authorization: Bearer` (cookies are convenience, not the only mode). Account module already complies. CI lint will fail any handler that 401s on a valid bearer-only request.

### D-7 — i18n: frontend-only via `next-intl`; backend returns codes + RFC 7807 *(resolves §16.A-7; also resolves §16.E-30)*

Backend strings interpolated into UI alerts is a known late-stage tax. Avoid it by returning machine-readable codes from day one. RFC 7807 `Problem` (already a candidate at §16.E-30) is the natural carrier — the `type` URI is the i18n key.

**Decision:**

1. **Error contract** — every 4xx/5xx returns RFC 7807 `Problem` with a stable `type` URI (e.g. `https://portal/errors/auth.refresh.reuse`). Lands in [shared/openapi.yaml](../../shared/openapi.yaml) in Phase 0.
2. **Money** — always `{ amount: "12345.67", currency: "USD" }` in API; never pre-formatted. Frontend uses `Intl.NumberFormat(user.locale, { style: 'currency', currency })`.
3. **Dates** — backend returns ISO 8601 UTC. Frontend formats per `users.locale` + `users.timezone`.
4. **User columns** — `users.locale TEXT NOT NULL DEFAULT 'en-US'` (BCP 47), `users.timezone TEXT NOT NULL DEFAULT 'UTC'` (IANA). Added during the migration `0001` audit (§16.C-18).

Frontend uses `next-intl`. Backend translation deferred until non-English content actually arrives — message catalogues stay on the frontend.

### D-8 — Observability: Grafana stack + GlitchTip behind opt-in compose profile *(resolves §16.B-8)*

Self-host friendliness rules out vendor SaaS (Datadog, Honeycomb, Grafana Cloud). The Grafana stack covers all four pillars (logs, metrics, traces, errors) under one UI; cost is ~5 extra compose services, made opt-in so a single-VM self-host can skip them.

**Decision:** four open-source services gated behind `--profile observability` in `docker-compose.yml`:

| Pillar | Tool | Approx RAM |
|---|---|---|
| Logs | Promtail → Loki → Grafana | ~200 MB |
| Metrics | `prometheus/client_golang` → Prometheus → Grafana | ~300 MB |
| Traces | OTel SDK → Tempo → Grafana (auto-instrument chi + pgx + asynq + http client) | ~200 MB |
| Errors | `getsentry/sentry-go` → GlitchTip | ~400 MB |

New package `backend/internal/platform/observability/` owns OTel + Sentry init. `/metrics` exposed on `METRICS_PORT` (separate from the public API port — not Traefik-routed). Sentry SDK is a no-op when `GLITCHTIP_DSN` is empty.

**New env vars:**

```
OTEL_EXPORTER_OTLP_ENDPOINT=http://tempo:4318
OTEL_SERVICE_NAME=portal-api
METRICS_PORT=9100
GLITCHTIP_DSN=                  # already present, plumb through
```

Lands alongside Phase 1 so RLS performance is measurable from day one.

### D-9 — CI/CD: GitHub Actions with drift + roundtrip checks *(resolves §16.B-9)*

Generated code (sqlc, oapi-codegen) drifts silently in review; CI must catch it. GitHub Actions is the standard for OSS with free public-repo minutes; Forgejo/Gitea Actions are compatible if the project moves later.

**Decision:** three pipelines under `.github/workflows/`:

- **`ci.yml` (per PR):**
  - `lint` — `golangci-lint run` + `pnpm lint`.
  - `test` — matrix `[unit, integration]`; integration spins postgres + dragonfly via compose.
  - `drift` — `make sqlc && git diff --exit-code` + same for `make openapi`.
  - `migration-roundtrip` — up → down → up; assert schema unchanged.
  - `build` — matrix `[api, worker, frontend]`, multi-arch (amd64 + arm64) via `docker buildx`.
  - `security` — `govulncheck ./...` + `pnpm audit --audit-level=high`.
- **`release.yml` (main + tags):** push images to GHCR with SHA tags; semver tags on release.
- Optional pre-commit via `lefthook` runs `gofmt + golangci-lint --fast + pnpm lint --fix` on staged files.

**Coverage targets** (proposed):

| Module | Target | Rationale |
|---|---|---|
| `account` | 80% | Auth + RBAC bugs are security incidents |
| `bank` | 80% | Financial correctness is non-negotiable |
| `media` | 60% | FFmpeg orchestration is integration-test heavy |
| Other modules | 60% | Standard CRUD + state machines |
| `platform/*` | 70% | Cross-cutting; breakage hits everyone |

Lands in Phase 0.

### D-10 — Backups: pgbackrest + MinIO→R2 + Dragonfly BGSAVE; restore drills *(resolves §16.B-10)*

Untested backups don't exist. Different surfaces need different recovery targets.

**RPO / RTO targets** (proposed; stakeholder confirmation outstanding):

| Surface | RPO | RTO | Why |
|---|---|---|---|
| `account` + `bank` | 5 min | 1 hour | Auth state + money is the highest stakes |
| `tenant` + `media` metadata | 1 hour | 4 hours | Recoverable but disruptive |
| Asset blobs (originals) | 24 hours | 4 hours | Users can re-upload if needed |
| `social` (once it lands) | 1 hour | 4 hours | Posts on the worst day are tolerable loss |

**Decision:**

- **Postgres** — `pgbackrest` sidecar to the Postgres container: incremental + WAL streaming (~5 min RPO for the bank/account tier). Logical `pg_dump` weekly as belt-and-braces (catches corruption that WAL replay would propagate).
- **MinIO** — continuous replication to R2 via `mc admin replicate`. Self-hosters without R2 fall back to a second MinIO node or `rclone sync` to any S3-compatible target.
- **Dragonfly** — daily `BGSAVE` → backup bucket. Asynq tasks persist on enqueue, so worst-case ~5 min RPO is met by Dragonfly's combination of in-memory + RDB snapshot.
- **Encryption at rest** in the backup bucket; per-tenant key if available.
- **Discipline** — quarterly restore drill (pick a random snapshot, restore to a sandbox, verify a recent transaction appears). Prometheus metric `backup_last_success_timestamp` → page on staleness.

New doc: `docs/operations/backups.md`. Pre-prod-launch deliverable.

### D-11 — Secrets: tiered (.env dev, Compose/K8s secrets prod, SOPS optional, Vault deferred) *(resolves §16.B-11)*

`.env` on a server is a footgun (shell-readable, leaks via backups). Vault is heavy until dynamic credentials are required. Self-host friendliness wins over feature completeness.

**Decision:**

- **Dev:** `.env` file (current).
- **Self-host prod:** Docker Compose secrets (Swarm) or Kubernetes secrets. `platform/config` reads env vars regardless of source — the orchestration injects them.
- **Git-ops shops:** SOPS-encrypted `secrets.enc.yaml` decrypted at deploy time into env. Drop-in.
- **HashiCorp Vault:** deferred until dynamic DB credentials or compliance audit force the issue.

**Rotation policy** (in `docs/operations/secrets.md`):

| Secret | Cadence | Notes |
|---|---|---|
| `JWT_SIGNING_KEYS` | Quarterly | Comma-separated key set supports overlap window |
| `OIDC_CLIENT_SECRET` | Per Authentik policy (annual typical) | |
| `WEB_PUSH_VAPID_*` | **Never** | Rotation invalidates every push subscription |
| `POSTGRES_PASSWORD` | Quarterly + on personnel change | Coordinate with PgBouncer reload |
| `S3_*` / `R2_*` | Quarterly | Atomic swap; app re-reads env on next request |
| `MAIL_PASSWORD` [D-4] | Per provider policy | |
| `GLITCHTIP_DSN` | Never (just a URL) | |

### D-12 — Migrations: forward-only in production *(resolves §16.B-12)*

`DROP COLUMN` destroys data; coordinating a down-migration with running app servers is brittle. A forward "reverse" migration is reviewable, testable, and atomic with the deploy.

**Decision:** production is **forward-only**. `make migrate-down` is reserved for:
1. Local dev — iterating on an up.sql you wrote five minutes ago.
2. CI — the migration-roundtrip job (up → down → up) catches typos in down.sql.

To revert a production schema change, ship a new forward migration that reverses it.

**Pattern: expand → migrate-data → contract** across two deploys. Examples:

- **Renaming a column:** add new column → write to both → backfill → switch reads → drop old. Three migrations, two deploys.
- **Tightening to NOT NULL:** add column nullable → backfill → add constraint.
- **Type change:** add new typed column → dual-write → backfill → switch reads → drop old.

Every up MUST be backward-compatible with the previous app version. Documented in `docs/operations/migrations.md`; a one-paragraph summary added to `CLAUDE.md` "Working in this repo".

Bank module benefits most — financial data must never be lost on a rollback.

### D-13 — Transcode: software x264 default, opt-in HW accel, quotas + backpressure *(resolves §16.B-13)*

FFmpeg is CPU-bound; on a single-VM self-host, transcoding is the limit. Without quotas, one user uploads 100 files and the queue stalls for hours.

**Decision:**

- **Encoder:** `libx264` default; opt-in `h264_nvenc` (NVIDIA), `h264_vaapi` (Intel/AMD), `h264_qsv` (Intel QSV) via `TRANSCODE_ENCODER` env. GPU passthrough recipes documented separately.
- **Concurrency:** `TRANSCODE_CONCURRENCY` per worker (default 1; sensible upper bound `max(1, nproc - 1)`).
- **HLS ladder:** 1080p / 720p / 480p / 360p, 6-second segments (Apple default). Skip 240p. Adaptive: detect input resolution, skip rungs above it.
- **Audio:** AAC 128 kbps single track unless input carries multiple language tracks.
- **Per-user quota:** `MAX_CONCURRENT_TRANSCODES_PER_USER` (default 2). Enforced at enqueue via Asynq introspection on tasks tagged with `user_id`.
- **Per-tenant cap:** `MAX_QUEUED_TRANSCODES_PER_TENANT` (default 200). Hard cap — reject with 429 + Retry-After.
- **Backpressure:** when queue depth × avg-transcode-duration > 30 minutes, reject new uploads.
- **Failure handling:** auto-retry only transient failures (network blip, OOM). After 3 retries → `transcode:dead` queue, operator action required. Codec errors and corrupt input are terminal from the first attempt.

**Sizing reference** (commodity hardware, libx264 medium preset; order-of-magnitude — measure on actual hardware):

| Source | Hardware | Wall-clock per minute of source |
|---|---|---|
| 1080p mp4 | 2 vCPU | ~3 min |
| 1080p mp4 | 4 vCPU | ~90 sec |
| 1080p mp4 | NVIDIA T4 (NVENC) | ~10 sec |
| 4K mp4 | 4 vCPU | ~6 min |
| 4K mp4 | NVIDIA T4 | ~30 sec |

**New env vars:**

```
TRANSCODE_ENCODER=libx264                 # libx264 | h264_nvenc | h264_vaapi | h264_qsv
TRANSCODE_CONCURRENCY=1                   # per worker
TRANSCODE_LADDER=1080p,720p,480p,360p     # comma-separated
TRANSCODE_HLS_SEGMENT_SECONDS=6
MAX_CONCURRENT_TRANSCODES_PER_USER=2
MAX_QUEUED_TRANSCODES_PER_TENANT=200
```

New doc: `docs/operations/transcode.md` with sizing table + GPU passthrough recipes. Lands in Phase 2.

### D-14 — Money: `numeric(20,8)` + `shopspring/decimal` + currency-safe `Money` value type *(resolves §16.C-14)*

`numeric(20,8)` (12 integer digits + 8 fractional) covers BTC precision and every fiat to penny precision. Wei-precision crypto (18 decimals) is an edge case — opt-in per-account `numeric(40,18)` only if a future requirement arrives. `int64` cents is too narrow (breaks for crypto and 3-decimal currencies like JOD). `shopspring/decimal` is ecosystem-standard.

**Decision:**

- **Storage:** every money column is `numeric(20,8) NOT NULL`.
- **Go:** `bank/internal/money/Money` wraps `shopspring/decimal.Decimal` with a currency tag. `Add(Money) (Money, error)` refuses non-matching currencies; explicit FX conversion required.
- **Wire:** string amount + ISO 4217 currency code (already mandated by [D-7]). Never JSON numbers.
- **Rounding:** `decimal.RoundBank` (banker's rounding) for halves; explicit per-call override.
- **Currency table:** `bank.currencies(code char(3) primary key, decimal_places smallint, symbol text, name text)`, seeded from ISO 4217 + common cryptos. `decimal_places` drives **display formatting only**; arithmetic uses full `numeric(20,8)` regardless.

Lands in Phase 5a.

### D-15 — Bank ledger: hybrid double-entry internals, single-entry-feel UI *(resolves §16.C-15)*

Investments alone force double-entry semantics. A buy is "−cash, +holding"; a sell-with-gain is "+cash, −holding, +investment-gain". Choosing single-entry first creates a painful migration when investments land in 5e. Paying the upfront cost ~30% more table complexity is worth the boundary discipline plus trivially provable reports.

**Decision:**

- **Account types:** `ASSET | LIABILITY | INCOME | EXPENSE | EQUITY`. Categories are auto-created INCOME/EXPENSE accounts; user never sees them as "accounts".
- **Schema:**
  ```sql
  bank.accounts(id, user_id, name, type, currency, opening_balance, ...)
  bank.transactions(id, user_id, date, description, ...)
  bank.ledger_entries(transaction_id, account_id, amount, currency)
    CHECK: SUM(amount) = 0 per (transaction_id, currency)
  ```
- **Splits** = multiple expense-side ledger entries on one transaction.
- **Transfers** = two asset-account entries (no income/expense involved).
- **Investment buy:** `holding.AAPL +$1000, checking −$1000`.
- **Investment sell with gain:** `checking +$1100, holding.AAPL −$1000, income.investment-gain +$100`.
- **UI never shows offset entries.** User-facing "transactions" feel single-entry.

Reports are provable by SQL constraint — every TB balances. Lands in Phase 5a.

### D-16 — Counterparty: per-owner table, optional `user_id` link, two-way settlement confirmation *(resolves §16.C-16)*

Free-text loses aggregation ("how much at Whole Foods this year?"). Shared rows between portal users break audit isolation and personal naming ("Mom" vs "John").

**Decision:**

```sql
bank.counterparties(
  id uuid,
  owner_user_id uuid not null,
  name text not null,           -- encrypted at rest per §8.12
  type text not null,           -- 'person' | 'institution' | 'merchant'
  user_id uuid null,            -- FK to users.id when counterparty IS a portal user
  created_at, updated_at
);
```

- Most rows have `user_id = NULL` (Whole Foods isn't a portal user).
- When `user_id` IS present, **each side owns their own row** pointing at the same portal user — independent names, independent audit.
- **Settle-a-debt UX**: matching debt/loan rows on both sides. One marks settled → other receives `notify:bank.debt.settle_pending` → must confirm before either book closes. Two-way confirmation prevents unilateral writes.

Lands in Phase 5c.

### D-17 — Time zones: UTC storage, user-TZ boundaries, hourly per-TZ snapshot scheduler *(resolves §16.C-17)*

UTC storage is the easy half. The hard half is **"when does a day start?"** — monthly reports, "yesterday's transactions", recurring rules, daily snapshots all need user-TZ semantics. Naïve fixed offsets break across DST.

**Decision:**

- **Storage:** every timestamp is `timestamptz` (UTC + offset). Never naive `timestamp`.
- **Wire:** ISO 8601 with explicit UTC offset (already in [D-7]).
- **Date boundaries:** computed in `users.timezone` (already added by [D-7]). Report queries convert at the SQL layer: `WHERE occurred_at >= ($from AT TIME ZONE $tz)::timestamptz`.
- **Recurring rules:** "1st of month" means the 1st in user TZ; the recurring-task generator must consult `users.timezone`.
- **Daily snapshots:** an **hourly** scheduler iterates `users WHERE timezone has just hit 00:05 in the last hour` and enqueues per-user `bank:snapshot_daily` tasks. Avoids fanning out N cron triggers per day.
- **Names not offsets:** IANA TZ names only (`Europe/Amsterdam`); never `UTC+1`. DST handled by IANA tzdata.

Cross-cutting; lands wherever date-bounded reports first ship (Phase 5g for bank snapshots).

### D-18 — Migration `0001` audit: full split *(resolves §16.C-18)*

No production data yet — splitting once costs less than living with mixed-concerns naming.

**Decision:** rewrite the migration tree before Phase 0 closes:

```
0001_platform_init.up.sql        extensions (uuid-ossp, unaccent, pg_trgm) + common types
0002_account_users.up.sql        users (no role col); + locale + timezone (D-7); + token_version + disabled_at
0003_account_rbac.up.sql         current 0002_account_rbac renumbered
0004_tenant_organizations.up.sql Phase 1; includes tenant.kind discriminator (D-24)
0005_media_assets.up.sql         assets table extracted from old 0001
0006_movie_init.up.sql ...
0009_comic_init.up.sql
0010_rls_enable.up.sql           RLS on every tenant-scoped table
0011+_bank_*.up.sql              Phase 5
```

`assets.owner_id` FK to `users.id` still valid because users (`0002`) lands before assets (`0005`). Audit log table moves to `platform/audit/` in the same pass (see [D-25]). Lands in Phase 0.

### D-19 — Profile vs Account split: identity on `users`, rich profile in `social.profiles` *(resolves §16.C-19)*

Piling bio/education/employment/hobbies on `users` bloats an auth-hot table read on every request. A dedicated `profile` module is premature; let social own profile pages when it arrives.

**Decision:**

- **Stays on `users`** (account module): `id`, `oidc_subject`, `email`, `display_name`, `avatar_url` (thumbnail), `locale`, `timezone`, `token_version`, `disabled_at`. Sufficient for auth + audit display strings.
- **Moves to `social.profiles`** in Phase 7 (1:1 with `users`): `bio`, `dob`, `gender`, `location`, `education`, `employment`, `hobbies`, `cover_image_url`, `widgets jsonb`.
- **No premature `profile` module.** If the profile-page surface eventually outgrows social (portfolio + resume features), revisit by extracting `profile` from social later — not now.

Phases 0–6 only use `users.display_name` + `users.avatar_url`. Account-settings UI fields beyond these don't materialise until Phase 7.

### D-20 — Progress: per-domain tables with shared shape; aggregator endpoint *(resolves §16.C-20)*

A shared `progress` module would violate "a module owns its own data" and require round-trips to validate `content_id` exists. Per-domain tables with a unified API contract preserve module boundaries.

**Decision:** each content module owns its progress table with identical column layout:

```sql
movie.watch_progress(user_id, movie_id,     position_seconds, duration_seconds, updated_at)
music.listen_progress(user_id, track_id,    position_seconds, duration_seconds, updated_at)
story.read_progress(user_id, chapter_id,    position_words,   total_words,      updated_at)
comic.read_progress(user_id, chapter_id,    page_number,      total_pages,      updated_at)
```

Cross-domain "continue" rail aggregates via each module's API:

```go
// In <module>api:
Continue(ctx context.Context, userID uuid.UUID, limit int) ([]ContinuingItem, error)

// Shared type (in platform package):
type ContinuingItem struct {
    Kind        string  // "movie" | "music" | "story" | "comic"
    ID          uuid.UUID
    Title       string
    Position    float64
    Duration    float64
    Thumbnail   string
    UpdatedAt   time.Time
}
```

`GET /api/v1/continue` aggregator in `cmd/api` fans out, merges, returns sorted by `updated_at DESC`. Lands in Phase 4.

### D-21 — Ratings: per-domain tables; no shared module *(resolves §16.C-21)*

Same shape as [D-20] but the case for centralisation is weaker — rating queries are dominated by "ratings for this content" (module-local). Cross-domain "top rated everywhere" surface is rare; deferred until UI demands it.

**Decision:** per-domain `<module>.ratings(user_id, content_id, rating smallint, review text, created_at, updated_at)`. No platform helper. Aggregator endpoint deferred. Lands per content module in Phase 4.

### D-22 — Tags / taxonomies: hybrid; genres enumerated, free-text tags as `text[]`, categories per module *(resolves §16.C-22)*

Three different beasts (closed genre enumerations, user-input free-text labels, bank's hierarchical categories) don't fit one taxonomy table. Different validation, visibility, lifecycle.

**Decision:**

- **Genres** (closed enumerations per content type): `genre TEXT[]` on each content table; seed list per module. App-layer validates against the seed list.
- **Free-text tags** (user-input labels): `tags TEXT[]` column + `GIN` index on whichever table needs them (`bank.transactions.tags`, `social.posts.tags`). Query `WHERE 'vacation-2026' = ANY(tags)` is fast under GIN. Skip the junction table — pragmatic over normalised.
- **Bank categories:** hierarchical, bank-specific, stays in module.

No centralised `tags` table. No `platform/tags/` package. Just a documented convention.

### D-23 — Tenant identification: URL prefix `/t/{tenant}/...`; synthetic `me` tenant *(resolves §16.C-23)*

Subdomain (`acme.portal.localhost`) creates DNS + wildcard-cert friction for self-hosters. Header-only (`X-Tenant`) breaks link sharing. Token-bound is fragile under multi-tenant access (refresh required to switch).

**Decision:** every tenant-scoped endpoint sits under `/t/{tenant}/...`:

- `/t/acme/api/v1/movies` — org Acme's movies.
- `/t/me/api/v1/bank/...` — personal bank data via a synthetic `me` tenant per user.
- `/api/v1/healthz` — non-tenant routes keep a flat path.

**Middleware** (`platform/middleware/tenant.go`):

1. Extract `tenant` slug from path.
2. Resolve `tenant_id` from slug (cached).
3. Verify caller is a member (membership table) OR `slug=me` matches caller's user.
4. Set `app.tenant_id` GUC for RLS via `db.BeginTenantScope`.

Single-tenant deployments map `/api/v1/...` directly to a hardcoded default tenant via a Traefik middleware rewrite — no app-side change. Lands in Phase 1.

### D-24 — Household = small tenant with `kind` discriminator *(resolves §16.C-24)*

Reuse tenancy infrastructure (RLS predicate, memberships table, audit). Storage shape is the same; role granularity and UX differ.

**Decision:** `tenant.kind` column added in the initial Phase 1 schema:

- **`org`:** full role hierarchy (admin / editor / member / viewer); unlimited members; bank module disabled by default.
- **`household`:** simplified roles (`owner` + `member` only); soft cap 6 members enforced at app layer; bank module enabled with full sharing.
- **RLS predicate unchanged:** `tenant_id = current_setting('app.tenant_id')::uuid` — indifferent to kind.
- **UX:** hides org-admin surfaces for households; hides household-specific flows for orgs.

Bank's "household sharing" (§8.11) creates a `kind='household'` tenant and assigns both users as `owner`. The `kind` column itself lands in Phase 1's `tenant.organizations` migration so Phase 5i doesn't need a schema migration on a populated table.

### D-25 — Audit log: move to `platform/audit/`; standardised event taxonomy *(resolves §16.C-25)*

Audit is cross-cutting; sitting inside account is a historical accident. Other modules (bank, tenant, media, social, notification) all need it; making them call into account violates "no cross-module dependencies on internals".

**Decision:** move `backend/internal/modules/account/audit/` → `backend/internal/platform/audit/`. Account becomes a consumer like every other module.

**Schema:**

```sql
audit_log(
  id uuid primary key,
  occurred_at timestamptz not null default now(),
  actor_user_id uuid,           -- nullable for system actions
  tenant_id uuid,               -- where
  event_type text not null,     -- '<module>.<resource>.<action>'
  resource_kind text,
  resource_id uuid,
  payload jsonb,
  ip_address inet,
  user_agent text
);
```

**Event-type taxonomy:** `<module>.<resource>.<action>` (period-separated; distinct from Asynq's `notify:*` prefix). Each module documents its events in its README; `backend/MODULES.md` §5.3 maintains an aggregated registry to prevent collisions.

Examples:

- `account.refresh.reuse_detected` (renamed from `auth.refresh.reuse_detected` to fit taxonomy)
- `bank.transaction.created`, `bank.account.created`, `bank.debt.settle_pending`
- `tenant.member.invited`, `tenant.organization.created`
- `media.asset.failed`
- `notification.delivery.failed`

Audit remains best-effort, non-blocking (per CLAUDE.md). Lands in Phase 0 alongside the migration `0001` audit ([D-18]) — the audit-log table moves files at the same time as the rename.

### D-26 — Roles: hybrid two-axis grants; tenant-scoped grants are Portal-only *(resolves §16.D-26)*

Authentik-only role management is a UX disaster — every change needs IDP admin access, and Authentik groups are global so tenant-scoped roles (`creator on tenant X`) don't fit. Portal-only management has a bootstrap problem (the very first admin has nowhere to come from). Hybrid two-axis grants give a smooth bootstrap path while keeping Portal as the source of truth for tenant-scoped roles.

**Decision:**

- **Global roles via Authentik groups.** ID token's `groups` claim maps to Portal global roles through `OIDC_GROUP_ROLE_MAP=portal-admins:admin,portal-mods:moderator,portal-creators:creator`. Reconciled into a `user_oidc_roles` join table on every callback.
- **Tenant-scoped grants are Portal-only.** Per-tenant `creator on tenant X` lives in `user_roles` and is managed via Portal's admin UI.
- **Effective permissions = `user_oidc_roles` ∪ `user_roles`**, walked through the role hierarchy.
- **Removing a user from an Authentik group** propagates on next login (reconciliation deletes the matching `user_oidc_roles` row).
- **Bootstrap admin** via env: `BOOTSTRAP_ADMIN_OIDC_SUBJECTS=sub1,sub2,sub3` grants `superadmin` at every callback for those `sub` values; remove from env once a Portal admin can manage roles in-app. Secondary `BOOTSTRAP_ADMIN_GROUPS=portal-bootstrap` accepted in case the operator prefers an Authentik-group-based bootstrap.

**Schema addition** (lands with the migration audit, [D-18], in `0003_account_rbac`):

```sql
user_oidc_roles(
  user_id uuid not null references users(id) on delete cascade,
  role_id uuid not null references roles(id),
  authentik_group text not null,    -- source-of-truth for re-sync
  synced_at timestamptz not null default now(),
  primary key (user_id, role_id)
);
create index on user_oidc_roles(authentik_group);
```

Distinct table from `user_roles` so we always know which grant came from where. Audit events `account.role.granted_via_oidc` / `account.role.revoked_via_oidc` fire on every reconciliation.

**New env vars:**

```
OIDC_GROUP_ROLE_MAP=portal-admins:admin,portal-mods:moderator,portal-creators:creator
BOOTSTRAP_ADMIN_OIDC_SUBJECTS=
BOOTSTRAP_ADMIN_GROUPS=
```

Lands in Phase 0 (`user_oidc_roles` table) and the OIDC callback handler.

### D-27 — Step-up auth: OIDC ACR-based; sensitive ops annotated explicitly *(resolves §16.D-27)*

Sensitive bank + account + tenant operations need fresher guarantees than "this session existed five hours ago". Re-prompting via OIDC `acr_values` is standard practice (GitHub, Google, AWS all do equivalents).

**ACR levels for Portal:**

| Level | Meaning |
|---|---|
| `acr:portal:basic` | Single-factor (OIDC password only). |
| `acr:portal:mfa` | Second factor verified this session. |
| `acr:portal:recent_mfa` | Second factor verified within last 5 minutes. |

**Middleware:**

```go
r.With(account.RequireACR("acr:portal:recent_mfa")).
  Delete("/bank/accounts/{id}", h.DeleteAccount)
```

`RequireACR` reads `acr` + `auth_time` claims from the access token. Insufficient → 403 with RFC 7807 `Problem`:

```json
{
  "type":         "https://portal/errors/auth.step_up_required",
  "title":        "Step-up authentication required",
  "status":       403,
  "required_acr": "acr:portal:recent_mfa",
  "return_to":    "/api/v1/t/me/bank/accounts/abc-123"
}
```

Frontend recognises the `type`, redirects to `/auth/login?step_up=mfa&return_to=...`, which re-runs OIDC with `acr_values=mfa prompt=login`. After successful re-auth, the new access token's `acr` claim allows the operation.

**Step-up window:** 5 minutes by default; configurable per middleware call (`RequireACR("...", account.WithWindow(2*time.Minute))`).

**Initial gated set** (no implicit list — every gated route opts in):

| Module | Operation | ACR |
|---|---|---|
| bank | `accounts.delete` | `recent_mfa` |
| bank | `transactions.delete` | `recent_mfa` |
| bank | `export.csv`, `export.json` | `recent_mfa` |
| bank | `household.invite` | `recent_mfa` |
| bank | `debt.settle` (counterparty is portal user) | `recent_mfa` |
| account | `delete_self`, `email.change`, `mfa.disable` | `recent_mfa` |
| tenant | `organization.delete`, `ownership.transfer` | `recent_mfa` |

Lands jointly with [D-28] as a Phase 5 prerequisite.

### D-28 — 2FA: entirely Authentik-managed; Portal enforces MFA at login for bank-permission users *(resolves §16.D-28)*

Authentik already ships TOTP, WebAuthn, SMS, push, recovery codes, and a polished enrollment UX. Reimplementing any of this in Portal duplicates work, adds a second 2FA secret store to compromise, and splits the user's mental model.

**Decision:**

- **No 2FA logic in Portal.** No TOTP secrets stored, no recovery codes generated. Authentik owns the entire surface.
- **Login-time enforcement.** If the authenticated user has any `bank:*` permission and the ID token's `amr` claim doesn't include `mfa`, refuse session with a `Problem` of type `https://portal/errors/auth.mfa_enrollment_required` carrying an `enrollment_url`. Frontend redirects user to enroll, then resumes original flow.
- **Auth-context surfacing.** Middleware exposes `amr`, `acr`, `auth_time` claims so [D-27] and future MFA-aware code can read them without re-parsing the JWT.
- **Settings UI deep-link.** Account-settings page has a "Manage MFA" button opening Authentik's user dashboard:

  ```
  ${OIDC_ISSUER}/if/user/#/settings;%7B%22page%22%3A%22page-mfa%22%7D
  ```

- **Required Authentik config** (documented in `docs/operations/authentik.md`):
  - Stages: "TOTP authenticator setup" + "WebAuthn authenticator setup" (recommended).
  - Authentication flow: prompt MFA when `acr_values=mfa` is requested in the auth URL.
  - Group: `portal-bank-users` (or any tag) — used by an Authentik policy to gate the MFA-required flow on the IDP side too, as defence in depth.

Lands jointly with [D-27] as a Phase 5 prerequisite. Step-up to a single-factor session adds no security, so D-27 and D-28 are useless without each other.

### D-29 — OpenAPI: spec-first non-negotiable; monolith until ~2000 lines; eager cross-module schemas in Phase 0 *(resolves §16.E-29)*

The OpenAPI spec is the contract for both Go server stubs and TS client types. Letting handlers drift from spec defeats the codegen story. Drift detection ([D-9]) catches the symptom; spec-first as policy avoids the cause.

**Decision:**

- **Process — spec-first.** Every new endpoint MUST be added to `shared/openapi.yaml` before its handler exists. CI fails any PR where `make openapi && git diff --exit-code` finds changes.
- **File layout — monolith until ~2000 lines.** Then split per-module via OpenAPI `$ref` into `shared/openapi/{module}.yaml` with a root that includes them. Don't pre-split at 400 lines.
- **Eager-spec inventory** (must land before Phase 0 closes):
  - `Problem` schema (RFC 7807, with Portal extensions: `required_acr`, `enrollment_url`, `return_to`).
  - `Money` schema (`{ amount: string, currency: string }`) [D-7, D-14].
  - `PaginatedResult<T>` (cursor-based: `{ items: T[], next_cursor: string|null }`).
  - `TenantContext` path parameter contract [D-23].
  - `ContinuingItem` schema for `/api/v1/continue` aggregator [D-20].
  - Standard 4xx/5xx response component refs.
- **Per-module endpoints** land with each module's `MountHTTP` (movie endpoints when movie ships, bank endpoints when bank ships). Aggregator endpoints + cross-module schemas land in Phase 0.

### D-31 — API versioning: URL versioning `/api/v{N}`; additive within major; RFC 9745 sunset for v2 *(resolves §16.E-31)*

URL versioning is already implicit in `/api/v1/...` throughout the codebase. Header-based (`X-API-Version`, `Accept: vnd.portal.v1+json`) is clean URL-side but invisible to debugging. Date-versioning (Stripe-style) is heavy for a self-host product. URL versioning aligns with current code and is the simplest answer.

**Decision:**

- **URL versioning.** Every API route lives under `/api/v{N}/`. Currently `/api/v1/`.
- **Within a major — additive only:**

  | Free | Breaking |
  |---|---|
  | New endpoints | Removing/renaming fields or endpoints |
  | New optional request fields | Changing field type or semantics |
  | New response fields | Tightening validation (new required field, shorter max length) |
  | New enum values (clients MUST accept unknown) | Removing enum values |

- **New major only when forced.** Process for v2:
  1. RFC issue describing the breaking change + alternatives considered.
  2. Deprecate v1 endpoint with `Deprecation: true` + `Sunset: <date>` headers (RFC 9745) at least **6 months** before removal.
  3. v1 and v2 coexist during the sunset window.
  4. Migration doc in `docs/api/migrating-v1-to-v2.md`.
- **CI gate:** OpenAPI drift check ([D-9]) compares spec against `main` and flags shape-breaking diffs (removed paths, removed fields, type changes). PR description must explicitly waive the flag with reason.

Self-hosters pin their frontend to a known API version, so even if a hosted instance moves to v2, a self-hosted frontend doesn't break.

### D-32 — State boundary: TanStack for server state, Zustand for UI state, RHF for forms *(resolves §16.F-32)*

The footgun is stuffing server-derived data into Zustand "for convenience" → manual sync → race conditions and stale-data bugs. Or building "derived Zustand stores" that duplicate TanStack's cache.

**Decision:**

| State category | Owner | Examples |
|---|---|---|
| **Server state** | TanStack Query | movie list, user profile, transactions, current user session |
| **UI state (persistent)** | Zustand + `persist` middleware | theme, sidebar collapsed, layout density |
| **UI state (ephemeral)** | Zustand (transient store) | active toast, command palette open, current modal |
| **Form state** | React Hook Form | drafts of any form before submit |
| **Shareable filter/pagination** | URL query params (read by TanStack) | `?page=2&sort=date&genre=action` |

**Hard rule:** no Zustand store may hold data fetched from the API. If you find yourself writing `setMovies(await fetch(...))`, you've taken a wrong turn — use TanStack's `useQuery` instead.

Documented in `frontend/CLAUDE.md` (created in Phase 0) with a worked anti-pattern example so contributors don't repeat the mistake.

### D-33 — Rendering: RSC-first; client islands for interactivity; player/reader mostly client *(resolves §16.F-33)*

Next.js 15 App Router is RSC-first by design. Reflexive `'use client'` everywhere forfeits SEO + bundle savings + streaming UX.

**Decision** — surface-by-surface:

| Surface | Mode | Why |
|---|---|---|
| Movie / music / story / comic **catalogue** | Server components | SEO; streaming HTML; personalisation via `cookies()` |
| **Detail** pages (single movie, track) | Server shell + client interactivity island | Metadata SEO-relevant; player must be client |
| **Player / reader** | Mostly client | Stateful, post-auth, SEO irrelevant |
| **Account settings, bank** | Server shell + client islands | Interactive but private; ergonomic to fetch server-side |
| **Newsfeed** (Phase 7) | Client primary; SSR first page | Highly interactive; realtime updates |

**Practical rules:**

- Default to server components. Opt into `'use client'` only when actually needed.
- Server components fetch through the Portal API over the Docker network — same-region latency is fine for SEO-relevant pages.
- Public catalogue pages use `next.revalidate` (ISR); per-user data uses `cache: 'no-store'`.
- Decision tree documented in `frontend/CLAUDE.md` alongside [D-32].

### D-34 — RSC auth handoff: cookie forwarding via `cookies()`; same-site domain mandate; refresh via redirect-and-return *(resolves §16.F-34)*

Three sub-problems:

1. **Cookie forwarding** — Next.js `cookies()` API gives RSC access to the request cookie store; outgoing fetches need explicit injection.
2. **SameSite=Strict** — same-site is registrable-domain (eTLD+1). Same-site means `portal.example.com` + `api.portal.example.com` work; `portal.com` + `api.portal-app.com` don't.
3. **Token refresh during RSC** — what happens when access token expires mid-render?

**Decision:**

- **Cookie scheme unchanged.** `portal_access` HttpOnly Secure SameSite=Strict Path=/; `portal_refresh` same but Path=/auth.
- **Same-site domain mandate.** Next.js and Portal API MUST share a registrable domain (e.g. `portal.example.com` + `api.portal.example.com`). Single-domain deployments use one Traefik host with path-based routing (`/api/*` → Go, `/*` → Next). Documented in `docs/operations/deployment.md`.
- **Server-only API client.** `frontend/src/lib/api-server.ts` with `import "server-only"` directive wraps `fetch` to read `cookies()` and inject `Cookie:` header on every outgoing request.
- **Refresh strategy — redirect on 401.** RSC fetches API; 401 → throw Next.js `redirect()` to `/auth/refresh-and-return?return_to=<path>`. That route runs server-side, calls `/auth/refresh` (refresh cookie's `Path=/auth` makes it sent), gets new access cookie, redirects back. User sees one navigation flash.
- **Future optimisation** (Phase 3 frontend): Next.js middleware proactively refreshes when access cookie is < 1 minute from expiry. Avoids the 401-redirect round trip in the common case. Not mandatory for v1.
- **CSRF.** Next.js server actions are origin-checked by the framework. Combined with SameSite=Strict, the threat surface is closed.

Lands in Phase 0 (server-only API client + refresh-and-return route).

### D-35 — "For You" feed: hand-tuned three-layer pipeline; "Following" chronological is default; DSA-aligned transparency *(resolves §16.G-35)*

ML-driven personalisation is a research project; hand-tuned signal weights ship far sooner with transparent behaviour. Defaulting to chronological "Following" + making "For You" opt-in sidesteps most DSA risk by giving users meaningful choice.

**Decision — three-layer ranking pipeline:**

1. **Candidate generation** — emit ~1000 candidates per request:
   - Posts from followed users (§9.12)
   - Posts from joined communities (§9.4)
   - Posts with followed hashtags (§9.17)
   - Posts reacted to by friends (§9.3)
   - Trending in viewer's region/communities
   - Optional admin "editor's picks"
2. **Ranking** — hand-tuned score per candidate. v1 weights:

   ```
   score = 0.40·recency_decay(age, half_life=12h)
         + 0.30·author_affinity(viewer, author)
         + 0.15·engagement_z_score(post)
         + 0.15·topic_calibration(viewer_history, post_topics)
         - 2.00·negative_signal(muted_words, blocks, "not interested")
   ```
3. **Diversity** — walk sorted list; cap at 2 consecutive items from same author / community / hashtag; reshuffle to enforce.

**Transparency (DSA Art. 27-aligned):**

- **Per-item popover "Why am I seeing this?"** — top 3 contributing signals with normalised weights.
- **`/settings/feed` UI** — toggle/weight signal categories; "reverse chronological only" toggle disables ranking entirely.
- **Default tab is "Following"** (chronological). "For You" is opt-in. Users actively choose algorithmic ranking — meaningful consent.

**Infrastructure:**

- Ranking runs at request time; hot feeds cached in Dragonfly for 60s.
- No ML models v1; all signals from existing tables (`social.reactions`, `social.follows`, `social.community_memberships`).
- Phase 13+: offline candidate-set pre-computation via Asynq cron; per-user embeddings.

Lands in Phase 10.

### D-36 — Live streaming: RTMP ingest + LL-HLS via `mediamtx`; replay is auto-VOD; per-tenant cap *(resolves §16.G-36)*

RTMP is universal (every streaming tool supports it). LL-HLS reuses the existing VOD pipeline + CDN edge. SRT and WebRTC ingest are higher-quality but smaller user bases; defer until demand surfaces.

**Decision:**

- **Ingest:** RTMP. New `mediamtx` container (Go-native open-source gateway) in `docker-compose.yml`; converts RTMP → LL-HLS on the fly; writes segments to MinIO.
- **Distribution:** LL-HLS via existing media pipeline. Apple Low-Latency HLS spec; CDN-friendly via HTTP/2.
- **Latency target:** `LIVE_LATENCY_SECONDS=4` env (§9.25).
- **Replay:** on `media:live_ended`, FFmpeg concatenates accumulated segments → standard HLS VOD asset → emits `media:asset_ready`. The live becomes a normal video on streamer's profile.
- **Live chat:** `platform/realtime/` WebSocket [D-3]; per-stream channel `live:{stream_id}` in Dragonfly pub/sub. Stream-specific mod controls: emote-only mode, follower-only mode, slow-mode (one msg per N seconds per user).
- **Capacity** (extends [D-13]):
  - Each 1080p LL-HLS encode ≈ 2 vCPU sustained.
  - `MAX_CONCURRENT_LIVE_STREAMS_PER_TENANT=3` env.
  - Step-up auth ([D-27]) gates "Go Live" if streamer hasn't streamed before.
- **Deferred:** SRT ingest, WebRTC ingest (sub-second), origin/edge split. `mediamtx` supports all three; switching is config-only when scale demands.

**New events:** `media:live_started`, `media:live_ended` consumed by social module to flip live-indicator state on streamer's profile.

Lands in Phase 10.

### D-37 — Reels music: user-uploaded audio only; viral-sound attribution chain *(resolves §16.G-37)*

Commercial-library deals (Sony/Universal/Warner) are non-starters for a self-host product — multi-million annual costs, per-region clearance complexity, and most operators don't need them.

**Decision:**

- **Audio source:** user-uploaded only. When user A uploads a reel, the reel's audio becomes a reusable **"sound"** in `social.sounds`. Other users' reels can attribute and reuse that sound.
- **Attribution chip** on every derivative reel: "Sound by @userA · 12.3k reels using this sound" — clickable, leads to sound-detail page with all reels using it.
- **Schema:**
  ```sql
  social.sounds(
    id uuid primary key,
    source_reel_id uuid references social.reels(id),
    original_creator_user_id uuid references users(id) on delete set null,
    audio_asset_id uuid references media.assets(id),
    duration_seconds float,
    use_count int default 0,
    name text,                          -- creator-named; default "Original sound"
    created_at timestamptz
  );
  ```
- **Derivative reel:** `social.reels.sound_id` set; `social.sounds.use_count` incremented via Asynq task (avoids hot-row contention).
- **DMCA take-down workflow** (Phase 11+): rights-holder submits a notice → operator reviews → audio asset removed; cascading: derivative reels keep their video, lose audio with a "sound removed" marker. Audit-logged.
- **Plug-in shape for v2:** `MusicLibrary` interface in `social/music_library/` lets operators integrate licensed catalogues (e.g. Epidemic Sound) without rewriting reel logic. Deferred.

Lands in Phase 10.

### D-38 — Safety classifiers: pluggable image + text interfaces; defaults NSFWJS + pHash; CSAM matches block + quarantine + page *(resolves §16.G-38)*

Two distinct problems with very different stakes. CSAM is mandatory, illegal-to-host, with NCMEC/IWF reporting obligations. NSFW is policy-driven, operator/community-configurable. The architecture must let operators substitute classifiers freely while keeping the workflow deterministic.

**Decision — interfaces:**

```go
package safety

type ImageClassifier interface {
  Classify(ctx context.Context, asset Asset) (ImageClassification, error)
}
type ImageClassification struct {
  IsNSFW         bool
  NSFWScore      float64           // 0..1
  CSAMHashMatch  bool
  RawScores      map[string]float64
}

type TextClassifier interface {
  Classify(ctx context.Context, text string) (TextClassification, error)
}
type TextClassification struct {
  Toxicity float64
  Threat   float64
  Hate     float64
  Identity float64
  Raw      map[string]float64
}
```

**Ships v1:**

- `safety/classifier/nsfwjs` — ONNX-compatible NSFWJS model in worker; runs locally; MIT.
- `safety/classifier/phash` — perceptual-hash matcher against an operator-supplied CSAM hash list. Default list is empty; operator loads from NCMEC / IWF / equivalent after legal partnership.
- `safety/classifier/detoxify` — text classifier; PyTorch ONNX-compatible; MIT.

**Available as plug-ins** (separate Go modules; operator opts in):

- `safety/classifier/perspective` — Google Perspective API (text).
- `safety/classifier/aws_rekognition` — AWS Rekognition (image).
- `safety/classifier/hive` — Hive Moderation (image + text).

**Operator config:**

```
IMAGE_CLASSIFIERS=nsfwjs,phash         # comma = parallel; OR semantics on results
TEXT_CLASSIFIERS=detoxify
NSFW_THRESHOLD=0.7
CSAM_HASH_LIST_PATH=/etc/portal/csam-hashes.txt
SAFETY_REVIEW_WEBHOOK=                 # optional alerting URL on CSAM match
```

**Workflow:**

1. `media:asset_ready` event consumed by `safety` worker.
2. Run all configured image classifiers in parallel (`OR` semantics across results).
3. **CSAM hash match** → block asset (set `assets.status='quarantined'`) + emit high-priority `safety:csam_detected` Asynq task + insert `safety.csam_incidents` row + page operator via configured webhook. **Quarantine, never delete** — legal evidence preservation.
4. **NSFW score > threshold** → set `assets.nsfw_flag = true`; community NSFW policy enforced on post visibility (§9.31).
5. **Text classifier** runs on post-create; toxicity > 0.85 → flag to mod queue (don't auto-delete; mod decision).
6. All classifier outputs stored on `safety.classifications` for audit + later threshold tuning.

**Critical invariant:** classifier output is **advisory** except for CSAM. NSFW + toxicity flag content for human moderator review; auto-block reserved for CSAM hash matches only.

Lands in Phase 12.

### D-39 — Group calls: LiveKit SFU; P2P for 1:1; signalling through Portal API for RBAC *(resolves §16.G-39)*

P2P mesh fails past ~4 peers (n² connections). Group calls need an SFU. For a Go-monolith self-host product, LiveKit is the clear winner — Go-native, single binary, polished SDKs, Apache 2.0, active commercial backing.

**Decision:**

- **1:1 calls:** WebRTC P2P; no SFU hop. Lower latency, zero server cost. Falls back to TURN through `coturn` if NAT blocks direct connection.
- **Group calls (≥3 peers):** LiveKit SFU.
- **Audio rooms / Spaces (§9.29):** LiveKit audio-only room (supports 100+ listeners).
- **Service:** new `livekit` container in `docker-compose.yml`, gated behind `--profile calls` opt-in flag (similar to `--profile observability` in [D-8]). Self-hosters without calls don't pay the resource cost.
- **Signalling:** through Portal API. Portal mediates LiveKit token issuance — every join request hits Portal first, which:
  1. Verifies RBAC (caller can join this room).
  2. Verifies privacy controls (caller can DM/call the host per §9.21).
  3. Issues a LiveKit token scoped to the room.
  Once tokens are issued, peers connect to LiveKit directly. **No raw LiveKit endpoint exposed publicly.**
- **Recording** (for audio rooms + opt-in for group calls): LiveKit's egress feature writes recording to MinIO; FFmpeg post-process to AAC; emits `media:asset_ready` so the recording surfaces as a normal asset.
- **STUN/TURN:** LiveKit ships with a built-in TURN server; for production behind NAT, configure a dedicated `coturn` service. Documented in `docs/operations/calls.md`.

**New env vars:**

```
LIVEKIT_API_KEY=
LIVEKIT_API_SECRET=
LIVEKIT_URL=ws://livekit:7880
LIVEKIT_RECORDING_BUCKET=portal-recordings
TURN_SERVER=                             # optional external coturn
```

Lands in Phase 12.

### D-40 — Creator payouts: pluggable `Provider` interface; `manual` v1 default; Stripe Connect first real integration *(resolves §16.G-40)*

Different operators have radically different needs (US-only, EU-only, non-profit, crypto-curious). Vendor lock-in is unavoidable for compliant fiat payouts, but the plug-in pattern means each operator can choose their own provider without forking the codebase.

**Decision — `Provider` interface in `bank/payout/`:**

```go
package bankpayout

type Provider interface {
  EnrollCreator(ctx context.Context, user UserID, kyc KYCData) (CreatorAccountID, error)
  Payout(ctx context.Context, from CreatorAccountID, amount Money) (PayoutID, error)
  Status(ctx context.Context, id PayoutID) (PayoutStatus, error)
  TaxFormsFor(ctx context.Context, creator CreatorAccountID, year int) ([]TaxForm, error)
}
```

**Ships in priority order:**

1. **`bank/payout/manual`** (v1 default; ships Phase 11) — operator manually wires payouts and marks them complete in the bank module's admin UI. KYC + tax forms are operator's problem. Always works; zero vendor dependency.
2. **`bank/payout/stripe`** — Stripe Connect Express accounts. Handles KYC + 1099-K filing for US creators. Fees ~3% + flat. Ships when the first operator needs it.
3. **`bank/payout/wise`** — cross-border cheap; no platform-payment primitive so reconciliation is manual. Per-operator request.
4. **`bank/payout/usdc`** — USDC on Stellar or Polygon for low fees. Compliance unclear; opt-in only; documented "experimental" in self-host docs.

**Workflow:**

1. Creator opens "Withdraw" UI → enters amount + destination provider.
2. **Step-up auth** ([D-27]) — `RequireACR("acr:portal:recent_mfa")` because payouts are irreversible.
3. If creator hasn't enrolled with the configured provider, run `EnrollCreator` (provider-specific KYC flow, usually a hosted redirect).
4. Submit `Payout` call. Bank module records balanced ledger entry: `creator.balance −X, payout.outstanding +X` [D-15].
5. Asynq cron job polls `Status` until completed or failed. On success: `payout.outstanding −X, payout.completed +X`. On failure: reverse to creator balance.
6. Tax forms surfaced annually in creator settings via `TaxFormsFor`.

**Operator config:**

```
PAYOUT_PROVIDER=manual                # manual | stripe | wise | usdc
STRIPE_CONNECT_CLIENT_ID=
STRIPE_CONNECT_SECRET=
PAYOUT_MIN_THRESHOLD=2500             # cents — don't pay out micro-amounts
PAYOUT_HOLD_DAYS=7                    # delay after balance change to allow chargebacks/refunds
```

**Constraint:** `manual` ships in Phase 11 with the creator economy. Stripe Connect lands per first paying operator. The interface guarantees swap is mechanical — no schema migration when adding a new provider.

Lands in Phase 11.

---

## How to read this document

- The status legend (✓ / ○ / △) on every section reflects the **code reality**, not aspiration.
- The roadmap is **sequential** — each phase's exit criterion guards the next.
- Open questions are **gates** — the marked items should be answered before the phase that depends on them opens; if not, the phase ships on assumptions that will need rework.
- **Stable identifiers vs section numbers** — the open-question IDs (`16.A-1`, `16.B-8`, …, `16.G-40`) and the decision IDs (`D-1` … `D-34`) are **stable strings**, not references to a current section number. The top-level sections holding them (§19 Open questions, §20 Decisions log) may renumber as new sections are inserted, but the IDs never change. Always cite by ID, not section number.
- Resolved questions get struck through with a `→ Resolved [D-N]` pointer. Never renumbered.
- Decisions are also stable; overturning one appends a revision (`D-N.r1`), never edits in place. The audit trail matters when the rationale stops applying.
