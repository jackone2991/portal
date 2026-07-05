import type { ShellProps } from "@/templates/types";
import { HelloPreloader } from "../partials/HelloPreloader";
import { GoToTop } from "../partials/GoToTop";
import { SidebarLeft } from "../components/menu/SidebarLeft";
import { SidebarRight } from "../components/menu/SidebarRight";
import { SidebarCenter } from "../components/menu/SidebarCenter";
import { SvgSprite } from "../components/footers/SvgSprite";
import { UpdateHeaderPhoto } from "../components/popup/UpdateHeaderPhoto";
import { ChoseFromMyPhoto } from "../components/popup/ChoseFromMyPhoto";
import { ChatResponsive } from "../components/popup/ChatResponsive";

/**
 * Authenticated app shell — port of `master/master-base.blade.php`.
 *
 * Blade composition reproduced:
 *   body-header → hellopreloader + sidebar{Left,Right,Center(+Responsive)}
 *   body-content → {children}
 *   popup       → goToTop + updateHeaderPhoto + choseFromMyPhoto + chatResponsive
 *   body-scripts→ footers.svg (sprite) [footers.js/ico handled by Next]
 */
export function MasterBase({ children }: ShellProps) {
  return (
    <div data-template="v1" className="min-h-screen">
      <HelloPreloader />

      {/* @section('body-header') */}
      <SidebarCenter />
      <SidebarLeft />
      <SidebarRight />
      <div className="header-spacer" style={{ height: "var(--tpl-header-h)" }} />

      {/* @section('body-content') */}
      <main className="mx-auto w-full max-w-6xl px-4 py-6 lg:pl-[calc(var(--tpl-sidebar-w)+1rem)]">
        {children}
      </main>

      {/* @section('popup') */}
      <GoToTop />
      <UpdateHeaderPhoto />
      <ChoseFromMyPhoto />
      <ChatResponsive />

      {/* @section('body-scripts') — inline svg sprite */}
      <SvgSprite />
    </div>
  );
}
