FriendRequestItem from portal-frontend. Use via `window.PortalUI.FriendRequestItem` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

A friend-request row — React port of the Olympus `.friend-requests` list item
(Your Account - Friends Requests 1751-1780): Avatar (+presence), name, mutual
count, and accept / decline circular buttons (reusing ControlBlockButtons).

## Examples

### Default

```jsx
() => (
  <div style={{ background: "#ffffff", padding: 16, maxWidth: 360 }}>
    <FriendRequestItem
      name="Green Goo Rock"
      mutual={12}
      status="online"
      onAccept={() => {}}
      onDecline={() => {}}
    />
  </div>
)
```
