import { AuthLanding } from "./AuthLanding";

/**
 * Register — same Olympus landing layout with the register tab active.
 * Rendered inside MasterPublic at /register. Real registration is via the
 * OIDC provider (Authentik); see AuthForm.
 */
export function RegisterView() {
  return <AuthLanding defaultTab="register" />;
}
