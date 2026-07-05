import { AuthLanding } from "./AuthLanding";

/**
 * Register — same Olympus landing layout with the register tab active.
 * Rendered inside MasterPublic at /register. Registration is local (Portal owns
 * credentials): the form POSTs to /api/v1/auth/register. See AuthForm.
 */
export function RegisterView() {
  return <AuthLanding defaultTab="register" />;
}
