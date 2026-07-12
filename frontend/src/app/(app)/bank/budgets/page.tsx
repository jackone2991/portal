import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Budgets" };

/** /bank/budgets — month picker + per-category budget tree (SPEC-03 P0.5). */
export default function BankBudgetsPage() {
  const View = activeTemplate().views.bankBudgets;
  return <View />;
}
