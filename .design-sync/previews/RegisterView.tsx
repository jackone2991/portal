import { RegisterView } from "portal-frontend";

// Route wrapper for /register — the same Olympus landing layout as LoginView
// with the register tab active and "Join the biggest social network" copy.
// Registration is local (Portal owns credentials); the form POSTs to
// /api/v1/auth/register.

export const Register = () => <RegisterView />;
