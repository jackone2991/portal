"use client";

import { useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "../../../components/ui/Icon";
import { useMusicPlayerOptional } from "../../../components/music/MusicPlayerProvider";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import { listAssets } from "@/lib/media-assets";
import {
  createTrack,
  deleteTrack,
  isPlayable,
  listMyTracks,
  listTracks,
  publishTrack,
  trackArtist,
  trackCoverURL,
  unpublishTrack,
  uploadAudioAsset,
  type Track,
} from "@/lib/music";

/**
 * Library · Nhạc — the music vertical's index, mirroring the comic vertical's
 * shape (title + compact tab toggle, "Thư viện" = published catalogue, "Của tôi"
 * = your own drafts and published tracks, create lives on the "Của tôi" tab only).
 *
 * Rows are a list rather than a card grid: a track is a row of metadata, and the
 * play affordance wants to sit on the left where the eye already is. Clicking any
 * row queues the WHOLE visible list from that point, through the app-wide
 * `MusicPlayerProvider`, so playback continues as the user navigates away.
 */
export function MusicIndexView() {
  const qc = useQueryClient();
  const player = useMusicPlayerOptional();

  const [tab, setTab] = useState<"all" | "mine">("all");
  const [modalOpen, setModalOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [artist, setArtist] = useState("");
  const [album, setAlbum] = useState("");
  const [audioAssetId, setAudioAssetId] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [rowErr, setRowErr] = useState<string | null>(null);
  const [uploadPct, setUploadPct] = useState<number | null>(null);
  const fileRef = useRef<HTMLInputElement | null>(null);

  const all = useQuery({ queryKey: ["tracks", "published"], queryFn: () => listTracks() });
  const mine = useQuery({
    queryKey: ["tracks", "mine"],
    queryFn: () => listMyTracks(),
    enabled: tab === "mine",
  });

  // Ready audio assets the user can attach. Fetched only while the modal is open —
  // a track with no audio asset is created fine but can never be played, so the
  // picker is the difference between a working feature and a list of dead rows.
  const audioAssets = useQuery({
    queryKey: ["assets", "audio", "ready"],
    queryFn: () => listAssets({ kind: "audio", status: "ready" }),
    enabled: modalOpen,
  });

  function resetForm() {
    setTitle("");
    setArtist("");
    setAlbum("");
    setAudioAssetId("");
    setUploadPct(null);
  }

  /**
   * Upload an audio file straight from this modal and select it. The upload
   * studio is video-shaped (it waits for HLS and renders a video element), so
   * routing audio through it would be the wrong journey — attaching the file
   * where the track is created keeps the whole flow in one dialog.
   */
  const upload = useMutation({
    mutationFn: (file: File) => uploadAudioAsset(file, setUploadPct),
    onMutate: () => {
      setErr(null);
      setUploadPct(0);
    },
    onSuccess: (assetId, file) => {
      setAudioAssetId(assetId);
      setUploadPct(null);
      if (!title.trim()) setTitle(file.name.replace(/\.[^.]+$/, ""));
      audioAssets.refetch();
    },
    onError: (e) => {
      setUploadPct(null);
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : errText(e, "Tải tệp lên thất bại."));
    },
  });

  const create = useMutation({
    mutationFn: () =>
      createTrack({
        title: title.trim(),
        artist: artist.trim() || null,
        album: album.trim() || null,
        audio_asset_id: audioAssetId || null,
      }),
    onSuccess: () => {
      setModalOpen(false);
      resetForm();
      qc.invalidateQueries({ queryKey: ["tracks"] });
    },
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không tạo được bài hát."),
  });

  const mutateRow = {
    publish: useMutation({
      mutationFn: (t: Track) => publishTrack(t.id),
      onSuccess: () => qc.invalidateQueries({ queryKey: ["tracks"] }),
      onError: (e) => setRowErr(rowMessage(e, "Không đăng được bài hát.")),
    }),
    unpublish: useMutation({
      mutationFn: (t: Track) => unpublishTrack(t.id),
      onSuccess: () => qc.invalidateQueries({ queryKey: ["tracks"] }),
      onError: (e) => setRowErr(rowMessage(e, "Không gỡ được bài hát.")),
    }),
    remove: useMutation({
      mutationFn: (t: Track) => deleteTrack(t.id),
      onSuccess: () => qc.invalidateQueries({ queryKey: ["tracks"] }),
      onError: (e) => setRowErr(rowMessage(e, "Không xoá được bài hát.")),
    }),
  };

  const active = tab === "all" ? all : mine;
  const tracks = tab === "all" ? all.data?.tracks ?? [] : mine.data?.tracks ?? [];
  const playable = tracks.filter(isPlayable);

  return (
    <section>
      <header className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
          Nhạc
        </h1>
        <div className="flex items-center gap-2">
          <TabBtn active={tab === "all"} onClick={() => setTab("all")}>
            Thư viện
          </TabBtn>
          <TabBtn active={tab === "mine"} onClick={() => setTab("mine")}>
            Của tôi
          </TabBtn>
        </div>
      </header>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => player?.playQueue(playable, 0)}
          disabled={playable.length === 0}
          className="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-40"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          <Icon name="music-play-icon-big" size={14} />
          Phát tất cả
        </button>

        {tab === "mine" && (
          <button
            type="button"
            onClick={() => {
              setErr(null);
              setModalOpen(true);
            }}
            className="rounded-lg border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
          >
            Thêm bài hát
          </button>
        )}
      </div>

      {rowErr && <Banner onDismiss={() => setRowErr(null)}>{rowErr}</Banner>}

      {active.isPending ? (
        <SkeletonList />
      ) : active.isError ? (
        <ErrorState onRetry={() => active.refetch()} />
      ) : tracks.length === 0 ? (
        <EmptyState mine={tab === "mine"} />
      ) : (
        <ul
          className="divide-y overflow-hidden rounded-xl border"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          {tracks.map((t, i) => (
            <TrackRow
              key={t.id}
              track={t}
              index={i + 1}
              showStatus={tab === "mine"}
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
              onPublish={() => mutateRow.publish.mutate(t)}
              onUnpublish={() => mutateRow.unpublish.mutate(t)}
              onDelete={() => mutateRow.remove.mutate(t)}
            />
          ))}
        </ul>
      )}

      {modalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={() => setModalOpen(false)}
          role="dialog"
          aria-modal="true"
          aria-label="Thêm bài hát"
        >
          <div
            className="w-full max-w-md rounded-xl p-6 shadow-lg"
            style={{ background: "var(--tpl-surface)" }}
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="mb-4 text-lg font-semibold" style={{ color: "var(--tpl-heading)" }}>
              Thêm bài hát
            </h2>

            {err && <Banner>{err}</Banner>}

            <div className="space-y-3">
              <Input label="Tên bài hát" value={title} onChange={setTitle} autoFocus />
              <Input label="Nghệ sĩ" value={artist} onChange={setArtist} />
              <Input label="Album" value={album} onChange={setAlbum} />

              <div>
                <label
                  className="mb-1 block text-xs font-semibold"
                  style={{ color: "var(--tpl-muted)" }}
                  htmlFor="track-audio-asset"
                >
                  Tệp âm thanh
                </label>
                <select
                  id="track-audio-asset"
                  value={audioAssetId}
                  onChange={(e) => setAudioAssetId(e.target.value)}
                  className="w-full rounded-lg border px-3 py-2 text-sm"
                  style={{
                    borderColor: "var(--tpl-border)",
                    background: "var(--tpl-bg)",
                    color: "var(--tpl-text)",
                  }}
                >
                  <option value="">— Chưa gắn (không phát được) —</option>
                  {(audioAssets.data?.assets ?? []).map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.title || a.original_filename || a.id.slice(0, 8)}
                    </option>
                  ))}
                </select>
                <div className="mt-2 flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => fileRef.current?.click()}
                    disabled={upload.isPending}
                    className="rounded-lg border px-3 py-1.5 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50"
                    style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-accent)" }}
                  >
                    {upload.isPending ? `Đang tải lên… ${uploadPct ?? 0}%` : "Tải tệp mới lên"}
                  </button>
                  <input
                    ref={fileRef}
                    type="file"
                    accept="audio/*"
                    className="hidden"
                    onChange={(e) => {
                      const f = e.target.files?.[0];
                      e.target.value = ""; // allow re-picking the same file
                      if (f) upload.mutate(f);
                    }}
                  />
                </div>

                <p className="mt-1 text-xs" style={{ color: "var(--tpl-muted)" }}>
                  {audioAssets.isPending
                    ? "Đang tải danh sách…"
                    : (audioAssets.data?.assets ?? []).length === 0
                      ? "Chưa có tệp âm thanh nào — bấm “Tải tệp mới lên” để thêm."
                      : "Không gắn tệp thì bài hát vẫn tạo được nhưng sẽ không phát được."}
                </p>
              </div>
            </div>

            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setModalOpen(false)}
                className="rounded-lg border px-4 py-2 text-sm font-medium transition hover:bg-[var(--tpl-surface-2)]"
                style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
              >
                Huỷ
              </button>
              <button
                type="button"
                onClick={() => create.mutate()}
                disabled={!title.trim() || create.isPending}
                className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
                style={{ background: "var(--tpl-accent)" }}
              >
                {create.isPending ? "Đang tạo…" : "Tạo"}
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

/* ── pieces ──────────────────────────────────────────────────────── */

function rowMessage(e: unknown, fallback: string): string {
  return e instanceof ApiError ? problemDisplayMessage(e.body) : fallback;
}

/** Surface a thrown Error message (upload helper) instead of a generic string. */
function errText(e: unknown, fallback: string): string {
  return e instanceof Error && e.message ? e.message : fallback;
}

function TrackRow({
  track,
  index,
  showStatus,
  current,
  playing,
  onPlay,
  onPublish,
  onUnpublish,
  onDelete,
}: {
  track: Track;
  index: number;
  showStatus: boolean;
  current: boolean;
  playing: boolean;
  onPlay: () => void;
  onPublish: () => void;
  onUnpublish: () => void;
  onDelete: () => void;
}) {
  const playableRow = isPlayable(track);
  const accent = current ? "var(--tpl-accent)" : "var(--tpl-heading)";

  return (
    <li
      className="flex items-center gap-3 px-3 py-2.5 transition hover:bg-[var(--tpl-surface-2)]"
      style={{ borderColor: "var(--tpl-border)" }}
    >
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
        aria-label={playing ? `Tạm dừng ${track.title}` : `Phát ${track.title}`}
        title={playableRow ? undefined : "Bài hát chưa gắn tệp âm thanh"}
        className="relative grid h-10 w-10 shrink-0 place-items-center overflow-hidden rounded-md disabled:opacity-40"
        style={{ background: "var(--tpl-surface-2)" }}
      >
        {track.cover_asset_id && (
          // eslint-disable-next-line @next/next/no-img-element -- dynamic, API-proxied variant, not a static/optimizable asset
          <img
            src={trackCoverURL(track.cover_asset_id)}
            alt=""
            className="absolute inset-0 h-full w-full object-cover"
          />
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

      {showStatus && (
        <span
          className="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase text-white"
          style={{
            background:
              track.status === "published" ? "var(--tpl-accent)" : "rgba(0,0,0,.45)",
          }}
        >
          {track.status === "published" ? "đã đăng" : "nháp"}
        </span>
      )}

      {showStatus && (
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            onClick={track.status === "published" ? onUnpublish : onPublish}
            className="text-xs font-semibold transition hover:opacity-80"
            style={{ color: "var(--tpl-accent)" }}
          >
            {track.status === "published" ? "Gỡ" : "Đăng"}
          </button>
          <button
            type="button"
            onClick={onDelete}
            aria-label={`Xoá ${track.title}`}
            className="transition hover:text-[#ef4444]"
            style={{ color: "var(--tpl-muted)" }}
          >
            <Icon name="little-delete" size={15} />
          </button>
        </div>
      )}
    </li>
  );
}

function TabBtn({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-lg px-3 py-1.5 text-sm font-semibold transition"
      style={{
        background: active ? "var(--tpl-accent)" : "transparent",
        color: active ? "#fff" : "var(--tpl-muted)",
        border: `1px solid ${active ? "var(--tpl-accent)" : "var(--tpl-border)"}`,
      }}
    >
      {children}
    </button>
  );
}

function Input({
  label,
  value,
  onChange,
  autoFocus,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  autoFocus?: boolean;
}) {
  return (
    <div>
      <label className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
        {label}
      </label>
      <input
        // eslint-disable-next-line jsx-a11y/no-autofocus -- modal opens on an explicit user action
        autoFocus={autoFocus}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border px-3 py-2 text-sm"
        style={{
          borderColor: "var(--tpl-border)",
          background: "var(--tpl-bg)",
          color: "var(--tpl-text)",
        }}
      />
    </div>
  );
}

function Banner({ children, onDismiss }: { children: ReactNode; onDismiss?: () => void }) {
  return (
    <p
      className="mb-3 flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm"
      style={{
        borderColor: "rgba(239,68,68,.4)",
        background: "rgba(239,68,68,.08)",
        color: "#ef4444",
      }}
    >
      <span>{children}</span>
      {onDismiss && (
        <button type="button" onClick={onDismiss} aria-label="Đóng">
          <Icon name="close-icon" size={10} />
        </button>
      )}
    </p>
  );
}

function SkeletonList() {
  return (
    <ul
      className="divide-y overflow-hidden rounded-xl border"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      {Array.from({ length: 6 }).map((_, i) => (
        <li key={i} className="flex items-center gap-3 px-3 py-3">
          <div className="h-10 w-10 shrink-0 animate-pulse rounded-md" style={{ background: "var(--tpl-surface-2)" }} />
          <div className="flex-1 space-y-1.5">
            <div className="h-3 w-1/3 animate-pulse rounded" style={{ background: "var(--tpl-surface-2)" }} />
            <div className="h-2.5 w-1/5 animate-pulse rounded" style={{ background: "var(--tpl-surface-2)" }} />
          </div>
        </li>
      ))}
    </ul>
  );
}

function EmptyState({ mine }: { mine: boolean }) {
  return (
    <div
      className="flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <Icon name="headphones-icon" size={28} style={{ color: "var(--tpl-muted)" }} />
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        {mine
          ? "Bạn chưa có bài hát nào — bấm “Thêm bài hát” để bắt đầu."
          : "Chưa có bài hát nào được xuất bản."}
      </p>
    </div>
  );
}

function ErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="rounded-xl border py-12 text-center" style={{ borderColor: "var(--tpl-border)" }}>
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        Không tải được danh sách nhạc.
      </p>
      <button
        type="button"
        onClick={onRetry}
        className="mt-3 rounded-md border px-3 py-1.5 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
        style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
      >
        Thử lại
      </button>
    </div>
  );
}
