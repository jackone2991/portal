import { Icon } from "../ui/Icon";
import { WidgetCard } from "./WidgetCard";

/**
 * Month-calendar rail widget — extracted from the inline `CalendarWidget` in
 * `views/home/HomeView.tsx`. A weekday header + day grid with the accent-filled
 * "today" pill. Prev/next month arrows are presentational; wire `onPrev`/`onNext`
 * if you need navigation. All props optional (defaults to the Olympus sample).
 */

const DEFAULT_WEEKDAYS = ["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"];

export function CalendarWidget({
  month = "May",
  today = 12,
  daysInMonth = 31,
  weekdays = DEFAULT_WEEKDAYS,
  onPrev,
  onNext,
}: {
  month?: string;
  today?: number;
  daysInMonth?: number;
  weekdays?: string[];
  onPrev?: () => void;
  onNext?: () => void;
} = {}) {
  const days = Array.from({ length: daysInMonth }, (_, i) => i + 1);
  return (
    <WidgetCard>
      <div
        className="mb-3 flex items-center justify-between px-1"
        style={{ color: "var(--tpl-muted)" }}
      >
        <button type="button" aria-label="Previous month" onClick={onPrev}>
          <Icon name="popup-left-arrow" size={12} />
        </button>
        <span className="font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {month}
        </span>
        <button type="button" aria-label="Next month" onClick={onNext}>
          <Icon name="popup-right-arrow" size={12} />
        </button>
      </div>
      <div className="grid grid-cols-7 gap-y-2 text-center text-[11px]">
        {weekdays.map((d) => (
          <span key={d} className="font-semibold" style={{ color: "var(--tpl-muted)" }}>
            {d}
          </span>
        ))}
        {days.map((n) => (
          <span
            key={n}
            className="grid h-7 place-items-center rounded-full text-xs"
            style={
              n === today
                ? { background: "var(--tpl-accent)", color: "#fff", margin: "0 auto", width: 28 }
                : { color: "var(--tpl-text)" }
            }
          >
            {n}
          </span>
        ))}
      </div>
    </WidgetCard>
  );
}
