FormField from portal-frontend. Use via `window.PortalUI.FormField` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Material floating-label text input — the standalone, richer sibling of the
`Field` used inside the popups. Ports the Olympus `.form-group.label-floating`
behaviour: the label sits in the input as a placeholder and animates up when
the field is focused or filled, while a bottom accent underline grows on focus.

Controlled (`value` + `onChange`) or uncontrolled (omit both) — an internal
value mirror keeps the floating label correct either way.

## Examples

### Fields

```jsx
() => (
  <div data-template="v1" style={box}>
    <FormField label="First Name" value="Marina" onChange={() => {}} />
    <FormField
      label="Your Email"
      type="email"
      value="marina@spiegel.io"
      icon="info-icon"
      onChange={() => {}}
    />
    <FormField label="Birthday" type="date" value="1993-04-12" onChange={() => {}} />
  </div>
)
```

### States

```jsx
() => (
  <div data-template="v1" style={box}>
    {/* uncontrolled + empty → label rests inside the field */}
    <FormField label="Display Name" placeholder="How should we call you?" />
    <FormField label="Account ID" value="usr_1f9c" onChange={() => {}} disabled />
  </div>
)
```
