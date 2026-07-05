# Frontend templates — versioned presentation layer

The whole UI lives under `src/templates/v{N}/`, one folder per template **version**.
This mirrors the Blade reference at `template-main/portal/resources/views/v1/`,
where the entire presentation layer is namespaced by version so a redesign can
ship as `v2/` without touching `v1/`.

## How it works

- `app/` is **routing only**. Route files never import a specific version — they
  call `activeTemplate()` and render whatever shell/view the active version
  provides. URLs stay clean (`/`, `/login`, `/library/comic`); the version is an
  internal concern, never a URL segment.
- `types.ts` defines the `TemplateManifest` contract: the layout **shells**
  (`public`, `app`) and the page **views** every version must provide.
- `registry.ts` is the **single switch point**. It maps version id → manifest and
  picks the active one from `NEXT_PUBLIC_TEMPLATE_VERSION` (default `v1`).

```
app/(public)/login/page.tsx ─┐
app/(app)/page.tsx ──────────┼─→ activeTemplate() ──→ registry.ts ──→ templates/v1
app/(app)/library/... ───────┘        (env: NEXT_PUBLIC_TEMPLATE_VERSION)
```

## Adding a new version (e.g. v2)

1. `cp -r templates/v1 templates/v2` and restyle / rebuild components.
2. Update `templates/v2/index.ts` to `version: "v2"`.
3. Register it in `registry.ts`: `const REGISTRY = { v1, v2 }`.
4. Run with `NEXT_PUBLIC_TEMPLATE_VERSION=v2` and swap the theme import in
   `app/layout.tsx` (`@/templates/v2/theme/theme.css`).

No `app/` route files change. v1 and v2 can coexist in the repo indefinitely.

## v1 — Blade → Next.js mapping

Source: `template-main/portal/resources/views/v1/` (Crumina "Olympus" theme).

| Blade                                   | Next.js (`templates/v1/`)                  |
| --------------------------------------- | ------------------------------------------ |
| `master/master-base.blade.php`          | `master/MasterBase.tsx` (app shell)        |
| `master/master-public.blade.php`        | `master/MasterPublic.tsx` (guest shell)    |
| `components/head/*` (css/js/fonts)      | `app/layout.tsx` `metadata` + `theme/theme.css` |
| `components/headers/menu`               | `components/headers/TopMenu.tsx`           |
| `components/menu/sidebarLeft`           | `components/menu/SidebarLeft.tsx`          |
| `components/menu/sidebarRight`          | `components/menu/SidebarRight.tsx`         |
| `components/menu/sidebarCenter(+Responsive)` | `components/menu/SidebarCenter.tsx`   |
| `components/footers/svg`                | `components/footers/SvgSprite.tsx`         |
| `components/footers/js` / `ico`         | React hooks / `metadata` favicons (no component) |
| `components/popup/*`                     | `components/popup/*.tsx` (modal stubs)     |
| `partials/hellopreloader`               | `partials/HelloPreloader.tsx`              |
| `partials/goToTop`                      | `partials/GoToTop.tsx`                     |
| `views/home/home`                       | `views/home/HomeView.tsx`                  |
| `public/login` / `register`             | `views/auth/{LoginView,RegisterView}.tsx` + `AuthForm.tsx` |
| `views/library/commic/index`            | `views/library/comic/ComicIndexView.tsx` (typo fixed) |
| `views/library/novel/detail`            | `views/library/novel/NovelDetailView.tsx` |

## Notes / deferred

- **Auth is OIDC** (Authentik) — there is no local password login. `AuthForm`
  keeps the email/password fields as visual scaffold only; the real entry point
  is the SSO button → `/api/v1/auth/login`. See `CLAUDE.md` "Account module".
- **Popups** and the **SVG sprite** are placeholders; port the Blade markup /
  icon `<symbol>`s when those features are built.
- Many views are skeletons — the product **v1 scope cut**
  (`doc/en/architecture/01-v1-scope-cut.md`) defers domain CRUD. This template
  set is the presentation scaffold; data wiring lands per the roadmap. (Note:
  the *template* version "v1" is unrelated to the *product* "v1" milestone.)
