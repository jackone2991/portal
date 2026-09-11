import { DSQuerySeed, MediaDetailView } from "portal-frontend";

// Video playback screen (SPEC-01): a Vidstack player with the default video
// layout, plus the resume/progress plumbing.
//
// It reads ["assets", id] and ["assets", id, "progress"] itself and shows
// "Asset not found" against no API, so both are seeded. The HLS URL below does
// not resolve in a preview — the player renders its own chrome, which is the
// part of this screen a design is actually composed against.

const id = "44444444-4444-4444-8444-444444444444";

const asset = {
  id,
  status: "ready" as const,
  kind: "video" as const,
  mime_type: "video/mp4",
  size_bytes: 734003200,
  duration_ms: 5_460_000,
  width: 1920,
  height: 1080,
  created_at: "2026-08-01T10:12:00Z",
  hls_url: `https://media.portal.localhost/hls/${id}/master.m3u8`,
  title: "Chuyến đi Đà Lạt 2026",
  original_filename: "dalat-2026.mp4",
};

export const Player = () => (
  <DSQuerySeed
    seed={[
      [["assets", id], asset],
      [["assets", id, "progress"], { position_ms: 0, updated_at: "2026-08-01T10:12:00Z" }],
    ]}
  >
    <div data-template="v1" style={{ background: "#0f1115" }}>
      <MediaDetailView id={id} />
    </div>
  </DSQuerySeed>
);
