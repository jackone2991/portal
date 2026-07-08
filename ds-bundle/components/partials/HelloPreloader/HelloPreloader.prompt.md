HelloPreloader from portal-frontend. Use via `window.PortalUI.HelloPreloader` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Loading overlay — port of `partials/hellopreloader.blade.php`.
Shown on first paint, removed once the client mounts.

## Examples

### Overlay

```jsx
() => {
  const [tick, setTick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setTick((t) => t + 1), 120);
    return () => clearInterval(id);
  }, []);

  return (
    <div
      style={{
        position: "relative",
        transform: "translateZ(0)",
        width: 300,
        height: 240,
        overflow: "hidden",
        borderRadius: 12,
        border: "1px solid var(--tpl-border, #e6e6ef)",
      }}
    >
      <HelloPreloader key={tick} />
    </div>
  );
}
```
