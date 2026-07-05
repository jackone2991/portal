import { TopMenu } from "../headers/TopMenu";

/**
 * Fixed top bar — port of `components/menu/sidebarCenter.blade.php`
 * (+ `sidebarCenterResponsive.blade.php`, collapsed into one responsive bar).
 */
export function SidebarCenter() {
  return (
    <header
      className="fixed inset-x-0 top-0 z-40 border-b backdrop-blur"
      style={{
        height: "var(--tpl-header-h)",
        borderColor: "var(--tpl-border)",
        background: "rgba(0,0,0,0.4)",
      }}
    >
      <TopMenu />
    </header>
  );
}
