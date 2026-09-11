"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  createTransaction,
  createTransfer,
  formatVND,
  isWallet,
  listAccounts,
  listCategories,
  today,
  type BankCategory,
} from "@/lib/bank";
import { Icon } from "../ui/Icon";
import { CategoryChip } from "./CategoryChip";

type Mode = "expense" | "income" | "transfer";

/**
 * Add a transaction, amount first.
 *
 * The order is the point. A ledger only gets used if logging a coffee takes a
 * couple of seconds, and the thing you know when you open it is the number —
 * so the amount is the first and biggest control, with its own keypad, and
 * everything else has a working default (today, first wallet, no note). The
 * category grid is the only other required choice.
 *
 * The keypad is on-screen rather than relying on the OS one because this is a
 * desktop-first shell: `inputMode="numeric"` summons nothing on a laptop, and a
 * digits-only pad next to the amount is faster than reaching for the keyboard.
 * The field stays a real focusable input as well, so typing works too.
 */
export function QuickAddModal({ onClose, defaultMode = "expense" }: { onClose: () => void; defaultMode?: Mode }) {
  const qc = useQueryClient();

  const [mode, setMode] = useState<Mode>(defaultMode);
  const [amount, setAmount] = useState(0);
  const [categoryId, setCategoryId] = useState<string | null>(null);
  const [accountId, setAccountId] = useState<string | null>(null);
  const [toAccountId, setToAccountId] = useState<string | null>(null);
  const [date, setDate] = useState(today());
  const [note, setNote] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const accounts = useQuery({ queryKey: ["bank", "accounts"], queryFn: listAccounts });
  const categories = useQuery({ queryKey: ["bank", "categories"], queryFn: listCategories });

  const openAccounts = useMemo(
    // Wallets only — booking "Ăn uống" against a debt account would be a
    // categorised flow on a liability, which is what an accrual is, not a spend.
    () => (accounts.data ?? []).filter((a) => !a.archived && isWallet(a)),
    [accounts.data],
  );
  const account = accountId ?? openAccounts[0]?.id ?? null;
  const toAccount = toAccountId ?? openAccounts.find((a) => a.id !== account)?.id ?? null;

  // Only leaf-level choices are offered: picking a parent that has children
  // ("Ăn uống" when "Cà phê" exists) produces a total that the report then
  // cannot attribute, so the grid shows children in place of their parent.
  const pickable = useMemo(() => {
    const all = categories.data ?? [];
    const kind = mode === "income" ? "income" : "expense";
    const inKind = all.filter((c) => c.kind === kind);
    const hasChild = new Set(inKind.filter((c) => c.parent_id).map((c) => c.parent_id));
    return inKind.filter((c) => c.parent_id !== null || !hasChild.has(c.id));
  }, [categories.data, mode]);

  const parentName = useMemo(() => {
    const byId = new Map((categories.data ?? []).map((c) => [c.id, c] as const));
    return (c: BankCategory) => (c.parent_id ? byId.get(c.parent_id)?.name ?? null : null);
  }, [categories.data]);

  const save = useMutation({
    mutationFn: async () => {
      if (mode === "transfer") {
        if (!account || !toAccount) throw new Error("Cần chọn cả hai ví.");
        return createTransfer({
          from_account: account,
          to_account: toAccount,
          amount,
          occurred_at: date,
          note: note.trim() || null,
        });
      }
      if (!account) throw new Error("Cần chọn ví.");
      if (!categoryId) throw new Error("Cần chọn danh mục.");
      return createTransaction({
        account_id: account,
        category_id: categoryId,
        amount,
        direction: mode === "income" ? "credit" : "debit",
        occurred_at: date,
        note: note.trim() || null,
      });
    },
    onMutate: () => setErr(null),
    onSuccess: () => {
      // Every ledger read is derived from transactions — balances, the month
      // totals, the report, the budget bars — so one broad invalidation is
      // correct here and cheaper to reason about than five targeted ones.
      qc.invalidateQueries({ queryKey: ["bank"] });
      onClose();
    },
    onError: (e) =>
      setErr(
        e instanceof ApiError
          ? problemDisplayMessage(e.body)
          : e instanceof Error
            ? e.message
            : "Không lưu được giao dịch.",
      ),
  });

  const canSave =
    amount > 0 &&
    !!account &&
    (mode === "transfer" ? !!toAccount && toAccount !== account : !!categoryId) &&
    !save.isPending;

  const press = (d: string) => {
    if (d === "back") {
      setAmount((a) => Math.floor(a / 10));
      return;
    }
    // Cap before it can overflow the int64 the server stores; VND has no minor
    // unit, so 15 digits is already an absurd number of đồng.
    setAmount((a) => {
      const next = Number(String(a) + d);
      return Number.isSafeInteger(next) && String(next).length <= 15 ? next : a;
    });
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/50 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label="Thêm giao dịch"
    >
      <div
        className="my-8 w-full max-w-lg rounded-2xl shadow-xl"
        style={{ background: "var(--tpl-surface)" }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* ── mode + amount ─────────────────────────────────────────── */}
        <div
          className="rounded-t-2xl px-5 pb-5 pt-4"
          style={{ background: "var(--tpl-surface-2)" }}
        >
          <div className="mb-4 flex items-center justify-between">
            <div className="flex gap-1 rounded-lg p-1" style={{ background: "var(--tpl-bg)" }}>
              {(
                [
                  ["expense", "Chi tiền"],
                  ["income", "Thu tiền"],
                  ["transfer", "Chuyển"],
                ] as const
              ).map(([m, label]) => (
                <button
                  key={m}
                  type="button"
                  onClick={() => {
                    setMode(m);
                    setCategoryId(null);
                  }}
                  aria-pressed={mode === m}
                  className="rounded-md px-3 py-1.5 text-sm font-semibold transition"
                  style={{
                    background: mode === m ? "var(--tpl-accent)" : "transparent",
                    color: mode === m ? "#fff" : "var(--tpl-muted)",
                  }}
                >
                  {label}
                </button>
              ))}
            </div>
            <button type="button" onClick={onClose} aria-label="Đóng" style={{ color: "var(--tpl-muted)" }}>
              <Icon name="close-icon" size={12} />
            </button>
          </div>

          <label className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }} htmlFor="qa-amount">
            Số tiền
          </label>
          <div className="flex items-baseline gap-2">
            <input
              id="qa-amount"
              // eslint-disable-next-line jsx-a11y/no-autofocus -- the amount is the first thing to type; the modal opens on an explicit action
              autoFocus
              inputMode="numeric"
              value={amount === 0 ? "" : formatVND(amount)}
              placeholder="0"
              onChange={(e) => {
                const digits = e.target.value.replace(/\D/g, "").slice(0, 15);
                setAmount(digits === "" ? 0 : Number(digits));
              }}
              className="w-full bg-transparent text-right text-3xl font-bold tabular-nums outline-none"
              style={{ color: amount > 0 ? "var(--tpl-heading)" : "var(--tpl-muted)" }}
            />
            <span className="text-sm font-semibold" style={{ color: "var(--tpl-muted)" }}>
              đ
            </span>
          </div>

          <div className="mt-3 grid grid-cols-4 gap-1.5">
            {["1", "2", "3", "4", "5", "6", "7", "8", "9", "000", "0", "back"].map((k) => (
              <button
                key={k}
                type="button"
                onClick={() => press(k)}
                aria-label={k === "back" ? "Xoá một chữ số" : k}
                className="grid h-10 place-items-center rounded-lg text-base font-semibold transition hover:opacity-80"
                style={{ background: "var(--tpl-surface)", color: "var(--tpl-text)" }}
              >
                {k === "back" ? "⌫" : k}
              </button>
            ))}
          </div>
        </div>

        {/* ── the rest ───────────────────────────────────────────────── */}
        <div className="space-y-4 p-5">
          {err && (
            <p
              className="rounded-lg border px-3 py-2 text-sm"
              style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
            >
              {err}
            </p>
          )}

          {mode !== "transfer" ? (
            <div>
              <p className="mb-2 text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
                Danh mục
              </p>
              {categories.isPending ? (
                <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
                  Đang tải…
                </p>
              ) : (
                <div className="grid max-h-56 grid-cols-4 gap-2 overflow-y-auto pr-1">
                  {pickable.map((c) => {
                    const on = categoryId === c.id;
                    const parent = parentName(c);
                    return (
                      <button
                        key={c.id}
                        type="button"
                        onClick={() => setCategoryId(c.id)}
                        aria-pressed={on}
                        title={parent ? `${parent} › ${c.name}` : c.name}
                        className="flex flex-col items-center gap-1 rounded-xl border px-1 py-2 transition"
                        style={{
                          borderColor: on ? "var(--tpl-accent)" : "transparent",
                          background: on ? "color-mix(in srgb, var(--tpl-accent) 10%, transparent)" : "transparent",
                        }}
                      >
                        <CategoryChip icon={c.icon} color={c.color} name={c.name} size={38} />
                        <span
                          className="line-clamp-2 text-center text-[11px] leading-tight"
                          style={{ color: on ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
                        >
                          {c.name}
                        </span>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          ) : null}

          <div className="grid grid-cols-2 gap-3">
            <Field label={mode === "transfer" ? "Từ ví" : "Ví"}>
              <Select value={account ?? ""} onChange={setAccountId}>
                {openAccounts.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </Select>
            </Field>

            {mode === "transfer" ? (
              <Field label="Đến ví">
                <Select value={toAccount ?? ""} onChange={setToAccountId}>
                  {openAccounts.map((a) => (
                    <option key={a.id} value={a.id}>
                      {a.name}
                    </option>
                  ))}
                </Select>
              </Field>
            ) : (
              <Field label="Ngày">
                <input
                  type="date"
                  value={date}
                  onChange={(e) => setDate(e.target.value)}
                  className="w-full rounded-lg border px-3 py-2 text-sm"
                  style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
                />
              </Field>
            )}
          </div>

          {mode === "transfer" && (
            <Field label="Ngày">
              <input
                type="date"
                value={date}
                onChange={(e) => setDate(e.target.value)}
                className="w-full rounded-lg border px-3 py-2 text-sm"
                style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
              />
            </Field>
          )}

          <Field label="Ghi chú">
            <input
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder="Không bắt buộc"
              className="w-full rounded-lg border px-3 py-2 text-sm"
              style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
            />
          </Field>

          {openAccounts.length === 0 && !accounts.isPending && (
            <p className="text-sm" style={{ color: "#f59e0b" }}>
              Chưa có ví nào — tạo một ví trước khi ghi giao dịch.
            </p>
          )}

          <button
            type="button"
            onClick={() => save.mutate()}
            disabled={!canSave}
            className="w-full rounded-xl px-4 py-3 text-sm font-bold text-white transition hover:opacity-90 disabled:opacity-40"
            style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
          >
            {save.isPending ? "Đang lưu…" : "Lưu giao dịch"}
          </button>
        </div>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-1 text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
        {label}
      </p>
      {children}
    </div>
  );
}

function Select({
  value,
  onChange,
  children,
}: {
  value: string;
  onChange: (v: string) => void;
  children: React.ReactNode;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="w-full rounded-lg border px-3 py-2 text-sm"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
    >
      {children}
    </select>
  );
}
