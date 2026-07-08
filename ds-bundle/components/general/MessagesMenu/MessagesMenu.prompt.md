MessagesMenu from portal-frontend. Use via `window.PortalUI.MessagesMenu` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Props

```ts
interface MessagesMenuProps {
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
      minHeight: 340,
      display: "flex",
      justifyContent: "flex-end",
      alignItems: "flex-start",
      padding: 16,
      borderRadius: 12,
      background: "var(--tpl-header, #3f4257)",
    }}
  >
    <MessagesMenu open onToggle={() => {}} />
  </div>
)
```
