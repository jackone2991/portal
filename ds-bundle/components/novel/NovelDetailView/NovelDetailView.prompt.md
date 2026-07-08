NovelDetailView from portal-frontend. Use via `window.PortalUI.NovelDetailView` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Library · Novel detail — port of `views/library/novel/detail.blade.php`.
Rendered inside MasterBase. `id` comes from the `[id]` route segment.

## Examples

### Detail

```jsx
() => (
  <div style={{ background: "var(--tpl-body, #f5f6fa)", padding: 32 }}>
    <NovelDetailView id="42" />
  </div>
)
```
