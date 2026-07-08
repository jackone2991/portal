ChatResponsive from portal-frontend. Use via `window.PortalUI.ChatResponsive` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Responsive chat panel — port of Olympus `.popup-chat-responsive`
(`Fav Page - Settings And Create Popup.html`). A floating chat window with a
presence header, a scrolling message list (incoming / outgoing bubbles), and
a composer. Controlled via `open` / `onClose`; docks bottom-right.

## Examples

### Chat

```jsx
() => (
  <div style={{ position: "relative", transform: "translateZ(0)", height: 440, background: "#eef0f5" }}>
    <ChatResponsive open onClose={() => {}} contact="Marina Valentine" />
  </div>
)
```
