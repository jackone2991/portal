"use client";

import { useState, type CSSProperties } from "react";
import type { ShellProps } from "@/templates/types";
import { HelloPreloader } from "../partials/HelloPreloader";
import { GoToTop } from "../partials/GoToTop";
import { SessionKeeper } from "../partials/SessionKeeper";
import { SidebarLeft } from "../components/menu/SidebarLeft";
import { SidebarRight } from "../components/menu/SidebarRight";
import { SidebarCenter } from "../components/menu/SidebarCenter";
import { SvgSprite } from "../components/footers/SvgSprite";
import { UpdateHeaderPhoto } from "../components/popup/UpdateHeaderPhoto";
import { ChoseFromMyPhoto } from "../components/popup/ChoseFromMyPhoto";
import { ChatResponsive } from "../components/popup/ChatResponsive";
import { MusicPlayerProvider } from "../components/music/MusicPlayerProvider";
import { NowPlayingBar, NowPlayingSpacer } from "../components/music/NowPlayingBar";

/**
 * Authenticated app shell — port of `master/master-base.blade.php` (Olympus).
 *
 *   fixed dark header (SidebarCenter) with the brand block above the menu
 *   fixed expandable left menu (SidebarLeft) + right avatar rail (SidebarRight)
 *   light content area between them, holding the page {children}
 *
 * The left menu collapses to an icon rail. Its current width lives in the CSS
 * custom property `--tpl-sidebar-cur`, which the header brand block and the
 * content padding both read — so everything shifts in lockstep.
 */
export function MasterBase({ children }: ShellProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);

  const rootStyle = {
    background: "var(--tpl-bg)",
    "--tpl-sidebar-cur": collapsed ? "var(--tpl-rail-w)" : "var(--tpl-sidebar-w)",
    "--tpl-rightbar-cur": rightCollapsed ? "var(--tpl-rail-w)" : "var(--tpl-rightbar-w)",
  } as CSSProperties;

  return (
    <MusicPlayerProvider>
    <div data-template="v1" className="min-h-screen" style={rootStyle}>
      <HelloPreloader />
      <SessionKeeper />


      {/* chrome */}
      <SidebarCenter />
      <SidebarLeft collapsed={collapsed} onToggle={() => setCollapsed((c) => !c)} />
      <SidebarRight collapsed={rightCollapsed} onToggle={() => setRightCollapsed((c) => !c)} />

      {/* content */}
      <main
        className="min-h-screen transition-[padding] duration-200 xl:pl-[var(--tpl-sidebar-cur)] xl:pr-[var(--tpl-rightbar-cur)]"
        style={{ paddingTop: "var(--tpl-header-h)", color: "var(--tpl-text)" }}
      >
        <div className="mx-auto w-full max-w-[1220px] px-3 py-5 sm:px-5">
          {children}
          <NowPlayingSpacer />
        </div>
      </main>

      {/* floating + modals */}
      <NowPlayingBar />
      <GoToTop />
      <UpdateHeaderPhoto />
      <ChoseFromMyPhoto />
      <ChatResponsive />
      <SvgSprite />
    </div>
    </MusicPlayerProvider>
  );
}
