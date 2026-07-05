import type { ReactNode } from "react";
import { activeTemplate } from "@/templates/registry";

/**
 * Authenticated routes — wrapped in the active template's app shell.
 *
 * TODO (auth wiring, deferred per v1 scope cut): redirect unauthenticated users
 * to /login here (or in middleware) once the API session check is in place.
 */
export default function AppLayout({ children }: { children: ReactNode }) {
  const Shell = activeTemplate().shells.app;
  return <Shell>{children}</Shell>;
}
