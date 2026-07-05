"use client";

/**
 * Scroll-to-top button — port of `partials/goToTop.blade.php`.
 */
export function GoToTop() {
  return (
    <button
      type="button"
      aria-label="Go to top"
      onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
      className="fixed bottom-6 right-6 z-40 rounded-full border px-3 py-2 text-xs backdrop-blur"
      style={{ borderColor: "var(--tpl-border)", background: "rgba(0,0,0,0.5)" }}
    >
      ↑ Top
    </button>
  );
}
