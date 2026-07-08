import { FriendRequestItem } from "portal-frontend";

// Olympus friend-requests list row: avatar with presence dot, name, mutual-friend
// count, and accept / decline circular buttons. Rendered on a white card at the
// list width it lives in.

export const Default = () => (
  <div style={{ background: "#ffffff", padding: 16, maxWidth: 360 }}>
    <FriendRequestItem
      name="Green Goo Rock"
      mutual={12}
      status="online"
      onAccept={() => {}}
      onDecline={() => {}}
    />
  </div>
);
