import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Accounts" };

/** /bank/accounts — list + create/archive (SPEC-03 P0.1). */
export default function BankAccountsPage() {
  const View = activeTemplate().views.bankAccounts;
  return <View />;
}
