"use client";

import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { Avatar } from "../ui/Avatar";
import { WidgetCard } from "./WidgetCard";
import { listNotifications } from "@/lib/notifications";
import { relativeTime, useTimeConfig } from "@/lib/time";

/**
 * Recent-activity rail — real notifications (SPEC-04), replacing the Olympus
 * "Activity Feed" fixtures. Self-fetching (D-32); a compact empty state keeps the
 * slot visible when there's no recent activity. Keeps the export name so
 * HomeView's rail is unchanged.
 */
export function ActivityFeed() {
  const { data } = useQuery({
    queryKey: ["notifications", "rail"],
    queryFn: () => listNotifications(),
    retry: false,
  });
  const { data: tc } = useTimeConfig();
  const nowMs = tc?.now.getTime() ?? Date.now();
  const items = (data?.items ?? []).slice(0, 5);

  return (
    <WidgetCard title="Recent activity">
      {items.length === 0 ? (
        <p className="py-1 text-xs" style={{ color: "var(--tpl-muted)" }}>
          You&apos;re all caught up.
        </p>
      ) : (
        <ul className="space-y-4">
          {items.map((it) => {
            const href = it.data?.href;
            return (
              <li key={it.id} className="flex items-start gap-3">
                <Avatar name={it.title} size={36} />
                <div className="min-w-0 flex-1">
                  <p className="text-sm leading-snug" style={{ color: "var(--tpl-text)" }}>
                    {href ? (
                      <Link
                        href={href as Route}
                        className="font-semibold hover:underline"
                        style={{ color: "var(--tpl-heading)" }}
                      >
                        {it.title}
                      </Link>
                    ) : (
                      <b style={{ color: "var(--tpl-heading)" }}>{it.title}</b>
                    )}
                    {it.body ? <span style={{ color: "var(--tpl-muted)" }}> — {it.body}</span> : null}
                  </p>
                  <p className="mt-0.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
                    {relativeTime(it.created_at, nowMs)}
                  </p>
                </div>
                {it.read_at === null && (
                  <span
                    className="mt-1.5 h-2 w-2 shrink-0 rounded-full"
                    style={{ background: "var(--tpl-accent)" }}
                    aria-label="unread"
                  />
                )}
              </li>
            );
          })}
        </ul>
      )}
    </WidgetCard>
  );
}
