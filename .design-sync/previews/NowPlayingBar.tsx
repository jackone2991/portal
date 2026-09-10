import { MusicPlayerProvider, NowPlayingBar, useMusicPlayerOptional } from "portal-frontend";
import { useEffect } from "react";

// The docked player bar. It renders nothing until the provider has a current
// track, and the ONLY ways to give it one — playQueue / playTrack / jumpTo — all
// start playback. In a preview the audio URL resolves to nothing, `play()`
// rejects, and the provider sets "Playback was blocked", so the card would show
// a red error banner and teach that the player is broken.
//
// So the preview neutralises playback itself, in two places — both were needed,
// and finding the second cost a capture round:
//   1. `play()` resolves instead of rejecting (no "Playback was blocked").
//   2. the <audio> src is swapped for a tiny silent WAV, because the element
//      ALSO fires `error` on a src that 404s, which is a different branch and
//      paints "This track could not be played."
// Nothing about the component changes; it just isn't punished for living in a
// page with no media server. Same family of hack as GoToTop's forced visibility.
//
// If MusicPlayerProvider ever grows a seed-without-playing action, delete this.

const SILENCE = "data:audio/wav;base64,UklGRkQDAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YSADAACAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgA==";

if (typeof HTMLMediaElement !== "undefined") {
  HTMLMediaElement.prototype.play = function () {
    return Promise.resolve();
  };
  const desc = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "src");
  if (desc?.set) {
    Object.defineProperty(HTMLMediaElement.prototype, "src", {
      ...desc,
      set(v: string) {
        desc.set!.call(this, /\/api\/v1\/assets\//.test(v) ? SILENCE : v);
      },
    });
  }
  const origSet = HTMLMediaElement.prototype.setAttribute;
  HTMLMediaElement.prototype.setAttribute = function (name: string, value: string) {
    return origSet.call(this, name, name === "src" && /\/api\/v1\/assets\//.test(value) ? SILENCE : value);
  };
}

const track = (id: string, title: string, artist: string) => ({
  id,
  owner_id: "u1",
  title,
  artist,
  album: null,
  description: null,
  audio_asset_id: `asset-${id}`, // non-null ⇒ isPlayable(); the URL never resolves
  cover_asset_id: null,
  status: "published" as const,
  created_at: "2026-09-01T10:00:00Z",
  updated_at: "2026-09-01T10:00:00Z",
  release_year: null,
  genre: null,
  mb_recording_id: null,
  lookup_status: "none",
  lookup_note: null,
  lookup_at: null,
});

const queue = [
  track("1", "Ông Bà Anh", "Lê Thiện Hiếu"),
  track("2", "Đừng Quên Tên Anh", "Hoa Vinh"),
  track("3", "Ừ Có Anh Đây", "Tino"),
];

function Seed({ at }: { at: number }) {
  const player = useMusicPlayerOptional();
  useEffect(() => {
    player?.playQueue(queue, at);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return null;
}

const stage = (at: number) => (
  <MusicPlayerProvider>
    <div data-template="v1" style={{ position: "relative", transform: "translateZ(0)", height: 220, background: "var(--tpl-bg)" }}>
      <Seed at={at} />
      <NowPlayingBar />
    </div>
  </MusicPlayerProvider>
);

export const Playing = () => stage(0);

// Mid-queue: the same bar, a different track — what the previous/next controls
// move between.
export const MidQueue = () => stage(1);
