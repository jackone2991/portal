import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Transactions" };

/** /bank/transactions — quick-add + filterable list (SPEC-03 P0.2). */
export default function BankTransactionsPage() {
  const View = activeTemplate().views.bankTransactions;
  return <View />;
}
