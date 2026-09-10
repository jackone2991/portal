import { FinanceWidget, DSQuerySeed } from "portal-frontend";

// This month's ledger summary on the home rail: income, expense, and a link into
// the full ledger. Reads ["bank","dashboard","home"] and returns null without
// data, so the preview seeds a dashboard payload.
//
// Amounts are integer minor units on the wire (D-41) — MoneyDisplay formats them.

const dashboard = (income: number, expense: number) => ({
  month: "2026-09",
  accounts: [],
  income,
  expense,
  budgets: [],
  recent: [],
});

const frame = (seed: unknown, children: React.ReactNode) => (
  <DSQuerySeed seed={[[["bank", "dashboard", "home"], seed]]}>
    <div data-template="v1" style={{ width: 320, background: "var(--tpl-bg)", padding: 12 }}>{children}</div>
  </DSQuerySeed>
);

export const Surplus = () => frame(dashboard(31_000_000, 11_030_000), <FinanceWidget />);

// A month that spent more than it earned — the colours are the whole point of
// the widget, so the overspent case deserves its own card.
export const Overspent = () => frame(dashboard(24_000_000, 25_800_000), <FinanceWidget />);

// A fresh ledger: zeroes are a real state, not a missing one.
export const NoMovement = () => frame(dashboard(0, 0), <FinanceWidget />);
