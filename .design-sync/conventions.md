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
- **`*View`** components (`HomeView`, `LoginView`, `RegisterView`, `AuthLanding`,
  `ComicIndexView`, `NovelDetailView`, `UploadStudio`) are self-contained screens;
  `AuthForm` takes `defaultTab?: "login" | "register"`.
- Dropdowns (`FriendRequestsMenu`, `MessagesMenu`, `NotificationsMenu`) take
  `open` + `onToggle`; frame them against a `--tpl-header`-dark bar.
