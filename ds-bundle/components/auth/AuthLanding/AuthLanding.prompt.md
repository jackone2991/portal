AuthLanding from portal-frontend. Use via `window.PortalUI.AuthLanding` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Olympus "Landing Page" two-column auth layout (welcome + form card) —
port of template-main/social/social/Landing Page.html. Shared by /login and
/register; only the active tab and the left-column copy differ.

## Examples

### Landing

```jsx
() => <AuthLanding defaultTab="login" />
```
