TrackItem from portal-frontend. Use via `window.PortalUI.TrackItem` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### Playlist

```jsx
() => (
  <div style={{ background: "#ffffff", padding: 16, maxWidth: 360 }}>
    <ol>
      <TrackItem index={1} title="ChillGroves" artist="Iron Maid" duration="3:24" playing onPlay={() => {}} />
      <TrackItem index={2} title="Midnight Static" artist="The Velvet Circuit" duration="4:07" onPlay={() => {}} />
      <TrackItem index={3} title="Paper Boats" artist="Marisol Vega" duration="2:58" onPlay={() => {}} />
    </ol>
  </div>
)
```
