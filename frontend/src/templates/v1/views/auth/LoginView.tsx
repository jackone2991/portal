import { AuthLanding } from "./AuthLanding";

/**
 * Landing + login — port of the Olympus "Landing Page" login form
 * (template-main/social). Rendered inside MasterPublic at /login.
 */
export function LoginView() {
  return <AuthLanding defaultTab="login" />;
}
