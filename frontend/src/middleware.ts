import { NextResponse, type NextRequest } from "next/server";

/**
 * Auth gate (ADR-06 local auth). A guest (no `portal_session` marker cookie) is
 * sent to the Portal `/login` page — a real email+password form served by Portal
 * itself, no IdP redirect. The path they were heading to is preserved in `?next=`
 * so the form can bounce them back after a successful login.
 *
 * Authenticated users who land on /login or /register are bounced to the app root.
 */
export function middleware(req: NextRequest) {
  // Gate on the durable session marker (`portal_session`), not the short-lived
  // access cookie: the access token expires every few minutes and is silently
  // re-minted client-side by SessionKeeper, so its absence ≠ logged out.
  const authenticated = req.cookies.has("portal_session");
  const { pathname } = req.nextUrl;
  const isAuthPage = pathname === "/login" || pathname === "/register";

  if (!authenticated && !isAuthPage) {
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    url.search = "";
    url.searchParams.set("next", pathname);
    return NextResponse.redirect(url);
  }

  if (authenticated && isAuthPage) {
    const url = req.nextUrl.clone();
    url.pathname = "/";
    url.search = "";
    return NextResponse.redirect(url);
  }

  return NextResponse.next();
}

export const config = {
  // App entry + the auth pages. Next excludes _next/* assets automatically.
  matcher: ["/", "/login", "/register", "/upload", "/library/:path*", "/bank/:path*", "/people/:path*"],
};
