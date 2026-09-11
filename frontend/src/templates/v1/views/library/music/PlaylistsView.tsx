"use client";

import { useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "../../../components/ui/Icon";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import { createPlaylist, deletePlaylist, listPlaylists, type Playlist } from "@/lib/music";

/**
 * Library · Nhạc · Playlist — the index (migration 0041).
 *
 * Playlists are filled from the library's selection toolbar ("Thêm vào
 * playlist"), not from here: this page is where you name them, open them and
 * throw them away. The empty state says so, because an empty page with only a
 * "create" button leaves you guessing where the tracks come from.
 */
export function PlaylistsView() {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<string | null>(null);

  const playlists = useQuery({ queryKey: ["playlists"], queryFn: listPlaylists });

  const create = useMutation({
    mutationFn: () => createPlaylist(name.trim()),
    onSuccess: () => {
      setName("");
      setErr(null);
      qc.invalidateQueries({ queryKey: ["playlists"] });
    },
    // 409 is the (owner, lower(name)) unique — a real answer, not a fault, so it
    // shows the server's own sentence rather than a generic failure.
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không tạo được playlist."),
  });

  const remove = useMutation({
    mutationFn: (id: string) => deletePlaylist(id),
    onSuccess: () => {
      setConfirming(null);
      qc.invalidateQueries({ queryKey: ["playlists"] });
    },
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không xoá được playlist."),
  });

  const items = playlists.data ?? [];

  return (
    <div className="mx-auto w-full max-w-[760px] px-3 py-6 sm:px-5">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold" style={{ color: "var(--tpl-heading)" }}>
            Playlist
          </h1>
          <p className="mt-0.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
            Tuyển tập riêng của bạn — thêm bài từ thư viện nhạc.
          </p>
        </div>
        <Link
          href={"/library/music" as Route}
          className="rounded-lg border px-3 py-1.5 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
        >
          ← Thư viện nhạc
        </Link>
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim()) create.mutate();
        }}
        className="mb-4 flex gap-2"
      >
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Tên playlist mới…"
          maxLength={120}
          className="min-w-0 flex-1 rounded-lg border bg-transparent px-3 py-2 text-sm outline-none"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
        />
        <button
          type="submit"
          disabled={!name.trim() || create.isPending}
          className="shrink-0 rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-40"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          {create.isPending ? "Đang tạo…" : "Tạo"}
        </button>
      </form>

      {err && (
        <p
          role="alert"
          className="mb-3 rounded-lg border px-3 py-2 text-xs"
          style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
        >
          {err}
        </p>
      )}

      {playlists.isPending ? (
        <p className="py-10 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
          Đang tải…
        </p>
      ) : items.length === 0 ? (
        <EmptyState />
      ) : (
        <ul
          className="divide-y overflow-hidden rounded-xl border"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          {items.map((p) => (
            <PlaylistRow
              key={p.id}
              playlist={p}
              confirming={confirming === p.id}
              busy={remove.isPending && confirming === p.id}
              onAskDelete={() => {
                setErr(null);
                setConfirming(p.id);
              }}
              onCancelDelete={() => setConfirming(null)}
              onConfirmDelete={() => remove.mutate(p.id)}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

function PlaylistRow({
  playlist,
  confirming,
  busy,
  onAskDelete,
  onCancelDelete,
  onConfirmDelete,
}: {
  playlist: Playlist;
  confirming: boolean;
  busy: boolean;
  onAskDelete: () => void;
  onCancelDelete: () => void;
  onConfirmDelete: () => void;
}) {
  return (
    <li className="flex items-center gap-3 px-3 py-3 transition hover:bg-[var(--tpl-surface-2)]">
      <span
        className="grid h-10 w-10 shrink-0 place-items-center rounded-lg text-white"
        style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
      >
        <Icon name="music-open-playlist-icon" size={16} />
      </span>

      <div className="min-w-0 flex-1">
        <Link
          href={`/library/music/playlists/${playlist.id}` as Route}
          className="block truncate text-sm font-semibold hover:underline"
          style={{ color: "var(--tpl-heading)" }}
        >
          {playlist.name}
        </Link>
        <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
          {playlist.track_count} bài
          {playlist.description ? ` · ${playlist.description}` : ""}
        </p>
      </div>

      {/* Delete asks in place rather than through a dialog: the playlist is one
          row, and a modal for "are you sure" on a list item is heavier than the
          action it guards. The tracks themselves are untouched either way. */}
      {confirming ? (
        <div className="flex shrink-0 items-center gap-2">
          <span className="text-xs" style={{ color: "var(--tpl-muted)" }}>
            Xoá playlist?
          </span>
          <button
            type="button"
            onClick={onConfirmDelete}
            disabled={busy}
            className="rounded-md px-2 py-1 text-xs font-semibold text-white disabled:opacity-50"
            style={{ background: "#ef4444" }}
          >
            {busy ? "Đang xoá…" : "Xoá"}
          </button>
          <button
            type="button"
            onClick={onCancelDelete}
            className="text-xs font-semibold"
            style={{ color: "var(--tpl-muted)" }}
          >
            Huỷ
          </button>
        </div>
      ) : (
        <button
          type="button"
          onClick={onAskDelete}
          aria-label={`Xoá ${playlist.name}`}
          className="shrink-0 transition hover:text-[#ef4444]"
          style={{ color: "var(--tpl-muted)" }}
        >
          <Icon name="little-delete" size={15} />
        </button>
      )}
    </li>
  );
}

function EmptyState() {
  return (
    <div
      className="rounded-xl border px-6 py-10 text-center"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      <p className="text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
        Chưa có playlist nào
      </p>
      <p className="mx-auto mt-1 max-w-[46ch] text-xs leading-relaxed" style={{ color: "var(--tpl-muted)" }}>
        Đặt tên một cái ở trên, rồi sang{" "}
        <Link href={"/library/music" as Route} className="font-semibold hover:underline" style={{ color: "var(--tpl-accent)" }}>
          thư viện nhạc
        </Link>
        , tích chọn các bài và bấm <strong>Thêm vào playlist</strong>.
      </p>
    </div>
  );
}
