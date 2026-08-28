"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { AudioPlayer } from "./AudioPlayer";
import { Icon } from "../ui/Icon";
import { useMusicPlayerOptional } from "./MusicPlayerProvider";
import { trackArtist, trackCoverURL, type Track } from "@/lib/music";

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
  const [queueOpen, setQueueOpen] = useState(false);

  // No provider (standalone render) or nothing queued → render nothing.
  if (!player || !player.current) return null;

  const {
    current, playing, progressPct, duration, shuffle, repeat, error, queue, index,
    toggle, next, prev, seekPct, toggleShuffle, cycleRepeat, stop, jumpTo,
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

          <div className="relative flex shrink-0 flex-col gap-1">
            {queueOpen && (
              <QueuePopup
                queue={queue}
                index={index}
                playing={playing}
                onPick={(i) => jumpTo(i)}
                onClose={() => setQueueOpen(false)}
              />
            )}
            <button
              type="button"
              onClick={() => setQueueOpen((v) => !v)}
              aria-label="Danh sách phát"
              aria-expanded={queueOpen}
              title="Danh sách phát"
              className="grid h-8 w-8 place-items-center rounded-full border transition hover:bg-[var(--tpl-surface-2)]"
              style={{
                borderColor: queueOpen ? "var(--tpl-accent)" : "var(--tpl-border)",
                color: queueOpen ? "var(--tpl-accent)" : "var(--tpl-muted)",
              }}
            >
              <Icon name="music-open-playlist-icon" size={14} />
            </button>
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

/**
 * The queue, as a popup above the player bar.
 *
 * The button used to be a link to the current track's page, which answered a
 * question nobody was asking mid-playback: what people want from a player is to
 * see what is coming and to skip to a particular song. The link is still here,
 * once, in the header — where it reads as "open this track" rather than
 * masquerading as a playlist icon.
 *
 * The list is `queue`, which is already in play order, so what you see is the
 * order it will play in — shuffle included. Clicking row N plays row N; there is
 * no re-shuffle under the click.
 */
function QueuePopup({
  queue,
  index,
  playing,
  onPick,
  onClose,
}: {
  queue: Track[];
  index: number;
  playing: boolean;
  onPick: (i: number) => void;
  onClose: () => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const currentRow = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (!root.current?.contains(e.target as Node)) onClose();
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  // Open onto the song that is playing, not onto row 1: in a 200-track queue the
  // top of the list is rarely where you are.
  useEffect(() => {
    currentRow.current?.scrollIntoView({ block: "nearest" });
  }, []);

  const now = queue[index];

  return (
    <div
      ref={root}
      role="dialog"
      aria-label="Danh sách phát"
      className="absolute bottom-full right-0 z-50 mb-2 w-80 overflow-hidden rounded-xl shadow-2xl"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      <div
        className="flex items-center justify-between border-b px-4 py-2.5"
        style={{ borderColor: "var(--tpl-border)" }}
      >
        <span className="text-xs font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
          Danh sách phát · {queue.length}
        </span>
        {now && (
          <Link
            href={`/library/music/${now.id}` as Route}
            onClick={onClose}
            className="text-xs font-semibold hover:underline"
            style={{ color: "var(--tpl-accent)" }}
          >
            Mở bài này
          </Link>
        )}
      </div>

      <ul className="max-h-72 overflow-y-auto">
        {queue.map((t, i) => {
          const isNow = i === index;
          return (
            <li key={`${t.id}-${i}`}>
              <button
                ref={isNow ? currentRow : undefined}
                type="button"
                onClick={() => onPick(i)}
                className="flex w-full items-center gap-3 px-3 py-2 text-left transition hover:bg-[var(--tpl-surface-2)]"
                style={{ background: isNow ? "var(--tpl-surface-2)" : undefined }}
              >
                <span
                  className="grid h-4 w-4 shrink-0 place-items-center text-[10px] tabular-nums"
                  style={{ color: isNow ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
                >
                  {isNow ? <Icon name={playing ? "music-pause-icon" : "play-icon"} size={9} /> : i + 1}
                </span>
                <span className="min-w-0 flex-1">
                  <span
                    className="block truncate text-sm font-semibold"
                    style={{ color: isNow ? "var(--tpl-accent)" : "var(--tpl-heading)" }}
                  >
                    {t.title}
                  </span>
                  <span className="block truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                    {trackArtist(t)}
                  </span>
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
