"use client";

// Comic reader container (SPEC-02 P0.3 + reader redesign R1–R4). Owns data, chrome
// visibility, keyboard, progress + preloading, and dispatches to the mode renderer
// (StripReader = seamless webtoon, PagedReader = single/double, LTR/RTL). Reader
// prefs come from the persisted Zustand store (D-32); direction resolves from the
// override or the comic's reading_direction (R3). Resume + progress are P0.4.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Route } from "next";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { getChapterPages, getComic } from "@/lib/comic";
import { effectiveDirection, qualityVariant, useReaderSettings } from "@/lib/reader-settings";
import { useReaderProgress } from "./reader/useReaderProgress";
import { usePagePreloader } from "./reader/usePagePreloader";
import { StripReader, type ActivePos, type StripHandle } from "./reader/StripReader";
import { PagedReader } from "./reader/PagedReader";
import { ReaderChrome } from "./reader/ReaderChrome";
import { ReaderSettings } from "./reader/ReaderSettings";
import { ChapterMenu } from "./reader/ChapterMenu";
import { ReaderHelp } from "./reader/ReaderHelp";

const CHROME_HIDE_MS = 2500;

export function ComicReaderView({ id, chapterId }: { id: string; chapterId: string }) {
  const router = useRouter();
  const { data: comic } = useQuery({ queryKey: ["comic", id], queryFn: () => getComic(id) });
  const { data: pages, isLoading } = useQuery({ queryKey: ["comic", id, "pages", chapterId], queryFn: () => getChapterPages(chapterId) });

  const { mode, fit, brightness, quality, direction } = useReaderSettings();
  const variant = qualityVariant(quality);
  const dir = effectiveDirection(direction, comic?.reading_direction);
  const { report } = useReaderProgress(id);

  const [mounted, setMounted] = useState(false); // gate on mount → no SSR/persist hydration mismatch
  const [chromeVisible, setChromeVisible] = useState(true);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [chaptersOpen, setChaptersOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  const [pagedIndex, setPagedIndex] = useState(0);
  const [didResume, setDidResume] = useState(false);
  const [active, setActive] = useState<ActivePos | null>(null); // webtoon active pos
  const stripRef = useRef<StripHandle>(null);
  const scrollRef = useRef<HTMLDivElement>(null); // full-screen scroll container (R5)
  useEffect(() => setMounted(true), []);

  // Full-screen: lock the page behind the reader overlay (R5).
  useEffect(() => {
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { document.body.style.overflow = prev; };
  }, []);

  // Reset per-chapter state when the route chapter changes.
  useEffect(() => { setPagedIndex(0); setDidResume(false); setActive(null); }, [chapterId]);

  const chapters = useMemo(() => comic?.chapters ?? [], [comic]);
  const isWebtoon = mode === "webtoon";
  const curChapterId = isWebtoon ? active?.chapterId ?? chapterId : chapterId;
  const chIdx = chapters.findIndex((c) => c.id === curChapterId);
  const prevHref: Route | null = chIdx > 0 ? (`/library/comic/${id}/read/${chapters[chIdx - 1]!.id}` as Route) : null;
  const nextHref: Route | null = chIdx >= 0 && chIdx < chapters.length - 1 ? (`/library/comic/${id}/read/${chapters[chIdx + 1]!.id}` as Route) : null;
  const nextChapterId = chIdx >= 0 && chIdx < chapters.length - 1 ? chapters[chIdx + 1]!.id : null;

  const initialPageId = comic?.progress && comic.progress.chapter_id === chapterId ? comic.progress.page_id : null;

  // Preload upcoming page images + next chapter list (paged modes).
  usePagePreloader({ comicId: id, pages: pages ?? [], index: pagedIndex, variant, nextChapterId: mode === "webtoon" ? null : nextChapterId });

  const toggleChrome = useCallback(() => setChromeVisible((v) => !v), []);

  // Paged navigation in STORY order (delta +1 = forward). Steps by 2 in double.
  const goPaged = useCallback((delta: number) => {
    if (!pages || pages.length === 0) return;
    const step = mode === "double" ? 2 : 1;
    let ni = pagedIndex + delta * step;
    if (mode === "double") ni -= ni % 2;
    if (ni < 0) { if (prevHref) router.push(prevHref); return; }
    if (ni >= pages.length) { if (nextHref) router.push(nextHref); return; }
    setPagedIndex(ni);
    setChromeVisible(true);
  }, [pages, pagedIndex, mode, prevHref, nextHref, router]);

  // Paged resume: land on the saved page once pages load.
  useEffect(() => {
    if (didResume || !pages || isWebtoon) return;
    if (initialPageId) {
      const i = pages.findIndex((p) => p.page_id === initialPageId);
      if (i >= 0) setPagedIndex(mode === "double" ? i - (i % 2) : i);
    }
    setDidResume(true);
  }, [pages, isWebtoon, initialPageId, didResume, mode]);

  // Paged progress: report the visible page.
  useEffect(() => {
    if (!isWebtoon && pages && pages[pagedIndex]) report(chapterId, pages[pagedIndex]!.page_id);
  }, [isWebtoon, pages, pagedIndex, chapterId, report]);

  // Webtoon active position → progress + chapter label + slider.
  const onWebtoonActive = useCallback((pos: ActivePos) => {
    setActive(pos);
    report(pos.chapterId, pos.pageId);
  }, [report]);

  // Keyboard: nav, chrome, settings, chapters, chapter jump. (Zoom keys live in PagedReader.)
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (settingsOpen || chaptersOpen || helpOpen) { if (e.key === "Escape") { setSettingsOpen(false); setChaptersOpen(false); setHelpOpen(false); } return; }
      switch (e.key) {
        case "f": case "F": e.preventDefault(); toggleChrome(); return;
        case "s": case "S": e.preventDefault(); setSettingsOpen(true); return;
        case "c": case "C": e.preventDefault(); setChaptersOpen(true); return;
        case "?": e.preventDefault(); setHelpOpen(true); return;
        case "[": if (prevHref) { e.preventDefault(); router.push(prevHref); } return;
        case "]": if (nextHref) { e.preventDefault(); router.push(nextHref); } return;
      }
      if (isWebtoon) return; // webtoon uses native scroll
      if (e.key === "ArrowRight") { e.preventDefault(); goPaged(dir === "rtl" ? -1 : 1); }
      else if (e.key === "ArrowLeft") { e.preventDefault(); goPaged(dir === "rtl" ? 1 : -1); }
      else if (e.key === " ") { e.preventDefault(); goPaged(1); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [settingsOpen, chaptersOpen, helpOpen, isWebtoon, dir, goPaged, toggleChrome, prevHref, nextHref, router]);

  // Auto-hide chrome after inactivity in paged mode.
  useEffect(() => {
    if (isWebtoon || !chromeVisible || settingsOpen || chaptersOpen || helpOpen) return;
    const t = setTimeout(() => setChromeVisible(false), CHROME_HIDE_MS);
    return () => clearTimeout(t);
  }, [isWebtoon, chromeVisible, settingsOpen, chaptersOpen, helpOpen, pagedIndex]);

  if (!mounted || isLoading) {
    return <div className="fixed inset-0 z-50 flex items-center justify-center bg-black text-gray-400">Đang tải…</div>;
  }

  const title = comic?.title ?? "Comic";
  const chapterLabel = chIdx >= 0 ? `Chương ${chIdx + 1}/${chapters.length}` : "";
  const hasPages = !!pages && pages.length > 0;

  const slider = isWebtoon
    ? active ? { current: active.index + 1, total: active.count, onSeek: (i: number) => stripRef.current?.seek(active.chapterId, i) } : undefined
    : hasPages ? { current: pagedIndex + 1, total: pages!.length, onSeek: (i: number) => setPagedIndex(mode === "double" ? i - (i % 2) : i) } : undefined;

  return (
    <div ref={scrollRef} className={`fixed inset-0 z-50 bg-black text-white overscroll-contain ${isWebtoon ? "overflow-y-auto" : "overflow-hidden"}`}>
      <ReaderChrome
        visible={chromeVisible}
        comicId={id}
        title={title}
        chapterLabel={chapterLabel}
        dir={dir}
        prevHref={prevHref}
        nextHref={nextHref}
        onOpenSettings={() => setSettingsOpen(true)}
        onOpenChapters={() => setChaptersOpen(true)}
        onOpenHelp={() => setHelpOpen(true)}
        slider={slider}
      />

      {isWebtoon ? (
        <div className="pt-12">
          {hasPages ? (
            <StripReader
              ref={stripRef}
              comicId={id}
              chapters={chapters}
              startChapterId={chapterId}
              startPages={pages!}
              variant={variant}
              initialPageId={initialPageId}
              scrollRef={scrollRef}
              onActive={onWebtoonActive}
              onToggleChrome={toggleChrome}
            />
          ) : (
            <div className="flex min-h-screen items-center justify-center text-gray-500">Chương này chưa có trang.</div>
          )}
        </div>
      ) : hasPages ? (
        <PagedReader
          pages={pages!}
          index={pagedIndex}
          mode={mode === "double" ? "double" : "single"}
          dir={dir}
          variant={variant}
          fit={fit}
          onPrev={() => goPaged(-1)}
          onNext={() => goPaged(1)}
          onToggleChrome={toggleChrome}
        />
      ) : (
        <div className="flex min-h-screen items-center justify-center text-gray-500">Chương này chưa có trang.</div>
      )}

      {brightness < 100 && (
        <div className="pointer-events-none fixed inset-0 z-10 bg-black" style={{ opacity: (100 - brightness) / 100 }} aria-hidden="true" />
      )}

      <ReaderSettings open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      <ChapterMenu open={chaptersOpen} onClose={() => setChaptersOpen(false)} comicId={id} chapters={chapters} activeChapterId={curChapterId} />
      <ReaderHelp open={helpOpen} onClose={() => setHelpOpen(false)} />
    </div>
  );
}
