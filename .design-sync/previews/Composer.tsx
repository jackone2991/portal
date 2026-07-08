import { Composer } from "portal-frontend";

// Tabbed create-post box — Status / Multimedia / Blog Post tabs, an avatar +
// textarea, and a footer of photo/tag/location icon buttons with a Preview button
// and the accent "Post Status" submit.

export const CreatePost = () => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16, maxWidth: 640 }}>
    <Composer displayName="Marina Valentine" onPost={() => {}} />
  </div>
);
