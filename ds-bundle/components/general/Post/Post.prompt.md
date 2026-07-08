Post from portal-frontend. Use via `window.PortalUI.Post` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

## Examples

### TextPost

```jsx
() => (
  <div data-template="v1" style={frame}>
    <Post
      author="Marina Valentine"
      action="shared a thought"
      time="18 minutes ago"
      text="Just wrapped the final colour pass on the summer short film. Six months of night shoots and it finally looks the way it sounded in my head. Screening the first cut for the crew on Friday."
      likes={128}
      likedBy={["Diego Morales", "Priya Anand", "Anselm Richter"]}
      comments={24}
      shares={7}
      liked
    />
  </div>
)
```

### VideoPost

```jsx
() => (
  <div data-template="v1" style={frame}>
    <Post
      author="Diego Morales"
      action="posted a video"
      time="2 hours ago"
      text="New behind-the-scenes reel is live — camera tests, lens breakdowns, and the gimbal rig that saved the rooftop chase sequence."
      media={{
        type: "video",
        title: "Behind the Lens: Shooting the Rooftop Chase",
        desc: "A twelve-minute walkthrough of the anamorphic setup and the lighting plan for the night exterior.",
        source: "PORTAL STUDIO · 12:04",
      }}
      likes={342}
      likedBy={["Marina Valentine", "Nadia Okonkwo", "Priya Anand", "Anselm Richter"]}
      comments={58}
      shares={19}
    />
  </div>
)
```

## Related

`PostControlButtons`
