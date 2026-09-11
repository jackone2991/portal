// The caller's identity, shared. `/auth/me` was being fetched ad-hoc from three
// places with three different shapes; this is the one query key for it, so the
// header, the menu and any screen that needs "who am I" hit the cache instead of
// the network.

import { useQuery } from "@tanstack/react-query";
import { api } from "./api-client";

export interface Session {
  id: string;
  email: string;
  display_name: string;
  roles: string[];
  /** Effective permission codes, wildcards included. See `can()`. */
  permissions: string[];
}

export const SESSION_KEY = ["session"] as const;

export function useSession() {
  return useQuery({
    queryKey: SESSION_KEY,
    queryFn: () => api<Session>("/api/v1/auth/me"),
    // Identity does not change under the user mid-page, and a stale menu is
    // worse than a slightly old one only in the other direction: keep it warm
    // for a minute, then let a navigation refresh it.
    staleTime: 60_000,
    retry: false,
  });
}

/**
 * Does this permission set satisfy `required`?
 *
 * Mirrors the server's grammar in
 * `backend/internal/modules/account/rbac/permission.go` — `<resource>:<action>[:<scope>]`
 * with `*` allowed in any segment — because the API hands back grants verbatim
 * (a superadmin's whole set is `["*"]`) and a plain `includes()` would decide
 * that a superadmin can do nothing.
 *
 * This is for deciding what to OFFER. The server re-checks everything; a wrong
 * answer here shows or hides a button, it never grants access.
 */
export function can(granted: string[] | undefined, required: string): boolean {
  if (!granted?.length) return false;
  const req = required.split(":");
  if (req.length < 2 || req.length > 3) return false;

  return granted.some((code) => {
    if (code === "*") return true;
    const g = code.split(":");
    if (g.length < 2 || g.length > 3) return false;
    if (!segMatch(g[0], req[0]) || !segMatch(g[1], req[1])) return false;

    // Scope rules, same as the server: a 2-segment grant satisfies a bare or
    // `:any` requirement but never `:own`, and an `:own` grant matches only
    // `:own`. Getting this wrong in the lenient direction would advertise
    // screens that 403 on arrival.
    const gScope = g[2];
    const rScope = req[2];
    if (gScope === undefined) return rScope === undefined || rScope === "any";
    if (gScope === "*") return true;
    if (gScope === "any") return rScope === undefined || rScope === "any";
    return gScope === rScope;
  });
}

function segMatch(granted: string | undefined, required: string | undefined): boolean {
  return granted === "*" || granted === required;
}

/** Convenience for the nav: may this session open the admin console at all? */
export function canOpenAdmin(permissions: string[] | undefined): boolean {
  return can(permissions, "users:read:any") || can(permissions, "rbac:role:read");
}
