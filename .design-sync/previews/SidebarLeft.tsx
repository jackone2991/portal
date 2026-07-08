import { SidebarLeft } from "portal-frontend";

// Left nav menu — port of Olympus `sidebarLeft`. In the app shell it is a
// fixed, full-height panel whose width is driven by the theme token
// --tpl-sidebar-w (272px) / --tpl-rail-w (68px collapsed). For a standalone
// card we neutralize the shell-only layout (fixed positioning, the `hidden
// xl:flex` breakpoint gate, and the calc(100vh) height) so it renders as a
// self-contained tall panel at its natural width. data-template="v1" seeds the
// --tpl-* tokens locally so widths/colours resolve without the shell root.

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

export const Expanded = () => (
  <div data-template="v1" className="sb-panel" style={frame}>
    <style>{css}</style>
    <SidebarLeft collapsed={false} onToggle={() => {}} />
  </div>
);

export const Collapsed = () => (
  <div data-template="v1" className="sb-panel sb-rail" style={frame}>
    <style>{css}</style>
    <SidebarLeft collapsed onToggle={() => {}} />
  </div>
);
