import { ReaderChrome } from "portal-frontend";

// The reader's two fixed bars: title/chapter/actions on top, a direction-aware
// page slider and counter on the bottom. Both slide away when `visible` is false,
// which is the immersive reading state.

const stage = (children: React.ReactNode) => (
  // overflow:hidden is load-bearing — the bars hide by translating themselves
  // off-screen, so without a clipping stage the "immersive" cell still shows
  // them, half-slid, and reads as a layout bug rather than a state.
  <div style={{ position: "relative", transform: "translateZ(0)", height: 300, overflow: "hidden", background: "#0b0b0d" }}>
    {children}
  </div>
);

const base = {
  comicId: "c1",
  title: "Vạn Cổ Thần Đế",
  chapterLabel: "Chương 408",
  prevHref: "/library/comic/c1/read/ch-407" as never,
  nextHref: "/library/comic/c1/read/ch-409" as never,
  onOpenSettings: () => {},
  onOpenChapters: () => {},
  onOpenHelp: () => {},
};

export const Visible = () =>
  stage(
    <ReaderChrome
      {...base}
      visible
      dir="ltr"
      slider={{ current: 12, total: 34, onSeek: () => {} }}
    />,
  );

// Right-to-left: the slider reads the other way so the thumb tracks the reading
// direction rather than the number line.
export const RightToLeft = () =>
  stage(
    <ReaderChrome
      {...base}
      visible
      dir="rtl"
      chapterLabel="第 12 話"
      title="ワンピース"
      slider={{ current: 3, total: 19, onSeek: () => {} }}
    />,
  );

// Immersive: both bars are off-screen. The card is deliberately near-empty —
// that IS the state, and it is what tapping the page toggles into.
export const Immersive = () =>
  stage(<ReaderChrome {...base} visible={false} dir="ltr" slider={{ current: 12, total: 34, onSeek: () => {} }} />);
