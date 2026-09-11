"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "../../../components/ui/Icon";
import { useMusicPlayerOptional } from "../../../components/music/MusicPlayerProvider";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  deleteTrack,
  getTrack,
  isPlayable,
  publishTrack,
  trackArtist,
  trackCoverURL,
  unpublishTrack,
  updateTrack,
  type Track,
} from "@/lib/music";

/**
 * Library · Nhạc · chi tiết — one track.
 *
 * Play routes through the app-wide `MusicPlayerProvider` (same engine as the
 * index and the home widget), so opening a track here and pressing play leaves
 * the docked bar in charge; navigating away does not interrupt it.
 *
 * Metadata editing is inline and only patches fields that actually changed — the
 * PATCH contract treats an absent key as "unchanged" and an explicit `null` as
 * "clear", so sending the whole form back would silently clobber fields this
 * form does not show.
 */
export function MusicDetailView({ id }: { id: string }) {
  const qc = useQueryClient();
  const player = useMusicPlayerOptional();

  const { data: track, isPending, isError, refetch } = useQuery({
    queryKey: ["tracks", id],
    queryFn: () => getTrack(id),
  });

  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({ title: "", artist: "", album: "", description: "" });
  const [err, setErr] = useState<string | null>(null);

  // Seed the edit form whenever a fresh record arrives.
  useEffect(() => {
    if (!track) return;
    setForm({
      title: track.title,
      artist: track.artist ?? "",
      album: track.album ?? "",
      description: track.description ?? "",
    });
  }, [track]);

  const save = useMutation({
    mutationFn: () => {
      if (!track) throw new Error("no track");
      // Only changed fields; "" means "clear" → null.
      const patch: Record<string, string | null> = {};
      if (form.title.trim() && form.title.trim() !== track.title) patch.title = form.title.trim();
      const norm = (s: string) => (s.trim() ? s.trim() : null);
      if (norm(form.artist) !== track.artist) patch.artist = norm(form.artist);
      if (norm(form.album) !== track.album) patch.album = norm(form.album);
      if (norm(form.description) !== track.description) patch.description = norm(form.description);
      return updateTrack(track.id, patch);
    },
    onSuccess: () => {
      setEditing(false);
      qc.invalidateQueries({ queryKey: ["tracks"] });
    },
    onError: (e) => setErr(msg(e, "Không lưu được thay đổi.")),
  });

  const publish = useMutation({
    mutationFn: () => (track!.status === "published" ? unpublishTrack(id) : publishTrack(id)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tracks"] }),
    onError: (e) => setErr(msg(e, "Không đổi được trạng thái.")),
  });

  const remove = useMutation({
    mutationFn: () => deleteTrack(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["tracks"] }),
    onError: (e) => setErr(msg(e, "Không xoá được bài hát.")),
  });

  if (isPending) return <DetailSkeleton />;
  if (isError || !track) {
    return (
      <div className="rounded-xl border py-12 text-center" style={{ borderColor: "var(--tpl-border)" }}>
        <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
          Không tìm thấy bài hát này.
        </p>
        <div className="mt-3 flex justify-center gap-2">
          <button
            type="button"
            onClick={() => refetch()}
            className="rounded-md border px-3 py-1.5 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
          >
            Thử lại
          </button>
          <Link
            href={"/library/music" as Route}
            className="rounded-md px-3 py-1.5 text-sm font-semibold text-white"
            style={{ background: "var(--tpl-accent)" }}
          >
            Về thư viện
          </Link>
        </div>
      </div>
    );
  }

  const current = player?.isCurrent(track.id) ?? false;
  const playing = current && (player?.playing ?? false);
  const canPlay = isPlayable(track);

  return (
    <section className="space-y-6">
      <Link
        href={"/library/music" as Route}
        className="inline-flex items-center gap-1.5 text-sm font-semibold transition hover:opacity-80"
        style={{ color: "var(--tpl-muted)" }}
      >
        <Icon name="popup-left-arrow" size={12} />
        Nhạc
      </Link>

      {err && <Banner onDismiss={() => setErr(null)}>{err}</Banner>}

      <div className="flex flex-col gap-6 sm:flex-row">
        <div
          className="relative h-44 w-44 shrink-0 overflow-hidden rounded-xl"
          style={{ background: "var(--tpl-surface-2)" }}
        >
          {track.cover_asset_id ? (
            // eslint-disable-next-line @next/next/no-img-element -- dynamic, API-proxied variant, not a static/optimizable asset
            <img
              src={trackCoverURL(track.cover_asset_id, "medium")}
              alt=""
              className="h-full w-full object-cover"
            />
          ) : (
            <div className="grid h-full w-full place-items-center" style={{ color: "var(--tpl-muted)" }}>
              <Icon name="headphones-icon" size={34} />
            </div>
          )}
        </div>

        <div className="min-w-0 flex-1">
          {editing ? (
            <div className="space-y-3">
              <Field label="Tên bài hát" value={form.title} onChange={(v) => setForm({ ...form, title: v })} />
              <Field label="Nghệ sĩ" value={form.artist} onChange={(v) => setForm({ ...form, artist: v })} />
              <Field label="Album" value={form.album} onChange={(v) => setForm({ ...form, album: v })} />
              <Field
                label="Mô tả"
                value={form.description}
                onChange={(v) => setForm({ ...form, description: v })}
                multiline
              />
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => save.mutate()}
                  disabled={save.isPending || !form.title.trim()}
                  className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
                  style={{ background: "var(--tpl-accent)" }}
                >
                  {save.isPending ? "Đang lưu…" : "Lưu"}
                </button>
                <button
                  type="button"
                  onClick={() => setEditing(false)}
                  className="rounded-lg border px-4 py-2 text-sm font-medium transition hover:bg-[var(--tpl-surface-2)]"
                  style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
                >
                  Huỷ
                </button>
              </div>
            </div>
          ) : (
            <>
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
                  {track.title}
                </h1>
                <span
                  className="rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase text-white"
                  style={{
                    background: track.status === "published" ? "var(--tpl-accent)" : "rgba(0,0,0,.45)",
                  }}
                >
                  {track.status === "published" ? "đã đăng" : "nháp"}
                </span>
              </div>
              <p className="mt-1 text-sm" style={{ color: "var(--tpl-muted)" }}>
                {trackArtist(track)}
                {track.album ? ` · ${track.album}` : ""}
              </p>
              {track.description && (
                <p className="mt-3 whitespace-pre-wrap text-sm" style={{ color: "var(--tpl-text)" }}>
                  {track.description}
                </p>
              )}

              {!canPlay && (
                <p className="mt-3 text-xs" style={{ color: "#ef4444" }}>
                  Bài hát chưa gắn tệp âm thanh nên chưa phát được.
                </p>
              )}

              <div className="mt-5 flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={() => (current ? player?.toggle() : player?.playTrack(track))}
                  disabled={!canPlay}
                  className="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-40"
                  style={{
                    background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))",
                  }}
                >
                  <Icon name={playing ? "music-pause-icon" : "music-play-icon-big"} size={14} />
                  {playing ? "Tạm dừng" : "Phát"}
                </button>

                <button
                  type="button"
                  onClick={() => setEditing(true)}
                  className="rounded-lg border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
                  style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
                >
                  Sửa
                </button>

                <button
                  type="button"
                  onClick={() => publish.mutate()}
                  disabled={publish.isPending}
                  className="rounded-lg border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50"
                  style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-accent)" }}
                >
                  {track.status === "published" ? "Gỡ xuống" : "Đăng"}
                </button>

                <button
                  type="button"
                  onClick={() => remove.mutate()}
                  disabled={remove.isPending}
                  className="ml-auto flex items-center gap-1.5 text-sm font-semibold transition hover:opacity-80 disabled:opacity-50"
                  style={{ color: "#ef4444" }}
                >
                  <Icon name="little-delete" size={14} />
                  Xoá
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </section>
  );
}

/* ── pieces ──────────────────────────────────────────────────────── */

function msg(e: unknown, fallback: string): string {
  return e instanceof ApiError ? problemDisplayMessage(e.body) : fallback;
}

function Field({
  label,
  value,
  onChange,
  multiline,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  multiline?: boolean;
}) {
  const style = {
    borderColor: "var(--tpl-border)",
    background: "var(--tpl-bg)",
    color: "var(--tpl-text)",
  };
  return (
    <div>
      <label className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
        {label}
      </label>
      {multiline ? (
        <textarea
          value={value}
          rows={3}
          onChange={(e) => onChange(e.target.value)}
          className="w-full rounded-lg border px-3 py-2 text-sm"
          style={style}
        />
      ) : (
        <input
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-full rounded-lg border px-3 py-2 text-sm"
          style={style}
        />
      )}
    </div>
  );
}

function Banner({ children, onDismiss }: { children: ReactNode; onDismiss?: () => void }) {
  return (
    <p
      className="flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm"
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

function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-6 sm:flex-row">
      <div className="h-44 w-44 shrink-0 animate-pulse rounded-xl" style={{ background: "var(--tpl-surface-2)" }} />
      <div className="flex-1 space-y-3">
        <div className="h-6 w-1/2 animate-pulse rounded" style={{ background: "var(--tpl-surface-2)" }} />
        <div className="h-3 w-1/3 animate-pulse rounded" style={{ background: "var(--tpl-surface-2)" }} />
        <div className="h-9 w-32 animate-pulse rounded-lg" style={{ background: "var(--tpl-surface-2)" }} />
      </div>
    </div>
  );
}
