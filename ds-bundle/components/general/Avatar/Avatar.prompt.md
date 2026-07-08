Avatar from portal-frontend. Use via `window.PortalUI.Avatar` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Deterministic initials avatar — a gradient disc with the name's initials.
Self-contained (no image assets); the colour is derived from the name so the
same person is always the same hue. Pass `status` to overlay an Olympus-style
presence dot (top-left, white-ringed, coloured by `--tpl-status-*`).

## Props

```ts
interface AvatarProps {
name: string; size?: number; className?: string;
}
```

## Examples

### Sizes

```jsx
() => (
  <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
    <Avatar name="Ada Lovelace" size={28} />
    <Avatar name="Ada Lovelace" size={40} />
    <Avatar name="Ada Lovelace" size={56} />
    <Avatar name="Ada Lovelace" size={72} />
  </div>
)
```

### People

```jsx
() => (
  <div style={{ display: "flex", alignItems: "flex-start", gap: 12 }}>
    {["Ada Lovelace", "Grace Hopper", "Alan Turing", "Katherine Johnson", "Linus Torvalds"].map(
      (n) => (
        <div key={n} style={{ display: "grid", justifyItems: "center", gap: 6, width: 88 }}>
          <Avatar name={n} size={52} />
          <span style={{ fontSize: 12, color: "#515365", textAlign: "center" }}>{n}</span>
        </div>
      ),
    )}
  </div>
)
```
