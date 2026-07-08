import { CommentThread } from "portal-frontend";

// A comment list — CommentItems (each with up to one nested reply level) followed
// by a "View more comments" affordance. Rendered on a white surface at post width.

export const Thread = () => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <div
      style={{
        width: 600,
        background: "var(--tpl-surface)",
        border: "1px solid var(--tpl-border)",
        borderRadius: 12,
        padding: 20,
      }}
    >
      <CommentThread
        comments={[
          {
            author: "Diego Morales",
            time: "1 hour ago",
            text: "The colour grade on the rooftop scene is unreal — those teal shadows are doing so much work.",
            likes: 12,
            replies: [
              {
                author: "Marina Valentine",
                time: "48 minutes ago",
                text: "Thank you! We pushed the anamorphic bloom a little further than the test reel — glad it landed.",
                likes: 4,
              },
            ],
          },
          {
            author: "Priya Anand",
            time: "37 minutes ago",
            text: "Saving this for the crew screening notes. Can we get a breakdown of the gimbal move at 6:12? It reads almost like a crane shot.",
            likes: 3,
          },
        ]}
      />
    </div>
  </div>
);
