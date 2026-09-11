"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  accrueDebtInterest,
  addDebtMovement,
  createDebt,
  deleteDebt,
  formatVND,
  listAccounts,
  listDebts,
  updateDebt,
  type BankAccount,
  type Debt,
  type DebtDirection,
  type InterestMethod,
} from "@/lib/bank";
import { MoneyInput } from "../../components/ui/Money";
import { BackLink } from "../../components/bank/BackLink";

/**
 * Nợ & cho vay — SPEC-10 phase 1.
 *
 * The number this screen exists to protect is the one it never shows: **the
 * month's income and expense do not move** when you borrow, lend, repay or
 * collect. That money is a transfer between a wallet and the debt's own
 * account, so the ledger's existing exclusion already covers it.
 *
 * Interest is the exception and is presented as one: "Ghi lãi" posts a real,
 * categorised transaction that DOES land in the month. An accrual you cannot
 * see in the donut is an accrual you cannot reconcile.
 */
export function DebtsView() {
  const qc = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const debts = useQuery({ queryKey: ["bank", "debts"], queryFn: listDebts });
  const accounts = useQuery({ queryKey: ["bank", "accounts"], queryFn: listAccounts });

  const invalidate = () => qc.invalidateQueries({ queryKey: ["bank"] });
  const fail = (fallback: string) => (e: unknown) =>
    setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : fallback);

  const create = useMutation({
    mutationFn: (body: Parameters<typeof createDebt>[0]) => createDebt(body),
    onSuccess: () => {
      setAdding(false);
      setErr(null);
      invalidate();
    },
    onError: fail("Không tạo được khoản nợ."),
  });

  // Wallets only: the debt accounts themselves are not somewhere you move money
  // "from" by hand — that is what the movement form is for.
  const wallets = (accounts.data ?? []).filter(
    (a) => !a.archived && a.type !== "loan_payable" && a.type !== "loan_receivable",
  );

  const items = debts.data ?? [];
  const iOwe = items.filter((d) => d.direction === "borrowed" && !d.closed);
  const owedToMe = items.filter((d) => d.direction === "lent" && !d.closed);
  const closed = items.filter((d) => d.closed);

  const total = (list: Debt[]) => list.reduce((s, d) => s + d.outstanding, 0);

  return (
    <section className="pb-8">
      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <BackLink />
          <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
            Nợ &amp; cho vay
          </h1>
          <p className="mt-0.5 text-sm" style={{ color: "var(--tpl-muted)" }}>
            Tiền vay và trả nợ không tính vào thu/chi của tháng — chỉ tiền lãi mới tính.
          </p>
        </div>
        <button
          type="button"
          onClick={() => {
            setErr(null);
            setAdding((v) => !v);
          }}
          disabled={wallets.length === 0}
          className="rounded-lg px-4 py-2 text-sm font-bold text-white transition hover:opacity-90 disabled:opacity-40"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          + Khoản nợ
        </button>
      </header>

      {err && (
        <p
          role="alert"
          className="mb-4 rounded-lg border px-3 py-2 text-sm"
          style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
        >
          {err}
        </p>
      )}

      {wallets.length === 0 && !accounts.isPending && (
        <p
          className="mb-4 rounded-lg border px-3 py-2 text-sm"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
        >
          Cần ít nhất một ví trước đã — tiền vay phải chảy vào đâu đó.
        </p>
      )}

      {adding && (
        <div
          className="mb-5 rounded-xl border p-4"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          <DebtForm wallets={wallets} busy={create.isPending} onCancel={() => setAdding(false)} onSubmit={(b) => create.mutate(b)} />
        </div>
      )}

      <div className="grid gap-4 md:grid-cols-2">
        <Bucket title="Mình nợ" total={total(iOwe)} tone="#ef4444">
          {iOwe.length === 0 ? (
            <Empty>Không nợ ai cả.</Empty>
          ) : (
            iOwe.map((d) => <DebtCard key={d.id} debt={d} wallets={wallets} onError={setErr} />)
          )}
        </Bucket>

        <Bucket title="Người khác nợ mình" total={total(owedToMe)} tone="#22c55e">
          {owedToMe.length === 0 ? (
            <Empty>Chưa cho ai vay.</Empty>
          ) : (
            owedToMe.map((d) => <DebtCard key={d.id} debt={d} wallets={wallets} onError={setErr} />)
          )}
        </Bucket>
      </div>

      {closed.length > 0 && (
        <div className="mt-6">
          <h2 className="mb-2 text-sm font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
            Đã tất toán · {closed.length}
          </h2>
          <div className="space-y-2">
            {closed.map((d) => (
              <DebtCard key={d.id} debt={d} wallets={wallets} onError={setErr} />
            ))}
          </div>
        </div>
      )}
    </section>
  );
}

function Bucket({ title, total, tone, children }: { title: string; total: number; tone: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
      <div className="flex items-baseline justify-between border-b px-4 py-3" style={{ borderColor: "var(--tpl-border)" }}>
        <h2 className="text-sm font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
          {title}
        </h2>
        <span className="text-lg font-bold tabular-nums" style={{ color: tone }}>
          {formatVND(total)}
        </span>
      </div>
      <div className="space-y-2 p-3">{children}</div>
    </div>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return (
    <p className="py-6 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
      {children}
    </p>
  );
}

/* ── one debt ─────────────────────────────────────────────────────── */

function DebtCard({
  debt,
  wallets,
  onError,
}: {
  debt: Debt;
  wallets: BankAccount[];
  onError: (m: string | null) => void;
}) {
  const qc = useQueryClient();
  const [open, setOpen] = useState<"none" | "move" | "accrue">("none");
  const invalidate = () => qc.invalidateQueries({ queryKey: ["bank"] });
  const fail = (fallback: string) => (e: unknown) =>
    onError(e instanceof ApiError ? problemDisplayMessage(e.body) : fallback);

  const move = useMutation({
    mutationFn: (b: Parameters<typeof addDebtMovement>[1]) => addDebtMovement(debt.id, b),
    onSuccess: () => {
      setOpen("none");
      onError(null);
      invalidate();
    },
    onError: fail("Không ghi được khoản này."),
  });
  const accrue = useMutation({
    mutationFn: (upTo: string) => accrueDebtInterest(debt.id, upTo),
    onSuccess: () => {
      setOpen("none");
      onError(null);
      invalidate();
    },
    onError: fail("Không ghi được lãi."),
  });
  const close = useMutation({
    mutationFn: (closed: boolean) => updateDebt(debt.id, { closed }),
    onSuccess: () => {
      onError(null);
      invalidate();
    },
    onError: fail("Không đổi được trạng thái."),
  });
  const remove = useMutation({
    mutationFn: () => deleteDebt(debt.id),
    onSuccess: () => {
      onError(null);
      invalidate();
    },
    onError: fail("Không xoá được khoản nợ."),
  });

  const pct = debt.principal > 0 ? Math.min(100, Math.round((debt.settled / debt.principal) * 100)) : 0;
  const overdue = !debt.closed && debt.due_on != null && debt.due_on < new Date().toISOString().slice(0, 10);
  const moveKind = debt.direction === "borrowed" ? "repay" : "collect";

  return (
    <div
      className="rounded-lg border p-3"
      style={{ borderColor: overdue ? "rgba(239,68,68,.5)" : "var(--tpl-border)", opacity: debt.closed ? 0.65 : 1 }}
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <span className="font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {debt.counterparty}
        </span>
        <span className="text-lg font-bold tabular-nums" style={{ color: "var(--tpl-heading)" }}>
          {formatVND(debt.outstanding)}
        </span>
      </div>

      {/* Settled-vs-principal, because "còn 6 triệu" means nothing without
          knowing it started at 10. */}
      <div className="mt-2 h-1.5 overflow-hidden rounded-full" style={{ background: "var(--tpl-surface-2)" }}>
        <div className="h-full rounded-full" style={{ width: `${pct}%`, background: "var(--tpl-accent)" }} />
      </div>
      {/* "Còn X trên gốc Y", not "đã trả X": once interest is accrued the gap
          between principal and outstanding is no longer what you handed over,
          and a label that says "đã trả" would be quietly wrong. */}
      <p className="mt-1 text-xs" style={{ color: "var(--tpl-muted)" }}>
        Còn {formatVND(debt.outstanding)} trên gốc {formatVND(debt.principal)}
        {debt.due_on && (
          <>
            {" · "}
            <span style={{ color: overdue ? "#ef4444" : "var(--tpl-muted)" }}>
              {overdue ? "quá hạn " : "hạn "}
              {debt.due_on}
            </span>
          </>
        )}
        {debt.interest_method !== "none" && (
          <> · {(debt.interest_rate_bps / 100).toFixed(2)}%/năm {debt.interest_method === "simple" ? "đơn" : "kép"}</>
        )}
      </p>

      {debt.projected_interest > 0 && (
        <p className="mt-1 text-xs" style={{ color: "var(--tpl-muted)" }}>
          Lãi dự kiến đến hạn: <strong>{formatVND(debt.projected_interest)}</strong> — chưa ghi vào sổ.
        </p>
      )}

      {!debt.closed && (
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={() => setOpen(open === "move" ? "none" : "move")}
            className="text-xs font-semibold"
            style={{ color: "var(--tpl-accent)" }}
          >
            {debt.direction === "borrowed" ? "Trả bớt" : "Thu bớt"}
          </button>
          {debt.interest_method !== "none" && (
            <button
              type="button"
              onClick={() => setOpen(open === "accrue" ? "none" : "accrue")}
              className="text-xs font-semibold"
              style={{ color: "var(--tpl-accent)" }}
            >
              Ghi lãi
            </button>
          )}
          <button
            type="button"
            onClick={() => close.mutate(true)}
            disabled={debt.outstanding !== 0}
            title={debt.outstanding !== 0 ? "Còn dư nợ — trả hết trước đã" : undefined}
            className="text-xs font-semibold disabled:opacity-40"
            style={{ color: "var(--tpl-muted)" }}
          >
            Tất toán
          </button>
        </div>
      )}

      {debt.closed && (
        <div className="mt-2 flex items-center gap-3">
          <button type="button" onClick={() => close.mutate(false)} className="text-xs font-semibold" style={{ color: "var(--tpl-accent)" }}>
            Mở lại
          </button>
          <button type="button" onClick={() => remove.mutate()} className="text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
            Xoá
          </button>
        </div>
      )}

      {open === "move" && (
        <MovementForm
          wallets={wallets}
          busy={move.isPending}
          max={debt.outstanding}
          label={debt.direction === "borrowed" ? "Trả" : "Thu"}
          onCancel={() => setOpen("none")}
          onSubmit={(walletID, amount, on) =>
            move.mutate({ kind: moveKind, wallet_id: walletID, amount, occurred_at: on })
          }
        />
      )}

      {open === "accrue" && (
        <AccrueForm busy={accrue.isPending} onCancel={() => setOpen("none")} onSubmit={(d) => accrue.mutate(d)} />
      )}
    </div>
  );
}

/* ── forms ────────────────────────────────────────────────────────── */

const today = () => new Date().toISOString().slice(0, 10);

function DebtForm({
  wallets,
  busy,
  onCancel,
  onSubmit,
}: {
  wallets: BankAccount[];
  busy: boolean;
  onCancel: () => void;
  onSubmit: (b: Parameters<typeof createDebt>[0]) => void;
}) {
  const [direction, setDirection] = useState<DebtDirection>("borrowed");
  const [counterparty, setCounterparty] = useState("");
  const [principal, setPrincipal] = useState(0);
  const [walletID, setWalletID] = useState(wallets[0]?.id ?? "");
  const [openedOn, setOpenedOn] = useState(today());
  const [dueOn, setDueOn] = useState("");
  const [method, setMethod] = useState<InterestMethod>("none");
  const [ratePct, setRatePct] = useState(0);

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (!counterparty.trim() || principal <= 0 || !walletID) return;
        onSubmit({
          counterparty: counterparty.trim(),
          direction,
          principal,
          wallet_id: walletID,
          opened_on: openedOn,
          due_on: dueOn || null,
          interest_method: method,
          // Basis points: no float reaches the API, and 12.5% stays exact.
          interest_rate_bps: method === "none" ? 0 : Math.round(ratePct * 100),
        });
      }}
      className="space-y-3"
    >
      <div className="flex gap-2">
        {(["borrowed", "lent"] as DebtDirection[]).map((d) => (
          <button
            key={d}
            type="button"
            onClick={() => setDirection(d)}
            className="rounded-lg px-3 py-1.5 text-sm font-semibold transition"
            style={
              direction === d
                ? { background: "var(--tpl-accent)", color: "#fff" }
                : { border: "1px solid var(--tpl-border)", color: "var(--tpl-muted)" }
            }
          >
            {d === "borrowed" ? "Mình đi vay" : "Mình cho vay"}
          </button>
        ))}
      </div>

      <div className="flex flex-wrap gap-3">
        <Field label={direction === "borrowed" ? "Vay của ai" : "Cho ai vay"} className="min-w-[180px] flex-1">
          <input
            value={counterparty}
            onChange={(e) => setCounterparty(e.target.value)}
            maxLength={120}
            autoFocus
            className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm outline-none"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
          />
        </Field>
        <Field label="Số tiền" className="min-w-[160px]">
          <MoneyInput value={principal} onChange={setPrincipal} />
        </Field>
        <Field label={direction === "borrowed" ? "Tiền vào ví" : "Tiền ra từ ví"} className="min-w-[160px]">
          <select
            value={walletID}
            onChange={(e) => setWalletID(e.target.value)}
            className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
          >
            {wallets.map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
          </select>
        </Field>
      </div>

      <div className="flex flex-wrap gap-3">
        <Field label="Ngày vay" className="min-w-[140px]">
          <input type="date" value={openedOn} onChange={(e) => setOpenedOn(e.target.value)} className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} />
        </Field>
        <Field label="Hạn trả (không bắt buộc)" className="min-w-[140px]">
          <input type="date" value={dueOn} onChange={(e) => setDueOn(e.target.value)} className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} />
        </Field>
        <Field label="Lãi" className="min-w-[140px]">
          <select
            value={method}
            onChange={(e) => setMethod(e.target.value as InterestMethod)}
            className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
          >
            <option value="none">Không lãi</option>
            <option value="simple">Lãi đơn</option>
            <option value="compound">Lãi kép</option>
          </select>
        </Field>
        {method !== "none" && (
          <Field label="%/năm" className="w-28">
            <input
              type="number"
              step="0.01"
              min="0.01"
              value={ratePct || ""}
              onChange={(e) => setRatePct(Number(e.target.value))}
              className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
            />
          </Field>
        )}
      </div>

      <div className="flex items-center gap-2">
        <button
          type="submit"
          disabled={busy || !counterparty.trim() || principal <= 0}
          className="rounded-lg px-4 py-2 text-sm font-bold text-white transition hover:opacity-90 disabled:opacity-40"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          Ghi khoản nợ
        </button>
        <button type="button" onClick={onCancel} className="text-sm font-semibold" style={{ color: "var(--tpl-muted)" }}>
          Huỷ
        </button>
      </div>
    </form>
  );
}

function MovementForm({
  wallets,
  busy,
  max,
  label,
  onCancel,
  onSubmit,
}: {
  wallets: BankAccount[];
  busy: boolean;
  max: number;
  label: string;
  onCancel: () => void;
  onSubmit: (walletID: string, amount: number, on: string) => void;
}) {
  const [walletID, setWalletID] = useState(wallets[0]?.id ?? "");
  const [amount, setAmount] = useState(max);
  const [on, setOn] = useState(today());

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (amount > 0 && walletID) onSubmit(walletID, amount, on);
      }}
      className="mt-3 flex flex-wrap items-end gap-2 border-t pt-3"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <Field label="Số tiền" className="w-36">
        <MoneyInput value={amount} onChange={setAmount} />
      </Field>
      <Field label="Ví" className="min-w-[130px] flex-1">
        <select value={walletID} onChange={(e) => setWalletID(e.target.value)} className="w-full rounded-lg border bg-transparent px-2 py-2 text-sm" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}>
          {wallets.map((w) => (
            <option key={w.id} value={w.id}>
              {w.name}
            </option>
          ))}
        </select>
      </Field>
      <Field label="Ngày" className="w-36">
        <input type="date" value={on} onChange={(e) => setOn(e.target.value)} className="w-full rounded-lg border bg-transparent px-2 py-2 text-sm" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} />
      </Field>
      <button type="submit" disabled={busy || amount <= 0} className="rounded-lg px-3 py-2 text-xs font-bold text-white disabled:opacity-40" style={{ background: "var(--tpl-accent)" }}>
        {label}
      </button>
      <button type="button" onClick={onCancel} className="px-1 text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
        Huỷ
      </button>
    </form>
  );
}

function AccrueForm({ busy, onCancel, onSubmit }: { busy: boolean; onCancel: () => void; onSubmit: (upTo: string) => void }) {
  const [upTo, setUpTo] = useState(today());
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(upTo);
      }}
      className="mt-3 flex flex-wrap items-end gap-2 border-t pt-3"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <Field label="Tính lãi đến ngày" className="w-40">
        <input type="date" value={upTo} onChange={(e) => setUpTo(e.target.value)} className="w-full rounded-lg border bg-transparent px-2 py-2 text-sm" style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }} />
      </Field>
      <button type="submit" disabled={busy} className="rounded-lg px-3 py-2 text-xs font-bold text-white disabled:opacity-40" style={{ background: "var(--tpl-accent)" }}>
        Ghi lãi
      </button>
      <button type="button" onClick={onCancel} className="px-1 text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
        Huỷ
      </button>
      <p className="w-full text-xs" style={{ color: "var(--tpl-muted)" }}>
        Lãi là một khoản chi thật — nó sẽ hiện trong báo cáo tháng, khác với tiền gốc.
      </p>
    </form>
  );
}

function Field({ label, className, children }: { label: string; className?: string; children: React.ReactNode }) {
  return (
    <label className={className}>
      <span className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
        {label}
      </span>
      {children}
    </label>
  );
}
