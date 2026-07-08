import { MasterBase } from "portal-frontend";

// Authenticated app shell — port of `master/master-base.blade.php` (Olympus):
// fixed dark header (SidebarCenter) + fixed left nav (SidebarLeft) + right
// friends rail (SidebarRight) around a light content area holding {children}.
//
// The shell's chrome is `position:fixed` and gated behind the `xl` (1280px)
// breakpoint, while the capture viewport is 900px. To show the FULL three-column
// shell in a single card we pin the shell to a bounded 640px frame: chrome
// becomes `position:absolute` inside it, the `hidden xl:flex` sidebars are
// forced visible, and the content gets the sidebar-width gutters the xl rule
// would otherwise add. See .design-sync/learnings/shells.md — a wider viewport
// override renders this at natural proportions without the overrides.

const css = `
#hellopreloader{display:none !important}
.mb-wrap{position:relative;width:850px;height:640px;overflow:hidden}
.mb-wrap header{position:absolute !important}
.mb-wrap aside{display:flex !important;position:absolute !important;height:568px !important}
.mb-wrap main{padding-left:var(--tpl-sidebar-w) !important;padding-right:var(--tpl-rightbar-w) !important;min-height:640px}
`;

const card = {
  background: "#fff",
  borderRadius: 12,
  border: "1px solid var(--tpl-border, #e6ecf5)",
  padding: 16,
  marginBottom: 14,
};

export const App = () => (
  <div className="mb-wrap">
    <style>{css}</style>
    <MasterBase>
      <div style={{ paddingTop: 4 }}>
        <h1
          style={{
            margin: "0 0 4px",
            fontSize: 20,
            fontWeight: 700,
            color: "var(--tpl-heading, #3f4257)",
          }}
        >
          Newsfeed
        </h1>
        <p style={{ margin: "0 0 16px", fontSize: 13, color: "var(--tpl-muted, #888da8)" }}>
          Latest from people you follow
        </p>

        <div style={card}>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
            <div
              style={{
                width: 38,
                height: 38,
                borderRadius: "50%",
                background:
                  "linear-gradient(135deg, var(--tpl-accent, #ff5e3a), var(--tpl-accent-2, #ff763a))",
              }}
            />
            <div>
              <div style={{ fontSize: 13, fontWeight: 600, color: "var(--tpl-heading, #3f4257)" }}>
                Carol Summers
              </div>
              <div style={{ fontSize: 11, color: "var(--tpl-muted, #888da8)" }}>2 hours ago</div>
            </div>
          </div>
          <p style={{ margin: 0, fontSize: 13, lineHeight: 1.6, color: "var(--tpl-text, #515365)" }}>
            Just uploaded a new travel vlog — transcoding to HLS now. Playback in a minute!
          </p>
          <div
            style={{
              height: 120,
              marginTop: 12,
              borderRadius: 10,
              background: "var(--tpl-surface-2, #f6f7fb)",
            }}
          />
        </div>

        <div style={card}>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <div
              style={{
                width: 38,
                height: 38,
                borderRadius: "50%",
                background: "var(--tpl-blue, #38a9ff)",
              }}
            />
            <p style={{ margin: 0, fontSize: 13, color: "var(--tpl-text, #515365)" }}>
              <strong style={{ color: "var(--tpl-heading, #3f4257)" }}>Nina Kraviz</strong> added 3
              tracks to a playlist
            </p>
          </div>
        </div>
      </div>
    </MasterBase>
  </div>
);
