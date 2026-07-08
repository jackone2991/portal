Icon from portal-frontend. Use via `window.PortalUI.Icon` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Renders an Olympus sprite icon by `<use>`-referencing the symbol injected by
<SvgSprite/>. `name` is the id without the `olymp-` prefix (e.g. "heart-icon").
Height = `size`; width is derived from the symbol's native viewBox so the icon
keeps its aspect ratio. Colour follows `currentColor`.

## Props

```ts
interface IconProps {
name: string; size?: number; className?: string; style?: React.CSSProperties;
}
```

## Examples

### Gallery

```jsx
() => (
  <div style={{ display: "flex", flexWrap: "wrap", gap: 18, color: "#3f4257", maxWidth: 470 }}>
    {COMMON.map((n) => (
      <div key={n} style={{ display: "grid", justifyItems: "center", gap: 6, width: 100 }}>
        <Icon name={n} size={26} />
        <span style={{ fontSize: 11, color: "#888da8" }}>{n.replace(/-icon$/, "")}</span>
      </div>
    ))}
  </div>
)
```

### Sizes

```jsx
() => (
  <div style={{ display: "flex", alignItems: "center", gap: 16, color: "#ff5e3a" }}>
    <Icon name="heart-icon" size={16} />
    <Icon name="heart-icon" size={24} />
    <Icon name="heart-icon" size={32} />
    <Icon name="heart-icon" size={48} />
  </div>
)
```
