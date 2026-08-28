"use client";

import { useMemo, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import type { Route } from "next";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Icon } from "../../../components/ui/Icon";
import { BulkImportModal } from "./BulkImportModal";
import { useMusicPlayerOptional } from "../../../components/music/MusicPlayerProvider";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import { listAssets } from "@/lib/media-assets";
import { useInfiniteScroll } from "@/lib/use-infinite-scroll";
import {
  addTracksToPlaylist,
  bulkSetTrackStatus,
  createPlaylist,
  createTrack,
  deleteTrack,
  fetchAllTracks,
  isPlayable,
  listMyTracks,
  listPlaylists,
  listTracks,
  publishTrack,
  trackArtist,
  trackCoverURL,
  unpublishTrack,
  uploadAudioAsset,
  type Playlist,
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
 *
 * Both tabs paginate with `useInfiniteQuery`. The API has always been
 * keyset-paginated (30 a page, cursor on `next_cursor`) but this view used a
 * plain `useQuery` and threw the cursor away, so a library of any size looked
 * like exactly 30 tracks with nothing to say otherwise.
 *
 * Pages load on scroll, not on a button — a "load more" click every 30 rows is
 * a poor way to walk a few hundred tracks. `useInfiniteScroll` watches a
 * sentinel below the list and fetches ahead of the fold.
 *
 * "Phát tất cả" therefore cannot queue `tracks` — that is only the pages fetched
 * so far. It walks the cursor to the end first (see `fetchAllTracks`), because a
 * button that says "all" and plays the first 30 is worse than one that takes a
 * moment.
 */
export function MusicIndexView() {
  const qc = useQueryClient();
  const player = useMusicPlayerOptional();

  const [tab, setTab] = useState<"all" | "mine">("all");

  // Bulk selection lives on the "Của tôi" tab only: the published catalogue has
  // nothing you can do to a selection. Keyed by track id so it survives paging.
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkError, setBulkError] = useState<string | null>(null);
  const [bulkNote, setBulkNote] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [artist, setArtist] = useState("");
  const [album, setAlbum] = useState("");
  const [audioAssetId, setAudioAssetId] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [rowErr, setRowErr] = useState<string | null>(null);
  const [uploadPct, setUploadPct] = useState<number | null>(null);
  const fileRef = useRef<HTMLInputElement | null>(null);

  const all = useInfiniteQuery({
    queryKey: ["tracks", "published"],
    queryFn: ({ pageParam }: { pageParam: string | undefined }) => listTracks(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
    retry: retryUnlessClientError,
  });
  const mine = useInfiniteQuery({
    queryKey: ["tracks", "mine"],
    queryFn: ({ pageParam }: { pageParam: string | undefined }) => listMyTracks(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
    enabled: tab === "mine",
    retry: retryUnlessClientError,
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

  // The same 403 that drives ErrorState also hides "Thêm bài hát": offering a
  // create button to an account that cannot create only produces a second error.
  const playlists = useQuery({
    queryKey: ["playlists"],
    queryFn: listPlaylists,
    enabled: tab === "mine",
  });

  function clearSelection() {
    setSelected(new Set());
    setBulkError(null);
    setBulkNote(null);
  }

  function toggleSelected(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
    setBulkNote(null);
  }

  // Every bulk action reports what actually happened rather than assuming: a
  // selection can contain tracks already published, or already in the playlist.
  function afterBulk(note: string) {
    setBulkError(null);
    setBulkNote(note);
    clearSelectionKeepNote(note);
    qc.invalidateQueries({ queryKey: ["tracks"] });
    qc.invalidateQueries({ queryKey: ["playlists"] });
  }

  function clearSelectionKeepNote(note: string) {
    setSelected(new Set());
    setBulkNote(note);
  }

  function onBulkError(e: unknown, fallback: string) {
    setBulkNote(null);
    setBulkError(e instanceof ApiError ? problemDisplayMessage(e.body) : fallback);
  }

  // "Chọn tất cả" walks the cursor rather than reading the rendered array —
  // frontend/CLAUDE.md: that array is only the pages fetched so far, so on an
  // imported library of hundreds the button's name would be a lie. Same source
  // of truth as "Phát tất cả".
  const selectAll = useMutation({
    mutationFn: () => fetchAllTracks(tab === "mine" ? "mine" : "published"),
    onSuccess: ({ tracks: all, truncated }) => {
      setSelected(new Set(all.map((t) => t.id)));
      setBulkError(null);
      // fetchAllTracks is bounded; say so rather than let the count imply the
      // whole library was selected when it was not.
      setBulkNote(truncated ? `Đã chọn ${all.length} bài đầu tiên (danh sách bị giới hạn).` : null);
    },
    onError: (e) => onBulkError(e, "Không tải được toàn bộ danh sách để chọn."),
  });

  const bulkStatus = useMutation({
    mutationFn: ({ ids, status }: { ids: string[]; status: "published" | "draft" }) =>
      bulkSetTrackStatus(ids, status),
    onSuccess: (r, v) =>
      afterBulk(
        v.status === "published"
          ? `Đã đăng ${r.changed}/${r.requested} bài.`
          : `Đã gỡ ${r.changed}/${r.requested} bài.`,
      ),
    onError: (e) => onBulkError(e, "Không đổi được trạng thái."),
  });

  const addToPlaylist = useMutation({
    mutationFn: ({ playlistID, ids }: { playlistID: string; ids: string[] }) =>
      addTracksToPlaylist(playlistID, ids),
    onSuccess: (r) =>
      afterBulk(
        r.added === r.requested
          ? `Đã thêm ${r.added} bài vào playlist.`
          : `Đã thêm ${r.added}/${r.requested} bài — số còn lại đã có sẵn trong playlist.`,
      ),
    onError: (e) => onBulkError(e, "Không thêm được vào playlist."),
  });

  const newPlaylist = useMutation({
    mutationFn: ({ name, ids }: { name: string; ids: string[] }) =>
      createPlaylist(name).then((p) =>
        addTracksToPlaylist(p.id, ids).then((r) => ({ ...r, name: p.name })),
      ),
    onSuccess: (r) => afterBulk(`Đã tạo “${r.name}” với ${r.added} bài.`),
    onError: (e) => onBulkError(e, "Không tạo được playlist."),
  });

  const mineForbidden = mine.error instanceof ApiError && mine.error.status === 403;
  const active = tab === "all" ? all : mine;
  const tracks = useMemo(
    () => active.data?.pages.flatMap((p) => p.tracks) ?? [],
    [active.data],
  );
  const playable = useMemo(() => tracks.filter(isPlayable), [tracks]);

  /**
   * Queue the whole scope, not just the pages on screen.
   *
   * Runs as a mutation rather than a query so the button gets a real pending
   * state: the walk is sequential (each page needs the previous cursor), so on a
   * large library it is visibly not instant and a dead-looking button would read
   * as another broken control.
   */
  const sentinelRef = useInfiniteScroll({
    onLoadMore: () => active.fetchNextPage(),
    hasMore: active.hasNextPage,
    isLoading: active.isFetchingNextPage,
  });

  const playAll = useMutation({
    mutationFn: () => fetchAllTracks(tab === "mine" ? "mine" : "published"),
    onMutate: () => setRowErr(null),
    onSuccess: ({ tracks: fetched, truncated }) => {
      const queue = fetched.filter(isPlayable);
      if (queue.length === 0) {
        setRowErr("Không có bài hát nào đã gắn tệp âm thanh để phát.");
        return;
      }
      player?.playQueue(queue, 0);
      if (truncated) {
        setRowErr(`Danh sách quá dài — đã xếp hàng ${queue.length} bài đầu tiên.`);
      }
    },
    onError: (e) => setRowErr(rowMessage(e, "Không tải được toàn bộ danh sách.")),
  });

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
          onClick={() => playAll.mutate()}
          disabled={playable.length === 0 || playAll.isPending}
          className="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-40"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          <Icon name="music-play-icon-big" size={14} />
          {playAll.isPending ? "Đang xếp hàng…" : "Phát tất cả"}
        </button>

        {tab === "mine" && !mineForbidden && (
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

        {tab === "mine" && !mineForbidden && (
          <button
            type="button"
            onClick={() => setBulkOpen(true)}
            className="rounded-lg border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
          >
            Nhập nhiều bài
          </button>
        )}
      </div>

      {rowErr && <Banner onDismiss={() => setRowErr(null)}>{rowErr}</Banner>}

      {active.isPending ? (
        <SkeletonList />
      ) : active.isError ? (
        <ErrorState error={active.error} onRetry={() => active.refetch()} />
      ) : tracks.length === 0 ? (
        <EmptyState mine={tab === "mine"} />
      ) : (
        <>
        {tab === "mine" && (
          <SelectionBar
            total={tracks.length}
            selected={selected}
            playlists={playlists.data ?? []}
            busy={bulkStatus.isPending || addToPlaylist.isPending || newPlaylist.isPending}
            note={bulkNote}
            error={bulkError}
            onSelectAll={() => selectAll.mutate()}
            selecting={selectAll.isPending}
            onClear={clearSelection}
            onPublish={() => bulkStatus.mutate({ ids: [...selected], status: "published" })}
            onUnpublish={() => bulkStatus.mutate({ ids: [...selected], status: "draft" })}
            onAddTo={(playlistID) => addToPlaylist.mutate({ playlistID, ids: [...selected] })}
            onCreateWith={(name) => newPlaylist.mutate({ name, ids: [...selected] })}
          />
        )}
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
              selectable={tab === "mine"}
              selected={selected.has(t.id)}
              onToggleSelect={() => toggleSelected(t.id)}
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
        </>
      )}

      {tracks.length > 0 && (
        <>
          {/* The scroll sentinel. Empty and zero-height by design: it exists to
              be intersected, and any padding of its own would show up as a gap
              under the last row. */}
          <div ref={sentinelRef} aria-hidden />

          <div className="mt-4 flex flex-col items-center gap-2" aria-live="polite">
            {active.isFetchingNextPage && <RowSkeleton />}
            <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
              {/* "30" on its own reads as the whole library; the trailing "+" is
                  the only signal that there is more behind it. */}
              {active.hasNextPage
                ? `Đang hiển thị ${tracks.length}+ bài hát`
                : `${tracks.length} bài hát`}
            </p>
          </div>
        </>
      )}

      {bulkOpen && (
        <BulkImportModal
          onClose={() => setBulkOpen(false)}
          onImported={() => qc.invalidateQueries({ queryKey: ["tracks"] })}
        />
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

/**
 * The bulk toolbar for the "Của tôi" tab.
 *
 * It is always present once there is anything to select — a toolbar that appears
 * only after the first tick is a feature nobody discovers. With nothing selected
 * it offers "select all"; with a selection it offers what you can do to it, and
 * says how many rows that is so a stray click on "all" is visible before it is
 * acted on.
 */
function SelectionBar({
  total,
  selected,
  playlists,
  busy,
  selecting,
  note,
  error,
  onSelectAll,
  onClear,
  onPublish,
  onUnpublish,
  onAddTo,
  onCreateWith,
}: {
  total: number;
  selected: Set<string>;
  playlists: Playlist[];
  busy: boolean;
  selecting: boolean;
  note: string | null;
  error: string | null;
  onSelectAll: () => void;
  onClear: () => void;
  onPublish: () => void;
  onUnpublish: () => void;
  onAddTo: (playlistID: string) => void;
  onCreateWith: (name: string) => void;
}) {
  const [picking, setPicking] = useState(false);
  const [newName, setNewName] = useState("");
  const count = selected.size;
  const allSelected = total > 0 && count === total;

  return (
    <div
      className="mb-3 rounded-xl border px-3 py-2.5"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      <div className="flex flex-wrap items-center gap-3">
        <label className="flex shrink-0 items-center gap-2 text-sm" style={{ color: "var(--tpl-text)" }}>
          <input
            type="checkbox"
            checked={allSelected}
            onChange={() => (allSelected ? onClear() : onSelectAll())}
            aria-label="Chọn tất cả bài đang hiển thị"
            className="h-4 w-4 accent-[var(--tpl-accent)]"
          />
          {count > 0
            ? `Đã chọn ${count}`
            : selecting
              ? "Đang tải danh sách…"
              : `Chọn tất cả (${total}+ đang hiển thị)`}
        </label>

        {count > 0 && (
          <>
            <BulkBtn onClick={onPublish} disabled={busy} primary>
              Đăng
            </BulkBtn>
            <BulkBtn onClick={onUnpublish} disabled={busy}>
              Gỡ
            </BulkBtn>

            <div className="relative">
              <BulkBtn onClick={() => setPicking((v) => !v)} disabled={busy}>
                Thêm vào playlist ▾
              </BulkBtn>
              {picking && (
                <div
                  className="absolute left-0 top-9 z-20 w-64 rounded-lg py-1 shadow-lg"
                  style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
                >
                  {playlists.length === 0 && (
                    <p className="px-3 py-2 text-xs" style={{ color: "var(--tpl-muted)" }}>
                      Chưa có playlist nào.
                    </p>
                  )}
                  {playlists.map((pl) => (
                    <button
                      key={pl.id}
                      type="button"
                      onClick={() => {
                        setPicking(false);
                        onAddTo(pl.id);
                      }}
                      className="flex w-full items-center justify-between px-3 py-2 text-left text-sm transition hover:bg-[var(--tpl-surface-2)]"
                      style={{ color: "var(--tpl-text)" }}
                    >
                      <span className="truncate">{pl.name}</span>
                      <span className="ml-2 shrink-0 text-xs" style={{ color: "var(--tpl-muted)" }}>
                        {pl.track_count}
                      </span>
                    </button>
                  ))}
                  <div className="mt-1 border-t px-2 pt-2" style={{ borderColor: "var(--tpl-border)" }}>
                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        const name = newName.trim();
                        if (!name) return;
                        setNewName("");
                        setPicking(false);
                        onCreateWith(name);
                      }}
                      className="flex gap-1"
                    >
                      <input
                        value={newName}
                        onChange={(e) => setNewName(e.target.value)}
                        placeholder="Playlist mới…"
                        className="min-w-0 flex-1 rounded-md border bg-transparent px-2 py-1 text-xs outline-none"
                        style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
                      />
                      <button
                        type="submit"
                        disabled={!newName.trim()}
                        className="rounded-md px-2 py-1 text-xs font-semibold text-white disabled:opacity-50"
                        style={{ background: "var(--tpl-accent)" }}
                      >
                        Tạo
                      </button>
                    </form>
                  </div>
                </div>
              )}
            </div>

            <button
              type="button"
              onClick={onClear}
              className="text-xs font-semibold transition hover:opacity-80"
              style={{ color: "var(--tpl-muted)" }}
            >
              Bỏ chọn
            </button>
          </>
        )}
      </div>

      {(note || error) && (
        <p
          className="mt-2 text-xs"
          role={error ? "alert" : undefined}
          style={{ color: error ? "#ef4444" : "var(--tpl-muted)" }}
        >
          {error ?? note}
        </p>
      )}
    </div>
  );
}

function BulkBtn({
  children,
  onClick,
  disabled,
  primary,
}: {
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  primary?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="shrink-0 rounded-md px-3 py-1.5 text-xs font-semibold transition hover:opacity-90 disabled:opacity-50"
      style={
        primary
          ? { background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))", color: "#fff" }
          : { border: "1px solid var(--tpl-border)", color: "var(--tpl-muted)" }
      }
    >
      {children}
    </button>
  );
}

function TrackRow({
  track,
  index,
  showStatus,
  selectable,
  selected,
  onToggleSelect,
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
  selectable: boolean;
  selected: boolean;
  onToggleSelect: () => void;
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
      {selectable ? (
        <input
          type="checkbox"
          checked={selected}
          onChange={onToggleSelect}
          aria-label={`Chọn ${track.title}`}
          className="h-4 w-4 shrink-0 accent-[var(--tpl-accent)]"
        />
      ) : (
        <span
          className="w-5 shrink-0 text-center text-xs tabular-nums"
          style={{ color: current ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
        >
          {index}
        </span>
      )}

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

/** Shown under the list while the next page is in flight. */
function RowSkeleton() {
  return (
    <div className="flex w-full items-center gap-3 px-3 py-2">
      <div
        className="h-10 w-10 shrink-0 animate-pulse rounded-md"
        style={{ background: "var(--tpl-surface-2)" }}
      />
      <div className="flex-1 space-y-1.5">
        <div className="h-3 w-1/3 animate-pulse rounded" style={{ background: "var(--tpl-surface-2)" }} />
        <div className="h-2.5 w-1/5 animate-pulse rounded" style={{ background: "var(--tpl-surface-2)" }} />
      </div>
    </div>
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

/**
 * "Của tôi" needs `music:write:own`, which the default `user` role does not
 * carry — so a 403 here is an ordinary state, not a fault. It gets its own copy
 * and no retry button: the request cannot start succeeding on its own, and the
 * generic "couldn't load / try again" pair invited a loop that never ends.
 */
function ErrorState({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const forbidden = error instanceof ApiError && error.status === 403;

  return (
    <div className="rounded-xl border py-12 text-center" style={{ borderColor: "var(--tpl-border)" }}>
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        {forbidden
          ? "Tài khoản của bạn chưa có quyền đăng nhạc — cần vai trò “creator” trở lên."
          : "Không tải được danh sách nhạc."}
      </p>
      {!forbidden && (
        <button
          type="button"
          onClick={onRetry}
          className="mt-3 rounded-md border px-3 py-1.5 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
        >
          Thử lại
        </button>
      )}
    </div>
  );
}

/**
 * A 4xx is the server's final answer — retrying a 403 or a 404 just burns three
 * more round trips before the same error lands. Only 5xx/network faults retry.
 */
function retryUnlessClientError(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false;
  return failureCount < 3;
}
