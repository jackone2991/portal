FriendRequestsMenu from portal-frontend. Use via `window.PortalUI.FriendRequestsMenu` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Props

```ts
interface FriendRequestsMenuProps {
open: boolean; onToggle: () => void;
}
```

## Examples

### Open

```jsx
() => (
  <div
    style={{
      position: "relative",
      width: 380,
      minHeight: 460,
      display: "flex",
      justifyContent: "flex-end",
      alignItems: "flex-start",
      padding: 16,
      borderRadius: 12,
      background: "var(--tpl-header, #3f4257)",
    }}
  >
    <FriendRequestsMenu open onToggle={() => {}} />
  </div>
)
```
