import { CategoryDonut } from "portal-frontend";

// The month breakdown behind GET /bank/report. Slices arrive already rolled up
// to their TOP-LEVEL category in SQL (COALESCE(parent_id, id)) — a breakdown that
// split "Cà phê" out of "Ăn uống" is a chart of slivers. `minSharePct` folds the
// long tail into one "Khác" slice for the same reason.
//
// `centerLabel` is the small uppercase CAPTION above the figure — the component
// formats `total` itself. Passing the amount here prints it twice.

const slices = (rows: [string, string, string, number, number][]) =>
  rows.map(([category_id, name, color, total, tx_count]) => ({
    category_id, name, kind: "expense" as const, icon: null, color, total, tx_count,
  }));

const month = slices([
  ["c1", "Ăn uống", "#f97316", 4_850_000, 42],
  ["c2", "Nhà cửa", "#8b5cf6", 3_200_000, 4],
  ["c3", "Đi lại", "#0ea5e9", 1_450_000, 18],
  ["c4", "Sức khoẻ", "#ef4444", 890_000, 3],
  ["c5", "Giải trí", "#ec4899", 640_000, 9],
]);

const frame = (children: React.ReactNode) => (
  <div data-template="v1" className="rounded-xl p-5" style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)", width: 340 }}>
    {children}
  </div>
);

export const MonthBreakdown = () =>
  frame(<CategoryDonut slices={month} total={11_030_000} centerLabel="Chi tháng 8" />);

// The long tail: eight small categories where six fall under the 3% floor and
// collapse into one slice instead of eighty unreadable slivers.
export const WithLongTail = () =>
  frame(
    <CategoryDonut
      slices={slices([
        ["a", "Ăn uống", "#f97316", 2_400_000, 30],
        ["b", "Nhà cửa", "#8b5cf6", 1_800_000, 2],
        ["c", "Cà phê", "#a16207", 120_000, 8],
        ["d", "Sách", "#14b8a6", 95_000, 2],
        ["e", "Quà", "#f43f5e", 80_000, 1],
        ["f", "Gửi xe", "#64748b", 60_000, 12],
        ["g", "Điện thoại", "#3b82f6", 50_000, 1],
        ["h", "Khác", "#94a3b8", 40_000, 3],
      ])}
      total={4_645_000}
      centerLabel="Tổng chi"
    />,
  );

// An empty month is a real state — a new ledger, or one with nothing spent yet.
export const Empty = () => frame(<CategoryDonut slices={[]} total={0} centerLabel="Chi tháng này" />);
