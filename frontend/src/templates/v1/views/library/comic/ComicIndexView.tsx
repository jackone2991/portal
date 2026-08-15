"use client";

import { useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type Comic, createComic, listComics, listMyComics, variantURL } from "@/lib/comic";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";

// Library · Comics (SPEC-02 P0.5, redesigned). A clean title + compact tab toggle
// (the old full-width toolbar strip is gone), and the "new comic" action is a
// create tile in the grid itself, opening a small dialog.
export function ComicIndexView() {
  const qc = useQueryClient();
  const router = useRouter();
  const [tab, setTab] = useState<"all" | "mine">("all");
  const [modalOpen, setModalOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const all = useQuery({ queryKey: ["comics", "published"], queryFn: () => listComics() });
  const mine = useQuery({ queryKey: ["comics", "mine"], queryFn: () => listMyComics(), enabled: tab === "mine" });

  const create = useMutation({
    mutationFn: () => createComic({ title: title.trim() }),
    onSuccess: (c) => {
      setTitle("");
      setModalOpen(false);
      setErr(null);
      qc.invalidateQueries({ queryKey: ["comics"] });
      router.push(`/library/comic/${c.id}` as Route);
    },
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không tạo được truyện"),
  });

  const active = tab === "all" ? all : mine;
  const comics = tab === "all" ? all.data?.comics ?? [] : mine.data?.comics ?? [];
  const loading = active.isLoading;

  return (
    <section style={{ color: "var(--tpl-text)" }}>
      {/* Header — title + compact toggle (no full-width toolbar) */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold tracking-tight" style={{ color: "var(--tpl-heading)" }}>Truyện tranh</h1>
        <p className="mt-1 text-sm" style={{ color: "var(--tpl-muted)" }}>Đọc và quản lý truyện tranh của bạn.</p>
        <div className="mt-4 inline-flex gap-1 rounded-xl p-1" style={{ background: "var(--tpl-surface-2)" }}>
          <TabBtn active={tab === "all"} onClick={() => setTab("all")}>Thư viện</TabBtn>
          <TabBtn active={tab === "mine"} onClick={() => setTab("mine")}>Của tôi</TabBtn>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {/* Create tile — the "new comic" action, first in the grid */}
        <button type="button" onClick={() => { setErr(null); setModalOpen(true); }} className="group block text-left">
          <div
            className="flex aspect-[3/4] flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed transition group-hover:border-[var(--tpl-accent)]"
            style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
          >
            <span className="grid h-11 w-11 place-items-center rounded-full text-2xl leading-none transition group-hover:scale-105" style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-accent)" }}>+</span>
            <span className="text-sm font-semibold transition group-hover:text-[var(--tpl-accent)]" style={{ color: "var(--tpl-muted)" }}>Thêm truyện</span>
          </div>
          <div className="mt-1.5 truncate text-sm font-medium" style={{ color: "var(--tpl-heading)" }}>Truyện mới</div>
          <div className="text-xs" style={{ color: "var(--tpl-muted)" }}>Tạo bản nháp</div>
        </button>

        {loading
          ? Array.from({ length: 4 }, (_, i) => <SkeletonCard key={`s${i}`} />)
          : comics.map((c) => <ComicCard key={c.id} comic={c} showStatus={tab === "mine"} />)}
      </div>

      {!loading && comics.length === 0 && (
        <p className="mt-4 text-sm" style={{ color: "var(--tpl-muted)" }}>
          {tab === "all" ? "Chưa có truyện nào được xuất bản." : "Bạn chưa có truyện nào — bấm “Thêm truyện” để bắt đầu."}
        </p>
      )}

      {modalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setModalOpen(false)} role="dialog" aria-modal="true" aria-label="Tạo truyện mới">
          <form
            onClick={(e) => e.stopPropagation()}
            onSubmit={(e) => { e.preventDefault(); if (title.trim()) create.mutate(); }}
            className="w-full max-w-md rounded-2xl p-5 shadow-xl"
            style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
          >
            <h2 className="text-base font-bold" style={{ color: "var(--tpl-heading)" }}>Tạo truyện mới</h2>
            <p className="mt-1 text-xs" style={{ color: "var(--tpl-muted)" }}>Đặt tên truyện. Bạn sẽ thêm chương và trang ở bước sau.</p>
            <input
              autoFocus
              className="mt-4 w-full rounded-lg border bg-transparent px-3 py-2.5 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
              placeholder="Tên truyện"
              value={title}
              maxLength={200}
              onChange={(e) => setTitle(e.target.value)}
            />
            {err && <p role="alert" className="mt-2 rounded-lg px-3 py-2 text-sm" style={{ background: "rgba(239,68,68,.08)", color: "#ef4444" }}>{err}</p>}
            <div className="mt-4 flex justify-end gap-2">
              <button type="button" onClick={() => setModalOpen(false)} className="rounded-lg border px-4 py-2 text-sm font-medium transition hover:bg-[var(--tpl-surface-2)]" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}>Huỷ</button>
              <button type="submit" disabled={create.isPending || !title.trim()} className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50" style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}>
                {create.isPending ? "Đang tạo…" : "Tạo truyện"}
              </button>
            </div>
          </form>
        </div>
      )}
    </section>
  );
}

function TabBtn({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className="rounded-lg px-4 py-1.5 text-sm font-semibold transition"
      style={active ? { background: "var(--tpl-surface)", color: "var(--tpl-heading)", boxShadow: "0 1px 2px rgba(0,0,0,.08)" } : { color: "var(--tpl-muted)" }}
    >
      {children}
    </button>
  );
}

function ComicCard({ comic, showStatus }: { comic: Comic; showStatus: boolean }) {
  return (
    <Link href={`/library/comic/${comic.id}` as Route} className="group block">
      <div className="relative aspect-[3/4] overflow-hidden rounded-xl border" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface-2)" }}>
        {comic.cover_asset_id ? (
          <img src={variantURL(comic.cover_asset_id, "thumb")} alt={comic.title} className="h-full w-full object-cover transition group-hover:scale-[1.02]" />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-xs" style={{ color: "var(--tpl-muted)" }}>Chưa có bìa</div>
        )}
        {showStatus && (
          <span className="absolute left-2 top-2 rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase text-white" style={{ background: comic.status === "published" ? "var(--tpl-accent)" : "rgba(0,0,0,.55)" }}>
            {comic.status === "published" ? "đã đăng" : "nháp"}
          </span>
        )}
      </div>
      <div className="mt-1.5 truncate text-sm font-medium transition group-hover:text-[var(--tpl-accent)]" style={{ color: "var(--tpl-heading)" }}>{comic.title}</div>
      <div className="text-xs" style={{ color: "var(--tpl-muted)" }}>{comic.chapter_count ?? 0} chương</div>
    </Link>
  );
}

function SkeletonCard() {
  return (
    <div>
      <div className="aspect-[3/4] animate-pulse rounded-xl" style={{ background: "var(--tpl-surface-2)" }} />
      <div className="mt-1.5 h-3 w-2/3 animate-pulse rounded" style={{ background: "var(--tpl-surface-2)" }} />
    </div>
  );
}
