"use client";

import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { PlaylistWidget } from "../music/PlaylistWidget";
import { useMusicPlayerOptional } from "../music/MusicPlayerProvider";
import { listTracks, trackArtist, trackCoverURL, isPlayable } from "@/lib/music";

/**
 * Home-rail music widget.
 *
 * Shows the newest published tracks and plays them through the app-wide
 * `MusicPlayerProvider`, so a click here starts audio that keeps running as the
 * user navigates away. Clicking a row queues the whole visible list from that
 * row, which is what people expect from a playlist card (rather than playing one
 * track and stopping).
 *
 * Failure-isolated like the other rail widgets: a query error renders nothing at
 * all rather than putting an error card in the sidebar. An *empty* result does
 * render, though — a silent widget would make the feature undiscoverable for
 * someone who simply hasn't added music yet.
 */
const WIDGET_LIMIT = 5;

export function MusicWidget() {
  const player = useMusicPlayerOptional();
  const { data, isError } = useQuery({
    queryKey: ["tracks", "widget"],
    queryFn: () => listTracks(),
    retry: false,
  });

  // Degrade to nothing on failure (matches ContinueWidget / the rail convention).
  if (isError) return null;

  const playable = (data?.tracks ?? []).filter(isPlayable).slice(0, WIDGET_LIMIT);

  const nowPlaying = player?.current
    ? playable.findIndex((t) => t.id === player.current!.id) + 1 || undefined
    : undefined;

  return (
    <PlaylistWidget
      title="Music"
      tracks={playable.map((t) => ({
        title: t.title,
        artist: trackArtist(t),
        coverUrl: t.cover_asset_id ? trackCoverURL(t.cover_asset_id) : null,
      }))}
      nowPlaying={nowPlaying}
      emptyLabel="No published tracks yet."
      onPlay={(index) => player?.playQueue(playable, index - 1)}
      footer={
        <Link
          href={"/library/music" as Route}
          className="text-xs font-semibold transition hover:opacity-80"
          style={{ color: "var(--tpl-accent)" }}
        >
          Open music library →
        </Link>
      }
    />
  );
}
