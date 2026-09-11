"use client";

import { useMemo, useState } from "react";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import { useInfiniteScroll } from "@/lib/use-infinite-scroll";
import {
  currentMonth,
  dayLabel,
  deleteTransaction,
  deleteTransfer,
  formatVND,
  listAccounts,
  listCategories,
  listTransactions,
  signedAmount,
  type BankTransaction,
} from "@/lib/bank";
import { BackLink } from "../../components/bank/BackLink";
import { QuickAddModal } from "../../components/bank/QuickAddModal";
import { MonthPager } from "./MonthPager";
import { groupByDay, TransactionRow } from "./DashboardView";

/**
 * The full ledger history for a month, grouped by day.
 *
 * Grouping is not decoration: a spending log is read as "what did Tuesday cost",
 * and a flat list makes the reader find the day boundaries themselves. Each day
 * carries its net so the common question is answered without arithmetic.
 *
 * Paginates on scroll (`useInfiniteScroll`) rather than a button — see
 * frontend/CLAUDE.md; the API is keyset-paginated and a month of daily spending
 * runs well past one page.
 */
export function TransactionsView() {
  const qc = useQueryClient();

  const [month, setMonth] = useState(currentMonth());
  const [accountId, setAccountId] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [adding, setAdding] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const accounts = useQuery({ queryKey: ["bank", "accounts"], queryFn: listAccounts });
  const cats = useQuery({ queryKey: ["bank", "categories"], queryFn: listCategories });

  const list = useInfiniteQuery({
    queryKey: ["bank", "transactions", month, accountId, categoryId],
    queryFn: ({ pageParam }: { pageParam: string | undefined }) =>
      listTransactions({
        month,
        account: accountId || undefined,
        category: categoryId || undefined,
        cursor: pageParam,
      }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });

  const rows = useMemo(
    () => list.data?.pages.flatMap((p) => p.transactions) ?? [],
    [list.data],
  );
  const days = useMemo(() => groupByDay(rows), [rows]);

  const sentinelRef = useInfiniteScroll({
    onLoadMore: () => list.fetchNextPage(),
    hasMore: list.hasNextPage,
    isLoading: list.isFetchingNextPage,
  });

  const catById = useMemo(() => new Map((cats.data ?? []).map((c) => [c.id, c] as const)), [cats.data]);
  const acctById = useMemo(() => new Map((accounts.data ?? []).map((a) => [a.id, a] as const)), [accounts.data]);

  const remove = useMutation({
    // Deleting one leg of a transfer would leave the other stranded and the two
    // wallet balances permanently out of step, so a transfer is removed whole.
    mutationFn: (t: BankTransaction) =>
      t.transfer_id ? deleteTransfer(t.transfer_id) : deleteTransaction(t.id),
    onMutate: () => setErr(null),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["bank"] }),
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không xoá được giao dịch."),
  });

  const monthNet = rows.reduce((s, t) => s + (t.is_transfer ? 0 : signedAmount(t)), 0);

  return (
    <section className="pb-8">
      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <BackLink />
          <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
            Giao dịch
          </h1>
        </div>
        <button
          type="button"
          onClick={() => setAdding(true)}
          className="rounded-lg px-4 py-2 text-sm font-bold text-white transition hover:opacity-90"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          + Ghi chép
        </button>
      </header>

      <MonthPager month={month} onChange={setMonth} />

      <div className="mt-3 flex flex-wrap gap-2">
        <Filter value={accountId} onChange={setAccountId} label="Tất cả ví">
          {(accounts.data ?? []).map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
            </option>
          ))}
        </Filter>
        <Filter value={categoryId} onChange={setCategoryId} label="Tất cả danh mục">
          {(cats.data ?? []).map((c) => (
            <option key={c.id} value={c.id}>
              {c.icon ? `${c.icon} ` : ""}
              {c.name}
            </option>
          ))}
        </Filter>
      </div>

      {err && (
        <p
          className="mt-3 rounded-lg border px-3 py-2 text-sm"
          style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
        >
          {err}
        </p>
      )}

      {list.isPending ? (
        <p className="mt-6 text-sm" style={{ color: "var(--tpl-muted)" }}>
          Đang tải…
        </p>
      ) : rows.length === 0 ? (
        <div
          className="mt-6 rounded-2xl border border-dashed py-12 text-center text-sm"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
        >
          Không có giao dịch nào trong tháng này.
        </div>
      ) : (
        <>
          <p className="mt-4 text-xs" style={{ color: "var(--tpl-muted)" }}>
            {/* Transfers are excluded from the net — moving your own money is
                neither income nor spending. */}
            Chênh lệch tháng:{" "}
            <span className="font-bold tabular-nums" style={{ color: monthNet < 0 ? "#ef4444" : "#22c55e" }}>
              {monthNet > 0 ? "+" : ""}
              {formatVND(monthNet)}
            </span>
          </p>

          <div className="mt-2 space-y-3">
            {days.map(([day, dayRows]) => {
              const net = dayRows.reduce((s, t) => s + (t.is_transfer ? 0 : signedAmount(t)), 0);
              return (
                <div
                  key={day}
                  className="overflow-hidden rounded-2xl border"
                  style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
                >
                  <div className="flex items-baseline justify-between px-4 py-2" style={{ background: "var(--tpl-surface-2)" }}>
                    <span className="text-xs font-bold" style={{ color: "var(--tpl-text)" }}>
                      {dayLabel(day)}
                    </span>
                    <span className="text-xs font-semibold tabular-nums" style={{ color: net < 0 ? "#ef4444" : "#22c55e" }}>
                      {net > 0 ? "+" : ""}
                      {formatVND(net)}
                    </span>
                  </div>
                  <ul className="divide-y" style={{ borderColor: "var(--tpl-border)" }}>
                    {dayRows.map((t) => (
                      <TransactionRow
                        key={t.id}
                        tx={t}
                        category={t.category_id ? catById.get(t.category_id) ?? null : null}
                        accountName={acctById.get(t.account_id)?.name ?? ""}
                        onDelete={() => remove.mutate(t)}
                      />
                    ))}
                  </ul>
                </div>
              );
            })}
          </div>

          <div ref={sentinelRef} aria-hidden />
          <p className="mt-4 text-center text-xs" style={{ color: "var(--tpl-muted)" }} aria-live="polite">
            {list.isFetchingNextPage
              ? "Đang tải…"
              : list.hasNextPage
                ? `Đang hiển thị ${rows.length}+ giao dịch`
                : `${rows.length} giao dịch`}
          </p>
        </>
      )}

      {adding && <QuickAddModal onClose={() => setAdding(false)} />}
    </section>
  );
}

function Filter({
  value,
  onChange,
  label,
  children,
}: {
  value: string;
  onChange: (v: string) => void;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      aria-label={label}
      className="rounded-lg border px-3 py-1.5 text-sm"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)", color: "var(--tpl-text)" }}
    >
      <option value="">{label}</option>
      {children}
    </select>
  );
}
