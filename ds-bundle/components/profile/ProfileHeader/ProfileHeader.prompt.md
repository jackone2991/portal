ProfileHeader from portal-frontend. Use via `window.PortalUI.ProfileHeader` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Profile page header — React port of the Olympus `.top-header`
(Profile Page 2918-3007): a gradient cover strip with an "Update Cover"
affordance, an overlapping Avatar, name + location, a horizontal tab menu,
and the ControlBlockButtons (add friend / message / settings).

`activeTab` / `onTab` are controlled by the parent; leave them unset for a
static header.

## Examples

### Timeline

```jsx
() => (
  <div data-template="v1" style={page}>
    <ProfileHeader
      name="James Spiegel"
      location="San Francisco, CA"
      activeTab="Timeline"
      onTab={() => {}}
    />
  </div>
)
```

### Photos

```jsx
() => (
  <div data-template="v1" style={page}>
    <ProfileHeader
      name="James Spiegel"
      location="San Francisco, CA"
      activeTab="Photos"
      onTab={() => {}}
    />
  </div>
)
```
