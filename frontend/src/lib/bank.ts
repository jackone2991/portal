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
  /** An emoji. Null is ordinary — the UI falls back to a letter chip. */
  icon: string | null;
  /** "#rrggbb". Null falls back to a neutral tone. */
  color: string | null;
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
  icon?: string | null;
  color?: string | null;
}): Promise<BankCategory> {
  return api<BankCategory>("/api/v1/bank/categories", { method: "POST", body: JSON.stringify(body) });
}
/**
 * Patch a category. Every field follows present-vs-absent: sending `icon: null`
 * CLEARS it, omitting `icon` leaves it — so build the body with only the keys
 * you mean to change.
 */
export async function updateCategory(
  id: string,
  body: { name?: string; parent_id?: string | null; icon?: string | null; color?: string | null },
): Promise<BankCategory> {
  return api<BankCategory>(`/api/v1/bank/categories/${id}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
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

// ── report ────────────────────────────────────────────────────────────

export interface BankCategoryTotal {
  /** The TOP-LEVEL category this total rolls up to (children are folded in). */
  category_id: string;
  name: string;
  kind: CategoryKind;
  icon: string | null;
  color: string | null;
  total: number;
  tx_count: number;
}

export interface BankMonthFlow {
  month: string; // YYYY-MM
  income: number;
  expense: number;
}

export interface BankReport {
  month: string;
  income: number;
  expense: number;
  expenses: BankCategoryTotal[];
  incomes: BankCategoryTotal[];
  trend: BankMonthFlow[];
}

export async function getReport(month?: string): Promise<BankReport> {
  const q = month ? `?month=${month}` : "";
  const r = await api<BankReport>(`/api/v1/bank/report${q}`);
  return { ...r, expenses: r.expenses ?? [], incomes: r.incomes ?? [], trend: r.trend ?? [] };
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

/** Today as YYYY-MM-DD in LOCAL time — `toISOString` would shift the date in
 *  any timezone east of UTC, logging an evening expense onto tomorrow. */
export function today(): string {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

/** Shift a YYYY-MM by n months. */
export function shiftMonth(month: string, n: number): string {
  const [y, m] = month.split("-").map(Number);
  const d = new Date(Date.UTC(y ?? 1970, (m ?? 1) - 1 + n, 1));
  return d.toISOString().slice(0, 7);
}

const MONTH_LABELS = [
  "Tháng 1", "Tháng 2", "Tháng 3", "Tháng 4", "Tháng 5", "Tháng 6",
  "Tháng 7", "Tháng 8", "Tháng 9", "Tháng 10", "Tháng 11", "Tháng 12",
];

/** "2026-08" → "Tháng 8, 2026". */
export function monthLabel(month: string): string {
  const [y, m] = month.split("-").map(Number);
  const name = MONTH_LABELS[(m ?? 1) - 1] ?? month;
  return `${name}, ${y}`;
}

/** "2026-08" → "T8" for compact chart axes. */
export function shortMonthLabel(month: string): string {
  return `T${Number(month.split("-")[1] ?? 0)}`;
}

const WEEKDAYS = ["Chủ nhật", "Thứ hai", "Thứ ba", "Thứ tư", "Thứ năm", "Thứ sáu", "Thứ bảy"];

/**
 * Day heading for the grouped transaction list: "Hôm nay", "Hôm qua", or
 * "Thứ tư, 12/08". Parsed as local parts rather than `new Date(iso)`, which
 * would read a bare YYYY-MM-DD as UTC midnight and show the previous day here.
 */
export function dayLabel(isoDate: string): string {
  const [y, m, d] = isoDate.split("-").map(Number);
  if (!y || !m || !d) return isoDate;
  const date = new Date(y, m - 1, d);
  const now = new Date();
  const midnight = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const days = Math.round((midnight.getTime() - date.getTime()) / 86_400_000);
  if (days === 0) return "Hôm nay";
  if (days === 1) return "Hôm qua";
  const wd = WEEKDAYS[date.getDay()] ?? "";
  return `${wd}, ${String(d).padStart(2, "0")}/${String(m).padStart(2, "0")}`;
}

/** Signed amount for display: a debit leaves the wallet. */
export function signedAmount(t: { amount: number; direction: Direction }): number {
  return t.direction === "debit" ? -t.amount : t.amount;
}
