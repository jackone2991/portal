import { SelectField } from "portal-frontend";

// Floating-label native <select>. The label stays floated (a select always
// carries a value) and recolours to the accent on focus. data-template="v1"
// seeds the --tpl-* tokens so colours resolve standalone.
const box: React.CSSProperties = {
  background: "#fff",
  padding: 20,
  maxWidth: 520,
  display: "grid",
  gap: 24,
};

export const Select = () => (
  <div data-template="v1" style={box}>
    <SelectField
      label="Band Type"
      options={["Rock Band", "Pop Band", "Jazz Band"]}
      value="Pop Band"
      onChange={() => {}}
    />
    <SelectField
      label="Primary Genre"
      options={[
        { value: "synthwave", label: "Synthwave" },
        { value: "ambient", label: "Ambient" },
        { value: "lofi", label: "Lo-Fi" },
      ]}
      value="synthwave"
      onChange={() => {}}
    />
  </div>
);
