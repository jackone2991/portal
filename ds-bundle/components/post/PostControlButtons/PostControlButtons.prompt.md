PostControlButtons from portal-frontend. Use via `window.PortalUI.PostControlButtons` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### FabColumn

```jsx
() => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <div
      style={{
        position: "relative",
        width: 500,
        marginRight: 40,
        minHeight: 180,
        background: "var(--tpl-surface)",
        border: "1px solid var(--tpl-border)",
        borderRadius: 12,
        padding: 20,
      }}
    >
      <p style={{ color: "var(--tpl-heading)", fontWeight: 600 }}>Marina Valentine</p>
      <p style={{ marginTop: 8, color: "var(--tpl-text)", fontSize: 14, lineHeight: 1.6 }}>
        The FAB column floats half-outside this card&apos;s right edge — award,
        like, comment, and share, with the like disc shown in its active accent
        state.
      </p>
      <PostControlButtons liked />
    </div>
  </div>
)
```
