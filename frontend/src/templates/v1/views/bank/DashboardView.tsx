"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import {
  currentMonth,
  dayLabel,
  formatVND,
  getDashboard,
  getReport,
  listCategories,
  listTransactions,
  monthLabel,
  shiftMonth,
  signedAmount,
  type BankAccount,
  type BankTransaction,
} from "@/lib/bank";
import { Icon } from "../../components/ui/Icon";
import { CategoryChip, categoryTint } from "../../components/bank/CategoryChip";
import { CategoryDonut, TrendBars } from "../../components/bank/Charts";
import { QuickAddModal } from "../../components/bank/QuickAddModal";
import { MonthPager } from "./MonthPager";

/**
 * Ledger home.
 *
 * Reads top-to-bottom as the three questions a spending log is opened to answer,
 * in that order: how much do I have, where did this month go, and what did I
 * just spend. Everything else (wallet admin, budgets, the full history) is a
 * link out — putting them here would push the month summary below the fold,
 * which is the one thing worth seeing without scrolling.
 *
 * Money is integer minor units on the wire (D-41); `formatVND` owns the display
 * string.
 */
export function DashboardView() {
  const [month, setMonth] = useState(currentMonth());
  const [adding, setAdding] = useState(false);

  const dash = useQuery({ queryKey: ["bank", "dashboard", month], queryFn: () => getDashboard(month) });
  const report = useQuery({ queryKey: ["bank", "report", month], queryFn: () => getReport(month) });
  const cats = useQuery({ queryKey: ["bank", "categories"], queryFn: listCategories });
  // Scoped to the SELECTED month, not `dashboard.recent`.
  //
  // The dashboard's `recent` is the last N transactions overall, which under a
  // month pager is actively misleading: browsing to August and being shown rows
  // from April reads as August data, and the day headings ("Chủ nhật, 12/07")
  // are the only thing contradicting it. What the screen means by "gần đây" is
  // "latest in the month you are looking at".
  const recent = useQuery({
    queryKey: ["bank", "transactions", month, "", ""],
    queryFn: () => listTransactions({ month }),
  });

  const catById = useMemo(
    () => new Map((cats.data ?? []).map((c) => [c.id, c] as const)),
    [cats.data],
  );
  const accountById = useMemo(
    () => new Map((dash.data?.accounts ?? []).map((a) => [a.id, a] as const)),
    [dash.data],
  );

  const openAccounts = (dash.data?.accounts ?? []).filter((a) => !a.archived);
  const net = (dash.data?.income ?? 0) - (dash.data?.expense ?? 0);

  // Budget bars: only categories with no budgeted ancestor, so a parent and its
  // child do not both draw a bar for overlapping money.
  const budgetSet = useMemo(() => new Set(dash.data?.budgets.map((b) => b.category_id)), [dash.data]);
  const topBudgets = (dash.data?.budgets ?? []).filter((b) => !(b.parent_id && budgetSet.has(b.parent_id)));

  return (
    <section className="pb-8">
      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
          Sổ thu chi
        </h1>
        <div className="flex items-center gap-2">
          <Link
            href={"/bank/reports" as Route}
            className="rounded-lg border px-3 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
          >
            Báo cáo
          </Link>
          <Link
            href={"/bank/accounts" as Route}
            className="rounded-lg border px-3 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
          >
            Ví
          </Link>
          <button
            type="button"
            onClick={() => setAdding(true)}
            className="flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-bold text-white transition hover:opacity-90"
            style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
          >
            + Ghi chép
          </button>
        </div>
      </header>

      <MonthPager month={month} onChange={setMonth} />

      {/* ── total balance + the month in three numbers ────────────────── */}
      <section
        className="mt-4 overflow-hidden rounded-2xl border"
        style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
      >
        <div className="p-5">
          <p className="text-xs font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
            Tổng số dư
          </p>
          <p className="mt-1 text-3xl font-bold tabular-nums" style={{ color: "var(--tpl-heading)" }}>
            {formatVND(openAccounts.reduce((s, a) => s + a.balance, 0))}
            <span className="ml-1.5 text-base font-semibold" style={{ color: "var(--tpl-muted)" }}>
              đ
            </span>
          </p>
        </div>

        <div className="grid grid-cols-3 border-t" style={{ borderColor: "var(--tpl-border)" }}>
          <Stat label="Thu" value={dash.data?.income ?? 0} tone="#22c55e" />
          <Stat label="Chi" value={dash.data?.expense ?? 0} tone="#ef4444" bordered />
          <Stat label="Còn lại" value={net} tone={net < 0 ? "#ef4444" : "var(--tpl-heading)"} signed />
        </div>
      </section>

      {/* ── wallets ───────────────────────────────────────────────────── */}
      {openAccounts.length > 0 ? (
        <section className="mt-4">
          <SectionHead title="Ví của tôi" href="/bank/accounts" />
          <div className="grid gap-2 sm:grid-cols-2">
            {openAccounts.map((a) => (
              <WalletCard key={a.id} account={a} />
            ))}
          </div>
        </section>
      ) : (
        !dash.isPending && (
          <Link
            href={"/bank/accounts" as Route}
            className="mt-4 block rounded-2xl border border-dashed p-8 text-center text-sm transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
          >
            Chưa có ví nào — tạo ví đầu tiên để bắt đầu ghi chép.
          </Link>
        )
      )}

      {/* ── where the month went ──────────────────────────────────────── */}
      <section className="mt-6">
        <SectionHead title="Chi tiêu theo danh mục" href="/bank/reports" />
        <div
          className="rounded-2xl border p-5"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          <div className="flex flex-col items-center gap-6 sm:flex-row sm:items-start">
            <CategoryDonut
              slices={report.data?.expenses ?? []}
              total={report.data?.expense ?? 0}
              centerLabel="Đã chi"
            />
            <ul className="w-full min-w-0 flex-1 space-y-2">
              {(report.data?.expenses ?? []).slice(0, 6).map((c) => {
                const pct = report.data?.expense ? (c.total / report.data.expense) * 100 : 0;
                return (
                  <li key={c.category_id} className="flex items-center gap-3">
                    <CategoryChip icon={c.icon} color={c.color} name={c.name} size={30} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-baseline justify-between gap-2">
                        <span className="truncate text-sm" style={{ color: "var(--tpl-text)" }}>
                          {c.name}
                        </span>
                        <span className="shrink-0 text-sm font-semibold tabular-nums" style={{ color: "var(--tpl-heading)" }}>
                          {formatVND(c.total)}
                        </span>
                      </div>
                      <div className="mt-1 h-1.5 overflow-hidden rounded-full" style={{ background: "var(--tpl-surface-2)" }}>
                        <div
                          className="h-full rounded-full"
                          style={{ width: `${Math.max(2, pct)}%`, background: categoryTint(c.color) }}
                        />
                      </div>
                    </div>
                    <span className="w-10 shrink-0 text-right text-xs tabular-nums" style={{ color: "var(--tpl-muted)" }}>
                      {pct.toFixed(0)}%
                    </span>
                  </li>
                );
              })}
              {(report.data?.expenses ?? []).length === 0 && !report.isPending && (
                <li className="text-sm" style={{ color: "var(--tpl-muted)" }}>
                  Tháng này chưa có khoản chi nào.
                </li>
              )}
            </ul>
          </div>
        </div>
      </section>

      {/* ── budgets ───────────────────────────────────────────────────── */}
      {topBudgets.length > 0 && (
        <section className="mt-6">
          <SectionHead title="Ngân sách" href="/bank/budgets" />
          <ul
            className="divide-y rounded-2xl border"
            style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
          >
            {topBudgets.map((b) => {
              const cat = catById.get(b.category_id);
              const pct = b.amount > 0 ? Math.min(100, (b.spent / b.amount) * 100) : 0;
              const over = b.spent > b.amount;
              return (
                <li key={b.category_id} className="flex items-center gap-3 px-4 py-3" style={{ borderColor: "var(--tpl-border)" }}>
                  <CategoryChip icon={cat?.icon} color={cat?.color} name={b.name} size={34} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-baseline justify-between gap-2">
                      <span className="truncate text-sm font-medium" style={{ color: "var(--tpl-text)" }}>
                        {b.name}
                      </span>
                      <span className="shrink-0 text-xs tabular-nums" style={{ color: over ? "#ef4444" : "var(--tpl-muted)" }}>
                        {formatVND(b.spent)} / {formatVND(b.amount)}
                      </span>
                    </div>
                    <div className="mt-1.5 h-2 overflow-hidden rounded-full" style={{ background: "var(--tpl-surface-2)" }}>
                      <div
                        className="h-full rounded-full transition-all"
                        style={{
                          width: `${Math.max(2, pct)}%`,
                          // Over budget turns red rather than just filling the bar:
                          // a full bar and an exceeded one must not look identical.
                          background: over ? "#ef4444" : pct > 80 ? "#f59e0b" : "#22c55e",
                        }}
                      />
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
        </section>
      )}

      {/* ── trend ─────────────────────────────────────────────────────── */}
      {(report.data?.trend ?? []).length > 0 && (
        <section className="mt-6">
          <SectionHead title="6 tháng gần đây" />
          <div
            className="rounded-2xl border px-5 py-4"
            style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
          >
            <TrendBars data={report.data?.trend ?? []} activeMonth={month} />
            <div className="mt-3 flex justify-center gap-4 text-xs" style={{ color: "var(--tpl-muted)" }}>
              <Legend color="#22c55e" label="Thu" />
              <Legend color="#ef4444" label="Chi" />
            </div>
          </div>
        </section>
      )}

      {/* ── recent activity ───────────────────────────────────────────── */}
      <section className="mt-6">
        <SectionHead title="Giao dịch gần đây" href="/bank/transactions" />
        <RecentList
          transactions={(recent.data?.transactions ?? []).slice(0, RECENT_LIMIT)}
          loading={recent.isPending}
          categoryOf={(id) => (id ? catById.get(id) ?? null : null)}
          accountOf={(id) => accountById.get(id) ?? null}
        />
      </section>

      {adding && <QuickAddModal onClose={() => setAdding(false)} />}
    </section>
  );
}

/** How many of the month's latest rows the home screen shows before sending the
 *  reader to the full history. */
const RECENT_LIMIT = 12;

/* ── pieces ────────────────────────────────────────────────────────── */

function Stat({
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
      className="px-5 py-3"
      style={bordered ? { borderLeft: "1px solid var(--tpl-border)", borderRight: "1px solid var(--tpl-border)" } : undefined}
    >
      <p className="text-[11px] font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
        {label}
      </p>
      <p className="mt-0.5 text-base font-bold tabular-nums" style={{ color: tone }}>
        {signed && value > 0 ? "+" : ""}
        {formatVND(value)}
      </p>
    </div>
  );
}

function WalletCard({ account }: { account: BankAccount }) {
  return (
    <div
      className="flex items-center justify-between rounded-xl border px-4 py-3"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      <div className="min-w-0">
        <p className="truncate text-sm font-medium" style={{ color: "var(--tpl-text)" }}>
          {account.name}
        </p>
        <p className="text-[11px] uppercase" style={{ color: "var(--tpl-muted)" }}>
          {ACCOUNT_TYPE_LABEL[account.type] ?? account.type}
        </p>
      </div>
      <span
        className="shrink-0 text-sm font-bold tabular-nums"
        style={{ color: account.balance < 0 ? "#ef4444" : "var(--tpl-heading)" }}
      >
        {formatVND(account.balance)}
      </span>
    </div>
  );
}

export const ACCOUNT_TYPE_LABEL: Record<string, string> = {
  cash: "Tiền mặt",
  checking: "Thanh toán",
  savings: "Tiết kiệm",
  credit_card: "Thẻ tín dụng",
  ewallet: "Ví điện tử",
  other: "Khác",
};

function SectionHead({ title, href }: { title: string; href?: string }) {
  return (
    <div className="mb-2 flex items-baseline justify-between">
      <h2 className="text-sm font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
        {title}
      </h2>
      {href && (
        <Link href={href as Route} className="text-xs font-semibold" style={{ color: "var(--tpl-accent)" }}>
          Xem tất cả
        </Link>
      )}
    </div>
  );
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <span className="h-2 w-2 rounded-full" style={{ background: color }} />
      {label}
    </span>
  );
}

/**
 * Recent transactions, grouped by day with a per-day net — the shape a spending
 * log is read in. A flat list forces the reader to work out where one day ends,
 * and the daily total is the number people actually check.
 */
function RecentList({
  transactions,
  loading,
  categoryOf,
  accountOf,
}: {
  transactions: BankTransaction[];
  loading: boolean;
  categoryOf: (id: string | null) => { name: string; icon: string | null; color: string | null } | null;
  accountOf: (id: string) => BankAccount | null;
}) {
  const days = useMemo(() => groupByDay(transactions), [transactions]);

  if (loading) {
    return (
      <div className="rounded-2xl border p-6 text-sm" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}>
        Đang tải…
      </div>
    );
  }
  if (transactions.length === 0) {
    return (
      <div
        className="flex flex-col items-center gap-2 rounded-2xl border border-dashed py-10 text-center"
        style={{ borderColor: "var(--tpl-border)" }}
      >
        <Icon name="stats-icon" size={24} style={{ color: "var(--tpl-muted)" }} />
        <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
          Chưa có giao dịch nào — bấm “Ghi chép” để thêm khoản đầu tiên.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {days.map(([day, rows]) => {
        const net = rows.reduce((s, t) => s + signedAmount(t), 0);
        return (
          <div
            key={day}
            className="overflow-hidden rounded-2xl border"
            style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
          >
            <div
              className="flex items-baseline justify-between px-4 py-2"
              style={{ background: "var(--tpl-surface-2)" }}
            >
              <span className="text-xs font-bold" style={{ color: "var(--tpl-text)" }}>
                {dayLabel(day)}
              </span>
              <span
                className="text-xs font-semibold tabular-nums"
                style={{ color: net < 0 ? "#ef4444" : "#22c55e" }}
              >
                {net > 0 ? "+" : ""}
                {formatVND(net)}
              </span>
            </div>
            <ul className="divide-y" style={{ borderColor: "var(--tpl-border)" }}>
              {rows.map((t) => (
                <TransactionRow
                  key={t.id}
                  tx={t}
                  category={categoryOf(t.category_id)}
                  accountName={accountOf(t.account_id)?.name ?? ""}
                />
              ))}
            </ul>
          </div>
        );
      })}
    </div>
  );
}

/** Rows in `occurred_at` order, newest day first, preserving server order within a day. */
export function groupByDay(rows: BankTransaction[]): [string, BankTransaction[]][] {
  const map = new Map<string, BankTransaction[]>();
  for (const t of rows) {
    const list = map.get(t.occurred_at);
    if (list) list.push(t);
    else map.set(t.occurred_at, [t]);
  }
  return [...map.entries()].sort((a, b) => (a[0] < b[0] ? 1 : -1));
}

export function TransactionRow({
  tx,
  category,
  accountName,
  onDelete,
}: {
  tx: BankTransaction;
  category: { name: string; icon: string | null; color: string | null } | null;
  accountName: string;
  onDelete?: () => void;
}) {
  const amount = signedAmount(tx);
  // A transfer leg has no category — it is a movement, not a spend, and showing
  // it as an uncategorised expense would double-count against the month.
  const title = tx.is_transfer ? "Chuyển khoản" : category?.name ?? "Chưa phân loại";

  return (
    <li className="flex items-center gap-3 px-4 py-2.5" style={{ borderColor: "var(--tpl-border)" }}>
      {tx.is_transfer ? (
        <CategoryChip icon="🔄" color="#0ea5e9" name={title} size={36} />
      ) : (
        <CategoryChip icon={category?.icon} color={category?.color} name={title} size={36} />
      )}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium" style={{ color: "var(--tpl-text)" }}>
          {title}
        </p>
        <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
          {[accountName, tx.note].filter(Boolean).join(" · ") || "—"}
        </p>
      </div>
      <span
        className="shrink-0 text-sm font-semibold tabular-nums"
        style={{ color: tx.is_transfer ? "var(--tpl-muted)" : amount < 0 ? "#ef4444" : "#22c55e" }}
      >
        {amount > 0 ? "+" : ""}
        {formatVND(amount)}
      </span>
      {onDelete && (
        <button
          type="button"
          onClick={onDelete}
          aria-label={`Xoá giao dịch ${title}`}
          className="shrink-0 transition hover:text-[#ef4444]"
          style={{ color: "var(--tpl-muted)" }}
        >
          <Icon name="little-delete" size={14} />
        </button>
      )}
    </li>
  );
}
