import { activeTemplate } from "@/templates/registry";

/** /library/comic/[id]/read/[chapterId] — vertical-scroll reader (SPEC-02 P0.3). */
export default async function ComicReaderPage({ params }: { params: Promise<{ id: string; chapterId: string }> }) {
  const { id, chapterId } = await params;
  const View = activeTemplate().views.libraryComicReader;
  return <View id={id} chapterId={chapterId} />;
}
