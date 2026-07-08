import { Post } from "portal-frontend";

// Full post card — header (avatar · author · action · time · options menu), body
// text, an optional media slot (video/photo link card), the ReactionBar footer,
// and the half-outside PostControlButtons FAB column. Text- and media-rich, so it
// exercises the DS tokens, sprite icons, and typographic scale end to end.

const frame = { background: "#f5f6fa", padding: 16, maxWidth: 640 };

export const TextPost = () => (
  <div data-template="v1" style={frame}>
    <Post
      author="Marina Valentine"
      action="shared a thought"
      time="18 minutes ago"
      text="Just wrapped the final colour pass on the summer short film. Six months of night shoots and it finally looks the way it sounded in my head. Screening the first cut for the crew on Friday."
      likes={128}
      likedBy={["Diego Morales", "Priya Anand", "Anselm Richter"]}
      comments={24}
      shares={7}
      liked
    />
  </div>
);

export const VideoPost = () => (
  <div data-template="v1" style={frame}>
    <Post
      author="Diego Morales"
      action="posted a video"
      time="2 hours ago"
      text="New behind-the-scenes reel is live — camera tests, lens breakdowns, and the gimbal rig that saved the rooftop chase sequence."
      media={{
        type: "video",
        title: "Behind the Lens: Shooting the Rooftop Chase",
        desc: "A twelve-minute walkthrough of the anamorphic setup and the lighting plan for the night exterior.",
        source: "PORTAL STUDIO · 12:04",
      }}
      likes={342}
      likedBy={["Marina Valentine", "Nadia Okonkwo", "Priya Anand", "Anselm Richter"]}
      comments={58}
      shares={19}
    />
  </div>
);
