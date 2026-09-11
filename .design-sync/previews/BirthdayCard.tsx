import { BirthdayCard, DSQuerySeed } from "portal-frontend";

// The home rail's birthday nudge. It queries ["people","upcoming"] itself and
// shows only the NEXT birthday — returning null while loading or when nobody has
// one coming up, which is why an unseeded card is blank.
//
// The wording branches on days_until (today / tomorrow / in N days), so each
// cell here is really a different branch of that sentence.

const person = (over: Record<string, unknown>) => [{
  person_id: "p1",
  display_name: "Nguyễn Thu Hà",
  next_occurrence: "2026-09-14",
  days_until: 4,
  age_turning: 29,
  ...over,
}];

const frame = (seed: unknown, children: React.ReactNode) => (
  <DSQuerySeed seed={[[["people", "upcoming"], seed]]}>
    <div data-template="v1" style={{ width: 320, background: "var(--tpl-bg)", padding: 12 }}>{children}</div>
  </DSQuerySeed>
);

export const InAFewDays = () => frame(person({}), <BirthdayCard />);

export const Today = () =>
  frame(person({ days_until: 0, display_name: "Trần Minh Khôi", age_turning: 34 }), <BirthdayCard />);

export const Tomorrow = () =>
  frame(person({ days_until: 1, display_name: "Lê Bảo Ngọc", age_turning: undefined }), <BirthdayCard />);
