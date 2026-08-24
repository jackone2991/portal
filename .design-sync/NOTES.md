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
  wired via `frontend/tsconfig.ds.json` paths (cfg.tsconfig).
- **next/navigation shim** (added 2026-08-24) — `frontend/.ds-shims/next-navigation.tsx`,
  wired through the same paths map. `useRouter`/`usePathname`/`useSearchParams` read
  the App Router context, which only a running Next app mounts; standalone they throw
  `invariant expected app router to be mounted` and blank the whole card. That single
  missing alias took out `TopMenu`, `MasterBase`, `SidebarCenter`, `NotificationsMenu`
  and `ComicIndexView` at once. `usePathname()` returns `"/"` so active-nav logic
  stays on its default branch instead of highlighting an arbitrary entry.
- **QueryClientProvider** (added 2026-08-24) — `DSProvider` now wraps every card in
  one. Server state is TanStack Query's (`D-32`), so any view that loads data throws
  `No QueryClient set` without it. `retry: false` is load-bearing: previews have no
  API origin, so every query fails, and the default exponential backoff would leave
  the card stuck in `isLoading` past the screenshot instead of showing the real
  empty/error state. See also `DSQuerySeed` under Re-sync risks.
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
export DS_CHROMIUM_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"   # macOS only — see Windows below
node .ds-sync/package-validate.mjs ./ds-bundle
```

Or, on a re-sync, the one-command driver (preferred — it also writes `.sync-diff.json`,
which the upload's `deletes` list must come from):
```sh
node .ds-sync/resync.mjs --config .design-sync/config.json --node-modules frontend/node_modules \
  --out ./ds-bundle --remote .design-sync/.cache/remote-sync.json
```

## Windows setup (added 2026-08-24 — the section above was written on macOS)

**The repo's `node_modules` trees are an Apple-Silicon install.** They are shared
across machines, so on Windows every native binary is missing and each one fails as
a different-looking error. `npm install` in `frontend/` is NOT safe to run casually
(no lockfile by design), so fetch just the missing binaries:
```sh
cd <scratch> && npm pack lightningcss-win32-x64-msvc@1.32.0
tar -xzf /c/<scratch>/lightningcss-win32-x64-msvc-1.32.0.tgz \
  -C /d/.../frontend/node_modules/lightningcss-win32-x64-msvc --strip-components=1
# same for @tailwindcss/oxide-win32-x64-msvc@4.3.2
```
Pin the versions to whatever `lightningcss/package.json` and `@tailwindcss/oxide/package.json`
report — a mismatch fails the same way as absence. `.ds-sync/node_modules` is a
*separate* Apple-Silicon tree and needs its own repair:
`cd .ds-sync && PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 npm install` (adds `@esbuild/win32-x64`).
Still missing, and not needed by design-sync: `@rollup/rollup-win32-x64-msvc` (this is
what blocks `vitest`), `@next/swc-win32-x64-msvc`, `@img/sharp-win32-x64`.

**The stub package must use junctions, not `ln -s`.** Git Bash `ln -s` on Windows
silently makes a *copy*. The Jul-12 stub was therefore a frozen snapshot: 59 source
files against 84 real ones, so the converter cheerfully rebuilt six-week-old
components and nothing anywhere said so. Recreate it from PowerShell:
```powershell
$stub="...\frontend\node_modules\portal-frontend"; $fe="...\frontend"
Remove-Item -Recurse -Force "$stub\src","$stub\.ds-shims"
New-Item -ItemType Junction -Path "$stub\src"       -Target "$fe\src"
New-Item -ItemType Junction -Path "$stub\.ds-shims" -Target "$fe\.ds-shims"
New-Item -ItemType HardLink -Path "$stub\tsconfig.ds.json" -Target "$fe\tsconfig.ds.json"
Copy-Item "$fe\.ds-compiled.css" "$stub\.ds-compiled.css" -Force   # real copy — cssEntry is PKG_DIR-bound
```
`tsconfig.ds.json` is a **hard link on purpose**: it started as a copy, and editing
`frontend/tsconfig.ds.json` to register the `next/navigation` shim then left the stub
holding the old paths map — the shim silently did not apply and burned a full
build cycle diagnosing a "fix that didn't work". Junctions/hard links need no admin
rights. **Verify before every build** (cheap, and it is the failure that hides best):
`ls frontend/node_modules/portal-frontend/src/templates/v1 | wc -l` must match the real tree.

**Render browser — no `DS_CHROMIUM_PATH` needed.** `.ds-sync`'s playwright (1.61.1)
pins chromium build **1228**, and `%LOCALAPPDATA%\ms-playwright\chromium-1228` is
already present, so the render check runs natively. (System Chrome is at
`C:\Program Files\Google\Chrome\Application\chrome.exe` if a future version ever
diverges from the cache.)

**Two shell gotchas that cost time here:**
- Git Bash `tar -xzf "C:/…"` fails with `Cannot connect to C: resolve failed` — it
  reads `C:` as a remote host. Use `/c/…`.
- `node -e` given a `/d/…` path resolves it as `D:\d\…`. Inside node, use `D:/…`.

## Re-sync risks

- **69 of 79 `.d.ts` files carry no real props** (`[key: string]: unknown`). This is
  the biggest quality gap in the sync and it is structural, not a bug: `[NO_DIST]`
  synth-entry mode means ts-morph has nothing to parse (`[DTS] parsed 0 .d.ts files`),
  so props exist only for the components hand-written into `cfg.dtsPropsFor`. The
  `.d.ts` IS the API contract the design agent codes against, so those 69 tell it
  "any prop is fine". Real fix: emit declarations (`tsc --emitDeclarationOnly` over
  `templates/v1` into the stub package) and point the converter at a built entry —
  that repairs all 79 at once. Stopgap: keep adding `cfg.dtsPropsFor` entries.
- **Authored previews are pinned to upstream component APIs and WILL rot.**
  `Composer` proved it this run: it went from `{displayName, onPost}` to a fully
  controlled `{displayName, bodyMd, onBodyMdChange, onSubmit, submitting?, error?}`,
  and the old preview died on `bodyMd.trim()` of `undefined`. On re-sync, any
  `✗ [RENDER] … root empty` on a component that used to work is this first, not a
  converter problem — diff the props before anything else.
- **Three views are carded only because their query cache is seeded**:
  `ContinueRail`, `PersonDetailView`, `MediaDetailView` fetch their own data and take
  no data props (`ContinueRail` returns literal `null` when empty), so their previews
  seed `["continue-items"]`, `["person", id]`, `["assets", id]`(+`"progress"`) through
  `DSQuerySeed`. Those fixtures are copies of `ContinueItem` / `Person` / `MediaAsset`
  — change those wire types and the cards silently show the "not found" branch again.
  `MediaDetailView`'s HLS URL never resolves; the card is the player chrome by design.
- **`DSQuerySeed` must stay in `.ds-shims/ds-provider.tsx`** (i.e. exported from the
  bundle). Previews externalize only react and the DS package, so a preview importing
  `@tanstack/react-query` itself gets a SECOND copy whose provider context the bundled
  components cannot see — it fails "No QueryClient set" exactly as if unwrapped.
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
  injected at runtime by `@vidstack/react`. Non-blocking, expected. (Since 2026-08-24
  `MediaDetailView` does card a real player, so these now matter to one card — it
  still renders correctly because vidstack defines them at runtime.)
- `[GRID_OVERFLOW]` — all nine known cases are answered by `cfg.overrides` and should
  NOT re-fire: `cardMode: "single"` for the fixed/portal overlays (`AddBook`,
  `ChatResponsive`, `ChoseFromMyPhoto`, `UpdateHeaderPhoto`, `HelloPreloader`,
  `MasterBase`) and `cardMode: "column"` for the too-wide rows (`CommentItem`,
  `ReactionBar`, `SidebarCenter`). If one re-fires, its `primaryStory` export was
  renamed. Note these can only be *seen* once a component renders — `MasterBase` and
  `SidebarCenter` only surfaced after the `next/navigation` shim un-blanked them, so
  expect a fresh crop of these the first time a batch of broken cards is fixed.

## Component groups (derived from src paths; `ui/` is a generic dir → `general`)

- **general**: Avatar, Icon, FriendRequestsMenu, MessagesMenu, NotificationsMenu
- **headers**: TopMenu · **menu**: SidebarLeft, SidebarCenter, SidebarRight
- **master**: MasterBase, MasterPublic
- **views**: HomeView (home), LoginView/RegisterView/AuthForm/AuthLanding (auth),
  ComicIndexView (comic), NovelDetailView (novel), UploadStudio (upload)
- **partials**: GoToTop, HelloPreloader
- **popup**: AddBook, ChatResponsive, ChoseFromMyPhoto, UpdateHeaderPhoto
