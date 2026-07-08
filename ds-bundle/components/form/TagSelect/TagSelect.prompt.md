TagSelect from portal-frontend. Use via `window.PortalUI.TagSelect` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Multi-value tag / chip input. Type a value and press Enter (or comma) to add a
chip; each chip carries a `little-delete` X to remove it; Backspace on an empty
field removes the last chip. When `suggestions` are supplied, the still-unused
matches surface as clickable pills below the field while it is focused.

Fully controlled — `values` is the source of truth, `onChange` receives the
next array on every add/remove.

## Examples

### Collaborators

```jsx
() => (
  <div data-template="v1" style={box}>
    <TagSelect
      label="Collaborators"
      values={["Mathilda Brinker", "Nicholas Grissom"]}
      onChange={() => {}}
      placeholder="Add a collaborator…"
    />
  </div>
)
```

### Genres

```jsx
() => (
  <div data-template="v1" style={box}>
    <TagSelect
      label="Genres"
      values={["Ambient", "Synthwave", "Lo-Fi"]}
      onChange={() => {}}
    />
  </div>
)
```
