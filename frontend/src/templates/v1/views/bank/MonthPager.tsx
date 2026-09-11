"use client";

import { currentMonth, monthLabel, shiftMonth } from "@/lib/bank";

/**
 * Month stepper — ‹ Tháng 8, 2026 ›.
 *
 * A spending log is browsed one month at a time, so the two arrows carry the
 * whole navigation and a raw `<input type="month">` (what this used to be) is
 * the wrong control: it takes a click, a picker and a second click to do what
 * "last month" should take one tap for. The picker is still there behind the
 * label for jumping somewhere distant.
 *
 * Stepping forward past the current month is allowed — future-dated
 * transactions are legitimate (a scheduled bill, a post-dated transfer) and a
 * blocked arrow would hide them.
 */
export function MonthPager({ month, onChange }: { month: string; onChange: (m: string) => void }) {
  const isCurrent = month === currentMonth();

  return (
    <div
      className="flex items-center justify-between rounded-xl border px-2 py-1.5"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      <Arrow label="Tháng trước" onClick={() => onChange(shiftMonth(month, -1))}>
        ‹
      </Arrow>

      <div className="relative flex items-center gap-2">
        <span className="text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>
          {monthLabel(month)}
        </span>
        {!isCurrent && (
          <button
            type="button"
            onClick={() => onChange(currentMonth())}
            className="rounded-md px-2 py-0.5 text-[11px] font-semibold"
            style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-accent)" }}
          >
            Hôm nay
          </button>
        )}
        {/* Transparent overlay: keeps the native month picker available for a
            distant jump without giving it any visual weight. */}
        <input
          type="month"
          value={month}
          onChange={(e) => e.target.value && onChange(e.target.value)}
          aria-label="Chọn tháng"
          className="absolute inset-0 cursor-pointer opacity-0"
        />
      </div>

      <Arrow label="Tháng sau" onClick={() => onChange(shiftMonth(month, 1))}>
        ›
      </Arrow>
    </div>
  );
}

function Arrow({
  label,
  onClick,
  children,
}: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      className="grid h-8 w-8 place-items-center rounded-lg text-lg leading-none transition hover:bg-[var(--tpl-surface-2)]"
      style={{ color: "var(--tpl-muted)" }}
    >
      {children}
    </button>
  );
}
