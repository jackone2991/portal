import { TopMenu } from "../headers/TopMenu";

/**
 * Fixed top bar — port of `components/menu/sidebarCenter.blade.php`.
 * The dark Olympus header that spans the full width above everything.
 */
export function SidebarCenter() {
  return (
    <header
      className="fixed inset-x-0 top-0 z-40 shadow-sm"
      style={{ height: "var(--tpl-header-h)", background: "var(--tpl-header)" }}
    >
      <TopMenu />
    </header>
  );
}
