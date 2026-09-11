"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "../../../components/ui/Icon";
import { useMusicPlayerOptional } from "../../../components/music/MusicPlayerProvider";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  getPlaylist,
  isPlayable,
  removeTrackFromPlaylist,
  renamePlaylist,
  trackArtist,
  trackCoverURL,
  type Track,
} from "@/lib/music";

/**
 * Library · Nhạc · Playlist · chi tiết — one playlist, in playlist order.
 *
 * Play routes through the app-wide `MusicPlayerProvider`, same as the library
 * and the track page, so starting here leaves the docked bar in charge and
 * navigating away does not interrupt it. The queue it hands over is the
 * playlist's own order, filtered to what can actually play.
 *
 * "Remove" takes the track out of THIS playlist and nowhere else — the track
 * itself, and any other playlist holding it, are untouched. That is worth
 * saying in the UI, because a bin icon next to a song reads as "delete song".
 */
export function PlaylistDetailView({ id }: { id: string }) {
  const qc = useQueryClient();
  const player = useMusicPlayerOptional();

  const { data: playlist, isPending, isError, refetch } = useQuery({
    queryKey: ["playlist", id],
    queryFn: () => getPlaylist(id),
  });

  const [editing, setEditing] = useState(false);
  const [name, setName] = useState("");
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (playlist) setName(playlist.name);
  }, [playlist]);

  const rename = useMutation({
    mutationFn: () => renamePlaylist(id, { name: name.trim() }),
    onSuccess: () => {
      setEditing(false);
      setErr(null);
      qc.invalidateQueries({ queryKey: ["playlist", id] });
      qc.invalidateQueries({ queryKey: ["playlists"] });
    },
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không đổi được tên."),
  });

  const remove = useMutation({
    mutationFn: (trackID: string) => removeTrackFromPlaylist(id, trackID),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["playlist", id] });
      qc.invalidateQueries({ queryKey: ["playlists"] });
    },
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không gỡ được bài."),
  });

  if (isPending) {
    return (
      <Frame>
        <p className="py-10 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
          Đang tải…
        </p>
      </Frame>
    );
  }

  if (isError || !playlist) {
    return (
      <Frame>
        <div className="rounded-xl border px-6 py-10 text-center" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
          <p className="text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
            Không mở được playlist này
          </p>
          <button
            type="button"
            onClick={() => refetch()}
            className="mt-3 rounded-lg px-3 py-1.5 text-xs font-semibold text-white"
            style={{ background: "var(--tpl-accent)" }}
          >
            Thử lại
          </button>
        </div>
      </Frame>
    );
  }

  const tracks = playlist.tracks ?? [];
  const playable = tracks.filter(isPlayable);

  return (
    <Frame>
      <div className="mb-4">
        <Link
          href={"/library/music/playlists" as Route}
          className="text-xs font-semibold hover:underline"
          style={{ color: "var(--tpl-muted)" }}
        >
          ← Tất cả playlist
        </Link>
      </div>

      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          {editing ? (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                if (name.trim()) rename.mutate();
              }}
              className="flex gap-2"
            >
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                maxLength={120}
                autoFocus
                className="min-w-0 flex-1 rounded-lg border bg-transparent px-3 py-1.5 text-lg font-bold outline-none"
                style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-heading)" }}
              />
              <button
                type="submit"
                disabled={!name.trim() || rename.isPending}
                className="shrink-0 rounded-lg px-3 py-1.5 text-xs font-semibold text-white disabled:opacity-50"
                style={{ background: "var(--tpl-accent)" }}
              >
                Lưu
              </button>
              <button
                type="button"
                onClick={() => {
                  setEditing(false);
                  setName(playlist.name);
                  setErr(null);
                }}
                className="shrink-0 text-xs font-semibold"
                style={{ color: "var(--tpl-muted)" }}
              >
                Huỷ
              </button>
            </form>
          ) : (
            <div className="flex items-center gap-2">
              <h1 className="truncate text-2xl font-bold" style={{ color: "var(--tpl-heading)" }}>
                {playlist.name}
              </h1>
              {/* A text button, not a pencil: the Olympus sprite is a social set
                  with no edit glyph, and an icon that resolves to nothing renders
                  as an empty box. */}
              <button
                type="button"
                onClick={() => setEditing(true)}
                className="shrink-0 text-xs font-semibold transition hover:opacity-80"
                style={{ color: "var(--tpl-accent)" }}
              >
                Đổi tên
              </button>
            </div>
          )}
          <p className="mt-0.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
            {tracks.length} bài
            {playable.length !== tracks.length ? ` · ${tracks.length - playable.length} bài chưa có tệp nhạc` : ""}
          </p>
        </div>

        <button
          type="button"
          disabled={playable.length === 0}
          onClick={() => player?.playQueue(playable, 0)}
          className="flex shrink-0 items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-40"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          <Icon name="play-icon" size={13} />
          Phát tất cả
        </button>
      </div>

      {err && (
        <p
          role="alert"
          className="mb-3 rounded-lg border px-3 py-2 text-xs"
          style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
        >
          {err}
        </p>
      )}

      {tracks.length === 0 ? (
        <div
          className="rounded-xl border px-6 py-10 text-center"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          <p className="text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
            Playlist này chưa có bài nào
          </p>
          <p className="mx-auto mt-1 max-w-[46ch] text-xs leading-relaxed" style={{ color: "var(--tpl-muted)" }}>
            Sang{" "}
            <Link href={"/library/music" as Route} className="font-semibold hover:underline" style={{ color: "var(--tpl-accent)" }}>
              thư viện nhạc
            </Link>
            , tích chọn các bài rồi bấm <strong>Thêm vào playlist</strong>.
          </p>
        </div>
      ) : (
        <ul
          className="divide-y overflow-hidden rounded-xl border"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          {tracks.map((t, i) => (
            <PlaylistTrackRow
              key={t.id}
              track={t}
              index={i + 1}
              current={player?.isCurrent(t.id) ?? false}
              playing={(player?.isCurrent(t.id) ?? false) && (player?.playing ?? false)}
              onPlay={() => {
                if (player?.isCurrent(t.id)) {
                  player.toggle();
                  return;
                }
                const at = playable.findIndex((p) => p.id === t.id);
                if (at >= 0) player?.playQueue(playable, at);
              }}
              onRemove={() => remove.mutate(t.id)}
            />
          ))}
        </ul>
      )}
    </Frame>
  );
}

function Frame({ children }: { children: React.ReactNode }) {
  return <div className="mx-auto w-full max-w-[760px] px-3 py-6 sm:px-5">{children}</div>;
}

function PlaylistTrackRow({
  track,
  index,
  current,
  playing,
  onPlay,
  onRemove,
}: {
  track: Track;
  index: number;
  current: boolean;
  playing: boolean;
  onPlay: () => void;
  onRemove: () => void;
}) {
  const playableRow = isPlayable(track);
  const accent = current ? "var(--tpl-accent)" : "var(--tpl-heading)";

  return (
    <li className="flex items-center gap-3 px-3 py-2.5 transition hover:bg-[var(--tpl-surface-2)]">
      <span
        className="w-5 shrink-0 text-center text-xs tabular-nums"
        style={{ color: current ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
      >
        {index}
      </span>

      <button
        type="button"
        onClick={onPlay}
        disabled={!playableRow}
        aria-label={`Phát ${track.title}`}
        className="relative h-9 w-9 shrink-0 overflow-hidden rounded disabled:cursor-not-allowed"
        style={{ background: "var(--tpl-surface-2)" }}
      >
        {track.cover_asset_id && (
          // eslint-disable-next-line @next/next/no-img-element -- dynamic, API-proxied variant, not a static/optimizable asset
          <img src={trackCoverURL(track.cover_asset_id)} alt="" className="absolute inset-0 h-full w-full object-cover" />
        )}
        <span
          className="relative grid h-full w-full place-items-center bg-black/35 text-white"
          style={{ opacity: playableRow ? 1 : 0.6 }}
        >
          <Icon name={playing ? "music-pause-icon" : "play-icon"} size={12} />
        </span>
      </button>

      <div className="min-w-0 flex-1">
        <Link
          href={`/library/music/${track.id}` as Route}
          className="block truncate text-sm font-semibold hover:underline"
          style={{ color: accent }}
        >
          {track.title}
        </Link>
        <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
          {trackArtist(track)}
          {track.album ? ` · ${track.album}` : ""}
        </p>
      </div>

      {/* "Gỡ khỏi playlist", not "xoá": the label carries the whole distinction,
          because a bin icon beside a song reads as deleting the song. */}
      <button
        type="button"
        onClick={onRemove}
        title="Gỡ khỏi playlist"
        aria-label={`Gỡ ${track.title} khỏi playlist`}
        className="shrink-0 text-xs font-semibold transition hover:text-[#ef4444]"
        style={{ color: "var(--tpl-muted)" }}
      >
        Gỡ
      </button>
    </li>
  );
}
