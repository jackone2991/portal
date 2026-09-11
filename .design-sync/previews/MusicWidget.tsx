import { MusicWidget, DSQuerySeed } from "portal-frontend";

// The home-rail music widget. It queries ["tracks","widget"] itself and takes no
// props, and — like ContinueWidget — it degrades to `null` on a failed query
// rather than showing a broken shell. Previews have no API origin, so every
// query fails: unseeded this card is literally empty, which is exactly what the
// [RENDER_BLANK] warn was reporting. Seeding the cache is the whole fix.

const track = (id: string, title: string, artist: string, album: string | null) => ({
  id,
  owner_id: "u1",
  title,
  artist,
  album,
  description: null,
  audio_asset_id: `asset-${id}`, // non-null ⇒ isPlayable(); the URL never resolves
  cover_asset_id: null,
  status: "published" as const,
  created_at: "2026-08-20T10:00:00Z",
  updated_at: "2026-08-20T10:00:00Z",
  release_year: null,
  genre: null,
  mb_recording_id: null,
  lookup_status: "none",
  lookup_note: null,
  lookup_at: null,
});

const tracks = [
  track("1", "Ông Bà Anh", "Lê Thiện Hiếu", null),
  track("2", "Đừng Quên Tên Anh", "Hoa Vinh", "Đừng Quên Tên Anh (Single)"),
  track("3", "Ừ Có Anh Đây", "Tino", "Ừ Có Anh Đây (Single)"),
  track("4", "Yêu Đơn Phương", "OnlyC, Karik", "Yêu Đơn Phương (Single)"),
  track("5", "Đóa Hoa Hồng", "Chi Pu", "Queen (Single)"),
];

export const Widget = () => (
  <DSQuerySeed seed={[[["tracks", "widget"], { tracks, next_cursor: undefined }]]}>
    <div data-template="v1" style={{ width: 320, background: "var(--tpl-bg)", padding: 12 }}>
      <MusicWidget />
    </div>
  </DSQuerySeed>
);

// One track is the honest small case — the widget is a rail, not a table.
export const SingleTrack = () => (
  <DSQuerySeed seed={[[["tracks", "widget"], { tracks: tracks.slice(0, 1), next_cursor: undefined }]]}>
    <div data-template="v1" style={{ width: 320, background: "var(--tpl-bg)", padding: 12 }}>
      <MusicWidget />
    </div>
  </DSQuerySeed>
);
