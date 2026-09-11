import { ChapterMenu } from "portal-frontend";

// The chapter picker: a bottom sheet listing every chapter with the current one
// marked. Chapters arrive out of order from the scraper, so `sort_order` — parsed
// from the title, never from arrival — is what orders this list.

const chapters = Array.from({ length: 14 }, (_, i) => {
  const n = 412 - i;
  return {
    id: `ch-${n}`,
    comic_id: "c1",
    title: `Chương ${n}`,
    sort_order: n,
    created_at: "2026-08-20T10:00:00Z",
  };
});

const stage = (children: React.ReactNode) => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 520, background: "#0b0b0d" }}>
    {children}
  </div>
);

export const Chapters = () =>
  stage(
    <ChapterMenu open onClose={() => {}} comicId="c1" chapters={chapters} activeChapterId="ch-408" />,
  );

// A short series: the sheet shrinks to its content rather than holding a
// fixed-height scroller.
export const FewChapters = () =>
  stage(
    <ChapterMenu
      open
      onClose={() => {}}
      comicId="c1"
      chapters={chapters.slice(0, 3)}
      activeChapterId="ch-411"
    />,
  );
