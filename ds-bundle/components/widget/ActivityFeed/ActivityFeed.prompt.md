ActivityFeed from portal-frontend. Use via `window.PortalUI.ActivityFeed` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Recent-activity rail — real notifications (SPEC-04), replacing the Olympus
"Activity Feed" fixtures. Self-fetching (D-32); a compact empty state keeps the
slot visible when there's no recent activity. Keeps the export name so
HomeView's rail is unchanged.
