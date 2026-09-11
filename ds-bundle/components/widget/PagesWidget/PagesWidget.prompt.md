PagesWidget from portal-frontend. Use via `window.PortalUI.PagesWidget` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Discover rail — real published comics from the comic module (SPEC-02),
replacing the Olympus "Pages You May Like" fixtures. Self-fetching (D-32); a
compact empty state keeps the slot visible when nothing is published yet.
Keeps the export name so HomeView's rail is unchanged.
