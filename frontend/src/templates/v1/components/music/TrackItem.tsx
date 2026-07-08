"use client";

import { Icon } from "../ui/Icon";

/**
 * One playlist row — React + Tailwind port of an Olympus `ol.w-playlist > li`
 * (template-main/social/social/"Music And Playlists.html", track rows ~2618-2652).
 *
 * Client component (binds the play click): an index number, a small gradient thumb
 * (no image assets) that reveals a play affordance on hover — shown persistently
 * while `playing` — the composition title + author, the duration, and a three-dots
 * menu button. Compose these inside `PlaylistWidget`.
 */

/** Deterministic two-stop gradient from a seed string (self-contained; no images). */
function seedGradient(seed: string): string {
  const hue = [...seed].reduce((a, c) => a + c.charCodeAt(0), 0) % 360;
  return `linear-gradient(135deg, hsl(${hue} 68% 58%), hsl(${(hue + 40) % 360} 68% 46%))`;
}

export function TrackItem({
  index,
  title,
  artist,
  duration,
  playing = false,
  onPlay,
}: {
  index: number;
  title: string;
  artist: string;
  duration: string;
  playing?: boolean;
  onPlay?: () => void;
}) {
  return (
    <li className="group flex items-center gap-3 rounded-lg px-2 py-2 transition hover:bg-[var(--tpl-surface-2)]">
      <span
        className="w-5 shrink-0 text-center text-xs tabular-nums"
        style={{ color: playing ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
      >
        {index}
      </span>

      <button
        type="button"
        onClick={onPlay}
        aria-label={`Play ${title}`}
        className="relative grid h-9 w-9 shrink-0 place-items-center overflow-hidden rounded-md"
        style={{ background: seedGradient(title + artist) }}
      >
        <span
          className={`grid h-full w-full place-items-center bg-black/35 text-white transition ${
            playing ? "opacity-100" : "opacity-0 group-hover:opacity-100"
          }`}
        >
          <Icon name="play-icon" size={12} />
        </span>
      </button>

      <div className="min-w-0 flex-1">
        <p
          className="truncate text-sm font-semibold"
          style={{ color: playing ? "var(--tpl-accent)" : "var(--tpl-heading)" }}
        >
          {title}
        </p>
        <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
          {artist}
        </p>
      </div>

      <time className="shrink-0 text-xs tabular-nums" style={{ color: "var(--tpl-muted)" }}>
        {duration}
      </time>

      <button
        type="button"
        aria-label="Track options"
        className="shrink-0 text-[var(--tpl-muted)] transition hover:text-[var(--tpl-heading)]"
      >
        <Icon name="three-dots-icon" size={14} />
      </button>
    </li>
  );
}
