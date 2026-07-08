ToggleRow from portal-frontend. Use via `window.PortalUI.ToggleRow` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Settings row — port of the Olympus `.description-toggle` + `.togglebutton`
pairing. Title (and optional description) on the left, an on/off switch on the
right that fills with the accent when on. The switch is a proper ARIA
`role="switch"` button so it stays keyboard- and screen-reader-friendly.
