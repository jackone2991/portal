import type { ReactNode } from "react";
import { Icon } from "../ui/Icon";

/**
 * Reusable rail-widget shell — extracted from the inline `Card` / `WidgetCard`
 * in `views/home/HomeView.tsx` (itself a port of the Olympus `.ui-block`).
 *
 * `Card` is the bare surface (rounded-xl, surface bg, 1px border, soft shadow)
 * with no padding or title bar of its own. `WidgetCard` is the padded variant
 * that adds the standard title bar + optional three-dots "more" affordance; all
 * the rail widgets compose it.
 */

/**
 * Bare card surface: rounded-xl `.ui-block` with a 1px border and soft shadow.
 * No inner padding — compose it directly, or use `WidgetCard` for the standard
 * padded rail-widget chrome.
 */
export function Card({
  children,
  className = "",
  as: Tag = "div",
}: {
  children: ReactNode;
  className?: string;
  as?: "div" | "article";
}) {
  return (
    <Tag
      className={`rounded-xl shadow-sm ${className}`}
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {children}
    </Tag>
  );
}

/**
 * Padded rail-widget shell: a `Card` with `p-4`, an optional `title` bar and an
 * optional `more` (three-dots) affordance. Wrap any rail content in `children`.
 */
export function WidgetCard({
  title,
  more,
  className = "",
  children,
}: {
  title?: string;
  more?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <Card className={`p-4 ${className}`}>
      {title && (
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>
            {title}
          </h3>
          {more && (
            <span style={{ color: "var(--tpl-muted)" }}>
              <Icon name="three-dots-icon" size={12} />
            </span>
          )}
        </div>
      )}
      {children}
    </Card>
  );
}
