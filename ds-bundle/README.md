## Building with Portal UI (Olympus v1)

Portal UI is the React port of the Crumina **"Olympus"** social theme — a
presentational kit: header bar, sidebars, auth/feed views, avatars, sprite icons.
Components are styled with **Tailwind v4 utilities + Olympus `--tpl-*` design
tokens** and render standalone (a few fire a best-effort `NEXT_PUBLIC_API_BASE_URL`
fetch that safely no-ops with no backend).

### Setup — mount the sprite once

Icons render from an SVG sprite: `<Icon name="heart-icon"/>` emits
`<use href="#olymp-heart-icon">`. Render **`<SvgSprite/>` once** at your app root so
every `<Icon>` resolves — without it, icons are empty boxes. Theme tokens live on
`:root` in the bound `styles.css`; no provider is needed for colors.

```tsx
import { SvgSprite, MasterBase, HomeView } from "portal-frontend";

export default function App() {
  return (
    <>
      <SvgSprite />              {/* once — powers every <Icon/> */}
      <MasterBase>
        <HomeView />
      </MasterBase>
    </>
  );
}
```

### Styling idiom — Tailwind utilities + `--tpl-*` tokens

Style your own layout glue with Tailwind classes; reach for the Olympus tokens
(not raw hexes) so it stays on-theme — as `var(--tpl-*)` or Tailwind arbitrary
values (`bg-[var(--tpl-surface)]`, `text-[var(--tpl-heading)]`).

| Group | Tokens |
|---|---|
| Brand | `--tpl-accent` (#ff5e3a) · `--tpl-accent-2` · `--tpl-blue` · `--tpl-blue-2` |
| Surfaces | `--tpl-bg` · `--tpl-surface` · `--tpl-surface-2` · `--tpl-header` · `--tpl-header-2` |
| Text | `--tpl-text` · `--tpl-heading` · `--tpl-muted` |
| Lines / metrics | `--tpl-border` · `--tpl-header-h` · `--tpl-rail-w` · `--tpl-sidebar-w` · `--tpl-rightbar-w` |
| Status dots | `--tpl-status-online` · `--tpl-status-work` · `--tpl-status-away` · `--tpl-status-offline` |

```tsx
import { Avatar, Icon } from "portal-frontend";

<div className="flex items-center gap-3 rounded-lg bg-[var(--tpl-surface)] p-3 text-[var(--tpl-text)]">
  <Avatar name="Ada Lovelace" size={40} />
  <span className="font-semibold text-[var(--tpl-heading)]">Ada Lovelace</span>
  <Icon name="heart-icon" size={18} style={{ color: "var(--tpl-accent)" }} />
</div>
```

### Where the truth lives

Read the bound **`styles.css`** (and its `_ds_bundle.css` import) for the full
token + utility surface, and each component's **`<Name>.prompt.md`** + **`<Name>.d.ts`**
for its API and examples. `<Icon name>` values are Olympus sprite ids —
`home-icon`, `newsfeed-icon`, `comments-post-icon`, `settings-icon`,
`magnifying-glass-icon`, `camera-icon`, `calendar-icon`, `photos-icon`, etc.

### Composition notes

- **`MasterBase`** = app shell (header + left nav + content + right rail);
  **`MasterPublic`** = auth/public shell — both take `children`.
- **`*View`** components are self-contained screens — compose them as the single
  child of a master shell, don't rebuild their internals:
  - *social / auth*: `HomeView`, `LoginView`, `RegisterView`, `AuthLanding`
    (`AuthForm` takes `defaultTab?: "login" | "register"`), `UploadStudio`
  - *library*: `ComicIndexView`, `ComicDetailView`, `ComicReaderView`,
    `NovelDetailView`, `MediaIndexView`, `MediaDetailView`, `ContinueRail`
  - *life OS*: `DashboardView`, `TransactionsView`, `AccountsView`, `BudgetsView`
    (bank) · `CalendarView` · `WeatherView` · `PeopleIndexView`, `PersonDetailView`
- **Screens fetch their own data** via TanStack Query (`D-32`) — most take only an
  `id`, not the record. Mount them under a `QueryClientProvider`; with no API
  reachable they render their real empty / "not found" state rather than failing.
- **Money is integer minor units on the wire** (`D-41`). Render it with
  `MoneyDisplay` (`amount`, optional `currency`, `signed` for +/- ledger colour)
  and take it with `MoneyInput` (`value`, `onChange(minor)`) — never format VND
  by hand.
- Dropdowns (`FriendRequestsMenu`, `MessagesMenu`, `NotificationsMenu`) take
  `open` + `onToggle`; frame them against a `--tpl-header`-dark bar.

# PortalUI (portal-frontend@0.1.0)

This design system is the published portal-frontend React library, bundled as a single
browser global. All 79 components are the real upstream code.

## Where things are

- `_ds_bundle.js` — the whole-DS bundle at the project root; loads every component to `window.PortalUI`. First line is a `/* @ds-bundle: … */` metadata header.
- `styles.css` — the single stylesheet entry: it `@import`s the tokens, fonts, and component styles (`_ds_bundle.css`). Link this one file.
- `components/<group>/<Name>/<Name>.prompt.md` (example JSX + variants), `<Name>.d.ts` (types), `<Name>.html` (variant grid).
- `tokens/*.css` — CSS custom properties, names verbatim from upstream.
- `fonts/` — `@font-face` files + `fonts.css` (when the package ships fonts).

For a specific component, `read_file("components/<group>/<Name>/<Name>.prompt.md")`.

## Loading

Add these two lines to your page once (React must be on the page first):

```html
<link rel="stylesheet" href="styles.css">
<script src="_ds_bundle.js"></script>
```

Components are then available at `window.PortalUI.*`. Mount into a dedicated child node (e.g. `<div id="ds-root">`), not the host page's own React root, so the two trees don't collide:

```jsx
const { AccountsView } = window.PortalUI;
ReactDOM.createRoot(document.getElementById('ds-root')).render(<AccountsView />);
```

Wrap the tree in the provider — most components read theme/i18n from context:

```jsx
<DSProvider>{children}</DSProvider>
```

## Tokens

292 CSS custom properties from portal-frontend. Names are
preserved verbatim from upstream. They are declared inside `_ds_bundle.css` (this DS ships one compiled stylesheet rather than separate token files).

- **color** (72): `--cue-color`, `--cue-bg-color`, `--cue-text-align`, …
- **spacing** (14): `--overlay-padding`, `--cue-padding-x`, `--cue-padding-y`, …
- **typography** (24): `--cue-default-font-size`, `--cue-font-size`, `--cue-line-height`, …
- **radius** (6): `--root-border-radius`, `--item-border-radius`, `--radius-md`, …
- **shadow** (7): `--tw-shadow`, `--tw-ring-shadow`, `--tw-ring-offset-shadow`, …
- **other** (169): `--size`, `--cue-width`, `--cue-top`, …

## Components

### bank
- `AccountsView`
- `BudgetsView`
- `DashboardView`
- `TransactionsView`

### widget
- `ActivityFeed`
- `BirthdayCard`
- `CalendarWidget`
- `ContinueWidget`
- `FinanceWidget`
- `FriendSuggestions`
- `PagesWidget`
- `PersonalInfoWidget`
- `WeatherWidget`
- `WidgetCard`

### popup
- `AddBook`
- `ChatResponsive`
- `ChoseFromMyPhoto`
- `UpdateHeaderPhoto`

### music
- `AudioPlayer`
- `PlaylistWidget`
- `TrackItem`

### auth
- `AuthForm`
- `AuthLanding`
- `LoginView`
- `RegisterView`

### general
- `Avatar`
- `Composer`
- `FriendRequestsMenu`
- `Icon`
- `MessagesMenu`
- `MoneyDisplay`
- `MoneyInput`
- `NotificationsMenu`
- `Post`

### social
- `BadgeCard`
- `EventItem`
- `FriendCard`
- `FriendRequestItem`

### blog
- `BlogCard`

### calendar
- `CalendarView`

### reader
- `ChapterMenu`
- `PagedReader`
- `ReaderChrome`
- `ReaderHelp`
- `ReaderSettings`
- `StripReader`

### comic
- `ComicDetailView`
- `ComicIndexView`
- `ComicReaderView`

### comment
- `CommentForm`
- `CommentItem`
- `CommentThread`

### library
- `ContinueRail`

### profile
- `ControlBlockButtons`
- `ProfileHeader`

### journal
- `EntryCard`

### form
- `FormField`
- `SelectField`
- `TagSelect`
- `ToggleRow`

### partials
- `GoToTop`
- `HelloPreloader`

### home
- `HomeView`

### master
- `MasterBase`
- `MasterPublic`

### media
- `MediaDetailView`
- `MediaIndexView`

### novel
- `NovelDetailView`

### people
- `PeopleIndexView`
- `PersonDetailView`

### post
- `PostControlButtons`
- `ReactionBar`

### menu
- `SidebarCenter`
- `SidebarLeft`
- `SidebarRight`

### stream
- `StreamItemCard`

### headers
- `TopMenu`

### upload
- `UploadStudio`

### weather
- `WeatherView`
