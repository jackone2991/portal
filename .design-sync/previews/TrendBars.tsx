import { TrendBars } from "portal-frontend";

// Six months of income vs expense, paired per month: green in, red out. The
// active month is tinted with --tpl-accent so the donut beside it and this chart
// agree on which month you are reading.

const flow = (rows: [string, number, number][]) =>
  rows.map(([month, income, expense]) => ({ month, income, expense }));

const frame = (children: React.ReactNode) => (
  <div data-template="v1" className="rounded-xl p-5" style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)", width: 420 }}>
    {children}
  </div>
);

export const SixMonths = () =>
  frame(
    <TrendBars
      activeMonth="2026-08"
      data={flow([
        ["2026-03", 24_000_000, 18_400_000],
        ["2026-04", 24_000_000, 21_900_000],
        ["2026-05", 26_500_000, 17_200_000],
        ["2026-06", 24_000_000, 25_800_000],
        ["2026-07", 24_000_000, 15_600_000],
        ["2026-08", 31_000_000, 11_030_000],
      ])}
    />,
  );

// No active month: nothing is tinted, every label stays muted.
export const NoActiveMonth = () =>
  frame(
    <TrendBars
      data={flow([
        ["2026-06", 24_000_000, 25_800_000],
        ["2026-07", 24_000_000, 15_600_000],
        ["2026-08", 31_000_000, 11_030_000],
      ])}
    />,
  );

// A month with no movement still occupies its slot — a missing column would read
// as "no data" rather than "nothing happened".
export const WithQuietMonth = () =>
  frame(
    <TrendBars
      activeMonth="2026-08"
      data={flow([
        ["2026-06", 12_000_000, 9_400_000],
        ["2026-07", 0, 0],
        ["2026-08", 12_000_000, 4_100_000],
      ])}
    />,
  );
