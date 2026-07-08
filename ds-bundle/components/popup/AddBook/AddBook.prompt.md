AddBook from portal-frontend. Use via `window.PortalUI.AddBook` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### Dialog

```jsx
() => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 560, background: "#f5f6fa" }}>
    <AddBook open onClose={() => {}} />
  </div>
)
```
