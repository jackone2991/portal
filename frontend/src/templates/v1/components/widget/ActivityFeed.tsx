import type { ReactNode } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { WidgetCard } from "./WidgetCard";

/**
 * "Activity Feed" rail widget — extracted from the inline `ActivityFeed` in
 * `views/home/HomeView.tsx`. Renders a list of actor/action rows (Avatar +
 * text + relative time + trailing action icon). `items` is optional and
 * defaults to the Olympus sample feed.
 */

export type ActivityItem = {
  /** Person who performed the action. */
  actor: string;
  /** What they did — free-form node so it can embed links, e.g. `commented on …`. */
  action: ReactNode;
  /** Relative time label, e.g. "2 mins ago". */
  time: string;
  /** Olympus sprite icon name for the action. */
  icon: string;
};

const DEFAULT_ITEMS: ActivityItem[] = [
  {
    actor: "Marina Polson",
    action: (
      <>
        commented on Jason Mark&apos;s <FeedLink>photo</FeedLink>.
      </>
    ),
    time: "2 mins ago",
    icon: "comments-post-icon",
  },
  {
    actor: "Jake Parker",
    action: (
      <>
        liked Nicholas Grissom&apos;s <FeedLink>status update</FeedLink>.
      </>
    ),
    time: "5 mins ago",
    icon: "like-post-icon",
  },
  {
    actor: "Mary Jane Stark",
    action: (
      <>
        added 20 new photos to her <FeedLink>gallery album</FeedLink>.
      </>
    ),
    time: "12 mins ago",
    icon: "photos-icon",
  },
  {
    actor: "Nicholas Grissom",
    action: (
      <>
        updated his profile <FeedLink>photo</FeedLink>.
      </>
    ),
    time: "1 hour ago",
    icon: "happy-face-icon",
  },
];

export function ActivityFeed({
  items = DEFAULT_ITEMS,
  title = "Activity Feed",
}: {
  items?: ActivityItem[];
  title?: string;
} = {}) {
  return (
    <WidgetCard title={title}>
      <ul className="space-y-4">
        {items.map((it, i) => (
          <li key={`${it.actor}-${i}`} className="flex items-start gap-3">
            <Avatar name={it.actor} size={36} />
            <div className="min-w-0 flex-1">
              <p className="text-sm leading-snug" style={{ color: "var(--tpl-text)" }}>
                <b style={{ color: "var(--tpl-heading)" }}>{it.actor}</b> {it.action}
              </p>
              <p className="mt-0.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
                {it.time}
              </p>
            </div>
            <span className="mt-1 shrink-0" style={{ color: "var(--tpl-muted)" }}>
              <Icon name={it.icon} size={16} />
            </span>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}

function FeedLink({ children }: { children: ReactNode }) {
  return (
    <a href="#" className="font-medium hover:underline" style={{ color: "var(--tpl-accent)" }}>
      {children}
    </a>
  );
}
