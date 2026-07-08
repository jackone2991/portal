CommentForm from portal-frontend. Use via `window.PortalUI.CommentForm` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### AddComment

```jsx
() => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <div
      style={{
        width: 600,
        background: "var(--tpl-surface)",
        border: "1px solid var(--tpl-border)",
        borderRadius: 12,
        padding: 20,
      }}
    >
      <CommentForm displayName="Marina Valentine" onSubmit={() => {}} />
    </div>
  </div>
)
```
