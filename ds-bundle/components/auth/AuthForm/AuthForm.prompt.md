AuthForm from portal-frontend. Use via `window.PortalUI.AuthForm` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Login / register card — React + Tailwind port of the Olympus "Landing Page"
form. Portal owns credentials (ADR-06 local auth): both actions POST straight
to the API and, on success, the browser carries the `portal_access` cookie to
the app. No IdP redirect, no SSO round-trip.

## Props

```ts
interface AuthFormProps {
defaultTab?: "login" | "register";
}
```

## Examples

### SignIn

```jsx
() => (
  <Frame>
    <AuthForm defaultTab="login" />
  </Frame>
)
```

### Register

```jsx
() => (
  <Frame>
    <AuthForm defaultTab="register" />
  </Frame>
)
```
