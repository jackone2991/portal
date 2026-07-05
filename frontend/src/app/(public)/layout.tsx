import type { ReactNode } from "react";
import { activeTemplate } from "@/templates/registry";

/** Guest routes — wrapped in the active template's public shell. */
export default function PublicLayout({ children }: { children: ReactNode }) {
  const Shell = activeTemplate().shells.public;
  return <Shell>{children}</Shell>;
}
