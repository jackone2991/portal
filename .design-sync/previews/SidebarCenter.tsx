import { SidebarCenter } from "portal-frontend";

// Top header bar — port of Olympus `sidebarCenter`: the dark full-width header
// (brand block + TopMenu: search, Find Friends, notification/message dropdowns,
// profile). In the shell it is `fixed inset-x-0 top-0`; we force static
// positioning so it flows inside a header-width, header-height card.
// data-template="v1" seeds the --tpl-* tokens (header colour, height, rail width).

const css = `.sc-wrap header{position:static !important}`;

export const Header = () => (
  <div data-template="v1" className="sc-wrap" style={{ width: 900 }}>
    <style>{css}</style>
    <SidebarCenter />
  </div>
);
