import { activeTemplate } from "@/templates/registry";

/** One playlist "/library/music/playlists/[id]". In Next 15 `params` is a Promise. */
export default async function PlaylistDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const View = activeTemplate().views.libraryMusicPlaylistDetail;
  return <View id={id} />;
}
