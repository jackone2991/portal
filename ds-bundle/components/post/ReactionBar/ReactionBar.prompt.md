ReactionBar from portal-frontend. Use via `window.PortalUI.ReactionBar` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### Liked

```jsx
() => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <div style={card}>
      <ReactionBar
        likes={128}
        likedBy={["Marina Valentine", "Diego Morales", "Priya Anand"]}
        comments={24}
        shares={7}
        liked
      />
    </div>
  </div>
)
```

### Default

```jsx
() => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <div style={card}>
      <ReactionBar likes={3} likedBy={["Anselm Richter"]} comments={1} shares={0} />
    </div>
  </div>
)
```
