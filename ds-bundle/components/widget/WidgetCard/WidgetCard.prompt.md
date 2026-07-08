WidgetCard from portal-frontend. Use via `window.PortalUI.WidgetCard` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Padded rail-widget shell: a `Card` with `p-4`, an optional `title` bar and an
optional `more` (three-dots) affordance. Wrap any rail content in `children`.
