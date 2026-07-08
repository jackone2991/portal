LoginView from portal-frontend. Use via `window.PortalUI.LoginView` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Landing + login — port of the Olympus "Landing Page" login form
(template-main/social). Rendered inside MasterPublic at /login.

## Examples

### Login

```jsx
() => <LoginView />
```
