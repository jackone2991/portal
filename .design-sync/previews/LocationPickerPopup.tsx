import { LocationPickerPopup } from "portal-frontend";

// Location picker for a journal entry or post. The map is a hand-rolled slippy
// map — a grid of 256px OpenStreetMap tiles, with the attribution the OSM tile
// policy requires in the corner.
//
// The tiles come from the network. In a preview that either works (real map) or
// leaves the grid blank behind the chrome; either way the dialog's controls,
// search box and attribution are the thing being carded.

const stage = (children: React.ReactNode) => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 620, background: "var(--tpl-bg)" }} data-template="v1">
    {children}
  </div>
);

export const Picker = () => stage(<LocationPickerPopup open onClose={() => {}} onPick={() => {}} />);

// Re-opening on a place already chosen: it centres there and zooms in, rather
// than starting from the default view again.
export const WithInitial = () =>
  stage(
    <LocationPickerPopup
      open
      onClose={() => {}}
      onPick={() => {}}
      initial={{ name: "Nhà thờ Đức Bà, Quận 1, TP.HCM", lat: 10.7797, lon: 106.699 }}
    />,
  );
