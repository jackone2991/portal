Composer from portal-frontend. Use via `window.PortalUI.Composer` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### CreatePost

```jsx
() => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16, maxWidth: 640 }}>
    <Composer displayName="Marina Valentine" onPost={() => {}} />
  </div>
)
```
