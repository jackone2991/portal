"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  currentMonth,
  formatVND,
  listBudgets,
  listCategories,
  setBudget,
  type BankCategory,
} from "@/lib/bank";
import { MoneyInput } from "../../components/ui/Money";
import { BackLink } from "../../components/bank/BackLink";
import { CategoryChip } from "../../components/bank/CategoryChip";
import { MonthPager } from "./MonthPager";

/** Expense categories, parents first with their children directly beneath. */
function tree(categories: BankCategory[]): BankCategory[] {
  const expense = categories.filter((c) => c.kind === "expense");
  const tops = expense.filter((c) => !c.parent_id).sort((a, b) => a.name.localeCompare(b.name, "vi"));
  const out: BankCategory[] = [];
  for (const p of tops) {
    out.push(p);
    out.push(
      ...expense
        .filter((c) => c.parent_id === p.id)
        .sort((a, b) => a.name.localeCompare(b.name, "vi")),
    );
  }
  return out;
}

/**
 * Monthly spending caps, one per expense category.
 *
 * Budgeted categories float to the top: once a few are set, they are the only
 * rows the user comes back to check, and hunting for them among thirty
 * unbudgeted ones every month is the difference between a screen that gets used
 * and one that does not. The rest stay below, ready to be given a number.
 *
 * A budget is stored per (category, month), so editing one month never touches
 * another — "January was tighter" stays true after February is set.
 */
export function BudgetsView() {
  const qc = useQueryClient();
  const [month, setMonth] = useState(currentMonth());
  const [edits, setEdits] = useState<Record<string, number>>({});
  const [err, setErr] = useState<string | null>(null);

  const { data: categories = [] } = useQuery({ queryKey: ["bank", "categories"], queryFn: listCategories });
  const { data: budgets } = useQuery({ queryKey: ["bank", "budgets", month], queryFn: () => listBudgets(month) });

  const byCat = useMemo(() => {
    const m = new Map<string, { amount: number; spent: number }>();
    budgets?.budgets.forEach((b) => m.set(b.category_id, { amount: b.amount, spent: b.spent }));
    return m;
  }, [budgets]);

  const rows = useMemo(() => {
    const all = tree(categories);
    const set = all.filter((c) => byCat.has(c.id));
    const unset = all.filter((c) => !byCat.has(c.id));
    return { set, unset };
  }, [categories, byCat]);

  const save = useMutation({
    mutationFn: (v: { category_id: string; amount: number }) =>
      // 0 means "no budget": the API takes null to clear, so an emptied field
      // removes the cap rather than storing a budget of zero that every spend
      // instantly blows through.
      setBudget({ category_id: v.category_id, month, amount: v.amount || null }),
    onMutate: () => setErr(null),
    onSuccess: (_d, v) => {
      setEdits((s) => {
        const next = { ...s };
        delete next[v.category_id];
        return next;
      });
      qc.invalidateQueries({ queryKey: ["bank"] });
    },
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không lưu được ngân sách."),
  });

  const totalBudget = (budgets?.budgets ?? []).reduce((s, b) => s + b.amount, 0);
  const totalSpent = (budgets?.budgets ?? []).reduce((s, b) => s + b.spent, 0);

  const row = (c: BankCategory) => {
    const existing = byCat.get(c.id);
    const draft = edits[c.id] ?? existing?.amount ?? 0;
    const dirty = edits[c.id] !== undefined && edits[c.id] !== (existing?.amount ?? 0);
    const pct = existing && existing.amount > 0 ? (existing.spent / existing.amount) * 100 : 0;
    const over = pct > 100;

    return (
      <li
        key={c.id}
        className="flex flex-wrap items-center gap-3 px-4 py-3"
        style={{ borderColor: "var(--tpl-border)" }}
      >
        <CategoryChip icon={c.icon} color={c.color} name={c.name} size={34} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium" style={{ color: "var(--tpl-text)" }}>
            {c.name}
          </p>
          {existing ? (
            <>
              <div className="mt-1.5 h-2 overflow-hidden rounded-full" style={{ background: "var(--tpl-surface-2)" }}>
                <div
                  className="h-full rounded-full transition-all"
                  style={{
                    width: `${Math.min(100, Math.max(2, pct))}%`,
                    background: over ? "#ef4444" : pct > 80 ? "#f59e0b" : "#22c55e",
                  }}
                />
              </div>
              <p className="mt-1 text-[11px] tabular-nums" style={{ color: over ? "#ef4444" : "var(--tpl-muted)" }}>
                Đã chi {formatVND(existing.spent)} / {formatVND(existing.amount)}
                {over
                  ? ` · vượt ${formatVND(existing.spent - existing.amount)}`
                  : ` · còn ${formatVND(existing.amount - existing.spent)}`}
              </p>
            </>
          ) : (
            <p className="text-[11px]" style={{ color: "var(--tpl-muted)" }}>
              Chưa đặt ngân sách
            </p>
          )}
        </div>

        <div className="w-36">
          <MoneyInput
            value={draft}
            onChange={(v) => setEdits((s) => ({ ...s, [c.id]: v }))}
            className="w-full rounded-lg border px-3 py-1.5 text-right text-sm tabular-nums outline-none"
          />
        </div>
        <button
          type="button"
          onClick={() => save.mutate({ category_id: c.id, amount: draft })}
          disabled={!dirty || save.isPending}
          className="rounded-lg px-3 py-1.5 text-xs font-semibold text-white transition disabled:opacity-30"
          style={{ background: "var(--tpl-accent)" }}
        >
          Lưu
        </button>
      </li>
    );
  };

  return (
    <section className="pb-8">
      <header className="mb-5">
        <BackLink />
        <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
          Ngân sách
        </h1>
      </header>

      <MonthPager month={month} onChange={setMonth} />

      {totalBudget > 0 && (
        <div
          className="mt-4 rounded-2xl border p-5"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          <div className="flex items-baseline justify-between">
            <p className="text-xs font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
              Tổng ngân sách
            </p>
            <p className="text-sm font-bold tabular-nums" style={{ color: totalSpent > totalBudget ? "#ef4444" : "var(--tpl-heading)" }}>
              {formatVND(totalSpent)} / {formatVND(totalBudget)}
            </p>
          </div>
          <div className="mt-2 h-2.5 overflow-hidden rounded-full" style={{ background: "var(--tpl-surface-2)" }}>
            <div
              className="h-full rounded-full"
              style={{
                width: `${Math.min(100, Math.max(2, (totalSpent / totalBudget) * 100))}%`,
                background: totalSpent > totalBudget ? "#ef4444" : "#22c55e",
              }}
            />
          </div>
        </div>
      )}

      {err && (
        <p
          className="mt-3 rounded-lg border px-3 py-2 text-sm"
          style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
        >
          {err}
        </p>
      )}

      {rows.set.length > 0 && (
        <>
          <h2 className="mb-2 mt-5 text-sm font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
            Đang theo dõi
          </h2>
          <ul
            className="divide-y rounded-2xl border"
            style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
          >
            {rows.set.map(row)}
          </ul>
        </>
      )}

      <h2 className="mb-2 mt-5 text-sm font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
        Danh mục khác
      </h2>
      <ul
        className="divide-y rounded-2xl border"
        style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
      >
        {rows.unset.map(row)}
      </ul>
    </section>
  );
}
