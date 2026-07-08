BirthdayCard from portal-frontend. Use via `window.PortalUI.BirthdayCard` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Birthday rail card — extracted from the inline `BirthdayCard` in
`views/home/HomeView.tsx` (Olympus birthday widget). A purple gradient card
with decorative blobs, a ringed avatar and a "Today is X's Birthday!"
headline. `name` is optional and defaults to the Olympus sample person.
