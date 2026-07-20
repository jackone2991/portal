"use client";

import Link from "next/link";
import type { Route } from "next";
import type { StreamItem } from "@/lib/stream";
import { Icon } from "../ui/Icon";
import { Post } from "../post/Post";
import { formatDate, useTimeConfig } from "@/lib/time";

// One life-stream card (SPEC-06 P0.2). A journal item renders as a full Olympus
// post card (Post: avatar header + text + reaction bar + Like/Comment/Share FABs);
// system items (media/bank/comic/people) render as a compact, optionally-linked
// event card. Reaction counts are 0 — the app has no social layer yet, so the bar
// is the design's chrome, not fake engagement.
export function StreamItemCard({ item, displayName }: { item: StreamItem; displayName?: string }) {
  const { data: tc } = useTimeConfig();
  const when = formatDate(item.occurred_at, tc?.timezone ?? "UTC");

  if (item.source_module === "journal") {
    return (
      <Post
        author={displayName ?? "You"}
        time={item.mood ? `${when} · ${item.mood}` : when}
        text={<span className="whitespace-pre-wrap">{item.body_md}</span>}
        likes={0}
        likedBy={[]}
        comments={0}
        shares={0}
      />
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
