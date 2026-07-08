CommentItem from portal-frontend. Use via `window.PortalUI.CommentItem` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### Single

```jsx
() => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <ul style={card}>
      <CommentItem
        author="Priya Anand"
        time="37 minutes ago"
        text="Saving this for the crew screening notes. Can we get a breakdown of the gimbal move at 6:12? It reads almost like a crane shot."
        likes={8}
      />
    </ul>
  </div>
)
```

### WithReply

```jsx
() => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <ul style={card}>
      <CommentItem
        author="Diego Morales"
        time="1 hour ago"
        text="The colour grade on the rooftop scene is unreal — those teal shadows are doing so much work."
        likes={12}
        replies={[
          {
            author: "Marina Valentine",
            time: "48 minutes ago",
            text: "Thank you! We pushed the anamorphic bloom further than the test reel — glad it landed.",
            likes: 4,
          },
        ]}
      />
    </ul>
  </div>
)
```
