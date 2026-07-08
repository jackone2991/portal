import { ReactionBar } from "portal-frontend";

// Post footer reaction row — heart + like count, overlapping liker avatars, and a
// "X liked this" label on the left; comment and share counts on the right.
// Rendered inline on a white surface at post width.

const card = {
  width: 560,
  background: "var(--tpl-surface)",
  border: "1px solid var(--tpl-border)",
  borderRadius: 12,
  padding: 20,
};

export const Liked = () => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <div style={card}>
      <ReactionBar
        likes={128}
        likedBy={["Marina Valentine", "Diego Morales", "Priya Anand"]}
        comments={24}
        shares={7}
        liked
      />
    </div>
  </div>
);

export const Default = () => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <div style={card}>
      <ReactionBar likes={3} likedBy={["Anselm Richter"]} comments={1} shares={0} />
    </div>
  </div>
);
