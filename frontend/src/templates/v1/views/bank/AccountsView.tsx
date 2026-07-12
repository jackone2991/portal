"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  type AccountType,
  createAccount,
  deleteAccount,
  listAccounts,
  updateAccount,
} from "@/lib/bank";
import { problemDisplayMessage } from "@/lib/problems";
import { ApiError } from "@/lib/api-client";
import { MoneyDisplay, MoneyInput } from "../../components/ui/Money";

const TYPES: AccountType[] = ["cash", "checking", "savings", "credit_card", "ewallet", "other"];

export function AccountsView() {
  const qc = useQueryClient();
  const { data: accounts = [], isLoading } = useQuery({ queryKey: ["bank", "accounts"], queryFn: listAccounts });

  const [name, setName] = useState("");
  const [type, setType] = useState<AccountType>("cash");
  const [currency, setCurrency] = useState("VND");
  const [opening, setOpening] = useState(0);
  const [err, setErr] = useState<string | null>(null);

  const invalidate = () => qc.invalidateQueries({ queryKey: ["bank"] });

  const create = useMutation({
    mutationFn: () => createAccount({ name, type, currency, opening_balance: opening }),
    onSuccess: () => {
      setName("");
      setOpening(0);
      setErr(null);
      invalidate();
    },
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not create account"),
  });
  const archive = useMutation({
    mutationFn: (a: { id: string; archived: boolean }) => updateAccount(a.id, { archived: a.archived }),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteAccount(id),
    onSuccess: invalidate,
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not delete"),
  });

  const active = accounts.filter((a) => !a.archived);
  const archived = accounts.filter((a) => a.archived);

  return (
    <main className="mx-auto max-w-3xl p-6 text-white">
      <h1 className="mb-6 text-2xl font-bold">Accounts</h1>

      <form
        className="mb-8 grid grid-cols-2 gap-3 rounded-lg border border-gray-800 bg-gray-900 p-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim()) create.mutate();
        }}
      >
        <input
          className="col-span-2 rounded-md border border-gray-700 bg-gray-800 px-3 py-2"
          placeholder="Account name (e.g. TCB, Cash, Momo)"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <select className="rounded-md border border-gray-700 bg-gray-800 px-3 py-2" value={type} onChange={(e) => setType(e.target.value as AccountType)}>
          {TYPES.map((t) => (
            <option key={t} value={t}>
              {t.replace("_", " ")}
            </option>
          ))}
        </select>
        <input
          className="rounded-md border border-gray-700 bg-gray-800 px-3 py-2 uppercase"
          value={currency}
          maxLength={3}
          onChange={(e) => setCurrency(e.target.value.toUpperCase())}
        />
        <label className="col-span-2 text-sm text-gray-400">Opening balance</label>
        <div className="col-span-2">
          <MoneyInput value={opening} onChange={setOpening} />
        </div>
        <button
          type="submit"
          disabled={create.isPending || !name.trim()}
          className="col-span-2 rounded-md bg-blue-600 py-2 font-medium hover:bg-blue-500 disabled:opacity-50"
        >
          Add account
        </button>
        {err && <p className="col-span-2 text-sm text-red-400">{err}</p>}
      </form>

      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : (
        <>
          <ul className="space-y-2">
            {active.map((a) => (
              <li key={a.id} className="flex items-center justify-between rounded-lg border border-gray-800 bg-gray-900 p-4">
                <div>
                  <div className="font-medium">{a.name}</div>
                  <div className="text-xs capitalize text-gray-400">{a.type.replace("_", " ")}</div>
                </div>
                <div className="flex items-center gap-4">
                  <MoneyDisplay amount={a.balance} currency={a.currency} className="font-semibold" />
                  <button className="text-xs text-gray-400 hover:text-white" onClick={() => archive.mutate({ id: a.id, archived: true })}>
                    Archive
                  </button>
                  <button className="text-xs text-red-400 hover:text-red-300" onClick={() => remove.mutate(a.id)}>
                    Delete
                  </button>
                </div>
              </li>
            ))}
            {active.length === 0 && <p className="text-gray-500">No accounts yet — add your first above.</p>}
          </ul>

          {archived.length > 0 && (
            <>
              <h2 className="mb-2 mt-8 text-sm font-semibold uppercase text-gray-500">Archived</h2>
              <ul className="space-y-2 opacity-60">
                {archived.map((a) => (
                  <li key={a.id} className="flex items-center justify-between rounded-lg border border-gray-800 bg-gray-900/50 p-3">
                    <span>{a.name}</span>
                    <div className="flex items-center gap-4">
                      <MoneyDisplay amount={a.balance} currency={a.currency} />
                      <button className="text-xs text-gray-400 hover:text-white" onClick={() => archive.mutate({ id: a.id, archived: false })}>
                        Unarchive
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            </>
          )}
        </>
      )}
    </main>
  );
}
