import { DialysisSessionDetailPage } from "@medigt/views/diyaliz";

export default async function Page({
  params,
}: {
  params: Promise<{ sessionId: string }>;
}) {
  const { sessionId } = await params;
  return <DialysisSessionDetailPage sessionId={sessionId} />;
}
