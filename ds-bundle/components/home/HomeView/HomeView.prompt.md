HomeView from portal-frontend. Use via `window.PortalUI.HomeView` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### Newsfeed

```jsx
() => (
  <div style={{ background: "var(--tpl-body, #f5f6fa)", padding: 24 }}>
    <HomeView />
  </div>
)
```
