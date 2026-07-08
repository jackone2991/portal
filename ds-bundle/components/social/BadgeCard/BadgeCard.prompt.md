BadgeCard from portal-frontend. Use via `window.PortalUI.BadgeCard` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

A community-badge card — React port of the Olympus `.birthday-item.badges`
(Community Badges 2563-2579): a badge icon disc with an optional count label,
a title + description, and an accent progress meter.

`progress` (0-100) is clamped; omit it to hide the meter.
