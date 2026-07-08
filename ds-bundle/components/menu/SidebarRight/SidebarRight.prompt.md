SidebarRight from portal-frontend. Use via `window.PortalUI.SidebarRight` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Props

```ts
interface SidebarRightProps {
collapsed: boolean; onToggle: () => void;
}
```

## Examples

### Friends

```jsx
() => (
  <div data-template="v1" className="sb-panel" style={frame}>
    <style>{css}</style>
    <SidebarRight collapsed={false} onToggle={() => {}} />
  </div>
)
```

### Rail

```jsx
() => (
  <div data-template="v1" className="sb-panel sb-rail" style={frame}>
    <style>{css}</style>
    <SidebarRight collapsed onToggle={() => {}} />
  </div>
)
```
