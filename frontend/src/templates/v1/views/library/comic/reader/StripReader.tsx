"use client";

// Webtoon vertical-scroll renderer (SPEC-02 P0.3 + R3 seamless continuity). Loads
// the current chapter, then auto-appends the NEXT chapter inline as the reader nears
// the bottom — continuous reading with no dead-end. Tracks which chapter/page is at
// the viewport middle and reports it (for progress, the chapter label, and the
// slider). Exposes `seek` so the bottom slider can jump within the active chapter.
// A click toggles the reader chrome (immersive).

import { forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState, type RefObject } from "react";
import Link from "next/link";
import type { Route } from "next";
import { getChapterPages, variantURL, type Chapter, type ReaderPage } from "@/lib/comic";

export interface StripHandle {
  /** Scroll to a page (0-based) within a loaded chapter. */
  seek: (chapterId: string, index: number) => void;
}

type Segment = { chapterId: string; pages: ReaderPage[] };
export interface ActivePos { chapterId: string; pageId: string; index: number; count: number }

export const StripReader = forwardRef<StripHandle, {
  comicId: string;
  chapters: Chapter[];
  startChapterId: string;
  startPages: ReaderPage[];
  variant: "thumb" | "medium" | "poster";
  initialPageId: string | null;
  scrollRef: RefObject<HTMLElement | null>; // the full-screen scroll container (R5)
  onActive: (pos: ActivePos) => void;
  onToggleChrome: () => void;
}>(function StripReader({ comicId, chapters, startChapterId, startPages, variant, initialPageId, scrollRef, onActive, onToggleChrome }, ref) {
  const [segments, setSegments] = useState<Segment[]>([{ chapterId: startChapterId, pages: startPages }]);
  const [failed, setFailed] = useState<Set<string>>(() => new Set());
  const els = useRef<Record<string, HTMLElement | null>>({});
  const loadingNext = useRef(false);
  const lastActive = useRef<string | null>(null);
  const [didResume, setDidResume] = useState(false);

  // pageId → {chapterId, index-within-chapter, chapter length}
  const meta = useMemo(() => {
    const m = new Map<string, { chapterId: string; index: number; count: number }>();
    for (const seg of segments) seg.pages.forEach((p, i) => m.set(p.page_id, { chapterId: seg.chapterId, index: i, count: seg.pages.length }));
    return m;
  }, [segments]);
  const flat = useMemo(() => segments.flatMap((s) => s.pages), [segments]);

  useImperativeHandle(ref, () => ({
    seek: (chapterId, index) => {
      const seg = segments.find((s) => s.chapterId === chapterId);
      const p = seg?.pages[index];
      if (p && els.current[p.page_id]) els.current[p.page_id]!.scrollIntoView({ block: "start" });
    },
  }), [segments]);

  // Resume: scroll to the saved page (start chapter) once mounted.
  useEffect(() => {
    if (didResume || flat.length === 0) return;
    if (initialPageId && els.current[initialPageId]) els.current[initialPageId]!.scrollIntoView();
    setDidResume(true);
  }, [initialPageId, didResume, flat]);

  const loadNext = useCallback(async () => {
    if (loadingNext.current) return;
    const lastCh = segments[segments.length - 1]?.chapterId;
    const idx = chapters.findIndex((c) => c.id === lastCh);
    const next = idx >= 0 ? chapters[idx + 1] : undefined;
    if (!next) return;
    loadingNext.current = true;
    try {
      const pages = await getChapterPages(next.id);
      setSegments((segs) => (segs.some((s) => s.chapterId === next.id) ? segs : [...segs, { chapterId: next.id, pages }]));
    } catch {
      // leave the "end" panel; a click still navigates
    } finally {
      loadingNext.current = false;
    }
  }, [segments, chapters]);

  const onScroll = useCallback(() => {
    const sc = scrollRef.current;
    if (!sc) return;
    const mid = sc.scrollTop + sc.clientHeight / 2;
    let cur: string | null = null;
    for (const p of flat) {
      const el = els.current[p.page_id];
      if (el && el.offsetTop <= mid) cur = p.page_id;
    }
    if (cur && cur !== lastActive.current) {
      lastActive.current = cur;
      const info = meta.get(cur);
      if (info) onActive({ chapterId: info.chapterId, pageId: cur, index: info.index, count: info.count });
    }
    if (sc.scrollTop + sc.clientHeight >= sc.scrollHeight - 1200) loadNext();
  }, [flat, meta, onActive, loadNext, scrollRef]);

  useEffect(() => {
    const sc = scrollRef.current;
    if (!sc) return;
    sc.addEventListener("scroll", onScroll, { passive: true });
    onScroll(); // seed active position
    return () => sc.removeEventListener("scroll", onScroll);
  }, [onScroll, scrollRef]);

  const noMoreChapters = (() => {
    const lastCh = segments[segments.length - 1]?.chapterId;
    const idx = chapters.findIndex((c) => c.id === lastCh);
    return idx < 0 || idx >= chapters.length - 1;
  })();

  return (
    <div className="mx-auto max-w-3xl" onClick={onToggleChrome}>
      {segments.map((seg) => (
        <div key={seg.chapterId}>
          {seg.chapterId !== startChapterId && (
            <div className="py-6 text-center text-xs uppercase tracking-widest text-gray-600">
              {chapters.find((c) => c.id === seg.chapterId)?.title ?? "Chương tiếp theo"}
            </div>
          )}
          {seg.pages.map((p) => (
            <div key={p.page_id} ref={(el) => { els.current[p.page_id] = el; }} className="border-b border-gray-900">
              {failed.has(p.page_id) ? (
                <div className="flex h-40 items-center justify-center text-gray-600">⚠ Trang không tải được</div>
              ) : (
                <img
                  src={variantURL(p.asset_id, variant)}
                  width={p.width ?? undefined}
                  height={p.height ?? undefined}
                  alt=""
                  loading="lazy"
                  className="w-full"
                  onError={() => setFailed((s) => { const n = new Set(s); n.add(p.page_id); return n; })}
                />
              )}
            </div>
          ))}
        </div>
      ))}

      {flat.length === 0 && <div className="p-8 text-center text-gray-500">This chapter has no pages yet.</div>}

      <div className="flex flex-col items-center gap-3 py-10">
        {noMoreChapters ? (
          <p className="text-gray-500">Hết truyện.</p>
        ) : (
          <p className="text-sm text-gray-600">Đang tải chương sau…</p>
        )}
        <Link href={`/library/comic/${comicId}` as Route} className="text-sm text-gray-400 hover:underline">Về trang truyện</Link>
      </div>
    </div>
  );
});
