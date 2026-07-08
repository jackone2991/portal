import { ControlBlockButtons } from "portal-frontend";

// Row of circular action discs (the Olympus .control-block-button). The three
// canonical profile actions: add-friend (blue), message (accent), settings
// (muted grey). Rendered on a light card so the coloured discs read.
// data-template="v1" seeds the --tpl-* tokens for the disc backgrounds/icons.
const box: React.CSSProperties = {
  background: "#fff",
  padding: 24,
  maxWidth: 520,
  display: "flex",
  gap: 24,
  alignItems: "center",
};

const buttons = [
  { icon: "happy-face-icon", label: "Add friend", color: "var(--tpl-blue)" },
  { icon: "speech-balloon-icon", label: "Message", color: "var(--tpl-accent)" },
  { icon: "three-dots-icon", label: "Settings" },
];

export const Actions = () => (
  <div data-template="v1" style={box}>
    <ControlBlockButtons buttons={buttons} />
  </div>
);

export const Large = () => (
  <div data-template="v1" style={box}>
    <ControlBlockButtons buttons={buttons} size={56} />
  </div>
);
