import { AuthForm } from "portal-frontend";

// Login / register card — the form half of the Olympus landing page. Two real
// tabs: sign-in and register. Portal owns credentials (ADR-06 local auth), so
// both actions POST straight to the API. Wrapped in a padded surface so the card
// has room to breathe.

const Frame = ({ children }: { children: React.ReactNode }) => (
  <div style={{ background: "var(--tpl-body, #f5f6fa)", padding: 32, display: "flex", justifyContent: "center" }}>
    {children}
  </div>
);

export const SignIn = () => (
  <Frame>
    <AuthForm defaultTab="login" />
  </Frame>
);

export const Register = () => (
  <Frame>
    <AuthForm defaultTab="register" />
  </Frame>
);
