"use client";

// Page preloader (SPEC-02 reader redesign, R3). Decodes the next few page images
// ahead of the reader so flips/scrolls never stall, and prefetches the NEXT
// chapter's page list (TanStack) as the reader nears the end — so "next chapter"
// (or seamless continuation) is instant. Bounded: never the whole chapter up front.

import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { getChapterPages, variantURL, type ReaderPage } from "@/lib/comic";

export function usePagePreloader({
  comicId,
  pages,
  index,
  variant,
  ahead = 4,
  nextChapterId,
}: {
  comicId: string;
  pages: ReaderPage[];
  index: number;
  variant: "thumb" | "medium" | "poster";
  ahead?: number;
  nextChapterId: string | null;
}) {
  // Decode the next `ahead` page images.
  useEffect(() => {
    if (typeof window === "undefined") return;
    for (let i = index + 1; i <= index + ahead && i < pages.length; i += 1) {
      const p = pages[i];
      if (p) {
        const img = new window.Image();
        img.decoding = "async";
        img.src = variantURL(p.asset_id, variant);
      }
    }
  }, [pages, index, variant, ahead]);

  // Prefetch the next chapter's page list when within `ahead` of the end.
  const qc = useQueryClient();
  useEffect(() => {
    if (!nextChapterId) return;
    if (index >= pages.length - ahead) {
      qc.prefetchQuery({
        queryKey: ["comic", comicId, "pages", nextChapterId],
        queryFn: () => getChapterPages(nextChapterId),
        staleTime: 5 * 60 * 1000,
      });
    }
  }, [nextChapterId, index, pages.length, ahead, qc, comicId]);
}
