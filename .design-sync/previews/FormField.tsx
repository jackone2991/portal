import { FormField } from "portal-frontend";

// Material floating-label input. The label rides up when a field is filled or
// focused; date always floats. Controlled here so the labels sit in the floated
// slot for the still screenshot; a resting/disabled cell shows the low state.
// data-template="v1" seeds the --tpl-* tokens so colours resolve standalone.
const box: React.CSSProperties = {
  background: "#fff",
  padding: 20,
  maxWidth: 520,
  display: "grid",
  gap: 20,
};

export const Fields = () => (
  <div data-template="v1" style={box}>
    <FormField label="First Name" value="Marina" onChange={() => {}} />
    <FormField
      label="Your Email"
      type="email"
      value="marina@spiegel.io"
      icon="info-icon"
      onChange={() => {}}
    />
    <FormField label="Birthday" type="date" value="1993-04-12" onChange={() => {}} />
  </div>
);

export const States = () => (
  <div data-template="v1" style={box}>
    {/* uncontrolled + empty → label rests inside the field */}
    <FormField label="Display Name" placeholder="How should we call you?" />
    <FormField label="Account ID" value="usr_1f9c" onChange={() => {}} disabled />
  </div>
);
