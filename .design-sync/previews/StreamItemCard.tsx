import { Composer, StreamItemCard, DSQuerySeed } from "portal-frontend";

// One row of the life stream (SPEC-06). The stream mixes two shapes: journal
// items carry a body the user wrote and can be edited in place — the caller
// hands the card a composer as `editor` and it renders where the card was
// (SPEC-12 T5) — while system items are projections of something that
// happened elsewhere (a track published, a comic chapter read) and carry a
// title + link instead.
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

// A placed Entry: the Location arrives as a field on the item (SPEC-12 T3) and
// renders as the pin chip that opens the map — never parsed out of the body.
const placed = {
  ...journal,
  id: "s3",
  ref_id: "e3",
  body_md: "Sáng nay đi bộ quanh hồ.",
  location: { name: "Hoàn Kiếm", lat: 21.0286, lon: 105.8506 },
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
  onDelete: () => {},
};

// The edit surface: the same composer, pre-filled from the Entry, with Cancel.
const editor = (submitting: boolean) => (
  <Composer
    displayName="Nguyễn Lâm"
    bodyMd={placed.body_md}
    onBodyMdChange={() => {}}
    onSubmit={() => false}
    onCancel={() => {}}
    initial={{ mood: placed.mood, assetIds: [], location: placed.location }}
    submitting={submitting}
  />
);

export const JournalItem = () => frame(<StreamItemCard item={journal} displayName="Nguyễn Lâm" {...handlers} />);

export const PlacedItem = () => frame(<StreamItemCard item={placed} displayName="Nguyễn Lâm" {...handlers} />);

// A system item: no body of its own, just what happened and a way back to it.
export const SystemItem = () => frame(<StreamItemCard item={system} displayName="Nguyễn Lâm" {...handlers} />);

// Editing in place — the same row, swapped for the composer rather than a dialog.
export const Editing = () =>
  frame(<StreamItemCard item={placed} displayName="Nguyễn Lâm" editor={editor(false)} {...handlers} />);

// Mid-save: the composer stays visible and locked rather than disappearing, so
// a slow request cannot look like the edit was lost.
export const Saving = () =>
  frame(<StreamItemCard item={placed} displayName="Nguyễn Lâm" editor={editor(true)} {...handlers} />);
