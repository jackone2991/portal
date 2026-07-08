RegisterView from portal-frontend. Use via `window.PortalUI.RegisterView` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Register — same Olympus landing layout with the register tab active.
Rendered inside MasterPublic at /register. Registration is local (Portal owns
credentials): the form POSTs to /api/v1/auth/register. See AuthForm.

## Examples

### Register

```jsx
() => <RegisterView />
```
