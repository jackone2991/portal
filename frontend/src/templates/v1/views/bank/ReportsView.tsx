"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { currentMonth, formatVND, getReport, type BankCategoryTotal } from "@/lib/bank";
import { BackLink } from "../../components/bank/BackLink";
import { CategoryChip, categoryTint } from "../../components/bank/CategoryChip";
import { CategoryDonut, TrendBars } from "../../components/bank/Charts";
import { MonthPager } from "./MonthPager";

/**
 * The month in detail: a donut per direction, the full category breakdown, and
 * the trailing six months.
 *
 * Separate from the dashboard on purpose — the dashboard answers "what is my
 * situation" in one screen, this answers "where exactly did it go" and needs
 * room for every category rather than a top-six. Both read the same
 * `/bank/report` aggregate, so the numbers cannot disagree.
 */
export function ReportsView() {
  const [month, setMonth] = useState(currentMonth());
  const [kind, setKind] = useState<"expense" | "income">("expense");

  const report = useQuery({ queryKey: ["bank", "report", month], queryFn: () => getReport(month) });

  const slices = kind === "expense" ? report.data?.expenses ?? [] : report.data?.incomes ?? [];
  const total = kind === "expense" ? report.data?.expense ?? 0 : report.data?.income ?? 0;
  const net = (report.data?.income ?? 0) - (report.data?.expense ?? 0);

  return (
    <section className="pb-8">
      <header className="mb-5">
        <BackLink />
        <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
          Báo cáo
        </h1>
      </header>

      <MonthPager month={month} onChange={setMonth} />

      <div
        className="mt-4 grid grid-cols-3 overflow-hidden rounded-2xl border"
        style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
      >
        <Total label="Thu" value={report.data?.income ?? 0} tone="#22c55e" />
        <Total label="Chi" value={report.data?.expense ?? 0} tone="#ef4444" bordered />
        <Total label="Còn lại" value={net} tone={net < 0 ? "#ef4444" : "var(--tpl-heading)"} signed />
      </div>

      <div className="mt-4 flex gap-1 rounded-lg p-1" style={{ background: "var(--tpl-surface-2)", width: "fit-content" }}>
        {(
          [
            ["expense", "Chi tiêu"],
            ["income", "Thu nhập"],
          ] as const
        ).map(([k, label]) => (
          <button
            key={k}
            type="button"
            onClick={() => setKind(k)}
            aria-pressed={kind === k}
            className="rounded-md px-4 py-1.5 text-sm font-semibold transition"
            style={{
              background: kind === k ? "var(--tpl-accent)" : "transparent",
              color: kind === k ? "#fff" : "var(--tpl-muted)",
            }}
          >
            {label}
          </button>
        ))}
      </div>

      <div
        className="mt-3 rounded-2xl border p-5"
        style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
      >
        {report.isPending ? (
          <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
            Đang tải…
          </p>
        ) : slices.length === 0 ? (
          <p className="py-8 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
            Tháng này chưa có {kind === "expense" ? "khoản chi" : "khoản thu"} nào.
          </p>
        ) : (
          <div className="flex flex-col items-center gap-6 md:flex-row md:items-start">
            <CategoryDonut
              slices={slices}
              total={total}
              centerLabel={kind === "expense" ? "Đã chi" : "Đã thu"}
            />
            <ul className="w-full min-w-0 flex-1 divide-y" style={{ borderColor: "var(--tpl-border)" }}>
              {slices.map((c) => (
                <BreakdownRow key={c.category_id} slice={c} total={total} />
              ))}
            </ul>
          </div>
        )}
      </div>

      {(report.data?.trend ?? []).length > 0 && (
        <div
          className="mt-4 rounded-2xl border px-5 py-4"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          <h2 className="mb-3 text-sm font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
            6 tháng gần đây
          </h2>
          <TrendBars data={report.data?.trend ?? []} activeMonth={month} />
        </div>
      )}
    </section>
  );
}

function Total({
  label,
  value,
  tone,
  bordered,
  signed,
}: {
  label: string;
  value: number;
  tone: string;
  bordered?: boolean;
  signed?: boolean;
}) {
  return (
    <div
      className="px-5 py-4"
      style={bordered ? { borderLeft: "1px solid var(--tpl-border)", borderRight: "1px solid var(--tpl-border)" } : undefined}
    >
      <p className="text-[11px] font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
        {label}
      </p>
      <p className="mt-0.5 text-lg font-bold tabular-nums" style={{ color: tone }}>
        {signed && value > 0 ? "+" : ""}
        {formatVND(value)}
      </p>
    </div>
  );
}

function BreakdownRow({ slice, total }: { slice: BankCategoryTotal; total: number }) {
  const pct = total > 0 ? (slice.total / total) * 100 : 0;
  return (
    <li className="flex items-center gap-3 py-2.5" style={{ borderColor: "var(--tpl-border)" }}>
      <CategoryChip icon={slice.icon} color={slice.color} name={slice.name} size={34} />
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-2">
          <span className="truncate text-sm font-medium" style={{ color: "var(--tpl-text)" }}>
            {slice.name}
          </span>
          <span className="shrink-0 text-sm font-semibold tabular-nums" style={{ color: "var(--tpl-heading)" }}>
            {formatVND(slice.total)}
          </span>
        </div>
        <div className="mt-1 flex items-center gap-2">
          <div className="h-1.5 flex-1 overflow-hidden rounded-full" style={{ background: "var(--tpl-surface-2)" }}>
            <div
              className="h-full rounded-full"
              style={{ width: `${Math.max(2, pct)}%`, background: categoryTint(slice.color) }}
            />
          </div>
          <span className="w-20 shrink-0 text-right text-[11px] tabular-nums" style={{ color: "var(--tpl-muted)" }}>
            {pct.toFixed(1)}% · {slice.tx_count} GD
          </span>
        </div>
      </div>
    </li>
  );
}
