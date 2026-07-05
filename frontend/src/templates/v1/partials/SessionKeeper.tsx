"use client";

import { useEffect } from "react";

/**
 * Keeps the session alive. The access token (JWT) is short-lived (~5 min); this
 * silently POSTs /auth/refresh so it never lapses while the app is open, so the
 * user is not bounced to /login mid-session. Rotation is coordinated across tabs
 * with a localStorage timestamp so concurrent tabs don't trip refresh-reuse
 * detection. On a hard refresh failure (revoked / expired) it clears the marker
 * and redirects to /login.
 */
const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "https://api.portal.localhost";

const REFRESH_MS = 4 * 60 * 1000; // refresh a minute before the 5-min access TTL
const CLAIM_MS = REFRESH_MS - 15_000; // skip if another tab refreshed this recently
const KEY = "portal_refresh_at";

export function SessionKeeper() {
  useEffect(() => {
    let stopped = false;

    async function refresh(force: boolean) {
      const last = Number(localStorage.getItem(KEY) || 0);
      const now = Date.now();
      if (!force && now - last < CLAIM_MS) return; // fresh enough (this or another tab)
      // optimistically claim the slot so a concurrent tab backs off
      localStorage.setItem(KEY, String(now));
      try {
        const res = await fetch(`${API_BASE}/api/v1/auth/refresh`, {
          method: "POST",
          credentials: "include",
        });
        if (res.ok) {
          localStorage.setItem(KEY, String(Date.now()));
        } else if (res.status === 401 && !stopped) {
          localStorage.removeItem(KEY);
          window.location.assign("/login");
        }
      } catch {
        // network blip — let the next tick retry; roll the claim back a little
        localStorage.setItem(KEY, String(last));
      }
    }

    // Ensure a fresh token on load (handles returning after the access TTL lapsed).
    refresh(false);
    const timer = setInterval(() => refresh(false), REFRESH_MS);

    const onVisible = () => {
      if (document.visibilityState === "visible") refresh(false);
    };
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("focus", onVisible);

    return () => {
      stopped = true;
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
      window.removeEventListener("focus", onVisible);
    };
  }, []);

  return null;
}
