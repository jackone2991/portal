UpdateHeaderPhoto from portal-frontend. Use via `window.PortalUI.UpdateHeaderPhoto` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

"Update Header Photo" popup — port of Olympus `#update-header-photo`
(`Profile Page.html`). Two options: upload from computer, or choose from the
user's existing photos (which opens the ChoseFromMyPhoto picker).

## Examples

### Dialog

```jsx
() => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 360, background: "#f5f6fa" }}>
    <UpdateHeaderPhoto open onClose={() => {}} />
  </div>
)
```
