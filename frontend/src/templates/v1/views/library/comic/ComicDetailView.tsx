"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  type Chapter,
  type ComicDetail,
  type ReadingDirection,
  createChapter,
  createPages,
  deleteChapter,
  deleteComic,
  deletePage,
  getChapterPages,
  getComic,
  publishComic,
  reorderChapters,
  reorderPages,
  unpublishComic,
  updateChapter,
  updateComic,
  variantURL,
} from "@/lib/comic";
import { uploadImage } from "@/lib/media-upload";
import { runComicZipImport, runZipImport, type RunImportProgress } from "@/lib/comic-import";
import { listSyncSources, createSyncSource, triggerSync, cancelSync, deleteSyncSource, type SyncSource, type SyncStatus } from "@/lib/comic-sync";

// Comic detail (SPEC-02 P0.5) — a read view for everyone, plus a full owner manager
// (edit metadata + cover, chapter CRUD/reorder, page upload/reorder/delete).
export function ComicDetailView({ id }: { id: string }) {
  const { data: comic, isLoading } = useQuery({ queryKey: ["comic", id], queryFn: () => getComic(id) });
  const { data: me } = useQuery({ queryKey: ["me"], queryFn: () => api<{ id?: string }>("/api/v1/auth/me").catch((): { id?: string } => ({})) });

  if (isLoading) return <p style={{ color: "var(--tpl-muted)" }}>Đang tải…</p>;
  if (!comic) return <p style={{ color: "var(--tpl-heading)" }}>Không tìm thấy truyện.</p>;

  const isOwner = !!me?.id && me.id === comic.owner_id;
  return isOwner ? <OwnerManager id={id} comic={comic} /> : <ReadView id={id} comic={comic} />;
}

/* ── Read view (non-owner / readers) ───────────────────────────────── */

function ReadView({ id, comic }: { id: string; comic: ComicDetail }) {
  const continueHref = comic.progress
    ? (`/library/comic/${id}/read/${comic.progress.chapter_id}` as Route)
    : comic.chapters[0]
      ? (`/library/comic/${id}/read/${comic.chapters[0].id}` as Route)
      : null;

  return (
    <div style={{ color: "var(--tpl-text)" }}>
      <div className="flex gap-6">
        <Cover assetId={comic.cover_asset_id} title={comic.title} />
        <div className="flex-1">
          <h1 className="text-2xl font-bold" style={{ color: "var(--tpl-heading)" }}>{comic.title}</h1>
          {comic.description && <p className="mt-2 text-sm" style={{ color: "var(--tpl-muted)" }}>{comic.description}</p>}
          {continueHref && (
            <Link href={continueHref} className="mt-4 inline-block rounded-lg px-5 py-2.5 text-sm font-semibold text-white transition hover:opacity-90" style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}>
              {comic.progress ? "Đọc tiếp" : "Bắt đầu đọc"}
            </Link>
          )}
        </div>
      </div>
      <ChapterList id={id} chapters={comic.chapters} />
    </div>
  );
}

function ChapterList({ id, chapters }: { id: string; chapters: Chapter[] }) {
  return (
    <>
      <h2 className="mb-2 mt-8 text-xs font-semibold uppercase tracking-wider" style={{ color: "var(--tpl-muted)" }}>Chương</h2>
      <ul className="overflow-hidden rounded-xl border" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
        {chapters.map((ch, i) => (
          <li key={ch.id} className="border-t first:border-t-0" style={{ borderColor: "var(--tpl-border)" }}>
            <Link href={`/library/comic/${id}/read/${ch.id}` as Route} className="flex justify-between px-4 py-3 text-sm transition hover:bg-[var(--tpl-surface-2)]">
              <span style={{ color: "var(--tpl-heading)" }}>{i + 1}. {ch.title}</span>
              <span style={{ color: "var(--tpl-muted)" }}>{new Date(ch.created_at).toLocaleDateString("vi-VN")}</span>
            </Link>
          </li>
        ))}
        {chapters.length === 0 && <li className="px-4 py-4 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>Chưa có chương nào.</li>}
      </ul>
    </>
  );
}

/* ── Owner manager ─────────────────────────────────────────────────── */

function OwnerManager({ id, comic }: { id: string; comic: ComicDetail }) {
  return (
    <div className="space-y-6" style={{ color: "var(--tpl-text)" }}>
      <div className="flex items-center justify-between">
        <div>
          <Link href={"/library/comic" as Route} className="mb-1 inline-flex items-center gap-1 text-sm font-medium transition hover:opacity-80" style={{ color: "var(--tpl-muted)" }}>
            ← Danh sách truyện
          </Link>
          <h1 className="text-2xl font-bold" style={{ color: "var(--tpl-heading)" }}>{comic.title}</h1>
          <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>Quản lý truyện · {comic.chapters.length} chương</p>
        </div>
        {comic.chapters[0] && (
          <Link href={`/library/comic/${id}/read/${comic.chapters[0].id}` as Route} className="rounded-lg border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-heading)" }}>
            Đọc thử
          </Link>
        )}
      </div>

      <SettingsCard id={id} comic={comic} />
      <SyncSourcesManager id={id} />
      <ChaptersManager id={id} comic={comic} />
    </div>
  );
}

/* ── Sync sources (P1.8) ────────────────────────────────────────────── */

function SyncSourcesManager({ id }: { id: string }) {
  const qc = useQueryClient();
  const [url, setUrl] = useState("");
  const [hint, setHint] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const sources = useQuery({
    queryKey: ["sync-sources", id],
    queryFn: () => listSyncSources(id),
    refetchInterval: (q) => ((q.state.data ?? []).some((s) => s.last_status === "syncing") ? 3000 : false),
  });
  const list = sources.data ?? [];

  const onErr = (e: unknown) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : e instanceof Error ? e.message : "Có lỗi xảy ra");
  const invalidate = () => qc.invalidateQueries({ queryKey: ["sync-sources", id] });
  const add = useMutation({ mutationFn: () => createSyncSource(id, { source_url: url.trim(), chapters_hint: hint.trim() }), onSuccess: () => { setErr(null); setUrl(""); setHint(""); invalidate(); }, onError: onErr });
  const sync = useMutation({ mutationFn: (sid: string) => triggerSync(sid), onSuccess: () => { setErr(null); invalidate(); }, onError: onErr });
  const cancel = useMutation({ mutationFn: (sid: string) => cancelSync(sid), onSuccess: () => { setErr(null); invalidate(); }, onError: onErr });
  const del = useMutation({ mutationFn: (sid: string) => deleteSyncSource(sid), onSuccess: invalidate, onError: onErr });

  return (
    <div className="rounded-2xl border p-5" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
      <h2 className="text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>Nguồn đồng bộ</h2>
      <p className="mt-0.5 text-xs" style={{ color: "var(--tpl-muted)" }}>Tự động cào truyện từ nguồn ngoài rồi nhập vào bộ này (chạy nền).</p>
      {err && <p className="mt-2 text-sm" style={{ color: "#ef4444" }}>{err}</p>}

      <div className="mt-3 space-y-2">
        {list.map((s) => (
          <SyncSourceRow key={s.id} source={s} onSync={() => sync.mutate(s.id)} onCancel={() => cancel.mutate(s.id)} onDelete={() => del.mutate(s.id)} busy={sync.isPending || del.isPending || cancel.isPending} />
        ))}
        {list.length === 0 && <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>Chưa có nguồn nào — thêm bên dưới.</p>}
      </div>

      <form className="mt-4 flex flex-col gap-2 sm:flex-row" onSubmit={(e) => { e.preventDefault(); if (url.trim()) add.mutate(); }}>
        <input className="flex-1 rounded-lg border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)]" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} placeholder="URL trang truyện (vd https://truyenqqno.com/truyen-tranh/...)" value={url} onChange={(e) => setUrl(e.target.value)} />
        <input className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)] sm:w-44" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} placeholder="Chương (trống = tất cả, vd 1-50)" value={hint} onChange={(e) => setHint(e.target.value)} />
        <button type="submit" disabled={add.isPending || !url.trim()} className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50" style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}>Thêm nguồn</button>
      </form>
    </div>
  );
}

function SyncSourceRow({ source, onSync, onCancel, onDelete, busy }: { source: SyncSource; onSync: () => void; onCancel: () => void; onDelete: () => void; busy: boolean }) {
  const badge: Record<SyncStatus, { label: string; color: string }> = {
    idle: { label: "Chưa chạy", color: "var(--tpl-muted)" },
    syncing: { label: "Đang cào…", color: "#f59e0b" },
    done: { label: "Đã cào xong", color: "#22c55e" },
    failed: { label: "Lỗi", color: "#ef4444" },
    cancelled: { label: "Đã ngừng", color: "var(--tpl-muted)" },
  };
  const b = badge[source.last_status];
  const syncing = source.last_status === "syncing";
  const pct = source.total_chapters > 0 ? Math.round((source.scraped_chapters / source.total_chapters) * 100) : 0;

  return (
    <div className="flex items-center justify-between gap-3 rounded-lg border px-3 py-2" style={{ borderColor: "var(--tpl-border)" }}>
      <div className="min-w-0">
        <div className="truncate text-sm font-medium" style={{ color: "var(--tpl-heading)" }}>{source.source_site || source.source_url}</div>
        <a href={source.source_url} target="_blank" rel="noreferrer" className="block truncate text-xs hover:underline" style={{ color: "var(--tpl-muted)" }}>{source.source_url}</a>
        <div className="mt-0.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
          <span style={{ color: b.color }}>● {b.label}</span>
          {source.chapters_hint && <span> · chương {source.chapters_hint}</span>}
          {source.total_chapters > 0 && (source.last_status === "syncing"
            ? <span> · Cào: {source.scraped_chapters}/{source.total_chapters} chương ({pct}%)</span>
            : <span> · {source.total_chapters} chương</span>)}
          {source.last_error && <span style={{ color: "#ef4444" }}> · ⚠ {source.last_error}</span>}
        </div>
      </div>
      <div className="flex shrink-0 gap-2">
        {syncing ? (
          <button type="button" onClick={onCancel} disabled={busy} className="rounded-lg border px-3 py-1.5 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50" style={{ borderColor: "#ef4444", color: "#ef4444" }}>
            ⏹ Ngừng
          </button>
        ) : (
          <button type="button" onClick={onSync} disabled={busy} className="rounded-lg border px-3 py-1.5 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-heading)" }}>
            ⟳ Đồng bộ
          </button>
        )}
        <button type="button" onClick={onDelete} disabled={busy || syncing} className="rounded-lg border px-3 py-1.5 text-xs font-medium transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}>
          Xóa
        </button>
      </div>
    </div>
  );
}

function SettingsCard({ id, comic }: { id: string; comic: ComicDetail }) {
  const qc = useQueryClient();
  const router = useRouter();
  const titleRef = useRef<HTMLInputElement>(null);
  const descRef = useRef<HTMLTextAreaElement>(null);
  const coverInput = useRef<HTMLInputElement>(null);
  const [uploadingCover, setUploadingCover] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const invalidate = () => qc.invalidateQueries({ queryKey: ["comic", id] });
  const onErr = (e: unknown) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Có lỗi xảy ra");

  const save = useMutation({
    mutationFn: () => updateComic(id, { title: titleRef.current?.value.trim() || comic.title, description: descRef.current?.value.trim() || null }),
    onSuccess: () => { setErr(null); setSaved(true); setTimeout(() => setSaved(false), 1500); invalidate(); },
    onError: onErr,
  });
  const setDir = useMutation({ mutationFn: (d: ReadingDirection) => updateComic(id, { reading_direction: d }), onSuccess: invalidate, onError: onErr });
  const pub = useMutation({ mutationFn: () => (comic.status === "published" ? unpublishComic(id) : publishComic(id)), onSuccess: () => { setErr(null); invalidate(); }, onError: onErr });
  const del = useMutation({ mutationFn: () => deleteComic(id), onSuccess: () => { qc.invalidateQueries({ queryKey: ["comics"] }); router.push("/library/comic" as Route); }, onError: onErr });

  async function onCover(file: File) {
    setErr(null);
    setUploadingCover(true);
    try {
      const { assetId } = await uploadImage(file);
      await updateComic(id, { cover_asset_id: assetId });
      invalidate();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Không đặt được bìa");
    } finally {
      setUploadingCover(false);
    }
  }

  return (
    <div className="rounded-2xl border p-5" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
      <div className="flex flex-col gap-5 sm:flex-row">
        {/* Cover */}
        <div className="w-36 shrink-0">
          <Cover assetId={comic.cover_asset_id} title={comic.title} />
          <button type="button" onClick={() => coverInput.current?.click()} disabled={uploadingCover} className="mt-2 w-full rounded-lg border px-3 py-1.5 text-xs font-medium transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-heading)" }}>
            {uploadingCover ? "Đang tải…" : "Đổi bìa"}
          </button>
          <input ref={coverInput} type="file" accept="image/*" className="hidden" onChange={(e) => { const f = e.target.files?.[0]; if (f) void onCover(f); e.target.value = ""; }} />
        </div>

        {/* Metadata */}
        <div className="min-w-0 flex-1 space-y-3">
          <Field label="Tên truyện">
            <input ref={titleRef} defaultValue={comic.title} maxLength={200} className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)]" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} />
          </Field>
          <Field label="Mô tả">
            <textarea ref={descRef} defaultValue={comic.description ?? ""} rows={2} className="w-full resize-none rounded-lg border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)]" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} />
          </Field>
          <Field label="Hướng đọc">
            <Segmented
              value={comic.reading_direction}
              onChange={(v) => setDir.mutate(v as ReadingDirection)}
              options={[{ v: "vertical", label: "Cuộn dọc" }, { v: "ltr", label: "Trái → Phải" }, { v: "rtl", label: "Phải → Trái (manga)" }]}
            />
          </Field>

          {err && <p role="alert" className="rounded-lg px-3 py-2 text-sm" style={{ background: "rgba(239,68,68,.08)", color: "#ef4444" }}>{err}</p>}

          <div className="flex flex-wrap items-center gap-2 pt-1">
            <button type="button" onClick={() => save.mutate()} disabled={save.isPending} className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50" style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}>
              {save.isPending ? "Đang lưu…" : saved ? "Đã lưu ✓" : "Lưu"}
            </button>
            <button type="button" onClick={() => pub.mutate()} disabled={pub.isPending} className="rounded-lg border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-heading)" }}>
              {comic.status === "published" ? "Gỡ xuất bản" : "Xuất bản"}
            </button>
            <span className="rounded-md px-2 py-0.5 text-[11px] font-semibold uppercase text-white" style={{ background: comic.status === "published" ? "var(--tpl-accent)" : "rgba(0,0,0,.45)" }}>
              {comic.status === "published" ? "đã đăng" : "nháp"}
            </span>
            <div className="ml-auto">
              <DangerConfirm label="Xoá truyện" confirmLabel="Xoá thật?" onConfirm={() => del.mutate()} />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ── Chapters manager ──────────────────────────────────────────────── */

function ChaptersManager({ id, comic }: { id: string; comic: ComicDetail }) {
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const zipRef = useRef<HTMLInputElement>(null);
  const [zip, setZip] = useState<{ label: string; done: number; total: number } | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const chapters = comic.chapters;

  const invalidate = () => qc.invalidateQueries({ queryKey: ["comic", id] });
  const add = useMutation({ mutationFn: () => createChapter(id, { title: title.trim(), sort_order: (chapters.length + 1) * 10 }), onSuccess: () => { setTitle(""); invalidate(); } });
  const reorder = useMutation({ mutationFn: (order: string[]) => reorderChapters(id, order), onSuccess: invalidate });

  const move = (idx: number, delta: number) => {
    const ids = chapters.map((c) => c.id);
    const j = idx + delta;
    if (j < 0 || j >= ids.length) return;
    const tmp = ids[idx]!; ids[idx] = ids[j]!; ids[j] = tmp;
    reorder.mutate(ids);
  };

  // Import a whole comic from a multi-chapter zip (folder = chapter), server-side.
  async function onZip(file: File) {
    setNote(null);
    setZip({ label: "Đang tải zip…", done: 0, total: 0 });
    try {
      const job = await runComicZipImport(id, file, (p: RunImportProgress) => {
        if (p.phase === "uploading") setZip({ label: `Đang tải zip ${p.uploadPct ?? 0}%`, done: 0, total: 0 });
        else setZip({ label: "Đang xử lý", done: p.job?.succeeded ?? 0, total: p.job?.total ?? 0 });
      });
      qc.invalidateQueries({ queryKey: ["comic", id] });
      setNote(job.status === "done"
        ? `Đã nhập ${job.succeeded} trang${job.failed ? `, ${job.failed} lỗi` : ""} (bộ nhiều chương).`
        : `Nhập thất bại: ${job.error ?? "lỗi không rõ"}.`);
    } catch (e) {
      setNote(e instanceof Error ? e.message : "Nhập ZIP thất bại.");
    } finally {
      setZip(null);
    }
  }

  return (
    <div className="rounded-2xl border p-5" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
      <div className="mb-3 flex items-center justify-between gap-2">
        <h2 className="text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>Chương ({chapters.length})</h2>
        <button type="button" onClick={() => zipRef.current?.click()} disabled={!!zip} title="ZIP có các thư mục con = mỗi chương (hoặc zip phẳng = 1 chương)" className="rounded-lg border px-3 py-1.5 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-heading)" }}>
          {zip ? `${zip.label} ${zip.total ? `${zip.done}/${zip.total}` : ""}` : "⬆ Nhập bộ từ ZIP"}
        </button>
        <input ref={zipRef} type="file" accept=".zip,application/zip,application/x-zip-compressed" className="hidden" onChange={(e) => { const f = e.target.files?.[0]; if (f) void onZip(f); e.target.value = ""; }} />
      </div>
      {note && <p className="mb-3 rounded-lg px-3 py-2 text-xs" style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-muted)" }}>{note}</p>}

      <div className="space-y-2">
        {chapters.map((ch, i) => (
          <ChapterRow key={ch.id} id={id} chapter={ch} index={i} total={chapters.length} onMove={move} />
        ))}
        {chapters.length === 0 && <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>Chưa có chương — thêm bên dưới, hoặc nhập từ ZIP.</p>}
      </div>

      <form className="mt-4 flex gap-2" onSubmit={(e) => { e.preventDefault(); if (title.trim()) add.mutate(); }}>
        <input className="flex-1 rounded-lg border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)]" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} placeholder="Tên chương mới" value={title} onChange={(e) => setTitle(e.target.value)} />
        <button type="submit" disabled={add.isPending || !title.trim()} className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50" style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}>Thêm chương</button>
      </form>
    </div>
  );
}

function ChapterRow({ id, chapter, index, total, onMove }: { id: string; chapter: Chapter; index: number; total: number; onMove: (i: number, d: number) => void }) {
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(chapter.title);

  const invalidate = () => qc.invalidateQueries({ queryKey: ["comic", id] });
  const rename = useMutation({ mutationFn: () => updateChapter(chapter.id, { title: title.trim() }), onSuccess: () => { setEditing(false); invalidate(); } });
  const del = useMutation({ mutationFn: () => deleteChapter(chapter.id), onSuccess: invalidate });

  return (
    <div className="rounded-xl border" style={{ borderColor: "var(--tpl-border)" }}>
      <div className="flex items-center gap-2 px-3 py-2">
        <span className="w-6 shrink-0 text-center font-mono text-xs" style={{ color: "var(--tpl-muted)" }}>{index + 1}</span>
        {editing ? (
          <form className="flex flex-1 gap-2" onSubmit={(e) => { e.preventDefault(); if (title.trim()) rename.mutate(); }}>
            <input autoFocus value={title} onChange={(e) => setTitle(e.target.value)} className="flex-1 rounded-md border bg-transparent px-2 py-1 text-sm outline-none" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} />
            <button type="submit" className="text-xs font-semibold" style={{ color: "var(--tpl-accent)" }}>Lưu</button>
            <button type="button" onClick={() => { setEditing(false); setTitle(chapter.title); }} className="text-xs" style={{ color: "var(--tpl-muted)" }}>Huỷ</button>
          </form>
        ) : (
          <button type="button" onClick={() => setOpen((o) => !o)} className="flex-1 truncate text-left text-sm font-medium" style={{ color: "var(--tpl-heading)" }}>
            {chapter.title} <span style={{ color: "var(--tpl-muted)" }}>{open ? "▾" : "▸"}</span>
          </button>
        )}
        <IconLink label={`Đọc: ${chapter.title}`} href={`/library/comic/${id}/read/${chapter.id}` as Route}>▶</IconLink>
        <IconBtn label="Lên" disabled={index === 0} onClick={() => onMove(index, -1)}>↑</IconBtn>
        <IconBtn label="Xuống" disabled={index === total - 1} onClick={() => onMove(index, 1)}>↓</IconBtn>
        <IconBtn label="Sửa tên" onClick={() => { setTitle(chapter.title); setEditing(true); }}>✎</IconBtn>
        <DangerConfirm label="Xoá" confirmLabel="Xoá?" small onConfirm={() => del.mutate()} />
      </div>
      {open && <PageManager id={id} chapterId={chapter.id} />}
    </div>
  );
}

/* ── Page manager (upload / reorder / delete) ──────────────────────── */

function PageManager({ id, chapterId }: { id: string; chapterId: string }) {
  const qc = useQueryClient();
  const fileRef = useRef<HTMLInputElement>(null);
  const zipRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState<{ done: number; total: number } | null>(null);
  const [zip, setZip] = useState<{ label: string; done: number; total: number } | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const { data: pages } = useQuery({ queryKey: ["comic", id, "pages", chapterId], queryFn: () => getChapterPages(chapterId) });
  const list = pages ?? [];
  const invalidate = () => qc.invalidateQueries({ queryKey: ["comic", id, "pages", chapterId] });

  const del = useMutation({ mutationFn: (pageId: string) => deletePage(pageId), onSuccess: invalidate });
  const reorder = useMutation({ mutationFn: (order: string[]) => reorderPages(chapterId, order), onSuccess: invalidate });

  const move = (idx: number, delta: number) => {
    const ids = list.map((p) => p.page_id);
    const j = idx + delta;
    if (j < 0 || j >= ids.length) return;
    const tmp = ids[idx]!; ids[idx] = ids[j]!; ids[j] = tmp;
    reorder.mutate(ids);
  };

  async function onFiles(files: FileList) {
    setErr(null);
    const arr = Array.from(files);
    setUploading({ done: 0, total: arr.length });
    const created: { asset_id: string; sort_order: number }[] = [];
    try {
      for (let i = 0; i < arr.length; i += 1) {
        const { assetId } = await uploadImage(arr[i]!);
        created.push({ asset_id: assetId, sort_order: (list.length + i + 1) * 10 });
        setUploading({ done: i + 1, total: arr.length });
      }
      if (created.length) await createPages(chapterId, created);
      invalidate();
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Tải trang thất bại");
    } finally {
      setUploading(null);
    }
  }

  // Import a zip into THIS chapter (server-side, P1.7).
  async function onZipImport(file: File) {
    setErr(null);
    setZip({ label: "Đang tải zip…", done: 0, total: 0 });
    try {
      const job = await runZipImport(chapterId, file, (p: RunImportProgress) => {
        if (p.phase === "uploading") setZip({ label: `Đang tải ${p.uploadPct ?? 0}%`, done: 0, total: 0 });
        else setZip({ label: "Đang xử lý", done: p.job?.succeeded ?? 0, total: p.job?.total ?? 0 });
      });
      invalidate();
      if (job.status !== "done") setErr(`Nhập thất bại: ${job.error ?? "lỗi không rõ"}.`);
      else if (job.failed) setErr(`Nhập xong: ${job.succeeded} trang, ${job.failed} lỗi.`);
    } catch (e) {
      setErr(e instanceof Error ? e.message : "Nhập ZIP thất bại.");
    } finally {
      setZip(null);
    }
  }

  return (
    <div className="border-t px-3 py-3" style={{ borderColor: "var(--tpl-border)" }}>
      <div className="mb-3 flex items-center justify-between gap-2">
        <span className="text-xs" style={{ color: "var(--tpl-muted)" }}>{list.length} trang</span>
        <button type="button" onClick={() => zipRef.current?.click()} disabled={!!zip || !!uploading} className="rounded-lg border px-2.5 py-1 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-heading)" }}>
          {zip ? `${zip.label}${zip.total ? ` ${zip.done}/${zip.total}` : ""}` : "⬆ Nhập ZIP"}
        </button>
      </div>

      <div className="grid grid-cols-3 gap-2 sm:grid-cols-5 md:grid-cols-6">
        {list.map((p, i) => (
          <div key={p.page_id} className="group relative overflow-hidden rounded-lg border" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface-2)" }}>
            <div className="aspect-[3/4]">
              <img src={variantURL(p.asset_id, "thumb")} alt={`Trang ${i + 1}`} className="h-full w-full object-cover" onError={(e) => { (e.currentTarget as HTMLImageElement).style.visibility = "hidden"; }} />
            </div>
            <span className="absolute left-1 top-1 rounded bg-black/60 px-1.5 py-0.5 text-[10px] font-mono text-white">{i + 1}</span>
            <div className="absolute inset-x-0 bottom-0 flex items-center justify-between gap-1 bg-black/60 px-1 py-0.5 opacity-0 transition group-hover:opacity-100">
              <button type="button" aria-label="Lên" disabled={i === 0} onClick={() => move(i, -1)} className="text-xs text-white disabled:opacity-30">↑</button>
              <button type="button" aria-label="Xuống" disabled={i === list.length - 1} onClick={() => move(i, 1)} className="text-xs text-white disabled:opacity-30">↓</button>
              <button type="button" aria-label="Xoá trang" onClick={() => del.mutate(p.page_id)} className="text-xs text-red-400">✕</button>
            </div>
          </div>
        ))}

        {/* add-pages tile */}
        <button type="button" onClick={() => fileRef.current?.click()} disabled={!!uploading} className="flex aspect-[3/4] flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed text-center transition hover:border-[var(--tpl-accent)] disabled:opacity-60" style={{ borderColor: "var(--tpl-border)" }}>
          {uploading ? (
            <span className="px-1 text-[11px] font-medium" style={{ color: "var(--tpl-muted)" }}>Đang tải {uploading.done}/{uploading.total}…</span>
          ) : (
            <>
              <span className="text-xl" style={{ color: "var(--tpl-accent)" }}>+</span>
              <span className="px-1 text-[11px] font-medium" style={{ color: "var(--tpl-muted)" }}>Thêm trang</span>
            </>
          )}
        </button>
      </div>

      <input ref={fileRef} type="file" accept="image/*" multiple className="hidden" onChange={(e) => { if (e.target.files?.length) void onFiles(e.target.files); e.target.value = ""; }} />
      <input ref={zipRef} type="file" accept=".zip,application/zip,application/x-zip-compressed" className="hidden" onChange={(e) => { const f = e.target.files?.[0]; if (f) void onZipImport(f); e.target.value = ""; }} />
      {err && <p role="alert" className="mt-2 text-xs" style={{ color: "#ef4444" }}>{err}</p>}
    </div>
  );
}

/* ── shared pieces ─────────────────────────────────────────────────── */

function Cover({ assetId, title }: { assetId: string | null; title: string }) {
  return (
    <div className="aspect-[3/4] w-full overflow-hidden rounded-xl border" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface-2)" }}>
      {assetId ? (
        <img src={variantURL(assetId, "medium")} alt={title} className="h-full w-full object-cover" />
      ) : (
        <div className="flex h-full w-full items-center justify-center text-xs" style={{ color: "var(--tpl-muted)" }}>Chưa có bìa</div>
      )}
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>{label}</span>
      {children}
    </label>
  );
}

function Segmented({ value, options, onChange }: { value: string; options: { v: string; label: string }[]; onChange: (v: string) => void }) {
  return (
    <div className="inline-flex flex-wrap gap-1 rounded-lg p-1" style={{ background: "var(--tpl-surface-2)" }}>
      {options.map((o) => (
        <button key={o.v} type="button" onClick={() => onChange(o.v)} aria-pressed={value === o.v} className="rounded-md px-3 py-1.5 text-xs font-semibold transition" style={value === o.v ? { background: "var(--tpl-accent)", color: "#fff" } : { color: "var(--tpl-muted)" }}>
          {o.label}
        </button>
      ))}
    </div>
  );
}

function IconBtn({ children, label, onClick, disabled }: { children: React.ReactNode; label: string; onClick: () => void; disabled?: boolean }) {
  return (
    <button type="button" aria-label={label} title={label} onClick={onClick} disabled={disabled} className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-sm transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-30" style={{ color: "var(--tpl-muted)" }}>
      {children}
    </button>
  );
}

// Navigation twin of IconBtn. A real <Link>, not a button with router.push, so
// middle-click and "open in new tab" work and it is announced as a link.
function IconLink({ children, label, href }: { children: React.ReactNode; label: string; href: Route }) {
  return (
    <Link href={href} aria-label={label} title={label} className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-sm transition hover:bg-[var(--tpl-surface-2)]" style={{ color: "var(--tpl-muted)" }}>
      {children}
    </Link>
  );
}

function DangerConfirm({ label, confirmLabel, onConfirm, small }: { label: string; confirmLabel: string; onConfirm: () => void; small?: boolean }) {
  const [armed, setArmed] = useState(false);
  const t = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => () => { if (t.current) clearTimeout(t.current); }, []);
  const cls = small ? "px-2 py-1 text-xs" : "px-3 py-2 text-sm";
  return (
    <button
      type="button"
      onClick={() => {
        if (armed) { setArmed(false); onConfirm(); return; }
        setArmed(true);
        if (t.current) clearTimeout(t.current);
        t.current = setTimeout(() => setArmed(false), 3000);
      }}
      className={`rounded-lg font-semibold transition ${cls}`}
      style={armed ? { background: "#ef4444", color: "#fff" } : { color: "#ef4444", border: "1px solid rgba(239,68,68,.4)" }}
    >
      {armed ? confirmLabel : label}
    </button>
  );
}
