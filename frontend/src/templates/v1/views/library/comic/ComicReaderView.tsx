"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { getChapterPages, getComic, saveComicProgress, variantURL } from "@/lib/comic";

const THROTTLE_MS = 10_000;

export function ComicReaderView({ id, chapterId }: { id: string; chapterId: string }) {
  const { data: comic } = useQuery({ queryKey: ["comic", id], queryFn: () => getComic(id) });
  const { data: pages, isLoading } = useQuery({ queryKey: ["comic", id, "pages", chapterId], queryFn: () => getChapterPages(chapterId) });

  const pageRefs = useRef<Record<string, HTMLElement | null>>({});
  const furthest = useRef<string | null>(null);
  const lastSync = useRef<number>(0);
  const [didResume, setDidResume] = useState(false);

  // Resume: scroll to the saved page for this chapter (P0.4).
  useEffect(() => {
    if (didResume || !pages || !comic?.progress) return;
    if (comic.progress.chapter_id === chapterId && comic.progress.page_id) {
      const el = pageRefs.current[comic.progress.page_id];
      if (el) el.scrollIntoView();
    }
    setDidResume(true);
  }, [pages, comic, chapterId, didResume]);

  const save = useCallback(() => {
    if (furthest.current) saveComicProgress(id, chapterId, furthest.current);
  }, [id, chapterId]);

  // Track the furthest page scrolled past; debounce the progress write.
  const onScroll = useCallback(() => {
    if (!pages) return;
    const mid = window.scrollY + window.innerHeight / 2;
    for (const p of pages) {
      const el = pageRefs.current[p.page_id];
      if (el && el.offsetTop <= mid) furthest.current = p.page_id;
    }
    const now = Date.now();
    if (now - lastSync.current > THROTTLE_MS) {
      lastSync.current = now;
      save();
    }
  }, [pages, save]);

  useEffect(() => {
    window.addEventListener("scroll", onScroll, { passive: true });
    const onHide = () => save();
    window.addEventListener("pagehide", onHide);
    document.addEventListener("visibilitychange", () => { if (document.visibilityState === "hidden") save(); });
    return () => {
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("pagehide", onHide);
    };
  }, [onScroll, save]);

  // chapter navigation
  const chapters = comic?.chapters ?? [];
  const idx = chapters.findIndex((c) => c.id === chapterId);
  const next = idx >= 0 ? chapters[idx + 1] : undefined;

  return (
    <div className="min-h-screen bg-black text-white">
      <header className="sticky top-0 z-10 flex items-center gap-3 bg-black/80 px-4 py-2 backdrop-blur">
        <Link href={`/library/comic/${id}` as Route} className="text-sm text-blue-400 hover:underline">← {comic?.title ?? "Comic"}</Link>
        {idx >= 0 && <span className="text-sm text-gray-400">Ch. {idx + 1}</span>}
      </header>

      {isLoading ? (
        <div className="p-8 text-center text-gray-400">Loading…</div>
      ) : (
        <div className="mx-auto max-w-3xl">
          {(pages ?? []).map((p) => (
            <div key={p.page_id} ref={(el) => { pageRefs.current[p.page_id] = el; }} className="border-b border-gray-900">
              <img
                src={variantURL(p.asset_id, "medium")}
                width={p.width ?? undefined}
                height={p.height ?? undefined}
                alt=""
                loading="lazy"
                className="w-full"
                onError={(e) => { (e.currentTarget as HTMLImageElement).replaceWith(document.createTextNode("⚠ page unavailable")); }}
              />
            </div>
          ))}
          {(pages ?? []).length === 0 && <div className="p-8 text-center text-gray-500">This chapter has no pages yet.</div>}

          <div className="flex flex-col items-center gap-3 py-10">
            {next ? (
              <Link href={`/library/comic/${id}/read/${next.id}` as Route} className="rounded-md bg-blue-600 px-6 py-2 font-medium hover:bg-blue-500">Next chapter →</Link>
            ) : (
              <p className="text-gray-500">End of the comic.</p>
            )}
            <Link href={`/library/comic/${id}` as Route} className="text-sm text-gray-400 hover:underline">Back to comic</Link>
          </div>
        </div>
      )}
    </div>
  );
}
