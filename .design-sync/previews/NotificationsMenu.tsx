import { NotificationsMenu } from "portal-frontend";

// Olympus header dropdown. Trigger (white thunder icon + unread badge) is built
// for the dark header bar; the panel renders only when `open`, absolute right-0
// under the trigger. Forced open on a header-tinted, sized relative container so
// the white notifications panel (avatar + who/action + type icon) stays in-card.

export const Open = () => (
  <div
    style={{
      position: "relative",
      width: 380,
      minHeight: 420,
      display: "flex",
      justifyContent: "flex-end",
      alignItems: "flex-start",
      padding: 16,
      borderRadius: 12,
      background: "var(--tpl-header, #3f4257)",
    }}
  >
    <NotificationsMenu open onToggle={() => {}} />
  </div>
);
