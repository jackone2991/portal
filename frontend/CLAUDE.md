# frontend/CLAUDE.md

Guidance for Claude Code (and humans) working in `frontend/`. Read the repo-root
[CLAUDE.md](../CLAUDE.md) first for the overall architecture; this file is the
**frontend conventions contract** — the state-ownership boundary ([D-32]) and the
RSC rendering decision tree ([D-33]). The prose spec is
[docs/architecture/frontend.md](../docs/architecture/frontend.md).

## Stack

Next.js 15 (App Router, RSC) · TypeScript · Tailwind v4 · TanStack Query · Zustand
· React Hook Form *(NOT INSTALLED — not in package.json, zero imports; every form hand-rolls `useState`. Either add the dependency or drop this row.)* · Vidstack (HLS). Presentation lives in a version-switched
`src/templates/v{N}/` tree selected by `NEXT_PUBLIC_TEMPLATE_VERSION` via
`templates/registry.ts` — read [src/templates/README.md](src/templates/README.md)
before adding a page or cutting a `v2`.

## State ownership boundary [D-32]

Every piece of state has exactly one owner. Pick by category — do not mix.

| State category | Owner | Examples |
| --- | --- | --- |
| **Server state** | TanStack Query | asset list, current user (`/auth/me`), any API-fetched data |
| **UI state (persistent)** | Zustand + `persist` | theme, sidebar collapsed, layout density |
| **UI state (ephemeral)** | Zustand (transient) | active toast, command-palette open, current modal |
| **Form state** | React Hook Form | any form draft before submit |
| **Shareable filter / pagination** | URL query params (read by TanStack) | `?page=2&sort=date&genre=action` |

### Hard rule: no server data in Zustand

If a value came from the API, TanStack Query owns it. A Zustand store must never
hold API-fetched data — that path leads to manual cache-sync, races, and stale
reads.

```ts
// ❌ WRONG — server data smuggled into a Zustand store
const useAssets = create<{ assets: Asset[]; load: () => Promise<void> }>((set) => ({
  assets: [],
  load: async () => set({ assets: await api.listAssets() }), // manual sync, races, staleness
}));

// ✅ RIGHT — TanStack owns server state; it handles caching, refetch, invalidation
function useAssets() {
  return useQuery({ queryKey: ["assets"], queryFn: () => api.listAssets() });
}
```

If you catch yourself writing `setX(await fetch(...))` into a store, stop and
reach for `useQuery` / `useMutation` instead.

## Cursor lists: `useInfiniteQuery` + the scroll sentinel

Every list endpoint in this API is keyset-paginated: it answers with a page plus
a `next_cursor`, and **the page default is 30** (`defaultLimit` in each module's
`types.go`; `limit` is capped at 50). A plain `useQuery` against one of these
silently truncates — the view renders exactly 30 rows and has nothing to say
about the rest, which reads as "the feature only loaded some of my data".

So: any list backed by a `next_cursor` uses `useInfiniteQuery`, and loads the
next page on scroll via `useInfiniteScroll` (`src/lib/use-infinite-scroll.ts`)
rather than a "load more" button. Two details in that hook are easy to get wrong
and are commented there — it rebuilds the observer after every page (an
`IntersectionObserver` reports *changes*, so a list shorter than the viewport
would otherwise stall one page in), and it fetches ahead of the fold.

Two things that follow from paginating, and are the usual bugs:

- **A "select/play/export all" action cannot use the rendered array** — that is
  only the pages fetched so far. Walk the cursor to the end first (see
  `fetchAllTracks` in `src/lib/music.ts`) and bound it, or the button's name is a
  lie the moment the list is longer than one page.
- **Show the loaded count with a `+` while more remain.** "30" on its own reads
  as the whole library.

## Rendering: RSC-first [D-33]

Next.js App Router is server-component-first. Reflexive `'use client'` everywhere
forfeits SEO, bundle savings, and streaming. Default to server components; opt into
`'use client'` only where interactivity actually requires it.

| Surface | Mode | Why |
| --- | --- | --- |
| Catalogue (movie/music/story/comic lists) | Server components | SEO; streamed HTML; personalise via `cookies()` |
| Detail pages (single movie/track) | Server shell + client island | Metadata is SEO-relevant; the player must be client |
| Player / reader | Mostly client | Stateful, post-auth, SEO-irrelevant |
| Account settings, bank | Server shell + client islands | Interactive but private; ergonomic to fetch server-side |
| Newsfeed (Phase 7) | Client primary; SSR first page | Highly interactive; realtime updates |

Practical rules:

- Public catalogue pages use `next.revalidate` (ISR); per-user data uses `cache: 'no-store'`.
- Server components fetch through the Portal API over the Docker network — same-region latency is fine for SEO pages.
- Push `'use client'` to the leaf that needs it, not the whole page.

## Auth handoff [D-34]

Routes are gated by `src/middleware.ts` on the durable `portal_session` marker
cookie (the tight-path `portal_refresh` cookie is invisible to middleware).
`SessionKeeper` does client-side silent access-token refresh (interval + focus,
multi-tab throttled). The server-only API client (`src/lib/api-server.ts` with
`import "server-only"` + `cookies()` forwarding) is future work — today only
`src/lib/api-client.ts` (browser, `credentials: 'include'`) exists.

> The older RSC `refresh-and-return` route design was **superseded** by
> `SessionKeeper` — see D-34.r1. Don't reintroduce it.

## Generated code — do not hand-edit

`src/lib/types.gen.ts` is generated from [shared/openapi.yaml](../shared/openapi.yaml)
via `pnpm openapi` (or `make openapi`). Edit the spec, regenerate, then use the
types — never hand-edit the generated file.

## Commands (from `frontend/`)

| Command | What |
| --- | --- |
| `pnpm dev` | Dev server (Turbopack) |
| `pnpm build` | Production build (runs typecheck + ESLint) |
| `pnpm test` | Vitest |
| `pnpm openapi` | Regenerate `src/lib/types.gen.ts` from the OpenAPI spec |
