import { NovelDetailView } from "portal-frontend";

// Library · Novel detail — title ("Novel #<id>") over a two-column layout: a
// 3:4 cover placeholder beside a synopsis/chapter-list column. v1 has no novel
// API yet, so the synopsis is placeholder copy; that is the real component.
// `id` comes from the [id] route segment. Wrapped in a padded page surface
// (normally rendered inside MasterBase).

export const Detail = () => (
  <div style={{ background: "var(--tpl-body, #f5f6fa)", padding: 32 }}>
    <NovelDetailView id="42" />
  </div>
);
