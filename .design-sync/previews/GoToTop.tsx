import { GoToTop } from "portal-frontend";

// Scroll-to-top FAB. In real use it's `position:fixed` bottom-right and stays
// `opacity-0 / pointer-events-none` until the page is scrolled past 300px — so
// raw in a preview it is both viewport-anchored and invisible. We frame a faux
// page surface (position:relative + transform so `fixed` is contained to it) and
// a scoped style reveals the button (the "scrolled" state) inside the card.

export const Button = () => (
  <div
    style={{
      position: "relative",
      transform: "translateZ(0)",
      width: 300,
      height: 260,
      overflow: "hidden",
      borderRadius: 12,
      padding: 20,
      background: "var(--tpl-surface, #fff)",
      border: "1px solid var(--tpl-border, #e6e6ef)",
    }}
  >
    <style>{`
      [aria-label="Go to top"]{
        position:absolute !important;
        top:auto !important;
        left:auto !important;
        bottom:20px !important;
        right:20px !important;
        opacity:1 !important;
        pointer-events:auto !important;
      }
    `}</style>
    {/* faux page content so the FAB reads as an in-page overlay */}
    <div style={{ display: "grid", gap: 10 }}>
      {[86, 70, 92, 60, 78, 66].map((w, i) => (
        <div
          key={i}
          style={{
            height: 12,
            width: `${w}%`,
            borderRadius: 6,
            background: "var(--tpl-surface-2, #f0f1f6)",
          }}
        />
      ))}
    </div>
    <GoToTop />
  </div>
);
