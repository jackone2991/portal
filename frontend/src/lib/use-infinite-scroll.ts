"use client";

import { useCallback, useEffect, useRef } from "react";

/**
 * Load the next page when a sentinel element scrolls into view.
 *
 * Pairs with TanStack's `useInfiniteQuery`: put the returned ref on an empty
 * element after the last row, and pass `hasNextPage` / `isFetchingNextPage` /
 * `fetchNextPage` straight through.
 *
 * Two things here are not obvious and are the reason this is a shared hook
 * rather than five lines inlined per view:
 *
 * 1. **The observer is deliberately rebuilt after every page.** An
 *    `IntersectionObserver` reports *changes* in intersection, not a continuous
 *    state — so if the loaded rows still do not fill the viewport, the sentinel
 *    stays visible, nothing changes, and no further callback ever fires. The
 *    list would stop one page in, on exactly the tall screens where it is most
 *    obvious. Re-observing on each `isLoading`/`hasMore` transition re-fires the
 *    initial intersection, so a short list keeps pulling pages until it is
 *    finally taller than the window.
 *
 * 2. **`rootMargin` fetches ahead of the fold.** Waiting for the sentinel to be
 *    literally on screen means the user always sees the list end before it
 *    grows. A margin starts the request while the bottom is still a screenful
 *    away, so scrolling stays continuous.
 *
 * The callback is held in a ref so an unstable `onLoadMore` (an inline arrow in
 * the caller) cannot tear down and rebuild the observer on every render.
 */
export function useInfiniteScroll<T extends HTMLElement = HTMLDivElement>({
  onLoadMore,
  hasMore,
  isLoading,
  rootMargin = "400px",
}: {
  onLoadMore: () => void;
  /** `hasNextPage` — when false the observer is not attached at all. */
  hasMore: boolean;
  /** `isFetchingNextPage` — guards against stacking requests for one page. */
  isLoading: boolean;
  /** How far ahead of the fold to start loading. */
  rootMargin?: string;
}) {
  const sentinelRef = useRef<T | null>(null);

  const cb = useRef(onLoadMore);
  cb.current = onLoadMore;

  const fire = useCallback(() => {
    // Re-read the guards at fire time: the observer may outlive the render that
    // created it by a frame, and a double fetch would skip a page.
    if (hasMore && !isLoading) cb.current();
  }, [hasMore, isLoading]);

  useEffect(() => {
    const el = sentinelRef.current;
    if (!el || !hasMore || isLoading) return;

    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) fire();
      },
      { rootMargin },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [hasMore, isLoading, rootMargin, fire]);

  return sentinelRef;
}
