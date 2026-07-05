import { NextResponse, type NextRequest } from "next/server";

/**
 * Auth gate. A guest (no `portal_access` session cookie) hitting the app root is
 * redirected to the login page — so https://portal.localhost/ shows login first.
 *
 * Once the OIDC flow sets `portal_access`, an authenticated user passes through
 * to the home/newsfeed at `/`. To protect more routes later, add them to
 * `matcher` (e.g. "/library/:path*", "/account/:path*").
 */
export function middleware(req: NextRequest) {
  const authenticated = req.cookies.has("portal_access");
  if (!authenticated) {
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    return NextResponse.redirect(url);
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/"],
};
