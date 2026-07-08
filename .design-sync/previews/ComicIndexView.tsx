import { ComicIndexView } from "portal-frontend";

// Library · Comics index — header ("Comics" + accent "Add book" button) over a
// responsive cover grid. v1 has no comics API yet, so the grid renders five
// placeholder cover skeletons (3:4 bordered surfaces); that is the real
// component, authored honestly. Normally lives inside MasterBase, so wrapped in
// a padded page surface here.

export const Index = () => (
  <div style={{ background: "var(--tpl-body, #f5f6fa)", padding: 32 }}>
    <ComicIndexView />
  </div>
);
