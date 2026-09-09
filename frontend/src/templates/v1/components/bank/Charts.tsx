"use client";

import { formatVND, shortMonthLabel, type BankCategoryTotal, type BankMonthFlow } from "@/lib/bank";
import { categoryTint } from "./CategoryChip";

/**
 * The two charts the month screen needs, as hand-written SVG.
 *
 * No charting library: a donut and a bar pair are a few dozen lines of maths,
 * and the alternatives (recharts, chart.js) are hundreds of kilobytes plus a
 * theming layer to make them match the template tokens. They would also have to
 * be loaded client-side on a page whose whole job is to render fast.
 */

const DONUT_SIZE = 168;
const DONUT_STROKE = 22;

/**
 * Spending by category.
 *
 * Slices below a threshold are folded into one "Khác" wedge rather than drawn:
 * a 0.4% slice is a sub-pixel arc that cannot be pointed at or read, and a dozen
 * of them turn the ring into visual noise. The folded total is still shown, so
 * nothing silently disappears from the chart.
 */
export function CategoryDonut({
  slices,
  total,
  centerLabel,
  minSharePct = 3,
}: {
  slices: BankCategoryTotal[];
  total: number;
  centerLabel: string;
  minSharePct?: number;
}) {
  const r = (DONUT_SIZE - DONUT_STROKE) / 2;
  const c = 2 * Math.PI * r;

  const big = slices.filter((s) => total > 0 && (s.total / total) * 100 >= minSharePct);
  const restTotal = slices.reduce((sum, s) => sum + s.total, 0) - big.reduce((sum, s) => sum + s.total, 0);
  const drawn: { key: string; value: number; color: string }[] = big.map((s) => ({
    key: s.category_id,
    value: s.total,
    color: categoryTint(s.color),
  }));
  if (restTotal > 0) drawn.push({ key: "__rest", value: restTotal, color: "#94a3b8" });

  if (total <= 0 || drawn.length === 0) {
    return (
      <div
        className="grid place-items-center rounded-full"
        style={{
          width: DONUT_SIZE,
          height: DONUT_SIZE,
          border: `${DONUT_STROKE}px solid var(--tpl-surface-2)`,
        }}
      >
        <span className="text-xs" style={{ color: "var(--tpl-muted)" }}>
          Chưa có dữ liệu
        </span>
      </div>
    );
  }

  let offset = 0;
  return (
    <div className="relative" style={{ width: DONUT_SIZE, height: DONUT_SIZE }}>
      <svg width={DONUT_SIZE} height={DONUT_SIZE} role="img" aria-label={`Tổng ${formatVND(total)}`}>
        {/* Rotated so the first slice starts at 12 o'clock, where a reader looks. */}
        <g transform={`rotate(-90 ${DONUT_SIZE / 2} ${DONUT_SIZE / 2})`}>
          {drawn.map((s) => {
            const len = (s.value / total) * c;
            const el = (
              <circle
                key={s.key}
                cx={DONUT_SIZE / 2}
                cy={DONUT_SIZE / 2}
                r={r}
                fill="none"
                stroke={s.color}
                strokeWidth={DONUT_STROKE}
                strokeDasharray={`${len} ${c - len}`}
                strokeDashoffset={-offset}
              />
            );
            offset += len;
            return el;
          })}
        </g>
      </svg>
      <div className="absolute inset-0 grid place-items-center">
        <div className="text-center">
          <p className="text-[10px] uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
            {centerLabel}
          </p>
          <p className="text-base font-bold tabular-nums" style={{ color: "var(--tpl-heading)" }}>
            {formatVND(total)}
          </p>
        </div>
      </div>
    </div>
  );
}

/**
 * Income vs expense over the trailing months.
 *
 * Both series share ONE scale — drawing each against its own maximum would make
 * a month that earned 20tr and spent 2tr look balanced, which is the exact
 * comparison the chart exists to make.
 */
export function TrendBars({ data, activeMonth }: { data: BankMonthFlow[]; activeMonth?: string }) {
  const max = Math.max(1, ...data.flatMap((d) => [d.income, d.expense]));

  return (
    <div className="flex items-end justify-between gap-2" style={{ height: 132 }}>
      {data.map((d) => {
        const active = d.month === activeMonth;
        return (
          <div key={d.month} className="flex flex-1 flex-col items-center gap-1.5">
            <div className="flex h-[96px] w-full items-end justify-center gap-1">
              <Bar value={d.income} max={max} color="#22c55e" label={`Thu ${formatVND(d.income)}`} />
              <Bar value={d.expense} max={max} color="#ef4444" label={`Chi ${formatVND(d.expense)}`} />
            </div>
            <span
              className="text-[10px] tabular-nums"
              style={{
                color: active ? "var(--tpl-accent)" : "var(--tpl-muted)",
                fontWeight: active ? 700 : 400,
              }}
            >
              {shortMonthLabel(d.month)}
            </span>
          </div>
        );
      })}
    </div>
  );
}

function Bar({ value, max, color, label }: { value: number; max: number; color: string; label: string }) {
  // A non-zero month always gets at least 2px, so "small" never renders as
  // "nothing" — an invisible bar and an empty month must not look alike.
  const h = value > 0 ? Math.max(2, Math.round((value / max) * 96)) : 0;
  return (
    <div
      className="w-2.5 rounded-t-sm"
      style={{ height: h, background: color, minHeight: h > 0 ? 2 : 0 }}
      title={label}
    />
  );
}
