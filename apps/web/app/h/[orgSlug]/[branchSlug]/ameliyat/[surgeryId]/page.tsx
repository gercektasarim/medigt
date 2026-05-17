import { SurgeryDetailPage } from "@medigt/views/ameliyat";

export default async function Page({
  params,
}: {
  params: Promise<{ surgeryId: string }>;
}) {
  const { surgeryId } = await params;
  return <SurgeryDetailPage surgeryId={surgeryId} />;
}
