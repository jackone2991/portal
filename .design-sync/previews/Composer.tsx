import { Composer } from "portal-frontend";

// Journal composer — the Olympus `.news-feed-form` create-post box: Status /
// Multimedia / Blog tabs, avatar + textarea, and an add-options row with the
// photo/tag/location icons beside Preview and the accent "Post Status" submit.
//
// Fully controlled (D-32): the caller owns the draft and the mutation, so every
// state below is just a different prop set — no interaction needed to show them.

const frame = (children: React.ReactNode) => (
  <div data-template="v1" style={{ background: "var(--tpl-bg, #f5f6fa)", padding: 16, maxWidth: 640 }}>
    {children}
  </div>
);

export const Empty = () =>
  frame(
    <Composer
      displayName="Marina Valentine"
      bodyMd=""
      onBodyMdChange={() => {}}
      onSubmit={() => {}}
    />,
  );

export const Drafting = () =>
  frame(
    <Composer
      displayName="Marina Valentine"
      bodyMd="Vừa đọc xong chương mới của Vạn Cổ Thần Đế — nhịp truyện nhanh hơn hẳn arc trước."
      onBodyMdChange={() => {}}
      onSubmit={() => {}}
    />,
  );

export const Submitting = () =>
  frame(
    <Composer
      displayName="Marina Valentine"
      bodyMd="Đang đăng ghi chú này…"
      onBodyMdChange={() => {}}
      onSubmit={() => {}}
      submitting
    />,
  );

export const WithError = () =>
  frame(
    <Composer
      displayName="Marina Valentine"
      bodyMd="Ghi chú không gửi được."
      onBodyMdChange={() => {}}
      onSubmit={() => {}}
      error="Không đăng được — thử lại sau."
    />,
  );
