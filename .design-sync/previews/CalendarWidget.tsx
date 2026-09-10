import { CalendarWidget, DSQuerySeed } from "portal-frontend";

// Month grid on the home rail, dotting the days that have journal entries.
//
// It needs TWO caches, which is why it was blank: ["time-config"] gives it the
// server's authoritative instant and timezone (the app never trusts the browser
// clock), and ["journal","calendar"] gives it the entries to dot. Without the
// first it never even picks a month to render.
//
// The capture harness runs a fixed clock, so `now` is pinned here rather than
// computed — a card whose month depends on when it was screenshotted would
// re-grade itself on every run.

const now = new Date("2026-09-10T09:00:00Z");

const entry = (day: number, body: string, mood: string | null) => ({
  id: `e${day}`,
  body_md: body,
  mood,
  asset_ids: [],
  occurred_at: `2026-09-${String(day).padStart(2, "0")}T08:00:00Z`,
  created_at: `2026-09-${String(day).padStart(2, "0")}T08:00:00Z`,
  updated_at: `2026-09-${String(day).padStart(2, "0")}T08:00:00Z`,
});

const entries = [
  entry(2, "Chạy bộ buổi sáng, 5km.", "good"),
  entry(3, "Họp review kiến trúc backend.", null),
  entry(6, "Đọc xong Đắc Nhân Tâm.", "great"),
  entry(7, "Cà phê với Hà.", "good"),
  entry(9, "Sửa xong bug seek của trình phát.", "great"),
  entry(10, "Dọn ledger tháng 8.", null),
];

const frame = (seed: unknown, children: React.ReactNode) => (
  <DSQuerySeed
    seed={[
      [["time-config"], { now, timezone: "Asia/Ho_Chi_Minh" }],
      [["journal", "calendar"], seed],
    ]}
  >
    <div data-template="v1" style={{ width: 320, background: "var(--tpl-bg)", padding: 12 }}>{children}</div>
  </DSQuerySeed>
);

export const WithEntries = () => frame({ items: entries, next_cursor: null }, <CalendarWidget />);

// A month nobody wrote in: the grid still renders, just undotted. That is an
// ordinary state for a new journal, not an empty-data failure.
export const EmptyMonth = () => frame({ items: [], next_cursor: null }, <CalendarWidget />);
