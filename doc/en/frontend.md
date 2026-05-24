# Portal — Frontend architecture

Companion to [feature.md](feature.md). Where `feature.md` defines the full system, this doc focuses on the **`frontend/`** Next.js 15 app: structure, state, rendering, auth handoff, and the page inventory built from `template-main/`.

Read [§16 Frontend](feature.md) of feature.md first for the high-level decisions ([D-32], [D-33], [D-34], [D-7]); this doc expands them with concrete patterns and a build-out roadmap.

---

## 1. Current state

```
frontend/
├── Dockerfile
├── next.config.ts
├── package.json
├── postcss.config.mjs
├── tsconfig.json
└── src/
    ├── app/
    │   ├── globals.css
    │   ├── layout.tsx
    │   └── page.tsx
    └── lib/
        └── api-client.ts          ← will be replaced by generated + server-only client (D-34)
```

Status: **bare scaffold**. Phase 0 lands the real structure:
- `app-router` + RSC, Tailwind v4, TypeScript already chosen.
- Vidstack for HLS playback ([§3 Media](feature.md)).
- Zustand + TanStack Query + React Hook Form for state ([D-32]).
- `next-intl` for i18n ([D-7]).

Two visual-design sources, both **reference only — not active code**:

| Source | Tech | Use |
|---|---|---|
| [template-main/portal/](../../template-main/portal/) | Laravel/Blade + Bootstrap 4 + jQuery | Admin/portal UI primitives: master layouts, sidebars, popups, header/menu, sidebar widgets, page-loader |
| [template-main/social/](../../template-main/social/) | Static HTML (Olympus theme) | Social product UI: ~70 pages across newsfeed/profile/friends/communities/events/messaging |

The Next.js rewrite reinvents both **with React + Tailwind**. Original CSS/JS is not imported. Only structure + interaction patterns + asset layout are reused as references.

---

## 2. Architecture

### 2.1 Route group structure

Next.js App Router groups (`(name)`) don't appear in URLs — they're for organizing layouts.

```
src/app/
├── layout.tsx                                ← root layout: providers, fonts, theme
├── globals.css
├── page.tsx                                  ← marketing home (RSC)
│
├── (marketing)/                              ← public-facing pages, SEO-heavy
│   ├── about/page.tsx
│   ├── careers/page.tsx
│   ├── faqs/page.tsx
│   ├── blog/
│   │   ├── page.tsx                          ← list
│   │   └── [slug]/page.tsx                   ← post
│   └── layout.tsx                            ← marketing header + footer
│
├── auth/                                     ← NOT a group; real path segment
│   ├── login/page.tsx
│   ├── callback/page.tsx                     ← OIDC redirect target
│   ├── refresh-and-return/page.tsx           ← D-34 server-side refresh handler
│   └── layout.tsx                            ← minimal "auth pages" chrome
│
├── t/                                        ← tenant URL prefix (D-23)
│   └── [tenant]/                             ← tenant slug (or "me" for personal)
│       ├── layout.tsx                        ← tenant context provider + chrome
│       │
│       ├── (app)/                            ← authenticated app shell
│       │   ├── layout.tsx                    ← left sidebar + header + right widgets
│       │   ├── page.tsx                      ← user home / dashboard
│       │   │
│       │   ├── (movies)/
│       │   │   ├── movies/page.tsx           ← catalog
│       │   │   ├── movies/[id]/page.tsx      ← detail + player
│       │   │   └── continue/page.tsx         ← per-user "continue watching"
│       │   ├── (music)/
│       │   ├── (stories)/
│       │   ├── (comics)/
│       │   │
│       │   ├── (bank)/                       ← Phase 5
│       │   │   ├── accounts/page.tsx
│       │   │   ├── transactions/page.tsx
│       │   │   ├── debts/page.tsx
│       │   │   ├── investments/page.tsx
│       │   │   ├── budgets/page.tsx
│       │   │   ├── goals/page.tsx
│       │   │   ├── reports/page.tsx          ← net-worth + cash-flow
│       │   │   └── household/page.tsx
│       │   │
│       │   ├── (social)/                     ← Phase 7+
│       │   │   ├── feed/page.tsx
│       │   │   ├── profile/[user]/page.tsx
│       │   │   ├── friends/page.tsx
│       │   │   ├── community/[slug]/page.tsx
│       │   │   ├── events/page.tsx
│       │   │   ├── messages/page.tsx
│       │   │   ├── stories/page.tsx          ← Phase 10
│       │   │   ├── reels/page.tsx            ← Phase 10
│       │   │   └── live/page.tsx             ← Phase 10
│       │   │
│       │   ├── (creator)/                    ← Phase 11
│       │   │   ├── studio/page.tsx
│       │   │   ├── earnings/page.tsx
│       │   │   └── payouts/page.tsx
│       │   │
│       │   └── account/                      ← user settings
│       │       ├── profile/page.tsx
│       │       ├── notifications/page.tsx
│       │       ├── privacy/page.tsx
│       │       ├── security/page.tsx
│       │       └── sessions/page.tsx
│       │
│       └── (admin)/                          ← gated to admin/superadmin role
│           ├── layout.tsx                    ← admin-only chrome
│           ├── users/page.tsx
│           ├── groups/page.tsx
│           ├── policies/page.tsx
│           ├── audit/page.tsx
│           └── safety/page.tsx               ← T&S dashboard (Phase 12)
│
├── search/page.tsx                           ← cross-tenant; doesn't take /t/ prefix
├── company/                                  ← optional marketing microsite
│   ├── about/...
│   ├── help/...
│   └── careers/...
└── not-found.tsx
```

**Why this shape:**

- **`/t/{tenant}/...`** is real path, matching [D-23]'s tenant URL prefix decision. Personal data routes use `/t/me/...`.
- **Route groups** `(app)`, `(admin)`, `(marketing)`, `(movies)`, etc. only organize layouts — invisible in URLs.
- **`/auth/*`** is NOT inside `/t/{tenant}/` because OIDC callback happens before tenant context is known.
- **`/search`** is cross-tenant (`D-2` aggregator); doesn't take prefix.
- Tenant slug `[tenant]` is resolved by middleware → injected via React context (see §4.2).

### 2.2 Layout hierarchy (3-deep)

```
RootLayout                          src/app/layout.tsx
├── <html>, <body>, fonts
├── ThemeProvider (dark/light)
├── TanStackQueryProvider
├── NextIntlClientProvider
└── Toaster (global)
    │
    ├── (marketing)/layout.tsx     ← marketing chrome
    ├── auth/layout.tsx            ← minimal centered auth pages
    │
    └── /t/[tenant]/layout.tsx     ← tenant context loaded server-side
        ├── TenantProvider (org_id, kind, settings)
        ├── AuthGuard               ← redirects to /auth/login if unauth
        │
        ├── (app)/layout.tsx       ← sidebar + header + right widgets
        │   ├── LeftSidebar (nav, communities I'm in)
        │   ├── HeaderBar (search, notifications bell, user menu)
        │   ├── <main>{children}</main>
        │   └── RightSidebar (widgets: weather, suggested friends, ads, etc.)
        │
        └── (admin)/layout.tsx     ← admin sidebar + admin header
```

**Rule:** layouts compose top-down. A child layout never overrides what a parent provides — parents wrap.

### 2.3 Rendering strategy per surface ([D-33])

| Surface | Mode | Why |
|---|---|---|
| Marketing pages, blog | Full RSC + ISR | SEO; cache via `next.revalidate` |
| Movie/music/story/comic **catalogue** | RSC + client list-pagination island | SEO; large lists stream HTML; user-state via `cookies()` |
| Movie/music/story/comic **detail** | RSC shell + client player island | Metadata is SEO; player needs JS |
| **Player / reader** | Mostly client component | Stateful; post-auth; SEO irrelevant |
| **Newsfeed** | First page SSR + subsequent client-paginated | Initial paint fast; load-more is client |
| **Account / bank** | RSC shell + client interactivity islands | Interactive but private; ergonomic to fetch server-side |
| **Real-time surfaces** (chat, live stream, live notifications) | Client component | Needs WebSocket / SSE |
| **Studio / admin forms** | Client component | Heavy form state via React Hook Form |

**Hard rule:** default to RSC. Only `'use client'` when you have:
- Event handlers (`onClick`, `onChange`, `onSubmit`)
- React hooks (`useState`, `useEffect`, `useReducer`, custom hooks)
- Browser APIs (`window`, `localStorage`, `Notification`, `navigator`)
- Vidstack player or anything requiring DOM

If a page has both — make it RSC with a small `'use client'` component island for interactivity. Example: movie detail page is RSC; only `<MoviePlayer />` and `<RatingForm />` are client.

---

## 3. State management ([D-32])

Five orthogonal kinds of state. Each has exactly one owner.

| State category | Owner | Examples | Persistence |
|---|---|---|---|
| **Server state** | TanStack Query | movie list, user profile, transactions, posts, notifications | Cache only; refetch on mutation |
| **UI persistent** | Zustand + `persist` middleware | theme, sidebar collapsed, layout density, last-tab | `localStorage` |
| **UI ephemeral** | Zustand (no persist) | active toast, command palette open, modal stack | In-memory, dies on reload |
| **Form state** | React Hook Form | every form (login, post composer, bank tx, account settings) | In-memory until submit |
| **Shareable filters** | URL query params (read by TanStack) | `?page=2&sort=date&genre=action&q=keyword` | URL |

### 3.1 Hard rule

**No Zustand store may hold data fetched from the API.** If you find yourself writing:

```typescript
// ❌ ANTI-PATTERN
const useStore = create((set) => ({
  movies: [],
  fetchMovies: async () => {
    const m = await fetch('/api/v1/.../movies').then(r => r.json());
    set({ movies: m });
  },
}));
```

You've taken a wrong turn. Use TanStack Query:

```typescript
// ✓ CORRECT
const { data: movies, isLoading } = useQuery({
  queryKey: ['movies', { page, sort, genre }],
  queryFn: () => apiClient.movies.list({ page, sort, genre }),
});
```

Why: TanStack handles caching, refetching, optimistic updates, race conditions, retries, dedup, stale-while-revalidate. Re-implementing any of that on top of Zustand creates bugs.

### 3.2 Zustand stores (allowed surface)

```typescript
// frontend/src/stores/ui.ts
type UIState = {
  sidebarCollapsed: boolean;
  theme: 'light' | 'dark' | 'system';
  layoutDensity: 'compact' | 'comfortable';
  toggleSidebar: () => void;
  setTheme: (t: UIState['theme']) => void;
};

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      theme: 'system',
      layoutDensity: 'comfortable',
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      setTheme: (theme) => set({ theme }),
    }),
    { name: 'portal-ui' },
  ),
);

// frontend/src/stores/modals.ts
type ModalState = {
  stack: ModalSpec[];
  push: (m: ModalSpec) => void;
  pop: () => void;
};
```

Stores stay small (< 100 lines each). One store per concern, not one big "appStore".

### 3.3 TanStack Query patterns

```typescript
// frontend/src/lib/api-hooks/movies.ts
export const moviesKeys = {
  all: ['movies'] as const,
  list: (filters: Filters) => [...moviesKeys.all, 'list', filters] as const,
  detail: (id: string) => [...moviesKeys.all, 'detail', id] as const,
  progress: (id: string) => [...moviesKeys.all, 'progress', id] as const,
};

export function useMovies(filters: Filters) {
  return useQuery({
    queryKey: moviesKeys.list(filters),
    queryFn: () => apiClient.movies.list(filters),
    staleTime: 60 * 1000,            // 1 min
    placeholderData: keepPreviousData,
  });
}

export function useUpsertProgress(movieId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (position: number) => apiClient.movies.upsertProgress(movieId, position),
    onSuccess: () => qc.invalidateQueries({ queryKey: moviesKeys.progress(movieId) }),
  });
}
```

**Key-factories:** every module gets one (`moviesKeys`, `bankKeys`, `socialKeys`). All keys go through the factory — never inline.

**Stale times:** 1 min default, 5 min for catalogue, 0 (always fresh) for personal counters (unread, notifications).

### 3.4 React Hook Form

Every form goes through RHF. Zod for schema validation; resolver auto-wires.

```typescript
const schema = z.object({
  title: z.string().min(1).max(120),
  description: z.string().max(2000).optional(),
});

function MovieEditForm({ defaultValues }: Props) {
  const form = useForm({ resolver: zodResolver(schema), defaultValues });
  const onSubmit = form.handleSubmit(async (values) => { /* mutation */ });
  return <form onSubmit={onSubmit}>...</form>;
}
```

**No form state in Zustand.** No "global form draft store" — that's RHF's job, and RHF persists drafts via `react-hook-form/devtools` or `localStorage` adapter when needed.

### 3.5 URL query params

Filter / pagination state lives in URL — shareable, back-button-friendly, copy-pasteable.

```typescript
// frontend/src/app/t/[tenant]/(app)/(movies)/movies/page.tsx
import { useSearchParams } from 'next/navigation';

const params = useSearchParams();
const page = Number(params.get('page') ?? 1);
const sort = params.get('sort') ?? 'recent';
const filters = { page, sort, genre: params.get('genre') };
const { data } = useMovies(filters);
```

`useQueryStates` from `nuqs` library can simplify type-safe URL params.

---

## 4. Auth handoff ([D-34])

### 4.1 Cookie scheme

| Cookie | Path | Lifetime | Purpose |
|---|---|---|---|
| `portal_access` | `/` | 5 min | Bearer for API; HS256 JWT |
| `portal_refresh` | `/auth` | 30 days | Mints new access via `/auth/refresh` |
| `portal_oidc` | `/auth/callback` | 5 min | State + nonce binding during OIDC flow |

All: `HttpOnly Secure SameSite=Strict`.

**Same-site mandate** ([D-34]): Next.js host + API host MUST share a registrable domain (eTLD+1), e.g. `portal.example.com` + `api.portal.example.com`. Single-host setups route via Traefik path-based.

### 4.2 Server-only API client

```typescript
// frontend/src/lib/api-server.ts
import "server-only";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";

type FetchOpts = RequestInit & { skipRefresh?: boolean };

export async function apiFetch(path: string, opts: FetchOpts = {}): Promise<Response> {
  const cookieHeader = cookies().toString();
  const res = await fetch(`${process.env.API_BASE_URL}${path}`, {
    ...opts,
    headers: {
      ...opts.headers,
      Cookie: cookieHeader,
      Accept: "application/json",
    },
    credentials: "include",
  });

  if (res.status === 401 && !opts.skipRefresh) {
    const here = `${process.env.NEXT_PUBLIC_BASE_URL}${path}`;
    redirect(`/auth/refresh-and-return?return_to=${encodeURIComponent(here)}`);
  }

  return res;
}

// Typed wrappers per module — uses generated OpenAPI types
export const apiServer = {
  movies: {
    list: (filters: MovieFilters) =>
      apiFetch(`/api/v1/t/${tenant}/movies?${qs(filters)}`).then(r => r.json() as Promise<MovieList>),
    get: (id: string) =>
      apiFetch(`/api/v1/t/${tenant}/movies/${id}`).then(r => r.json() as Promise<Movie>),
  },
  // ...
};
```

`import "server-only"` makes the file impossible to bundle into client code — prevents accidentally shipping it to the browser.

### 4.3 Refresh-and-return route

```typescript
// frontend/src/app/auth/refresh-and-return/page.tsx
import { redirect } from "next/navigation";
import { apiFetch } from "@/lib/api-server";

export default async function RefreshAndReturn({
  searchParams,
}: {
  searchParams: Promise<{ return_to?: string }>;
}) {
  const { return_to = "/" } = await searchParams;

  // Refresh cookie has Path=/auth so it WILL be sent here.
  const res = await apiFetch("/auth/refresh", {
    method: "POST",
    skipRefresh: true,       // avoid infinite loop
  });

  if (!res.ok) {
    redirect(`/auth/login?return_to=${encodeURIComponent(return_to)}`);
  }

  // Server-side Set-Cookie from /auth/refresh response is automatically forwarded
  // by Next.js' fetch + redirect; access cookie now fresh on next request.
  redirect(return_to);
}
```

User sees a single navigation flash; no full re-auth needed if refresh token is still valid.

### 4.4 Client-side mutations

Client components POST/PATCH/DELETE via TanStack Query. Cookies travel with `credentials: 'include'` since same-site.

```typescript
// frontend/src/lib/api-client.ts (different from api-server.ts!)
export async function apiMutate(path: string, opts: RequestInit) {
  const res = await fetch(`${process.env.NEXT_PUBLIC_API_BASE_URL}${path}`, {
    ...opts,
    credentials: "include",
    headers: { "Content-Type": "application/json", ...opts.headers },
  });

  if (res.status === 401) {
    window.location.href = `/auth/refresh-and-return?return_to=${encodeURIComponent(window.location.href)}`;
  }

  return res;
}
```

### 4.5 Step-up auth UX ([D-27])

When a sensitive operation returns 403 + `auth.step_up_required` Problem:

```typescript
// frontend/src/lib/error-handlers.ts
import type { Problem } from "@/lib/types.gen";

export function handleProblem(problem: Problem): void {
  switch (problem.type) {
    case "https://portal/errors/auth.step_up_required":
      const stepUp = problem.required_acr;
      const returnTo = problem.return_to ?? window.location.href;
      window.location.href = `/auth/login?step_up=${stepUp}&return_to=${encodeURIComponent(returnTo)}`;
      return;

    case "https://portal/errors/auth.mfa_enrollment_required":
      // Show modal explaining MFA is required, with deep-link to Authentik
      showModal({
        title: "MFA required for this feature",
        body: "Bank operations require multi-factor authentication.",
        cta: { label: "Set up MFA", href: problem.enrollment_url },
      });
      return;

    default:
      showToast({ kind: "error", title: problem.title, body: problem.detail });
  }
}
```

The "Manage MFA" button in account-security settings deep-links to Authentik's dashboard ([D-28]).

---

## 5. i18n & locale formatting ([D-7])

### 5.1 Library: `next-intl`

```typescript
// frontend/src/i18n/config.ts
export const locales = ["en-US", "vi-VN", "ja-JP"] as const;
export const defaultLocale = "en-US";
```

Locale comes from `users.locale` (server-side) → cookie → Accept-Language header → default.

### 5.2 Messages

Per-locale JSON catalogues:

```
frontend/src/i18n/messages/
├── en-US/
│   ├── common.json
│   ├── bank.json
│   ├── movies.json
│   └── errors.json
└── vi-VN/
    ├── common.json
    └── ...
```

Errors keyed by RFC 7807 `type` URI:

```json
// errors.json
{
  "https://portal/errors/auth.refresh.reuse": "Session compromised. Please sign in again.",
  "https://portal/errors/auth.step_up_required": "Additional verification required.",
  "https://portal/errors/bank.account.has_balance": "Cannot delete an account with balance."
}
```

When backend returns a Problem, frontend looks up `errors[problem.type]` for localised display. Falls back to `problem.title` if key missing.

### 5.3 Money formatting

Backend never pre-formats. API returns `{ amount: "12345.67", currency: "USD" }`.

```typescript
// frontend/src/lib/format.ts
import type { Money } from "@/lib/types.gen";

export function formatMoney(money: Money, locale: string): string {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency: money.currency,
  }).format(Number(money.amount));
}
```

### 5.4 Date / time formatting

Backend returns ISO 8601 UTC. Frontend formats per `users.locale` + `users.timezone`.

```typescript
// frontend/src/lib/format.ts
import { Temporal } from "@js-temporal/polyfill";

export function formatDateTime(
  isoUTC: string,
  locale: string,
  timeZone: string,
  style: "short" | "medium" | "long" = "medium",
): string {
  const instant = Temporal.Instant.from(isoUTC);
  const zoned = instant.toZonedDateTimeISO(timeZone);
  return zoned.toLocaleString(locale, { dateStyle: style, timeStyle: style });
}

export function formatRelative(isoUTC: string, locale: string): string {
  // "2 minutes ago", "in 3 days" — via Intl.RelativeTimeFormat
}
```

Use `Temporal` (TC39 stage 3) — better than `Date` for TZ. Polyfilled until browser support is universal.

---

## 6. Page inventory

Mapping every template asset to a Next.js page. Status:
- **A** — asset exists in template-main; UI re-implemented in React
- **N** — no template; design from scratch

### 6.1 Admin / portal pages (from `template-main/portal/`)

| Template asset | Next.js route | Phase | Status |
|---|---|---|---|
| `portal/resources/views/v1/views/home/home.blade.php` | `/t/{tenant}/(app)/page.tsx` | Phase 0 stub | A |
| `portal/resources/views/v1/public/login.blade.php` | `/auth/login/page.tsx` | Phase 0 | A |
| `portal/resources/views/v1/public/register.blade.php` | (out of scope — Authentik handles) | — | — |
| `portal/resources/views/v1/views/library/...` | `/t/{tenant}/(app)/(stories)/library/page.tsx` | Phase 4 | A |
| `portal/resources/views/v1/components/menu/sidebarLeft.blade.php` | Component `<LeftSidebar />` | Phase 0 | A |
| `portal/resources/views/v1/components/menu/sidebarRight.blade.php` | Component `<RightSidebar />` | Phase 0 | A |
| `portal/resources/views/v1/components/headers/menu.blade.php` | Component `<TopHeader />` (with search bar, notifications bell, friend requests dropdown, user menu) | Phase 0 | A |
| `portal/resources/views/v1/components/popup/chatResponsive.blade.php` | Component `<ChatDrawer />` (mobile) | Phase 7 | A |
| `portal/resources/views/v1/components/popup/updateHeaderPhoto.blade.php` | Modal `<CoverPhotoEditor />` | Phase 7 | A |
| `portal/resources/views/v1/components/popup/choseFromMyPhoto.blade.php` | Modal `<PhotoPickerFromAlbums />` | Phase 7 | A |
| `portal/resources/views/v1/components/popup/addBook.blade.php` | Modal `<AddContentDialog />` (generic uploader for books/movies/etc.) | Phase 4 | A |
| `portal/resources/views/v1/partials/hellopreloader.blade.php` | Component `<RouterLoader />` (intercept route transitions) | Phase 0 | A |
| `portal/resources/views/v1/partials/goToTop.blade.php` | Component `<ScrollToTopButton />` | Phase 0 | A |
| `portal/resources/views/errors/{401,403,404,419,429,500,503}.blade.php` | `/app/error.tsx`, `/app/not-found.tsx`, `/app/global-error.tsx` | Phase 0 | A |
| `portal/document/anh1.png` (group admin) | `/t/{tenant}/(admin)/groups/page.tsx` + detail | Phase 4 | A |
| `portal/document/anh2.png` (policy admin) | `/t/{tenant}/(admin)/policies/page.tsx` + detail | Phase 4 | A |
| `portal/document/anh3.png` (create-group modal, policy search) | Modals on group / policy pages | Phase 4 | A |

### 6.2 Social pages (from `template-main/social/`)

Mapped to feature.md §9 subsections. Per-page inventory:

| Template HTML | Next.js route | feature.md ref | Phase |
|---|---|---|---|
| `Newsfeed.html`, `Newsfeed - Masonry.html` | `(social)/feed/page.tsx` | §9.1 | 7 |
| `Profile Page.html`, `ProfilePage-LoggedOut.html` | `(social)/profile/[user]/page.tsx` | §9.2 | 7 |
| `Profile Page - About/Friends/Photos/Videos.html` | `(social)/profile/[user]/(tabs)/{about,friends,photos,videos}/page.tsx` | §9.2 | 7 |
| `Your Account - Friends Requests.html` | `(social)/friends/requests/page.tsx` | §9.3 | 7 |
| `Friend Groups.html` | `(social)/friends/groups/page.tsx` | §9.3 | 7 |
| `Favorit Page Feed.html`, `Favorit Page - About.html`, `Favorit Page - Events.html`, `Favourite Page With Tabs.html` | `(social)/community/[slug]/(tabs)/{feed,about,events}/page.tsx` | §9.4 | 7 |
| `Fav Page - Settings And Create Popup.html` | Modal `<CommunitySettingsDialog />` | §9.4 | 7 |
| `Calendar and Events - Create Event POPUP (Private_Public).html` | `(social)/events/page.tsx` + `<CreateEventDialog />` | §9.5 | 7 |
| `Your Account - Chat Messages.html` | `(social)/messages/page.tsx` | §9.6 | 7 |
| `Your Account - Notifications.html` | `account/notifications/page.tsx` + `<NotificationsBellDropdown />` | §9.7 | 6 |
| `Social Search Results.html` | `/search/page.tsx` | §9.8 | 8 |
| `Community Badges.html` | `account/badges/page.tsx` | §9.9, §9.36 | 10 |
| `Statistics.html` | `(creator)/studio/analytics/page.tsx` | §9.10 | 11 |
| `Weather Widget.html`, `Sticky Sidebars.html` | Component `<RightSidebar />` widget slots | §9.11 | 7 |
| `Manage Widgets.html` | `account/widgets/page.tsx` | §9.11 | 7 |
| `Music And Playlists.html` | `(music)/page.tsx` | §5 | 4 |
| `Post Versions.html` | Reference for post-type rendering variants; informs `<PostCard />` design | §9.1 | 7 |
| `Blog V1 Grid.html` | `(marketing)/blog/page.tsx` | §13 | 9 |
| `Landing Page.html` | `/page.tsx` (marketing root) | §13 | 9 |
| `Your Account - Account Settings.html`, `Change Password.html`, `Personal Information.html`, `Education And Employement.html`, `Hobbies And Interests.html`, `Tabs Version.html` | `account/(tabs)/...` | §9.2, §12 | 7 |
| `Olympus Shortcodes.html`, `Theme Icons.html` | Component library Storybook entries | — | 0 |
| `Page Without Left Panel.html`, `Page Without Right Panel.html` | Layout variants for special pages (live stream, reels) | §9.24, §9.25 | 10 |

### 6.3 Media verticals + bank (no template — design from scratch)

| Route | Component highlights | Phase |
|---|---|---|
| `(movies)/movies/page.tsx` | Hero rail + genre grids + continue rail | 3 |
| `(movies)/movies/[id]/page.tsx` | Vidstack player + episode list + ratings | 3 |
| `(music)/page.tsx` | Track grids + now-playing bar + playlist editor | 4 |
| `(stories)/page.tsx` | Library grid + recently-read rail + chapter reader | 4 |
| `(comics)/page.tsx` | Cover grid + chapter selector + multi-mode reader | 4 |
| `(bank)/accounts/page.tsx` | Account list with balance + add-account flow | 5a |
| `(bank)/transactions/page.tsx` | Infinite scroll + filters (category, date, account) + bulk-edit | 5a |
| `(bank)/investments/page.tsx` | Holdings table + cost-basis + lots breakdown | 5e |
| `(bank)/reports/page.tsx` | Net-worth chart + cash-flow Sankey + per-category breakdowns | 5g |
| `(creator)/studio/page.tsx` | Tip earnings + subscriber count + recent payouts | 11 |

---

## 7. Component library

Build with **Radix UI primitives + Tailwind** as the base. No Material UI / AntD — they fight Tailwind and add bundle weight.

### 7.1 Primitives layer

| Component | Source | Purpose |
|---|---|---|
| Button, Input, Textarea, Select, Checkbox, Switch | Radix UI + Tailwind | Form controls |
| Dialog, Sheet, Drawer, Popover, Tooltip, Toast | Radix UI | Floating UI |
| DropdownMenu, Tabs, Accordion | Radix UI | Compound |
| Avatar, Badge, Skeleton, Spinner | Custom (Tailwind) | Display |

Wrap each in `frontend/src/components/ui/` to centralize tailwind classes.

### 7.2 Feature components

```
frontend/src/components/
├── ui/                      ← primitives
├── layout/
│   ├── LeftSidebar.tsx
│   ├── RightSidebar.tsx
│   ├── TopHeader.tsx
│   ├── MobileNav.tsx
│   └── BottomTabBar.tsx     ← mobile-only quick switcher
├── auth/
│   ├── SignInButton.tsx
│   └── SessionGuard.tsx
├── media/
│   ├── HLSPlayer.tsx        ← Vidstack wrapper
│   ├── PosterCard.tsx
│   ├── ContinueRail.tsx
│   └── EpisodeList.tsx
├── social/
│   ├── PostComposer.tsx
│   ├── PostCard.tsx
│   ├── CommentThread.tsx
│   ├── ReactionPicker.tsx
│   ├── FriendList.tsx
│   ├── ChatThread.tsx
│   ├── StoryRing.tsx
│   ├── ReelPlayer.tsx
│   └── LiveStreamPlayer.tsx
├── bank/
│   ├── AccountCard.tsx
│   ├── TransactionRow.tsx
│   ├── MoneyDisplay.tsx     ← uses formatMoney + locale
│   ├── CategoryPicker.tsx
│   ├── BudgetProgress.tsx
│   └── NetWorthChart.tsx    ← Recharts or Visx
├── admin/
│   ├── PolicyEditor.tsx
│   ├── GroupTree.tsx
│   └── ModQueue.tsx
└── forms/
    ├── FormField.tsx        ← RHF + Zod wrapper
    └── DateRangePicker.tsx
```

### 7.3 Charting

Bank reports + admin analytics need charts. Pick:

| Library | Bundle | When |
|---|---|---|
| **Recharts** | ~80kb | Quick + composable. Default. |
| **Visx** | tree-shakes from 0 | Complex viz (Sankey, custom interactive). Phase 5g+. |
| **D3** | direct | Only if Visx is too high-level. Avoid. |

### 7.4 Storybook

Optional but valuable: `@storybook/react-vite`. Story per component; visual regression via Chromatic if budget allows. Defer until component library has ~30 pieces.

---

## 8. Performance & accessibility budgets

Set explicit budgets in `next.config.ts` + CI checks.

### 8.1 Performance budgets

| Metric | Budget | Tool |
|---|---|---|
| First Contentful Paint (FCP) | < 1.5 s | Lighthouse CI |
| Largest Contentful Paint (LCP) | < 2.5 s | Lighthouse CI |
| Interaction to Next Paint (INP) | < 200 ms | Lighthouse CI |
| Cumulative Layout Shift (CLS) | < 0.1 | Lighthouse CI |
| Initial JS bundle | < 200 KB compressed | `@next/bundle-analyzer` |
| Per-route additional JS | < 100 KB | Bundle analyzer |

### 8.2 Accessibility budget

- WCAG 2.1 AA target.
- All interactive elements have keyboard support and visible focus rings.
- All images have `alt` text (D-7 makes alt also i18n-searchable).
- Form controls have associated `<label>`.
- Color contrast ≥ 4.5:1 for body text, ≥ 3:1 for large text.
- ARIA attributes only when semantic HTML can't express the role.
- Automated check: `axe-core` via Playwright in CI.

### 8.3 Loading patterns

- **`<Suspense>` boundaries** at the catalogue / detail / shell level — show skeleton, not a spinner.
- **Image strategy:** Next.js `<Image>` with `loading="lazy"` for all media + sized placeholders. AVIF/WebP via Sharp.
- **Prefetch:** `<Link prefetch>` for top-level nav. Hover prefetch for content cards.
- **Streaming HTML** via RSC for catalogue + newsfeed pages — first paint while data still loading.

---

## 9. Asset migration from `template-main/`

What to take, what to leave.

### 9.1 Take

- **Page structure / IA** — the section ordering, sidebar widgets, header layout. Re-implement in React; don't copy HTML.
- **Iconography** — `template-main/portal/public/v1/ico/` (SVG sprites) → migrate to single `<symbol>` sheet or `lucide-react` equivalents.
- **Color palette + spacing scale** — extract from `bootstrap.css`/`main.min.css` into `tailwind.config.js`.
- **Page-loader concept** — `hellopreloader` pattern (page-transition overlay).
- **anh1/anh2/anh3.png mocks** — design source-of-truth for admin (group/policy management). Not implemented yet.

### 9.2 Drop

- **jQuery, Bootstrap JS, selectize, magnific-popup, isotope, daterangepicker** — replace with Tailwind + Radix + headless libs.
- **CKEditor 5** — replace with **Tiptap** (React-native, more modular) for the long-form article composer.
- **Swiper** — keep if needed for carousels; or use `embla-carousel-react`.
- **All Blade templates** — re-architect as React server/client components.
- **All `.css` files from template** — extract palette only; rewrite as Tailwind utility classes.

### 9.3 Image assets

- Avatars / placeholders from `template-main/social/img/` — usable as dev fixtures; replace with real CDN content in prod.
- Logo: needs redesign — current `template-main/social/img/logo.png` is "Olympus" branded.

### 9.4 Don't auto-port

Do not write a script that converts Blade → React. Manual re-architect ensures we don't carry forward the jQuery/Bootstrap layering. Use the templates as visual references in a second monitor while building from scratch.

---

## 10. Phase-by-phase frontend deliverables

### Phase 0 — Foundation

- Wire root layout: providers (TanStack, NextIntl, Theme, Toast), fonts, global CSS.
- **Server-only API client** (`api-server.ts`) per [D-34]; **mutation client** (`api-client.ts`) for client components.
- **Refresh-and-return route** `/auth/refresh-and-return/page.tsx`.
- **Generated TS types** from OpenAPI → `frontend/src/lib/types.gen.ts`.
- **OIDC login flow** — `/auth/login/page.tsx`, `/auth/callback/page.tsx`.
- **Auth context** — read `users.locale`, `users.timezone`, current tenant via RSC.
- **`frontend/CLAUDE.md` conventions doc** ([D-32, D-33]) with anti-pattern examples.
- **Error pages** (`error.tsx`, `not-found.tsx`, `global-error.tsx`) styled.
- **Component library kickoff** — `<Button />`, `<Dialog />`, `<Toast />`, `<Avatar />`, `<TopHeader />`, `<LeftSidebar />`, `<RightSidebar />`.

**Exit:** sign in via Authentik, see authenticated home page with sidebar shell. RBAC-gated route returns 403 → toast displays it.

### Phase 1 — Tenant URL prefix

- Move all authenticated routes under `/t/[tenant]/...`.
- `TenantProvider` reads tenant slug from URL params.
- Tenant switcher in `<TopHeader />` (only if user has multiple memberships).

**Exit:** `/t/me/...` and `/t/{orgSlug}/...` both work; switching reloads tenant context.

### Phase 2-3 — Media + Movies

- `<HLSPlayer />` Vidstack wrapper.
- Movies catalogue + detail + player.
- `<PosterCard />`, `<ContinueRail />`.
- Watch-progress upsert (debounced; commits every 10 seconds + on pause).

### Phase 4 — Music + Stories + Comics

- Repeat Phase 3 pattern per vertical.
- `<MusicPlayer />` (persistent bottom bar), `<PlaylistEditor />`.
- Story / comic reader components.
- `/api/v1/continue` aggregator → unified "Continue" rail on home dashboard.

### Phase 5 — Bank

- Money types: `<MoneyDisplay />`, `<MoneyInput />` (string-amount-aware).
- Transaction list with infinite scroll + filter chips.
- Net-worth chart, cash-flow Sankey.
- Step-up auth UX wired into every destructive bank op (delete account, export).
- MFA enrollment flow if user lacks `amr=mfa` ([D-28]).

### Phase 6 — Notifications

- `<NotificationsBellDropdown />`.
- **SSE client** subscribed to `/api/v1/events/stream`; pushes new notifications into TanStack cache (mutate cache directly, no refetch).
- Web Push subscription via Service Worker; ask for permission only after user opts in via settings.

### Phase 7 — Social baseline

- Newsfeed, profile, friends, communities, events, messaging.
- `<PostComposer />` (text + image + video upload + poll + draft + schedule).
- `<PostCard />` rendering all post-types.
- `<ChatThread />` with WebSocket via `platform/realtime/`.
- Privacy controls UI; per-post visibility selector.

### Phase 8 — Search

- `<CommandPalette />` global search bar (Cmd+K).
- Typeahead via `useDeferredValue` + 150ms debounce.
- Result categories rendered as sections.

### Phase 9 — Marketing microsite

- Static-rendered marketing pages.
- Blog reader.
- Optional merch checkout (defer).

### Phase 10 — Advanced social

- **Stories ring** at top of feed.
- **Reels** vertical-scroll feed with `IntersectionObserver` auto-play.
- **Live stream** player + chat overlay.
- Long-form article composer (**Tiptap**).
- Voting + karma UI (Reddit-style).
- "For You" feed with `<WhyAmISeeingThis />` popover ([D-35]).

### Phase 11 — Creator economy

- Tip dialog with step-up auth.
- Subscriber list, payouts dashboard.
- Stripe Connect Express OAuth handoff for creator onboarding.

### Phase 12 — Marketplace + calls + safety

- Marketplace listings UI.
- WebRTC call UI via LiveKit JS SDK ([D-39]).
- Trust & safety dashboard for `superadmin`.
- GDPR export flow (request, status, download).

---

## 11. Anti-patterns (don't do)

- **`'use client'` on a page just to use one hook.** Extract the hook-using piece into a small client island; keep the page RSC.
- **Storing server data in Zustand.** Use TanStack (see §3.1). Hard rule.
- **Calling `fetch` directly from a server component.** Always use `apiFetch` from `api-server.ts` — it forwards cookies + handles 401.
- **Calling `fetch` directly from a client component.** Always use `apiMutate` from `api-client.ts` — it handles 401 redirects.
- **Reading `process.env.SECRET_*` in a client component.** Only `NEXT_PUBLIC_*` should ever leave the server. Server-only secrets live in server components / route handlers.
- **Pre-formatting money or dates on the server.** Always send raw ISO 8601 / `{amount, currency}`. Frontend formats per user locale + timezone. ([D-7])
- **Importing the Blade template HTML.** Re-architect as React. Templates are reference, not source.
- **One mega-store in Zustand.** Many small stores per concern.
- **`useState` for filter / pagination state.** Use URL query params (`useSearchParams`) — back button works, URL is shareable.
- **Optimistic update without rollback.** Every `useMutation` with `onMutate` must have `onError` that rolls back.
- **Missing `<Suspense>` boundary on streaming RSC pages.** Page renders blank if data isn't ready. Wrap async server components.
- **Hardcoded English strings in components.** Always `t('key')` via next-intl, even for "OK", "Cancel" — gets translated automatically.

---

## 12. Open questions (frontend-specific)

These aren't blocking but each needs an answer when the relevant phase opens:

1. **Component library: build vs adopt?** Radix + Tailwind is the plan; shadcn/ui is a styled-Radix accelerator worth evaluating in Phase 0.
2. **Chart library final pick.** Recharts default; revisit Visx when Phase 5g (bank reports) opens.
3. **Rich-text editor for articles.** Tiptap proposed (Phase 10). Confirm before article composer ships.
4. **Mobile experience.** PWA-first per [D-6]; manifest + service worker + add-to-home-screen prompts land in Phase 7 (when social makes mobile usage compelling).
5. **End-to-end test framework.** Playwright default; Cypress as alternate. Playwright recommended for parity with axe-core integration.
6. **Bundle splitting strategy.** Per-route splitting is default. Revisit if any route exceeds 200 KB JS budget.
7. **Theme system.** CSS variables + Tailwind v4 native theme support. Confirm token list before component library matures.
8. **Image CDN.** Cloudflare R2 with on-the-fly resize (Cloudflare Image Resizing) vs `next/image` self-hosted. R2 + Cloudflare Resize is the lower-friction default.

---

## How this doc relates to others

- **[feature.md](feature.md)** — system-wide design; this doc expands the `frontend/` slice.
- **[diagrams.md](diagrams.md)** — visual architecture; see "System landscape" diagram for frontend's place in the system, "Authenticated request flow" for the cookie path.
- **[archivetech.md](archivetech.md)** — UI mocks anh1/2/3 referenced in §6.1 admin pages.
- **[CLAUDE.md](../../CLAUDE.md)** — backend conventions; the frontend's `frontend/CLAUDE.md` (Phase 0 deliverable) will mirror its tone.
