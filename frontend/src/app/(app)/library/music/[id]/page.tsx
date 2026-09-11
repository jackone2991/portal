import { activeTemplate } from "@/templates/registry";

/** Track detail "/library/music/[id]". In Next 15 `params` is a Promise. */
export default async function MusicDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const View = activeTemplate().views.libraryMusicDetail;
  return <View id={id} />;
}
