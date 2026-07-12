"use client";

import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { getDashboard } from "@/lib/bank";
import { MoneyDisplay } from "../ui/Money";

// Finance month card (SPEC-06 P0.4). Wired to /bank/dashboard; renders nothing if
// the bank module is absent or errors (empty-state degrade, failure-isolated).
export function FinanceWidget() {
  const { data } = useQuery({ queryKey: ["bank", "dashboard", "home"], queryFn: () => getDashboard(), retry: false });
  if (!data) return null;
  return (
    <div className="rounded-xl border p-4" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>This month</h3>
        <Link href={"/bank" as Route} className="text-xs" style={{ color: "var(--tpl-accent)" }}>Ledger</Link>
      </div>
      <div className="grid grid-cols-2 gap-2 text-sm">
        <div>
          <div className="text-xs" style={{ color: "var(--tpl-muted)" }}>Income</div>
          <MoneyDisplay amount={data.income} className="font-semibold text-green-500" />
        </div>
        <div>
          <div className="text-xs" style={{ color: "var(--tpl-muted)" }}>Expense</div>
          <MoneyDisplay amount={data.expense} className="font-semibold text-red-500" />
        </div>
      </div>
    </div>
  );
}
