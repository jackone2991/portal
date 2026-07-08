SelectField from portal-frontend. Use via `window.PortalUI.SelectField` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Floating-label native `<select>` — the Olympus `.form-group.label-floating
.is-select` control. A select always carries a value, so the label stays in
the floated slot; it recolours to the accent on focus alongside the growing
underline. Accepts either a plain `string[]` or `{value,label}[]` options.

## Examples

### Select

```jsx
() => (
  <div data-template="v1" style={box}>
    <SelectField
      label="Band Type"
      options={["Rock Band", "Pop Band", "Jazz Band"]}
      value="Pop Band"
      onChange={() => {}}
    />
    <SelectField
      label="Primary Genre"
      options={[
        { value: "synthwave", label: "Synthwave" },
        { value: "ambient", label: "Ambient" },
        { value: "lofi", label: "Lo-Fi" },
      ]}
      value="synthwave"
      onChange={() => {}}
    />
  </div>
)
```
