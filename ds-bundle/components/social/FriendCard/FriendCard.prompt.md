FriendCard from portal-frontend. Use via `window.PortalUI.FriendCard` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

A friend card — React port of the Olympus `.friend-item`
(Profile Page - Friends): a gradient cover strip, an overlapping Avatar,
name + country, a Friends/Photos/Videos stats row, and the add / message
ControlBlockButtons.

## Examples

### Default

```jsx
() => (
  <div style={{ background: "#f5f6fa", padding: 16, maxWidth: 360 }}>
    <FriendCard
      name="Diana Jameson"
      country="United States"
      stats={{ friends: 428, photos: 197, videos: 34 }}
    />
  </div>
)
```
