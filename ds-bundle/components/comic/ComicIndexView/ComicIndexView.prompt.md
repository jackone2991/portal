ComicIndexView from portal-frontend. Use via `window.PortalUI.ComicIndexView` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Library · Comics index — port of `views/library/commic/index.blade.php`
(folder typo "commic" → "comic"). Rendered inside MasterBase.

## Examples

### Index

```jsx
() => (
  <div style={{ background: "var(--tpl-body, #f5f6fa)", padding: 32 }}>
    <ComicIndexView />
  </div>
)
```
