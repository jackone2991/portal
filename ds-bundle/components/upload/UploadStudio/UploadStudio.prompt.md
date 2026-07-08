UploadStudio from portal-frontend. Use via `window.PortalUI.UploadStudio` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Upload → transcode → playback studio (v1 media slice). The browser uploads the
original through the API (POST /assets → PUT /assets/{id}/source → complete),
polls until the worker's HLS output is ready, then plays it with Vidstack.

## Examples

### Studio

```jsx
() => (
  <div style={{ background: "var(--tpl-body, #f5f6fa)", padding: 32 }}>
    <UploadStudio />
  </div>
)
```
