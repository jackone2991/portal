BlogCard from portal-frontend. Use via `window.PortalUI.BlogCard` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### Default

```jsx
() => (
  <div style={{ background: "#f5f6fa", padding: 16, maxWidth: 360 }}>
    <BlogCard
      title="The Majestic Canyon"
      excerpt="We hiked the north rim at dawn and watched the light pour into a mile of red sandstone."
      author="Marina Valentine"
      date="March 4, 2024"
      category="Travel"
      likes={248}
      comments={31}
    />
  </div>
)
```
