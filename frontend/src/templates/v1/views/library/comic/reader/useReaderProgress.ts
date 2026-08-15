"use client";

// Reading-progress upsert (SPEC-02 P0.4), extracted from ComicReaderView. Callers
// invoke `report(chapterId, pageId)` whenever the current / furthest-visible page
// changes — chapterId is dynamic so seamless webtoon (R3) can report progress for
// whichever chapter the reader has scrolled into. Writes are debounced (10 s) and
// flushed on tab-hide / pagehide.

import { useCallback, useEffect, useRef } from "react";
import { saveComicProgress } from "@/lib/comic";

const THROTTLE_MS = 10_000;

export function useReaderProgress(comicId: string) {
  const posRef = useRef<{ chapterId: string; pageId: string } | null>(null);
  const lastSync = useRef(0);

  const flush = useCallback(() => {
    if (posRef.current) saveComicProgress(comicId, posRef.current.chapterId, posRef.current.pageId);
  }, [comicId]);

  const report = useCallback(
    (chapterId: string, pageId: string) => {
      posRef.current = { chapterId, pageId };
      const now = Date.now();
      if (now - lastSync.current > THROTTLE_MS) {
        lastSync.current = now;
        flush();
      }
    },
    [flush],
  );

  useEffect(() => {
    const onHide = () => flush();
    const onVis = () => {
      if (document.visibilityState === "hidden") flush();
    };
    window.addEventListener("pagehide", onHide);
    document.addEventListener("visibilitychange", onVis);
    return () => {
      window.removeEventListener("pagehide", onHide);
      document.removeEventListener("visibilitychange", onVis);
    };
  }, [flush]);

  return { report };
}
