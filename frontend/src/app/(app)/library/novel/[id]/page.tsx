import { activeTemplate } from "@/templates/registry";

/** Novel detail "/library/novel/[id]". In Next 15 `params` is a Promise. */
export default async function NovelDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const View = activeTemplate().views.libraryNovelDetail;
  return <View id={id} />;
}
