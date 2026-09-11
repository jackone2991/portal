import { StreamItemCard, DSQuerySeed } from "portal-frontend";

// One row of the life stream (SPEC-06). The stream mixes two shapes: journal
// items carry a body the user wrote and can edit inline, while system items are
// projections of something that happened elsewhere (a track published, a comic
// chapter read) and carry a title + link instead.
//
// It reads ["time-config"] for the display timezone — the app never trusts the
// browser clock — so that cache is seeded; without it the date line is wrong.

const now = new Date("2026-09-10T09:00:00Z");

const frame = (children: React.ReactNode) => (
  <DSQuerySeed seed={[[["time-config"], { now, timezone: "Asia/Ho_Chi_Minh" }]]}>
    <div data-template="v1" style={{ width: 440, background: "var(--tpl-bg)", padding: 12 }}>{children}</div>
  </DSQuerySeed>
);

const journal = {
  id: "s1",
  source_module: "journal",
  event_type: "journal.entry_created",
  ref_id: "e1",
  occurred_at: "2026-09-09T01:40:00Z",
  body_md: "Sáng chạy 5km ở công viên Thống Nhất. Trời mát, chạy thấy nhẹ hẳn so với tuần trước.",
  mood: "good",
};

const system = {
  id: "s2",
  source_module: "music",
  event_type: "music.track_published",
  ref_id: "t1",
  occurred_at: "2026-09-08T15:05:00Z",
  title: "Đã đăng “Ông Bà Anh” — Lê Thiện Hiếu",
  href: "/library/music/t1",
};

const handlers = {
  onStartEdit: () => {},
  onCancelEdit: () => {},
  onSave: () => {},
  onDelete: () => {},
};

export const JournalItem = () => frame(<StreamItemCard item={journal} displayName="Nguyễn Lâm" {...handlers} />);

// A system item: no body of its own, just what happened and a way back to it.
export const SystemItem = () => frame(<StreamItemCard item={system} displayName="Nguyễn Lâm" {...handlers} />);

// Inline editing — the same row, swapped for its editor rather than a dialog.
export const Editing = () =>
  frame(<StreamItemCard item={journal} displayName="Nguyễn Lâm" editing {...handlers} />);

// Mid-save: the editor stays visible and locked rather than disappearing, so a
// slow request cannot look like the edit was lost.
export const Saving = () =>
  frame(<StreamItemCard item={journal} displayName="Nguyễn Lâm" editing saving {...handlers} />);
