"use client";

import Link from "next/link";
import type { Route } from "next";
import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { WidgetCard } from "./WidgetCard";
import { listEntries } from "@/lib/journal";
import { useTimeConfig, zonedYMD } from "@/lib/time";
import { solarToLunar } from "@/lib/lunar";

const WEEKDAYS = ["T2", "T3", "T4", "T5", "T6", "T7", "CN"];

/**
 * Month-calendar rail (âm dương) — the current month per the app time config
 * (server clock + APP_TIMEZONE), each day showing its solar + lunar date; days
 * with a journal entry get a dot. Links to the full /calendar page. Lunar dates
 * are computed for Vietnam (UTC+7) via lib/lunar, matching that page.
 */
export function CalendarWidget() {
  const { data: tc } = useTimeConfig();
  const { data } = useQuery({
    queryKey: ["journal", "calendar"],
    queryFn: () => listEntries(),
    retry: false,
  });

  const activeDays = useMemo(() => {
    const s = new Set<number>();
    if (!tc) return s;
    const cur = zonedYMD(tc.now, tc.timezone);
    for (const e of data?.items ?? []) {
      const p = zonedYMD(new Date(e.occurred_at), tc.timezone);
      if (p.year === cur.year && p.month === cur.month) s.add(p.day);
    }
    return s;
  }, [data, tc]);

  const lunarByDay = useMemo(() => {
    const map = new Map<number, { day: number; month: number }>();
    if (!tc) return map;
    const c = zonedYMD(tc.now, tc.timezone);
    const dim = new Date(Date.UTC(c.year, c.month + 1, 0)).getUTCDate();
    for (let d = 1; d <= dim; d += 1) {
      const l = solarToLunar(d, c.month + 1, c.year);
      map.set(d, { day: l.day, month: l.month });
    }
    return map;
  }, [tc]);

  if (!tc) return null; // until the time config loads

  const { year, month, day: today } = zonedYMD(tc.now, tc.timezone);
  const daysInMonth = new Date(Date.UTC(year, month + 1, 0)).getUTCDate();
  const firstDow = (new Date(Date.UTC(year, month, 1)).getUTCDay() + 6) % 7; // weekday of a date is tz-independent
  const cells: (number | null)[] = [
    ...Array.from({ length: firstDow }, () => null),
    ...Array.from({ length: daysInMonth }, (_, i) => i + 1),
  ];

  return (
    <Link href={"/calendar" as Route} className="block transition hover:opacity-90" title="Mở Lịch Âm Dương">
      <WidgetCard title={`Tháng ${month + 1}, ${year}`}>
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
            const isToday = day === today;
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
      </WidgetCard>
    </Link>
  );
}
