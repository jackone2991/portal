TopMenu from portal-frontend. Use via `window.PortalUI.TopMenu` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Header bar content — port of `components/headers/menu.blade.php`.
Logo · page title · search · Find Friends · notification dropdowns · profile.
The right cluster shows one dropdown at a time; clicking outside or Escape
closes it. Olympus sprite icons throughout.

## Examples

### Header

```jsx
() => (
  <div
    style={{
      height: "var(--tpl-header-h, 72px)",
      background: "var(--tpl-header, #3f4257)",
      color: "#fff",
    }}
  >
    <TopMenu />
  </div>
)
```
