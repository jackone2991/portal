import { ChatResponsive } from "portal-frontend";

// Docks bottom-right; contain it to the card with a transformed relative box.
export const Chat = () => (
  <div style={{ position: "relative", transform: "translateZ(0)", height: 440, background: "#eef0f5" }}>
    <ChatResponsive open onClose={() => {}} contact="Marina Valentine" />
  </div>
);
