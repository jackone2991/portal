import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Vai trò & quyền" };

export default function AdminRolesPage() {
  const View = activeTemplate().views.adminRoles;
  return <View />;
}
