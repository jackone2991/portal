import { EntryCard } from "portal-frontend";

// One journal entry in the life stream. Pure props — the reason it was blank is
// that `entry` is required and auto-props had nothing to put in it.
//
// The card shows "đã sửa" only when updated_at is more than a minute after
// created_at, so the edited state needs two genuinely different timestamps
// rather than a flag.

const entry = (over: Record<string, unknown> = {}) => ({
  id: "e1",
  body_md: "Sáng chạy 5km ở công viên Thống Nhất. Trời mát, chạy thấy nhẹ hẳn so với tuần trước.",
  mood: "good",
  asset_ids: [],
  occurred_at: "2026-09-09T01:30:00Z",
  created_at: "2026-09-09T01:40:00Z",
  updated_at: "2026-09-09T01:40:00Z",
  ...over,
});

const frame = (children: React.ReactNode) => (
  <div data-template="v1" style={{ width: 440, background: "var(--tpl-bg)", padding: 12 }}>{children}</div>
);

const handlers = { onRequestEdit: () => {}, onRequestDelete: () => {} };

export const Entry = () => frame(<EntryCard displayName="Nguyễn Lâm" entry={entry()} {...handlers} />);

// Edited after the fact: updated_at drifts more than a minute past created_at,
// which is what turns on the "edited" marker.
export const Edited = () =>
  frame(
    <EntryCard
      displayName="Nguyễn Lâm"
      entry={entry({
        body_md: "Họp review kiến trúc backend.\n\nChốt: tách module bank khỏi journal, event đi qua Asynq chứ không JOIN chéo bảng.",
        mood: null,
        updated_at: "2026-09-09T09:12:00Z",
      })}
      {...handlers}
    />,
  );

// A long entry with markdown — the body is rendered, not shown raw.
export const LongBody = () =>
  frame(
    <EntryCard
      displayName="Nguyễn Lâm"
      entry={entry({
        mood: "great",
        body_md:
          "Cuối cùng cũng sửa xong lỗi seek của trình phát nhạc.\n\n" +
          "Nguyên nhân không nằm ở front-end: route `/assets/{id}/original` trả về bằng `io.Copy`, " +
          "nên không có `Accept-Ranges`. Trình duyệt báo `seekable = [0,0]` và **im lặng** bỏ qua mọi cú tua.\n\n" +
          "Bài học: thanh tiến trình vẫn chạy không có nghĩa là tua được.",
      })}
      {...handlers}
    />,
  );
