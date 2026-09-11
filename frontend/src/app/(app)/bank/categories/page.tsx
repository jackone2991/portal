import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Danh mục" };

export default function BankCategoriesPage() {
  const View = activeTemplate().views.bankCategories;
  return <View />;
}
