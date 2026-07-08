# NOTES — design-sync: portal-frontend → "Portal UI — Olympus v1"

Syncs the Olympus v1 presentation layer at `frontend/src/templates/v1/` (React
port of the Crumina "Olympus" social theme) — 21 components across
ui/headers/menu/master/views/partials. **This is a Next.js app, not a component
library**, so the sync runs in **synth-entry** mode with several shims.

## Build setup (why the config is unusual)

- **Not an installed package.** `portal-frontend` is the app itself, so PKG_DIR
  (`frontend/node_modules/portal-frontend`) must be a **real stub dir with NO
  self-referential `node_modules`** — a `portal-frontend → ..` symlink makes
  ts-morph's descendant scan (`exportedNames`) recurse into
  `portal-frontend/node_modules/portal-frontend/…` forever → `ENAMETOOLONG`.
  Set it up (recreate after any `npm install`; gitignored):
  ```sh
  cd frontend/node_modules && rm -f portal-frontend && mkdir portal-frontend && cd portal-frontend
  printf '{"name":"portal-frontend","version":"0.1.0","private":true}\n' > package.json
  ln -sfn ../../src src; ln -sfn ../../tsconfig.ds.json tsconfig.ds.json; ln -sfn ../../.ds-shims .ds-shims
  cp ../../.ds-compiled.css .ds-compiled.css   # REAL copy — cssEntry is bound to PKG_DIR; a symlink escaping it is rejected
  ```
- **No lockfile, no pnpm.** Install with `npm install --legacy-peer-deps` in
  `frontend/` (next@15.0.0's peer range names a React 19 RC; the repo pins stable
  react@19.0.0, which npm rejects without the flag). This only feeds esbuild
  module resolution.
- **Tailwind v4 must be compiled** (cfg.buildCmd). `cd frontend && node .ds-compile-css.mjs`
  produces `frontend/.ds-compiled.css` (cfg.cssEntry) — Tailwind utilities used
  across templates/v1 + inlined `theme.css` `--tpl-*` tokens. Uses
  `@import "tailwindcss" source(none)` + explicit `@source` (auto-detection would
  follow the stub-package symlinks and loop). Regenerable, gitignored; re-run
  before each converter build **and `cp` it into the stub package dir** (above).
- **next/link shim** — `frontend/.ds-shims/next-link.tsx` renders a plain `<a>`;
  wired via `frontend/tsconfig.ds.json` paths (cfg.tsconfig). Only next/link is used.
- **process shim** — several components read `process.env.NEXT_PUBLIC_API_BASE_URL`
  at module top level; in the browser IIFE `process` is undefined → the whole
  bundle throws. `frontend/.ds-shims/ds-provider.tsx` defines `globalThis.process`
  before any component module evaluates (the converter emits extraEntries ahead of
  the main entry). **Keep that line.**
- **Sprite provider** — cfg.provider = `DSProvider` (same ds-provider.tsx) mounts
  `<SvgSprite/>` so `Icon` and everything composing it renders.
- **Render browser** — no playwright chromium installed; validate/capture use the
  system Chrome via `export DS_CHROMIUM_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"`.
  playwright JS is in `.ds-sync` (installed with `PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1`).

### Full build command
```sh
cd frontend && node .ds-compile-css.mjs && cd ..
node .ds-sync/package-build.mjs --config .design-sync/config.json --node-modules frontend/node_modules --out ./ds-bundle
export DS_CHROMIUM_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
node .ds-sync/package-validate.mjs ./ds-bundle
```

## Re-sync risks

- **The 4 popups are now implemented** (`AddBook`, `ChatResponsive`,
  `ChoseFromMyPhoto`, `UpdateHeaderPhoto`) on a shared `Modal` shell
  (`components/popup/Modal.tsx`, controlled `open`/`onClose`). They render as fixed
  overlays — the previews contain them with a `transform` + `position:relative`
  wrapper (no `cfg.overrides` needed). `Modal` and its form helpers (`Field`,
  `BtnPrimary`, `BtnSecondary`) are excluded as cards via `componentSrcMap` null
  (primitives, not standalone) but ship in the bundle.
- `SessionKeeper` (auth-refresh infra, no UI) and `SvgSprite` (invisible sprite
  injector → used as the provider) are excluded via `componentSrcMap` null.

## Preview hacks tied to upstream behavior (re-verify these on re-sync)

- **GoToTop** — `fixed` + `opacity-0` until `scrollY>300`, no props. Preview forces
  it visible with a scoped `<style>` (`[aria-label="Go to top"]` → position:absolute,
  opacity:1) inside a `transform` wrapper. If the source gains an `initiallyVisible`
  prop, drop the hack.
- **HelloPreloader** — `fixed inset-0` overlay that **self-dismisses after ~300ms**
  (returns null), no props. Preview keeps it on screen via a 120ms **keyed-remount
  loop** (useState/setInterval bumping `key`) + transform containment. Fragile: if
  the dismiss timer changes, adjust the interval. A harness timer-freeze would
  obsolete this.
- **Dropdowns** (FriendRequestsMenu/MessagesMenu/NotificationsMenu) — require
  `open` + `onToggle` props; preview passes `open onToggle={()=>{}}` and frames each
  on a dark `var(--tpl-header)` relative container (~380px, flex-end) so the white
  panel pops. No cfg.overrides.
- **Sidebars + MasterBase** — real layout is `position:fixed`, `xl:`(1280px)-gated
  (`hidden xl:flex`), `calc(100vh-…)`-sized; capture viewport is 900px. Previews
  inject `<style>` (display:flex, position:static/absolute, explicit height) +
  `data-template="v1"` to seed `--tpl-*` tokens. MasterBase is capped ~850px to fit
  the screenshot. Optional clean-up: `cfg.overrides.MasterBase
  {cardMode:"single", viewport:"1440x900"}` would render the natural 3-column shell
  full-size and drop the forced overrides — not applied (current render grades good).

## Known render warns (recorded — not new next time)

- `[TOKENS_MISSING]` ~36 `--media-*` / `--cue-*` vars — Vidstack player CSS vars,
  injected at runtime by `@vidstack/react`; only relevant to a player state no card
  renders. Non-blocking, expected.

## Component groups (derived from src paths; `ui/` is a generic dir → `general`)

- **general**: Avatar, Icon, FriendRequestsMenu, MessagesMenu, NotificationsMenu
- **headers**: TopMenu · **menu**: SidebarLeft, SidebarCenter, SidebarRight
- **master**: MasterBase, MasterPublic
- **views**: HomeView (home), LoginView/RegisterView/AuthForm/AuthLanding (auth),
  ComicIndexView (comic), NovelDetailView (novel), UploadStudio (upload)
- **partials**: GoToTop, HelloPreloader
- **popup**: AddBook, ChatResponsive, ChoseFromMyPhoto, UpdateHeaderPhoto
