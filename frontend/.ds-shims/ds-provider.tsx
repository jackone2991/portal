// Design-sync render provider. Icon (and everything composing it: TopMenu,
// the sidebars, views) renders via <use href="#olymp-*"> against symbols that
// <SvgSprite/> injects. Each design-tool card renders standalone, so every
// preview must carry the sprite — cfg.provider mounts this around every card.
import * as React from "react";
import { SvgSprite } from "../src/templates/v1/components/footers/SvgSprite";

// Several Olympus components read `process.env.NEXT_PUBLIC_API_BASE_URL` at
// module top level; in a browser IIFE `process` is undefined and the whole
// bundle throws before assigning window.PortalUI. This provider entry is
// bundled ahead of the component modules (converter emits extraEntries first),
// so defining `process` here runs before any component evaluates. Idempotent.
(globalThis as unknown as { process?: { env: Record<string, string | undefined> } }).process ??= {
  env: {},
};

export function DSProvider({ children }: { children?: React.ReactNode }) {
  return React.createElement(
    React.Fragment,
    null,
    React.createElement(SvgSprite, null),
    children as React.ReactNode,
  );
}
