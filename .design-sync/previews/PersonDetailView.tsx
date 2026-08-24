import { DSQuerySeed, PersonDetailView } from "portal-frontend";

// People registry detail screen (SPEC-08): name + relationship, a delete action,
// and a definition list of birthday / contact / note.
//
// It fetches ["person", id] itself and renders "Person not found" when the query
// fails, so the preview seeds the cache rather than showing an empty-state card.

const person = {
  id: "p1",
  display_name: "Nguyễn Thu Hà",
  relationship: "Chị gái",
  birthday: { month: 4, day: 17, year: 1993 },
  contact: { phone: "0912 345 678", email: "ha.nguyen@example.com" },
  note_md: "Thích cà phê sữa đá. Hẹn cà phê mỗi sáng chủ nhật.",
  avatar_asset_id: null,
  created_at: "2026-02-11T09:00:00Z",
  updated_at: "2026-08-19T15:22:00Z",
};

export const Detail = () => (
  <DSQuerySeed seed={[[["person", "p1"], person]]}>
    <div data-template="v1" style={{ background: "#0f1115" }}>
      <PersonDetailView id="p1" />
    </div>
  </DSQuerySeed>
);

// A person with only the required fields — the optional rows drop out, which is
// the layout most real entries actually have.
const sparse = {
  ...person,
  id: "p2",
  display_name: "Trần Minh Quân",
  relationship: null,
  birthday: null,
  contact: {},
  note_md: null,
};

export const Minimal = () => (
  <DSQuerySeed seed={[[["person", "p2"], sparse]]}>
    <div data-template="v1" style={{ background: "#0f1115" }}>
      <PersonDetailView id="p2" />
    </div>
  </DSQuerySeed>
);
