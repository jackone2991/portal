FriendSuggestions from portal-frontend. Use via `window.PortalUI.FriendSuggestions` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

People rail — real contacts from the people module (SPEC-08), replacing the
Olympus "Friend Suggestions" fixtures. Self-fetching (D-32: TanStack owns
server state); shows a compact empty state so the rail slot stays visible when
the registry is empty. Keeps the export name so HomeView's rail is unchanged.
