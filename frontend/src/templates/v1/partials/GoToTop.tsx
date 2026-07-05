"use client";

import { useEffect, useState } from "react";

/**
 * Scroll-to-top button — port of `partials/goToTop.blade.php`.
 * Appears once the page is scrolled; sits at the bottom-right, offset clear of
 * the right friends panel on xl+ (its width lives in --tpl-rightbar-cur). The
 * chat launcher is the "Olympus Chat" bar in the right sidebar, not a FAB here.
 */
export function GoToTop() {
  const [show, setShow] = useState(false);

  useEffect(() => {
    const onScroll = () => setShow(window.scrollY > 300);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <button
      type="button"
      aria-label="Go to top"
      onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
      className={`fixed bottom-6 right-6 z-40 grid h-12 w-12 place-items-center rounded-full text-white shadow-lg transition-opacity duration-200 hover:opacity-90 xl:right-[calc(var(--tpl-rightbar-cur)+1.5rem)] ${
        show ? "opacity-100" : "pointer-events-none opacity-0"
      }`}
      style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
    >
      <svg
        width="22"
        height="22"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden
      >
        <polyline points="18 15 12 9 6 15" />
      </svg>
    </button>
  );
}
