import { MessagesMenu } from "portal-frontend";

// Olympus header dropdown. Trigger (white icon + unread badge) is built for the
// dark header bar; the message panel renders only when `open`, absolute right-0
// under the trigger. Forced open on a header-tinted, sized relative container so
// the white panel with avatar rows + previews stays inside the card.

export const Open = () => (
  <div
    style={{
      position: "relative",
      width: 380,
      minHeight: 340,
      display: "flex",
      justifyContent: "flex-end",
      alignItems: "flex-start",
      padding: 16,
      borderRadius: 12,
      background: "var(--tpl-header, #3f4257)",
    }}
  >
    <MessagesMenu open onToggle={() => {}} />
  </div>
);
