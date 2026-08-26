"use client";

import type { CSSProperties, KeyboardEvent, MouseEvent } from "react";
import { Icon } from "../ui/Icon";

/**
 * Compact player bar — React + Tailwind port of the Olympus `.mejs-*` audio player
 * (template-main/social/social/"Music And Playlists.html", ~3783).
 *
 * Presentational and fully controlled: transport controls (previous / play-pause /
 * next), the current track title + artist, an accent progress bar with
 * current/total time, and shuffle + repeat affordances. It owns no audio — state
 * lives with the parent (in the app that is `MusicPlayerProvider`).
 *
 * Every prop is optional so the component still renders standalone with sample
 * data. When `durationSec` is omitted the timer falls back to a fixed reference
 * length, which is what the design-system preview card shows; pass a real
 * `durationSec` (and `onSeek`) to make the bar a working scrubber.
 */

// Fallback only — used when no real `durationSec` is supplied, so the timer
// reads sensibly in a standalone/preview render.
const REF_TOTAL_SEC = 200; // 3:20

function formatTime(sec: number): string {
  const s = Math.max(0, Math.round(sec));
  const m = Math.floor(s / 60);
  const r = s % 60;
  return `${m}:${r.toString().padStart(2, "0")}`;
}

export function AudioPlayer({
  track = { title: "The Past Starts Slow...", artist: "System of a Revenge" },
  playing = false,
  progress = 0,
  durationSec,
  coverUrl,
  shuffle = false,
  repeat = "off",
  disabled = false,
  onPlayPause,
  onNext,
  onPrev,
  onSeek,
  onShuffle,
  onRepeat,
}: {
  track?: { title: string; artist: string };
  playing?: boolean;
  /** Playback position 0-100. */
  progress?: number;
  /** Real track length in seconds. Falls back to a reference length when absent. */
  durationSec?: number;
  /** Optional cover art shown left of the transport controls. */
  coverUrl?: string | null;
  shuffle?: boolean;
  repeat?: "off" | "one" | "all";
  /** Greys out the transport when there is nothing loaded. */
  disabled?: boolean;
  onPlayPause?: () => void;
  onNext?: () => void;
  onPrev?: () => void;
  /** Called with a 0-100 percentage when the progress bar is scrubbed. */
  onSeek?: (pct: number) => void;
  onShuffle?: () => void;
  onRepeat?: () => void;
}) {
  const pct = Math.min(100, Math.max(0, progress));
  const total = durationSec && durationSec > 0 ? durationSec : REF_TOTAL_SEC;
  const current = formatTime((pct / 100) * total);
  const seekable = Boolean(onSeek);

  function seekFromPointer(e: MouseEvent<HTMLDivElement>) {
    if (!onSeek) return;
    const rect = e.currentTarget.getBoundingClientRect();
    if (rect.width <= 0) return;
    onSeek(((e.clientX - rect.left) / rect.width) * 100);
  }

  function seekFromKey(e: KeyboardEvent<HTMLDivElement>) {
    if (!onSeek) return;
    const step = e.shiftKey ? 10 : 5;
    if (e.key === "ArrowRight") {
      e.preventDefault();
      onSeek(pct + step);
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      onSeek(pct - step);
    } else if (e.key === "Home") {
      e.preventDefault();
      onSeek(0);
    } else if (e.key === "End") {
      e.preventDefault();
      onSeek(100);
    }
  }

  const activeToggle: CSSProperties = { color: "var(--tpl-accent)" };

  return (
    <div
      className="flex items-center gap-4 rounded-xl px-4 py-3 shadow-sm"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {coverUrl && (
        // eslint-disable-next-line @next/next/no-img-element -- dynamic, API-proxied variant, not a static/optimizable asset
        <img
          src={coverUrl}
          alt=""
          className="h-11 w-11 shrink-0 rounded-md object-cover"
          style={{ background: "var(--tpl-surface-2)" }}
        />
      )}

      {/* Transport controls */}
      <div
        className="flex items-center gap-2"
        style={{ color: "var(--tpl-muted)", opacity: disabled ? 0.5 : 1 }}
      >
        <button
          type="button"
          onClick={onPrev}
          disabled={disabled}
          aria-label="Previous track"
          className="grid h-8 w-8 place-items-center rounded-full transition hover:text-[var(--tpl-heading)] disabled:cursor-default"
        >
          <Icon name="music-previous-song-icon" size={16} />
        </button>
        <button
          type="button"
          onClick={onPlayPause}
          disabled={disabled}
          aria-label={playing ? "Pause" : "Play"}
          className="grid h-11 w-11 place-items-center rounded-full text-white shadow-sm transition hover:opacity-90 disabled:cursor-default"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          <Icon name={playing ? "music-pause-icon" : "music-play-icon-big"} size={18} />
        </button>
        <button
          type="button"
          onClick={onNext}
          disabled={disabled}
          aria-label="Next track"
          className="grid h-8 w-8 place-items-center rounded-full transition hover:text-[var(--tpl-heading)] disabled:cursor-default"
        >
          <Icon name="music-next-song-icon" size={16} />
        </button>
      </div>

      {/* Track meta + progress */}
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-3">
          <p className="min-w-0 truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
            {track.title}
            <span className="font-normal" style={{ color: "var(--tpl-muted)" }}>
              {" "}
              — {track.artist}
            </span>
          </p>
          <span className="shrink-0 text-xs tabular-nums" style={{ color: "var(--tpl-muted)" }}>
            {current} / {formatTime(total)}
          </span>
        </div>
        <div
          role={seekable ? "slider" : undefined}
          tabIndex={seekable ? 0 : undefined}
          aria-label={seekable ? "Seek" : undefined}
          aria-valuemin={seekable ? 0 : undefined}
          aria-valuemax={seekable ? 100 : undefined}
          aria-valuenow={seekable ? Math.round(pct) : undefined}
          aria-valuetext={seekable ? `${current} of ${formatTime(total)}` : undefined}
          onClick={seekable ? seekFromPointer : undefined}
          onKeyDown={seekable ? seekFromKey : undefined}
          className={`mt-2 h-1.5 w-full overflow-hidden rounded-full ${
            seekable ? "cursor-pointer" : ""
          }`}
          style={{ background: "var(--tpl-surface-2)" }}
        >
          <div
            className="h-full rounded-full"
            style={{
              width: `${pct}%`,
              background: "linear-gradient(90deg, var(--tpl-accent), var(--tpl-accent-2))",
            }}
          />
        </div>
      </div>

      {/* Shuffle / repeat */}
      <div className="flex items-center gap-1" style={{ color: "var(--tpl-muted)" }}>
        <button
          type="button"
          onClick={onShuffle}
          aria-label="Shuffle"
          aria-pressed={onShuffle ? shuffle : undefined}
          className="grid h-8 w-8 place-items-center rounded-full transition hover:text-[var(--tpl-accent)]"
          style={shuffle ? activeToggle : undefined}
        >
          <Icon name="music-shuffle-icon" size={16} />
        </button>
        <button
          type="button"
          onClick={onRepeat}
          aria-label={repeat === "one" ? "Repeat one" : "Repeat"}
          aria-pressed={onRepeat ? repeat !== "off" : undefined}
          className="relative grid h-8 w-8 place-items-center rounded-full transition hover:text-[var(--tpl-accent)]"
          style={repeat !== "off" ? activeToggle : undefined}
        >
          <Icon name="music-repeat-icon" size={16} />
          {repeat === "one" && (
            <span className="absolute -bottom-0.5 text-[9px] font-bold leading-none">1</span>
          )}
        </button>
      </div>
    </div>
  );
}
