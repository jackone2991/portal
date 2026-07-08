import { TopMenu } from "portal-frontend";

// The Olympus header bar: brand block, search, Find Friends, notification
// dropdowns, profile. Full-width and header-height; render it in a sized bar.

export const Header = () => (
  <div
    style={{
      height: "var(--tpl-header-h, 72px)",
      background: "var(--tpl-header, #3f4257)",
      color: "#fff",
    }}
  >
    <TopMenu />
  </div>
);
