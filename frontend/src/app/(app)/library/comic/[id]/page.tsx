import { activeTemplate } from "@/templates/registry";

/** /library/comic/[id] — comic detail (SPEC-02 P0.5). */
export default async function ComicDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const View = activeTemplate().views.libraryComicDetail;
  return <View id={id} />;
}
