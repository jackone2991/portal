import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Login" };

export default function LoginPage() {
  const View = activeTemplate().views.login;
  return <View />;
}
