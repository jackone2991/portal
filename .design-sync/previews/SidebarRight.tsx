import { SidebarRight } from "portal-frontend";

// Right friends/contacts panel — port of Olympus `sidebarRight`. Grouped friend
// lists with presence dots, a live "Search Friends" filter, a settings footer,
// and the "Olympus Chat" launcher. Collapses to a thin avatar rail. In the shell
// it is fixed + full-height (width = --tpl-rightbar-w 280 / --tpl-rail-w 68); for
// a standalone card we force display/static/height so it renders as a tall panel.

const css = `
.sb-panel aside{display:flex !important;position:static !important;height:600px !important}
.sb-rail aside{width:var(--tpl-rail-w) !important}
`;

const frame = {
  display: "inline-block",
  padding: 20,
  background: "var(--tpl-bg, #f5f6fa)",
  borderRadius: 12,
};

export const Friends = () => (
  <div data-template="v1" className="sb-panel" style={frame}>
    <style>{css}</style>
    <SidebarRight collapsed={false} onToggle={() => {}} />
  </div>
);

export const Rail = () => (
  <div data-template="v1" className="sb-panel sb-rail" style={frame}>
    <style>{css}</style>
    <SidebarRight collapsed onToggle={() => {}} />
  </div>
);
