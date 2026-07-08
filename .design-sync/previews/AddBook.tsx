import { AddBook } from "portal-frontend";

// Fixed-overlay modal — contain it to the card with a transformed relative box.
export const Dialog = () => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 560, background: "#f5f6fa" }}>
    <AddBook open onClose={() => {}} />
  </div>
);
