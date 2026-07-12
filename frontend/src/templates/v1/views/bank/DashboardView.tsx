"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { type BankAccount, currentMonth, getDashboard } from "@/lib/bank";
import { MoneyDisplay } from "../../components/ui/Money";

function byCurrency(accounts: BankAccount[]): Record<string, BankAccount[]> {
  const g: Record<string, BankAccount[]> = {};
  for (const a of accounts) {
    if (a.archived) continue;
    (g[a.currency] ??= []).push(a);
  }
  return g;
}

export function DashboardView() {
  const [month, setMonth] = useState(currentMonth());
  const { data, isLoading } = useQuery({ queryKey: ["bank", "dashboard", month], queryFn: () => getDashboard(month) });

  const groups = useMemo(() => byCurrency(data?.accounts ?? []), [data]);
  // Dashboard budget block: one bar per budgeted category with no budgeted ancestor.
  const budgetSet = useMemo(() => new Set(data?.budgets.map((b) => b.category_id)), [data]);
  const topBudgets = (data?.budgets ?? []).filter((b) => !(b.parent_id && budgetSet.has(b.parent_id)));

  return (
    <main className="mx-auto max-w-4xl p-6 text-white">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">Ledger</h1>
        <div className="flex items-center gap-2">
          <input type="month" className="rounded-md border border-gray-700 bg-gray-800 px-3 py-1.5 text-sm" value={month} onChange={(e) => setMonth(e.target.value)} />
          <Link href={"/bank/transactions" as Route} className="rounded-md bg-blue-600 px-4 py-1.5 text-sm font-medium hover:bg-blue-500">
            + Add
          </Link>
        </div>
      </div>

      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : (
        <div className="space-y-8">
          {/* balances by currency */}
          <section>
            {Object.entries(groups).map(([cur, accts]) => {
              const total = accts.reduce((s, a) => s + a.balance, 0);
              return (
                <div key={cur} className="mb-4 rounded-lg border border-gray-800 bg-gray-900 p-4">
                  <div className="mb-2 flex items-center justify-between text-sm text-gray-400">
                    <span>Balance ({cur})</span>
                    <MoneyDisplay amount={total} className="text-lg font-bold text-white" />
                  </div>
                  <ul className="divide-y divide-gray-800">
                    {accts.map((a) => (
                      <li key={a.id} className="flex justify-between py-1.5 text-sm">
                        <span>{a.name}</span>
                        <MoneyDisplay amount={a.balance} />
                      </li>
                    ))}
                  </ul>
                </div>
              );
            })}
            {Object.keys(groups).length === 0 && (
              <Link href={"/bank/accounts" as Route} className="block rounded-lg border border-dashed border-gray-700 p-6 text-center text-gray-400 hover:bg-gray-900">
                No accounts yet — add one to get started.
              </Link>
            )}
          </section>

          {/* month flow */}
          <section className="grid grid-cols-2 gap-4">
            <div className="rounded-lg border border-gray-800 bg-gray-900 p-4">
              <div className="text-xs uppercase text-gray-500">Income</div>
              <MoneyDisplay amount={data?.income ?? 0} className="text-xl font-bold text-green-400" />
            </div>
            <div className="rounded-lg border border-gray-800 bg-gray-900 p-4">
              <div className="text-xs uppercase text-gray-500">Expense</div>
              <MoneyDisplay amount={data?.expense ?? 0} className="text-xl font-bold text-red-400" />
            </div>
          </section>

          {/* budgets */}
          {topBudgets.length > 0 && (
            <section>
              <div className="mb-2 flex items-center justify-between">
                <h2 className="text-sm font-semibold uppercase text-gray-500">Budgets</h2>
                <Link href={"/bank/budgets" as Route} className="text-xs text-blue-400 hover:underline">Manage</Link>
              </div>
              <ul className="space-y-3">
                {topBudgets.map((b) => {
                  const pct = b.amount > 0 ? Math.round((b.spent / b.amount) * 100) : 0;
                  const over = pct > 100;
                  return (
                    <li key={b.category_id}>
                      <div className="mb-1 flex justify-between text-sm">
                        <span>{b.name}</span>
                        <span className={over ? "text-red-400" : "text-gray-400"}>{pct}%</span>
                      </div>
                      <div className="h-2 overflow-hidden rounded-full bg-gray-700">
                        <div className={`h-full ${over ? "bg-red-500" : "bg-blue-500"}`} style={{ width: `${Math.min(pct, 100)}%` }} />
                      </div>
                    </li>
                  );
                })}
              </ul>
            </section>
          )}

          {/* recent */}
          <section>
            <div className="mb-2 flex items-center justify-between">
              <h2 className="text-sm font-semibold uppercase text-gray-500">Recent</h2>
              <Link href={"/bank/transactions" as Route} className="text-xs text-blue-400 hover:underline">All</Link>
            </div>
            <ul className="divide-y divide-gray-800 rounded-lg border border-gray-800 bg-gray-900">
              {(data?.recent ?? []).map((t) => (
                <li key={t.id} className="flex justify-between px-4 py-2 text-sm">
                  <span className="text-gray-300">
                    {t.occurred_at}
                    {t.is_transfer ? " · Transfer" : ""}
                    {t.note ? ` · ${t.note}` : ""}
                  </span>
                  <MoneyDisplay amount={t.direction === "credit" ? t.amount : -t.amount} signed />
                </li>
              ))}
              {(data?.recent ?? []).length === 0 && <li className="px-4 py-4 text-center text-gray-500">No transactions yet.</li>}
            </ul>
          </section>
        </div>
      )}
    </main>
  );
}
