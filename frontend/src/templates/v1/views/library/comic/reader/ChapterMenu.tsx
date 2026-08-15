"use client";

// In-reader chapter list (SPEC-02 reader redesign, R2). A bottom sheet; the active
// chapter is highlighted; picking one navigates to its reader route.

import Link from "next/link";
import type { Route } from "next";
import type { Chapter } from "@/lib/comic";

export function ChapterMenu({
  open,
  onClose,
  comicId,
  chapters,
  activeChapterId,
}: {
  open: boolean;
  onClose: () => void;
  comicId: string;
  chapters: Chapter[];
  activeChapterId: string;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-30 flex items-end justify-center bg-black/60" onClick={onClose} role="dialog" aria-modal="true" aria-label="Danh sách chương">
      <div className="max-h-[70vh] w-full max-w-md overflow-y-auto rounded-t-2xl border-t border-white/10 bg-neutral-900 p-3 text-white" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between px-2 py-2">
          <h2 className="text-xs font-bold uppercase tracking-wider text-gray-400">Chương ({chapters.length})</h2>
          <button type="button" onClick={onClose} className="rounded-md px-2 py-1 text-sm text-gray-300 hover:bg-white/10">Đóng</button>
        </div>
        <ul>
          {chapters.map((ch, i) => {
            const active = ch.id === activeChapterId;
            return (
              <li key={ch.id}>
                <Link
                  href={`/library/comic/${comicId}/read/${ch.id}` as Route}
                  onClick={onClose}
                  aria-current={active ? "true" : undefined}
                  className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm ${active ? "bg-blue-600/20 text-blue-300" : "text-gray-200 hover:bg-white/5"}`}
                >
                  <span className="w-6 shrink-0 text-right font-mono text-xs text-gray-500">{i + 1}</span>
                  <span className="truncate">{ch.title}</span>
                  {active && <span className="ml-auto text-xs">đang đọc</span>}
                </Link>
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}
