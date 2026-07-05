import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Register" };

export default function RegisterPage() {
  const View = activeTemplate().views.register;
  return <View />;
}
