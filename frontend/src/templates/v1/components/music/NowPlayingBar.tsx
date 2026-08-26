"use client";

import Link from "next/link";
import type { Route } from "next";
import { AudioPlayer } from "./AudioPlayer";
import { Icon } from "../ui/Icon";
import { useMusicPlayerOptional } from "./MusicPlayerProvider";
import { trackArtist, trackCoverURL } from "@/lib/music";

/**
 * The docked "now playing" bar.
 *
 * Mounted once in `MasterBase`, below the router outlet — it renders nothing
 * until something is actually queued, so pages that never touch music pay no
 * visual cost. It is pure glue: all state comes from `MusicPlayerProvider`, all
 * chrome comes from `AudioPlayer`.
 *
 * The left/right padding mirrors `MasterBase`'s `<main>` so the bar lines up
 * with the content column instead of sliding under the fixed sidebars.
 */
/**
 * Reserves the space the fixed bar covers, so the last rows of a page are never
 * hidden behind it. Rendered in normal flow at the end of the content column;
 * collapses to nothing whenever the bar itself is hidden.
 */
export function NowPlayingSpacer() {
  const player = useMusicPlayerOptional();
  if (!player?.current) return null;
  return <div aria-hidden className="h-24" />;
}

export function NowPlayingBar() {
  const player = useMusicPlayerOptional();

  // No provider (standalone render) or nothing queued → render nothing.
  if (!player || !player.current) return null;

  const {
    current, playing, progressPct, duration, shuffle, repeat, error,
    toggle, next, prev, seekPct, toggleShuffle, cycleRepeat, stop,
  } = player;

  return (
    <div
      className="fixed inset-x-0 bottom-0 z-40 transition-[padding] duration-200 xl:pl-[var(--tpl-sidebar-cur)] xl:pr-[var(--tpl-rightbar-cur)]"
    >
      <div className="mx-auto w-full max-w-[1220px] px-3 pb-3 sm:px-5">
        {error && (
          <p
            className="mb-2 rounded-lg border px-3 py-2 text-xs"
            style={{
              borderColor: "rgba(239,68,68,.4)",
              background: "rgba(239,68,68,.08)",
              color: "#ef4444",
            }}
          >
            {error}
          </p>
        )}

        <div className="flex items-center gap-2">
          <div className="min-w-0 flex-1">
            <AudioPlayer
              track={{ title: current.title, artist: trackArtist(current) }}
              playing={playing}
              progress={progressPct}
              durationSec={duration}
              coverUrl={current.cover_asset_id ? trackCoverURL(current.cover_asset_id) : null}
              shuffle={shuffle}
              repeat={repeat}
              onPlayPause={toggle}
              onNext={next}
              onPrev={prev}
              onSeek={seekPct}
              onShuffle={toggleShuffle}
              onRepeat={cycleRepeat}
            />
          </div>

          <div className="flex shrink-0 flex-col gap-1">
            <Link
              href={`/library/music/${current.id}` as Route}
              aria-label={`Open ${current.title}`}
              title="Open track"
              className="grid h-8 w-8 place-items-center rounded-full border transition hover:bg-[var(--tpl-surface-2)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
            >
              <Icon name="music-open-playlist-icon" size={14} />
            </Link>
            <button
              type="button"
              onClick={stop}
              aria-label="Close player"
              title="Close player"
              className="grid h-8 w-8 place-items-center rounded-full border transition hover:bg-[var(--tpl-surface-2)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
            >
              <Icon name="close-icon" size={12} />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
