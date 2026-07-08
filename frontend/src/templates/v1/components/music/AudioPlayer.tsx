"use client";

import { Icon } from "../ui/Icon";

/**
 * Compact player bar — React + Tailwind port of the Olympus `.mejs-*` audio player
 * (template-main/social/social/"Music And Playlists.html", ~3783).
 *
 * Controlled client component: transport controls (previous / play-pause / next),
 * the current track title + artist, an accent progress bar with current/total time,
 * and shuffle + repeat affordances. State lives with the parent — pass `playing` and
 * `progress` (0-100) and wire the `onPlayPause` / `onNext` / `onPrev` callbacks.
 */

// No duration prop in the contract; derive current/total labels from `progress`
// against a fixed reference length so the timer reads sensibly.
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
  onPlayPause,
  onNext,
  onPrev,
}: {
  track?: { title: string; artist: string };
  playing?: boolean;
  /** Playback position 0-100. */
  progress?: number;
  onPlayPause?: () => void;
  onNext?: () => void;
  onPrev?: () => void;
}) {
  const pct = Math.min(100, Math.max(0, progress));
  const current = formatTime((pct / 100) * REF_TOTAL_SEC);
  const total = formatTime(REF_TOTAL_SEC);

  return (
    <div
      className="flex items-center gap-4 rounded-xl px-4 py-3 shadow-sm"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {/* Transport controls */}
      <div className="flex items-center gap-2" style={{ color: "var(--tpl-muted)" }}>
        <button
          type="button"
          onClick={onPrev}
          aria-label="Previous track"
          className="grid h-8 w-8 place-items-center rounded-full transition hover:text-[var(--tpl-heading)]"
        >
          <Icon name="music-previous-song-icon" size={16} />
        </button>
        <button
          type="button"
          onClick={onPlayPause}
          aria-label={playing ? "Pause" : "Play"}
          className="grid h-11 w-11 place-items-center rounded-full text-white shadow-sm transition hover:opacity-90"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          <Icon name={playing ? "music-pause-icon" : "music-play-icon-big"} size={18} />
        </button>
        <button
          type="button"
          onClick={onNext}
          aria-label="Next track"
          className="grid h-8 w-8 place-items-center rounded-full transition hover:text-[var(--tpl-heading)]"
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
            {current} / {total}
          </span>
        </div>
        <div
          className="mt-2 h-1.5 w-full overflow-hidden rounded-full"
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
          aria-label="Shuffle"
          className="grid h-8 w-8 place-items-center rounded-full transition hover:text-[var(--tpl-accent)]"
        >
          <Icon name="music-shuffle-icon" size={16} />
        </button>
        <button
          type="button"
          aria-label="Repeat"
          className="grid h-8 w-8 place-items-center rounded-full transition hover:text-[var(--tpl-accent)]"
        >
          <Icon name="music-repeat-icon" size={16} />
        </button>
      </div>
    </div>
  );
}
