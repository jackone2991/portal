import { FriendRequestsMenu } from "portal-frontend";

// Olympus header dropdown. The trigger is a white icon+badge built for the dark
// header bar; the panel renders only when `open`, positioned absolute right-0
// under the trigger. We force it open and frame it on a header-tinted, sized
// relative container so the white panel pops and stays inside the card.

export const Open = () => (
  <div
    style={{
      position: "relative",
      width: 380,
      minHeight: 460,
      display: "flex",
      justifyContent: "flex-end",
      alignItems: "flex-start",
      padding: 16,
      borderRadius: 12,
      background: "var(--tpl-header, #3f4257)",
    }}
  >
    <FriendRequestsMenu open onToggle={() => {}} />
  </div>
);
