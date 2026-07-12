import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Ledger" };

/** /bank — the ledger dashboard (SPEC-03 P0.6). */
export default function BankDashboardPage() {
  const View = activeTemplate().views.bankDashboard;
  return <View />;
}
