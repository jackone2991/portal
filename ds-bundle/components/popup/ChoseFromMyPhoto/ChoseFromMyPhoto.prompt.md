ChoseFromMyPhoto from portal-frontend. Use via `window.PortalUI.ChoseFromMyPhoto` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### Dialog

```jsx
() => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 600, background: "#f5f6fa" }}>
    <ChoseFromMyPhoto open onClose={() => {}} />
  </div>
)
```
