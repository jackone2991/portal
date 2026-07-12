"use client";

import Link from "next/link";
import type { Route } from "next";
import type { StreamItem } from "@/lib/stream";
import { Icon } from "../ui/Icon";

// One life-stream card (SPEC-06 P0.2). Journal items render their text; system
// items render the synthesized title as a compact, optionally-linked card.
export function StreamItemCard({ item, displayName }: { item: StreamItem; displayName?: string }) {
  const when = new Date(item.occurred_at).toLocaleDateString(undefined, { day: "numeric", month: "short" });

  if (item.source_module === "journal") {
    return (
      <article
        className="rounded-xl border p-4"
        style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
      >
        <div className="mb-1 flex items-center justify-between text-xs" style={{ color: "var(--tpl-muted)" }}>
          <span className="font-medium" style={{ color: "var(--tpl-heading)" }}>{displayName ?? "You"}</span>
          <span>{when}{item.mood ? ` · ${item.mood}` : ""}</span>
        </div>
        <p className="whitespace-pre-wrap text-sm" style={{ color: "var(--tpl-heading)" }}>{item.body_md}</p>
      </article>
    );
  }

  const icon = iconFor(item.source_module);
  const body = (
    <div
      className="flex items-center gap-3 rounded-xl border p-3"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      <span className="grid h-9 w-9 shrink-0 place-items-center rounded-full" style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-accent)" }}>
        <Icon name={icon} size={18} />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium" style={{ color: "var(--tpl-heading)" }}>{item.title || item.event_type}</p>
        <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>{when}</p>
      </div>
    </div>
  );

  return item.href ? (
    <Link href={item.href as Route} className="block transition hover:opacity-90">{body}</Link>
  ) : (
    body
  );
}

function iconFor(module: string): string {
  switch (module) {
    case "media":
      return "multimedia-icon";
    case "bank":
      return "stats-icon";
    case "comic":
      return "star-icon";
    case "people":
      return "cupcake-icon";
    default:
      return "newsfeed-icon";
  }
}
