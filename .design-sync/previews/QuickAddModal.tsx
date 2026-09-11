import { QuickAddModal, DSQuerySeed } from "portal-frontend";

// The ledger's quick-entry dialog. Fixed overlay, so it is contained to the card
// with a transformed relative box (same treatment as the other popups).
//
// It reads accounts and categories itself, so both caches are seeded — unseeded,
// the selects render empty and the card teaches the wrong thing about the form.
//
// minHeight is sized so "Lưu giao dịch" stays inside the card: a form preview
// that crops its own submit button teaches an incomplete composition.

const accounts = [
  { id: "a1", name: "Vietcombank", type: "bank", currency: "VND", opening_balance: 0, balance: 32_400_000, archived: false, created_at: "2026-01-04T00:00:00Z" },
  { id: "a2", name: "Tiền mặt", type: "cash", currency: "VND", opening_balance: 0, balance: 1_250_000, archived: false, created_at: "2026-01-04T00:00:00Z" },
  { id: "a3", name: "Momo", type: "ewallet", currency: "VND", opening_balance: 0, balance: 480_000, archived: false, created_at: "2026-02-11T00:00:00Z" },
] as never;

const categories = [
  { id: "c1", parent_id: null, name: "Ăn uống", kind: "expense", seed: true, icon: "🍜", color: "#f97316" },
  { id: "c2", parent_id: "c1", name: "Cà phê", kind: "expense", seed: false, icon: "☕", color: "#a16207" },
  { id: "c3", parent_id: null, name: "Đi lại", kind: "expense", seed: true, icon: "🏍️", color: "#0ea5e9" },
  { id: "c4", parent_id: null, name: "Nhà cửa", kind: "expense", seed: true, icon: "🏠", color: "#8b5cf6" },
  { id: "c5", parent_id: null, name: "Lương", kind: "income", seed: true, icon: "💰", color: "#22c55e" },
  { id: "c6", parent_id: null, name: "Thưởng", kind: "income", seed: false, icon: "🎁", color: "#eab308" },
] as never;

const seeded = (children: React.ReactNode) => (
  <DSQuerySeed seed={[[["bank", "accounts"], accounts], [["bank", "categories"], categories]]}>
    <div data-template="v1" style={{ position: "relative", transform: "translateZ(0)", minHeight: 780, background: "var(--tpl-bg)" }}>
      {children}
    </div>
  </DSQuerySeed>
);

export const Expense = () => seeded(<QuickAddModal onClose={() => {}} />);

// Income flips the category list to the income side — the same form, a different
// half of the taxonomy.
export const Income = () => seeded(<QuickAddModal onClose={() => {}} defaultMode="income" />);

// A transfer takes two accounts and no category: it is a movement, not a spend.
export const Transfer = () => seeded(<QuickAddModal onClose={() => {}} defaultMode="transfer" />);
