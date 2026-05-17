import { RaporRunnerPage } from "@medigt/views/rapor";

export default async function Page({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  return <RaporRunnerPage slug={slug} />;
}
