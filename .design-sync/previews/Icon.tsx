import { Icon } from "portal-frontend";

// Olympus sprite icon — `name` is the symbol id without the `olymp-` prefix.
// Colour follows currentColor; width derives from the symbol's viewBox.

const COMMON = [
  "home-icon",
  "newsfeed-icon",
  "heart-icon",
  "comments-post-icon",
  "camera-icon",
  "photos-icon",
  "calendar-icon",
  "settings-icon",
  "magnifying-glass-icon",
  "badge-icon",
  "headphones-icon",
  "music-play-icon-big",
];

export const Gallery = () => (
  <div style={{ display: "flex", flexWrap: "wrap", gap: 18, color: "#3f4257", maxWidth: 470 }}>
    {COMMON.map((n) => (
      <div key={n} style={{ display: "grid", justifyItems: "center", gap: 6, width: 100 }}>
        <Icon name={n} size={26} />
        <span style={{ fontSize: 11, color: "#888da8" }}>{n.replace(/-icon$/, "")}</span>
      </div>
    ))}
  </div>
);

export const Sizes = () => (
  <div style={{ display: "flex", alignItems: "center", gap: 16, color: "#ff5e3a" }}>
    <Icon name="heart-icon" size={16} />
    <Icon name="heart-icon" size={24} />
    <Icon name="heart-icon" size={32} />
    <Icon name="heart-icon" size={48} />
  </div>
);
