"use client";

import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { getContinueItems } from "@/lib/media-assets";

// Continue rail (SPEC-06 P0.4). Wired to /continue; renders nothing when empty or
// on error (empty-state degrade, failure-isolated).
export function ContinueWidget() {
  const { data } = useQuery({ queryKey: ["continue", "home"], queryFn: () => getContinueItems(5), retry: false });
  const items = data?.items ?? [];
  if (items.length === 0) return null;
  return (
    <div className="rounded-xl border p-4" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
      <h3 className="mb-3 text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>Continue</h3>
      <ul className="space-y-2">
        {items.map((it) => (
          <li key={it.ref_id}>
            <Link href={it.href as Route} className="flex items-center gap-2 text-sm hover:opacity-90" style={{ color: "var(--tpl-heading)" }}>
              <span className="h-1.5 w-1.5 rounded-full" style={{ background: "var(--tpl-accent)" }} />
              <span className="truncate">{it.title}</span>
              <span className="ml-auto text-xs" style={{ color: "var(--tpl-muted)" }}>{it.progress_pct}%</span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
