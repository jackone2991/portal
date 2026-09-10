import { ContinueWidget, DSQuerySeed } from "portal-frontend";

// The compact "keep going" list for the home rail — the sidebar sibling of
// ContinueRail. Queries ["continue","home"] itself and renders nothing when the
// list is empty, so the preview seeds it.

const items = [
  { module: "movie" as const, ref_id: "m1", title: "Dune: Part Two", progress_pct: 62, href: "/library/movie/m1", poster_url: null, updated_at: "2026-09-08T20:14:00Z" },
  { module: "comic" as const, ref_id: "c1", title: "Vạn Cổ Thần Đế — Chương 408", progress_pct: 18, href: "/library/comic/c1", poster_url: null, updated_at: "2026-09-09T07:02:00Z" },
  { module: "story" as const, ref_id: "s1", title: "Đắc Nhân Tâm", progress_pct: 91, href: "/library/story/s1", poster_url: null, updated_at: "2026-09-07T11:40:00Z" },
];

const frame = (seed: unknown, children: React.ReactNode) => (
  <DSQuerySeed seed={[[["continue", "home"], seed]]}>
    <div data-template="v1" style={{ width: 320, background: "var(--tpl-bg)", padding: 12 }}>{children}</div>
  </DSQuerySeed>
);

export const Widget = () => frame({ items }, <ContinueWidget />);

// One item in progress — the ordinary case for someone reading a single series.
export const SingleItem = () => frame({ items: items.slice(1, 2) }, <ContinueWidget />);
