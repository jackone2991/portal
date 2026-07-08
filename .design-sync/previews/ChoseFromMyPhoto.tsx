import { ChoseFromMyPhoto } from "portal-frontend";

export const Dialog = () => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 600, background: "#f5f6fa" }}>
    <ChoseFromMyPhoto open onClose={() => {}} />
  </div>
);
