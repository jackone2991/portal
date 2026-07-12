"use client";

import { useMemo, useState } from "react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  type Direction,
  createTransaction,
  deleteTransaction,
  listAccounts,
  listCategories,
  listTransactions,
} from "@/lib/bank";
import { problemDisplayMessage } from "@/lib/problems";
import { ApiError } from "@/lib/api-client";
import { MoneyDisplay, MoneyInput } from "../../components/ui/Money";

function today(): string {
  return new Date().toISOString().slice(0, 10);
}

export function TransactionsView() {
  const qc = useQueryClient();
  const { data: accounts = [] } = useQuery({ queryKey: ["bank", "accounts"], queryFn: listAccounts });
  const { data: categories = [] } = useQuery({ queryKey: ["bank", "categories"], queryFn: listCategories });

  const [monthFilter, setMonthFilter] = useState("");
  const [accountFilter, setAccountFilter] = useState("");

  const list = useInfiniteQuery({
    queryKey: ["bank", "transactions", { month: monthFilter, account: accountFilter }],
    queryFn: ({ pageParam }) =>
      listTransactions({ cursor: pageParam || undefined, month: monthFilter || undefined, account: accountFilter || undefined }),
    initialPageParam: "",
    getNextPageParam: (last) => last.next_cursor || undefined,
  });

  // quick-add state
  const active = accounts.filter((a) => !a.archived);
  const [amount, setAmount] = useState(0);
  const [direction, setDirection] = useState<Direction>("debit");
  const [accountId, setAccountId] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [date, setDate] = useState(today());
  const [note, setNote] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const kindForDir = direction === "debit" ? "expense" : "income";
  const pickableCats = useMemo(() => categories.filter((c) => c.kind === kindForDir), [categories, kindForDir]);
  const accountName = (id: string | null) => accounts.find((a) => a.id === id)?.name ?? "—";
  const categoryName = (id: string | null) => categories.find((c) => c.id === id)?.name ?? null;

  const invalidate = () => qc.invalidateQueries({ queryKey: ["bank"] });
  const add = useMutation({
    mutationFn: () =>
      createTransaction({
        account_id: accountId || active[0]?.id || "",
        category_id: categoryId || pickableCats[0]?.id || "",
        amount,
        direction,
        occurred_at: date,
        note: note || null,
      }),
    onSuccess: () => {
      setAmount(0);
      setNote("");
      setErr(null);
      invalidate();
    },
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not save"),
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteTransaction(id),
    onSuccess: invalidate,
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not delete"),
  });

  const rows = list.data?.pages.flatMap((p) => p.transactions) ?? [];

  return (
    <main className="mx-auto max-w-3xl p-6 text-white">
      <h1 className="mb-6 text-2xl font-bold">Transactions</h1>

      {/* quick-add */}
      <form
        className="mb-8 grid grid-cols-2 gap-3 rounded-lg border border-gray-800 bg-gray-900 p-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (amount > 0) add.mutate();
        }}
      >
        <div className="col-span-2 flex gap-2">
          <button
            type="button"
            className={`flex-1 rounded-md py-2 text-sm font-medium ${direction === "debit" ? "bg-red-600" : "bg-gray-800"}`}
            onClick={() => { setDirection("debit"); setCategoryId(""); }}
          >
            Expense
          </button>
          <button
            type="button"
            className={`flex-1 rounded-md py-2 text-sm font-medium ${direction === "credit" ? "bg-green-600" : "bg-gray-800"}`}
            onClick={() => { setDirection("credit"); setCategoryId(""); }}
          >
            Income
          </button>
        </div>
        <div className="col-span-2">
          <MoneyInput value={amount} onChange={setAmount} placeholder="Amount" />
        </div>
        <select className="rounded-md border border-gray-700 bg-gray-800 px-3 py-2" value={accountId} onChange={(e) => setAccountId(e.target.value)}>
          <option value="">{active[0] ? `Account: ${active[0].name}` : "No account"}</option>
          {active.map((a) => (
            <option key={a.id} value={a.id}>{a.name}</option>
          ))}
        </select>
        <select className="rounded-md border border-gray-700 bg-gray-800 px-3 py-2" value={categoryId} onChange={(e) => setCategoryId(e.target.value)}>
          <option value="">{pickableCats[0] ? `Category: ${pickableCats[0].name}` : "No category"}</option>
          {pickableCats.map((c) => (
            <option key={c.id} value={c.id}>{c.parent_id ? "· " : ""}{c.name}</option>
          ))}
        </select>
        <input type="date" className="rounded-md border border-gray-700 bg-gray-800 px-3 py-2" value={date} onChange={(e) => setDate(e.target.value)} />
        <input className="rounded-md border border-gray-700 bg-gray-800 px-3 py-2" placeholder="Note (optional)" value={note} onChange={(e) => setNote(e.target.value)} />
        <button type="submit" disabled={add.isPending || amount <= 0 || active.length === 0} className="col-span-2 rounded-md bg-blue-600 py-2 font-medium hover:bg-blue-500 disabled:opacity-50">
          Add transaction
        </button>
        {active.length === 0 && <p className="col-span-2 text-sm text-yellow-500">Create an account first.</p>}
        {err && <p className="col-span-2 text-sm text-red-400">{err}</p>}
      </form>

      {/* filters */}
      <div className="mb-4 flex gap-2">
        <input type="month" className="rounded-md border border-gray-700 bg-gray-800 px-3 py-1.5 text-sm" value={monthFilter} onChange={(e) => setMonthFilter(e.target.value)} />
        <select className="rounded-md border border-gray-700 bg-gray-800 px-3 py-1.5 text-sm" value={accountFilter} onChange={(e) => setAccountFilter(e.target.value)}>
          <option value="">All accounts</option>
          {accounts.map((a) => (
            <option key={a.id} value={a.id}>{a.name}</option>
          ))}
        </select>
      </div>

      {/* list */}
      <ul className="divide-y divide-gray-800 rounded-lg border border-gray-800 bg-gray-900">
        {rows.map((t) => (
          <li key={t.id} className="flex items-center justify-between px-4 py-3">
            <div>
              <div className="flex items-center gap-2 text-sm">
                <span>{categoryName(t.category_id) ?? (t.is_transfer ? "Transfer" : "—")}</span>
                {t.is_transfer && <span className="rounded bg-indigo-900 px-1.5 py-0.5 text-[10px] uppercase text-indigo-300">transfer</span>}
              </div>
              <div className="text-xs text-gray-500">
                {t.occurred_at} · {accountName(t.account_id)}
                {t.note ? ` · ${t.note}` : ""}
              </div>
            </div>
            <div className="flex items-center gap-3">
              <MoneyDisplay amount={t.direction === "credit" ? t.amount : -t.amount} signed className="text-sm font-medium" />
              {!t.is_transfer && (
                <button className="text-xs text-red-400 hover:text-red-300" onClick={() => remove.mutate(t.id)}>
                  ✕
                </button>
              )}
            </div>
          </li>
        ))}
        {rows.length === 0 && !list.isLoading && <li className="px-4 py-6 text-center text-gray-500">No transactions.</li>}
      </ul>

      {list.hasNextPage && (
        <button className="mt-4 w-full rounded-md border border-gray-700 py-2 text-sm text-gray-300 hover:bg-gray-800" onClick={() => list.fetchNextPage()} disabled={list.isFetchingNextPage}>
          {list.isFetchingNextPage ? "Loading…" : "Load more"}
        </button>
      )}
    </main>
  );
}
