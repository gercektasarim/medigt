import { LabOrderDetailPage } from "@medigt/views/laboratuvar";

export default async function Page({
  params,
}: {
  params: Promise<{ orderId: string }>;
}) {
  const { orderId } = await params;
  return <LabOrderDetailPage orderId={orderId} />;
}
