import { HomeView } from "portal-frontend";

// Home / newsfeed — the full Olympus three-rail social page: left rail
// (weather · calendar · pages), centre composer + post feed, right rail
// (story card · friend suggestions · activity feed). Carries its own sample
// feed; posting from the composer optimistically prepends locally.

export const Newsfeed = () => (
  <div style={{ background: "var(--tpl-body, #f5f6fa)", padding: 24 }}>
    <HomeView />
  </div>
);
