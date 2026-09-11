import { MonthPager } from "portal-frontend";

// Month navigation for every bank screen. Controlled: it owns no month of its
// own, so a preview holds one in state — which is also how the real views use it.
//
// The "Hôm nay" chip is a jump-back button, shown only while you are AWAY from
// the current month. Note the capture harness runs on a fixed clock, so the
// month label in the card is not today's date — only the two states matter.

import { useState } from "react";

const frame = (children: React.ReactNode) => (
  <div data-template="v1" style={{ width: 300 }}>{children}</div>
);

export const Current = () => {
  const [month, setMonth] = useState(new Date().toISOString().slice(0, 7));
  return frame(<MonthPager month={month} onChange={setMonth} />);
};

// Away from the current month: the "Hôm nay" chip appears to jump back, and the
// forward arrow is live (on the current month it is blocked — you cannot page
// into a future that has no data).
export const AwayFromToday = () => {
  const [month, setMonth] = useState("2026-03");
  return frame(<MonthPager month={month} onChange={setMonth} />);
};
