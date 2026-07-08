"use client";

import { WidgetCard } from "../widget/WidgetCard";
import { TrackItem } from "./TrackItem";

/**
 * Playlist card — React + Tailwind port of the Olympus `ol.widget.w-playlist`
 * (template-main/social/social/"Music And Playlists.html", ~2618-2652).
 *
 * Wraps an ordered list of `TrackItem` rows in the standard `.ui-block` widget
 * shell (title bar + three-dots affordance). `nowPlaying` is the 1-based index of
 * the active row; `onPlay` receives that same 1-based index when a row is clicked.
 * Ships with deterministic sample tracks so it renders standalone.
 */

type Track = { title: string; artist: string; duration: string };

const SAMPLE_TRACKS: Track[] = [
  { title: "The Past Starts Slow...", artist: "System of a Revenge", duration: "3:22" },
  { title: "The Pretender", artist: "Kung Fighters", duration: "5:48" },
  { title: "Bohemian Rhapsody", artist: "Green Groove", duration: "4:16" },
  { title: "Sunset Overdrive", artist: "Neon Rebels", duration: "3:51" },
  { title: "Midnight City Lights", artist: "The Wanderers", duration: "4:07" },
];

export function PlaylistWidget({
  title = "Playlist",
  tracks = SAMPLE_TRACKS,
  nowPlaying,
  onPlay,
}: {
  title?: string;
  tracks?: Track[];
  /** 1-based index of the currently-playing row. */
  nowPlaying?: number;
  /** Called with the clicked row's 1-based index. */
  onPlay?: (index: number) => void;
}) {
  return (
    <WidgetCard title={title} more>
      <ol className="space-y-1">
        {tracks.map((t, i) => {
          const index = i + 1;
          return (
            <TrackItem
              key={`${t.title}-${index}`}
              index={index}
              title={t.title}
              artist={t.artist}
              duration={t.duration}
              playing={index === nowPlaying}
              onPlay={() => onPlay?.(index)}
            />
          );
        })}
      </ol>
    </WidgetCard>
  );
}
