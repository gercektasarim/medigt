import { AdmissionDetailPage } from "@medigt/views/yatis";

export default async function Page({
  params,
}: {
  params: Promise<{ admissionId: string }>;
}) {
  const { admissionId } = await params;
  return <AdmissionDetailPage admissionId={admissionId} />;
}
