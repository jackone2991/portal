import { AuthLanding } from "portal-frontend";

// Shared Olympus "Landing Page" two-column auth layout — welcome column on the
// left, AuthForm card on the right. Used by both /login and /register; only the
// active tab and left-column copy differ. Full-screen with its own ambient
// radial background.

export const Landing = () => <AuthLanding defaultTab="login" />;
