import { activeTemplate } from "@/templates/registry";

/** /people/[id] — person detail (SPEC-08 P0.5). */
export default async function PersonDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const View = activeTemplate().views.peopleDetail;
  return <View id={id} />;
}
