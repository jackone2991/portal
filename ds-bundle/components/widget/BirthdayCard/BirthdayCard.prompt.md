BirthdayCard from portal-frontend. Use via `window.PortalUI.BirthdayCard` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Birthday rail card (SPEC-08 P0.5) — wired to /people/upcoming-birthdays. Shows
the nearest upcoming birthday with real state; renders nothing when there are
none (the rail slot degrades to empty, SPEC-06 pattern). Olympus purple design.
