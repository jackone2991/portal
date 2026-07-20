"use client";

import { useEffect, useMemo, useState } from "react";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import { createEntry, listEntries, type CreateEntryInput, type JournalEntry } from "@/lib/journal";
import { fromDatetimeLocalInTz, useTimeConfig, zonedYMD } from "@/lib/time";
import { canChiDay, canChiYear, solarToLunar, type LunarDate } from "@/lib/lunar";

const WEEKDAYS = ["T2", "T3", "T4", "T5", "T6", "T7", "CN"];
const WEEKDAY_FULL = ["Chủ Nhật", "Thứ Hai", "Thứ Ba", "Thứ Tư", "Thứ Năm", "Thứ Sáu", "Thứ Bảy"];
const MAX_PAGES = 12;

/**
 * Lịch Âm Dương (Vietnamese lunar-solar calendar) at `/calendar`. Each day shows
 * the solar date + its lunar date (lib/lunar, Hồ Ngọc Đức). Notes are journal
 * entries on their `occurred_at` day: pick a day, write a note. Solar dates use the
 * app time config (lib/time); lunar dates are computed for Vietnam (UTC+7).
 */
export function CalendarView() {
  const { data: tc } = useTimeConfig();
  const qc = useQueryClient();

  const [ym, setYm] = useState<{ year: number; month: number } | null>(null);
  const [selectedDay, setSelectedDay] = useState<number | null>(null);
  const [noteTime, setNoteTime] = useState("12:00");
  const [bodyMd, setBodyMd] = useState("");
  const [mood, setMood] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  const q = useInfiniteQuery({
    queryKey: ["journal", "all"],
    queryFn: ({ pageParam }: { pageParam: string | undefined }) => listEntries({ cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const entries = useMemo(() => q.data?.pages.flatMap((p) => p.items) ?? [], [q.data]);

  useEffect(() => {
    if (tc && !ym) {
      const c = zonedYMD(tc.now, tc.timezone);
      setYm({ year: c.year, month: c.month });
    }
  }, [tc, ym]);

  useEffect(() => {
    if (!ym || !q.hasNextPage || q.isFetchingNextPage) return;
    const oldest = entries[entries.length - 1];
    const monthStart = Date.UTC(ym.year, ym.month, 1);
    if ((!oldest || new Date(oldest.occurred_at).getTime() >= monthStart) && (q.data?.pages.length ?? 0) < MAX_PAGES) {
      q.fetchNextPage();
    }
  }, [entries, ym, q]);

  // Solar day → its lunar date, for the displayed month (memoised: 28–31 calls).
  const lunarByDay = useMemo(() => {
    const map = new Map<number, LunarDate>();
    if (!ym) return map;
    const dim = new Date(Date.UTC(ym.year, ym.month + 1, 0)).getUTCDate();
    for (let d = 1; d <= dim; d += 1) map.set(d, solarToLunar(d, ym.month + 1, ym.year));
    return map;
  }, [ym]);

  // Solar day → its journal entries, for the displayed month + configured tz.
  const byDay = useMemo(() => {
    const map = new Map<number, JournalEntry[]>();
    if (!tc || !ym) return map;
    for (const e of entries) {
      const p = zonedYMD(new Date(e.occurred_at), tc.timezone);
      if (p.year === ym.year && p.month === ym.month) {
        const list = map.get(p.day);
        if (list) list.push(e);
        else map.set(p.day, [e]);
      }
    }
    return map;
  }, [entries, tc, ym]);

  const create = useMutation({
    mutationFn: (input: CreateEntryInput) => createEntry(input),
    onSuccess: () => {
      setBodyMd("");
      setMood("");
      setNoteTime("12:00");
      setFormError(null);
      qc.invalidateQueries({ queryKey: ["journal"] });
      qc.invalidateQueries({ queryKey: ["stream"] });
    },
    onError: (err) => setFormError(err instanceof ApiError ? problemDisplayMessage(err.body) : "Không lưu được ghi chú"),
  });

  if (!tc || !ym) return <p style={{ color: "var(--tpl-muted)" }}>Đang tải lịch…</p>;

  const { year, month } = ym;
  const cfgTz = tc.timezone;
  const today = zonedYMD(tc.now, cfgTz);
  const isThisMonth = today.year === year && today.month === month;
  const daysInMonth = new Date(Date.UTC(year, month + 1, 0)).getUTCDate();
  const firstDow = (new Date(Date.UTC(year, month, 1)).getUTCDay() + 6) % 7;
  const cells: (number | null)[] = [
    ...Array.from({ length: firstDow }, () => null),
    ...Array.from({ length: daysInMonth }, (_, i) => i + 1),
  ];
  const yearLunar = lunarByDay.get(1)?.year ?? year;

  function shiftMonth(delta: number) {
    setSelectedDay(null);
    setYm((m) => {
      if (!m) return m;
      const d = new Date(Date.UTC(m.year, m.month + delta, 1));
      return { year: d.getUTCFullYear(), month: d.getUTCMonth() };
    });
  }

  function selectDay(day: number) {
    setSelectedDay(day);
    setNoteTime("12:00");
    setBodyMd("");
    setMood("");
    setFormError(null);
  }

  function handleCreate() {
    const body = bodyMd.trim();
    if (!body || create.isPending || selectedDay == null) return;
    const mm = String(month + 1).padStart(2, "0");
    const dd = String(selectedDay).padStart(2, "0");
    const input: CreateEntryInput = {
      body_md: body,
      occurred_at: fromDatetimeLocalInTz(`${year}-${mm}-${dd}T${noteTime}`, cfgTz),
    };
    const m = mood.trim();
    if (m) input.mood = m;
    create.mutate(input);
  }

  const timeLabel = (iso: string) =>
    new Intl.DateTimeFormat("en-GB", { timeZone: cfgTz, hour: "2-digit", minute: "2-digit" }).format(new Date(iso));

  const selLunar = selectedDay != null ? lunarByDay.get(selectedDay) : null;
  const selEntries = selectedDay != null ? byDay.get(selectedDay) ?? [] : [];
  const selWeekday = selectedDay != null ? WEEKDAY_FULL[new Date(Date.UTC(year, month, selectedDay)).getUTCDay()] : "";

  return (
    <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_360px]">
      {/* ── Lịch tháng ─────────────────────────────────────────────── */}
      <div className="rounded-2xl border p-5 shadow-sm" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
        <header className="mb-4 flex items-center justify-between">
          <div>
            <h1 className="text-lg font-bold" style={{ color: "var(--tpl-heading)" }}>
              Tháng {month + 1}, {year}
            </h1>
            <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>Âm lịch · năm {canChiYear(yearLunar)}</p>
          </div>
          <div className="flex items-center gap-1">
            <NavBtn label="Tháng trước" glyph="‹" onClick={() => shiftMonth(-1)} />
            <button
              type="button"
              onClick={() => { const c = zonedYMD(tc.now, cfgTz); setSelectedDay(null); setYm({ year: c.year, month: c.month }); }}
              className="rounded-lg border px-3 py-1.5 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
            >
              Hôm nay
            </button>
            <NavBtn label="Tháng sau" glyph="›" onClick={() => shiftMonth(1)} />
          </div>
        </header>

        <div className="mb-2 grid grid-cols-7 gap-1.5 text-center text-[11px] font-bold" style={{ color: "var(--tpl-muted)" }}>
          {WEEKDAYS.map((d, i) => (
            <div key={d} style={i === 6 ? { color: "var(--tpl-accent)" } : undefined}>{d}</div>
          ))}
        </div>
        <div className="grid grid-cols-7 gap-1.5">
          {cells.map((day, i) => {
            if (day === null) return <span key={`b${i}`} />;
            const lunar = lunarByDay.get(day);
            const hasNotes = (byDay.get(day)?.length ?? 0) > 0;
            const isToday = isThisMonth && day === today.day;
            const isSelected = day === selectedDay;
            const isMonthStart = lunar?.day === 1;
            return (
              <button
                key={day}
                type="button"
                onClick={() => selectDay(day)}
                className="relative flex aspect-square flex-col items-center justify-center rounded-xl border transition hover:border-[var(--tpl-accent)]"
                style={{
                  borderColor: isSelected ? "var(--tpl-accent)" : "var(--tpl-border)",
                  background: isSelected ? "var(--tpl-surface-2)" : isToday ? "color-mix(in srgb, var(--tpl-accent) 12%, transparent)" : "transparent",
                }}
              >
                {hasNotes && (
                  <span className="absolute right-1.5 top-1.5 h-1.5 w-1.5 rounded-full" style={{ background: "var(--tpl-accent)" }} />
                )}
                <span className="text-base font-semibold leading-none" style={{ color: isToday ? "var(--tpl-accent)" : "var(--tpl-heading)" }}>
                  {day}
                </span>
                <span
                  className="mt-1 text-[10px] leading-none"
                  style={{ color: isMonthStart ? "var(--tpl-accent)" : "var(--tpl-muted)", fontWeight: isMonthStart ? 700 : 400 }}
                >
                  {isMonthStart ? `1/${lunar?.month}` : lunar?.day}
                </span>
              </button>
            );
          })}
        </div>
      </div>

      {/* ── Ngày đã chọn: âm lịch + ghi chú ────────────────────────── */}
      <aside className="space-y-4">
        {selectedDay == null || !selLunar ? (
          <div className="rounded-2xl border p-6 text-center text-sm" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)", color: "var(--tpl-muted)" }}>
            Chọn một ngày để xem âm lịch và thêm ghi chú.
          </div>
        ) : (
          <>
            {/* Day header — âm dương */}
            <div className="overflow-hidden rounded-2xl border shadow-sm" style={{ borderColor: "var(--tpl-border)" }}>
              <div className="p-4 text-white" style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-blue))" }}>
                <p className="text-xs text-white/80">{selWeekday}</p>
                <p className="text-2xl font-bold leading-tight">{selectedDay} tháng {month + 1}, {year}</p>
                <p className="mt-1 text-sm text-white/90">
                  Âm lịch: <b>{selLunar.day}/{selLunar.month}{selLunar.leap ? " (nhuận)" : ""}</b> · năm {canChiYear(selLunar.year)}
                </p>
                <p className="text-xs text-white/80">Ngày {canChiDay(selectedDay, month + 1, year)}</p>
              </div>
            </div>

            {/* Note form — redesigned */}
            <form
              onSubmit={(e) => { e.preventDefault(); handleCreate(); }}
              className="rounded-2xl border p-4 shadow-sm"
              style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
            >
              <label className="mb-2 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>Ghi chú mới</label>
              <textarea
                value={bodyMd}
                onChange={(e) => setBodyMd(e.target.value)}
                rows={3}
                placeholder="Hôm nay có gì đáng nhớ?"
                className="w-full resize-none rounded-xl border bg-transparent p-3 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
                style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
              />
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <label className="flex items-center gap-2 rounded-lg border px-3 py-2 text-sm" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }} title="Giờ">
                  <span aria-hidden>🕑</span>
                  <input
                    type="time"
                    value={noteTime}
                    onChange={(e) => setNoteTime(e.target.value)}
                    aria-label="Giờ"
                    className="bg-transparent text-sm outline-none"
                    style={{ color: "var(--tpl-text)" }}
                  />
                </label>
                <input
                  type="text"
                  value={mood}
                  onChange={(e) => setMood(e.target.value)}
                  maxLength={80}
                  placeholder="Tâm trạng (tuỳ chọn)"
                  aria-label="Tâm trạng"
                  className="min-w-0 flex-1 rounded-lg border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
                  style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
                />
              </div>
              {formError && (
                <p role="alert" className="mt-3 rounded-lg px-3 py-2 text-sm" style={{ background: "rgba(239,68,68,.08)", color: "#ef4444" }}>
                  {formError}
                </p>
              )}
              <div className="mt-3 flex justify-end">
                <button
                  type="submit"
                  disabled={!bodyMd.trim() || create.isPending}
                  className="rounded-xl px-5 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
                  style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
                >
                  {create.isPending ? "Đang lưu…" : "Lưu ghi chú"}
                </button>
              </div>
            </form>

            {/* Notes of the day */}
            {selEntries.length === 0 ? (
              <p className="px-1 text-xs" style={{ color: "var(--tpl-muted)" }}>Chưa có ghi chú cho ngày này.</p>
            ) : (
              selEntries.map((e) => (
                <article key={e.id} className="rounded-2xl border p-4 shadow-sm" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
                  <div className="mb-1 flex items-center gap-2 text-xs" style={{ color: "var(--tpl-muted)" }}>
                    <span className="rounded-md px-1.5 py-0.5 font-semibold" style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-accent)" }}>{timeLabel(e.occurred_at)}</span>
                    {e.mood ? <span>· {e.mood}</span> : null}
                  </div>
                  <p className="whitespace-pre-wrap text-sm" style={{ color: "var(--tpl-heading)" }}>{e.body_md}</p>
                </article>
              ))
            )}
          </>
        )}
      </aside>
    </div>
  );
}

function NavBtn({ label, glyph, onClick }: { label: string; glyph: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className="grid h-8 w-8 place-items-center rounded-lg border text-lg leading-none transition hover:bg-[var(--tpl-surface-2)]"
      style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
    >
      {glyph}
    </button>
  );
}
