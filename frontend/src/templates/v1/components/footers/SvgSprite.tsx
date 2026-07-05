/**
 * Inline SVG icon sprite — port of `components/footers/svg.blade.php`.
 *
 * The Olympus theme references icons via `<use xlink:href="#olymp-*">`. Paste the
 * `<symbol id="olymp-…">` definitions from `template-main/portal/public/v1`
 * here (or serve the sprite from `frontend/public/` and `<use href="/sprite.svg#…">`).
 *
 * `footers/js.blade.php` (script tags) and `footers/ico.blade.php` (favicons) are
 * not ported as components — scripts become React/hooks, favicons go in `metadata`.
 */
export function SvgSprite() {
  return (
    <svg
      aria-hidden
      style={{ position: "absolute", width: 0, height: 0, overflow: "hidden" }}
    >
      {/* TODO: <symbol id="olymp-menu-icon">…</symbol> etc. */}
    </svg>
  );
}
