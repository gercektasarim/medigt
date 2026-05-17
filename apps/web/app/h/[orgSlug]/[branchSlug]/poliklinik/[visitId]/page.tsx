import { VisitDetailPage } from "@medigt/views/poliklinik";

export default async function Page({
  params,
}: {
  params: Promise<{ visitId: string }>;
}) {
  const { visitId } = await params;
  return <VisitDetailPage visitId={visitId} />;
}
