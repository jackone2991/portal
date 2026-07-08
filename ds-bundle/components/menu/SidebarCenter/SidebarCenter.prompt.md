SidebarCenter from portal-frontend. Use via `window.PortalUI.SidebarCenter` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Fixed top bar — port of `components/menu/sidebarCenter.blade.php`.
The dark Olympus header that spans the full width above everything.

## Examples

### Header

```jsx
() => (
  <div data-template="v1" className="sc-wrap" style={{ width: 900 }}>
    <style>{css}</style>
    <SidebarCenter />
  </div>
)
```
