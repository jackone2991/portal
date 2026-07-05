# Portal — Kiến trúc Frontend

Tài liệu đi kèm [feature.md](feature.md). Trong khi `feature.md` định nghĩa toàn hệ thống, doc này focus vào app **`frontend/`** Next.js 15: cấu trúc, state, rendering, auth handoff, và page inventory build từ `template-main/`.

Đọc [§16 Frontend](feature.md) của feature.md trước cho high-level decision ([D-32], [D-33], [D-34], [D-7]); doc này mở rộng chúng với pattern cụ thể và roadmap build-out.

---

## 1. Trạng thái hiện tại

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
        └── api-client.ts          ← sẽ thay bằng generated + server-only client (D-34)
```

Trạng thái: **scaffold trần**. Phase 0 land cấu trúc thực:
- `app-router` + RSC, Tailwind v4, TypeScript đã chọn.
- Vidstack cho HLS playback ([§3 Media](feature.md)).
- Zustand + TanStack Query + React Hook Form cho state ([D-32]).
- `next-intl` cho i18n ([D-7]).

Hai nguồn visual-design, cả hai **chỉ tham chiếu — không phải code active**:

| Nguồn | Tech | Dùng cho |
|---|---|---|
| [template-main/portal/](../../template-main/portal/) | Laravel/Blade + Bootstrap 4 + jQuery | Primitive UI admin/portal: master layouts, sidebars, popups, header/menu, sidebar widget, page-loader |
| [template-main/social/](../../template-main/social/) | HTML tĩnh (Olympus theme) | UI sản phẩm social: ~70 page across newsfeed/profile/friends/communities/events/messaging |

Bản rewrite Next.js viết lại cả hai **với React + Tailwind**. CSS/JS gốc không import. Chỉ structure + pattern interaction + layout asset được reuse như tham chiếu.

---

## 2. Kiến trúc

### 2.1 Cấu trúc route group

Route group của Next.js App Router (`(name)`) không xuất hiện trong URL — chúng để tổ chức layout.

```
src/app/
├── layout.tsx                                ← root layout: provider, font, theme
├── globals.css
├── page.tsx                                  ← marketing home (RSC)
│
├── (marketing)/                              ← page public-facing, SEO-heavy
│   ├── about/page.tsx
│   ├── careers/page.tsx
│   ├── faqs/page.tsx
│   ├── blog/
│   │   ├── page.tsx                          ← list
│   │   └── [slug]/page.tsx                   ← post
│   └── layout.tsx                            ← marketing header + footer
│
├── auth/                                     ← KHÔNG phải group; là path segment thực
│   ├── login/page.tsx
│   ├── callback/page.tsx                     ← target redirect OIDC
│   ├── refresh-and-return/page.tsx           ← D-34 handler refresh server-side
│   └── layout.tsx                            ← chrome tối thiểu cho "auth pages"
│
├── t/                                        ← URL prefix tenant (D-23)
│   └── [tenant]/                             ← tenant slug (hoặc "me" cho personal)
│       ├── layout.tsx                        ← provider context tenant + chrome
│       │
│       ├── (app)/                            ← shell app đã authenticate
│       │   ├── layout.tsx                    ← sidebar trái + header + widget phải
│       │   ├── page.tsx                      ← home / dashboard user
│       │   │
│       │   ├── (movies)/
│       │   │   ├── movies/page.tsx           ← catalog
│       │   │   ├── movies/[id]/page.tsx      ← detail + player
│       │   │   └── continue/page.tsx         ← "continue watching" per-user
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
│       │   └── account/                      ← settings user
│       │       ├── profile/page.tsx
│       │       ├── notifications/page.tsx
│       │       ├── privacy/page.tsx
│       │       ├── security/page.tsx
│       │       └── sessions/page.tsx
│       │
│       └── (admin)/                          ← gate cho role admin/superadmin
│           ├── layout.tsx                    ← chrome admin-only
│           ├── users/page.tsx
│           ├── groups/page.tsx
│           ├── policies/page.tsx
│           ├── audit/page.tsx
│           └── safety/page.tsx               ← T&S dashboard (Phase 12)
│
├── search/page.tsx                           ← cross-tenant; không lấy prefix /t/
├── company/                                  ← marketing microsite optional
│   ├── about/...
│   ├── help/...
│   └── careers/...
└── not-found.tsx
```

**Vì sao shape này:**

- **`/t/{tenant}/...`** là path thực, match quyết định URL prefix tenant của [D-23]. Route data cá nhân dùng `/t/me/...`.
- **Route group** `(app)`, `(admin)`, `(marketing)`, `(movies)`, v.v. chỉ tổ chức layout — invisible trong URL.
- **`/auth/*`** KHÔNG nằm trong `/t/{tenant}/` vì callback OIDC xảy ra trước khi tenant context biết.
- **`/search`** cross-tenant (aggregator `D-2`); không lấy prefix.
- Slug tenant `[tenant]` được resolve bởi middleware → inject qua React context (xem §4.2).

### 2.2 Phân cấp layout (3-deep)

```
RootLayout                          src/app/layout.tsx
├── <html>, <body>, font
├── ThemeProvider (dark/light)
├── TanStackQueryProvider
├── NextIntlClientProvider
└── Toaster (global)
    │
    ├── (marketing)/layout.tsx     ← chrome marketing
    ├── auth/layout.tsx            ← page auth tối thiểu, căn giữa
    │
    └── /t/[tenant]/layout.tsx     ← tenant context load server-side
        ├── TenantProvider (org_id, kind, settings)
        ├── AuthGuard               ← redirect /auth/login nếu unauth
        │
        ├── (app)/layout.tsx       ← sidebar + header + widget phải
        │   ├── LeftSidebar (nav, community mình ở)
        │   ├── HeaderBar (search, bell notification, menu user)
        │   ├── <main>{children}</main>
        │   └── RightSidebar (widget: weather, suggested friend, ad, v.v.)
        │
        └── (admin)/layout.tsx     ← sidebar admin + header admin
```

**Quy tắc:** layout compose top-down. Child layout không bao giờ override cái parent cung cấp — parent wrap.

### 2.3 Strategy rendering per surface ([D-33])

| Surface | Mode | Vì sao |
|---|---|---|
| Page marketing, blog | RSC full + ISR | SEO; cache qua `next.revalidate` |
| **Catalogue** movie/music/story/comic | RSC + client list-pagination island | SEO; list lớn stream HTML; user-state qua `cookies()` |
| **Detail** movie/music/story/comic | RSC shell + client player island | Metadata SEO; player cần JS |
| **Player / reader** | Chủ yếu client component | Stateful; post-auth; SEO irrelevant |
| **Newsfeed** | SSR page đầu + sau đó client-paginated | Initial paint nhanh; load-more là client |
| **Account / bank** | RSC shell + island interactivity client | Interactive nhưng private; ergonomic fetch server-side |
| **Surface real-time** (chat, live stream, notification live) | Client component | Cần WebSocket / SSE |
| **Form Studio / admin** | Client component | Form state nặng qua React Hook Form |

**Quy tắc cứng:** default sang RSC. Chỉ `'use client'` khi bạn có:
- Event handler (`onClick`, `onChange`, `onSubmit`)
- React hook (`useState`, `useEffect`, `useReducer`, hook custom)
- Browser API (`window`, `localStorage`, `Notification`, `navigator`)
- Vidstack player hoặc bất cứ gì cần DOM

Nếu page có cả hai — làm RSC với client component island nhỏ cho interactivity. Ví dụ: page detail movie là RSC; chỉ `<MoviePlayer />` và `<RatingForm />` là client.

---

## 3. Quản lý state ([D-32])

Năm loại state trực giao. Mỗi cái có đúng một owner.

| Category state | Owner | Ví dụ | Persistence |
|---|---|---|---|
| **Server state** | TanStack Query | movie list, profile user, transaction, post, notification | Cache thôi; refetch on mutation |
| **UI persistent** | Zustand + `persist` middleware | theme, sidebar collapsed, density layout, last-tab | `localStorage` |
| **UI ephemeral** | Zustand (không persist) | toast active, command palette open, modal stack | In-memory, chết khi reload |
| **Form state** | React Hook Form | mọi form (login, post composer, bank tx, account settings) | In-memory cho tới submit |
| **Filter chia sẻ được** | URL query param (read bởi TanStack) | `?page=2&sort=date&genre=action&q=keyword` | URL |

### 3.1 Quy tắc cứng

**Không có Zustand store giữ data fetched từ API.** Nếu thấy mình write:

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

Bạn đã rẽ sai. Dùng TanStack Query:

```typescript
// ✓ CORRECT
const { data: movies, isLoading } = useQuery({
  queryKey: ['movies', { page, sort, genre }],
  queryFn: () => apiClient.movies.list({ page, sort, genre }),
});
```

Vì sao: TanStack xử lý cache, refetch, optimistic update, race condition, retry, dedup, stale-while-revalidate. Re-implement bất kỳ cái nào trên Zustand tạo bug.

### 3.2 Zustand stores (surface được phép)

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

Store giữ nhỏ (< 100 dòng mỗi cái). Một store per concern, không phải một "appStore" lớn.

### 3.3 Pattern TanStack Query

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
    staleTime: 60 * 1000,            // 1 phút
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

**Key-factories:** mỗi module có một (`moviesKeys`, `bankKeys`, `socialKeys`). Mọi key đi qua factory — không bao giờ inline.

**Stale time:** 1 phút default, 5 phút cho catalogue, 0 (luôn fresh) cho counter cá nhân (unread, notification).

### 3.4 React Hook Form

Mọi form đi qua RHF. Zod cho schema validation; resolver auto-wire.

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

**Không có form state trong Zustand.** Không "global form draft store" — đó là job của RHF, và RHF persist draft qua `react-hook-form/devtools` hoặc adapter `localStorage` khi cần.

### 3.5 URL query param

State filter / pagination sống trong URL — chia sẻ được, back-button-friendly, copy-paste được.

```typescript
// frontend/src/app/t/[tenant]/(app)/(movies)/movies/page.tsx
import { useSearchParams } from 'next/navigation';

const params = useSearchParams();
const page = Number(params.get('page') ?? 1);
const sort = params.get('sort') ?? 'recent';
const filters = { page, sort, genre: params.get('genre') };
const { data } = useMovies(filters);
```

`useQueryStates` từ thư viện `nuqs` có thể đơn giản hoá URL param type-safe.

---

## 4. Auth handoff ([D-34])

### 4.1 Scheme cookie

| Cookie | Path | Lifetime | Mục đích |
|---|---|---|---|
| `portal_access` | `/` | 5 phút | Bearer cho API; JWT HS256 |
| `portal_refresh` | `/auth` | 30 ngày | Mint access mới qua `/auth/refresh` |
| `portal_oidc` | `/auth/callback` | 5 phút | Bind state + nonce trong flow OIDC |

Tất cả: `HttpOnly Secure SameSite=Strict`.

**Mandate same-site** ([D-34]): host Next.js + host API PHẢI chia sẻ registrable domain (eTLD+1), vd `portal.example.com` + `api.portal.example.com`. Setup single-host route qua Traefik path-based.

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

// Wrapper typed per module — dùng type generated từ OpenAPI
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

`import "server-only"` làm file không thể bundle vào client code — chống vô tình ship sang browser.

### 4.3 Route refresh-and-return

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

  // Cookie refresh có Path=/auth nên SẼ được send ở đây.
  const res = await apiFetch("/auth/refresh", {
    method: "POST",
    skipRefresh: true,       // tránh loop vô hạn
  });

  if (!res.ok) {
    redirect(`/auth/login?return_to=${encodeURIComponent(return_to)}`);
  }

  // Set-Cookie server-side từ response /auth/refresh tự động được forward
  // bởi fetch + redirect của Next.js; access cookie giờ fresh ở request kế tiếp.
  redirect(return_to);
}
```

User thấy một flash navigation; không cần re-auth full nếu refresh token còn hợp lệ.

### 4.4 Mutation client-side

Client component POST/PATCH/DELETE qua TanStack Query. Cookie travel với `credentials: 'include'` vì same-site.

```typescript
// frontend/src/lib/api-client.ts (khác với api-server.ts!)
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

### 4.5 UX step-up auth ([D-27])

Khi op nhạy cảm trả 403 + Problem `auth.step_up_required`:

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
      // Show modal giải thích MFA yêu cầu, với deep-link sang Authentik
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

Button "Manage MFA" trong settings account-security deep-link sang dashboard Authentik ([D-28]).

---

## 5. i18n & format locale ([D-7])

### 5.1 Library: `next-intl`

```typescript
// frontend/src/i18n/config.ts
export const locales = ["en-US", "vi-VN", "ja-JP"] as const;
export const defaultLocale = "en-US";
```

Locale đến từ `users.locale` (server-side) → cookie → header Accept-Language → default.

### 5.2 Messages

Catalog JSON per-locale:

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

Error key theo URI `type` của RFC 7807:

```json
// errors.json
{
  "https://portal/errors/auth.refresh.reuse": "Session compromised. Please sign in again.",
  "https://portal/errors/auth.step_up_required": "Additional verification required.",
  "https://portal/errors/bank.account.has_balance": "Cannot delete an account with balance."
}
```

Khi backend trả Problem, frontend lookup `errors[problem.type]` cho display localised. Fall back về `problem.title` nếu key thiếu.

### 5.3 Format money

Backend không bao giờ pre-format. API trả `{ amount: "12345.67", currency: "USD" }`.

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

### 5.4 Format date / time

Backend trả ISO 8601 UTC. Frontend format theo `users.locale` + `users.timezone`.

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
  // "2 phút trước", "trong 3 ngày" — qua Intl.RelativeTimeFormat
}
```

Dùng `Temporal` (TC39 stage 3) — tốt hơn `Date` cho TZ. Polyfill cho tới khi browser support universal.

---

## 6. Page inventory

Map mỗi template asset sang một page Next.js. Status:
- **A** — asset tồn tại trong template-main; UI re-implement trong React
- **N** — không có template; design from scratch

### 6.1 Page admin / portal (từ `template-main/portal/`)

| Template asset | Route Next.js | Phase | Status |
|---|---|---|---|
| `portal/resources/views/v1/views/home/home.blade.php` | `/t/{tenant}/(app)/page.tsx` | Phase 0 stub | A |
| `portal/resources/views/v1/public/login.blade.php` | `/auth/login/page.tsx` | Phase 0 | A |
| `portal/resources/views/v1/public/register.blade.php` | (out of scope — Authentik xử lý) | — | — |
| `portal/resources/views/v1/views/library/...` | `/t/{tenant}/(app)/(stories)/library/page.tsx` | Phase 4 | A |
| `portal/resources/views/v1/components/menu/sidebarLeft.blade.php` | Component `<LeftSidebar />` | Phase 0 | A |
| `portal/resources/views/v1/components/menu/sidebarRight.blade.php` | Component `<RightSidebar />` | Phase 0 | A |
| `portal/resources/views/v1/components/headers/menu.blade.php` | Component `<TopHeader />` (search bar, bell notification, dropdown friend request, menu user) | Phase 0 | A |
| `portal/resources/views/v1/components/popup/chatResponsive.blade.php` | Component `<ChatDrawer />` (mobile) | Phase 7 | A |
| `portal/resources/views/v1/components/popup/updateHeaderPhoto.blade.php` | Modal `<CoverPhotoEditor />` | Phase 7 | A |
| `portal/resources/views/v1/components/popup/choseFromMyPhoto.blade.php` | Modal `<PhotoPickerFromAlbums />` | Phase 7 | A |
| `portal/resources/views/v1/components/popup/addBook.blade.php` | Modal `<AddContentDialog />` (uploader generic cho book/movie/v.v.) | Phase 4 | A |
| `portal/resources/views/v1/partials/hellopreloader.blade.php` | Component `<RouterLoader />` (intercept transition route) | Phase 0 | A |
| `portal/resources/views/v1/partials/goToTop.blade.php` | Component `<ScrollToTopButton />` | Phase 0 | A |
| `portal/resources/views/errors/{401,403,404,419,429,500,503}.blade.php` | `/app/error.tsx`, `/app/not-found.tsx`, `/app/global-error.tsx` | Phase 0 | A |
| `portal/document/anh1.png` (admin group) | `/t/{tenant}/(admin)/groups/page.tsx` + detail | Phase 4 | A |
| `portal/document/anh2.png` (admin policy) | `/t/{tenant}/(admin)/policies/page.tsx` + detail | Phase 4 | A |
| `portal/document/anh3.png` (modal create-group, search policy) | Modal trên page group / policy | Phase 4 | A |

### 6.2 Page social (từ `template-main/social/`)

Map vào subsection §9 của feature.md. Inventory per-page:

| Template HTML | Route Next.js | Ref feature.md | Phase |
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
| `Weather Widget.html`, `Sticky Sidebars.html` | Slot widget của component `<RightSidebar />` | §9.11 | 7 |
| `Manage Widgets.html` | `account/widgets/page.tsx` | §9.11 | 7 |
| `Music And Playlists.html` | `(music)/page.tsx` | §5 | 4 |
| `Post Versions.html` | Tham chiếu cho render variant post-type; inform design `<PostCard />` | §9.1 | 7 |
| `Blog V1 Grid.html` | `(marketing)/blog/page.tsx` | §13 | 9 |
| `Landing Page.html` | `/page.tsx` (marketing root) | §13 | 9 |
| `Your Account - Account Settings.html`, `Change Password.html`, `Personal Information.html`, `Education And Employement.html`, `Hobbies And Interests.html`, `Tabs Version.html` | `account/(tabs)/...` | §9.2, §12 | 7 |
| `Olympus Shortcodes.html`, `Theme Icons.html` | Entry component library Storybook | — | 0 |
| `Page Without Left Panel.html`, `Page Without Right Panel.html` | Variant layout cho page đặc biệt (live stream, reels) | §9.24, §9.25 | 10 |

### 6.3 Vertical media + bank (không có template — design from scratch)

| Route | Highlight component | Phase |
|---|---|---|
| `(movies)/movies/page.tsx` | Hero rail + grid genre + continue rail | 3 |
| `(movies)/movies/[id]/page.tsx` | Player Vidstack + list episode + rating | 3 |
| `(music)/page.tsx` | Grid track + bar now-playing + editor playlist | 4 |
| `(stories)/page.tsx` | Grid library + rail recently-read + reader chapter | 4 |
| `(comics)/page.tsx` | Grid cover + selector chapter + reader multi-mode | 4 |
| `(bank)/accounts/page.tsx` | List account với balance + flow add-account | 5a |
| `(bank)/transactions/page.tsx` | Infinite scroll + chip filter (category, date, account) + bulk-edit | 5a |
| `(bank)/investments/page.tsx` | Table holdings + cost-basis + breakdown lot | 5e |
| `(bank)/reports/page.tsx` | Chart net-worth + Sankey cash-flow + breakdown per-category | 5g |
| `(creator)/studio/page.tsx` | Earnings tip + subscriber count + payout gần nhất | 11 |

---

## 7. Component library

Build với **Radix UI primitives + Tailwind** làm base. Không Material UI / AntD — chúng fight với Tailwind và thêm bundle weight.

### 7.1 Layer primitive

| Component | Source | Mục đích |
|---|---|---|
| Button, Input, Textarea, Select, Checkbox, Switch | Radix UI + Tailwind | Control form |
| Dialog, Sheet, Drawer, Popover, Tooltip, Toast | Radix UI | Floating UI |
| DropdownMenu, Tabs, Accordion | Radix UI | Compound |
| Avatar, Badge, Skeleton, Spinner | Custom (Tailwind) | Display |

Wrap mỗi cái trong `frontend/src/components/ui/` để centralize class tailwind.

### 7.2 Component feature

```
frontend/src/components/
├── ui/                      ← primitive
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
│   ├── HLSPlayer.tsx        ← wrapper Vidstack
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
│   ├── MoneyDisplay.tsx     ← dùng formatMoney + locale
│   ├── CategoryPicker.tsx
│   ├── BudgetProgress.tsx
│   └── NetWorthChart.tsx    ← Recharts hoặc Visx
├── admin/
│   ├── PolicyEditor.tsx
│   ├── GroupTree.tsx
│   └── ModQueue.tsx
└── forms/
    ├── FormField.tsx        ← wrapper RHF + Zod
    └── DateRangePicker.tsx
```

### 7.3 Charting

Report bank + analytics admin cần chart. Chọn:

| Library | Bundle | Khi nào |
|---|---|---|
| **Recharts** | ~80kb | Nhanh + composable. Default. |
| **Visx** | tree-shake từ 0 | Viz phức tạp (Sankey, custom interactive). Phase 5g+. |
| **D3** | direct | Chỉ nếu Visx quá high-level. Tránh. |

### 7.4 Storybook

Optional nhưng valuable: `@storybook/react-vite`. Story per component; visual regression qua Chromatic nếu ngân sách cho phép. Defer cho tới khi component library có ~30 piece.

---

## 8. Budget performance & accessibility

Set budget tường minh trong `next.config.ts` + check CI.

### 8.1 Budget performance

| Metric | Budget | Tool |
|---|---|---|
| First Contentful Paint (FCP) | < 1.5 s | Lighthouse CI |
| Largest Contentful Paint (LCP) | < 2.5 s | Lighthouse CI |
| Interaction to Next Paint (INP) | < 200 ms | Lighthouse CI |
| Cumulative Layout Shift (CLS) | < 0.1 | Lighthouse CI |
| Initial JS bundle | < 200 KB compressed | `@next/bundle-analyzer` |
| Per-route JS thêm | < 100 KB | Bundle analyzer |

### 8.2 Budget accessibility

- Target WCAG 2.1 AA.
- Mọi element interactive có keyboard support và focus ring visible.
- Mọi image có `alt` text (D-7 làm alt cũng i18n-searchable).
- Control form có `<label>` associated.
- Color contrast ≥ 4.5:1 cho text body, ≥ 3:1 cho text lớn.
- Attribute ARIA chỉ khi semantic HTML không express được role.
- Check tự động: `axe-core` qua Playwright trong CI.

### 8.3 Pattern loading

- **Boundary `<Suspense>`** ở mức catalogue / detail / shell — show skeleton, không phải spinner.
- **Strategy image:** Next.js `<Image>` với `loading="lazy"` cho mọi media + placeholder có size. AVIF/WebP qua Sharp.
- **Prefetch:** `<Link prefetch>` cho nav top-level. Hover prefetch cho card content.
- **HTML streaming** qua RSC cho page catalogue + newsfeed — paint đầu tiên trong khi data còn loading.

---

## 9. Migration asset từ `template-main/`

Lấy cái gì, bỏ cái gì.

### 9.1 Lấy

- **Structure page / IA** — thứ tự section, widget sidebar, layout header. Re-implement trong React; không copy HTML.
- **Iconography** — `template-main/portal/public/v1/ico/` (SVG sprite) → migrate sang single sheet `<symbol>` hoặc equivalent `lucide-react`.
- **Palette màu + scale spacing** — extract từ `bootstrap.css`/`main.min.css` vào `tailwind.config.js`.
- **Concept page-loader** — pattern `hellopreloader` (overlay page-transition).
- **Mock anh1/anh2/anh3.png** — source-of-truth design cho admin (group/policy management). Chưa implement.

### 9.2 Bỏ

- **jQuery, Bootstrap JS, selectize, magnific-popup, isotope, daterangepicker** — thay bằng Tailwind + Radix + lib headless.
- **CKEditor 5** — thay bằng **Tiptap** (React-native, modular hơn) cho composer long-form article.
- **Swiper** — giữ nếu cần cho carousel; hoặc dùng `embla-carousel-react`.
- **Mọi Blade template** — re-architect như component server/client React.
- **Mọi file `.css` từ template** — extract chỉ palette; rewrite như class utility Tailwind.

### 9.3 Asset image

- Avatar / placeholder từ `template-main/social/img/` — usable như fixture dev; thay bằng content CDN thực ở prod.
- Logo: cần redesign — `template-main/social/img/logo.png` hiện tại branded "Olympus".
- **Storage origin (đã chốt):** media bytes lưu ở **MinIO trỏ vào folder local `./data/minio` lúc dev**, và **Cloudflare R2 ở prod**. Cả hai đều nói giọng S3, app đọc `S3_*` cho cả hai — lên live chỉ đổi `.env`, không sửa code (xem [architecture/04-storage-tier-budget.md](architecture/04-storage-tier-budget.md)). Frontend dựng URL media từ S3/R2 endpoint đã cấu hình; tối ưu ảnh ở prod qua Cloudflare Image Resizing trên R2.

### 9.4 Không auto-port

Đừng viết script convert Blade → React. Re-architect manual đảm bảo không carry forward layering jQuery/Bootstrap. Dùng template như tham chiếu visual ở monitor thứ hai trong khi build from scratch.

---

## 10. Deliverable frontend phase-by-phase

### Phase 0 — Foundation

- Wire root layout: provider (TanStack, NextIntl, Theme, Toast), font, global CSS.
- **API client server-only** (`api-server.ts`) theo [D-34]; **mutation client** (`api-client.ts`) cho client component.
- **Route refresh-and-return** `/auth/refresh-and-return/page.tsx`.
- **Generated TS type** từ OpenAPI → `frontend/src/lib/types.gen.ts`.
- **Flow OIDC login** — `/auth/login/page.tsx`, `/auth/callback/page.tsx`.
- **Auth context** — đọc `users.locale`, `users.timezone`, tenant hiện tại qua RSC.
- **Doc convention `frontend/CLAUDE.md`** ([D-32, D-33]) với ví dụ anti-pattern.
- **Error page** (`error.tsx`, `not-found.tsx`, `global-error.tsx`) styled.
- **Khởi động component library** — `<Button />`, `<Dialog />`, `<Toast />`, `<Avatar />`, `<TopHeader />`, `<LeftSidebar />`, `<RightSidebar />`.

**Exit:** sign in qua Authentik, thấy page home đã authenticate với shell sidebar. Route RBAC-gated trả 403 → toast display nó.

### Phase 1 — URL prefix tenant

- Move mọi route đã authenticate dưới `/t/[tenant]/...`.
- `TenantProvider` đọc tenant slug từ URL param.
- Switcher tenant trong `<TopHeader />` (chỉ nếu user có nhiều membership).

**Exit:** `/t/me/...` và `/t/{orgSlug}/...` đều work; switching reload context tenant.

### Phase 2-3 — Media + Movies

- Wrapper Vidstack `<HLSPlayer />`.
- Catalogue + detail + player movie.
- `<PosterCard />`, `<ContinueRail />`.
- Upsert watch-progress (debounce; commit mỗi 10 giây + on pause).

### Phase 4 — Music + Stories + Comics

- Lặp pattern Phase 3 per vertical.
- `<MusicPlayer />` (bar bottom persistent), `<PlaylistEditor />`.
- Component reader story / comic.
- Aggregator `/api/v1/continue` → rail "Continue" thống nhất trên home dashboard.

### Phase 5 — Bank

- Type money: `<MoneyDisplay />`, `<MoneyInput />` (string-amount-aware).
- List transaction với infinite scroll + chip filter.
- Chart net-worth, Sankey cash-flow.
- UX step-up auth wire vào mọi op huỷ diệt bank (xoá account, export).
- Flow MFA enrollment nếu user thiếu `amr=mfa` ([D-28]).

### Phase 6 — Notifications

- `<NotificationsBellDropdown />`.
- **SSE client** subscribe `/api/v1/events/stream`; push notification mới vào cache TanStack (mutate cache trực tiếp, không refetch).
- Subscription Web Push qua Service Worker; ask permission chỉ sau khi user opt in qua settings.

### Phase 7 — Social baseline

- Newsfeed, profile, friend, community, event, messaging.
- `<PostComposer />` (text + image + video upload + poll + draft + schedule).
- `<PostCard />` render mọi post-type.
- `<ChatThread />` với WebSocket qua `platform/realtime/`.
- UI privacy control; selector visibility per-post.

### Phase 8 — Search

- `<CommandPalette />` search bar global (Cmd+K).
- Typeahead qua `useDeferredValue` + debounce 150ms.
- Category result render như section.

### Phase 9 — Marketing microsite

- Page marketing static-render.
- Reader blog.
- Checkout merch optional (defer).

### Phase 10 — Social nâng cao

- **Ring stories** trên top feed.
- **Reels** feed vertical-scroll với auto-play `IntersectionObserver`.
- **Player live stream** + overlay chat.
- Composer long-form article (**Tiptap**).
- UI voting + karma (Reddit-style).
- Feed "For You" với popover `<WhyAmISeeingThis />` ([D-35]).

### Phase 11 — Creator economy

- Dialog tip với step-up auth.
- List subscriber, dashboard payout.
- OAuth handoff Stripe Connect Express cho onboarding creator.

### Phase 12 — Marketplace + call + safety

- UI listing marketplace.
- UI WebRTC call qua LiveKit JS SDK ([D-39]).
- Dashboard trust & safety cho `superadmin`.
- Flow export GDPR (request, status, download).

---

## 11. Anti-pattern (không làm)

- **`'use client'` trên một page chỉ để dùng một hook.** Extract piece hook-using thành island client nhỏ; giữ page là RSC.
- **Lưu server data trong Zustand.** Dùng TanStack (xem §3.1). Quy tắc cứng.
- **Gọi `fetch` trực tiếp từ server component.** Luôn dùng `apiFetch` từ `api-server.ts` — nó forward cookie + handle 401.
- **Gọi `fetch` trực tiếp từ client component.** Luôn dùng `apiMutate` từ `api-client.ts` — nó handle redirect 401.
- **Đọc `process.env.SECRET_*` trong client component.** Chỉ `NEXT_PUBLIC_*` được phép rời server. Secret server-only sống trong server component / route handler.
- **Pre-format money hoặc date trên server.** Luôn send raw ISO 8601 / `{amount, currency}`. Frontend format theo locale + timezone user. ([D-7])
- **Import HTML template Blade.** Re-architect như React. Template là tham chiếu, không phải source.
- **Một mega-store trong Zustand.** Nhiều store nhỏ per concern.
- **`useState` cho state filter / pagination.** Dùng URL query param (`useSearchParams`) — back button work, URL chia sẻ được.
- **Optimistic update không có rollback.** Mỗi `useMutation` với `onMutate` phải có `onError` rollback.
- **Thiếu boundary `<Suspense>` trên page RSC streaming.** Page render trống nếu data chưa ready. Wrap server component async.
- **String tiếng Anh hardcode trong component.** Luôn `t('key')` qua next-intl, kể cả "OK", "Cancel" — được translate tự động.

---

## 12. Open question (frontend-specific)

Những cái này không block nhưng mỗi cái cần answer khi phase liên quan mở:

1. **Component library: build vs adopt?** Radix + Tailwind là plan; shadcn/ui là accelerator styled-Radix đáng evaluate trong Phase 0.
2. **Pick library chart cuối.** Recharts default; revisit Visx khi Phase 5g (report bank) mở.
3. **Editor rich-text cho article.** Tiptap proposed (Phase 10). Confirm trước khi composer article ship.
4. **Experience mobile.** PWA-first theo [D-6]; manifest + service worker + prompt add-to-home-screen land Phase 7 (khi social làm mobile usage compelling).
5. **Framework test end-to-end.** Playwright default; Cypress là alternate. Playwright recommended cho parity với integration axe-core.
6. **Strategy bundle splitting.** Splitting per-route là default. Revisit nếu route nào vượt budget 200 KB JS.
7. **System theme.** CSS variable + support theme native Tailwind v4. Confirm list token trước khi component library mature.
8. **Image CDN / storage origin.** *Đã chốt:* dev = MinIO trên folder local `./data/minio`; prod = Cloudflare R2 (S3-compatible). Chuyển đổi chỉ bằng `.env` (`S3_*`), không sửa code — xem [architecture/04-storage-tier-budget.md](architecture/04-storage-tier-budget.md). Tối ưu ảnh ở prod qua Cloudflare Image Resizing trên R2; `next/image` self-hosted là fallback.

---

## Doc này liên quan thế nào với cái khác

- **[feature.md](feature.md)** — design system-wide; doc này mở rộng slice `frontend/`.
- **[diagrams.md](diagrams.md)** — kiến trúc trực quan; xem sơ đồ "System landscape" cho vị trí của frontend trong hệ thống, "Authenticated request flow" cho path cookie.
- **[archivetech.md](archivetech.md)** — mock UI anh1/2/3 reference trong §6.1 page admin.
- **[CLAUDE.md](../../CLAUDE.md)** — convention backend; `frontend/CLAUDE.md` (deliverable Phase 0) sẽ mirror tone của nó.
