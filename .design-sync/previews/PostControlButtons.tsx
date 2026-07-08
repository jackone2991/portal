import { PostControlButtons } from "portal-frontend";

// Floating FAB column for a post card — award · like · comment · share discs. It
// is positioned absolutely and translated half-outside its parent's right edge
// (render inside a `relative` box) and is hidden below the `sm` breakpoint, so we
// mount it on a light relative card wide enough to clear the sm gate. The like
// disc is shown in its active accent state.

export const FabColumn = () => (
  <div data-template="v1" style={{ background: "#f5f6fa", padding: 16 }}>
    <div
      style={{
        position: "relative",
        width: 500,
        marginRight: 40,
        minHeight: 180,
        background: "var(--tpl-surface)",
        border: "1px solid var(--tpl-border)",
        borderRadius: 12,
        padding: 20,
      }}
    >
      <p style={{ color: "var(--tpl-heading)", fontWeight: 600 }}>Marina Valentine</p>
      <p style={{ marginTop: 8, color: "var(--tpl-text)", fontSize: 14, lineHeight: 1.6 }}>
        The FAB column floats half-outside this card&apos;s right edge — award,
        like, comment, and share, with the like disc shown in its active accent
        state.
      </p>
      <PostControlButtons liked />
    </div>
  </div>
);
