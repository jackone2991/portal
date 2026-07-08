// Design-sync shim: next/link renders a Next router-context-bound anchor that
// blank-renders outside a Next app. In the design tool each card renders
// standalone, so alias next/link -> this plain <a> (see tsconfig.ds.json paths).
import * as React from "react";

type Href = string | { pathname?: string };

export default function Link({
  href,
  children,
  ...rest
}: { href?: Href; children?: React.ReactNode } & Record<string, unknown>) {
  const to = typeof href === "string" ? href : href?.pathname ?? "#";
  return React.createElement("a", { href: to, ...rest }, children as React.ReactNode);
}
