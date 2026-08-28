import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Menu & Widget" };

export default function AdminLayoutPage() {
  const View = activeTemplate().views.adminLayout;
  return <View />;
}
