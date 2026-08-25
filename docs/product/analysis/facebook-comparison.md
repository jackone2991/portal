# Portal vs. Facebook — Feature Comparison

**Last verified:** 2026-07-06 — reflects the closed v1 demo loop (local password auth + upload→HLS→playback); **This document is ARCHIVED.** It predates [ADR-08](../../adr/08-life-os-pivot.md)'s life-OS pivot, and several of its "missing" claims (image pipeline, notifications backend, birthdays) are now false. `vision.md` + the SPECs are the live yardstick.

How Portal's social surface compares to Facebook, feature-area by feature-area.
Companion to [missing-features.md](missing-features.md) (which compares against
Portal's own spec); this one uses **Facebook as the yardstick** because the UI is
a port of the Facebook-like Olympus template.

**Status:** ✅ working · ◐ UI only (screen exists, no backend) · ○ not started · ⛔ out of Portal's scope.

> **One-line verdict:** Portal today is a **Facebook-shaped UI shell with one real
> capability — the video pipeline (upload → transcode → HLS playback)**. Basic auth
> works. Almost every *social* backend (posts, friends, messaging, notifications,
> groups, search) is **UI-only sample data**. Coverage of Facebook's social product
> is low; the video/"Watch" slice is the standout that actually works.

---

## Coverage at a glance

| Facebook area | Portal | Note |
|---|---|---|
| Sign-up / login | ◐ | email+password only; no phone/Google/Apple, 2FA, recovery |
| Profile & timeline | ○ | no profile page; initials avatar only |
| Newsfeed & posting | ◐ | composer posts to local state; no posts API/ranking |
| Reactions / comments / shares | ○ | counters are static |
| Stories (24h) | ○ | — |
| Reels (short video) | ○ | — |
| Video / Watch | ✅ | **upload → HLS → playback works** (single rendition) |
| Photos & albums | ○ | pipeline handles video only |
| Friends / social graph | ◐ | requests/suggestions/groups are sample data |
| Follow (asymmetric) | ○ | — |
| Messenger (chat/calls) | ◐ | chat bar + message dropdown decorative |
| Groups / communities | ○ | menu item only |
| Pages | ○ | "Pages You May Like" is sample |
| Events / birthdays | ◐ | static widgets |
| Notifications | ◐ | bell/activity dropdowns are hard-coded |
| Search | ◐ | input exists, no backend/results |
| Marketplace | ⛔ | deferred (ADR-01) |
| Privacy & settings | ○ | none |
| Moderation & safety | ○ | none |
| Monetization / ads | ⛔ | deferred |
| Mobile apps / i18n / dark mode | ○ | web only, English only, light theme only |

---

## 1. Account, login & identity
- ◐ Email + password sign-up/login, remember-me, brute-force lockout, sessions via JWT+refresh.
- ○ **Missing vs FB:** phone-number login, **Login with Google/Apple**, **two-factor auth**, account recovery, email/phone verification, trusted-device management, "log out of all sessions" UI, name/username, verified badge.

## 2. Profile & timeline
- ○ No profile at all beyond `display_name` + initials avatar.
- ○ **Missing vs FB:** profile page (timeline), **avatar + cover photo upload**, bio/intro, about (work/education/places/contact), life events, featured, "friends"/"photos"/"videos" tabs, activity log, follow/message buttons on profile.

## 3. Newsfeed & posting
- ◐ Composer with Status/Media/Blog tabs + a feed — but posting only prepends to **local React state**, feed is sample data.
- ○ **Missing vs FB:** posts persistence + feed API, **audience selector** (public/friends/only-me/custom), photo/video/link/GIF/poll posts, **feeling/activity**, **check-in/location**, tag friends, background colors, **feed ranking**, hide/snooze/"see first", edit/delete, drafts & scheduling.

## 4. Reactions, comments & shares
- ○ Like count + comment/share counts are static; no interaction persists.
- ○ **Missing vs FB:** **6 reactions** (like/love/haha/wow/sad/angry), **threaded comments** + comment reactions, **share/quote-share** to own feed/group/message, save/bookmark, react to comments, mentions in comments.

## 5. Stories & Reels
- ○ Neither exists.
- ○ **Missing vs FB:** **Stories** (24h ephemeral photo/video, viewers, replies, reactions, highlights), **Reels** (vertical short-form video feed, audio, remix).

## 6. Video / "Watch"  ← Portal's strength
- ✅ Direct upload → worker `ffmpeg` → **VOD HLS** → **Vidstack playback** at `/upload`; ffprobe metadata.
- ○ **Missing vs FB:** multi-bitrate ladder + master playlist, **thumbnails/preview**, a Watch feed/discovery, video reactions/comments, view counts, captions/subtitles, **live streaming**, playlists, download control, monetization.

## 7. Photos & albums
- ○ No image pipeline (schema allows `image` but unused).
- ○ **Missing vs FB:** photo upload, **albums/carousels**, photo tagging, face-ish tag suggestions, EXIF/date, lightbox viewer, cover/profile photo history.

## 8. Friends & social graph
- ◐ Header **friend-requests dropdown**, "Friend Suggestions", right-panel **friend groups** (Close Friends/Family/Uncategorized) — all **sample data**; accept/decline mutate local state only.
- ○ **Missing vs FB:** friendship table + request/accept/decline/unfriend endpoints, **People You May Know** (mutuals), **block**, custom friend lists, mutual-friends view, "friends" privacy scoping.

## 9. Follow (asymmetric)
- ○ None (FB has follow alongside friendship for public figures/creators).
- ○ **Missing vs FB:** follow/unfollow, followers/following counts, "see first", public-follow for pages/creators.

## 10. Messenger (chat & calls)
- ◐ "Olympus Chat" bar + messages dropdown are decorative; a `ChatResponsive` modal stub exists.
- ○ **Missing vs FB:** conversations + messages storage, **1:1 & group chat**, realtime (WS/SSE), media/voice/file attachments, **read receipts + typing + active status**, message reactions, message requests, **voice/video calls**, e2e-encryption option, unsend/edit.

## 11. Groups / communities
- ○ Left-menu "Friend Groups" is nav-only; no group entity.
- ○ **Missing vs FB:** create/join groups, group feed, **roles (admin/moderator)**, membership approval, rules, group events, pinned posts, discussion vs. feed, privacy (public/private/hidden).

## 12. Pages
- ○ "Pages You May Like" + "Fav Pages Feed" are sample.
- ○ **Missing vs FB:** page entity (create/follow/like), page roles, posting as a page, insights/analytics, categories, reviews.

## 13. Events, birthdays & calendar
- ◐ Birthday card, calendar widget, "Calendar and Events"/"Friends Birthdays" menu — **all static**.
- ○ **Missing vs FB:** events (create/RSVP/invite/recurring), event feed, **birthday reminders**, calendar integration.

## 14. Notifications
- ◐ Bell dropdown + "Activity Feed" are hard-coded lists; badges are constants.
- ○ **Missing vs FB:** notification store + `GET /me/notifications`, **realtime delivery** (SSE/WS), **web push + email**, per-type settings, mark-read/all-read persisted, grouping/aggregation. *(Blocked on the Notifications module — see missing-features §5.)*

## 15. Search
- ◐ Header "Search here people or pages…" + "Find Friends" — no backend.
- ○ **Missing vs FB:** **universal search** (people/posts/pages/groups/photos/videos), typeahead, filters, recent searches, ranked results.

## 16. Marketplace / commerce
- ⛔ Deferred ([ADR-01](architecture/01-v1-scope-cut.md), feature.md §11). FB has listings, categories, seller chat, shops.

## 17. Privacy, settings & data rights
- ○ None beyond auth cookies.
- ○ **Missing vs FB:** **audience/visibility controls** per post & profile field, blocking, **activity log**, **download-your-data (GDPR)**, delete account, ad/notification preferences, 2FA & sessions UI, "who can find me".

## 18. Moderation, safety & trust
- ○ None.
- ○ **Missing vs FB:** **report content/user**, block, hide, community standards + enforcement, content warnings, spam/abuse detection, appeal flow, admin moderation dashboard. *(Audit log exists on the auth side.)*

## 19. Monetization
- ⛔ Deferred (feature.md §10). FB has ads, creator payouts, stars, subscriptions, fundraisers.

## 20. Platform, reach & polish
- ○ **Missing vs FB:** native **mobile apps** (web only; responsiveness is partial), **internationalization** (UI is English-only; docs are bilingual), **dark mode** (light theme only — Olympus dark exists as tokens but no toggle), offline/PWA, accessibility pass, real-time presence, and FB's broader surfaces (Dating, Gaming, Jobs, Fundraisers, Memories).

---

## What to build to look most like Facebook, fastest
The cheapest path to a "believable Facebook-lite" from today's shell. Ordering here optimizes for Facebook-likeness; the canonical build order (which puts the Notifications module first because it unblocks password reset) is [missing-features.md — Suggested next order (P1)](missing-features.md#suggested-next-order-p1):

1. **Posts + reactions + comments** — turns the flagship newsfeed real. *(P1)*
2. **Friend graph** — requests/accept/suggestions wired to the existing dropdowns. *(P1)*
3. **Notifications module + realtime** — the bell/activity feed become live. *(P1)*
4. **Messenger** — conversations + WS; the chat bar becomes usable. *(P1/P2)*
5. **Profile page** + avatar/cover upload (reuses the media pipeline). *(P2)*
6. **Search** (people/posts/pages). *(P2)*
7. **Photos/albums** + **Stories** (reuse media). *(P2/P3)*
8. **Groups & Pages & Events**. *(P3)*

Everything above is already **designed in the UI** — the work is backend + wiring,
not new screens. Marketplace, monetization, ads, mobile apps, and Dating/Gaming are
**out of Portal's stated scope** and not recommended for v1.
