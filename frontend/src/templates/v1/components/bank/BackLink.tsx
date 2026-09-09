"use client";

import Link from "next/link";
import type { Route } from "next";

/**
 * "← Sổ thu chi" above a sub-screen's title.
 *
 * The ledger is a hub with four leaves (giao dịch, ví, ngân sách, báo cáo) that
 * are only reachable from the hub, and the app shell's sidebar has a single
 * "Ledger" entry — so once you are on a leaf there is nothing in the chrome that
 * takes you back, and the browser's Back button is the only way out. That is
 * fine for a link you followed, useless for a page you landed on or reloaded.
 *
 * A real `<Link>`, not `router.back()`: history-back would return to whatever
 * page preceded this one, which after a reload or a shared URL is not the hub at
 * all. This always goes to the parent, and it is a plain anchor, so
 * middle-click and "open in new tab" behave.
 */
export function BackLink({ href = "/bank", label = "Sổ thu chi" }: { href?: string; label?: string }) {
  return (
    <Link
      href={href as Route}
      className="mb-1 inline-flex items-center gap-1 text-sm font-medium transition hover:opacity-80"
      style={{ color: "var(--tpl-muted)" }}
    >
      ← {label}
    </Link>
  );
}
