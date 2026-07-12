"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type BankCategory, currentMonth, listBudgets, listCategories, setBudget } from "@/lib/bank";
import { MoneyDisplay, MoneyInput } from "../../components/ui/Money";

// Order expense categories parents-first, children indented under them.
function tree(categories: BankCategory[]): BankCategory[] {
  const expense = categories.filter((c) => c.kind === "expense");
  const tops = expense.filter((c) => !c.parent_id).sort((a, b) => a.name.localeCompare(b.name));
  const out: BankCategory[] = [];
  for (const p of tops) {
    out.push(p);
    out.push(...expense.filter((c) => c.parent_id === p.id).sort((a, b) => a.name.localeCompare(b.name)));
  }
  return out;
}

export function BudgetsView() {
  const qc = useQueryClient();
  const [month, setMonth] = useState(currentMonth());
  const { data: categories = [] } = useQuery({ queryKey: ["bank", "categories"], queryFn: listCategories });
  const { data: budgets } = useQuery({ queryKey: ["bank", "budgets", month], queryFn: () => listBudgets(month) });

  const rows = useMemo(() => tree(categories), [categories]);
  const budgetByCat = useMemo(() => {
    const m = new Map<string, { amount: number; spent: number }>();
    budgets?.budgets.forEach((b) => m.set(b.category_id, { amount: b.amount, spent: b.spent }));
    return m;
  }, [budgets]);

  const [edits, setEdits] = useState<Record<string, number>>({});
  const save = useMutation({
    mutationFn: (v: { category_id: string; amount: number }) => setBudget({ category_id: v.category_id, month: `${month}`, amount: v.amount || null }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["bank", "budgets", month] }),
  });

  return (
    <main className="mx-auto max-w-3xl p-6 text-white">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">Budgets</h1>
        <input type="month" className="rounded-md border border-gray-700 bg-gray-800 px-3 py-1.5 text-sm" value={month} onChange={(e) => setMonth(e.target.value)} />
      </div>

      <ul className="space-y-1">
        {rows.map((c) => {
          const existing = budgetByCat.get(c.id);
          const draft = edits[c.id] ?? existing?.amount ?? 0;
          const pct = existing && existing.amount > 0 ? Math.round((existing.spent / existing.amount) * 100) : 0;
          const over = pct > 100;
          return (
            <li key={c.id} className={`rounded-lg border border-gray-800 bg-gray-900 p-3 ${c.parent_id ? "ml-6" : ""}`}>
              <div className="flex items-center justify-between gap-3">
                <span className="flex-1 text-sm">
                  {c.parent_id ? "· " : ""}
                  {c.name}
                </span>
                <div className="w-40">
                  <MoneyInput value={draft} onChange={(v) => setEdits((s) => ({ ...s, [c.id]: v }))} />
                </div>
                <button
                  className="rounded-md bg-blue-600 px-3 py-2 text-xs font-medium hover:bg-blue-500 disabled:opacity-50"
                  disabled={save.isPending || (edits[c.id] ?? existing?.amount ?? 0) === (existing?.amount ?? 0)}
                  onClick={() => save.mutate({ category_id: c.id, amount: draft })}
                >
                  Save
                </button>
              </div>
              {existing && (
                <div className="mt-2">
                  <div className="h-2 overflow-hidden rounded-full bg-gray-700">
                    <div className={`h-full ${over ? "bg-red-500" : "bg-blue-500"}`} style={{ width: `${Math.min(pct, 100)}%` }} />
                  </div>
                  <div className="mt-1 flex justify-between text-[11px] text-gray-400">
                    <span>
                      <MoneyDisplay amount={existing.spent} /> / <MoneyDisplay amount={existing.amount} />
                    </span>
                    <span className={over ? "text-red-400" : ""}>{pct}%</span>
                  </div>
                </div>
              )}
            </li>
          );
        })}
      </ul>
      <p className="mt-4 text-xs text-gray-500">Set an amount and Save. Save with 0 to remove a budget.</p>
    </main>
  );
}
