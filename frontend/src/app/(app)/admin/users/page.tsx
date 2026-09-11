import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Người dùng" };

export default function AdminUsersPage() {
  const View = activeTemplate().views.adminUsers;
  return <View />;
}
