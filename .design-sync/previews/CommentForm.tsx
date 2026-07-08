import { CommentForm } from "portal-frontend";

// Comment composer — an avatar + textarea with an inline camera (photo) button,
// then a "Post Comment" submit and a Cancel that clears the draft. Rendered on a
// white surface at post width.

export const AddComment = () => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <div
      style={{
        width: 600,
        background: "var(--tpl-surface)",
        border: "1px solid var(--tpl-border)",
        borderRadius: 12,
        padding: 20,
      }}
    >
      <CommentForm displayName="Marina Valentine" onSubmit={() => {}} />
    </div>
  </div>
);
