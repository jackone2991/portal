# Portal — Kiến trúc Frontend

Tài liệu đi kèm [feature.md](feature.md). Trong khi `feature.md` định nghĩa toàn bộ hệ thống, tài liệu này tập trung vào app **`frontend/`** Next.js 15: cấu trúc, state, rendering, auth handoff, và page inventory được build từ `template-main/`.

Đọc [§16 Frontend](feature.md) trong feature.md trước để nắm các quyết định ở tầm cao ([D-32], [D-33], [D-34] — đã bị thay thế bởi SessionKeeper phía client, xem banner ở §4 — và [D-7]); tài liệu này mở rộng chúng với các pattern cụ thể và roadmap build-out.

> **Trạng thái (2026-07-06).** Vòng demo v1 đã khép kín và được commit: đăng nhập bằng mật khẩu local → home đã authenticate → upload mp4 (`/upload` Vidstack studio) → MinIO(dev)/R2(prod) → worker transcode HLS → playback → logout có thể revoke (tracker sống: `MILESTONE_CHECKS.md`). Theo [ADR-06](architecture/06-local-auth-model.md), Authentik/OIDC đã bị gỡ bỏ hoàn toàn — mọi nhắc đến OIDC/callback/Authentik bên dưới chỉ còn tính lịch sử. Thiết kế refresh-and-return ở §4 đã bị thay thế bởi `SessionKeeper` phía client. Route tree ở §2.1 và các phase ở §6/§10 là mục tiêu dài hạn, không phải scope v1 (xem [architecture/01-v1-scope-cut.md](architecture/01-v1-scope-cut.md)).

---

## 1. Trạng thái hiện tại

```
frontend/
├── Dockerfile
├── next.config.ts
├── package.json
├── postcss.config.mjs
├── tsconfig.json                     ← @/* → src/*
└── src/
    ├── middleware.ts                 ← auth gate: portal_session marker → redirect guests to /login (matcher: /, /login, /register, /upload, /library/*)
    ├── app/                          ← ROUTING ONLY (thin); views resolve via the registry
    │   ├── globals.css
    │   ├── layout.tsx                ← root: <html>, Providers, active-template theme import
    │   ├── providers.tsx            ← TanStack Query client ('use client')
    │   ├── (public)/                 ← guest shell (MasterPublic)
    │   │   ├── layout.tsx
    │   │   ├── login/page.tsx
    │   │   └── register/page.tsx
    │   └── (app)/                    ← authenticated shell (MasterBase)
    │       ├── layout.tsx
    │       ├── page.tsx              ← "/" home / newsfeed
    │       ├── upload/page.tsx       ← v1 upload → transcode → HLS playback studio
    │       └── library/
    │           ├── comic/page.tsx
    │           └── novel/[id]/page.tsx
    ├── templates/                    ← VERSIONED presentation layer (see §1.1)
    │   ├── types.ts                  ← TemplateManifest contract
    │   ├── registry.ts               ← single version-switch point (env NEXT_PUBLIC_TEMPLATE_VERSION)
    │   ├── README.md
    │   └── v1/                       ← "Olympus" theme, ported from template-main
    │       ├── index.ts              ← v1 manifest
    │       ├── theme/theme.css
    │       ├── master/{MasterBase,MasterPublic}.tsx
    │       ├── components/headers/{TopMenu,NotifMenus}.tsx   ← NotifMenus = header dropdowns (static placeholder data)
    │       ├── components/{menu,footers,popup}/*
    │       ├── components/ui/{Avatar,Icon}.tsx
    │       ├── partials/{HelloPreloader,GoToTop,SessionKeeper}.tsx  ← SessionKeeper = client-side silent token refresh (§4 banner)
    │       └── views/
    │           ├── home/HomeView.tsx
    │           ├── auth/{LoginView,RegisterView,AuthForm,AuthLanding}.tsx
    │           ├── upload/UploadStudio.tsx   ← Vidstack studio: presigned upload → poll status → HLS playback
    │           └── library/{comic,novel}/*
    └── lib/
        └── api-client.ts             ← working fetch wrapper (credentials: include, throws ApiError); gets typed via types.gen.ts once `make openapi` runs — D-34 server-client design superseded, see §4 banner
```

Trạng thái (2026-07-06): **vòng demo v1 đã khép kín** — đăng nhập/đăng ký bằng mật khẩu local (ADR-06), auth gate ở middleware trên `portal_session`, silent refresh của `SessionKeeper`, và `/upload` Vidstack studio đều hoạt động end-to-end. Các layout shell, sidebar, partial, và view được port từ Blade reference; popup và SVG sprite là placeholder, và widget feed/friends/notification vẫn là placeholder tĩnh. Còn phải làm: TS type generated từ OpenAPI (`types.gen.ts`), API client server-only, i18n, component library Radix.

Stack đã chọn: App Router + RSC, Tailwind v4, TypeScript; Vidstack cho HLS ([§3 Media](feature.md)); Zustand + TanStack Query + React Hook Form cho state ([D-32]); `next-intl` cho i18n ([D-7]).

Hai nguồn visual-design, cả hai đều **chỉ tham chiếu — không phải code active**:

| Nguồn | Tech | Dùng cho |
|---|---|---|
| [template-main/portal/](../../template-main/portal/) | Laravel/Blade + Bootstrap 4 + jQuery | Primitive UI admin/portal: master layout, sidebar, popup, header/menu, sidebar widget, page-loader |
| [template-main/social/](../../template-main/social/) | HTML tĩnh (theme Olympus) | UI sản phẩm social: ~70 page trải khắp newsfeed/profile/friends/communities/events/messaging |

Bản rewrite Next.js viết lại cả hai **bằng React + Tailwind**. CSS/JS gốc không được import. Chỉ structure + pattern tương tác + bố cục asset được tái sử dụng làm tham chiếu.

### 1.1 Lớp template được versioned (implementation)

Toàn bộ UI sống dưới `src/templates/v{N}/`, **mỗi phiên bản template một folder**. Cách này mirror Blade reference tại `template-main/portal/resources/views/v1/`, nơi toàn bộ lớp presentation được namespace theo version để một bản redesign có thể ship dưới dạng `v2/` mà không đụng vào `v1/`.

Quy tắc load-bearing: **`app/` chỉ làm routing.** File route không bao giờ import một version cụ thể — chúng gọi `activeTemplate()` và render bất kỳ shell/view nào mà version đang active cung cấp. URL giữ sạch (`/`, `/login`, `/library/comic`); version là mối quan tâm nội bộ, **không bao giờ là một URL segment**.

```
app/(public)/login/page.tsx ─┐
app/(app)/page.tsx ──────────┼─→ activeTemplate() ──→ registry.ts ──→ templates/v1/index.ts
app/(app)/library/... ───────┘        (env: NEXT_PUBLIC_TEMPLATE_VERSION, default "v1")
```

- **`templates/types.ts`** — hợp đồng `TemplateManifest`: layout `shells` (`public`, `app`) + page `views` (`home`, `login`, `register`, `libraryComic`, `libraryNovelDetail`). Mỗi version implement đúng shape này.
- **`templates/registry.ts`** — điểm switch duy nhất: map version id → manifest, chọn cái active từ `NEXT_PUBLIC_TEMPLATE_VERSION`, throw nếu gặp id không xác định.
- **`templates/v1/index.ts`** — manifest v1 bind các component Olympus vào hợp đồng.

Do đó một file route là một resolver 3 dòng, ví dụ `app/(app)/page.tsx`:

```typescript
import { activeTemplate } from "@/templates/registry";
export default function HomePage() {
  const View = activeTemplate().views.home;
  return <View />;
}
```

Layout resolve shell theo cùng cách: `(public)/layout.tsx` bọc children trong `shells.public`, `(app)/layout.tsx` trong `shells.app` (≈ Blade `master-public` / `master-base`). Theme token là per version (`templates/v1/theme/theme.css`, scoped dưới `[data-template="v1"]`) và được import một lần trong `app/layout.tsx`.

### 1.2 Thêm version mới (vd v2)

1. `cp -r templates/v1 templates/v2`; restyle / rebuild component.
2. Set `version: "v2"` trong `templates/v2/index.ts`.
3. Đăng ký nó: `const REGISTRY = { v1, v2 }` trong `registry.ts`.
4. Chạy với `NEXT_PUBLIC_TEMPLATE_VERSION=v2` và swap theme import trong `app/layout.tsx`.

Không sửa file route `app/` nào; v1 và v2 coexist trong repo vô thời hạn. Đây là cùng ý định như namespace Blade `v1/`, được adapt cho App Router.

### 1.3 Mapping Blade → Next.js (v1)

Nguồn: `template-main/portal/resources/views/v1/` (theme "Olympus" của Crumina).

| Blade | Next.js (`templates/v1/`) |
|---|---|
| `master/master-base.blade.php` | `master/MasterBase.tsx` (app shell: preloader + sidebar + popup + sprite) |
| `master/master-public.blade.php` | `master/MasterPublic.tsx` (guest shell) |
| `components/head/*` (css/js/fonts) | `app/layout.tsx` `metadata` + `theme/theme.css` |
| `components/headers/menu` | `components/headers/TopMenu.tsx` |
| `components/menu/sidebar{Left,Right}` | `components/menu/Sidebar{Left,Right}.tsx` |
| `components/menu/sidebarCenter(+Responsive)` | `components/menu/SidebarCenter.tsx` |
| `components/footers/svg` | `components/footers/SvgSprite.tsx` (placeholder) |
| `components/footers/js` / `ico` | React hooks / favicon `metadata` (không có component) |
| `components/popup/*` | `components/popup/*.tsx` (modal stub → `null` cho tới khi open-state được wire) |
| `partials/hellopreloader` / `goToTop` | `partials/HelloPreloader.tsx` / `GoToTop.tsx` |
| `views/home/home` | `views/home/HomeView.tsx` |
| `public/login` / `register` | `views/auth/{LoginView,RegisterView}.tsx` + `AuthForm.tsx` dùng chung |
| `views/library/commic/index` | `views/library/comic/ComicIndexView.tsx` (đã sửa lỗi chính tả "commic") |
| `views/library/novel/detail` | `views/library/novel/NovelDetailView.tsx` |

### 1.4 Quan hệ với route tree mục tiêu (§2.1)

Route tree đầy đủ ở §2.1 (`/t/{tenant}/(app)/...`, marketing, admin, mọi vertical) là **mục tiêu dài hạn**. Các route được implement hôm nay là tập con bị cắt cho v1, và prefix tenant `/t/[tenant]` ([D-23]) bị deferred. Vì routing resolve qua registry, **hình dạng URL cuối cùng và version template độc lập với nhau** — thêm `/t/[tenant]` sau này, hoặc đổi sang `v2`, không đụng tới `templates/` lẫn resolver pattern.

Hai điểm cần đối chiếu với các section sau:

- **Components vs. templates.** §7 mô tả một library primitives/feature cross-version dưới `src/components/` (Radix + Tailwind). Lớp đó dành cho building block dùng chung, không phụ thuộc version; `src/templates/v{N}/` compose chúng (cộng với markup riêng của version) thành shell và view mà một design version cụ thể ship ra. Primitive nằm trong `components/`, composition riêng-version nằm trong `templates/`.
- **Trang Register.** Đã implement và wire theo local auth [ADR-06](architecture/06-local-auth-model.md): `AuthForm` POST email + password (+ `remember`) tới `POST /api/v1/auth/login`; `RegisterView` POST `POST /api/v1/auth/register` (201, không tạo session, redirect về `/login`). Entry cũ "Authentik handles" ở §6.1 đã retired — CHỈ CÒN local-password auth (xem [CLAUDE.md](../../CLAUDE.md) mục "Account module").

---

## 2. Kiến trúc

### 2.1 Cấu trúc route group

Route group của Next.js App Router (`(name)`) không xuất hiện trong URL — chúng dùng để tổ chức layout.

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
│   ├── ~~callback/page.tsx~~                 ← retired by ADR-06 — no OIDC callback exists
│   ├── ~~refresh-and-return/page.tsx~~       ← superseded by SessionKeeper (templates/v1/partials/SessionKeeper.tsx)
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

Các route auth v1 đã implement là `/login` và `/register` qua group `(public)` (§1), không phải segment `auth/` này.

**Vì sao shape này:**

- **`/t/{tenant}/...`** là path thực, khớp với quyết định URL prefix tenant của [D-23]. Route dữ liệu cá nhân dùng `/t/me/...`.
- **Route group** `(app)`, `(admin)`, `(marketing)`, `(movies)`, v.v. chỉ tổ chức layout — invisible trong URL.
- **`/auth/*`** KHÔNG nằm trong `/t/{tenant}/` vì login xảy ra trước khi có tenant context.
- **`/search`** cross-tenant (aggregator `D-2`); không lấy prefix.
- Slug tenant `[tenant]` được resolve bởi middleware → inject qua React context (xem §4.2).

### 2.2 Phân cấp layout (3 tầng)

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

**Quy tắc:** layout compose top-down. Layout con không bao giờ override cái layout cha cung cấp — cha wrap con.

### 2.3 Chiến lược rendering theo surface ([D-33])

| Surface | Mode | Vì sao |
|---|---|---|
| Page marketing, blog | RSC full + ISR | SEO; cache qua `next.revalidate` |
| **Catalogue** movie/music/story/comic | RSC + client list-pagination island | SEO; list lớn stream HTML; user-state qua `cookies()` |
| **Detail** movie/music/story/comic | RSC shell + client player island | Metadata là SEO; player cần JS |
| **Player / reader** | Chủ yếu client component | Stateful; post-auth; SEO không liên quan |
| **Newsfeed** | SSR trang đầu + client-paginated cho các trang sau | Initial paint nhanh; load-more là client |
| **Account / bank** | RSC shell + island interactivity client | Interactive nhưng private; ergonomic để fetch server-side |
| **Surface real-time** (chat, live stream, notification live) | Client component | Cần WebSocket / SSE |
| **Form Studio / admin** | Client component | Form state nặng qua React Hook Form |

**Quy tắc cứng:** mặc định dùng RSC. Chỉ `'use client'` khi bạn có:
- Event handler (`onClick`, `onChange`, `onSubmit`)
- React hook (`useState`, `useEffect`, `useReducer`, hook custom)
- Browser API (`window`, `localStorage`, `Notification`, `navigator`)
- Vidstack player hoặc bất cứ thứ gì cần DOM

Nếu một page có cả hai — làm nó RSC với một component island `'use client'` nhỏ cho interactivity. Ví dụ: page detail movie là RSC; chỉ `<MoviePlayer />` và `<RatingForm />` là client.

---

## 3. Quản lý state ([D-32])

Năm loại state trực giao (orthogonal). Mỗi loại có đúng một owner.

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

**Key-factories:** mỗi module có một cái (`moviesKeys`, `bankKeys`, `socialKeys`). Mọi key đi qua factory — không bao giờ inline.

**Stale time:** 1 phút mặc định, 5 phút cho catalogue, 0 (luôn fresh) cho counter cá nhân (unread, notification).

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

> **Đã bị thay thế (2026-07-05).** OIDC đã biến mất ([ADR-06](architecture/06-local-auth-model.md)) và route refresh-and-return bên dưới đã được thay bằng `SessionKeeper` phía client (`templates/v1/partials/SessionKeeper.tsx` — interval 4 phút + refresh khi focus, throttle multi-tab qua `localStorage`, hard redirect về `/login` khi refresh thất bại). §4.2–4.4 được giữ lại như lịch sử thiết kế; API client server-only vẫn là việc cần làm trong tương lai. §4.1 là fact hiện tại.

### 4.1 Scheme cookie

| Cookie | Path | Lifetime | Mục đích |
|---|---|---|---|
| `portal_access` | `/` | 5 phút | Bearer cho API; JWT HS256 |
| `portal_refresh` | `/api/v1/auth` | `REFRESH_TOKEN_TTL` (hiện tại 24h; là session cookie trừ khi `remember=true`) | Mint access mới qua `/api/v1/auth/refresh` |
| `portal_session` | `/` | khớp lifetime của refresh | Marker đăng nhập không nhạy cảm, được Next.js middleware đọc (auth gate cho `/`, `/upload`, `/library/*`); không cần `HttpOnly`-nhạy cảm — không mang token |

Tất cả: `HttpOnly Secure SameSite=Strict` (marker `portal_session` đọc được bởi edge middleware và không giữ secret nào).

**Mandate same-site** ([D-34]): host Next.js + host API PHẢI chia sẻ một registrable domain (eTLD+1), vd `portal.example.com` + `api.portal.example.com`. Setup single-host route qua Traefik theo path.

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

`import "server-only"` khiến file này không thể bundle vào code client — ngăn việc vô tình ship nó sang browser.

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

User thấy một flash navigation duy nhất; không cần re-auth đầy đủ nếu refresh token vẫn còn hợp lệ.

### 4.4 Mutation phía client

Client component POST/PATCH/DELETE qua TanStack Query. Cookie đi kèm nhờ `credentials: 'include'` vì same-site.

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

### 4.5 UX step-up auth ([D-27])

Khi một thao tác nhạy cảm trả về 403 + Problem `auth.step_up_required`:

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
      // Show modal explaining MFA is required, pointing at Portal-native MFA enrollment (later phase — ADR-06)
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

Nút "Manage MFA" trong settings account-security mở MFA enrollment Portal-native (phase sau — [ADR-06](architecture/06-local-auth-model.md) §"New responsibilities"). [D-28] vẫn chi phối yêu cầu step-up, nhưng deep-link sang dashboard Authentik của nó đã bị thay thế — Authentik đã bị gỡ bỏ hoàn toàn.

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
  // "2 minutes ago", "in 3 days" — via Intl.RelativeTimeFormat
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
| `portal/resources/views/v1/public/login.blade.php` | `/login` qua `app/(public)/login/page.tsx` | Phase 0 — done | A |
| `portal/resources/views/v1/public/register.blade.php` | `/register` qua `app/(public)/register/page.tsx` (local auth [ADR-06](architecture/06-local-auth-model.md)) | Phase 0 — done | A |
| `portal/resources/views/v1/views/library/...` | `/t/{tenant}/(app)/(stories)/library/page.tsx` | Phase 4 | A |
| `portal/resources/views/v1/components/menu/sidebarLeft.blade.php` | Component `<LeftSidebar />` | Phase 0 | A |
| `portal/resources/views/v1/components/menu/sidebarRight.blade.php` | Component `<RightSidebar />` | Phase 0 | A |
| `portal/resources/views/v1/components/headers/menu.blade.php` | Component `<TopHeader />` (gồm search bar, bell notification, dropdown friend request, menu user) | Phase 0 | A |
| `portal/resources/views/v1/components/popup/chatResponsive.blade.php` | Component `<ChatDrawer />` (mobile) | Phase 7 | A |
| `portal/resources/views/v1/components/popup/updateHeaderPhoto.blade.php` | Modal `<CoverPhotoEditor />` | Phase 7 | A |
| `portal/resources/views/v1/components/popup/choseFromMyPhoto.blade.php` | Modal `<PhotoPickerFromAlbums />` | Phase 7 | A |
| `portal/resources/views/v1/components/popup/addBook.blade.php` | Modal `<AddContentDialog />` (uploader chung cho book/movie/v.v.) | Phase 4 | A |
| `portal/resources/views/v1/partials/hellopreloader.blade.php` | Component `<RouterLoader />` (intercept chuyển route) | Phase 0 | A |
| `portal/resources/views/v1/partials/goToTop.blade.php` | Component `<ScrollToTopButton />` | Phase 0 | A |
| `portal/resources/views/errors/{401,403,404,419,429,500,503}.blade.php` | `/app/error.tsx`, `/app/not-found.tsx`, `/app/global-error.tsx` | Phase 0 | A |
| `portal/document/anh1.png` (admin nhóm) | `/t/{tenant}/(admin)/groups/page.tsx` + detail | Phase 4 | A |
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

- Wire root layout: provider (TanStack, NextIntl, Theme, Toast), font, global CSS — một phần: provider TanStack + theme template đã wired; NextIntl + Toaster còn pending.
- **Mutation client** (`api-client.ts`) — ĐÃ XONG (fetch wrapper hoạt động); **API client server-only** (`api-server.ts`, [D-34]) vẫn là việc tương lai (xem banner §4).
- **Silent session refresh** — ĐÃ XONG qua `SessionKeeper` (thay thế route refresh-and-return của [D-34]).
- **TS type generated** từ OpenAPI → `frontend/src/lib/types.gen.ts` — pending (file chưa tồn tại; cần chạy `make openapi`).
- **Flow login/register local** — ĐÃ XONG: `(public)/login`, `(public)/register` → `POST /api/v1/auth/login`, `/auth/register` ([ADR-06](architecture/06-local-auth-model.md) đã thay thế deliverable OIDC ban đầu).
- **Auth context** — đọc `users.locale`, `users.timezone`, tenant hiện tại qua RSC — pending.
- **Doc convention `frontend/CLAUDE.md`** ([D-32, D-33]) với ví dụ anti-pattern — pending (chưa tồn tại).
- **Error page** (`error.tsx`, `not-found.tsx`, `global-error.tsx`) đã styled — pending.
- **Khởi động component library** — một phần: template v1 ship `Avatar`, `Icon`, `TopMenu`, sidebar; primitive cross-version `components/ui/` (`<Button />`, `<Dialog />`, `<Toast />`) còn pending.

**Exit (đạt 2026-07-05):** sign in qua Portal `/login` (credentials local), thấy home shell đã authenticate; guest bị middleware redirect về `/login`.

### Phase 1 — URL prefix tenant

- Move mọi route đã authenticate dưới `/t/[tenant]/...`.
- `TenantProvider` đọc tenant slug từ URL param.
- Switcher tenant trong `<TopHeader />` (chỉ nếu user có nhiều membership).

**Exit:** `/t/me/...` và `/t/{orgSlug}/...` đều work; switching reload context tenant.

### Phase 2-3 — Media + Movies

> Đã landed một phần (2026-07-06): playback HLS Vidstack đã hoạt động end-to-end trong `UploadStudio` v1 (`/upload`); các page catalogue / detail / continue-rail bên dưới vẫn thuộc Phase 3.

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
8. **Image CDN / storage origin.** *Đã chốt* — MinIO (`./data/minio`) ở dev, Cloudflare R2 ở prod, chuyển đổi chỉ qua `.env`: xem §9.3 để biết phát biểu canonical.

---

## Doc này liên quan thế nào với cái khác

- **[feature.md](feature.md)** — design system-wide; doc này mở rộng slice `frontend/`.
- **[diagrams.md](diagrams.md)** — kiến trúc trực quan; xem sơ đồ "System landscape" cho vị trí của frontend trong hệ thống, "Authenticated request flow" cho path cookie.
- **[archivetech.md](archivetech.md)** — mock UI anh1/2/3 reference trong §6.1 page admin.
- **[CLAUDE.md](../../CLAUDE.md)** — convention backend; `frontend/CLAUDE.md` (deliverable Phase 0) sẽ mirror tone của nó.
