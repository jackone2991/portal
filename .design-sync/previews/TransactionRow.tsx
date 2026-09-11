import { TransactionRow } from "portal-frontend";

// One line of the ledger. The category chip carries the colour, the amount
// carries the sign, and a transfer leg is deliberately NOT shown as an
// uncategorised expense — it is a movement between accounts, and counting it as
// a spend would double-count the month.

const tx = (over: Partial<Record<string, unknown>> = {}) => ({
  id: "t1",
  account_id: "a1",
  category_id: "c1",
  amount: 85_000,
  direction: "debit" as const,
  transfer_id: null,
  is_transfer: false,
  occurred_at: "2026-08-27",
  note: null,
  created_at: "2026-08-27T09:12:00Z",
  updated_at: "2026-08-27T09:12:00Z",
  ...over,
}) as never;

const cat = (name: string, icon: string | null, color: string | null) => ({ name, icon, color });

const frame = (children: React.ReactNode) => (
  <div data-template="v1" className="divide-y rounded-xl" style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)", borderColor: "var(--tpl-border)", width: 460 }}>
    {children}
  </div>
);

export const Expense = () =>
  frame(
    <>
      <TransactionRow tx={tx({ note: "Bún bò sáng" })} category={cat("Ăn uống", "🍜", "#f97316")} accountName="Tiền mặt" />
      <TransactionRow tx={tx({ id: "t2", amount: 45_000, note: null })} category={cat("Cà phê", "☕", "#a16207")} accountName="Momo" />
      <TransactionRow tx={tx({ id: "t3", amount: 1_200_000, note: "Tiền điện tháng 8" })} category={cat("Nhà cửa", "🏠", "#8b5cf6")} accountName="Vietcombank" />
    </>,
  );

export const IncomeAndUncategorised = () =>
  frame(
    <>
      <TransactionRow tx={tx({ id: "t4", amount: 24_000_000, direction: "credit", note: "Lương tháng 8" })} category={cat("Lương", "💰", "#22c55e")} accountName="Vietcombank" />
      <TransactionRow tx={tx({ id: "t5", amount: 320_000, category_id: null })} category={null} accountName="Tiền mặt" />
    </>,
  );

// A transfer leg reads as "Chuyển khoản" with no category chip colour of its own.
export const Transfer = () =>
  frame(
    <>
      <TransactionRow tx={tx({ id: "t6", amount: 5_000_000, is_transfer: true, transfer_id: "tr1", category_id: null, note: "Rút về ví" })} category={null} accountName="Vietcombank" />
      <TransactionRow tx={tx({ id: "t7", amount: 11_000, is_transfer: true, transfer_id: "tr1", note: "Phí chuyển" })} category={cat("Phí ngân hàng", "🏦", "#64748b")} accountName="Vietcombank" />
    </>,
  );

// With the delete affordance the list view passes down.
export const Deletable = () =>
  frame(
    <TransactionRow tx={tx({ note: "Xem lại khoản này" })} category={cat("Giải trí", "🎬", "#ec4899")} accountName="Momo" onDelete={() => {}} />,
  );
