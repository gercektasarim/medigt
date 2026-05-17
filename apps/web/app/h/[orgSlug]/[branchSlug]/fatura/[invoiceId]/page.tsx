import { InvoiceDetailPage } from "@medigt/views/fatura";

export default async function Page({
  params,
}: {
  params: Promise<{ invoiceId: string }>;
}) {
  const { invoiceId } = await params;
  return <InvoiceDetailPage invoiceId={invoiceId} />;
}
