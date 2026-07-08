import type { ReactNode } from "react";
import { WidgetCard } from "./WidgetCard";

/**
 * "Personal Info" rail widget — a `WidgetCard` listing label/value rows, modeled
 * on the Olympus profile `.w-personal-info` block (`Profile Page.html` ~3744).
 * Each row shows a muted label above its value. `items` is required in spirit but
 * defaults to a sample profile so the widget renders standalone.
 */

export type PersonalInfoItem = {
  /** Field label, e.g. "About" / "Lives in" / "Email". */
  label: string;
  /** Field value — free-form node so it can embed links. */
  value: ReactNode;
};

const DEFAULT_ITEMS: PersonalInfoItem[] = [
  {
    label: "About",
    value:
      "Hi, I'm James — a Digital Designer at the “Daydreams” Agency in Pier 56.",
  },
  { label: "Lives in", value: "San Francisco, CA" },
  { label: "Email", value: "james@example.com" },
  { label: "Website", value: "portal.example" },
];

export function PersonalInfoWidget({
  items = DEFAULT_ITEMS,
  title = "Personal Info",
}: {
  items?: PersonalInfoItem[];
  title?: string;
} = {}) {
  return (
    <WidgetCard title={title}>
      <ul className="space-y-3">
        {items.map((it, i) => (
          <li key={`${it.label}-${i}`}>
            <p
              className="text-[11px] font-semibold uppercase tracking-wide"
              style={{ color: "var(--tpl-muted)" }}
            >
              {it.label}
            </p>
            <p className="mt-0.5 text-sm leading-relaxed" style={{ color: "var(--tpl-text)" }}>
              {it.value}
            </p>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}
