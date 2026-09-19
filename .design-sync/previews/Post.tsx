import { Post } from "portal-frontend";

// Full post card — header (avatar · author · action · time · options menu), body
// text, an optional media slot (a video/link card, or the Entry's photos as a
// hero + thumb strip), the ReactionBar footer, and the half-outside
// PostControlButtons FAB column. Text- and media-rich, so it exercises the DS
// tokens, sprite icons, and typographic scale end to end.
//
// Photo variants point at the media API and will not resolve here, so the
// gallery stories show its placeholder tiles — the honest look of "these
// pictures exist but this page cannot reach them"; the layout is the point.

const frame = { background: "#f5f6fa", padding: 16, maxWidth: 640 };
const ids = (n: number) => Array.from({ length: n }, (_, i) => `photo-${i + 1}`);

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

// One photo: the hero alone, no thumb row — the frame a single-photo Entry
// has always had.
export const PhotoPost = () => (
  <div data-template="v1" style={frame}>
    <Post
      author="Nguyễn Lâm"
      time="Yesterday · good"
      text="Sáng chạy 5km ở công viên Thống Nhất."
      media={{ type: "photos", assetIds: ids(1) }}
      likes={0}
      likedBy={[]}
      comments={0}
      shares={0}
    />
  </div>
);

// Six photos: hero, four thumbs, and "+1" on the last thumb for the rest.
export const GalleryPost = () => (
  <div data-template="v1" style={frame}>
    <Post
      author="Nguyễn Lâm"
      time="2 days ago"
      text="Đà Lạt cuối tuần — cả một cuộn phim trong một ghi chú."
      media={{ type: "photos", assetIds: ids(6) }}
      location={{ name: "Đà Lạt", href: "https://www.openstreetmap.org/#map=13/11.9404/108.4583" }}
      likes={0}
      likedBy={[]}
      comments={0}
      shares={0}
    />
  </div>
);
