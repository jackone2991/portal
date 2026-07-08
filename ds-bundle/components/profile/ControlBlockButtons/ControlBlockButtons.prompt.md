ControlBlockButtons from portal-frontend. Use via `window.PortalUI.ControlBlockButtons` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

A row of circular action buttons — the Olympus `.control-block-button`
(Profile Page 2965-2995): coloured discs each showing one sprite icon.
Reused by ProfileHeader, FriendCard and FriendRequestItem.

The per-button icon is clamped to fit its disc so wide sprites (notably
`three-dots-icon`, a 128×32 glyph) don't overflow the circle.

## Examples

### Actions

```jsx
() => (
  <div data-template="v1" style={box}>
    <ControlBlockButtons buttons={buttons} />
  </div>
)
```

### Large

```jsx
() => (
  <div data-template="v1" style={box}>
    <ControlBlockButtons buttons={buttons} size={56} />
  </div>
)
```
