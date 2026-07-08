import { FriendCard } from "portal-frontend";

// Olympus profile "friend-item" card: gradient cover strip, overlapping avatar,
// name + country, a Friends/Photos/Videos stats row, and the add/message
// control buttons. Framed on the app body surface at card width.

export const Default = () => (
  <div style={{ background: "#f5f6fa", padding: 16, maxWidth: 360 }}>
    <FriendCard
      name="Diana Jameson"
      country="United States"
      stats={{ friends: 428, photos: 197, videos: 34 }}
    />
  </div>
);
