SidebarLeft from portal-frontend. Use via `window.PortalUI.SidebarLeft` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Props

```ts
interface SidebarLeftProps {
collapsed: boolean; onToggle: () => void;
}
```

## Examples

### Expanded

```jsx
() => (
  <div data-template="v1" className="sb-panel" style={frame}>
    <style>{css}</style>
    <SidebarLeft collapsed={false} onToggle={() => {}} />
  </div>
)
```

### Collapsed

```jsx
() => (
  <div data-template="v1" className="sb-panel sb-rail" style={frame}>
    <style>{css}</style>
    <SidebarLeft collapsed onToggle={() => {}} />
  </div>
)
```
