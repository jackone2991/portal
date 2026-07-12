// Data layer for the bank / personal ledger (SPEC-03). Server state is owned by
// TanStack Query (D-32); this module holds the fetch functions + wire types.
//
// Types mirror the actually-wired handler JSON (snake_case), not the camelCase
// OpenAPI schema — same documented spec/handler drift as media-assets.ts. All
// money fields are integer minor units (VND exponent 0, D-41); formatting to
// "1.500.000" happens at the display layer (see components/ui/Money).

import { api } from "./api-client";

export type AccountType = "cash" | "checking" | "savings" | "credit_card" | "ewallet" | "other";
export type CategoryKind = "income" | "expense";
export type Direction = "debit" | "credit";

export interface BankAccount {
  id: string;
  name: string;
  type: AccountType;
  currency: string;
  opening_balance: number;
  balance: number;
  archived: boolean;
  created_at: string;
}

export interface BankCategory {
  id: string;
  parent_id: string | null;
  name: string;
  kind: CategoryKind;
  seed: boolean;
}

export interface BankTransaction {
  id: string;
  account_id: string;
  category_id: string | null;
  amount: number;
  direction: Direction;
  transfer_id: string | null;
  is_transfer: boolean;
  occurred_at: string; // YYYY-MM-DD
  note: string | null;
  created_at: string;
  updated_at: string;
}

export interface BankBudgetLine {
  category_id: string;
  parent_id: string | null;
  name: string;
  parent_name: string | null;
  amount: number;
  spent: number;
}

export interface BankDashboard {
  month: string;
  accounts: BankAccount[];
  income: number;
  expense: number;
  budgets: BankBudgetLine[];
  recent: BankTransaction[];
}

// ── accounts ──────────────────────────────────────────────────────────

export async function listAccounts(): Promise<BankAccount[]> {
  const r = await api<{ accounts: BankAccount[] }>("/api/v1/bank/accounts");
  return r.accounts ?? [];
}

export interface AccountCreate {
  name: string;
  type: AccountType;
  currency?: string;
  opening_balance?: number;
}
export async function createAccount(body: AccountCreate): Promise<BankAccount> {
  return api<BankAccount>("/api/v1/bank/accounts", { method: "POST", body: JSON.stringify(body) });
}
export async function updateAccount(
  id: string,
  body: { name?: string; archived?: boolean; currency?: string },
): Promise<BankAccount> {
  return api<BankAccount>(`/api/v1/bank/accounts/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}
export async function deleteAccount(id: string): Promise<void> {
  await api<void>(`/api/v1/bank/accounts/${id}`, { method: "DELETE" });
}

// ── categories ────────────────────────────────────────────────────────

export async function listCategories(): Promise<BankCategory[]> {
  const r = await api<{ categories: BankCategory[] }>("/api/v1/bank/categories");
  return r.categories ?? [];
}
export async function createCategory(body: {
  name: string;
  kind: CategoryKind;
  parent_id?: string | null;
}): Promise<BankCategory> {
  return api<BankCategory>("/api/v1/bank/categories", { method: "POST", body: JSON.stringify(body) });
}
export async function deleteCategory(id: string, reassignTo?: string): Promise<void> {
  const q = reassignTo ? `?reassign_to=${reassignTo}` : "";
  await api<void>(`/api/v1/bank/categories/${id}${q}`, { method: "DELETE" });
}

// ── transactions ──────────────────────────────────────────────────────

export interface TransactionsPage {
  transactions: BankTransaction[];
  next_cursor?: string | null;
}
export interface ListTxParams {
  account?: string;
  category?: string;
  month?: string;
  cursor?: string;
}
export async function listTransactions(params: ListTxParams = {}): Promise<TransactionsPage> {
  const q = new URLSearchParams();
  if (params.account) q.set("account", params.account);
  if (params.category) q.set("category", params.category);
  if (params.month) q.set("month", params.month);
  if (params.cursor) q.set("cursor", params.cursor);
  const qs = q.toString();
  const r = await api<TransactionsPage>(`/api/v1/bank/transactions${qs ? `?${qs}` : ""}`);
  return { transactions: r.transactions ?? [], next_cursor: r.next_cursor };
}
export interface TransactionCreate {
  account_id: string;
  category_id: string;
  amount: number;
  direction: Direction;
  occurred_at?: string;
  note?: string | null;
}
export async function createTransaction(body: TransactionCreate): Promise<BankTransaction> {
  return api<BankTransaction>("/api/v1/bank/transactions", { method: "POST", body: JSON.stringify(body) });
}
export async function deleteTransaction(id: string): Promise<void> {
  await api<void>(`/api/v1/bank/transactions/${id}`, { method: "DELETE" });
}

// ── transfers ─────────────────────────────────────────────────────────

export async function createTransfer(body: {
  from_account: string;
  to_account: string;
  amount: number;
  occurred_at?: string;
  note?: string | null;
}): Promise<{ transfer_id: string | null; legs: BankTransaction[] }> {
  return api("/api/v1/bank/transfers", { method: "POST", body: JSON.stringify(body) });
}
export async function deleteTransfer(transferId: string): Promise<void> {
  await api<void>(`/api/v1/bank/transfers/${transferId}`, { method: "DELETE" });
}

// ── budgets ───────────────────────────────────────────────────────────

export interface BudgetsResponse {
  month: string;
  budgets: BankBudgetLine[];
}
export async function listBudgets(month?: string): Promise<BudgetsResponse> {
  const q = month ? `?month=${month}` : "";
  const r = await api<BudgetsResponse>(`/api/v1/bank/budgets${q}`);
  return { month: r.month, budgets: r.budgets ?? [] };
}
export async function setBudget(body: { category_id: string; month: string; amount: number | null }): Promise<void> {
  await api<void>("/api/v1/bank/budgets", { method: "PUT", body: JSON.stringify(body) });
}

// ── dashboard ─────────────────────────────────────────────────────────

export async function getDashboard(month?: string): Promise<BankDashboard> {
  const q = month ? `?month=${month}` : "";
  return api<BankDashboard>(`/api/v1/bank/dashboard${q}`);
}

// ── money helpers (VND exponent 0) ────────────────────────────────────

/** Format integer minor units as VND thousands: 1500000 → "1.500.000". */
export function formatVND(minor: number): string {
  const neg = minor < 0;
  const digits = Math.abs(minor).toString();
  const grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return (neg ? "-" : "") + grouped;
}

/** Parse a thousands-separated string back to integer minor units. "" → 0. */
export function parseVND(s: string): number {
  const cleaned = s.replace(/[^\d-]/g, "");
  if (cleaned === "" || cleaned === "-") return 0;
  return parseInt(cleaned, 10) || 0;
}

/** Current month as YYYY-MM. */
export function currentMonth(): string {
  return new Date().toISOString().slice(0, 7);
}
