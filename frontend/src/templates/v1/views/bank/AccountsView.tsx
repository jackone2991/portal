"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  createAccount,
  deleteAccount,
  formatVND,
  listAccounts,
  updateAccount,
  type AccountType,
  type BankAccount,
} from "@/lib/bank";
import { MoneyInput } from "../../components/ui/Money";
import { Icon } from "../../components/ui/Icon";
import { BackLink } from "../../components/bank/BackLink";
import { ACCOUNT_TYPE_LABEL } from "./DashboardView";

const TYPES: AccountType[] = ["cash", "checking", "savings", "credit_card", "ewallet", "other"];

const TYPE_ICON: Record<string, string> = {
  cash: "💵",
  checking: "🏦",
  savings: "🐷",
  credit_card: "💳",
  ewallet: "📱",
  other: "📁",
};

/**
 * Wallets — the accounts money moves between.
 *
 * A balance here is always DERIVED (opening balance plus every transaction), never
 * stored, so it cannot drift from the ledger. That is also why the opening balance
 * is only settable at creation: editing it later would silently rewrite history for
 * every month already reported.
 *
 * Archive is offered before delete because delete is refused once an account has
 * transactions — an emptied wallet you no longer use should disappear from the
 * pickers without taking its history with it.
 */
export function AccountsView() {
  const qc = useQueryClient();
  const { data: accounts = [], isPending } = useQuery({ queryKey: ["bank", "accounts"], queryFn: listAccounts });

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [type, setType] = useState<AccountType>("cash");
  const [opening, setOpening] = useState(0);
  const [err, setErr] = useState<string | null>(null);

  const invalidate = () => qc.invalidateQueries({ queryKey: ["bank"] });

  const create = useMutation({
    mutationFn: () => createAccount({ name: name.trim(), type, currency: "VND", opening_balance: opening }),
    onSuccess: () => {
      setName("");
      setOpening(0);
      setErr(null);
      setOpen(false);
      invalidate();
    },
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không tạo được ví."),
  });
  const archive = useMutation({
    mutationFn: (a: { id: string; archived: boolean }) => updateAccount(a.id, { archived: a.archived }),
    onMutate: () => setErr(null),
    onSuccess: invalidate,
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Không đổi được trạng thái ví."),
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteAccount(id),
    onMutate: () => setErr(null),
    onSuccess: invalidate,
    onError: (e) =>
      setErr(
        e instanceof ApiError
          ? problemDisplayMessage(e.body)
          : "Không xoá được ví — ví đã có giao dịch thì chỉ có thể lưu trữ.",
      ),
  });

  const active = accounts.filter((a) => !a.archived);
  const archived = accounts.filter((a) => a.archived);
  const total = active.reduce((s, a) => s + a.balance, 0);

  return (
    <section className="pb-8">
      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <BackLink />
          <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
            Ví của tôi
          </h1>
        </div>
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          className="rounded-lg px-4 py-2 text-sm font-bold text-white transition hover:opacity-90"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          {open ? "Huỷ" : "+ Thêm ví"}
        </button>
      </header>

      {active.length > 0 && (
        <div
          className="mb-4 rounded-2xl border p-5"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          <p className="text-xs font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
            Tổng số dư
          </p>
          <p className="mt-1 text-2xl font-bold tabular-nums" style={{ color: "var(--tpl-heading)" }}>
            {formatVND(total)} <span className="text-base font-semibold" style={{ color: "var(--tpl-muted)" }}>đ</span>
          </p>
        </div>
      )}

      {err && (
        <p
          className="mb-3 rounded-lg border px-3 py-2 text-sm"
          style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
        >
          {err}
        </p>
      )}

      {open && (
        <form
          className="mb-5 grid gap-3 rounded-2xl border p-5 sm:grid-cols-2"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim()) create.mutate();
          }}
        >
          <div className="sm:col-span-2">
            <Label htmlFor="acct-name">Tên ví</Label>
            <input
              id="acct-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Ví dụ: Tiền mặt, Techcombank, Momo"
              className="w-full rounded-lg border px-3 py-2 text-sm"
              style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
            />
          </div>
          <div>
            <Label htmlFor="acct-type">Loại</Label>
            <select
              id="acct-type"
              value={type}
              onChange={(e) => setType(e.target.value as AccountType)}
              className="w-full rounded-lg border px-3 py-2 text-sm"
              style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
            >
              {TYPES.map((t) => (
                <option key={t} value={t}>
                  {TYPE_ICON[t]} {ACCOUNT_TYPE_LABEL[t]}
                </option>
              ))}
            </select>
          </div>
          <div>
            <Label htmlFor="acct-opening">Số dư ban đầu</Label>
            <MoneyInput
              id="acct-opening"
              value={opening}
              onChange={setOpening}
              className="w-full rounded-lg border px-3 py-2 text-right text-sm tabular-nums outline-none"
            />
          </div>
          <p className="text-xs sm:col-span-2" style={{ color: "var(--tpl-muted)" }}>
            Số dư ban đầu chỉ đặt được lúc tạo — sau đó số dư được tính từ giao dịch, nên sửa lại
            sẽ làm sai lệch các tháng đã chốt.
          </p>
          <div className="sm:col-span-2">
            <button
              type="submit"
              disabled={!name.trim() || create.isPending}
              className="rounded-lg px-4 py-2 text-sm font-bold text-white transition disabled:opacity-40"
              style={{ background: "var(--tpl-accent)" }}
            >
              {create.isPending ? "Đang tạo…" : "Tạo ví"}
            </button>
          </div>
        </form>
      )}

      {isPending ? (
        <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
          Đang tải…
        </p>
      ) : accounts.length === 0 ? (
        <div
          className="rounded-2xl border border-dashed py-12 text-center text-sm"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
        >
          Chưa có ví nào — bấm “Thêm ví” để tạo ví đầu tiên.
        </div>
      ) : (
        <ul
          className="divide-y rounded-2xl border"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          {active.map((a) => (
            <AccountRow
              key={a.id}
              account={a}
              onArchive={() => archive.mutate({ id: a.id, archived: true })}
              onDelete={() => remove.mutate(a.id)}
            />
          ))}
        </ul>
      )}

      {archived.length > 0 && (
        <>
          <h2 className="mb-2 mt-6 text-sm font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
            Đã lưu trữ
          </h2>
          <ul
            className="divide-y rounded-2xl border opacity-60"
            style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
          >
            {archived.map((a) => (
              <AccountRow
                key={a.id}
                account={a}
                onRestore={() => archive.mutate({ id: a.id, archived: false })}
                onDelete={() => remove.mutate(a.id)}
              />
            ))}
          </ul>
        </>
      )}
    </section>
  );
}

function AccountRow({
  account,
  onArchive,
  onRestore,
  onDelete,
}: {
  account: BankAccount;
  onArchive?: () => void;
  onRestore?: () => void;
  onDelete: () => void;
}) {
  return (
    <li className="flex items-center gap-3 px-4 py-3" style={{ borderColor: "var(--tpl-border)" }}>
      <span
        className="grid h-10 w-10 shrink-0 place-items-center rounded-full text-lg"
        style={{ background: "var(--tpl-surface-2)" }}
      >
        {TYPE_ICON[account.type] ?? "📁"}
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-text)" }}>
          {account.name}
        </p>
        <p className="text-[11px]" style={{ color: "var(--tpl-muted)" }}>
          {ACCOUNT_TYPE_LABEL[account.type] ?? account.type} · {account.currency}
        </p>
      </div>
      <span
        className="shrink-0 text-sm font-bold tabular-nums"
        style={{ color: account.balance < 0 ? "#ef4444" : "var(--tpl-heading)" }}
      >
        {formatVND(account.balance)}
      </span>
      <div className="flex shrink-0 items-center gap-2">
        {onArchive && (
          <button
            type="button"
            onClick={onArchive}
            className="text-xs font-semibold transition hover:opacity-80"
            style={{ color: "var(--tpl-accent)" }}
          >
            Lưu trữ
          </button>
        )}
        {onRestore && (
          <button
            type="button"
            onClick={onRestore}
            className="text-xs font-semibold transition hover:opacity-80"
            style={{ color: "var(--tpl-accent)" }}
          >
            Khôi phục
          </button>
        )}
        <button
          type="button"
          onClick={onDelete}
          aria-label={`Xoá ví ${account.name}`}
          className="transition hover:text-[#ef4444]"
          style={{ color: "var(--tpl-muted)" }}
        >
          <Icon name="little-delete" size={15} />
        </button>
      </div>
    </li>
  );
}

function Label({ htmlFor, children }: { htmlFor: string; children: React.ReactNode }) {
  return (
    <label htmlFor={htmlFor} className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
      {children}
    </label>
  );
}
