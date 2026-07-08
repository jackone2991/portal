MasterBase from portal-frontend. Use via `window.PortalUI.MasterBase` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Authenticated app shell — port of `master/master-base.blade.php` (Olympus).

  fixed dark header (SidebarCenter) with the brand block above the menu
  fixed expandable left menu (SidebarLeft) + right avatar rail (SidebarRight)
  light content area between them, holding the page {children}

The left menu collapses to an icon rail. Its current width lives in the CSS
custom property `--tpl-sidebar-cur`, which the header brand block and the
content padding both read — so everything shifts in lockstep.

## Props

```ts
interface MasterBaseProps {
children: React.ReactNode;
}
```

## Examples

### App

```jsx
() => (
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
)
```
