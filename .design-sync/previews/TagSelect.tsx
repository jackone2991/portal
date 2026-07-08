import { TagSelect } from "portal-frontend";

// Multi-value chip input. Each value is a removable pill; the input trails the
// chips. Suggestions only surface while focused, so a still screenshot shows the
// resolved chips. data-template="v1" seeds the --tpl-* tokens.
const box: React.CSSProperties = {
  background: "#fff",
  padding: 20,
  maxWidth: 520,
  display: "grid",
  gap: 24,
};

export const Collaborators = () => (
  <div data-template="v1" style={box}>
    <TagSelect
      label="Collaborators"
      values={["Mathilda Brinker", "Nicholas Grissom"]}
      onChange={() => {}}
      placeholder="Add a collaborator…"
    />
  </div>
);

export const Genres = () => (
  <div data-template="v1" style={box}>
    <TagSelect
      label="Genres"
      values={["Ambient", "Synthwave", "Lo-Fi"]}
      onChange={() => {}}
    />
  </div>
);
