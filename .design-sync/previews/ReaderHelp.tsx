import { ReaderHelp } from "portal-frontend";

// The comic reader's keyboard-shortcut sheet. A fixed inset-0 overlay, so the
// card contains it with a transformed relative box like the other popups.
// Controlled by `open`; closed it renders null, which is why the card only has
// the open state — a card of "nothing" teaches nothing.

const stage = (children: React.ReactNode) => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 420, background: "#0b0b0d" }}>
    {children}
  </div>
);

export const Shortcuts = () => stage(<ReaderHelp open onClose={() => {}} />);
