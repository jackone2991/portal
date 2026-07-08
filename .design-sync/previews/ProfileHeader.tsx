import { ProfileHeader } from "portal-frontend";

// Profile page header: gradient cover + Update Cover affordance, overlapping
// avatar, name + location, the add-friend/message/settings control discs, and a
// horizontal tab menu. Wide + tall and self-surfaced, so it renders directly on
// a light page backdrop. data-template="v1" seeds the --tpl-* tokens.
const page: React.CSSProperties = {
  background: "#f5f6fa",
  padding: 24,
  maxWidth: 720,
};

export const Timeline = () => (
  <div data-template="v1" style={page}>
    <ProfileHeader
      name="James Spiegel"
      location="San Francisco, CA"
      activeTab="Timeline"
      onTab={() => {}}
    />
  </div>
);

export const Photos = () => (
  <div data-template="v1" style={page}>
    <ProfileHeader
      name="James Spiegel"
      location="San Francisco, CA"
      activeTab="Photos"
      onTab={() => {}}
    />
  </div>
);
