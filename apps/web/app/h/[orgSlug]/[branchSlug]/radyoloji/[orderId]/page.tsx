import { RadyolojiOrderDetailPage } from "@medigt/views/radyoloji";

export default async function Page({
  params,
}: {
  params: Promise<{ orderId: string }>;
}) {
  const { orderId } = await params;
  return <RadyolojiOrderDetailPage orderId={orderId} />;
}
