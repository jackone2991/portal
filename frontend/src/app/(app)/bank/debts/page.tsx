import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Nợ & cho vay" };

export default function BankDebtsPage() {
  const View = activeTemplate().views.bankDebts;
  return <View />;
}
