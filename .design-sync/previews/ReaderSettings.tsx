import { ReaderSettings } from "portal-frontend";

// Reading preferences: mode, direction, fit, brightness, quality. State lives in
// a Zustand store (`useReaderSettings`), not in props — so the panel needs no
// provider and the card shows whatever the store's defaults are.
//
// Fixed bottom sheet, contained to the card the same way as the other overlays.

const stage = (children: React.ReactNode) => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 460, background: "#0b0b0d" }}>
    {children}
  </div>
);

export const Panel = () => stage(<ReaderSettings open onClose={() => {}} />);
