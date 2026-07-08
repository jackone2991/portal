import { Avatar } from "portal-frontend";

// Deterministic initials avatar — hue derived from the name, so each person is
// a stable colour. Text-heavy: exercises the DS font + initials layout.

export const Sizes = () => (
  <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
    <Avatar name="Ada Lovelace" size={28} />
    <Avatar name="Ada Lovelace" size={40} />
    <Avatar name="Ada Lovelace" size={56} />
    <Avatar name="Ada Lovelace" size={72} />
  </div>
);

export const People = () => (
  <div style={{ display: "flex", alignItems: "flex-start", gap: 12 }}>
    {["Ada Lovelace", "Grace Hopper", "Alan Turing", "Katherine Johnson", "Linus Torvalds"].map(
      (n) => (
        <div key={n} style={{ display: "grid", justifyItems: "center", gap: 6, width: 88 }}>
          <Avatar name={n} size={52} />
          <span style={{ fontSize: 12, color: "#515365", textAlign: "center" }}>{n}</span>
        </div>
      ),
    )}
  </div>
);
