import { activeTemplate } from "@/templates/registry";

/** Media player "/library/media/[id]" (SPEC-07 P0.4). In Next 15 `params` is a Promise. */
export default async function MediaDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const View = activeTemplate().views.libraryMediaDetail;
  return <View id={id} />;
}
