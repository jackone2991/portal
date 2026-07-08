import { CommentItem } from "portal-frontend";

// A single comment — avatar · author · time · options menu, body text, a heart
// like count and a Reply link. `replies` renders one nested level as a
// `ul.children`. Wrapped in a `ul` on a white surface for valid list markup.

const card = {
  width: 600,
  background: "var(--tpl-surface)",
  border: "1px solid var(--tpl-border)",
  borderRadius: 12,
  padding: 20,
};

export const Single = () => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <ul style={card}>
      <CommentItem
        author="Priya Anand"
        time="37 minutes ago"
        text="Saving this for the crew screening notes. Can we get a breakdown of the gimbal move at 6:12? It reads almost like a crane shot."
        likes={8}
      />
    </ul>
  </div>
);

export const WithReply = () => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <ul style={card}>
      <CommentItem
        author="Diego Morales"
        time="1 hour ago"
        text="The colour grade on the rooftop scene is unreal — those teal shadows are doing so much work."
        likes={12}
        replies={[
          {
            author: "Marina Valentine",
            time: "48 minutes ago",
            text: "Thank you! We pushed the anamorphic bloom further than the test reel — glad it landed.",
            likes: 4,
          },
        ]}
      />
    </ul>
  </div>
);
