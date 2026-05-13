# Portal — Danh sách tính năng

Bắt nguồn từ [CLAUDE.md](../../CLAUDE.md) (kiến trúc + chia module) và [template-main/social/](../../template-main/social/) (tham chiếu UX/UI cho lớp social). Mỗi tính năng map vào backend module sở hữu nó ([backend/MODULES.md](../../backend/MODULES.md) luật áp dụng: truy cập cross-module qua `api/` only).

Status legend: **✓ scaffolded** = module + một số code đã có, **○ planned** = directory/spec tồn tại nhưng trống, **△ inferred** = derive từ template, chưa phải module.

---

## 1. Identity, Auth & Access — module `account` ✓

Nguồn: [backend/internal/modules/account/](../../backend/internal/modules/account/), CLAUDE.md §"Account module".

- **OIDC sign-in qua Authentik** — `/auth/login` → `/auth/callback`; CSRF + nonce bind trong cookie `portal_oidc` 5-phút. Không có local password.
- **Session hai-token** — access JWT HS256 5-phút (`kid` xoay) + refresh token 256-bit 30 ngày (SHA-256 khi lưu).
- **Mode Cookie + Bearer** — cookie `portal_access` / `portal_refresh` HttpOnly Secure cho browser, `Authorization: Bearer` cho API client.
- **Logout** — single-session `/auth/logout` + global `/auth/logout-all` (bump `users.token_version`).
- **Phát hiện refresh-token reuse** — trình một token đã rotate revoke toàn bộ chain và emit `auth.refresh.reuse_detected`.
- **`/auth/me`** — trả snapshot user hiện tại.
- **RBAC engine** — grammar permission `<resource>:<action>[:<scope>]`, wildcards, parser fail-closed, role hierarchy (guest → user → creator → editor → moderator → admin → superadmin) với walk effective-permission qua recursive CTE.
- **OIDC group → role sync** — env `OIDC_GROUP_ROLE_MAP` map group Authentik → role global Portal; reconcile vào `user_oidc_roles` mỗi callback. Effective permission là union với `user_roles` Portal-managed. Grant tenant-scoped chỉ Portal. Bootstrap qua `BOOTSTRAP_ADMIN_OIDC_SUBJECTS`. [D-26]
- **Step-up auth** — middleware `account.RequireACR("acr:portal:recent_mfa")` trên route nhạy cảm; 403 + Problem `step_up_required` trigger round trip re-auth với `acr_values=mfa prompt=login`. Window mặc định 5 phút. [D-27]
- **MFA enforcement** — hoàn toàn Authentik-managed (không có 2FA secret trong Portal). Lúc login, nếu user có permission `bank:*` nào và `amr` thiếu `mfa`, trả `mfa_enrollment_required` với deep-link sang Authentik MFA dashboard. [D-28]
- **Permission cache** — Redis-backed, namespace theo `token_version` nên revocation = cache bust trong một bump.
- **UI Account-settings** (△ từ template): `Account Settings`, `Change Password` (cho fallback non-OIDC nếu thêm), `Personal Information`, `Education & Employment`, `Hobbies & Interests`, preference `Notifications`.
- **Audit log** — write best-effort qua `audit.Logger`; không bao giờ block request.

---

## 2. Multi-tenancy — module `tenant` ○

Nguồn: [backend/internal/modules/tenant/](../../backend/internal/modules/tenant/) (skeleton), migration `0010_rls_enable` trong [backend/MODULES.md](../../backend/MODULES.md).

- **Organizations** — entity tenant top-level.
- **Memberships** — assignment user ↔ org, role scoped.
- **Bootstrap RLS** — per-request `BeginTenantScope` set GUC session để Postgres RLS filter mọi table tenant-scoped.
- **Đường batch cross-tenant** — `cmd/sysjobs` dùng `internal/sysrepository` (BYPASSRLS); depguard chặn ai khác import nó.

---

## 3. Media Pipeline — module `media` ✓

Nguồn: [backend/internal/modules/media/](../../backend/internal/modules/media/) (có `worker/transcode.go`, `worker/thumbnail.go`, `query/assets.sql`).

- **Upload asset** — multipart → MinIO origin bucket, key tenant-prefix.
- **Worker transcode** (Asynq queue `transcode`, priority 5) — HLS ladder qua FFmpeg.
- **Worker thumbnail** (queue `thumbnail`, priority 3) — generate poster + sprite.
- **State machine asset** — `pending → processing → ready | failed`; transition emit `media:asset_ready` cho module downstream.
- **Signed URL** — `mediaapi.SignedURL(assetID, ttl)` cho playback time-limited.
- **CDN edge** — Cloudflare R2 trước MinIO origin; hook invalidate cache khi replace asset.
- **HLS playback** — frontend dùng Vidstack.

---

## 4. Movies — module `movie` ○

Nguồn: [backend/internal/modules/movie/](../../backend/internal/modules/movie/) (skeleton); CLAUDE.md nhắc đến route group `(movies)` trong Next.js.

- **Catalog CRUD** — title, synopsis, cast, genre, year, rating.
- **Episodes / seasons** — cho series.
- **Asset binding** — phụ thuộc `mediaapi` cho HLS asset playable; subscribe `media:asset_ready` để flip `status=ready`.
- **Browse / search / filter** theo genre, year, rating.
- **Watch progress** — timestamp resume per-user.
- **Continue watching** rail.
- **Ratings & reviews** (△ có thể overlap với comment social).

---

## 5. Music — module `music` ○

Nguồn: [backend/internal/modules/music/](../../backend/internal/modules/music/) (skeleton); page template `Music And Playlists.html`.

- **Tracks** — metadata, artist, album, duration, asset binding qua `mediaapi`.
- **Albums**, **artists**.
- **Playlists** — user-curated, public/private, collaborative (△).
- **Queue & playback state** — phía frontend, persist per-user.
- **Widget "Now playing"**.

---

## 6. Stories — module `story` ○

Nguồn: [backend/internal/modules/story/](../../backend/internal/modules/story/) (skeleton); CLAUDE.md nhắc route group `(stories)`.

- **Story** với **chapters** (có thứ tự).
- **Reader** (paginated hoặc long-scroll).
- **Reading progress** per-user.
- **Bookmarks**, **drafts**, **publish workflow** (yêu cầu role `creator`+).

---

## 7. Comics — module `comic` ○

Nguồn: [backend/internal/modules/comic/](../../backend/internal/modules/comic/) (skeleton).

- **Comic** với **chapters** và **pages** (asset image qua `mediaapi`).
- **Reader** — mode single-page, double-page, vertical-scroll.
- **Progress tracking**, **bookmarks**.
- **Workflow publish** mirror stories.

---

## 8. Personal Finance / Bank — module `bank` △ (planned — chưa phải module)

Track **mọi dạng tiền user có** ở một nơi: account, transaction, debt (nợ), loan (cho vay), investment, savings, budget. Module mới; chưa scaffold. Theo layout chuẩn trong [backend/MODULES.md](../../backend/MODULES.md) §3.

### 8.1 Accounts
- **Loại account**: cash, checking, savings, credit card, loan account, investment account, retirement, crypto wallet, gift card, "other".
- **Currency per account**; báo cáo multi-currency qua FX rate hàng ngày.
- **Opening balance**, state **active / archived**.
- **Metadata định chế** (tên ngân hàng, số account masked khi lưu).

### 8.2 Transactions
- **Primitives debit / credit / transfer** — mỗi entry có account source hoặc destination (transfer = một source + một destination, không cần category).
- **Splits** — một transaction across nhiều category (vd hoá đơn siêu thị = groceries + household + alcohol).
- **Categories** — phân cấp (Income → Salary; Expense → Food → Groceries…); seed default, user mở rộng được.
- **Tags** cho label cross-cutting (`vacation-2026`, `tax-deductible`).
- **Recurring transaction** — rent, salary, subscription; schedule cron-style, generate như draft user confirm.
- **Notes & attachments** (receipt) — qua `mediaapi`.

### 8.3 Debts — tiền user nợ
- **Counterparty** (person hoặc institution).
- **Principal**, **interest rate**, **schedule** (one-shot hoặc amortising).
- **Repayment plan** — installment generate; tracking on-time / late.
- **Outstanding balance** derive từ payment, không lưu mutable.
- **Status**: active, paid, defaulted.

### 8.4 Loans — tiền user cho vay
- Mirror của Debts; cùng field, hướng cash-flow ngược lại.
- **Reminder due-date** qua Asynq `notify:loan_due`.

### 8.5 Investments
- **Holdings** — position của securities / crypto trong investment account.
- **Cost basis** vs **current market value**; gain / loss chưa thực hiện.
- **Lots / FIFO** cho cost basis chính xác khi bán một phần.
- **Price feed** — nhập manual trước, provider pluggable sau.
- **Dividends / interest** book như income transaction tied vào holding.

### 8.6 Savings goals
- **Goal** = target amount + target date + account(s) linked.
- **Progress** tính từ contribution.
- **Quy tắc auto-contribute** — "round up mỗi transaction", "X% mỗi salary".

### 8.7 Budgets
- **Cap per-category** cho period (week / month / custom).
- **Roll-over** option cho budget chưa dùng.
- **Alert threshold** (50% / 80% / 100%) — Asynq `notify:budget_alert`.

### 8.8 Net worth & reports
- **Net worth = assets − liabilities**, time-series (snapshot hàng ngày trong table denormalised cho đọc nhanh).
- **Báo cáo cash-flow** — income vs expense theo category, theo period.
- **Savings rate**, **debt-to-income**, **investment performance**.
- **Forecast** — chiếu trajectory balance từ recurring rule + goal active.

### 8.9 Multi-currency
- **Currency per-account** + preference **reporting currency** trên user.
- **FX rate** snapshot hàng ngày; báo cáo historical value account ngoại tệ theo FX as-of-date.

### 8.10 Import / Export
- **Import CSV** với mapping column + dedupe (hash trên date+amount+counterparty).
- **OFX / QFX** sau.
- **Bank API integration** (Plaid-style) defer sang v2.
- **Export** sang CSV / JSON cho backup user-owned.

### 8.11 Permission & sharing
- Default: mỗi user sở hữu data bank riêng — scope RBAC `bank:*:own`.
- **Household sharing** — mời user khác xem/edit account chia sẻ; dùng `tenant` cho household + RBAC `bank:*:any` trong đó.

### 8.12 Privacy
- Data bank là nhạy cảm nhất trong hệ thống. **Encrypt số account và tên counterparty khi lưu** qua envelope encryption với key platform-managed.
- **Audit mọi read** bởi người không phải owner; route qua `audit.Logger` chung.

### Sở hữu schema
Tables dưới `bank.*`: `accounts`, `transactions`, `transaction_splits`, `categories`, `tags`, `transaction_tags`, `recurring_rules`, `debts`, `loans`, `repayments`, `holdings`, `holding_lots`, `price_history`, `fx_rates`, `goals`, `budgets`, `budget_periods`, `networth_snapshots`. Tất cả RLS-scope trên `user_id` (hoặc `household_id` khi sharing land). Module sở hữu: `bank` — không module khác join được.

### Event async
- **Emit**: `bank:transaction_created`, `bank:debt_overdue`, `bank:budget_threshold_crossed`, `bank:goal_reached`.
- **Subscribe**: chưa có ban đầu; có thể consume `media:asset_ready` khi flow receipt-attachment được wire.

### Sequencing migration
Bank tables land trong block migration riêng, vd `00NN_bank_init.up.sql` rồi `00NN+1_bank_investments.up.sql` v.v. Enable RLS fold vào migration `*_rls_enable` hiện có.

---

## 9. Social Layer △ (planned — chưa phải module)

Nguồn: [template-main/social/](../../template-main/social/) page inventory. Có thể thành module `social/` (hoặc chia thành `social`, `messaging`, `community`).

### 9.1 Newsfeed
- Feed reverse-chronological + algorithmic (`Newsfeed.html`, `Newsfeed - Masonry.html`).
- **Post composer** — text, image, video, link, poll (post version: `Post Versions.html`).
- **Reactions, comments, shares**.
- **Toggle layout** Masonry vs list.

### 9.2 Profile
- **Public profile** (`Profile Page.html`, `ProfilePage-LoggedOut.html`).
- Tab: **About**, **Friends**, **Photos**, **Videos** (`Profile Page - About/Friends/Photos/Videos.html`).
- **Cover & avatar**, custom widget (`Manage Widgets.html`).

### 9.3 Friend graph
- **Friend requests** (`Your Account - Friends Requests.html`).
- **Friend groups** (`Friend Groups.html`) — close friends, work, family, v.v.
- **Block / mute**.

### 9.4 Communities / "Favourite Pages"
- **Page Feed** (`Favorit Page Feed.html`), **About** (`Favorit Page - About.html`), **Events** (`Favorit Page - Events.html`), **Tabs** (`Favourite Page With Tabs.html`).
- **Page settings & popup create-page** (`Fav Page - Settings And Create Popup.html`).
- **Role trong page** (admin / mod / member) — slot vào RBAC engine hiện có với resource page-scoped.

### 9.5 Events
- **Calendar view**, popup **create event** với scope **private / public** (`Calendar and Events - Create Event POPUP (Private_Public).html`).
- **RSVP** (going / interested / declined).
- Reminder → Asynq `notify:event_reminder`.

### 9.6 Messaging
- **Direct chat** (`Your Account - Chat Messages.html`).
- Thread 1:1 và group, typing/read indicator, attachment qua `mediaapi`.

### 9.7 Notifications
- Feed in-app + email + push (`Your Account - Notifications.html`).
- Preference per category notification.
- Asynq-driven: mọi module emit notification task; module notification duy nhất fan out.

### 9.8 Search
- **Search thống nhất** across people, posts, movies, music, stories, comics, events, pages (`Social Search Results.html`).

### 9.9 Community badges & gamification
- **Badges** (`Community Badges.html`) — earn cho đóng góp, streak, v.v.

### 9.10 Statistics dashboard
- View engagement per-user (`Statistics.html`).

### 9.11 Widgets
- **Weather widget** (`Weather Widget.html`), **sticky sidebar**, customisable per profile (`Sticky Sidebars.html`, `Manage Widgets.html`).

### 9.12 Follow graph bất đối xứng (song song với friendship)

Model social của template là friendship đối xứng kiểu Facebook (§9.3), nhưng module content của project (movie/music/story/comic) đều cần model **creator → followers**. Cả hai coexist.

- **Following / followers** — một chiều, không có bước accept (trừ khi profile target là private).
- **Following feed** riêng với feed algorithmic — chỉ post từ người mình follow.
- Setting **visibility follower count** (public / private).
- **Suggested users to follow** — algorithm + curation admin.
- **Notification** khi có follower mới (rate-limit nếu popular).
- Riêng biệt với friendship §9.3: có thể follow mà không friend; có thể friend mà không follow (configurable per user).

### 9.13 Feed ranking & discovery

- **Sort mode** per feed: Hot / New / Top / Controversial (Reddit-style), opt-in per-community.
- **Feed algorithmic "For You"** (Twitter-style; weight = recency × follow-graph × engagement × diversity).
- **Trending topics / hashtags** surface, scope per locale + per community.
- **Explore / Discover** — recommendation algorithmic across content type (movies + music + stories + posts).
- Popover **"Why am I seeing this?"** transparency trên mọi surface algorithmic (hedge regulatory cho EU DSA compliance).

### 9.14 Stories (ephemeral 24h)

- Story photo / video; auto-expire sau 24h (background job xoá media sau 7 ngày để giảm cost).
- **Story replies** route sang DM (§9.6) như quote-context.
- **Highlights** — curate story đã save vĩnh viễn trên profile (escape-hatch từ expiry).
- Privacy: public / followers / **close-friends list**.
- Stories dùng media pipeline (consume `media:asset_ready`; thumbnail generate như poster).

### 9.15 Threads, replies & quote-shares

- **Quote-share** (kèm commentary) vs re-share thường (không commentary). `post_kind` khác; render feed khác.
- **Reply chain** tạo thành **thread** trên profile của poster gốc (Twitter thread).
- **Nested comment threading** — max depth configurable per community (default 8); UI collapse nhánh dài.
- **Conversation view** — render thread graph như tree với permalink per node.

### 9.16 Tagging & mentions

- `@mention` user trong post, comment, story, DM.
- Tag user trong **photo** với bounding-box (clickable hot-spot).
- User đã tag có thể untag mình; có thể yêu cầu approval trước khi tag visible (privacy setting).
- Mention emit task Asynq `notify:social.mention` [D-1].

### 9.17 Hashtags

- `#hashtags` free-text parse từ body post.
- **Landing page hashtag**: feed aggregate mọi public post dùng tag đó.
- **Follow một hashtag** — post dưới tag follow surface trong feed kể cả khi không follow author.
- Widget **trending hashtag** (windowed: 1h / 24h / 7d gần nhất).
- Convention prefix tag per-community được phép (`#tech:rust`).

### 9.18 Bookmarks / save-for-later

- Save post (và comment) cho sau. **Hoàn toàn private** — không ai khác thấy save của bạn.
- **Collections** tuỳ chọn (folder): "Read later", "Recipes", "Tax docs".
- Move/copy giữa collection; export collection sang CSV/JSON (tie vào pattern export kiểu bank).

### 9.19 Reactions (rich, vượt binary)

- Sáu reaction default theo norm Facebook: 👍 like, ❤️ love, 😂 haha, 😮 wow, 😢 sad, 😡 angry. Admin per-community có thể thêm tối đa 4 emoji custom.
- Configurable per post-type (post tưởng niệm có thể disable laughter).
- Count reaction visible cho author và viewer; list reactor visible cho author + chính reactor.

### 9.20 Voting kiểu Reddit (opt-in per-community)

- **Upvote / downvote** trên post và comment.
- Score = upvote − downvote; ranking Wilson-confidence-interval cho sort "Hot".
- Aggregation **karma score** (xem §9.32).
- Community chọn **reactions XOR voting** (không đồng thời, mặc định — cả hai làm user rối).

### 9.21 Controls privacy & visibility

- **Visibility per-post**: public / followers / friends / close-friends list / custom list / only-me (archive).
- **Visibility profile**: public / followers-only / private (request-to-follow gate post).
- **DM gate**: anyone / followers / friends / nobody.
- **Block** (cut mutual; cả hai biết) vs **Mute** (ẩn silent; người mute không biết).
- **Mute keywords / topics** trong feed.
- **Ẩn online-status** indicator.
- **Privacy presets** ("Public profile", "Friends only", "Locked down") cho onboarding nhanh.
- Setting privacy có audit event `https://portal/errors/privacy.changed` [D-25] — flag bảo mật nếu thay đổi bulk + bất thường.

### 9.22 Lists & custom feeds

- List user-curated ("Family", "News I read", "Tech twitter"), public optionally.
- Collection **multireddit-style** community.
- **Filtered feed** — chỉ video, chỉ link, chỉ text-post, chỉ original (không re-share).
- Subscribe list public của người khác.

### 9.23 Photo galleries / carousels / albums

- **Post multi-image (carousel)** — tối đa 20 image, navigation swipe.
- **Albums** trên profile ("Trip 2026", "Wedding").
- **Caption per-image**; hỗ trợ reorder.
- **Alt-text per image** (accessibility + i18n searchability).

### 9.24 Reels / video short-form

- Feed video short-form vertical (max 60s, hard cap 90s).
- Discovery algorithmic qua search/ranking infra shared ([D-2]).
- **Music / sound overlay** — audio track reusable; audio reel của user nào trở thành "sound" cho reel derivative (credit cho original).
- **Duets / stitches** — video response đặt side-by-side hoặc như continuation.
- Effects/filter browser-side (filter camera, AR sticker) — lift frontend lớn; scope Phase 10+.

### 9.25 Live streaming

- Go-live → media pipeline chạy **HLS low-latency** (LL-HLS).
- Chat viewer qua WebSocket `platform/realtime/` [D-3].
- **Replay** — HLS-VOD save tự động; streamer có thể xoá.
- Overlay **live reactions** — thumbs/heart bay trên player.
- Concurrent viewer count.
- Asynq event mới: `media:live_started`, `media:live_ended`.
- Capacity transcode ([D-13]) mở rộng với setting LL-HLS; env mới `LIVE_LATENCY_SECONDS=4`.
- **Moderation gate** — chat live enforce auto-mod rule real-time; mod có thể timeout / kick.

### 9.26 Drafts, scheduling, edit history

- State **Draft** — post private với author, không ai khác thấy.
- **Schedule publish tương lai** — task Asynq scheduled fire ở `publish_at` TZ-aware [D-17].
- **Edit window** — 15 phút edit silent; sau đó edit hiện marker "edited" + history revision đầy đủ.
- View **Edit history** per post (mỗi revision diff).
- Draft + scheduled post sống trên surface private "Studio" trên profile.

### 9.27 Pinned content

- **Pin một post lên top profile** (hoặc tới 3 với badge verified-creator).
- **Pin một comment per post** (đặc quyền author; mod cũng có thể pin).
- **Post mod-pinned lên top community** (tối đa 2).
- Pinned content mang timestamp `pinned_at` + audit event.

### 9.28 Long-form articles

- Composer rich-text (block-based: heading, paragraph, code, embed, image, callout, table of contents).
- Auto TOC + ước tính reading-time.
- `post_kind = 'article'` khác — render feed khác (cover image + excerpt + read-time).
- Article có thể draft, schedule, edit (§9.26).
- Article cũng nằm trong search index ([D-2]).

### 9.29 Audio rooms (Spaces / Clubhouse-style)

- Audio room live với role: **host**, **co-host**, **speaker**, **listener**.
- **Hand-raise** → request to speak; host approve.
- Recording → replay (dùng pipeline audio media; AAC).
- Scope Phase 10+; tính năng opt-in.

### 9.30 Công cụ moderation

- Workflow **report content** (category: spam, harassment, NSFW-trong-context-SFW, illegal, copyright, other). Queue report per community.
- **Mod queue** per community — sort được theo report count / severity / freshness.
- Action mod: **remove** (visible với author kèm lý do), **lock** thread, **hide**, **pin**, **ban** member, **mute** member, **warn**, **shadow-ban** (content visible chỉ với author).
- **Quy tắc auto-mod** per community:
  - block-list keyword
  - block-list link-domain
  - throttle post account mới (không post 24h đầu)
  - delay comment low-karma
  - filter duplicate-content
- **Audit log per-community** qua `platform/audit/` [D-25]: mỗi action mod được ghi.
- Workflow **Appeal** — user bị ban/restrict submit appeal; mod community review; appeal bị reject có thể escalate sang admin platform.
- **Trust & safety dashboard** cấp platform cho role `superadmin`.

### 9.31 Cảnh báo content & tags

- Tag **NSFW** (policy per-community: allowed / forbidden / opt-in).
- Tag **Spoiler** (per-post; reader thấy blur cho đến click-through).
- **Trigger warning** (list category configurable: violence, self-harm, eating disorder, v.v.).
- Preference reader: auto-blur NSFW, ẩn spoiler-tagged, click-to-reveal.

### 9.32 Karma / reputation

- Score per-user từ vote nhận được.
- Hai loại: **post karma** + **comment karma**.
- Aggregate **per-community** + **global**.
- Community có thể disable voting → không earn karma trong community đó.
- **"Cake day"** — surface account anniversary (UI flag trên profile ngày đó).
- Karma là **signal feed-ranking mild**, không bao giờ là hard gate.

### 9.33 Community wiki

- Page wiki per-community (Reddit-style).
- **Mod-editable** mặc định; có thể opt member in (grant permission per-page).
- **Versioned history** — mỗi revision diff được; rollback hỗ trợ.
- Search wiki per-community.

### 9.34 Memories / on-this-day

- Surface **"On this day"** cho post từ N năm trước (1, 5, 10).
- Post **account anniversary** mỗi năm ("Bạn join Portal N năm trước"); shareable.
- **Birthday reminder** cho friend (opt-in; off mặc định cho privacy).

### 9.35 AMAs / scheduled Q&A

- Thông báo session Q&A scheduled; user có thể subscribe để nhận thông báo.
- **Window submit question** trước session.
- **Question voted-up** float lên top.
- Host mark **"answered"**; answer thread dưới question.
- Replay-friendly — Q+A đầy đủ persist sau session.

### 9.36 Verification & trust identity

- **Badge verified** — assign bởi admin Portal qua permission RBAC `account:verify`.
- **Badge notable-creator** per module (verified-movie-creator, top-musician, v.v.).
- Card **"About this account"** trên profile: ngày join, location (nếu chia sẻ), link, lý do verification, total content count.
- Marker **Restricted / suspended** account (visible cho viewer, có lý do nếu public).
- Verification **không trả phí** trong v1 (không có equivalent Twitter Blue chưa).

### 9.37 Messaging extensions (mở rộng §9.6)

- **Reaction message** (subset của emoji §9.19).
- **Reply-quote** một message cụ thể trong thread.
- **Voice notes** — message audio record qua media pipeline.
- **Disappearing messages** — auto-delete sau N giây-read (config per thread).
- **Voice / video calls** — WebRTC; lift đáng kể; scope Phase 10+. Dùng `platform/realtime/` cho signalling [D-3] nhưng media plane peer-to-peer hoặc qua SFU (infra mới).
- **Message search** trong thread (Postgres FTS [D-2]).
- **Forward** message sang thread khác hoặc pin như post.

---

## 10. Creator economy & monetisation △ (planned — module bridge mới)

Bridge `social` ↔ `bank`. Sống như module mới `internal/modules/creator/` mà api/ public expose surface "tipping" + "subscription". Touch post social + bank ledger ([D-15]) để payout route qua double-entry chuẩn.

- **Tips / awards** — viewer gửi tip $X trên post, reel, hoặc live stream. Tạo ledger entry cân: bank account của viewer debit; account của creator credit (trừ optional platform fee).
- **Creator subscriptions** — payment monthly recurring từ subscriber sang creator; subscriber được badge + post private subscriber-only. Task Asynq scheduled emit event `bank:transaction_created` mỗi billing cycle.
- **Paid posts / paywalls** — author mark post là paid; viewer pay một lần cho permanent access; ledger entry tương tự tip.
- **Creator analytics** — subscriber count, revenue theo period, top-paying fan, churn rate.
- **Payouts** — creator có thể rút balance tích luỹ sang external bank account (Phase 11+; cần KYC + integration payment provider).

Roadmap: Phase 11+, sau khi bank + social stable.

---

## 11. Marketplace / commerce △ (planned — module bridge mới)

Listing kiểu Facebook Marketplace / Reddit r/HardwareSwap. Module `internal/modules/marketplace/` bridge listing social + payment bank optional.

- **Listings** — title, price (type `Money` [D-14]), image qua media, location, category, condition.
- **Categories** + **search** (qua FTS [D-2]).
- **Chat buyer/seller** qua messaging §9.6 (tab inbox riêng).
- **Integration payment optional** — escrow qua ledger module bank; flow release-on-confirmation.
- **Fraud protection** — workflow report listing; badge verified-seller sau N sale thành công.
- **Out-of-scope v1** trừ khi yêu cầu rõ; page merchandise công ty của template phần nào cover surface commerce (xem §13).

Roadmap: Phase 12+, chỉ khi commerce trong scope.

---

## 12. Privacy, data rights & anti-abuse △ (cross-cutting; không phải module riêng)

Sở hữu across **`account`**, **`platform/audit/`**, và module **`safety`** mới. Hầu hết là feature cross-cutting compose module hiện có.

### 12.1 Quyền data user (GDPR / CCPA)

- **Export data** — user yêu cầu ZIP của mọi data của họ (post, comment, bank, profile, message). Task Asynq long-running; email link download khi ready (link expire sau 7 ngày). Land trong UI settings module account.
- **Xoá account** — soft-delete (mark `users.deleted_at`; content còn 30 ngày cho recover), rồi hard-delete sau grace period.
- **Pause account** — temp deactivate (không login được; profile invisible; không trigger notification liên quan ban).
- **Quyền rectification** — user có thể edit data của họ; back bằng pattern edit-history (§9.26).
- **Activity log** — xem login history riêng, action gần nhất, security event; nguồn từ `platform/audit/` [D-25].
- **Connected session** — list mọi refresh token active với device/UA; revoke từng cái (dùng chain rotation hiện có).

### 12.2 Anti-abuse

- **Spam detection** — middleware rate-limit ([D-11] hiện có) + limit velocity post per-account.
- **Auto-mod rule** ở cấp platform (thêm vào per-community §9.30).
- **Image classifier** (NSFW + CSAM) — out of band; integrate với event `media:asset_ready`. Model open-source (vd NSFWJS, perceptual hashing kiểu Apple); content flagged đi vào queue `safety_review`.
- **Text classifier** — toxicity, hate speech. Provider pluggable; open-source (Detoxify) hoặc external (Perspective API). Self-host có thể disable.
- Lifecycle **Shadow-ban** — content visible với author nhưng không ai khác; dùng filter kiểu RLS trên query social.
- **Appeals** — user bị ban submit appeal (§9.30); audit-logged.

### 12.3 Trust & safety dashboard cấp platform

- View role `superadmin`: mod queue mở, appeal pending, content flagged, trend abuse.
- Hỗ trợ bulk-action (ban một wave spam account).
- Integration với observability [D-8] — metric abuse trên Grafana dashboard.

Roadmap: Phase 7 bao gồm moderation community cơ bản §9.30; Phase 11+ thêm ML classifier + workflow appeal + safety dashboard.

---

## 13. Company / Marketing Microsite △

Nguồn: [template-main/social/Olympus Company/](../../template-main/social/Olympus%20Company/) — có thể là route group Next.js riêng, serve từ cùng entry Traefik, có thể tenant-aware.

- **Landing / Home** (`Company Page - Home.html`).
- **About**, **Careers**, **Contacts**, **FAQs**.
- **Help & Support** + page chi tiết topic.
- **Blog** — grid, masonry, list; layout **post** V1 / V2 / V3.
- **Merchandise store** — product grid, masonry, product detail, shopping cart, checkout (out of scope v1 trừ khi cần rõ; flag là optional).
- **Error pages** — 404 / 500.

---

## 14. Platform & Cross-cutting — `internal/platform/` ✓

Nguồn: [backend/internal/platform/](../../backend/internal/platform/).

- **Config loader** (env-based) — `internal/platform/config/`.
- **DB pool** (pgx) + `BeginTenantScope` — `internal/platform/db/` ○.
- **Cache** (Redis/Dragonfly) với helper key tenant-aware — `internal/platform/cache/` ○.
- **Storage** (S3/MinIO + R2) với key tenant-prefix — `internal/platform/storage/` ○.
- **Jobs** (setup client Asynq) — `internal/platform/jobs/` ○.
- **Realtime** (SSE + WebSocket, backplane pub/sub Dragonfly) — `internal/platform/realtime/` ○. [D-3]
- **Mail** (SMTP) — `internal/platform/mail/` ○. [D-4]
- **Observability** (OTel SDK, Prometheus `/metrics`, init Sentry/GlitchTip) — `internal/platform/observability/` ○. [D-8]
- **Audit** (event log cross-cutting, move ra khỏi `account`) — `internal/platform/audit/` ○. [D-25]
- **Middleware** — rate limit ✓ (`ratelimit.go`), request ID, logging, recovery, **resolver tenant URL-prefix** [D-23].
- **Reverse proxy** — Traefik v3 route qua label `docker-compose.yml`.

---

## 15. API Contract — `shared/openapi.yaml`

- OpenAPI là source of truth — mỗi endpoint flow: edit spec → `make openapi` → implement interface generate. **Spec-first non-negotiable** [D-29]; CI drift check fail mọi PR handler-không-spec.
- **URL versioning** `/api/v{N}/`; hiện tại `/api/v1/`. Thay đổi additive free trong major; breaking change cần major mới + sunset 6-tháng RFC 9745 [D-31].
- **Layout file:** một `shared/openapi.yaml` cho tới ~2000 dòng; split per-module qua `$ref` sau đó [D-29].
- **Schema cross-module** (phải land trong Phase 0): `Problem` (RFC 7807), `Money`, `PaginatedResult<T>`, `TenantContext` path param, `ContinuingItem` cho `/api/v1/continue` [D-29].
- Go server stub generated: `backend/internal/handler/api.gen.go` (gitignored).
- TS client types generated: `frontend/src/lib/types.gen.ts` (gitignored).

---

## 16. Frontend — `frontend/` (Next.js 15)

- App Router + RSC. **RSC-first mặc định**; opt vào `'use client'` chỉ khi thật sự cần (event handler, hook, browser API) [D-33].
- Route group: `(movies)`, `(music)`, `(stories)`, thêm `(social)` / `(comics)` sau.
- **Chiến lược render theo surface** [D-33]: catalogue/detail = server component với island interactivity client; player/reader = chủ yếu client; account/bank = server shell + interactivity client; newsfeed = client primary với page đầu server-rendered.
- **Ranh giới state** [D-32]:
  - TanStack Query — mọi server state (không có Zustand store giữ data API-fetched).
  - Zustand — preference UI persistent (theme, sidebar) và state UI ephemeral (modal, toast).
  - React Hook Form — form state.
  - URL query param — filter / pagination chia sẻ được (read bởi TanStack).
- **Auth handoff cho RSC** [D-34]: `frontend/src/lib/api-server.ts` (`import "server-only"`) wrap `fetch`, đọc `cookies()`, inject `Cookie:` lên request đi ra. 401 từ API → `redirect()` sang `/auth/refresh-and-return?return_to=...` gọi `/auth/refresh` server-side rồi redirect lại.
- **Mandate domain same-site** [D-34]: host Next.js + host Portal API PHẢI chia sẻ registrable domain (vd `portal.example.com` + `api.portal.example.com`) để SameSite=Strict hoạt động. Deployment single-domain dùng một Traefik host với routing theo path.
- **Styling**: Tailwind v4.
- **Player**: Vidstack cho HLS.
- **API client**: generated từ OpenAPI; cookie forward tự động bởi wrapper server-only.
- **Convention doc** ở `frontend/CLAUDE.md` (tạo trong Phase 0) ghi rule boundary + ví dụ anti-pattern [D-32, D-33].

---

## 17. Out-of-band / Operational

- **Migrations** — sequence numeric duy nhất trong `backend/db/migrations/`, file prefix theo module sở hữu. **Forward-only ở production** [D-12]; `migrate-down` chỉ dev + CI roundtrip. Dùng expand → migrate-data → contract across hai deploy cho breaking change.
- **sqlc** — block per-module trong `backend/sqlc.yaml`; output sống trong `repository/` của mỗi module. CI fail on drift [D-9].
- **Hot-reload dev** — `make dev` (`air` cho Go, `pnpm dev` cho Next).
- **Tests** — `go test ./... -race -count=1` + `pnpm test`. Single test: `cd backend && go test ./internal/modules/account/rbac -run TestMatches -v`. Coverage target per module trong [D-9].
- **Lint** — `golangci-lint` (gồm depguard enforce module-boundary rule) + `pnpm lint`. Pre-commit hook tuỳ chọn qua `lefthook` [D-9].
- **CI/CD** — GitHub Actions: `.github/workflows/{ci,release}.yml` với lint, test, sqlc/openapi-drift, migration-roundtrip, build, security [D-9].
- **Observability** — opt-in `--profile observability` trong `docker-compose.yml`: Loki + Prometheus + Tempo + Grafana + GlitchTip [D-8].
- **Backups** — `pgbackrest` (Postgres), MinIO → R2 replication, Dragonfly `BGSAVE`; drill restore hàng quý. Target + procedure trong `docs/operations/backups.md` [D-10].
- **Secrets** — `.env` ở dev, Compose/K8s secret (hoặc SOPS optional) ở prod; policy rotation per class secret trong `docs/operations/secrets.md` [D-11].
- **GitNexus** — index code-intelligence (xem section `<!-- gitnexus:start -->` trong `CLAUDE.md`); chạy impact analysis trước khi edit symbol.

---

## 18. Roadmap (phased — chưa commit)

Mỗi phase có **deliverable** rõ ràng và **tiêu chí exit**. Phase tuần tự vì mỗi phase land một lớp phase tiếp theo phụ thuộc; sub-phase trong phase có thể parallelize.

### Phase 0 — Wiring foundation (ngay lập tức)

*Mục tiêu: biến scaffold hiện có thành flow auth chạy end-to-end.*

- **Wire `cmd/api/main.go`** — load `platform/config`, mở pool pgx, construct `account.Module`, mount `MountHTTP(r)` dưới chain `/api/v1` với middleware standard (request-id, CORS, rate-limit, tenant). Thay comment `TODO: mount OpenAPI-generated handlers`.
- **Chạy `make sqlc`** cho block `account`; commit artefact generate `internal/modules/account/repository/*.sql.go` (chúng là gitignored — generate lại locally, không check in).
- **Viết repository adapter** sau interface account đang consume: `AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserUpserter`.
- **Split migration `0001`** — hiện trộn `users` + `assets` (module khác nhau) và có cột text `users.role` orphan overlap với table RBAC. Rewrite thành `0001_platform_init` (extension), `0002_account_users` (chỉ users, +`locale`+`timezone`), `0003_account_rbac` (renumbered), `0005_media_assets`, v.v. [D-18]
- **Thêm `users.locale` (BCP 47, default `'en-US'`) và `users.timezone` (IANA, default `'UTC'`)** như một phần của `0002_account_users`. [D-7]
- **Move `audit/` từ account → `platform/audit/`** — audit là cross-cutting; account trở thành consumer. Đổi tên event `auth.refresh.reuse_detected` → `account.refresh.reuse_detected` cho hợp taxonomy `<module>.<resource>.<action>` mới. [D-25]
- **Định nghĩa registry event-type taxonomy** trong `backend/MODULES.md` §5.3 để chống collision. [D-25]
- **Surface claim `amr`, `acr`, `auth_time`** vào auth context (`account/auth/context.go`) để middleware step-up [D-27] và enforce MFA [D-28] plug in sau mà không rewrite auth middleware.
- **Thêm table `user_oidc_roles`** vào `0003_account_rbac` để OIDC group → role sync [D-26] có chỗ ghi lúc callback đầu.
- **Adopt shape RFC 7807 `Problem`** cho mọi 4xx/5xx trong `shared/openapi.yaml`; URI `type` stable trở thành key i18n. [D-7]
- **Reserve prefix Asynq `notify:*`** trong `backend/MODULES.md` §5.2 để module tương lai không vô tình collide. [D-1]
- **Mở rộng spec OpenAPI** — thêm tag comics + tenant. **Schema cross-module eager** phải land trước Phase 0 close [D-29]: `Problem` (RFC 7807 với extension Portal như `required_acr`/`enrollment_url`), `Money`, `PaginatedResult<T>`, `TenantContext` path param, `ContinuingItem`, component response 4xx/5xx tiêu chuẩn.
- **Lock URL versioning** — mỗi route sống dưới `/api/v1/`; document policy additive-only + thủ tục deprecation RFC 9745 trong `docs/api/versioning.md`. [D-31]
- **API client frontend server-only** — `frontend/src/lib/api-server.ts` wrap `fetch` với forwarding `cookies()`; route `/auth/refresh-and-return` xử lý 401 RSC. [D-34]
- **Doc convention frontend** — `frontend/CLAUDE.md` document boundary state Zustand/TanStack/RHF [D-32] và decision tree render RSC-first [D-33] với ví dụ anti-pattern worked.
- **Land CI workflows** — `.github/workflows/ci.yml` với job lint + test + sqlc-drift + openapi-drift + migration-roundtrip + build + security. Drift detection từ ngày 1. [D-9]

**Exit:** developer có thể `make up && make dev`, sign in qua Authentik, hit `/auth/me`, và `RequireAuth` + `RequirePermission` reject call unauthenticated. CI fail mọi PR cho generated code drift.

### Phase 1 — Tenancy + RLS

- Schema `tenant.organizations` gồm cột **`kind` (`'org' | 'household'`)** từ ngày một để thêm household support Phase 5i không cần migrate table populated. [D-24]
- Schema + query `tenant.memberships`; role granularity khác per kind (org: hierarchy đầy đủ; household: chỉ owner + member, soft cap 6).
- `0010_rls_enable.up.sql` — enable RLS trên mọi table tenant-scoped; `USING (tenant_id = current_setting('app.tenant_id')::uuid)`.
- `platform/db.BeginTenantScope(ctx, tenantID)` set GUC bên trong tx per-request.
- **Middleware tenant-resolution** — extract slug từ `/t/{tenant}/...` URL prefix; resolve sang `tenant_id`; verify membership (hoặc `tenant=me` match caller); set GUC. Synthetic tenant `me` per user cho route personal-data (`/t/me/bank/...`). Deployment single-tenant map `/api/v1/...` thẳng vào tenant default qua Traefik. [D-23]
- Skeleton `cmd/sysjobs` wire với pool BYPASSRLS.
- **Profile observability** — thêm Loki + Prometheus + Tempo + Grafana + GlitchTip vào `docker-compose.yml` sau `--profile observability`; endpoint `/metrics` trên port riêng; OTel SDK auto-instrument chi + pgx + asynq. Performance RLS measurable từ ngày 1. [D-8]

**Exit:** integration test chứng minh row của tenant A invisible với request bound tenant B, trong khi `cmd/sysjobs` thấy cả hai. Grafana show latency request per-route chia theo tenant.

### Phase 2 — Media pipeline end-to-end

- Pick **video** trước (đây là test fidelity cao nhất cho pipeline đầy đủ).
- Endpoint upload → `platform/storage` → MinIO origin → enqueue Asynq `transcode`.
- Worker: FFmpeg HLS ladder (1080p/720p/480p/360p, segment 6s) + poster + sprite; write output sang prefix key sibling. [D-13]
- Encoder selectable qua `TRANSCODE_ENCODER` (`libx264` default; `h264_nvenc` / `h264_vaapi` / `h264_qsv` opt-in). [D-13]
- **Quota transcode per-user + per-tenant + backpressure** — middleware check `asynq inspect` lúc enqueue, reject với 429 + Retry-After khi limit qua. [D-13]
- Emit `media:asset_ready { asset_id, hls_master_url, duration_ms, thumbnail_url }`.
- Transcode fail đi vào `transcode:dead` sau 3 retry; cần action operator. [D-13]
- `mediaapi.GetAsset(ctx, id)` và `mediaapi.SignedURL(ctx, id, ttl)`.

**Exit:** mp4 30-giây round-trip: upload → transcode → HLS playable trong frontend với Vidstack. User thứ hai không thể starve queue.

### Phase 3 — Vertical domain đầu: Movies

- Schema + query `movies`, `seasons`, `episodes`.
- Movie subscribe `media:asset_ready` và flip `movies.status = ready`.
- Endpoint catalog: list với pagination + filter (genre, year, rating), detail, upsert watch-progress.
- Route group frontend `(movies)`: list, detail, player wire với upsert `progress`.

**Exit:** happy path end-to-end qua frontend — thêm movie, transcode, browse, play, resume.

### Phase 4 — Vertical lặp lại: Music, Stories, Comics

- Mỗi cái theo template Phase 3.
- **Table progress per-domain** — `movie.watch_progress`, `music.listen_progress`, `story.read_progress`, `comic.read_progress` với column layout giống nhau. [D-20]
- **Table ratings per-domain** — `<module>.ratings(user_id, content_id, rating, review, ...)`. [D-21]
- **Aggregator `GET /api/v1/continue`** trong `cmd/api` fan-out sang `<module>api.Continue(ctx, userID, limit)` của mỗi module, merge, trả sort theo `updated_at DESC`. [D-20]
- Music thêm playlist.
- Story/comic thêm thứ tự chapter + draft (gate role `creator`).

**Exit:** mỗi domain có loop browse → consume → resume làm việc trong frontend, kèm rail "continue" thống nhất trên trang home.

### Phase 5 — Bank (Personal Finance)

Đáng kể; ship trong sub-phase mỗi cái deliver giá trị visible user. Encryption-at-rest và audit là không thương lượng từ sub-phase 5a.

**Prerequisites Phase 5** (gate Phase 5a):

- **Middleware step-up `RequireACR`** wire vào module `account`. Implementation đọc claim `acr` + `auth_time`; pattern annotation route nhạy cảm in place; frontend recognise Problem `auth.step_up_required` và chạy round trip re-auth. [D-27]
- **Login gate MFA-enforcement** — ở callback OIDC, nếu user có permission `bank:*` nào và `amr` thiếu `mfa`, refuse session với Problem `auth.mfa_enrollment_required` (mang URL enrollment Authentik). [D-28]
- **Authentik configured** với stage TOTP + WebAuthn và policy ACR elevate tới `mfa` on demand. Document trong `docs/operations/authentik.md`.

- **5a — Core ledger** — `bank.currencies` (seed ISO 4217 + cryptos), `accounts` (với `type` ∈ `ASSET|LIABILITY|INCOME|EXPENSE|EQUITY`) [D-15], `categories` (auto-tạo account income/expense, phân cấp), `transactions`, `ledger_entries` với `CHECK SUM(amount)=0` per-tx-per-currency [D-15]. Cột money là `numeric(20,8)`; Go dùng `shopspring/decimal` wrap trong type value `Money` currency-safe [D-14]. Op huỷ diệt (`accounts.delete`, `transactions.delete`) gate bởi `RequireACR("acr:portal:recent_mfa")` [D-27].
- **5b — Multi-currency** — `fx_rates` snapshot hàng ngày; reporting currency trên `users`. Arithmetic cross-currency tường minh qua entry FX conversion trên transaction.
- **5c — Debts** — `debts`, `repayments`, `bank.counterparties` (per-owner, FK `user_id` optional cho link portal-user) [D-16].
- **5d — Loans** — mirror của debts. Confirmation hai-chiều cho settlement khi counterparty là portal user [D-16].
- **5e — Investments** — `holdings`, `holding_lots` (FIFO), `price_history`; price feed manual trước. Buy/sell tự nhiên produce ledger entry cân [D-15].
- **5f — Budgets + goals** — `budgets`, `budget_periods`, `goals`; alert threshold emit task Asynq.
- **5g — Net-worth + reports** — `networth_snapshots`; **scheduler per-TZ hourly** iterate user mà local 00:05 vừa pass và enqueue `bank:snapshot_daily` per user [D-17]. Endpoint cash-flow, savings rate, debt-to-income, investment performance; mọi date range tính trong `users.timezone` [D-17].
- **5h — Import/export** — CSV import với mapper column + dedupe; CSV/JSON export.
- **5i — Household sharing** — tạo tenant `tenant.kind = 'household'`; assign cả hai user làm `owner`; predicate RLS module bank không đổi [D-24]. `bank:*:any` trong household.

**Exit per sub-phase:** op có thể làm qua frontend với entry audit log present.

### Phase 6 — Notifications

Module `notification` standalone — quyết định settle trong [D-1]. Quyết định channel settle trong [D-3], [D-4], [D-5].

- Module `notification` sở hữu `notifications`, `notification_preferences`, `delivery_attempts`, `push_subscriptions`.
- Fan-out Asynq: mỗi emitter publish task `notify:*`; worker module dispatch per-channel.
- **Channels:**
  - **Feed in-app** — row DB + update live qua SSE endpoint `platform/realtime/` `GET /api/v1/events/stream`. [D-3]
  - **Email** — SMTP `platform/mail/` (`wneessen/go-mail`); template dưới `backend/templates/email/<category>/`. [D-4]
  - **Web Push** — VAPID qua `SherClockHolmes/webpush-go`; subscription trong `notification.push_subscriptions`. Không APNS/FCM trong v1. [D-5]
- Preference user per category × per channel.
- Re-emit event lịch sử vào module mới khi cutover (backfill best-effort từ `audit_log`).

**Exit:** ít nhất một notification per module emit (`bank:budget_threshold_crossed`, `media:asset_ready`, `auth.refresh.reuse_detected`, `loan_due`) deliver end-to-end qua mỗi channel đã enable.

### Phase 7 — Social layer (baseline)

Social baseline core. Format nâng cao (stories/reels/live/audio/voting/articles) defer sang [Phase 10](#phase-10--social-advanced-formats--engagement). Theo thứ tự:

1. **Newsfeed** — post (text/image/link/poll), **reaction phong phú** (§9.19), comment, **quote-share** (§9.15), nested threading.
2. **Profile** — `social.profiles` (1:1 với `users`) cho bio/education/employment/hobbies/cover/widget. Field identity-critical ở lại trên `users` [D-19].
3. **Follow graph bất đối xứng** (§9.12) — riêng biệt với friendship §9.3. Following / followers, feed "Following".
4. **Friend graph** — request, group, block/mute (§9.3).
5. **Communities** — page, membership, RBAC page-scoped, **moderation cơ bản** (§9.30 core: report, mod queue, remove/lock/pin/ban).
6. **Events** — calendar, RSVP, reminder.
7. **Messaging** — DM 1:1 + group qua [D-3].
8. **Hashtags + mentions** (§9.16, §9.17) — parse `@user` và `#tag`, landing page, follow-a-tag.
9. **Bookmarks** (§9.18), **pinned content** (§9.27), **drafts + scheduled posts** (§9.26).
10. **Controls privacy** (§9.21) — visibility per-post, DM gate, mute keyword, preset.
11. **Search integration** — post social index qua `social/api.Search` [D-2].

**Exit:** user có thể post, follow user khác, join community, react/comment/quote-share, RSVP, DM, mention qua `@`, dùng `#hashtags`, save bookmark, pin post, schedule draft, và tune privacy setting. Mod có thể chạy community cơ bản.

### Phase 8 — Search & discovery

- Resolve lựa chọn search engine (open question 2) trước khi phase này mở.
- Index builder per-module subscribe event `*` liên quan để giữ index hot.
- Endpoint aggregator `/search?q=...&type=…`.
- Command-palette / search bar global frontend.

**Exit:** typeahead làm việc across people, post, movie, music, story, comic, event, page.

### Phase 9 — Marketing microsite + extras

- Page company, blog (CMS nhẹ).
- Badge / gamification (§9.36 verification + §9.32 karma).
- Optional merchandise store (defer trừ khi yêu cầu rõ).
- 404 / 500 polish.

### Phase 10 — Social: format nâng cao & engagement

Mở rộng "stories + reels + live + long-form". Mỗi cái độc lập đủ để ship:

1. **Stories** (§9.14) — ephemeral 24h; replies → DM; highlight; tier visibility close-friends.
2. **Reels / video short-form** (§9.24) — feed vertical; **audio user-uploaded với chain attribution viral-sound** qua `social.sounds` [D-37]; duets/stitches. Effect browser-side defer trừ khi yêu cầu.
3. **Live streaming** (§9.25) — **RTMP ingest + distribution LL-HLS qua sidecar `mediamtx`** [D-36]; chat live qua `platform/realtime/`; replay là auto-VOD; cap `MAX_CONCURRENT_LIVE_STREAMS_PER_TENANT` per-tenant; env mới `LIVE_LATENCY_SECONDS=4`.
4. **Photo carousels & albums** (§9.23).
5. **Long-form articles** (§9.28) — `post_kind = 'article'` riêng; composer rich-text.
6. **Voting + karma kiểu Reddit** (§9.20, §9.32) — opt-in per community; ảnh hưởng feed ranking như signal mild.
7. **Feed ranking** (§9.13) — Hot/New/Top/Controversial sort mode per-community; **pipeline ba-lớp "For You" hand-tuned** (candidate generation → ranking → diversity) với UI transparency `/settings/feed`; "Following" chronological vẫn là tab default để hedge risk DSA [D-35].
8. **Lists & custom feeds** (§9.22).
9. **Content warnings** (§9.31) — tag NSFW / spoiler / trigger-warning; reader pref.
10. **Messaging extensions** (§9.37) — reaction, reply-quote, voice note, disappearing message. Voice/video call defer Phase 12.
11. **Moderation nâng cao** (§9.30) — auto-mod rule, shadow-ban, appeal workflow.
12. **Community wiki** (§9.33), **AMAs** (§9.35), **memories / on-this-day** (§9.34).
13. **Verification & creator badge** (§9.36).

**Exit:** creator có thể record story, post reel với sound reused (chain attribution visible), go live với chat, viết long-form article. Community có thể chạy voting + karma + auto-mod. User thấy memories "On this day". Feed "For You" có popover transparency và fallback chronological.

### Phase 11 — Creator economy

Bridge module — `internal/modules/creator/` ↔ `bank`.

1. **Tips / awards** (§10) — viewer gửi tip trên post/reel/live; ledger entry cân route platform fee + creator credit [D-15].
2. **Creator subscriptions** — billing monthly recurring qua task Asynq scheduled → ledger entry bank.
3. **Paid posts / paywalls** — payment một lần grant permanent access; check access lúc render.
4. **Creator analytics** dashboard — subscriber count, MRR, churn, top fan.
5. **Payouts** — interface `bank/payout/Provider` pluggable [D-40]; **provider `manual` ship như default v1** (operator chạy payout qua wire transfer, mark bank module); Stripe Connect land theo demand operator. Mỗi payout gate trên `RequireACR("acr:portal:recent_mfa")` [D-27].
6. **MFA bắt buộc** cho account creator-có-monetisation-active [D-28].
7. **Workflow DMCA take-down** cho music reels + paid content [D-37].

**Exit:** creator có thể publish paid post và nhận tip; subscriber bill monthly; balance flow qua ledger module bank; operator có thể complete payout (manual hoặc Stripe).

### Phase 12 — Marketplace + safety + voice call

Ba track độc lập; ship thứ tự bất kỳ.

1. **Marketplace** (§11) — `internal/modules/marketplace/`; listing + chat (qua §9.6) + escrow optional qua bank.
2. **Anti-abuse / ML moderation** (§12.2) — `internal/modules/safety/` với interface `ImageClassifier` + `TextClassifier` pluggable [D-38]; **default NSFWJS + pHash** cho self-host; CSAM hash match block + quarantine + page; flag NSFW advisory cho mod; vendor API (AWS, Hive, Perspective) có sẵn như plug-in.
3. **Export data GDPR + xoá account** (§12.1) — task export Asynq long-running; soft-delete với grace 30 ngày; surface right-to-rectification.
4. **Voice / video calls** (§9.37) — **LiveKit SFU** cho group call; **P2P cho 1:1** [D-39]; service compose `livekit` mới sau `--profile calls`; signalling route qua Portal API cho check RBAC + privacy; `coturn` cho NAT traversal.
5. **Audio rooms / Spaces** (§9.29) — LiveKit audio-only room với role host/co-host/speaker/listener + hand-raise [D-39]; recording qua LiveKit egress → MinIO → AAC.
6. **T&S dashboard cấp platform** cho `superadmin` (§12.3) — mod queue mở, content flagged, trend abuse, bulk action; integrate với metric Grafana [D-8].

**Exit:** user có thể bán item + chat với buyer + payment escrow qua bank; content NSFW auto-tag trước publish; asset CSAM-matched quarantine + operator page; user có thể export và xoá data; call 1:1 hoạt động peer-to-peer và group call qua LiveKit; trust & safety có dashboard riêng.

---

## 19. Open questions — cần phân tích thêm

Quyết định defer. Mỗi cái ảnh hưởng ít nhất một phase sắp tới; nhiều cái nên land trước code. Số là ref stable (đừng renumber khi resolve — strike through và link sang decision doc).

### 16.A — Architecture / cross-cutting ✓ tất cả resolved

1. ~~**Notifications: module riêng, hay sub-feature của social?**~~ → **Resolved [D-1]** — module `notification` standalone; emitter publish task Asynq `notify:*`, không có dependency ngược.
2. ~~**Lựa chọn search engine.**~~ → **Resolved [D-2]** — Postgres FTS trước; re-evaluate Meilisearch trong Phase 8.
3. ~~**Transport realtime.**~~ → **Resolved [D-3]** — SSE cho push stream, WebSocket cho chat, backplane pub/sub Dragonfly qua `platform/realtime/`.
4. ~~**Provider email.**~~ → **Resolved [D-4]** — chỉ SMTP qua `platform/mail/`; provider user-configurable; Mailpit ở dev.
5. ~~**Web/mobile push.**~~ → **Resolved [D-5]** — chỉ Web Push (VAPID); APNS/FCM defer.
6. ~~**Mobile client.**~~ → **Resolved [D-6]** — PWA-first; preserve tương thích bearer-token trên mọi route API.
7. ~~**i18n / l10n.**~~ → **Resolved [D-7]** — chỉ frontend qua `next-intl`; backend trả code + RFC 7807; cột mới `users.locale` + `users.timezone`.

### 16.B — Operations / infra ✓ tất cả resolved

8. ~~**Stack observability.**~~ → **Resolved [D-8]** — Grafana stack (Loki + Prometheus + Tempo + Grafana) + GlitchTip sau flag compose opt-in `--profile observability`.
9. ~~**CI/CD.**~~ → **Resolved [D-9]** — GitHub Actions; check drift + roundtrip; target coverage per module.
10. ~~**Backups.**~~ → **Resolved [D-10]** — `pgbackrest` + replication MinIO→R2 + Dragonfly `BGSAVE`; drill restore hàng quý; ma trận RPO/RTO per surface.
11. ~~**Quản lý secret ở production.**~~ → **Resolved [D-11]** — tiered: `.env` (dev), Compose/K8s secret (prod), SOPS optional, Vault defer. Policy rotation per class secret.
12. ~~**Policy down-migration.**~~ → **Resolved [D-12]** — production là forward-only; down chỉ cho dev + CI roundtrip; revert qua migration forward mới.
13. ~~**Planning capacity transcode.**~~ → **Resolved [D-13]** — software x264 default, NVENC/VAAPI opt-in; quota per-user + per-tenant; backpressure trên queue depth.

### 16.C — Schema / data model ✓ tất cả resolved

14. ~~**Representation Money / decimal.**~~ → **Resolved [D-14]** — `numeric(20,8)` + `shopspring/decimal` + type value `Money` với arithmetic currency-safe; table seed `bank.currencies` drive display.
15. ~~**Bookkeeping single-entry vs double-entry trong bank.**~~ → **Resolved [D-15]** — internal double-entry hybrid, UI feel single-entry; category là account income/expense auto-tạo; `ledger_entries` có `CHECK` balance.
16. ~~**Model counterparty trong bank.**~~ → **Resolved [D-16]** — table `bank.counterparties` per-owner với link `user_id` optional; confirmation hai-chiều cho settlement portal-user.
17. ~~**Canonicalisation time-zone.**~~ → **Resolved [D-17]** — storage UTC `timestamptz`; date boundaries user-TZ khắp nơi; scheduler per-TZ hourly cho snapshot hàng ngày.
18. ~~**Audit migration `0001`.**~~ → **Resolved [D-18]** — split đầy đủ thành `0001_platform_init` / `0002_account_users` (+locale/tz) / `0003_account_rbac` / `0005_media_assets` / v.v.
19. ~~**Split Profile vs Account.**~~ → **Resolved [D-19]** — field identity-critical ở lại trên `users`; bio/education/employment/hobbies move sang `social.profiles` trong Phase 7.
20. ~~**Module `progress` shared vs table per-domain.**~~ → **Resolved [D-20]** — table per-domain với shape shared; aggregator `GET /api/v1/continue` trong `cmd/api`.
21. ~~**Module ratings/reviews shared vs per-domain.**~~ → **Resolved [D-21]** — table per-domain `<module>.ratings`; không module shared; aggregator cross-domain defer.
22. ~~**Thống nhất tag/taxonomy.**~~ → **Resolved [D-22]** — hybrid: genre enumerated `text[]` per content table; free-text tag qua `text[]` + GIN; category bank ở lại trong module.
23. ~~**Đường identification tenant.**~~ → **Resolved [D-23]** — URL prefix `/t/{tenant}/...`; tenant synthetic `me` cho data cá nhân.
24. ~~**"Household" vs "Tenant".**~~ → **Resolved [D-24]** — household = tenant nhỏ; `tenant.kind` (`'org' | 'household'`) land trong schema initial của Phase 1.
25. ~~**Vị trí table audit log.**~~ → **Resolved [D-25]** — moved sang `platform/audit/`; taxonomy event `<module>.<resource>.<action>` register trong `backend/MODULES.md` §5.3.

### 16.D — Auth / RBAC ✓ tất cả resolved

26. ~~**OIDC group → role sync.**~~ → **Resolved [D-26]** — grant hai-trục hybrid; group Authentik → role global qua `OIDC_GROUP_ROLE_MAP`; grant tenant-scoped chỉ Portal; bootstrap qua `BOOTSTRAP_ADMIN_OIDC_SUBJECTS`.
27. ~~**Step-up auth.**~~ → **Resolved [D-27]** — OIDC ACR-based; middleware `RequireACR` trả 403 + Problem `step_up_required`; opt-in tường minh per-route; window default 5 phút.
28. ~~**2FA / TOTP.**~~ → **Resolved [D-28]** — hoàn toàn Authentik-managed; Portal enforce "MFA bắt buộc cho user permission bank" lúc login qua claim `amr`; settings deep-link sang MFA dashboard Authentik.

### 16.E — API / contract ✓ tất cả resolved

29. ~~**Gap coverage OpenAPI.**~~ → **Resolved [D-29]** — spec-first không thương lượng; monolith cho tới ~2000 dòng; schema cross-module eager (Problem, Money, PaginatedResult, TenantContext, ContinuingItem) land trong Phase 0.
30. ~~**Shape error.** Adopt RFC 7807 `Problem`?~~ → **Resolved [D-7]** — RFC 7807 `Problem` adopt trong Phase 0 (fold vào quyết định i18n; URI `type` là key i18n).
31. ~~**Policy versioning API.**~~ → **Resolved [D-31]** — URL versioning `/api/v{N}/`; additive trong major; deprecation RFC 9745 (sunset 6-tháng) khi v2 bị force.

### 16.F — Frontend ✓ tất cả resolved

32. ~~**Ranh giới Zustand vs TanStack.**~~ → **Resolved [D-32]** — TanStack sở hữu server state (rule cứng: không data API trong Zustand); Zustand sở hữu UI state; React Hook Form sở hữu form state; URL param cho filter chia sẻ được. Document trong `frontend/CLAUDE.md`.
33. ~~**SSR vs CSR cho catalogue.**~~ → **Resolved [D-33]** — RSC-first cho shell catalogue/detail; island client cho interactivity; player/reader chủ yếu client; default sang server component, opt vào `'use client'` chỉ khi cần.
34. ~~**Auth handoff cho RSC.**~~ → **Resolved [D-34]** — API client server-only wrap `fetch` với forwarding `cookies()`; route refresh-and-return xử lý 401; Next.js + API phải chia sẻ registrable domain.

### 16.G — Advanced social, creator economy, safety ✓ tất cả resolved

Những cái này surface từ mở rộng feature §9-13. Mỗi cái defer cho đến khi Phase của nó vào focus; resolved như lựa chọn kiến trúc với interface plug-in nên operator riêng có thể substitute alternatives.

35. ~~**Algorithm "For You".**~~ → **Resolved [D-35]** — pipeline ba-lớp hand-tuned (candidates → ranking → diversity); "Following" chronological là tab default; "Why am I seeing this?" + UI preference ranking hedge risk DSA; ML defer sang v2.
36. ~~**Hạ tầng live streaming.**~~ → **Resolved [D-36]** — RTMP ingest + distribution LL-HLS qua sidecar `mediamtx`; replay là auto-VOD; chat live reuse lớp realtime; cap concurrent-stream per-tenant.
37. ~~**Attribution & licensing music reels.**~~ → **Resolved [D-37]** — chỉ audio user-uploaded; chain attribution viral-sound qua `social.sounds`; workflow DMCA take-down trong Phase 11; commercial library defer sang plug-in.
38. ~~**Lựa chọn classifier NSFW / CSAM.**~~ → **Resolved [D-38]** — interface `ImageClassifier` + `TextClassifier` pluggable; default NSFWJS + pHash cho self-host; CSAM match block + quarantine + page; vendor API (AWS, Hive, Perspective) như plug-in.
39. ~~**Lựa chọn WebRTC SFU.**~~ → **Resolved [D-39]** — LiveKit SFU cho group call + audio room; P2P cho 1:1; service compose `livekit` mới sau `--profile calls`.
40. ~~**Provider creator-payout.**~~ → **Resolved [D-40]** — interface `Provider` pluggable; `manual` là default v1; Stripe Connect là integration thực đầu; provider khác theo demand operator. Step-up auth ([D-27]) gate mỗi payout.

---

## 20. Decisions log

Open question đã resolve kèm rationale. Mỗi entry có id stable `D-N`. **Không bao giờ edit decision in place** — nếu PR tương lai lật ngược, append revision mới `D-N.r1` bên dưới với rationale mới và ngày.

### D-1 — Notifications: module `notification` standalone *(resolve §16.A-1)*

Emitter span account, bank, media, mọi module content, tenant, và cuối cùng social. Nếu notification là sub-feature của social, thì bank / account / media sẽ phụ thuộc internal của social — vi phạm trực tiếp rule api-only trong [backend/MODULES.md](../../backend/MODULES.md) §4.

**Decision:** module standalone tại `backend/internal/modules/notification/`. Sở hữu `notifications`, `notification_preferences`, `delivery_attempts`, `push_subscriptions`. Subscribe mọi task Asynq `notify:*`; emitter không bao giờ call vào module.

**Side effects:** reserve prefix Asynq `notify:*` trong `backend/MODULES.md` §5.2 (deliverable Phase 0).

### D-2 — Search: Postgres FTS trước, defer Meilisearch *(resolve §16.A-2)*

User self-hosted đẩy lùi mỗi service thêm. Postgres FTS (`tsvector` + `pg_trgm` + GIN) cover hầu hết nhu cầu với zero infra mới và inherit RLS native. Meilisearch / Typesense thắng về typo-tolerance và ranking nhưng thêm service.

**Decision:** mỗi module expose `<module>api.Search(ctx, q, opts)` back bằng column `tsvector` và index `GIN`. Endpoint aggregator mỏng trong `cmd/api` fan-out across API module và merge. **Phase 8 re-evaluate** — nếu chất lượng không đủ, swap index-builder của mỗi module sang push doc vào Meilisearch; surface API không đổi.

### D-3 — Realtime: SSE cho push, WebSocket cho chat, backplane Dragonfly *(resolve §16.A-3)*

Ba nhu cầu real-time: notification stream (push-only), media event (push-only), chat (bi-directional với typing/presence). Hai cái đầu shape SSE; chỉ chat thật sự cần WS.

**Decision:** package mới `backend/internal/platform/realtime/` expose `Publish(ctx, channel, event)` / `Subscribe(ctx, channel) <-chan Event` over Dragonfly pub/sub. Endpoint:
- `GET /api/v1/events/stream` (SSE, authed, channel = `user:<id>`) — Phase 6.
- `GET /api/v1/chat/ws` (WebSocket qua `coder/websocket`, trước đây `nhooyr/websocket`) — Phase 7.

Không service external (Centrifugo, Soketi, v.v.) trừ khi scale demand.

### D-4 — Email: chỉ SMTP qua `platform/mail/` *(resolve §16.A-4)*

Lock-in vendor SDK (Resend, Postmark, SendGrid) exclude self-hoster với SMTP corp, AWS SES, Mailgun, Mailpit, Postfix, hoặc Gmail SMTP. SMTP là sàn universal.

**Decision:** `backend/internal/platform/mail/` expose interface `Mailer` với một implementation SMTP (`wneessen/go-mail`). Mailpit ở dev. Template: Go `html/template` dưới `backend/templates/email/<category>/*.html.tmpl`.

**Env vars mới** (phải land trong `.env.example` trước Phase 6):

```
MAIL_HOST=mailpit
MAIL_PORT=1025
MAIL_USERNAME=
MAIL_PASSWORD=
MAIL_FROM_NAME=Portal
MAIL_FROM_ADDRESS=no-reply@portal.localhost
MAIL_ENCRYPTION=none           # none | tls | starttls
```

### D-5 — Push: chỉ Web Push (VAPID); defer APNS/FCM *(resolve §16.A-5)*

Không có native mobile trong v1 (xem [D-6]) ⇒ APNS/FCM out of scope. Web Push cover desktop Chrome/Firefox/Edge, Android Chrome, và iOS 16.4+ PWA installed. Library: `SherClockHolmes/webpush-go`. Key VAPID generate locally; không cần vendor account.

**Decision:** module notification sở hữu `push_subscriptions(user_id, endpoint, p256dh, auth, user_agent, created_at, last_seen_at)`. Service worker frontend subscribe với key VAPID public; subscription POST sang `/api/v1/notification/subscriptions`. Dispatcher prune endpoint trả 410 Gone.

**Env vars mới:**

```
WEB_PUSH_VAPID_PUBLIC=
WEB_PUSH_VAPID_PRIVATE=
WEB_PUSH_SUBJECT=mailto:admin@portal.localhost
```

### D-6 — Mobile: PWA-first; preserve tương thích bearer-token *(resolve §16.A-6)*

Native mobile là cost thật (React Native: shared OpenAPI client nhưng auth + push riêng; native: 2× maintenance). Cho v1, Next.js như PWA installable cover hầu hết usage; Web Push xử lý surface notification.

**Decision:** không có native client trong v1. **Constraint cứng preserved cho v2:** mọi route API PHẢI accept `Authorization: Bearer` (cookie là tiện lợi, không phải mode duy nhất). Module account đã comply. CI lint sẽ fail mọi handler 401 trên request bearer-only hợp lệ.

### D-7 — i18n: chỉ frontend qua `next-intl`; backend trả code + RFC 7807 *(resolve §16.A-7; cũng resolve §16.E-30)*

String backend interpolate vào alert UI là tax late-stage đã biết. Tránh bằng cách trả code machine-readable từ ngày 1. RFC 7807 `Problem` (đã là candidate ở §16.E-30) là carrier tự nhiên — URI `type` là key i18n.

**Decision:**

1. **Contract error** — mỗi 4xx/5xx trả RFC 7807 `Problem` với URI `type` stable (vd `https://portal/errors/auth.refresh.reuse`). Land trong [shared/openapi.yaml](../../shared/openapi.yaml) trong Phase 0.
2. **Money** — luôn `{ amount: "12345.67", currency: "USD" }` trong API; không bao giờ pre-formatted. Frontend dùng `Intl.NumberFormat(user.locale, { style: 'currency', currency })`.
3. **Dates** — backend trả ISO 8601 UTC. Frontend format theo `users.locale` + `users.timezone`.
4. **Cột User** — `users.locale TEXT NOT NULL DEFAULT 'en-US'` (BCP 47), `users.timezone TEXT NOT NULL DEFAULT 'UTC'` (IANA). Thêm trong audit migration `0001` (§16.C-18).

Frontend dùng `next-intl`. Backend translation defer cho đến khi content không-English thật sự xuất hiện — message catalogue ở lại trên frontend.

### D-8 — Observability: Grafana stack + GlitchTip sau profile compose opt-in *(resolve §16.B-8)*

Self-host friendliness loại vendor SaaS (Datadog, Honeycomb, Grafana Cloud). Grafana stack cover cả bốn trụ cột (log, metric, trace, error) dưới một UI; cost là ~5 service compose thêm, made opt-in nên self-host single-VM có thể skip.

**Decision:** bốn service open-source gate sau `--profile observability` trong `docker-compose.yml`:

| Trụ cột | Tool | RAM approx |
|---|---|---|
| Logs | Promtail → Loki → Grafana | ~200 MB |
| Metrics | `prometheus/client_golang` → Prometheus → Grafana | ~300 MB |
| Traces | OTel SDK → Tempo → Grafana (auto-instrument chi + pgx + asynq + http client) | ~200 MB |
| Errors | `getsentry/sentry-go` → GlitchTip | ~400 MB |

Package mới `backend/internal/platform/observability/` sở hữu init OTel + Sentry. `/metrics` expose trên `METRICS_PORT` (riêng với port API public — không Traefik-route). Sentry SDK là no-op khi `GLITCHTIP_DSN` empty.

**Env vars mới:**

```
OTEL_EXPORTER_OTLP_ENDPOINT=http://tempo:4318
OTEL_SERVICE_NAME=portal-api
METRICS_PORT=9100
GLITCHTIP_DSN=                  # đã có, plumb through
```

Land cùng Phase 1 để performance RLS measurable từ ngày 1.

### D-9 — CI/CD: GitHub Actions với check drift + roundtrip *(resolve §16.B-9)*

Code generate (sqlc, oapi-codegen) drift silent trong review; CI phải bắt. GitHub Actions là standard cho OSS với phút public-repo free; Forgejo/Gitea Actions tương thích nếu project move sau.

**Decision:** ba pipeline dưới `.github/workflows/`:

- **`ci.yml` (per PR):**
  - `lint` — `golangci-lint run` + `pnpm lint`.
  - `test` — matrix `[unit, integration]`; integration spin postgres + dragonfly qua compose.
  - `drift` — `make sqlc && git diff --exit-code` + same cho `make openapi`.
  - `migration-roundtrip` — up → down → up; assert schema không đổi.
  - `build` — matrix `[api, worker, frontend]`, multi-arch (amd64 + arm64) qua `docker buildx`.
  - `security` — `govulncheck ./...` + `pnpm audit --audit-level=high`.
- **`release.yml` (main + tag):** push image sang GHCR với tag SHA; tag semver lúc release.
- Pre-commit optional qua `lefthook` chạy `gofmt + golangci-lint --fast + pnpm lint --fix` trên file staged.

**Coverage target** (đề xuất):

| Module | Target | Rationale |
|---|---|---|
| `account` | 80% | Bug auth + RBAC là incident bảo mật |
| `bank` | 80% | Correctness tài chính không thương lượng |
| `media` | 60% | Orchestration FFmpeg integration-test heavy |
| Module khác | 60% | CRUD + state machine standard |
| `platform/*` | 70% | Cross-cutting; vỡ ảnh hưởng mọi người |

Land trong Phase 0.

### D-10 — Backups: pgbackrest + MinIO→R2 + Dragonfly BGSAVE; drill restore *(resolve §16.B-10)*

Backup chưa test không tồn tại. Surface khác cần target recovery khác.

**RPO / RTO target** (đề xuất; cần stakeholder confirm):

| Surface | RPO | RTO | Vì sao |
|---|---|---|---|
| `account` + `bank` | 5 phút | 1 giờ | State auth + money là stake cao nhất |
| `tenant` + `media` metadata | 1 giờ | 4 giờ | Recover được nhưng disrupt |
| Asset blob (original) | 24 giờ | 4 giờ | User có thể re-upload nếu cần |
| `social` (khi land) | 1 giờ | 4 giờ | Post ngày tệ nhất là loss chấp nhận được |

**Decision:**

- **Postgres** — sidecar `pgbackrest` cạnh container Postgres: incremental + WAL streaming (~5 phút RPO cho tier bank/account). `pg_dump` logical hàng tuần như belt-and-braces (bắt corruption WAL replay sẽ propagate).
- **MinIO** — replication liên tục sang R2 qua `mc admin replicate`. Self-hoster không có R2 fall back sang MinIO node thứ hai hoặc `rclone sync` sang target S3-compatible bất kỳ.
- **Dragonfly** — `BGSAVE` hàng ngày → bucket backup. Task Asynq persist khi enqueue, nên RPO worst-case ~5 phút match bởi combination in-memory + snapshot RDB của Dragonfly.
- **Encryption at rest** trong bucket backup; key per-tenant nếu available.
- **Discipline** — drill restore hàng quý (chọn snapshot ngẫu nhiên, restore sang sandbox, verify một transaction gần đây xuất hiện). Metric Prometheus `backup_last_success_timestamp` → page on staleness.

Doc mới: `docs/operations/backups.md`. Deliverable pre-prod-launch.

### D-11 — Secrets: tiered (.env dev, Compose/K8s secret prod, SOPS optional, Vault defer) *(resolve §16.B-11)*

`.env` trên server là footgun (shell-readable, leak qua backup). Vault heavy cho đến khi dynamic credential yêu cầu. Self-host friendliness thắng over feature completeness.

**Decision:**

- **Dev:** file `.env` (hiện tại).
- **Self-host prod:** Docker Compose secret (Swarm) hoặc Kubernetes secret. `platform/config` đọc env var bất kể source — orchestration inject chúng.
- **Git-ops shop:** SOPS-encrypted `secrets.enc.yaml` decrypt lúc deploy vào env. Drop-in.
- **HashiCorp Vault:** defer cho đến khi dynamic DB credential hoặc compliance audit force vấn đề.

**Policy rotation** (trong `docs/operations/secrets.md`):

| Secret | Cadence | Notes |
|---|---|---|
| `JWT_SIGNING_KEYS` | Hàng quý | Set key comma-separated hỗ trợ window overlap |
| `OIDC_CLIENT_SECRET` | Theo policy Authentik (typical annual) | |
| `WEB_PUSH_VAPID_*` | **Không bao giờ** | Rotation invalidate mọi subscription push |
| `POSTGRES_PASSWORD` | Hàng quý + on personnel change | Coordinate với reload PgBouncer |
| `S3_*` / `R2_*` | Hàng quý | Atomic swap; app re-đọc env lần request kế tiếp |
| `MAIL_PASSWORD` [D-4] | Theo policy provider | |
| `GLITCHTIP_DSN` | Không bao giờ (chỉ là URL) | |

### D-12 — Migrations: forward-only ở production *(resolve §16.B-12)*

`DROP COLUMN` huỷ diệt data; coordinate down-migration với app server đang chạy là brittle. Migration "reverse" forward review được, test được, atomic với deploy.

**Decision:** production là **forward-only**. `make migrate-down` reserve cho:
1. Local dev — iterate trên up.sql viết năm phút trước.
2. CI — job migration-roundtrip (up → down → up) bắt typo trong down.sql.

Để revert thay đổi schema production, ship migration forward mới reverse nó.

**Pattern: expand → migrate-data → contract** across hai deploy. Ví dụ:

- **Rename column:** thêm column mới → write cả hai → backfill → switch reads → drop cũ. Ba migration, hai deploy.
- **Tighten sang NOT NULL:** thêm column nullable → backfill → thêm constraint.
- **Type change:** thêm column typed mới → dual-write → backfill → switch reads → drop cũ.

Mỗi up PHẢI backward-compatible với version app trước. Document trong `docs/operations/migrations.md`; summary một-đoạn thêm vào `CLAUDE.md` "Working in this repo".

Module bank hưởng lợi nhất — data tài chính không bao giờ được mất khi rollback.

### D-13 — Transcode: software x264 default, HW accel opt-in, quota + backpressure *(resolve §16.B-13)*

FFmpeg CPU-bound; trên self-host single-VM, transcode là giới hạn. Không có quota, một user upload 100 file và queue stall hàng giờ.

**Decision:**

- **Encoder:** `libx264` default; opt-in `h264_nvenc` (NVIDIA), `h264_vaapi` (Intel/AMD), `h264_qsv` (Intel QSV) qua env `TRANSCODE_ENCODER`. Recipe GPU passthrough document riêng.
- **Concurrency:** `TRANSCODE_CONCURRENCY` per worker (default 1; upper bound hợp lý `max(1, nproc - 1)`).
- **HLS ladder:** 1080p / 720p / 480p / 360p, segment 6 giây (Apple default). Skip 240p. Adaptive: detect resolution input, skip rung trên đó.
- **Audio:** AAC 128 kbps single track trừ khi input mang nhiều track ngôn ngữ.
- **Quota per-user:** `MAX_CONCURRENT_TRANSCODES_PER_USER` (default 2). Enforce lúc enqueue qua introspection Asynq trên task tagged `user_id`.
- **Cap per-tenant:** `MAX_QUEUED_TRANSCODES_PER_TENANT` (default 200). Hard cap — reject với 429 + Retry-After.
- **Backpressure:** khi queue depth × avg-transcode-duration > 30 phút, reject upload mới.
- **Failure handling:** auto-retry chỉ failure transient (network blip, OOM). Sau 3 retry → queue `transcode:dead`, cần action operator. Codec error và input corrupt là terminal từ attempt đầu.

**Sizing reference** (hardware commodity, preset libx264 medium; order-of-magnitude — measure trên hardware thực):

| Source | Hardware | Wall-clock per phút source |
|---|---|---|
| 1080p mp4 | 2 vCPU | ~3 phút |
| 1080p mp4 | 4 vCPU | ~90 giây |
| 1080p mp4 | NVIDIA T4 (NVENC) | ~10 giây |
| 4K mp4 | 4 vCPU | ~6 phút |
| 4K mp4 | NVIDIA T4 | ~30 giây |

**Env vars mới:**

```
TRANSCODE_ENCODER=libx264                 # libx264 | h264_nvenc | h264_vaapi | h264_qsv
TRANSCODE_CONCURRENCY=1                   # per worker
TRANSCODE_LADDER=1080p,720p,480p,360p     # comma-separated
TRANSCODE_HLS_SEGMENT_SECONDS=6
MAX_CONCURRENT_TRANSCODES_PER_USER=2
MAX_QUEUED_TRANSCODES_PER_TENANT=200
```

Doc mới: `docs/operations/transcode.md` với bảng sizing + recipe GPU passthrough. Land trong Phase 2.

### D-14 — Money: `numeric(20,8)` + `shopspring/decimal` + type value `Money` currency-safe *(resolve §16.C-14)*

`numeric(20,8)` (12 chữ số nguyên + 8 phân số) cover độ chính xác BTC và mọi fiat tới penny. Wei-precision crypto (18 decimal) là edge case — opt-in per-account `numeric(40,18)` chỉ nếu yêu cầu tương lai đến. `int64` cents quá hẹp (vỡ với crypto và currency 3-decimal như JOD). `shopspring/decimal` là chuẩn ecosystem.

**Decision:**

- **Storage:** mỗi column money là `numeric(20,8) NOT NULL`.
- **Go:** `bank/internal/money/Money` wrap `shopspring/decimal.Decimal` với tag currency. `Add(Money) (Money, error)` từ chối currency không match; yêu cầu FX conversion tường minh.
- **Wire:** string amount + ISO 4217 currency code (đã mandate bởi [D-7]). Không bao giờ JSON number.
- **Rounding:** `decimal.RoundBank` (banker's rounding) cho halves; override tường minh per-call.
- **Table currency:** `bank.currencies(code char(3) primary key, decimal_places smallint, symbol text, name text)`, seed từ ISO 4217 + crypto thông dụng. `decimal_places` drive **chỉ format display**; arithmetic dùng `numeric(20,8)` đầy đủ bất kể.

Land trong Phase 5a.

### D-15 — Bank ledger: hybrid internal double-entry, UI feel single-entry *(resolve §16.C-15)*

Riêng investment force semantic double-entry. Một buy là "−cash, +holding"; sell-with-gain là "+cash, −holding, +investment-gain". Chọn single-entry trước tạo migration painful khi investment land trong 5e. Trả cost upfront ~30% complexity table thêm đáng cho boundary discipline cộng report provable nhỏ.

**Decision:**

- **Loại account:** `ASSET | LIABILITY | INCOME | EXPENSE | EQUITY`. Category là account INCOME/EXPENSE auto-tạo; user không bao giờ thấy chúng như "account".
- **Schema:**
  ```sql
  bank.accounts(id, user_id, name, type, currency, opening_balance, ...)
  bank.transactions(id, user_id, date, description, ...)
  bank.ledger_entries(transaction_id, account_id, amount, currency)
    CHECK: SUM(amount) = 0 per (transaction_id, currency)
  ```
- **Splits** = nhiều ledger entry expense-side trên một transaction.
- **Transfers** = hai entry asset-account (không có income/expense involve).
- **Investment buy:** `holding.AAPL +$1000, checking −$1000`.
- **Investment sell với gain:** `checking +$1100, holding.AAPL −$1000, income.investment-gain +$100`.
- **UI không bao giờ show offset entry.** "Transaction" user-facing feel single-entry.

Report provable bởi constraint SQL — mọi TB cân. Land trong Phase 5a.

### D-16 — Counterparty: table per-owner, link `user_id` optional, confirmation settlement hai-chiều *(resolve §16.C-16)*

Free-text mất aggregation ("how much at Whole Foods this year?"). Row shared giữa portal user phá audit isolation và personal naming ("Mom" vs "John").

**Decision:**

```sql
bank.counterparties(
  id uuid,
  owner_user_id uuid not null,
  name text not null,           -- encrypted at rest theo §8.12
  type text not null,           -- 'person' | 'institution' | 'merchant'
  user_id uuid null,            -- FK sang users.id khi counterparty LÀ portal user
  created_at, updated_at
);
```

- Hầu hết row có `user_id = NULL` (Whole Foods không phải portal user).
- Khi `user_id` CÓ, **mỗi bên sở hữu row riêng** point sang cùng portal user — name độc lập, audit độc lập.
- **UX Settle-a-debt**: matching row debt/loan trên cả hai bên. Một bên mark settled → bên kia nhận `notify:bank.debt.settle_pending` → phải confirm trước khi book nào close. Confirmation hai-chiều chống write đơn phương.

Land trong Phase 5c.

### D-17 — Time zone: storage UTC, boundaries user-TZ, scheduler snapshot per-TZ hourly *(resolve §16.C-17)*

Storage UTC là nửa dễ. Nửa khó là **"khi nào một day bắt đầu?"** — report monthly, "yesterday's transactions", recurring rule, daily snapshot đều cần semantic user-TZ. Offset cố định naive vỡ across DST.

**Decision:**

- **Storage:** mỗi timestamp là `timestamptz` (UTC + offset). Không bao giờ `timestamp` naive.
- **Wire:** ISO 8601 với UTC offset tường minh (đã trong [D-7]).
- **Date boundaries:** tính trong `users.timezone` (đã thêm bởi [D-7]). Query report convert ở lớp SQL: `WHERE occurred_at >= ($from AT TIME ZONE $tz)::timestamptz`.
- **Recurring rule:** "1st of month" nghĩa 1st trong user TZ; generator recurring-task phải consult `users.timezone`.
- **Daily snapshot:** scheduler **hourly** iterate `users WHERE timezone vừa hit 00:05 trong giờ vừa qua` và enqueue task `bank:snapshot_daily` per-user. Tránh fan-out N cron trigger per day.
- **Tên không offset:** chỉ tên IANA TZ (`Europe/Amsterdam`); không bao giờ `UTC+1`. DST xử lý bởi tzdata IANA.

Cross-cutting; land bất cứ đâu report date-bounded ship lần đầu (Phase 5g cho snapshot bank).

### D-18 — Audit migration `0001`: split đầy đủ *(resolve §16.C-18)*

Chưa có data production — split một lần cost ít hơn sống với naming mixed-concerns.

**Decision:** rewrite tree migration trước khi Phase 0 close:

```
0001_platform_init.up.sql        extension (uuid-ossp, unaccent, pg_trgm) + type chung
0002_account_users.up.sql        users (không col role); + locale + timezone (D-7); + token_version + disabled_at
0003_account_rbac.up.sql         hiện tại 0002_account_rbac renumber
0004_tenant_organizations.up.sql Phase 1; gồm discriminator tenant.kind (D-24)
0005_media_assets.up.sql         table assets extract từ 0001 cũ
0006_movie_init.up.sql ...
0009_comic_init.up.sql
0010_rls_enable.up.sql           RLS trên mọi table tenant-scoped
0011+_bank_*.up.sql              Phase 5
```

FK `assets.owner_id` sang `users.id` vẫn hợp lệ vì users (`0002`) land trước assets (`0005`). Table audit log move sang `platform/audit/` trong cùng pass (xem [D-25]). Land trong Phase 0.

### D-19 — Split Profile vs Account: identity trên `users`, profile rich trong `social.profiles` *(resolve §16.C-19)*

Piling bio/education/employment/hobbies lên `users` bloat table auth-hot đọc mỗi request. Module `profile` dedicated premature; để social sở hữu page profile khi nó đến.

**Decision:**

- **Ở lại trên `users`** (module account): `id`, `oidc_subject`, `email`, `display_name`, `avatar_url` (thumbnail), `locale`, `timezone`, `token_version`, `disabled_at`. Đủ cho auth + string display audit.
- **Move sang `social.profiles`** trong Phase 7 (1:1 với `users`): `bio`, `dob`, `gender`, `location`, `education`, `employment`, `hobbies`, `cover_image_url`, `widgets jsonb`.
- **Không có module `profile` premature.** Nếu surface profile-page cuối cùng outgrow social (portfolio + resume), revisit bằng cách extract `profile` từ social sau — không phải bây giờ.

Phase 0–6 chỉ dùng `users.display_name` + `users.avatar_url`. Field UI account-settings ngoài đó không materialize cho tới Phase 7.

### D-20 — Progress: table per-domain với shape shared; endpoint aggregator *(resolve §16.C-20)*

Module `progress` shared sẽ vi phạm "module sở hữu data riêng" và yêu cầu round-trip validate `content_id` tồn tại. Table per-domain với contract API thống nhất preserve module boundary.

**Decision:** mỗi module content sở hữu table progress với column layout giống nhau:

```sql
movie.watch_progress(user_id, movie_id,     position_seconds, duration_seconds, updated_at)
music.listen_progress(user_id, track_id,    position_seconds, duration_seconds, updated_at)
story.read_progress(user_id, chapter_id,    position_words,   total_words,      updated_at)
comic.read_progress(user_id, chapter_id,    page_number,      total_pages,      updated_at)
```

Rail "continue" cross-domain aggregate qua API của mỗi module:

```go
// Trong <module>api:
Continue(ctx context.Context, userID uuid.UUID, limit int) ([]ContinuingItem, error)

// Type shared (trong package platform):
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

Aggregator `GET /api/v1/continue` trong `cmd/api` fan-out, merge, trả sort theo `updated_at DESC`. Land trong Phase 4.

### D-21 — Ratings: table per-domain; không module shared *(resolve §16.C-21)*

Cùng shape như [D-20] nhưng case cho centralisation yếu hơn — query rating dominate bởi "rating cho content này" (module-local). Surface "top rated everywhere" cross-domain hiếm; defer cho đến khi UI demand.

**Decision:** per-domain `<module>.ratings(user_id, content_id, rating smallint, review text, created_at, updated_at)`. Không platform helper. Endpoint aggregator defer. Land per content module trong Phase 4.

### D-22 — Tags / taxonomies: hybrid; genre enumerated, free-text tag là `text[]`, category per module *(resolve §16.C-22)*

Ba thú khác nhau (enumeration genre closed, label free-text user-input, category phân cấp của bank) không fit một table taxonomy. Validation, visibility, lifecycle khác.

**Decision:**

- **Genres** (enumeration closed per loại content): `genre TEXT[]` trên mỗi table content; seed list per module. App-layer validate against seed list.
- **Free-text tag** (label user-input): cột `tags TEXT[]` + index `GIN` trên bất cứ table nào cần (`bank.transactions.tags`, `social.posts.tags`). Query `WHERE 'vacation-2026' = ANY(tags)` nhanh dưới GIN. Skip table junction — pragmatic over normalised.
- **Category bank:** phân cấp, bank-specific, ở lại trong module.

Không table `tags` centralised. Không package `platform/tags/`. Chỉ convention được document.

### D-23 — Identification tenant: URL prefix `/t/{tenant}/...`; tenant synthetic `me` *(resolve §16.C-23)*

Subdomain (`acme.portal.localhost`) tạo friction DNS + wildcard-cert cho self-hoster. Header-only (`X-Tenant`) phá link sharing. Token-bound brittle dưới truy cập multi-tenant (refresh yêu cầu để switch).

**Decision:** mỗi endpoint tenant-scoped sit dưới `/t/{tenant}/...`:

- `/t/acme/api/v1/movies` — movie của org Acme.
- `/t/me/api/v1/bank/...` — data bank cá nhân qua tenant synthetic `me` per user.
- `/api/v1/healthz` — route non-tenant giữ path phẳng.

**Middleware** (`platform/middleware/tenant.go`):

1. Extract slug `tenant` từ path.
2. Resolve `tenant_id` từ slug (cached).
3. Verify caller là member (table membership) HOẶC `slug=me` match user caller.
4. Set GUC `app.tenant_id` cho RLS qua `db.BeginTenantScope`.

Deployment single-tenant map `/api/v1/...` thẳng vào tenant default hardcoded qua middleware rewrite Traefik — không thay đổi phía app. Land trong Phase 1.

### D-24 — Household = tenant nhỏ với discriminator `kind` *(resolve §16.C-24)*

Reuse hạ tầng tenancy (predicate RLS, table membership, audit). Shape storage giống; granularity role và UX khác.

**Decision:** cột `tenant.kind` thêm trong schema initial Phase 1:

- **`org`:** hierarchy role đầy đủ (admin / editor / member / viewer); member không giới hạn; module bank disable mặc định.
- **`household`:** role đơn giản (chỉ `owner` + `member`); soft cap 6 member enforce ở app layer; module bank enable với sharing đầy đủ.
- **Predicate RLS không đổi:** `tenant_id = current_setting('app.tenant_id')::uuid` — indifferent với kind.
- **UX:** ẩn surface org-admin cho household; ẩn flow household-specific cho org.

"Household sharing" của bank (§8.11) tạo tenant `kind='household'` và assign cả hai user làm `owner`. Bản thân cột `kind` land trong migration `tenant.organizations` của Phase 1 nên Phase 5i không cần schema migration trên table populated.

### D-25 — Audit log: move sang `platform/audit/`; taxonomy event chuẩn hoá *(resolve §16.C-25)*

Audit là cross-cutting; sitting bên trong account là historical accident. Module khác (bank, tenant, media, social, notification) đều cần; bắt chúng call vào account vi phạm "không có dependency cross-module trên internal".

**Decision:** move `backend/internal/modules/account/audit/` → `backend/internal/platform/audit/`. Account trở thành consumer như mọi module khác.

**Schema:**

```sql
audit_log(
  id uuid primary key,
  occurred_at timestamptz not null default now(),
  actor_user_id uuid,           -- nullable cho action system
  tenant_id uuid,               -- nơi
  event_type text not null,     -- '<module>.<resource>.<action>'
  resource_kind text,
  resource_id uuid,
  payload jsonb,
  ip_address inet,
  user_agent text
);
```

**Taxonomy event-type:** `<module>.<resource>.<action>` (period-separated; riêng với prefix Asynq `notify:*`). Mỗi module document event của nó trong README; `backend/MODULES.md` §5.3 maintain registry aggregate để chống collision.

Ví dụ:

- `account.refresh.reuse_detected` (rename từ `auth.refresh.reuse_detected` cho fit taxonomy)
- `bank.transaction.created`, `bank.account.created`, `bank.debt.settle_pending`
- `tenant.member.invited`, `tenant.organization.created`
- `media.asset.failed`
- `notification.delivery.failed`

Audit vẫn best-effort, non-blocking (theo CLAUDE.md). Land trong Phase 0 cùng audit migration `0001` ([D-18]) — table audit-log move file cùng lúc với rename.

### D-26 — Roles: grant hai-trục hybrid; grant tenant-scoped chỉ Portal *(resolve §16.D-26)*

Quản lý role chỉ-Authentik là disaster UX — mỗi thay đổi cần admin IDP access, và group Authentik global nên role tenant-scoped (`creator on tenant X`) không fit. Quản lý chỉ-Portal có vấn đề bootstrap (admin đầu tiên không có nơi đến). Grant hai-trục hybrid cho path bootstrap mượt mà trong khi giữ Portal làm source of truth cho role tenant-scoped.

**Decision:**

- **Role global qua group Authentik.** Claim `groups` của ID token map sang role global Portal qua `OIDC_GROUP_ROLE_MAP=portal-admins:admin,portal-mods:moderator,portal-creators:creator`. Reconcile vào table join `user_oidc_roles` mỗi callback.
- **Grant tenant-scoped chỉ Portal.** Per-tenant `creator on tenant X` sống trong `user_roles` và quản lý qua UI admin Portal.
- **Effective permission = `user_oidc_roles` ∪ `user_roles`**, walked qua hierarchy role.
- **Remove user khỏi group Authentik** propagate lần login tiếp theo (reconciliation xoá row `user_oidc_roles` matching).
- **Bootstrap admin** qua env: `BOOTSTRAP_ADMIN_OIDC_SUBJECTS=sub1,sub2,sub3` grant `superadmin` mỗi callback cho `sub` values đó; remove khỏi env khi admin Portal có thể quản lý role in-app. Secondary `BOOTSTRAP_ADMIN_GROUPS=portal-bootstrap` accept nếu operator prefer bootstrap dựa group-Authentik.

**Schema addition** (land với audit migration, [D-18], trong `0003_account_rbac`):

```sql
user_oidc_roles(
  user_id uuid not null references users(id) on delete cascade,
  role_id uuid not null references roles(id),
  authentik_group text not null,    -- source-of-truth cho re-sync
  synced_at timestamptz not null default now(),
  primary key (user_id, role_id)
);
create index on user_oidc_roles(authentik_group);
```

Table riêng với `user_roles` nên luôn biết grant đến từ đâu. Audit event `account.role.granted_via_oidc` / `account.role.revoked_via_oidc` fire mỗi reconciliation.

**Env vars mới:**

```
OIDC_GROUP_ROLE_MAP=portal-admins:admin,portal-mods:moderator,portal-creators:creator
BOOTSTRAP_ADMIN_OIDC_SUBJECTS=
BOOTSTRAP_ADMIN_GROUPS=
```

Land trong Phase 0 (table `user_oidc_roles`) và handler callback OIDC.

### D-27 — Step-up auth: OIDC ACR-based; op nhạy cảm annotated tường minh *(resolve §16.D-27)*

Op bank + account + tenant nhạy cảm cần guarantee fresh hơn "session này tồn tại năm giờ trước". Re-prompt qua `acr_values` OIDC là practice standard (GitHub, Google, AWS đều làm equivalent).

**ACR levels cho Portal:**

| Level | Nghĩa |
|---|---|
| `acr:portal:basic` | Single-factor (chỉ OIDC password). |
| `acr:portal:mfa` | Second factor verify session này. |
| `acr:portal:recent_mfa` | Second factor verify trong 5 phút gần nhất. |

**Middleware:**

```go
r.With(account.RequireACR("acr:portal:recent_mfa")).
  Delete("/bank/accounts/{id}", h.DeleteAccount)
```

`RequireACR` đọc claim `acr` + `auth_time` từ access token. Không đủ → 403 với RFC 7807 `Problem`:

```json
{
  "type":         "https://portal/errors/auth.step_up_required",
  "title":        "Step-up authentication required",
  "status":       403,
  "required_acr": "acr:portal:recent_mfa",
  "return_to":    "/api/v1/t/me/bank/accounts/abc-123"
}
```

Frontend recognise `type`, redirect sang `/auth/login?step_up=mfa&return_to=...`, chạy lại OIDC với `acr_values=mfa prompt=login`. Sau re-auth thành công, claim `acr` của access token mới cho phép op.

**Window step-up:** 5 phút mặc định; configurable per middleware call (`RequireACR("...", account.WithWindow(2*time.Minute))`).

**Set gated initial** (không có list implicit — mỗi route gated opt in):

| Module | Op | ACR |
|---|---|---|
| bank | `accounts.delete` | `recent_mfa` |
| bank | `transactions.delete` | `recent_mfa` |
| bank | `export.csv`, `export.json` | `recent_mfa` |
| bank | `household.invite` | `recent_mfa` |
| bank | `debt.settle` (counterparty là portal user) | `recent_mfa` |
| account | `delete_self`, `email.change`, `mfa.disable` | `recent_mfa` |
| tenant | `organization.delete`, `ownership.transfer` | `recent_mfa` |

Land chung với [D-28] như prerequisite Phase 5.

### D-28 — 2FA: hoàn toàn Authentik-managed; Portal enforce MFA lúc login cho user permission bank *(resolve §16.D-28)*

Authentik đã ship TOTP, WebAuthn, SMS, push, recovery code, và UX enrollment polished. Re-implement bất kỳ cái nào trong Portal duplicate work, thêm store secret 2FA thứ hai để compromise, và split mental model user.

**Decision:**

- **Không có logic 2FA trong Portal.** Không TOTP secret lưu, không recovery code generate. Authentik sở hữu toàn bộ surface.
- **Enforcement lúc login.** Nếu user authenticated có permission `bank:*` nào và claim `amr` của ID token không gồm `mfa`, refuse session với Problem type `https://portal/errors/auth.mfa_enrollment_required` mang `enrollment_url`. Frontend redirect user enroll, rồi resume flow gốc.
- **Surface auth-context.** Middleware expose claim `amr`, `acr`, `auth_time` nên [D-27] và code MFA-aware tương lai có thể đọc chúng mà không re-parse JWT.
- **Deep-link UI settings.** Page account-settings có button "Manage MFA" mở dashboard user Authentik:

  ```
  ${OIDC_ISSUER}/if/user/#/settings;%7B%22page%22%3A%22page-mfa%22%7D
  ```

- **Config Authentik yêu cầu** (document trong `docs/operations/authentik.md`):
  - Stage: "TOTP authenticator setup" + "WebAuthn authenticator setup" (recommended).
  - Flow authentication: prompt MFA khi `acr_values=mfa` được yêu cầu trong URL auth.
  - Group: `portal-bank-users` (hoặc tag bất kỳ) — dùng bởi policy Authentik gate flow MFA-required ở phía IDP cũng, như defence in depth.

Land chung với [D-27] như prerequisite Phase 5. Step-up sang single-factor session không thêm bảo mật, nên D-27 và D-28 vô dụng nếu không có nhau.

### D-29 — OpenAPI: spec-first không thương lượng; monolith cho tới ~2000 dòng; schema cross-module eager trong Phase 0 *(resolve §16.E-29)*

Spec OpenAPI là contract cho cả Go server stub và TS client type. Để handler drift với spec defeat codegen story. Drift detection ([D-9]) bắt triệu chứng; spec-first như policy tránh nguyên nhân.

**Decision:**

- **Process — spec-first.** Mỗi endpoint mới PHẢI thêm vào `shared/openapi.yaml` trước khi handler tồn tại. CI fail mọi PR nơi `make openapi && git diff --exit-code` tìm thấy change.
- **Layout file — monolith cho tới ~2000 dòng.** Sau đó split per-module qua `$ref` OpenAPI thành `shared/openapi/{module}.yaml` với root include chúng. Đừng pre-split ở 400 dòng.
- **Inventory eager-spec** (phải land trước Phase 0 close):
  - Schema `Problem` (RFC 7807, với extension Portal: `required_acr`, `enrollment_url`, `return_to`).
  - Schema `Money` (`{ amount: string, currency: string }`) [D-7, D-14].
  - `PaginatedResult<T>` (cursor-based: `{ items: T[], next_cursor: string|null }`).
  - Contract path parameter `TenantContext` [D-23].
  - Schema `ContinuingItem` cho aggregator `/api/v1/continue` [D-20].
  - Component response 4xx/5xx standard refs.
- **Endpoint per-module** land với `MountHTTP` của mỗi module (endpoint movie khi movie ship, endpoint bank khi bank ship). Endpoint aggregator + schema cross-module land trong Phase 0.

### D-31 — API versioning: URL versioning `/api/v{N}`; additive trong major; sunset RFC 9745 cho v2 *(resolve §16.E-31)*

URL versioning đã implicit trong `/api/v1/...` xuyên codebase. Header-based (`X-API-Version`, `Accept: vnd.portal.v1+json`) sạch URL-side nhưng invisible debug. Date-versioning (Stripe-style) heavy cho self-host product. URL versioning align với code hiện tại và là answer đơn giản nhất.

**Decision:**

- **URL versioning.** Mỗi route API sống dưới `/api/v{N}/`. Hiện tại `/api/v1/`.
- **Trong một major — chỉ additive:**

  | Free | Breaking |
  |---|---|
  | Endpoint mới | Remove/rename field hoặc endpoint |
  | Field request optional mới | Thay đổi type hoặc semantic field |
  | Field response mới | Tighten validation (field required mới, max length ngắn hơn) |
  | Enum value mới (client PHẢI accept unknown) | Remove enum value |

- **Major mới chỉ khi forced.** Process cho v2:
  1. Issue RFC mô tả breaking change + alternatives considered.
  2. Deprecate endpoint v1 với header `Deprecation: true` + `Sunset: <date>` (RFC 9745) ít nhất **6 tháng** trước remove.
  3. v1 và v2 coexist trong window sunset.
  4. Doc migration trong `docs/api/migrating-v1-to-v2.md`.
- **Cổng CI:** check drift OpenAPI ([D-9]) so spec với `main` và flag diff shape-breaking (path removed, field removed, type change). Description PR phải tường minh waive flag với lý do.

Self-hoster pin frontend với version API biết, nên kể cả instance hosted move sang v2, frontend self-hosted không vỡ.

### D-32 — Boundary state: TanStack cho server state, Zustand cho UI state, RHF cho form *(resolve §16.F-32)*

Footgun là stuff data server-derived vào Zustand "for convenience" → sync manual → bug race condition và data stale. Hoặc build "Zustand store derived" duplicate cache TanStack.

**Decision:**

| Category state | Owner | Ví dụ |
|---|---|---|
| **Server state** | TanStack Query | movie list, user profile, transaction, session user hiện tại |
| **UI state (persistent)** | Zustand + middleware `persist` | theme, sidebar collapsed, layout density |
| **UI state (ephemeral)** | Zustand (transient store) | toast active, command palette open, modal hiện tại |
| **Form state** | React Hook Form | draft của form bất kỳ trước submit |
| **Filter/pagination chia sẻ được** | URL query param (read bởi TanStack) | `?page=2&sort=date&genre=action` |

**Rule cứng:** không có Zustand store giữ data fetched từ API. Nếu thấy mình write `setMovies(await fetch(...))`, bạn đã rẽ sai — dùng `useQuery` của TanStack.

Document trong `frontend/CLAUDE.md` (tạo trong Phase 0) với ví dụ anti-pattern worked nên contributor không lặp mistake.

### D-33 — Rendering: RSC-first; island client cho interactivity; player/reader chủ yếu client *(resolve §16.F-33)*

Next.js 15 App Router là RSC-first by design. Reflexive `'use client'` khắp nơi forfeit SEO + bundle savings + UX streaming.

**Decision** — surface-by-surface:

| Surface | Mode | Vì sao |
|---|---|---|
| **Catalogue** movie / music / story / comic | Server components | SEO; HTML streaming; personalisation qua `cookies()` |
| **Page Detail** (movie, track riêng) | Server shell + island interactivity client | Metadata SEO-relevant; player phải client |
| **Player / reader** | Chủ yếu client | Stateful, post-auth, SEO irrelevant |
| **Account settings, bank** | Server shell + island client | Interactive nhưng private; ergonomic fetch server-side |
| **Newsfeed** (Phase 7) | Client primary; SSR page đầu | Interactive cao; update realtime |

**Rule thực tế:**

- Default sang server component. Opt vào `'use client'` chỉ khi thật sự cần.
- Server component fetch qua Portal API trên Docker network — latency cùng region ok cho page SEO-relevant.
- Page catalogue public dùng `next.revalidate` (ISR); data per-user dùng `cache: 'no-store'`.
- Cây quyết định document trong `frontend/CLAUDE.md` cạnh [D-32].

### D-34 — Handoff auth RSC: forwarding cookie qua `cookies()`; mandate domain same-site; refresh qua redirect-and-return *(resolve §16.F-34)*

Ba sub-problem:

1. **Forwarding cookie** — API `cookies()` của Next.js cho RSC access request cookie store; fetch đi ra cần inject tường minh.
2. **SameSite=Strict** — same-site là registrable-domain (eTLD+1). Same-site nghĩa `portal.example.com` + `api.portal.example.com` works; `portal.com` + `api.portal-app.com` thì không.
3. **Refresh token trong RSC** — gì xảy ra khi access token expire giữa render?

**Decision:**

- **Scheme cookie không đổi.** `portal_access` HttpOnly Secure SameSite=Strict Path=/; `portal_refresh` same nhưng Path=/auth.
- **Mandate domain same-site.** Next.js và Portal API PHẢI chia sẻ registrable domain (vd `portal.example.com` + `api.portal.example.com`). Deployment single-domain dùng một Traefik host với routing theo path (`/api/*` → Go, `/*` → Next). Document trong `docs/operations/deployment.md`.
- **API client server-only.** `frontend/src/lib/api-server.ts` với directive `import "server-only"` wrap `fetch` đọc `cookies()` và inject header `Cookie:` trên mỗi request đi ra.
- **Strategy refresh — redirect on 401.** RSC fetch API; 401 → throw `redirect()` Next.js sang `/auth/refresh-and-return?return_to=<path>`. Route đó chạy server-side, call `/auth/refresh` (cookie refresh `Path=/auth` làm nó sent), nhận cookie access mới, redirect back. User thấy một flash navigation.
- **Optimisation tương lai** (Phase 3 frontend): middleware Next.js refresh proactive khi cookie access < 1 phút trước expiry. Tránh round trip 401-redirect trong common case. Không bắt buộc cho v1.
- **CSRF.** Server action Next.js origin-check bởi framework. Combine với SameSite=Strict, surface threat đóng.

Land trong Phase 0 (API client server-only + route refresh-and-return).

### D-35 — Feed "For You": pipeline ba-lớp hand-tuned; "Following" chronological là default; transparency DSA-aligned *(resolve §16.G-35)*

Personalisation ML-driven là project research; weight signal hand-tuned ship sớm hơn nhiều với behaviour transparent. Default sang chronological "Following" + làm "For You" opt-in sidestep hầu hết risk DSA bằng cách cho user choice meaningful.

**Decision — pipeline ranking ba-lớp:**

1. **Candidate generation** — emit ~1000 candidate per request:
   - Post từ user đã follow (§9.12)
   - Post từ community đã join (§9.4)
   - Post với hashtag đã follow (§9.17)
   - Post được react bởi friend (§9.3)
   - Trending trong region/community của viewer
   - "Editor's pick" admin optional
2. **Ranking** — score per candidate hand-tuned. Weight v1:

   ```
   score = 0.40·recency_decay(age, half_life=12h)
         + 0.30·author_affinity(viewer, author)
         + 0.15·engagement_z_score(post)
         + 0.15·topic_calibration(viewer_history, post_topics)
         - 2.00·negative_signal(muted_words, blocks, "not interested")
   ```
3. **Diversity** — walk list sorted; cap 2 item liên tiếp cùng author / community / hashtag; reshuffle để enforce.

**Transparency (DSA Art. 27-aligned):**

- **Popover per-item "Why am I seeing this?"** — top 3 signal contributing với weight normalised.
- **UI `/settings/feed`** — toggle/weight category signal; toggle "reverse chronological only" disable ranking hoàn toàn.
- **Tab default là "Following"** (chronological). "For You" opt-in. User chủ động chọn ranking algorithmic — consent meaningful.

**Hạ tầng:**

- Ranking chạy lúc request; feed hot cache trong Dragonfly 60s.
- Không model ML v1; mọi signal từ table hiện có (`social.reactions`, `social.follows`, `social.community_memberships`).
- Phase 13+: pre-computation candidate-set offline qua cron Asynq; embedding per-user.

Land trong Phase 10.

### D-36 — Live streaming: RTMP ingest + LL-HLS qua `mediamtx`; replay là auto-VOD; cap per-tenant *(resolve §16.G-36)*

RTMP universal (mọi tool streaming hỗ trợ). LL-HLS reuse pipeline VOD + CDN edge hiện có. SRT và WebRTC ingest chất lượng cao hơn nhưng user base nhỏ hơn; defer cho đến khi demand surface.

**Decision:**

- **Ingest:** RTMP. Container `mediamtx` mới (gateway open-source Go-native) trong `docker-compose.yml`; convert RTMP → LL-HLS on the fly; write segment sang MinIO.
- **Distribution:** LL-HLS qua media pipeline hiện có. Spec Apple Low-Latency HLS; CDN-friendly qua HTTP/2.
- **Target latency:** env `LIVE_LATENCY_SECONDS=4` (§9.25).
- **Replay:** on `media:live_ended`, FFmpeg concatenate segment tích luỹ → asset HLS VOD standard → emit `media:asset_ready`. Live trở thành video bình thường trên profile streamer.
- **Chat live:** WebSocket `platform/realtime/` [D-3]; channel per-stream `live:{stream_id}` trong Dragonfly pub/sub. Controls mod stream-specific: emote-only mode, follower-only mode, slow-mode (một msg per N giây per user).
- **Capacity** (mở rộng [D-13]):
  - Mỗi encode 1080p LL-HLS ≈ 2 vCPU sustained.
  - Env `MAX_CONCURRENT_LIVE_STREAMS_PER_TENANT=3`.
  - Step-up auth ([D-27]) gate "Go Live" nếu streamer chưa stream trước.
- **Defer:** SRT ingest, WebRTC ingest (sub-second), split origin/edge. `mediamtx` hỗ trợ cả ba; switching là config-only khi scale demand.

**Event mới:** `media:live_started`, `media:live_ended` consume bởi social module để flip state live-indicator trên profile streamer.

Land trong Phase 10.

### D-37 — Music reels: chỉ audio user-uploaded; chain attribution viral-sound *(resolve §16.G-37)*

Deal commercial-library (Sony/Universal/Warner) là non-starter cho self-host product — cost annual nhiều triệu, complexity clearance per-region, và hầu hết operator không cần.

**Decision:**

- **Source audio:** chỉ user-uploaded. Khi user A upload reel, audio của reel trở thành **"sound"** reusable trong `social.sounds`. Reel của user khác có thể attribute và reuse sound đó.
- **Chip attribution** trên mỗi reel derivative: "Sound bởi @userA · 12.3k reel dùng sound này" — clickable, dẫn sang page sound-detail với mọi reel dùng nó.
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
- **Reel derivative:** `social.reels.sound_id` set; `social.sounds.use_count` increment qua task Asynq (tránh contention hot-row).
- **Workflow DMCA take-down** (Phase 11+): rights-holder submit notice → operator review → audio asset removed; cascading: reel derivative giữ video, mất audio với marker "sound removed". Audit-logged.
- **Shape plug-in cho v2:** interface `MusicLibrary` trong `social/music_library/` cho operator integrate catalogue licensed (vd Epidemic Sound) mà không rewrite logic reel. Defer.

Land trong Phase 10.

### D-38 — Safety classifier: interface image + text pluggable; default NSFWJS + pHash; CSAM match block + quarantine + page *(resolve §16.G-38)*

Hai vấn đề khác biệt với stake rất khác. CSAM mandatory, illegal-to-host, với nghĩa vụ report NCMEC/IWF. NSFW policy-driven, operator/community-configurable. Kiến trúc phải cho operator substitute classifier tự do trong khi giữ workflow deterministic.

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

**Ship v1:**

- `safety/classifier/nsfwjs` — model NSFWJS ONNX-compatible trong worker; chạy locally; MIT.
- `safety/classifier/phash` — matcher perceptual-hash against list hash CSAM operator-supplied. List default empty; operator load từ NCMEC / IWF / equivalent sau legal partnership.
- `safety/classifier/detoxify` — text classifier; PyTorch ONNX-compatible; MIT.

**Có sẵn như plug-in** (Go module riêng; operator opt in):

- `safety/classifier/perspective` — Google Perspective API (text).
- `safety/classifier/aws_rekognition` — AWS Rekognition (image).
- `safety/classifier/hive` — Hive Moderation (image + text).

**Config operator:**

```
IMAGE_CLASSIFIERS=nsfwjs,phash         # comma = parallel; semantic OR trên result
TEXT_CLASSIFIERS=detoxify
NSFW_THRESHOLD=0.7
CSAM_HASH_LIST_PATH=/etc/portal/csam-hashes.txt
SAFETY_REVIEW_WEBHOOK=                 # URL alert optional on CSAM match
```

**Workflow:**

1. Event `media:asset_ready` consume bởi worker `safety`.
2. Chạy mọi image classifier configured parallel (`OR` semantic across result).
3. **CSAM hash match** → block asset (set `assets.status='quarantined'`) + emit task Asynq priority cao `safety:csam_detected` + insert row `safety.csam_incidents` + page operator qua webhook configured. **Quarantine, không bao giờ delete** — bảo tồn evidence legal.
4. **NSFW score > threshold** → set `assets.nsfw_flag = true`; policy NSFW community enforce trên visibility post (§9.31).
5. **Text classifier** chạy on post-create; toxicity > 0.85 → flag sang mod queue (không auto-delete; mod decide).
6. Mọi output classifier lưu trên `safety.classifications` cho audit + tuning threshold sau.

**Invariant critical:** output classifier là **advisory** trừ CSAM. NSFW + toxicity flag content cho mod review human; auto-block reserve chỉ cho CSAM hash match.

Land trong Phase 12.

### D-39 — Group call: LiveKit SFU; P2P cho 1:1; signalling qua Portal API cho RBAC *(resolve §16.G-39)*

Mesh P2P fail qua ~4 peer (n² connection). Group call cần SFU. Cho Go-monolith self-host product, LiveKit là winner rõ — Go-native, single binary, SDK polished, Apache 2.0, backing commercial active.

**Decision:**

- **Call 1:1:** WebRTC P2P; không SFU hop. Latency thấp hơn, zero server cost. Fall back sang TURN qua `coturn` nếu NAT block direct connection.
- **Group call (≥3 peer):** LiveKit SFU.
- **Audio room / Spaces (§9.29):** LiveKit audio-only room (hỗ trợ 100+ listener).
- **Service:** container `livekit` mới trong `docker-compose.yml`, gate sau flag opt-in `--profile calls` (giống `--profile observability` trong [D-8]). Self-hoster không call không trả resource cost.
- **Signalling:** qua Portal API. Portal mediate phát token LiveKit — mỗi request join hit Portal trước, mà:
  1. Verify RBAC (caller có thể join room này).
  2. Verify privacy controls (caller có thể DM/call host theo §9.21).
  3. Phát token LiveKit scope vào room.
  Khi token đã phát, peer connect LiveKit directly. **Không có endpoint LiveKit raw expose public.**
- **Recording** (cho audio room + opt-in cho group call): feature egress LiveKit write recording sang MinIO; FFmpeg post-process sang AAC; emit `media:asset_ready` nên recording surface như asset bình thường.
- **STUN/TURN:** LiveKit ship với TURN server built-in; cho production sau NAT, configure service `coturn` dedicated. Document trong `docs/operations/calls.md`.

**Env vars mới:**

```
LIVEKIT_API_KEY=
LIVEKIT_API_SECRET=
LIVEKIT_URL=ws://livekit:7880
LIVEKIT_RECORDING_BUCKET=portal-recordings
TURN_SERVER=                             # coturn external optional
```

Land trong Phase 12.

### D-40 — Payout creator: interface `Provider` pluggable; `manual` default v1; Stripe Connect integration thực đầu *(resolve §16.G-40)*

Operator khác có nhu cầu radically khác (US-only, EU-only, non-profit, crypto-curious). Vendor lock-in không tránh được cho payout fiat compliant, nhưng pattern plug-in nghĩa mỗi operator có thể chọn provider riêng mà không fork codebase.

**Decision — interface `Provider` trong `bank/payout/`:**

```go
package bankpayout

type Provider interface {
  EnrollCreator(ctx context.Context, user UserID, kyc KYCData) (CreatorAccountID, error)
  Payout(ctx context.Context, from CreatorAccountID, amount Money) (PayoutID, error)
  Status(ctx context.Context, id PayoutID) (PayoutStatus, error)
  TaxFormsFor(ctx context.Context, creator CreatorAccountID, year int) ([]TaxForm, error)
}
```

**Ship theo thứ tự priority:**

1. **`bank/payout/manual`** (default v1; ship Phase 11) — operator manual wire payout và mark complete trong UI admin bank module. KYC + tax form là vấn đề operator. Luôn works; zero dependency vendor.
2. **`bank/payout/stripe`** — Stripe Connect Express account. Xử lý KYC + filing 1099-K cho creator US. Phí ~3% + flat. Ship khi operator đầu cần.
3. **`bank/payout/wise`** — cross-border rẻ; không có primitive platform-payment nên reconciliation manual. Per-operator request.
4. **`bank/payout/usdc`** — USDC trên Stellar hoặc Polygon cho phí thấp. Compliance unclear; chỉ opt-in; document "experimental" trong doc self-host.

**Workflow:**

1. Creator mở UI "Withdraw" → nhập amount + provider destination.
2. **Step-up auth** ([D-27]) — `RequireACR("acr:portal:recent_mfa")` vì payout không reversible.
3. Nếu creator chưa enroll với provider configured, chạy `EnrollCreator` (flow KYC provider-specific, thường redirect hosted).
4. Submit call `Payout`. Module bank ghi ledger entry cân: `creator.balance −X, payout.outstanding +X` [D-15].
5. Cron Asynq poll `Status` cho tới complete hoặc fail. On success: `payout.outstanding −X, payout.completed +X`. On failure: reverse sang creator balance.
6. Tax form surface annually trong settings creator qua `TaxFormsFor`.

**Config operator:**

```
PAYOUT_PROVIDER=manual                # manual | stripe | wise | usdc
STRIPE_CONNECT_CLIENT_ID=
STRIPE_CONNECT_SECRET=
PAYOUT_MIN_THRESHOLD=2500             # cents — không payout micro-amount
PAYOUT_HOLD_DAYS=7                    # delay sau balance change để allow chargeback/refund
```

**Constraint:** `manual` ship trong Phase 11 với creator economy. Stripe Connect land per operator paying đầu. Interface đảm bảo swap mechanical — không schema migration khi thêm provider mới.

Land trong Phase 11.

---

## Cách đọc tài liệu này

- Status legend (✓ / ○ / △) trên mỗi section phản ánh **thực tế code**, không aspiration.
- Roadmap là **tuần tự** — tiêu chí exit của mỗi phase guard phase tiếp theo.
- Open question là **gate** — item marked nên được trả lời trước khi phase phụ thuộc mở; nếu không, phase ship trên giả định sẽ cần rework.
- **Identifier stable vs section number** — ID open-question (`16.A-1`, `16.B-8`, …, `16.G-40`) và ID decision (`D-1` … `D-40`) là **string stable**, không tham chiếu section number hiện tại. Section top-level chứa chúng (§19 Open questions, §20 Decisions log) có thể renumber khi section mới được insert, nhưng ID không bao giờ thay đổi. Luôn cite theo ID, không theo section number.
- Question resolved gạch ngang với pointer `→ Resolved [D-N]`. Không bao giờ renumber.
- Decision cũng stable; lật ngược một cái append revision (`D-N.r1`), không bao giờ edit in place. Trail audit quan trọng khi rationale ngừng apply.
