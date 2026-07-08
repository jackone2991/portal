import { TrackItem } from "portal-frontend";

// Olympus playlist rows: index, gradient thumb with a hover/active play affordance,
// title + artist, duration, and a three-dots menu. TrackItem renders an <li>, so
// several are composed inside an <ol> on a white card; the first row is `playing`.

export const Playlist = () => (
  <div style={{ background: "#ffffff", padding: 16, maxWidth: 360 }}>
    <ol>
      <TrackItem index={1} title="ChillGroves" artist="Iron Maid" duration="3:24" playing onPlay={() => {}} />
      <TrackItem index={2} title="Midnight Static" artist="The Velvet Circuit" duration="4:07" onPlay={() => {}} />
      <TrackItem index={3} title="Paper Boats" artist="Marisol Vega" duration="2:58" onPlay={() => {}} />
    </ol>
  </div>
);
