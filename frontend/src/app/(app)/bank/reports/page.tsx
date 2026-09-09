import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Báo cáo" };

/** /bank/reports — the month breakdown (donut + trend) over /bank/report. */
export default function BankReportsPage() {
  const View = activeTemplate().views.bankReports;
  return <View />;
}
