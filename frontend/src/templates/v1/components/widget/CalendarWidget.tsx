"use client";

import Link from "next/link";
import type { Route } from "next";
import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Card } from "./WidgetCard";
import { listEntries } from "@/lib/journal";
import { useTimeConfig, zonedYMD } from "@/lib/time";
import { solarToLunar } from "@/lib/lunar";

const WEEKDAYS = ["T2", "T3", "T4", "T5", "T6", "T7", "CN"];

/**
 * Month-calendar rail (âm dương). Opens on the current month per the app time
 * config (server clock + APP_TIMEZONE) and lets you page through other months and
 * years in place — prev/next month (‹ ›) and prev/next year (« ») — instead of
 * navigating away. Each day shows its solar + lunar date; days with a journal
 * entry get a dot. A footer link still opens the full /calendar page. Lunar dates
 * are computed for Vietnam (UTC+7) via lib/lunar, matching that page.
 */
export function CalendarWidget() {
  const { data: tc } = useTimeConfig();
  const { data } = useQuery({
    queryKey: ["journal", "calendar"],
    queryFn: () => listEntries(),
    retry: false,
  });

  // The month currently being viewed; initialised to "now" once the time config loads.
  const [ym, setYm] = useState<{ year: number; month: number } | null>(null);
  useEffect(() => {
    if (tc && !ym) {
      const c = zonedYMD(tc.now, tc.timezone);
      setYm({ year: c.year, month: c.month });
    }
  }, [tc, ym]);

  const activeDays = useMemo(() => {
    const s = new Set<number>();
    if (!tc || !ym) return s;
    for (const e of data?.items ?? []) {
      const p = zonedYMD(new Date(e.occurred_at), tc.timezone);
      if (p.year === ym.year && p.month === ym.month) s.add(p.day);
    }
    return s;
  }, [data, tc, ym]);

  const lunarByDay = useMemo(() => {
    const map = new Map<number, { day: number; month: number }>();
    if (!ym) return map;
    const dim = new Date(Date.UTC(ym.year, ym.month + 1, 0)).getUTCDate();
    for (let d = 1; d <= dim; d += 1) {
      const l = solarToLunar(d, ym.month + 1, ym.year);
      map.set(d, { day: l.day, month: l.month });
    }
    return map;
  }, [ym]);

  if (!tc || !ym) return null; // until the time config loads

  const { year, month } = ym;
  const now = zonedYMD(tc.now, tc.timezone);
  const isThisMonth = now.year === year && now.month === month;
  const daysInMonth = new Date(Date.UTC(year, month + 1, 0)).getUTCDate();
  const firstDow = (new Date(Date.UTC(year, month, 1)).getUTCDay() + 6) % 7; // weekday of a date is tz-independent
  const cells: (number | null)[] = [
    ...Array.from({ length: firstDow }, () => null),
    ...Array.from({ length: daysInMonth }, (_, i) => i + 1),
  ];

  const shiftMonth = (delta: number) =>
    setYm((m) => {
      if (!m) return m;
      const d = new Date(Date.UTC(m.year, m.month + delta, 1));
      return { year: d.getUTCFullYear(), month: d.getUTCMonth() };
    });
  const goToday = () => setYm({ year: now.year, month: now.month });

  return (
    <Card className="p-4">
      <div className="mb-3 flex items-center justify-between gap-1">
        <div className="flex items-center gap-0.5">
          <NavBtn label="Năm trước" glyph="«" onClick={() => shiftMonth(-12)} />
          <NavBtn label="Tháng trước" glyph="‹" onClick={() => shiftMonth(-1)} />
        </div>
        <button
          type="button"
          onClick={goToday}
          title={isThisMonth ? "Tháng này" : "Về hôm nay"}
          className="rounded-md px-1.5 py-0.5 text-sm font-bold transition hover:bg-[var(--tpl-surface-2)]"
          style={{ color: "var(--tpl-heading)" }}
        >
          Tháng {month + 1}, {year}
        </button>
        <div className="flex items-center gap-0.5">
          <NavBtn label="Tháng sau" glyph="›" onClick={() => shiftMonth(1)} />
          <NavBtn label="Năm sau" glyph="»" onClick={() => shiftMonth(12)} />
        </div>
      </div>

      <div className="grid grid-cols-7 gap-y-1.5 text-center">
        {WEEKDAYS.map((d, i) => (
          <div key={d} className="text-[10px] font-bold" style={{ color: i === 6 ? "var(--tpl-accent)" : "var(--tpl-muted)" }}>
            {d}
          </div>
        ))}
        {cells.map((day, i) => {
          if (day === null) return <span key={`b${i}`} />;
          const lunar = lunarByDay.get(day);
          const isMonthStart = lunar?.day === 1;
          const isToday = isThisMonth && day === now.day;
          return (
            <div key={day} className="flex flex-col items-center leading-none">
              <span
                className="relative grid h-5 w-5 place-items-center rounded-full text-[11px]"
                style={isToday ? { background: "var(--tpl-accent)", color: "#fff", fontWeight: 600 } : { color: "var(--tpl-heading)" }}
              >
                {day}
                {activeDays.has(day) && !isToday && (
                  <span className="absolute -right-0.5 -top-0.5 h-1 w-1 rounded-full" style={{ background: "var(--tpl-accent)" }} aria-label="có ghi chú" />
                )}
              </span>
              <span
                className="mt-0.5 text-[8px]"
                style={{ color: isMonthStart ? "var(--tpl-accent)" : "var(--tpl-muted)", fontWeight: isMonthStart ? 700 : 400 }}
              >
                {isMonthStart ? `1/${lunar?.month}` : lunar?.day}
              </span>
            </div>
          );
        })}
      </div>

      <div className="mt-3 border-t pt-2 text-center" style={{ borderColor: "var(--tpl-border)" }}>
        <Link
          href={"/calendar" as Route}
          className="text-[11px] font-semibold transition hover:opacity-80"
          style={{ color: "var(--tpl-accent)" }}
        >
          Mở Lịch Âm Dương →
        </Link>
      </div>
    </Card>
  );
}

function NavBtn({ label, glyph, onClick }: { label: string; glyph: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="grid h-6 w-6 place-items-center rounded-md text-sm leading-none transition hover:bg-[var(--tpl-surface-2)]"
      style={{ color: "var(--tpl-muted)" }}
    >
      {glyph}
    </button>
  );
}
