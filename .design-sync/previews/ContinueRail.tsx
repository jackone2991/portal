import { ContinueRail, DSQuerySeed } from "portal-frontend";

// "Continue Watching" rail for the library home — a horizontally scrolling row of
// in-progress items with a poster and a progress bar.
//
// It takes no props and reads ["continue-items"] itself, returning `null` when
// the list is empty. Against no API that is a literally invisible card, so the
// preview seeds the cache it queries.

const items = [
  {
    module: "movie" as const,
    ref_id: "11111111-1111-4111-8111-111111111111",
    title: "Dune: Part Two",
    progress_pct: 62,
    href: "/library/movie/11111111-1111-4111-8111-111111111111",
    poster_url: null,
    updated_at: "2026-08-23T20:14:00Z",
  },
  {
    module: "comic" as const,
    ref_id: "22222222-2222-4222-8222-222222222222",
    title: "Vạn Cổ Thần Đế — Chương 412",
    progress_pct: 18,
    href: "/library/comic/22222222-2222-4222-8222-222222222222",
    poster_url: null,
    updated_at: "2026-08-24T07:02:00Z",
  },
  {
    module: "music" as const,
    ref_id: "33333333-3333-4333-8333-333333333333",
    title: "Bản Tình Ca Mùa Đông",
    progress_pct: 91,
    href: "/library/music/33333333-3333-4333-8333-333333333333",
    poster_url: null,
    updated_at: "2026-08-22T11:40:00Z",
  },
];

export const Rail = () => (
  <DSQuerySeed seed={[[["continue-items"], { items }]]}>
    <div data-template="v1" style={{ background: "#0f1115", padding: 16 }}>
      <ContinueRail />
    </div>
  </DSQuerySeed>
);
