GoToTop from portal-frontend. Use via `window.PortalUI.GoToTop` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Scroll-to-top button — port of `partials/goToTop.blade.php`.
Appears once the page is scrolled; sits at the bottom-right, offset clear of
the right friends panel on xl+ (its width lives in --tpl-rightbar-cur). The
chat launcher is the "Olympus Chat" bar in the right sidebar, not a FAB here.

## Examples

### Button

```jsx
() => (
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
)
```
