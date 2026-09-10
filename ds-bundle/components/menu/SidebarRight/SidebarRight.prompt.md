SidebarRight from portal-frontend. Use via `window.PortalUI.SidebarRight` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Right fixed sidebar — port of `components/menu/sidebarRight.blade.php`
(Olympus), rebuilt on the people registry (SPEC-08).

The reference is a friends list with presence dots and a chat launcher. Portal
has neither a social graph nor presence nor chat, so the port used to ship
eleven invented people with invented ONLINE/AWAY states. What Portal does have
is the people registry, so that is what the rail lists — in three fixed
sections:

  Close Friends / My Family   the two circles (migration 0035)
  Có thể bạn biết             accounts on this instance not yet in the
                              registry, so an empty registry still has
                              something to offer instead of a blank rail

Each heading carries a Settings link to the page that manages that section.

The dots are the reference's presence dots, repurposed. Portal has no presence
system — nobody is "online" — but it does now know whether you are connected
to an account, whether they are waiting on your answer, or whether you are
waiting on theirs. That is real, it is the thing you would act on, and it maps
onto the same four colours. A dot here never claims someone is at their desk.

Everything the data cannot support is gone rather than faked: no status dots,
no per-group "Settings", no row menu, no chat bar. What replaces the chat bar
is a link to the page that can actually add someone.

Failure-isolated: this renders in the shell of every authenticated page, so a
failing or empty query collapses to a quiet empty state, never to a broken
layout.

## Props

```ts
interface SidebarRightProps {
collapsed: boolean; onToggle: () => void;
}
```

## Examples

### Friends

```jsx
() => (
  <div data-template="v1" className="sb-panel" style={frame}>
    <style>{css}</style>
    <SidebarRight collapsed={false} onToggle={() => {}} />
  </div>
)
```

### Rail

```jsx
() => (
  <div data-template="v1" className="sb-panel sb-rail" style={frame}>
    <style>{css}</style>
    <SidebarRight collapsed onToggle={() => {}} />
  </div>
)
```
